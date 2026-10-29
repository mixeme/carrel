package web_test

import (
	"bytes"
	"encoding/base64"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// A browser may place the photo before the name, and a note's file fields
// outside its form. Save must read fields on both sides of every file.
func saveFiles(path string, sess *session.Session, values url.Values, field string, data ...[]byte) *http.Request {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("_csrf", sess.CSRFToken())
	for i, b := range data {
		p, _ := w.CreateFormFile(field, string(rune('a'+i))+".bin")
		_, _ = p.Write(b)
	}
	for k, vs := range values {
		for _, v := range vs {
			_ = w.WriteField(k, v)
		}
	}
	_ = w.Close()
	r := httptest.NewRequest(http.MethodPost, path, &body)
	r.Header.Set("Content-Type", w.FormDataContentType())
	r.AddCookie(&http.Cookie{Name: "carrel_sid", Value: sess.ID()})
	return r
}

func TestFormSaveAttachments(t *testing.T) {
	for _, fresh := range []bool{false, true} {
		name := "edit"
		if fresh {
			name = "new"
		}
		t.Run(name, func(t *testing.T) {
			f := attachFixture(t, 0, nil, "e1.ics", attachEvent("e1", "Before", "X-KEEP:untouched\n"))
			path := f.path("calendar") + "/e1/edit"
			if fresh {
				path = f.path("calendar") + "/new"
			}
			p := decode[view.EventEditPage](t, f.h.do(get(path, f.sess)))
			v := formFields(p.Form)
			v.Set("summary", "With files")
			r := f.h.do(saveFiles(p.Form.Action, f.sess, v, "file", []byte("first"), []byte("second")))
			if r.Code != http.StatusSeeOther {
				t.Fatalf("Save: %d %s", r.Code, r.Body.String())
			}
			got := decode[view.EventPage](t, f.h.do(get(r.Header().Get("Location"), f.sess)))
			if len(got.Attach) != 2 {
				t.Fatalf("Save kept %d files, want two", len(got.Attach))
			}
			if !fresh {
				mustContain(t, body(t, f, "e1.ics"), "SUMMARY:With files", "X-KEEP:untouched", base64.StdEncoding.EncodeToString([]byte("second")))
			}
		})
	}
}

func TestFormSaveFileAlone(t *testing.T) {
	f := attachFixture(t, 0, nil, "e1.ics", attachEvent("e1", "Unchanged", ""))
	p := eventEditAt(t, f, "e1")
	r := f.h.do(saveFiles(p.Form.Action, f.sess, formFields(p.Form), "file", []byte("only change")))
	if r.Code != http.StatusSeeOther {
		t.Fatalf("Save: %d %s", r.Code, r.Body.String())
	}
	if len(eventAt(t, f, "e1").Attach) != 1 {
		t.Fatal("Save without text changes discarded its file")
	}
}

func TestFormSaveLargeAttachmentAndRollback(t *testing.T) {
	for _, valid := range []bool{true, false} {
		name := "valid"
		if !valid {
			name = "invalid"
		}
		t.Run(name, func(t *testing.T) {
			f := attachFixture(t, 2048, []string{"carrel"}, "e1.ics", attachEvent("e1", "Before", ""))
			p := eventEditAt(t, f, "e1")
			v := formFields(p.Form)
			v.Set("summary", "After")
			if !valid {
				v.Set("start.date", "not a date")
			}
			before := body(t, f, "e1.ics")
			r := f.h.do(saveFiles(p.Form.Action, f.sess, v, "file", bytes.Repeat([]byte("file"), 300000)))
			if valid {
				if r.Code != http.StatusSeeOther {
					t.Fatalf("large Save: %d %s", r.Code, r.Body.String())
				}
				p := eventAt(t, f, "e1")
				if len(p.Attach) != 1 || p.Attach[0].Inline {
					t.Fatalf("large file: %+v", p.Attach)
				}
			} else {
				if r.Code != http.StatusOK || !strings.Contains(r.Body.String(), "Choose the files again") {
					t.Fatalf("refusal: %d %s", r.Code, r.Body.String())
				}
				if body(t, f, "e1.ics") != before {
					t.Fatal("invalid form changed the record")
				}
				if names := f.srv.Names(attachFiles + "carrel/"); len(names) != 0 {
					t.Fatalf("failed Save left files: %v", names)
				}
			}
		})
	}
}

func TestFormSavePhoto(t *testing.T) {
	for _, fresh := range []bool{false, true} {
		name := "edit"
		if fresh {
			name = "new"
		}
		t.Run(name, func(t *testing.T) {
			f := bookFixture(t, nil, "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:c1\r\nFN:Before\r\nX-KEEP:untouched\r\nEND:VCARD\r\n")
			path := f.path("contacts") + "/c1/edit"
			if fresh {
				path = f.path("contacts") + "/new"
			}
			p := decode[view.ContactEditPage](t, f.h.do(get(path, f.sess)))
			v := formFields(p.Form)
			v.Set("fn", "With photo")
			r := f.h.do(saveFiles(p.Form.Action, f.sess, v, "photo", withOrientation(jpegOf(t, halvesImage(600, 400)), 6)))
			if r.Code != http.StatusSeeOther {
				t.Fatalf("Save: %d %s", r.Code, r.Body.String())
			}
			edit := decode[view.ContactEditPage](t, f.h.do(get(r.Header().Get("Location")+"/edit", f.sess)))
			if edit.Photo.Href == "" {
				t.Fatal("Save discarded its photo")
			}
			if !fresh {
				mustContain(t, body(t, f, "c1.vcf"), "FN:With photo", "X-KEEP:untouched", "PHOTO;")
			}
		})
	}
}

func TestFormSaveRefusals(t *testing.T) {
	for _, reason := range []string{"conflict", "fields", "csrf"} {
		t.Run(reason, func(t *testing.T) {
			f := attachFixture(t, 2048, []string{"carrel"}, "e1.ics", attachEvent("e1", "Before", ""))
			p := eventEditAt(t, f, "e1")
			v := formFields(p.Form)
			v.Set("summary", "Mine")
			switch reason {
			case "conflict":
				f.srv.Seed(f.col+"e1.ics", attachEvent("e1", "Theirs", ""))
			case "fields":
				v.Set("description", strings.Repeat("x", 1024*1024))
			}
			before := body(t, f, "e1.ics")
			req := saveFiles(p.Form.Action, f.sess, v, "file", bytes.Repeat([]byte("x"), 3000))
			if reason == "csrf" {
				req.Header.Set("X-CSRF-Token", "wrong")
			}
			r := f.h.do(req)
			switch reason {
			case "conflict":
				if r.Code != http.StatusSeeOther || !strings.HasSuffix(r.Header().Get("Location"), "/conflict") {
					t.Fatalf("conflict: %d %s", r.Code, r.Body.String())
				}
				page := f.h.do(get(r.Header().Get("Location"), f.sess))
				if !strings.Contains(page.Body.String(), "Choose the files again") {
					t.Fatal("conflict silently discarded file")
				}
			case "fields":
				if r.Code != http.StatusRequestEntityTooLarge {
					t.Fatalf("fields: %d", r.Code)
				}
			case "csrf":
				if r.Code != http.StatusForbidden {
					t.Fatalf("CSRF: %d", r.Code)
				}
			}
			if body(t, f, "e1.ics") != before {
				t.Fatal("refused Save changed record")
			}
			if names := f.srv.Names(attachFiles + "carrel/"); len(names) != 0 {
				t.Fatalf("refused Save left files: %v", names)
			}
		})
	}
}

func TestFormSaveInvalidPhotoKeepsTyping(t *testing.T) {
	f := bookFixture(t, nil)
	p := decode[view.ContactEditPage](t, f.h.do(get(f.path("contacts")+"/new", f.sess)))
	v := formFields(p.Form)
	v.Set("fn", "Still here")
	r := f.h.do(saveFiles(p.Form.Action, f.sess, v, "photo", []byte("not an image")))
	back := decode[view.ContactEditPage](t, r)
	if formFields(back.Form).Get("fn") != "Still here" || !strings.Contains(r.Body.String(), "Nothing was saved") {
		t.Fatalf("refusal lost typing: %s", r.Body.String())
	}
	if len(f.srv.Paths(f.col)) != 0 {
		t.Fatal("invalid photo created a card")
	}
}

func TestFormSaveNamesLargeFilesAfterTheFinalFields(t *testing.T) {
	for _, names := range []string{sealed.AttachNamesTitled, sealed.AttachNamesDated} {
		t.Run(names, func(t *testing.T) {
			f := attachFixture(t, 2048, []string{"carrel"}, "e1.ics", attachEvent("e1", "Before", ""))
			editBlob(t, f.h, f.sess, func(b *sealed.Blob) { b.AttachNames = names; b.Attachments.Monthly = true })
			p := eventEditAt(t, f, "e1")
			v := formFields(p.Form)
			v.Set("summary", "After")
			v.Set("start.date", "2026-11-23")
			v.Set("end.date", "2026-11-23")
			v.Set("end.time", "11:00")
			r := f.h.do(saveFiles(p.Form.Action, f.sess, v, "file", bytes.Repeat([]byte("x"), 3000)))
			if r.Code != http.StatusSeeOther {
				t.Fatalf("Save: %d %s", r.Code, r.Body.String())
			}
			want := attachFiles + "carrel/2026-11/2026-11-23 a.bin"
			if names == sealed.AttachNamesTitled {
				want = attachFiles + "carrel/2026-11/After - a.bin"
			}
			if data, ok := f.srv.File(want); !ok || len(data) != 3000 {
				t.Errorf("final file missing or damaged: %s", want)
			}
			if left := f.srv.Names(attachFiles + "carrel/2026-08/"); len(left) != 0 {
				t.Errorf("old placement remains: %v", left)
			}
		})
	}
}

func TestFormSaveMultipartWithoutFiles(t *testing.T) {
	f := attachFixture(t, 0, nil, "e1.ics", attachEvent("e1", "Before", ""))
	p := eventEditAt(t, f, "e1")
	v := formFields(p.Form)
	v.Set("summary", "Without a file")
	r := f.h.do(saveFiles(p.Form.Action, f.sess, v, "file"))
	if r.Code != http.StatusSeeOther {
		t.Fatalf("empty multipart Save: %d %s", r.Code, r.Body.String())
	}
	mustContain(t, body(t, f, "e1.ics"), "SUMMARY:Without a file")
}
