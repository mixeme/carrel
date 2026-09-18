//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"math/rand/v2"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/discover"
	"carrel.dev/carrel/server/view"
)

// S-30.2 for S-13: a photo saved through the request layer is kept by a live
// server the way Carrel sent it — for vCard 4.0 a data URI whose comma is not
// escaped, for 3.0 base64 with ENCODING and TYPE — on one PHOTO line of some
// hundreds of KiB that the server neither folds nor cuts, and it decodes to
// the whole framed JPEG.
//
// Radicale (3.5.4 and 3.8.0, through vobject) cuts a 4.0 PHOTO data URI at its
// comma: the image is gone, and Carrel says so with a loss report. Carrel keeps
// writing RFC 6350 there (owner's decision 2026-09-15); the test holds Radicale
// to that loss, and fails the day Radicale keeps the photo.
func TestRadicalePhoto(t *testing.T) {
	runPhoto(t, radicaleEndpoint(t), radicaleUser, radicalePass, false)
}

func TestBaikalPhoto(t *testing.T) {
	endpoint, user, pass := baikalConfig(t)
	runPhoto(t, endpoint, user, pass, true)
}

// upload sends a file the way a page without scripts does: _csrf first.
func (a *liveApp) upload(path, name string, data []byte) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("_csrf", a.sess.CSRFToken())
	part, _ := w.CreateFormFile("photo", name)
	_, _ = part.Write(data)
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: a.sess.ID()})
	rec := httptest.NewRecorder()
	a.srv.Handler().ServeHTTP(rec, req)
	return rec
}

// uploadFiles sends a set of files the way a page without scripts does: _csrf
// first, then one part per file named by its path inside the chosen folder
// (S-17.1). The map's order does not matter: the service creates the folders
// it needs whichever file comes first.
func (a *liveApp) uploadFiles(path string, files map[string]string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("_csrf", a.sess.CSRFToken())
	for name, body := range files {
		part, _ := w.CreateFormFile("file", name)
		_, _ = part.Write([]byte(body))
	}
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: a.sess.ID()})
	rec := httptest.NewRecorder()
	a.srv.Handler().ServeHTTP(rec, req)
	return rec
}

// rawCard is a card's body as the server answers a GET with this Accept.
func rawCard(ctx context.Context, t *testing.T, endpoint, user, pass, path, accept string) string {
	t.Helper()
	u, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	req.SetBasicAuth(user, pass)
	resp, err := testGuard(t).HTTPClient().Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: %s %v", path, resp.Status, err)
	}
	return string(body)
}

// freshClient is a client with no idle connection. A client kept across the
// seconds a step through the request layer takes sends its next PUT down a
// connection Apache (KeepAliveTimeout 5 s, Baikal) has already closed: the
// PUT fails with EOF, and Go does not send a PUT twice.
func freshClient(t *testing.T, endpoint, user, pass string) *dav.Client {
	t.Helper()
	c, err := dav.NewClient(testGuard(t), endpoint, user, pass, "")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// noiseJPEG does not compress: the framed photo stays large.
func noiseJPEG(t *testing.T, side int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, side, side))
	rng := rand.New(rand.NewPCG(13, 17))
	for y := range side {
		for x := range side {
			img.Set(x, y, color.RGBA{uint8(rng.UintN(256)), uint8(rng.UintN(256)), uint8(rng.UintN(256)), 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func runPhoto(t *testing.T, endpoint, user, pass string, keepsDataURI bool) {
	ctx := context.Background()
	res, trace, err := discover.Run(ctx, testGuard(t), endpoint, user, pass, "")
	if err != nil || res.AddressBookHome == "" {
		t.Fatalf("discovery: %v %+v %+v", err, res, trace)
	}
	suffix := fmt.Sprintf("carrel-s13-%d-%d", os.Getpid(), time.Now().Unix()%100000)
	bookPath := strings.TrimSuffix(res.AddressBookHome, "/") + "/" + suffix + "-book/"
	bookPath = makeCollection(ctx, t, freshClient(t, endpoint, user, pass), endpoint, user, pass, "MKCOL", bookPath, `<?xml version="1.0" encoding="utf-8"?>
<D:mkcol xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
  <D:set><D:prop>
    <D:resourcetype><D:collection/><C:addressbook/></D:resourcetype>
    <D:displayname>Carrel S-13</D:displayname>
  </D:prop></D:set>
</D:mkcol>`)
	t.Cleanup(func() {
		if err := freshClient(t, endpoint, user, pass).Delete(context.Background(), bookPath, ""); err != nil {
			t.Logf("cleanup %s: %v", bookPath, err)
		}
	})

	app := newLiveApp(t)
	redirected(t, "connect", app.post("/app/settings/connections", url.Values{
		"server": {endpoint}, "login": {user}, "password": {pass},
	}))
	key, _ := app.sess.DataKey()
	blob, _ := app.users.Catalog(app.sess.AccountID(), key)
	book := catalogEntry(t, blob, bookPath)
	upload := noiseJPEG(t, 800)

	for _, version := range []string{"4.0", "3.0"} {
		t.Run("vCard "+version, func(t *testing.T) {
			uid := "photo-" + strings.ReplaceAll(version, ".", "") + "-" + suffix
			file := book.Path + uid + ".vcf"
			card := "BEGIN:VCARD\r\nVERSION:" + version + "\r\nUID:" + uid + "\r\nFN:Ada Lovelace\r\nN:Lovelace;Ada;;;\r\nX-CUSTOM:keep-me\r\nEND:VCARD\r\n"
			if _, err := freshClient(t, endpoint, user, pass).PutCreate(ctx, file, dav.Typed(strings.NewReader(card), "text/vcard; charset=utf-8")); err != nil {
				t.Fatalf("seed: %v", err)
			}
			rec := "/app/contacts/" + blob.Connections[0].ID + "/" + url.PathEscape(book.Path) + "/" + url.PathEscape(uid)

			edit := liveDecode[view.ContactEditPage](t, "edit", app.get(rec+"/edit"))
			resp := app.upload(liveAction(t, edit.Photo.Actions, "photo.add"), "noise.jpg", upload)
			if resp.Code != http.StatusSeeOther {
				t.Fatalf("upload: %d %s", resp.Code, resp.Body.String())
			}
			preview := liveDecode[view.PhotoPreviewPage](t, "preview", app.get(rec+"/photo-preview"))
			saved := app.post(liveAction(t, preview.Actions, "photo.save"), url.Values{
				"zoom": {"100"}, "offsetX": {"0"}, "offsetY": {"0"}, "rotate": {"0"},
			})
			if saved.Code != http.StatusSeeOther {
				page := liveDecode[view.ContactEditPage](t, "save", saved)
				var photoLost bool
				if page.Loss != nil {
					for _, p := range page.Loss.Properties {
						photoLost = photoLost || p == "PHOTO"
					}
				}
				if keepsDataURI || !photoLost {
					t.Fatalf("save answered %d with loss %+v and notices %+v", saved.Code, page.Loss, page.Notices)
				}
				held := rawCard(ctx, t, endpoint, user, pass, file, "text/vcard; version=4.0")
				for _, l := range strings.Split(held, "\n") {
					if strings.HasPrefix(l, "PHOTO") && strings.Contains(l, ",") {
						t.Errorf("a loss was reported, yet the server holds a whole PHOTO: %.80q", l)
					}
				}
				t.Logf("seeded as %s: the server cut the PHOTO data URI and Carrel reported the loss", version)

				// The person takes the older form the report offers.
				var older string
				for _, a := range page.Loss.Actions {
					if a.ID == "photo.save.legacy" {
						older = a.Href
					}
				}
				if older == "" {
					t.Fatalf("the loss report offers no older form: %+v", page.Loss)
				}
				redirected(t, "older form", app.post(older, url.Values{}))
				held = rawCard(ctx, t, endpoint, user, pass, file, "text/vcard; version=4.0")
				var line string
				for _, l := range strings.Split(held, "\n") {
					if strings.HasPrefix(l, "PHOTO") {
						line = strings.TrimRight(l, "\r")
					}
				}
				head, value, _ := strings.Cut(line, ":")
				if !strings.Contains(strings.ToUpper(head), "ENCODING=B") || !strings.Contains(strings.ToUpper(head), "TYPE=JPEG") {
					t.Fatalf("older form PHOTO starts %.80q", line)
				}
				data, err := base64.StdEncoding.DecodeString(value)
				if err != nil {
					t.Fatalf("older form PHOTO is not whole base64: %v", err)
				}
				if img, err := jpeg.Decode(bytes.NewReader(data)); err != nil || img.Bounds().Dx() != 512 {
					t.Errorf("older form photo: %v", err)
				}
				mustHave(t, "older form", held, "VERSION:4.0", "X-CUSTOM:keep-me")
				return
			}

			// Carrel reads cards as 4.0 (S-7), so on a converting server a 3.0
			// card comes back 4.0 after the save; a server that keeps cards as
			// sent keeps the version Carrel read. Ask for 4.0: sabre/dav answers
			// a card it holds in 4.0 as held, Radicale any card as held. A
			// converted answer would not pass the checks below — its PHOTO is
			// folded or in another form. (sabre's PRODID proves nothing here: a
			// card Carrel once read converted keeps the PRODID as foreign.)
			raw := []byte(rawCard(ctx, t, endpoint, user, pass, file, "text/vcard; version=4.0"))
			var line, stored string
			for _, l := range strings.Split(string(raw), "\n") {
				l = strings.TrimRight(l, "\r")
				switch {
				case strings.HasPrefix(l, "PHOTO"):
					line = l
				case strings.HasPrefix(l, "VERSION:"):
					stored = strings.TrimPrefix(l, "VERSION:")
				}
			}
			t.Logf("seeded as %s, held as %s after the save", version, stored)
			head, value, _ := strings.Cut(line, ":")
			switch stored {
			case "4.0":
				if head != "PHOTO" || !strings.HasPrefix(value, "data:image/jpeg;base64,") {
					t.Fatalf("4.0 PHOTO starts %.80q", line)
				}
				_, value, _ = strings.Cut(value, ",")
			case "3.0":
				if !strings.Contains(strings.ToUpper(head), "ENCODING=B") || !strings.Contains(strings.ToUpper(head), "TYPE=JPEG") {
					t.Fatalf("3.0 PHOTO starts %.80q", line)
				}
			default:
				t.Fatalf("VERSION %q after the save", stored)
			}
			if len(line) < 100<<10 {
				t.Fatalf("PHOTO line is %d bytes: the server folded or cut it, or the photo is too small to tell", len(line))
			}
			data, err := base64.StdEncoding.DecodeString(value)
			if err != nil {
				t.Fatalf("PHOTO is not whole base64 (%d bytes): %v", len(value), err)
			}
			img, err := jpeg.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("PHOTO is not a whole JPEG: %v", err)
			}
			if b := img.Bounds(); b.Dx() != 512 || b.Dy() != 512 {
				t.Errorf("written photo %v, want CARREL_PHOTO_MAX_SIDE", b)
			}
			mustHave(t, "save", string(raw), "X-CUSTOM:keep-me", "UID:"+uid)

			edit = liveDecode[view.ContactEditPage](t, "edit after save", app.get(rec+"/edit"))
			if edit.Photo.Href == "" {
				t.Fatalf("edit page after save has no photo: %+v", edit.Photo)
			}
			served := app.get(edit.Photo.Href)
			if served.Code != http.StatusOK || served.Header().Get("Content-Type") != "image/jpeg" || !bytes.Equal(served.Body.Bytes(), data) {
				t.Errorf("photo route: %d %q, %d bytes, same as kept %v", served.Code, served.Header().Get("Content-Type"), served.Body.Len(), bytes.Equal(served.Body.Bytes(), data))
			}
		})
	}
}
