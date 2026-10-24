package web_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// formWithPhoto is the new form sent whole to a photo step, the way its
// photo buttons send it (Action.Submits): the typed fields first, the file
// where the form holds it — the token in the body, or in the header as a
// script sends it.
func formWithPhoto(t *testing.T, f davFixture, href string, header bool, typed url.Values, photo []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if !header {
		_ = w.WriteField("_csrf", f.sess.CSRFToken())
	}
	if photo != nil {
		part, _ := w.CreateFormFile("photo", "IMG_0003.jpg")
		_, _ = part.Write(photo)
	}
	for k, vs := range typed {
		for _, v := range vs {
			_ = w.WriteField(k, v)
		}
	}
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, href, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if header {
		req.Header.Set("X-CSRF-Token", f.sess.CSRFToken())
	}
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: f.sess.ID()})
	return req
}

func formValue(t *testing.T, p view.ContactEditPage, name string) string {
	t.Helper()
	r := formRow(p.Form, name)
	if r == nil {
		t.Fatalf("no row %q in the form", name)
	}
	return string(r.Value)
}

// s-contacts-27 (the lead's decision): Add of the new form sends the whole
// form — its photo buttons are the form's own (Action.Submits) —, so what
// was typed is not lost: the server keeps the typed values with the draft
// under the form's UID, and the form comes back with them after Save photo,
// after Cancel of a first upload and after Remove.
func TestNewContactPhotoKeepsWhatWasTyped(t *testing.T) {
	f := bookFixture(t, nil)
	list := f.path("contacts")
	fresh := decode[view.ContactEditPage](t, f.h.do(get(list+"/new", f.sess)))
	var add view.Action
	for _, a := range fresh.Photo.Actions {
		if a.ID == "photo.add" {
			add = a
		}
	}
	if !add.Submits || add.File != "photo" || add.Method != "POST" {
		t.Fatalf("Add of the new form = %+v, want it to send the form with the file", add)
	}
	typed := url.Values{"fn": {"Ada Lovelace"}, "n.given": {"Ada"}, "n.family": {"Lovelace"}, "org": {"Analytical Engines"}}
	for _, header := range []bool{false, true} {
		resp := f.h.do(formWithPhoto(t, f, add.Href, header, typed, jpegOf(t, halvesImage(400, 400))))
		if resp.Code != http.StatusSeeOther {
			t.Fatalf("upload (header %v): %d %s", header, resp.Code, resp.Body.String())
		}
		preview := resp.Header().Get("Location")
		page := decode[view.PhotoPreviewPage](t, f.h.do(get(preview, f.sess)))
		resp = f.h.do(postForm(actionHref(t, page.Actions, "photo.save"), f.sess, url.Values{"zoom": {"100"}, "x": {"0"}, "y": {"0"}, "rotate": {"0"}}))
		back := resp.Header().Get("Location")
		if !strings.HasPrefix(back, list+"/new?uid=") {
			t.Fatalf("save photo: %d %s", resp.Code, back)
		}
		form := decode[view.ContactEditPage](t, f.h.do(get(back, f.sess)))
		if formValue(t, form, "fn") != "Ada Lovelace" || formValue(t, form, "org") != "Analytical Engines" || form.Photo.Href == "" {
			t.Errorf("after Save photo (header %v): fn %q, org %q, photo %q", header, formValue(t, form, "fn"), formValue(t, form, "org"), form.Photo.Href)
		}

		// Remove, sent with the form as it now stands: the photo goes, the
		// typing stays.
		var remove view.Action
		for _, a := range form.Photo.Actions {
			if a.ID == "photo.remove" {
				remove = a
			}
		}
		if !remove.Submits {
			t.Errorf("Remove of the new form sends no form: %+v", remove)
		}
		again := url.Values{"fn": {"Ada King"}, "n.given": {"Ada"}, "n.family": {"King"}, "org": {"Analytical Engines"}}
		resp = f.h.do(formWithPhoto(t, f, remove.Href, header, again, nil))
		back = resp.Header().Get("Location")
		form = decode[view.ContactEditPage](t, f.h.do(get(back, f.sess)))
		if formValue(t, form, "fn") != "Ada King" || form.Photo.Href != "" || formRow(form.Form, "uid") != nil {
			t.Errorf("after Remove (header %v): %q at %s, photo %q, uid %+v", header, formValue(t, form, "fn"), back, form.Photo.Href, formRow(form.Form, "uid"))
		}
	}

	// Cancel of a first upload sent with the form: the typing comes back.
	resp := f.h.do(formWithPhoto(t, f, add.Href, false, typed, jpegOf(t, halvesImage(400, 400))))
	page := decode[view.PhotoPreviewPage](t, f.h.do(get(resp.Header().Get("Location"), f.sess)))
	resp = f.h.do(postForm(actionHref(t, page.Actions, "photo.cancel"), f.sess, url.Values{}))
	form := decode[view.ContactEditPage](t, f.h.do(get(resp.Header().Get("Location"), f.sess)))
	if formValue(t, form, "fn") != "Ada Lovelace" || form.Photo.Href != "" {
		t.Errorf("after Cancel: fn %q, photo %q", formValue(t, form, "fn"), form.Photo.Href)
	}
	if n := f.srv.CountRequests("PUT", f.col); n != 0 {
		t.Errorf("the photo steps of a new form wrote %d times", n)
	}
}
