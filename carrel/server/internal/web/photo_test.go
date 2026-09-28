package web_test

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math/rand/v2"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

var b64 = base64.StdEncoding.EncodeToString

// halvesImage is red on the left half and blue on the right.
func halvesImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBA{20, 20, 220, 255}
			if x < w/2 {
				c = color.RGBA{220, 20, 20, 255}
			}
			img.Set(x, y, c)
		}
	}
	return img
}

func jpegOf(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func pngOf(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// withOrientation is what a phone writes: an EXIF segment with the
// orientation, and next to it where the photo was taken.
func withOrientation(data []byte, o uint16) []byte {
	tiff := make([]byte, 26)
	copy(tiff, "II")
	binary.LittleEndian.PutUint16(tiff[2:], 42)
	binary.LittleEndian.PutUint32(tiff[4:], 8)
	binary.LittleEndian.PutUint16(tiff[8:], 1)
	binary.LittleEndian.PutUint16(tiff[10:], 0x0112)
	binary.LittleEndian.PutUint16(tiff[12:], 3)
	binary.LittleEndian.PutUint32(tiff[14:], 1)
	binary.LittleEndian.PutUint16(tiff[18:], o)
	payload := append(append([]byte("Exif\x00\x00"), tiff...), "GPS 55.7558N 37.6173E"...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
	out := append(append(append([]byte{}, data[:2]...), seg...), payload...)
	return append(out, data[2:]...)
}

func reddish(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return r>>8 > 150 && g>>8 < 90 && b>>8 < 90
}

func bluish(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return b>>8 > 150 && r>>8 < 90 && g>>8 < 90
}

// bookFixture is one address book with these cards, and a hook on the
// harness before the person connects.
func bookFixture(t *testing.T, configure func(*harness), cards ...string) davFixture {
	t.Helper()
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	book := davtest.AddressBookHome + "people/"
	srv.AddAddressBook(carddav.AddressBook{Path: book, Name: "People"})
	for _, c := range cards {
		srv.Seed(book+uidLine.FindStringSubmatch(c)[1]+".vcf", c)
	}
	return connectFixture(t, srv, book, configure)
}

// uploadRequest sends a file the way a page does. Without scripts _csrf
// comes first in the body; with them it goes in X-CSRF-Token and the body is
// the file alone. An empty name is the form sent with no file chosen.
func uploadRequest(path string, sess *session.Session, header bool, name string, data []byte) *http.Request {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if !header {
		_ = w.WriteField("_csrf", sess.CSRFToken())
	}
	part, _ := w.CreateFormFile("photo", name)
	_, _ = part.Write(data)
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if header {
		req.Header.Set("X-CSRF-Token", sess.CSRFToken())
	}
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: sess.ID()})
	return req
}

// fetchImage GETs an image address and decodes what came back.
func fetchImage(t *testing.T, f davFixture, href string) (*httptest.ResponseRecorder, image.Image) {
	t.Helper()
	rec := f.h.do(get(href, f.sess))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: %d %s", href, rec.Code, rec.Body.String())
	}
	img, _, err := image.Decode(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatalf("GET %s: not an image: %v", href, err)
	}
	return rec, img
}

// photoBytes is the image in the PHOTO line of a card as the server holds it.
func photoBytes(t *testing.T, doc string) []byte {
	t.Helper()
	for _, line := range strings.Split(doc, "\n") {
		line = strings.TrimRight(line, "\r")
		if !strings.HasPrefix(line, "PHOTO") {
			continue
		}
		_, value, _ := strings.Cut(line, ":")
		if strings.HasPrefix(value, "data:") {
			_, value, _ = strings.Cut(value, ",")
		}
		data, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			t.Fatalf("PHOTO is not base64: %v", err)
		}
		return data
	}
	t.Fatalf("no PHOTO in\n%s", doc)
	return nil
}

func hasAction(actions []view.Action, id string) bool {
	for _, a := range actions {
		if a.ID == id {
			return true
		}
	}
	return false
}

// S-13: the photo is its own route. A list's thumbnails come from the cards
// the list read, so showing them asks the server nothing; the version of the
// card is the address and the ETag; the type is the one declared, as an HTTP
// type, and a declared type the bytes do not match is not served at all.
func TestPhotoRouteServesWhatTheCardKeeps(t *testing.T) {
	pngPhoto := pngOf(t, halvesImage(120, 120))
	jpegPhoto := jpegOf(t, halvesImage(120, 120))
	f := bookFixture(t, nil,
		"BEGIN:VCARD\nVERSION:3.0\nUID:png3\nFN:Png Three\nPHOTO;ENCODING=b;TYPE=PNG:"+b64(pngPhoto)+"\nEND:VCARD\n",
		"BEGIN:VCARD\nVERSION:3.0\nUID:jpeg3\nFN:Jpeg Three\nPHOTO;ENCODING=b;TYPE=JPEG:"+b64(jpegPhoto)+"\nEND:VCARD\n",
		"BEGIN:VCARD\nVERSION:4.0\nUID:data4\nFN:Data Four\nPHOTO:data:image/jpeg;base64,"+b64(jpegPhoto)+"\nEND:VCARD\n",
		"BEGIN:VCARD\nVERSION:3.0\nUID:tiff\nFN:Tiff\nPHOTO;ENCODING=b;TYPE=TIFF:"+b64(pngPhoto)+"\nEND:VCARD\n",
		"BEGIN:VCARD\nVERSION:3.0\nUID:liar\nFN:Liar\nPHOTO;ENCODING=b;TYPE=JPEG:"+b64(pngPhoto)+"\nEND:VCARD\n",
		"BEGIN:VCARD\nVERSION:4.0\nUID:none\nFN:None\nEND:VCARD\n",
	)
	list := decode[view.ContactsListPage](t, f.h.do(get(f.path("contacts"), f.sess)))
	avatars := map[string]string{}
	for _, g := range list.Groups {
		for _, row := range g.Rows {
			avatars[row.UID] = row.Avatar.Href
		}
	}
	for _, uid := range []string{"png3", "jpeg3", "data4"} {
		if !strings.Contains(avatars[uid], "/"+uid+"/photo?size=thumb&v=") {
			t.Errorf("%s: avatar %q, want the thumbnail of its version", uid, avatars[uid])
		}
	}
	for _, uid := range []string{"tiff", "liar", "none"} {
		if avatars[uid] != "" {
			t.Errorf("%s: avatar %q, want initials", uid, avatars[uid])
		}
	}

	reports := f.srv.CountRequests("REPORT", f.col)
	rec, thumb := fetchImage(t, f, avatars["data4"])
	if n := f.srv.CountRequests("REPORT", f.col) - reports; n != 0 {
		t.Errorf("a thumbnail asked the server %d times; the list already read the card", n)
	}
	h := rec.Header()
	if h.Get("Content-Type") != "image/jpeg" || h.Get("Cache-Control") != "private, no-cache" || h.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("headers: %v", h)
	}
	if b := thumb.Bounds(); b.Dx() != 96 || b.Dy() != 96 {
		t.Errorf("thumbnail %v, want CARREL_PHOTO_THUMB_SIDE", b)
	}
	etag := h.Get("ETag")
	if !strings.HasSuffix(etag, `-thumb"`) {
		t.Errorf("ETag %q", etag)
	}
	again := get(avatars["data4"], f.sess)
	again.Header.Set("If-None-Match", etag)
	if rec := f.h.do(again); rec.Code != http.StatusNotModified || rec.Body.Len() != 0 {
		t.Errorf("revalidation: %d with %d bytes, want 304", rec.Code, rec.Body.Len())
	}

	card := decode[view.ContactPage](t, f.h.do(get(f.path("contacts")+"/png3", f.sess)))
	if card.Avatar.Href == "" || card.PhotoHref == "" {
		t.Fatalf("card: avatar %q, photo %q", card.Avatar.Href, card.PhotoHref)
	}
	rec = f.h.do(get(card.PhotoHref, f.sess))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" || !bytes.Equal(rec.Body.Bytes(), pngPhoto) {
		t.Errorf("TYPE=PNG: %d %q, same bytes %v", rec.Code, rec.Header().Get("Content-Type"), bytes.Equal(rec.Body.Bytes(), pngPhoto))
	}
	card = decode[view.ContactPage](t, f.h.do(get(f.path("contacts")+"/jpeg3", f.sess)))
	if rec := f.h.do(get(card.PhotoHref, f.sess)); rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("TYPE=JPEG is served as %q, want image/jpeg", rec.Header().Get("Content-Type"))
	}

	for _, uid := range []string{"tiff", "liar", "none", "nobody"} {
		href := "/c/" + f.id + "/" + url.PathEscape(f.col) + "/" + uid + "/photo"
		if rec := f.h.do(get(href, f.sess)); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", uid, rec.Code)
		}
	}
	if rec := f.h.do(get("/c/a-foreign/"+url.PathEscape(f.col)+"/png3/photo", f.sess)); rec.Code != http.StatusNotFound {
		t.Errorf("a foreign connection: %d, want 404", rec.Code)
	}
}

// A phone photo is uploaded, framed and saved. The orientation the camera
// wrote is applied before the frame, the quarter turn chosen on the server
// after it; what reaches the card is a square JPEG with no metadata, in the
// card's own version, and nothing else of the card changes.
func TestPhotoUploadFrameAndSave(t *testing.T) {
	f := bookFixture(t, nil, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nX-CUSTOM:keep\nEND:VCARD\n")
	rec := f.path("contacts") + "/ada"
	edit := decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
	if !edit.Photo.Editable || edit.Photo.Note != "JPEG, PNG, GIF or WebP." || edit.Photo.Href != "" {
		t.Errorf("photo block without a photo: %+v", edit.Photo)
	}
	add := actionHref(t, edit.Photo.Actions, "photo.add")

	// Stored landscape, red on the left; orientation 6 says it is seen turned
	// clockwise, red on top.
	upload := withOrientation(jpegOf(t, halvesImage(800, 600)), 6)
	resp := f.h.do(uploadRequest(add, f.sess, false, "IMG_0001.jpg", upload))
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != rec+"/photo-preview" {
		t.Fatalf("upload: %d %s %s", resp.Code, resp.Header().Get("Location"), resp.Body.String())
	}
	if n := f.srv.CountRequests("PUT", f.col); n != 0 {
		t.Fatalf("the upload wrote %d times; nothing is written before Save photo", n)
	}

	page := decode[view.PhotoPreviewPage](t, f.h.do(get(rec+"/photo-preview", f.sess)))
	if page.Zoom != 100 || page.OffsetX != 0 || page.OffsetY != 0 || page.Rotate != 0 || page.Title != "Photo" {
		t.Errorf("default frame: %+v", page)
	}
	if hasAction(page.Actions, "photo.remove") {
		t.Error("Remove photo offered for a card that keeps none")
	}
	_, img := fetchImage(t, f, page.Href)
	if b := img.Bounds(); b.Dx() != 512 || b.Dy() != 512 {
		t.Errorf("preview %v, want CARREL_PHOTO_MAX_SIDE", b)
	}
	if !reddish(img.At(256, 60)) || !bluish(img.At(256, 450)) {
		t.Errorf("orientation not applied: top %v, bottom %v", img.At(256, 60), img.At(256, 450))
	}

	// The form sends its fields along with a turn; the turn's own value wins.
	turn := actionHref(t, page.Actions, "photo.rotate.90")
	resp = f.h.do(postForm(turn, f.sess, url.Values{"zoom": {"100"}, "offsetX": {"0"}, "offsetY": {"0"}, "rotate": {"0"}}))
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("turn: %d %s", resp.Code, resp.Body.String())
	}
	page = decode[view.PhotoPreviewPage](t, f.h.do(get(rec+"/photo-preview", f.sess)))
	if a := actionByID(page.Actions, "photo.rotate.90"); page.Rotate != 90 || a == nil || !a.On || !a.Enabled {
		t.Errorf("after the turn: rotate %d, actions %+v", page.Rotate, page.Actions)
	}
	_, img = fetchImage(t, f, page.Href)
	if !reddish(img.At(450, 256)) || !bluish(img.At(60, 256)) {
		t.Errorf("quarter turn not applied: right %v, left %v", img.At(450, 256), img.At(60, 256))
	}

	save := actionHref(t, page.Actions, "photo.save")
	resp = f.h.do(postForm(save, f.sess, url.Values{"zoom": {"100"}, "offsetX": {"0"}, "offsetY": {"0"}, "rotate": {"90"}}))
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != rec+"/edit" {
		t.Fatalf("save: %d %s %s", resp.Code, resp.Header().Get("Location"), resp.Body.String())
	}
	doc := body(t, f, "ada.vcf")
	mustContain(t, doc, "VERSION:4.0", "UID:ada", "FN:Ada Lovelace", "X-CUSTOM:keep", "PHOTO:data:image/jpeg;base64,")
	if strings.Contains(doc, `base64\,`) {
		t.Errorf("the data URI's comma went out escaped:\n%.300s", doc)
	}
	written := photoBytes(t, doc)
	if bytes.Contains(written, []byte("Exif")) || bytes.Contains(written, []byte("GPS")) {
		t.Error("the upload's metadata reached the card")
	}
	img, err := jpeg.Decode(bytes.NewReader(written))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 512 || b.Dy() != 512 || !reddish(img.At(450, 256)) {
		t.Errorf("written photo %v, right %v", b, img.At(450, 256))
	}

	if resp := f.h.do(get(rec+"/photo-preview", f.sess)); resp.Code != http.StatusSeeOther {
		t.Errorf("the draft outlived Save photo: %d", resp.Code)
	}
	edit = decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
	for _, id := range []string{"photo.replace", "photo.crop", "photo.remove"} {
		if !hasAction(edit.Photo.Actions, id) {
			t.Errorf("edit page with a photo has no %s: %+v", id, edit.Photo.Actions)
		}
	}
	if edit.Photo.Href == "" || hasAction(edit.Photo.Actions, "photo.add") {
		t.Errorf("edit page with a photo: %+v", edit.Photo)
	}
}

// S-13: a server that cuts a 4.0 PHOTO data: URI (Radicale) keeps no image.
// The loss report says so, names the risk and offers the older form; the
// frame waits in the draft; the older form goes into the same card version,
// and the draft goes with it. A 3.0 card already writes that form: its loss
// report offers nothing.
func TestPhotoOlderFormAfterACutDataURI(t *testing.T) {
	f := bookFixture(t, nil, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nEND:VCARD\n")
	f.srv.CutPhotoDataURI = true
	rec := f.path("contacts") + "/ada"
	add := actionHref(t, decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Photo.Actions, "photo.add")
	if resp := f.h.do(uploadRequest(add, f.sess, false, "a.jpg", jpegOf(t, halvesImage(600, 600)))); resp.Code != http.StatusSeeOther {
		t.Fatalf("upload: %d %s", resp.Code, resp.Body.String())
	}
	preview := decode[view.PhotoPreviewPage](t, f.h.do(get(rec+"/photo-preview", f.sess)))
	saved := decode[view.ContactEditPage](t, f.h.do(postForm(actionHref(t, preview.Actions, "photo.save"), f.sess, url.Values{})))
	if saved.Loss == nil || len(saved.Loss.Properties) != 1 || saved.Loss.Properties[0] != "PHOTO" {
		t.Fatalf("loss = %+v", saved.Loss)
	}
	if !hasNotice(saved.Notices, "The server did not keep the photo") {
		t.Errorf("the risk is not named: %+v", saved.Notices)
	}
	older := actionHref(t, saved.Loss.Actions, "photo.save.legacy")
	if resp := f.h.do(get(rec+"/photo-preview", f.sess)); resp.Code != http.StatusOK {
		t.Fatalf("the frame did not wait in the draft: %d", resp.Code)
	}

	resp := f.h.do(postForm(older, f.sess, url.Values{}))
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != rec+"/edit" {
		t.Fatalf("older form: %d %s %s", resp.Code, resp.Header().Get("Location"), resp.Body.String())
	}
	doc := body(t, f, "ada.vcf")
	mustContain(t, doc, "VERSION:4.0", "FN:Ada Lovelace", "PHOTO;ENCODING=b;TYPE=JPEG:")
	img, err := jpeg.Decode(bytes.NewReader(photoBytes(t, doc)))
	if err != nil || img.Bounds().Dx() != 512 {
		t.Errorf("older form photo: %v", err)
	}
	if resp := f.h.do(get(rec+"/photo-preview", f.sess)); resp.Code != http.StatusSeeOther {
		t.Errorf("the draft outlived the older form: %d", resp.Code)
	}

	g := bookFixture(t, nil, "BEGIN:VCARD\nVERSION:3.0\nUID:bob\nFN:Bob\nEND:VCARD\n")
	g.srv.Drop = []string{"PHOTO"}
	bob := g.path("contacts") + "/bob"
	add = actionHref(t, decode[view.ContactEditPage](t, g.h.do(get(bob+"/edit", g.sess))).Photo.Actions, "photo.add")
	if resp := g.h.do(uploadRequest(add, g.sess, false, "b.jpg", jpegOf(t, halvesImage(600, 600)))); resp.Code != http.StatusSeeOther {
		t.Fatalf("3.0 upload: %d %s", resp.Code, resp.Body.String())
	}
	preview = decode[view.PhotoPreviewPage](t, g.h.do(get(bob+"/photo-preview", g.sess)))
	saved = decode[view.ContactEditPage](t, g.h.do(postForm(actionHref(t, preview.Actions, "photo.save"), g.sess, url.Values{})))
	if saved.Loss == nil || len(saved.Loss.Actions) != 0 {
		t.Errorf("3.0 card losing PHOTO: loss %+v", saved.Loss)
	}
	if resp := g.h.do(get(bob+"/photo-preview", g.sess)); resp.Code != http.StatusSeeOther {
		t.Errorf("a loss without an offer kept the draft: %d", resp.Code)
	}
}

func actionEnabled(actions []view.Action, id string) bool {
	for _, a := range actions {
		if a.ID == id {
			return a.Enabled
		}
	}
	return false
}

// The photo route's body cap is C-12's, not the 1 MiB of a form, whichever
// way the token comes. Over the photo cap, the file read as it arrives is
// refused on the screen; a body parsed for its token is refused before the
// handler.
func TestPhotoUploadCaps(t *testing.T) {
	big := image.NewRGBA(image.Rect(0, 0, 800, 800))
	rng := rand.New(rand.NewPCG(3, 4))
	for i := range big.Pix {
		big.Pix[i] = byte(rng.UintN(256))
	}
	large := pngOf(t, big)
	if len(large) <= 1<<20 {
		t.Fatalf("the test file is %d bytes, not over a form's 1 MiB", len(large))
	}
	card := "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada\nEND:VCARD\n"

	f := bookFixture(t, nil, card)
	rec := f.path("contacts") + "/ada"
	add := actionHref(t, decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Photo.Actions, "photo.add")
	for _, header := range []bool{true, false} {
		if resp := f.h.do(uploadRequest(add, f.sess, header, "noise.png", large)); resp.Code != http.StatusSeeOther {
			t.Errorf("token in header %v: %d %.200s", header, resp.Code, resp.Body.String())
		}
	}

	g := bookFixture(t, func(h *harness) { h.cfg.PhotoMaxUploadBytes = 64 << 10 }, card)
	rec = g.path("contacts") + "/ada"
	add = actionHref(t, decode[view.ContactEditPage](t, g.h.do(get(rec+"/edit", g.sess))).Photo.Actions, "photo.add")
	page := decode[view.ContactEditPage](t, g.h.do(uploadRequest(add, g.sess, true, "noise.png", large)))
	if !hasNotice(page.Notices, "The photo is too large") {
		t.Errorf("over the photo cap, streamed: %+v", page.Notices)
	}
	if resp := g.h.do(uploadRequest(add, g.sess, false, "noise.png", large)); resp.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("over the photo cap, token in the body: %d, want 413", resp.Code)
	}
	if resp := g.h.do(get(rec+"/photo-preview", g.sess)); resp.Code != http.StatusSeeOther {
		t.Errorf("a refused upload left a draft: %d", resp.Code)
	}
}

// A refused upload is a notice on the edit page. Nothing is written and no
// draft is left.
func TestPhotoRefusalsLeaveNothing(t *testing.T) {
	f := bookFixture(t, func(h *harness) { h.cfg.PhotoMaxPixels = 10_000 },
		"BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada\nEND:VCARD\n")
	rec := f.path("contacts") + "/ada"
	add := actionHref(t, decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Photo.Actions, "photo.add")
	cases := []struct {
		name, file string
		data       []byte
		notice     string
	}{
		{"too many pixels", "wide.jpg", jpegOf(t, halvesImage(200, 100)), "The photo has too many pixels"},
		{"not an image", "drawing.svg", []byte("<svg xmlns='http://www.w3.org/2000/svg'/>"), "This file is not a photo Carrel can use"},
		{"no file", "", nil, "No photo was chosen"},
	}
	for _, tc := range cases {
		page := decode[view.ContactEditPage](t, f.h.do(uploadRequest(add, f.sess, false, tc.file, tc.data)))
		if !hasNotice(page.Notices, tc.notice) {
			t.Errorf("%s: notices %+v", tc.name, page.Notices)
		}
		if resp := f.h.do(get(rec+"/photo-preview", f.sess)); resp.Code != http.StatusSeeOther {
			t.Errorf("%s: a draft was left", tc.name)
		}
	}
	page := decode[view.ContactEditPage](t, f.h.do(uploadRequest(rec+"/photo-preview?op=upload", f.sess, false, "small.jpg", jpegOf(t, halvesImage(80, 60)))))
	if !hasNotice(page.Notices, "Nothing was saved") {
		t.Errorf("an upload without the version: %+v", page.Notices)
	}
	if n := f.srv.CountRequests("PUT", f.col); n != 0 {
		t.Errorf("refusals wrote %d times", n)
	}
}

// S-8: a photo chosen for one version is not written over another. The
// draft stays, now at the version the server holds, and saving again is the
// person's choice.
func TestPhotoSaveOnAMovedCard(t *testing.T) {
	f := bookFixture(t, nil, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada\nEND:VCARD\n")
	rec := f.path("contacts") + "/ada"
	add := actionHref(t, decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Photo.Actions, "photo.add")
	if resp := f.h.do(uploadRequest(add, f.sess, false, "p.jpg", jpegOf(t, halvesImage(300, 200)))); resp.Code != http.StatusSeeOther {
		t.Fatalf("upload: %d", resp.Code)
	}
	f.srv.Seed(f.col+"ada.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada King\nEND:VCARD\n")

	page := decode[view.PhotoPreviewPage](t, f.h.do(postForm(rec+"/photo-preview?op=save", f.sess, url.Values{})))
	if !hasNotice(page.Notices, "Changed elsewhere, so the photo was not saved") {
		t.Errorf("notices %+v", page.Notices)
	}
	if strings.Contains(body(t, f, "ada.vcf"), "PHOTO") {
		t.Fatal("the photo was written over the other client's version")
	}
	resp := f.h.do(postForm(actionHref(t, page.Actions, "photo.save"), f.sess, url.Values{}))
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("second save: %d %s", resp.Code, resp.Body.String())
	}
	mustContain(t, body(t, f, "ada.vcf"), "FN:Ada King", "PHOTO:data:image/jpeg;base64,")
}

// A 3.0 card stays 3.0: its photo is framed again and written back with
// ENCODING and TYPE. Remove takes the property away, and only from the
// version the page showed.
func TestVCard3PhotoCropAndRemove(t *testing.T) {
	f := bookFixture(t, nil, "BEGIN:VCARD\nVERSION:3.0\nUID:grace\nFN:Grace Hopper\nPHOTO;ENCODING=b;TYPE=PNG:"+b64(pngOf(t, halvesImage(200, 100)))+"\nEND:VCARD\n")
	rec := f.path("contacts") + "/grace"
	first := decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
	if first.Photo.Href == "" {
		t.Fatalf("photo block: %+v", first.Photo)
	}
	resp := f.h.do(postForm(actionHref(t, first.Photo.Actions, "photo.crop"), f.sess, url.Values{}))
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != rec+"/photo-preview" {
		t.Fatalf("crop: %d %s", resp.Code, resp.Header().Get("Location"))
	}
	page := decode[view.PhotoPreviewPage](t, f.h.do(get(rec+"/photo-preview", f.sess)))
	if !hasAction(page.Actions, "photo.remove") {
		t.Error("no Remove photo for a card that keeps one")
	}
	_, img := fetchImage(t, f, page.Href)
	if b := img.Bounds(); b.Dx() != 100 || !reddish(img.At(10, 50)) || !bluish(img.At(90, 50)) {
		t.Errorf("a 100 px square is not enlarged and keeps both halves: %v, %v %v", b, img.At(10, 50), img.At(90, 50))
	}

	// All the way left: the red half only.
	resp = f.h.do(postForm(actionHref(t, page.Actions, "photo.save"), f.sess, url.Values{"offsetX": {"-100"}}))
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("save: %d %s", resp.Code, resp.Body.String())
	}
	doc := body(t, f, "grace.vcf")
	mustContain(t, doc, "VERSION:3.0", "PHOTO;ENCODING=b;TYPE=JPEG:")
	if strings.Contains(doc, "TYPE=PNG") {
		t.Errorf("the old photo's parameters stayed:\n%.300s", doc)
	}
	written, err := jpeg.Decode(bytes.NewReader(photoBytes(t, doc)))
	if err != nil {
		t.Fatal(err)
	}
	if !reddish(written.At(90, 50)) {
		t.Errorf("frame on the left edge: right side %v, want red", written.At(90, 50))
	}

	stale := decode[view.ContactEditPage](t, f.h.do(postForm(actionHref(t, first.Photo.Actions, "photo.remove"), f.sess, url.Values{})))
	if !hasNotice(stale.Notices, "Changed elsewhere, so the photo was not removed") {
		t.Errorf("remove from an old version: %+v", stale.Notices)
	}
	if !strings.Contains(body(t, f, "grace.vcf"), "PHOTO") {
		t.Fatal("removed from a version the page did not show")
	}
	current := decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
	resp = f.h.do(postForm(actionHref(t, current.Photo.Actions, "photo.remove"), f.sess, url.Values{}))
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != rec+"/edit" {
		t.Fatalf("remove: %d %s", resp.Code, resp.Body.String())
	}
	doc = body(t, f, "grace.vcf")
	if strings.Contains(doc, "PHOTO") || !strings.Contains(doc, "FN:Grace Hopper") {
		t.Errorf("after remove:\n%s", doc)
	}
}

// A photo the card links to is shown through Carrel, fetched through the
// guard when the browser asks for it, and cannot be edited: no step on the
// edit page, and a step sent anyway is 404.
func TestLinkedPhotoIsProxiedAndNotEdited(t *testing.T) {
	pngPhoto := pngOf(t, halvesImage(120, 120))
	var hits atomic.Int32
	far := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/p.png" {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngPhoto)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html></html>"))
	}))
	defer far.Close()
	f := bookFixture(t, nil,
		"BEGIN:VCARD\nVERSION:3.0\nUID:linked\nFN:Linked\nPHOTO;VALUE=uri:"+far.URL+"/p.png\nEND:VCARD\n",
		"BEGIN:VCARD\nVERSION:4.0\nUID:page\nFN:Page\nPHOTO:"+far.URL+"/page\nEND:VCARD\n",
		"BEGIN:VCARD\nVERSION:4.0\nUID:private\nFN:Private\nPHOTO:http://10.0.0.1/p.png\nEND:VCARD\n",
	)
	list := decode[view.ContactsListPage](t, f.h.do(get(f.path("contacts"), f.sess)))
	if n := hits.Load(); n != 0 {
		t.Errorf("building the list fetched %d linked photos", n)
	}
	var thumb string
	for _, g := range list.Groups {
		for _, row := range g.Rows {
			if row.UID == "linked" {
				thumb = row.Avatar.Href
			}
		}
	}
	rec, img := fetchImage(t, f, thumb)
	if rec.Header().Get("Content-Type") != "image/jpeg" || img.Bounds().Dx() != 96 || hits.Load() != 1 {
		t.Errorf("linked thumbnail: %q %v, %d fetches", rec.Header().Get("Content-Type"), img.Bounds(), hits.Load())
	}

	card := decode[view.ContactPage](t, f.h.do(get(f.path("contacts")+"/linked", f.sess)))
	if card.PhotoNote != "External photo link; editing is not available." {
		t.Errorf("PhotoNote %q", card.PhotoNote)
	}
	rec = f.h.do(get(card.PhotoHref, f.sess))
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" || !bytes.Equal(rec.Body.Bytes(), pngPhoto) || hits.Load() != 1 {
		t.Errorf("linked photo: %d %q, %d fetches", rec.Code, rec.Header().Get("Content-Type"), hits.Load())
	}

	for _, uid := range []string{"page", "private"} {
		href := "/c/" + f.id + "/" + url.PathEscape(f.col) + "/" + uid + "/photo"
		if rec := f.h.do(get(href, f.sess)); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", uid, rec.Code)
		}
	}

	rec2 := f.path("contacts") + "/linked"
	edit := decode[view.ContactEditPage](t, f.h.do(get(rec2+"/edit", f.sess)))
	if edit.Photo.Editable || len(edit.Photo.Actions) != 0 || edit.Photo.Note != "External photo link; editing is not available." || edit.Photo.Href == "" {
		t.Errorf("linked photo block: %+v", edit.Photo)
	}
	etag := url.Values{"etag": {"x"}}.Encode()
	if resp := f.h.do(uploadRequest(rec2+"/photo-preview?op=upload&"+etag, f.sess, false, "p.jpg", jpegOf(t, halvesImage(80, 60)))); resp.Code != http.StatusNotFound {
		t.Errorf("upload over a link: %d, want 404", resp.Code)
	}
	if resp := f.h.do(postForm(rec2+"/photo-preview?op=remove&"+etag, f.sess, url.Values{})); resp.Code != http.StatusNotFound {
		t.Errorf("remove a link: %d, want 404", resp.Code)
	}
	if n := f.srv.CountRequests("PUT", f.col); n != 0 {
		t.Errorf("%d writes", n)
	}
}
