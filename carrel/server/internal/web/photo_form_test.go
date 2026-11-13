package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Б9 of the audit of 11.10.2026: the photo buttons of a card's edit form send
// the form itself, as the new form's do (Action.Submits, s-contacts-3). What
// was typed rides with the step and comes back with the form — after Save
// photo, after Cancel, after Remove — and none of it is written by the step:
// the step writes the photo, Save of the form writes the typing.
func TestCardPhotoKeepsWhatWasTyped(t *testing.T) {
	f := bookFixture(t, nil, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nORG:Analytical Engines\nEND:VCARD\n")
	rec := f.path("contacts") + "/ada"
	edit := decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
	add := actionByID(edit.Photo.Actions, "photo.add")
	if add == nil || !add.Submits || add.File != "photo" || add.Method != "POST" {
		t.Fatalf("Add of the edit form = %+v, want it to send the form with the file", add)
	}
	typed := url.Values{"fn": {"Ada King"}, "org": {"Analytical Engines"}, "title": {"Countess"}}

	// Add, then Cancel of the frame: the typing comes back, nothing written.
	resp := f.h.do(formWithPhoto(t, f, add.Href, false, typed, jpegOf(t, halvesImage(400, 400))))
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != rec+"/photo-preview" {
		t.Fatalf("upload: %d %s %s", resp.Code, resp.Header().Get("Location"), resp.Body.String())
	}
	page := decode[view.PhotoPreviewPage](t, f.h.do(get(rec+"/photo-preview", f.sess)))
	// Cancel is a link to the form (s-contacts-5).
	form := decode[view.ContactEditPage](t, f.h.do(get(actionHref(t, page.Actions, "photo.cancel"), f.sess)))
	if formValue(t, form, "fn") != "Ada King" || formValue(t, form, "title") != "Countess" || form.Photo.Href != "" {
		t.Errorf("after Cancel: fn %q, title %q, photo %q", formValue(t, form, "fn"), formValue(t, form, "title"), form.Photo.Href)
	}
	if n := f.srv.CountRequests("PUT", f.col); n != 0 {
		t.Fatalf("a cancelled photo wrote %d times", n)
	}
	// The typing is shown once: the form opened again is the card's.
	form = decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
	if formValue(t, form, "fn") != "Ada Lovelace" {
		t.Errorf("the form opened again shows %q, want the card's name", formValue(t, form, "fn"))
	}

	// Add, then Save photo: the photo is written, the typing is not — it is
	// back in the form, which now edits the version the photo left.
	for _, header := range []bool{false, true} {
		edit = decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
		upload := edit.Photo.Actions[0] // Add, or Replace on the second round
		if !upload.Submits || upload.File != "photo" {
			t.Fatalf("the upload of the edit form = %+v", upload)
		}
		resp = f.h.do(formWithPhoto(t, f, upload.Href, header, typed, jpegOf(t, halvesImage(400, 400))))
		if resp.Code != http.StatusSeeOther {
			t.Fatalf("upload (header %v): %d %s", header, resp.Code, resp.Body.String())
		}
		page = decode[view.PhotoPreviewPage](t, f.h.do(get(rec+"/photo-preview", f.sess)))
		resp = f.h.do(postForm(actionHref(t, page.Actions, "photo.save"), f.sess, url.Values{"zoom": {"100"}, "x": {"0"}, "y": {"0"}, "rotate": {"0"}}))
		if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != rec+"/edit" {
			t.Fatalf("save photo (header %v): %d %s %s", header, resp.Code, resp.Header().Get("Location"), resp.Body.String())
		}
		doc := body(t, f, "ada.vcf")
		mustContain(t, doc, "FN:Ada Lovelace", "PHOTO:data:image/jpeg;base64,")
		if strings.Contains(doc, "Countess") {
			t.Fatalf("Save photo wrote what was typed into the form:\n%.300s", doc)
		}
		_, etag, _ := f.srv.Object(f.col + "ada.vcf")
		form = decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
		if formValue(t, form, "fn") != "Ada King" || formValue(t, form, "title") != "Countess" || form.Photo.Href == "" {
			t.Errorf("after Save photo (header %v): fn %q, title %q, photo %q", header, formValue(t, form, "fn"), formValue(t, form, "title"), form.Photo.Href)
		}
		if got := formValue(t, form, "etag"); got != etag {
			t.Errorf("after Save photo (header %v) the form edits version %q, the card is at %q", header, got, etag)
		}
	}

	// Crop and Remove send the form too. Remove takes the photo away and
	// leaves the typing.
	edit = decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
	crop, remove := actionByID(edit.Photo.Actions, "photo.crop"), actionByID(edit.Photo.Actions, "photo.remove")
	if crop == nil || !crop.Submits || remove == nil || !remove.Submits {
		t.Fatalf("Crop %+v and Remove %+v of the edit form: want both to send the form", crop, remove)
	}
	resp = f.h.do(formWithPhoto(t, f, crop.Href, false, typed, nil))
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != rec+"/photo-preview" {
		t.Fatalf("crop: %d %s", resp.Code, resp.Header().Get("Location"))
	}
	page = decode[view.PhotoPreviewPage](t, f.h.do(get(rec+"/photo-preview", f.sess)))
	form = decode[view.ContactEditPage](t, f.h.do(get(actionHref(t, page.Actions, "photo.cancel"), f.sess)))
	if formValue(t, form, "fn") != "Ada King" || form.Photo.Href == "" {
		t.Errorf("after Crop and Cancel: fn %q, photo %q", formValue(t, form, "fn"), form.Photo.Href)
	}
	remove = actionByID(form.Photo.Actions, "photo.remove")
	resp = f.h.do(formWithPhoto(t, f, remove.Href, false, typed, nil))
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("remove: %d %s", resp.Code, resp.Body.String())
	}
	if doc := body(t, f, "ada.vcf"); strings.Contains(doc, "PHOTO") || strings.Contains(doc, "Countess") || !strings.Contains(doc, "FN:Ada Lovelace") {
		t.Errorf("Remove left\n%.300s", doc)
	}
	form = decode[view.ContactEditPage](t, f.h.do(get(resp.Header().Get("Location"), f.sess)))
	if formValue(t, form, "fn") != "Ada King" || formValue(t, form, "title") != "Countess" || form.Photo.Href != "" {
		t.Errorf("after Remove: fn %q, title %q, photo %q", formValue(t, form, "fn"), formValue(t, form, "title"), form.Photo.Href)
	}
}

// The typing a photo step brings back was made on the version the form was
// opened at. A card changed elsewhere in between is not that version: the
// form comes back naming the old one, so its Save meets the conflict screen
// and does not put the typing over the other change unseen.
func TestCardPhotoTypingKeepsItsVersion(t *testing.T) {
	f := bookFixture(t, nil, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nEND:VCARD\n")
	rec := f.path("contacts") + "/ada"
	edit := decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
	opened := formValue(t, edit, "etag")
	typed := url.Values{"fn": {"Ada King"}}
	resp := f.h.do(formWithPhoto(t, f, actionHref(t, edit.Photo.Actions, "photo.add"), false, typed, jpegOf(t, halvesImage(400, 400))))
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("upload: %d %s", resp.Code, resp.Body.String())
	}
	f.srv.Seed(f.col+"ada.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nNOTE:changed elsewhere\nEND:VCARD\n")
	frame := url.Values{"zoom": {"100"}, "x": {"0"}, "y": {"0"}, "rotate": {"0"}}
	page := decode[view.PhotoPreviewPage](t, f.h.do(get(rec+"/photo-preview", f.sess)))
	again := decode[view.PhotoPreviewPage](t, f.h.do(postForm(actionHref(t, page.Actions, "photo.save"), f.sess, frame)))
	if len(again.Notices) == 0 {
		t.Fatalf("a photo saved over a changed card said nothing: %+v", again)
	}
	resp = f.h.do(postForm(actionHref(t, again.Actions, "photo.save"), f.sess, frame))
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("save again: %d %s", resp.Code, resp.Body.String())
	}
	form := decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
	if formValue(t, form, "fn") != "Ada King" {
		t.Fatalf("the typing did not come back: %q", formValue(t, form, "fn"))
	}
	if got := formValue(t, form, "etag"); got != opened {
		t.Errorf("the form names version %q, want the one the typing was made at, %q", got, opened)
	}
}

// Б9, its rest: Save photo on a server that does not keep the photo answers
// with the loss report over the edit form. The form under it showed the
// card as the server holds it, the typing waiting for the next open of the
// form — a Save under the report wrote the server's card and left the typing
// for later. Now the form under the report is the typing, as after a photo
// step that lost nothing, at the version the photo left.
func TestTheLossReportOfAPhotoKeepsWhatWasTyped(t *testing.T) {
	f := bookFixture(t, nil, "BEGIN:VCARD\nVERSION:3.0\nUID:ada\nFN:Ada Lovelace\nORG:Analytical Engines\nEND:VCARD\n")
	f.srv.Drop = []string{"PHOTO"}
	rec := f.path("contacts") + "/ada"
	edit := decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
	typed := url.Values{"fn": {"Ada King"}, "org": {"Analytical Engines"}, "title": {"Countess"}}
	resp := f.h.do(formWithPhoto(t, f, actionHref(t, edit.Photo.Actions, "photo.add"), false, typed, jpegOf(t, halvesImage(400, 400))))
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("upload: %d %s", resp.Code, resp.Body.String())
	}
	page := decode[view.PhotoPreviewPage](t, f.h.do(get(rec+"/photo-preview", f.sess)))
	saved := decode[view.ContactEditPage](t, f.h.do(postForm(actionHref(t, page.Actions, "photo.save"), f.sess, cropForm(page))))
	if saved.Loss == nil || len(saved.Loss.Properties) != 1 || saved.Loss.Properties[0] != "PHOTO" {
		t.Fatalf("loss = %+v", saved.Loss)
	}
	if formValue(t, saved, "fn") != "Ada King" || formValue(t, saved, "title") != "Countess" {
		t.Errorf("the form under the report: fn %q, title %q, want the typing", formValue(t, saved, "fn"), formValue(t, saved, "title"))
	}
	_, etag, _ := f.srv.Object(f.col + "ada.vcf")
	if got := formValue(t, saved, "etag"); got != etag {
		t.Errorf("the form under the report edits version %q, the card is at %q", got, etag)
	}
	if strings.Contains(body(t, f, "ada.vcf"), "Countess") {
		t.Error("Save photo wrote the typing")
	}
	// Shown once: the form opened again is the card's.
	if again := decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess))); formValue(t, again, "fn") != "Ada Lovelace" {
		t.Errorf("the form opened again shows %q, want the card's", formValue(t, again, "fn"))
	}
}
