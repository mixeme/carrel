package web_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// submitWithFile sends a form the way a browser sends it from a button with
// formaction and a file (Action.Submits, Action.File): every field of the
// form, the file among them.
func submitWithFile(path string, sess *session.Session, form url.Values, field, name string, data []byte) *http.Request {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("_csrf", sess.CSRFToken())
	for k, vs := range form {
		for _, v := range vs {
			_ = w.WriteField(k, v)
		}
	}
	part, _ := w.CreateFormFile(field, name)
	_, _ = part.Write(data)
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: sess.ID()})
	return req
}

// Attach on a new event (s-agenda-15; the lead's decision with task 106, as
// the photo of a new card, v0.2.133): the button sends the form itself with
// the file, the files wait under the UID the event will have, the form comes
// back as it was typed with them; Remove takes one off, From Files picks one
// for this draft; Save creates the event under that UID with the files in
// it. Nothing is written before Save.
func TestNewEventAttachIsTheFrame(t *testing.T) {
	// «From this computer» is the label of its file input (Typecase's task
	// 109), «From Files» a button.
	items := strings.Join(frameMenu(t, "s-agenda-15"), "\n")
	if !strings.Contains(items, "From this computer [i-up]") || !strings.Contains(items, "From Files [i-file]") {
		t.Fatal("s-agenda-15 draws no Attach with its arrow: read the frame again")
	}
	f := attachFixture(t, 0, []string{"carrel"})
	f.srv.SeedFile(attachFiles+"carrel/plan.md", "the plan of the trip")
	list := f.path("calendar")
	before := len(f.srv.Paths(f.col))

	page := decode[view.EventEditPage](t, f.h.do(get(list+"/new", f.sess)))
	attach := actionByID(page.Form.Actions, "attach.add")
	if attach == nil || !attach.Submits || attach.File != "file" || len(attach.Arrow) != 2 || !attach.Arrow[1].Submits {
		t.Fatalf("Attach of a new event = %+v, want the button that sends the form with its file, and its arrow", attach)
	}
	form := formFields(page.Form)
	form.Set("summary", "Поездка с планом")
	resp := f.h.do(submitWithFile(attach.Href, f.sess, form, "file", "agenda.md", []byte("# Agenda")))
	if resp.Code != http.StatusSeeOther || !strings.Contains(resp.Header().Get("Location"), "/new?uid=") {
		t.Fatalf("attach: %d %s %s", resp.Code, resp.Header().Get("Location"), resp.Body.String())
	}
	if len(f.srv.Paths(f.col)) != before {
		t.Fatal("attaching to a new event wrote a record")
	}
	back := resp.Header().Get("Location")
	page = decode[view.EventEditPage](t, f.h.do(get(back, f.sess)))
	uid := strings.TrimPrefix(back[strings.Index(back, "uid="):], "uid=")
	if formFields(page.Form).Get("summary") != "Поездка с планом" || len(page.Attach) != 1 || page.Attach[0].Name != "agenda.md" || !page.Attach[0].Inline {
		t.Fatalf("the form back = %+v %+v", formFields(page.Form), page.Attach)
	}
	if formFields(page.Form).Get("uid") != uid {
		t.Errorf("the form does not carry its draft's UID %s", uid)
	}

	// From Files, for this draft.
	pick := actionByID(page.Form.Actions, "attach.add").Arrow[1]
	resp = f.h.do(postForm(pick.Href, f.sess, formFields(page.Form)))
	if resp.Code != http.StatusSeeOther || !strings.Contains(resp.Header().Get("Location"), "new="+uid) {
		t.Fatalf("From Files: %d %s", resp.Code, resp.Header().Get("Location"))
	}
	sheet := fromFiles(t, f, []view.MenuItem{{Action: view.Action{ID: "attach.files", Href: resp.Header().Get("Location")}}}, "carrel/", "plan.md")
	if sheet.Subject != "to the new event" {
		t.Errorf("subject = %q", sheet.Subject)
	}
	resp = f.h.do(postForm(actionByID(sheet.Actions, "files.picker.attach").Href, f.sess, url.Values{}))
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("attach from Files: %d %s", resp.Code, resp.Body.String())
	}
	page = decode[view.EventEditPage](t, f.h.do(get(resp.Header().Get("Location"), f.sess)))
	if len(page.Attach) != 2 || page.Attach[1].Name != "plan.md" || formFields(page.Form).Get("summary") != "Поездка с планом" {
		t.Fatalf("after From Files = %+v", page.Attach)
	}

	// Remove the first, sending the form.
	remove := page.Attach[0].Actions[0]
	if !remove.Submits {
		t.Errorf("Remove = %+v, want it to send the form", remove)
	}
	resp = f.h.do(postForm(remove.Href, f.sess, formFields(page.Form)))
	page = decode[view.EventEditPage](t, f.h.do(get(resp.Header().Get("Location"), f.sess)))
	if len(page.Attach) != 1 || page.Attach[0].Name != "plan.md" {
		t.Fatalf("after Remove = %+v", page.Attach)
	}

	// Save: the event under the draft's UID, the file in it.
	resp = f.h.do(postForm(page.Form.Action, f.sess, formFields(page.Form)))
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("save: %d %s", resp.Code, resp.Body.String())
	}
	got := body(t, f, uid+".ics")
	mustContain(t, got, "UID:"+uid, "SUMMARY:Поездка с планом", "FILENAME=plan.md")
	if strings.Contains(got, "agenda.md") {
		t.Errorf("the removed file was written:\n%s", got)
	}
}
