package web_test

import (
	"bytes"
	"image/jpeg"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// The full form of a new contact draws its photo as the card's edit does
// (s-contacts-27): the empty square, Add and the formats. The photo is
// chosen before the card exists: the draft lives under the UID of the new
// form until Save, the steps are the card's own (S-13), and nothing is
// written until Save. Save writes the card with the photo, under that UID;
// Cancel of the frame and a form left unsaved write nothing.
func TestNewContactPhoto(t *testing.T) {
	f := bookFixture(t, nil)
	list := f.path("contacts")
	fresh := decode[view.ContactEditPage](t, f.h.do(get(list+"/new", f.sess)))
	if !fresh.Photo.Editable || fresh.Photo.Note != "JPEG, PNG, GIF or WebP." || fresh.Photo.Href != "" {
		t.Fatalf("photo of a new form: %+v", fresh.Photo)
	}
	if fresh.UID != "" || formRow(fresh.Form, "uid") != nil {
		t.Errorf("a new form without a photo carries a UID: %q, %+v", fresh.UID, formRow(fresh.Form, "uid"))
	}
	add := actionHref(t, fresh.Photo.Actions, "photo.add")
	if len(fresh.Photo.Actions) != 1 {
		t.Errorf("new form, photo actions %+v, want Add alone", fresh.Photo.Actions)
	}
	if !strings.HasPrefix(add, list+"/new/photo-preview?") {
		t.Fatalf("Add of a new form posts to %q", add)
	}

	// Cancel of the frame: back to the form, nothing kept, nothing written.
	resp := f.h.do(uploadRequest(add, f.sess, false, "IMG_0001.jpg", jpegOf(t, halvesImage(400, 400))))
	if resp.Code != http.StatusSeeOther || !strings.HasPrefix(resp.Header().Get("Location"), list+"/new/photo-preview?uid=") {
		t.Fatalf("upload: %d %s %s", resp.Code, resp.Header().Get("Location"), resp.Body.String())
	}
	preview := resp.Header().Get("Location")
	page := decode[view.PhotoPreviewPage](t, f.h.do(get(preview, f.sess)))
	if hasAction(page.Actions, "photo.remove") {
		t.Error("Remove photo offered for a card that does not exist yet")
	}
	if len(page.Rail.Nav) == 0 || page.Rail.Nav[0].Label != "Back to the form" || !strings.HasPrefix(page.Rail.Nav[0].Href, list+"/new?uid=") {
		t.Errorf("the frame of a new form goes back to %+v", page.Rail.Nav)
	}
	resp = f.h.do(postForm(actionHref(t, page.Actions, "photo.cancel"), f.sess, url.Values{}))
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != list+"/new" {
		t.Fatalf("cancel: %d %s", resp.Code, resp.Header().Get("Location"))
	}
	if resp := f.h.do(get(preview, f.sess)); resp.Code != http.StatusSeeOther {
		t.Errorf("the draft outlived Cancel: %d", resp.Code)
	}

	// Add, frame, Save photo: the form again, with the photo and its UID.
	resp = f.h.do(uploadRequest(add, f.sess, true, "IMG_0002.jpg", jpegOf(t, halvesImage(800, 600))))
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("upload: %d %s", resp.Code, resp.Body.String())
	}
	preview = resp.Header().Get("Location")
	uid := strings.TrimPrefix(preview, list+"/new/photo-preview?uid=")
	if len(uid) != 32 {
		t.Fatalf("the new form's UID %q", uid)
	}
	page = decode[view.PhotoPreviewPage](t, f.h.do(get(preview, f.sess)))
	resp = f.h.do(postForm(cropStep(page), f.sess, cropForm(page, "rotate", "90")))
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != preview {
		t.Fatalf("turn: %d %s", resp.Code, resp.Header().Get("Location"))
	}
	page = decode[view.PhotoPreviewPage](t, f.h.do(get(preview, f.sess)))
	resp = f.h.do(postForm(actionHref(t, page.Actions, "photo.save"), f.sess, cropForm(page, "rotate", "90")))
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != list+"/new?uid="+uid {
		t.Fatalf("save photo: %d %s %s", resp.Code, resp.Header().Get("Location"), resp.Body.String())
	}
	if n := f.srv.CountRequests("PUT", f.col); n != 0 {
		t.Fatalf("the photo steps of a new form wrote %d times", n)
	}
	form := decode[view.ContactEditPage](t, f.h.do(get(list+"/new?uid="+uid, f.sess)))
	if form.UID != "" {
		t.Errorf("ContactEditPage.UID of a new form = %q; the form carries it", form.UID)
	}
	if row := formRow(form.Form, "uid"); row == nil || row.Kind != view.InputHidden || string(row.Value) != uid {
		t.Errorf("the form with a photo carries uid %+v, want %s", row, uid)
	}
	if form.Photo.Href == "" {
		t.Errorf("the form does not show the chosen photo: %+v", form.Photo)
	}
	for _, id := range []string{"photo.replace", "photo.crop", "photo.remove"} {
		if !hasAction(form.Photo.Actions, id) {
			t.Errorf("new form with a photo has no %s: %+v", id, form.Photo.Actions)
		}
	}
	if hasAction(form.Photo.Actions, "photo.add") {
		t.Errorf("new form with a photo offers Add: %+v", form.Photo.Actions)
	}
	_, img := fetchImage(t, f, form.Photo.Href)
	// Red on the left as uploaded; a quarter turn takes it off the left.
	if reddish(img.At(60, 256)) || !(reddish(img.At(256, 60)) || bluish(img.At(256, 60))) {
		t.Errorf("the form shows the photo unturned: left %v, top %v", img.At(60, 256), img.At(256, 60))
	}

	// Crop again and cancel: the photo stays as it was saved.
	resp = f.h.do(postForm(actionHref(t, form.Photo.Actions, "photo.crop"), f.sess, url.Values{}))
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != preview {
		t.Fatalf("crop: %d %s", resp.Code, resp.Header().Get("Location"))
	}
	page = decode[view.PhotoPreviewPage](t, f.h.do(get(preview, f.sess)))
	if cropField(page, "rotate") != "90" {
		t.Errorf("the frame of a chosen photo starts at %s°, want the saved 90°", cropField(page, "rotate"))
	}
	_ = f.h.do(postForm(cropStep(page), f.sess, cropForm(page, "rotate", "180")))
	resp = f.h.do(postForm(actionHref(t, page.Actions, "photo.cancel"), f.sess, url.Values{}))
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != list+"/new?uid="+uid {
		t.Fatalf("cancel of a crop: %d %s", resp.Code, resp.Header().Get("Location"))
	}
	form = decode[view.ContactEditPage](t, f.h.do(get(list+"/new?uid="+uid, f.sess)))
	if form.Photo.Href == "" {
		t.Fatalf("Cancel of a crop dropped the chosen photo: %+v", form.Photo)
	}

	// Save: the card, under the form's UID, with the photo as framed.
	values := formFields(form.Form)
	values.Set("fn", "Grace Hopper")
	resp = f.h.do(postForm(list+"/new", f.sess, values))
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != list+"/"+uid {
		t.Fatalf("save: %d %s %s", resp.Code, resp.Header().Get("Location"), resp.Body.String())
	}
	doc := body(t, f, uid+".vcf")
	mustContain(t, doc, "UID:"+uid, "FN:Grace Hopper", "PHOTO")
	written, err := jpeg.Decode(bytes.NewReader(photoBytes(t, doc)))
	if err != nil {
		t.Fatal(err)
	}
	if b := written.Bounds(); b.Dx() != 512 || b.Dy() != 512 || reddish(written.At(60, 256)) || !(reddish(written.At(256, 60)) || bluish(written.At(256, 60))) {
		t.Errorf("written photo %v, left %v, top %v: not the frame saved", b, written.At(60, 256), written.At(256, 60))
	}
	if resp := f.h.do(get(preview, f.sess)); resp.Code != http.StatusSeeOther {
		t.Errorf("the draft outlived Save: %d", resp.Code)
	}

	// A form left with a photo chosen writes nothing; Remove takes the
	// photo off the form and keeps nothing.
	puts := f.srv.CountRequests("PUT", f.col)
	resp = f.h.do(uploadRequest(add, f.sess, false, "IMG_0003.jpg", jpegOf(t, halvesImage(300, 300))))
	preview = resp.Header().Get("Location")
	uid = strings.TrimPrefix(preview, list+"/new/photo-preview?uid=")
	page = decode[view.PhotoPreviewPage](t, f.h.do(get(preview, f.sess)))
	_ = f.h.do(postForm(actionHref(t, page.Actions, "photo.save"), f.sess, cropForm(page)))
	form = decode[view.ContactEditPage](t, f.h.do(get(list+"/new?uid="+uid, f.sess)))
	resp = f.h.do(postForm(actionHref(t, form.Photo.Actions, "photo.remove"), f.sess, url.Values{}))
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != list+"/new" {
		t.Fatalf("remove: %d %s", resp.Code, resp.Header().Get("Location"))
	}
	form = decode[view.ContactEditPage](t, f.h.do(get(list+"/new?uid="+uid, f.sess)))
	if form.Photo.Href != "" || !hasAction(form.Photo.Actions, "photo.add") || formRow(form.Form, "uid") != nil {
		t.Errorf("after Remove the form still has the photo: %+v", form.Photo)
	}
	if n := f.srv.CountRequests("PUT", f.col) - puts; n != 0 {
		t.Errorf("a form left unsaved wrote %d times", n)
	}

	// A form that names a photo the session no longer holds (another card's
	// photo took the one slot) saves nothing silently without it.
	values = formFields(form.Form)
	values.Set("fn", "Lost Photo")
	values.Set("uid", uid)
	page2 := decode[view.ContactEditPage](t, f.h.do(postForm(list+"/new", f.sess, values)))
	if !hasNotice(page2.Notices, "Nothing was saved") || formRow(page2.Form, "uid") != nil {
		t.Errorf("a lost photo: %+v", page2.Notices)
	}
	if n := f.srv.CountRequests("PUT", f.col) - puts; n != 0 {
		t.Errorf("a form with a lost photo wrote %d times", n)
	}

	// A UID the server did not issue is not taken from the address.
	if form := decode[view.ContactEditPage](t, f.h.do(get(list+"/new?uid=../x", f.sess))); formRow(form.Form, "uid") != nil {
		t.Errorf("a UID from the address was taken as is: %+v", formRow(form.Form, "uid"))
	}
}

// The upload of a new form keeps the caps of a card's upload (S-13,
// security.md): too large is refused on the form, and no draft is left.
func TestNewContactPhotoRefusal(t *testing.T) {
	f := bookFixture(t, func(h *harness) { h.cfg.PhotoMaxUploadBytes = 64 << 10 })
	list := f.path("contacts")
	add := actionHref(t, decode[view.ContactEditPage](t, f.h.do(get(list+"/new", f.sess))).Photo.Actions, "photo.add")
	big := make([]byte, 80<<10)
	copy(big, jpegOf(t, halvesImage(64, 64)))
	resp := f.h.do(uploadRequest(add, f.sess, true, "big.jpg", big))
	if resp.Code != http.StatusOK {
		t.Fatalf("too large: %d %s", resp.Code, resp.Header().Get("Location"))
	}
	page := decode[view.ContactEditPage](t, resp)
	if !hasNotice(page.Notices, "The photo is too large") || page.Name != "New contact" || page.Photo.Href != "" {
		t.Errorf("refusal of a new form's upload: %+v %+v", page.Notices, page.Photo)
	}
	resp = f.h.do(uploadRequest(add, f.sess, false, "notes.txt", []byte("not a photo")))
	if page := decode[view.ContactEditPage](t, resp); !hasNotice(page.Notices, "This file is not a photo Carrel can use") {
		t.Errorf("a text file: %+v", page.Notices)
	}
}
