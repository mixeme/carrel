package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// A new form whose files a later form pushed out (Б7; Typecase's note 181
// p. 2, task 256, s-agenda-32): Save writes nothing, the alert under the
// head says so in the frame's words, what was typed stays, and each lost
// file stays a row with «not kept» and Remove; Save again writes the event
// without them.
func TestTheLostFilesOfANewFormStayNamed(t *testing.T) {
	frame := mockupFrame(t, "s-agenda-32")
	const title, body = "Nothing was saved:", "the files attached here are no longer kept — newer forms took their place. Attach them again, or save without them."
	for _, want := range []string{"<b>" + title + "</b> " + body, "not kept"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("s-agenda-32 does not draw %q: read the frame again", want)
		}
	}
	f := attachFixture(t, 0, []string{"carrel"})
	list := f.path("calendar")
	attach := func(summary, name string) (uid string, form url.Values) {
		t.Helper()
		page := decode[view.EventEditPage](t, f.h.do(get(list+"/new", f.sess)))
		values := formFields(page.Form)
		values.Set("summary", summary)
		resp := f.h.do(submitWithFile(actionByID(page.Form.Actions, "attach.add").Href, f.sess, values, "file", name, []byte("%PDF-1.4 "+name)))
		back := resp.Header().Get("Location")
		if resp.Code != http.StatusSeeOther || !strings.Contains(back, "uid=") {
			t.Fatalf("attach %s: %d %s", name, resp.Code, back)
		}
		page = decode[view.EventEditPage](t, f.h.do(get(back, f.sess)))
		return back[strings.Index(back, "uid=")+4:], formFields(page.Form)
	}
	uid, first := attach("Планёрка по обложке", "макет-обложки-v3.pdf")
	for _, name := range []string{"a.pdf", "b.pdf", "c.pdf", "d.pdf"} {
		attach("Другая форма", name)
	}
	if first.Get("uid") != uid {
		t.Fatalf("the first form carries %q, not its UID %q", first.Get("uid"), uid)
	}
	before := len(f.srv.Paths(f.col))
	page := decode[view.EventEditPage](t, f.h.do(postForm(list+"/new", f.sess, first)))
	if len(f.srv.Paths(f.col)) != before {
		t.Fatal("a form with lost files wrote the event")
	}
	if len(page.Notices) != 1 || page.Notices[0].Title != title || page.Notices[0].Body != body {
		t.Errorf("the alert = %+v, want %q %q", page.Notices, title, body)
	}
	if formFields(page.Form).Get("summary") != "Планёрка по обложке" || formFields(page.Form).Get("uid") != "" {
		t.Errorf("the form back: summary %q, uid %q", formFields(page.Form).Get("summary"), formFields(page.Form).Get("uid"))
	}
	if len(page.Attach) != 1 {
		t.Fatalf("the lost files are %+v, want the one row", page.Attach)
	}
	a := page.Attach[0]
	if a.Name != "макет-обложки-v3.pdf" || a.State != view.RecordUnavailable || a.Inline || a.Size == "" ||
		len(a.Actions) != 1 || a.Actions[0].Label != "Remove" || !a.Actions[0].Danger || a.Actions[0].Href != "" {
		t.Errorf("the lost row = %+v", a)
	}
	// Save again: the event without them.
	f.h.do(postForm(list+"/new", f.sess, formFields(page.Form)))
	if len(f.srv.Paths(f.col)) != before+1 {
		t.Error("Save again did not write the event without its files")
	}
}
