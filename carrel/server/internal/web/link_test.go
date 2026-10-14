package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Р8 of scans/kinds-parity.md: an event and a task showed their URL as
// «Link» and no form could change it; a note neither showed nor edited it.
// Now every kind's form has «Link», a change writes that one line (S-7),
// and a note's column shows the link as a task's panel does.
func TestEveryKindEditsItsLink(t *testing.T) {
	f := calendarFixture(t, nil,
		"e1.ics", ics("BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260101T000000Z\nDTSTART:20260115T100000Z\nSUMMARY:Review\nURL:https://a.example/e\nX-KEEP:1\nEND:VEVENT"),
		"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Slides\nURL:https://a.example/t\nX-KEEP:1\nEND:VTODO"),
		"n1.ics", ics("BEGIN:VJOURNAL\nUID:n1\nDTSTAMP:20260101T000000Z\nSUMMARY:Plan\nURL:https://a.example/n\nX-KEEP:1\nEND:VJOURNAL"))
	for _, c := range []struct{ list, uid, was string }{
		{"calendar", "e1", "https://a.example/e"},
		{"tasks", "t1", "https://a.example/t"},
		{"notes", "n1", "https://a.example/n"},
	} {
		rec := f.path(c.list) + "/" + c.uid
		var form url.Values
		if c.list == "notes" {
			form = noteFormOf(t, f, c.uid)
		} else {
			form = formFields(decode[struct {
				Form view.Form `json:"form"`
			}](t, f.h.do(get(rec+"/edit", f.sess))).Form)
		}
		if got := form.Get("url"); got != c.was {
			t.Errorf("%s: Link = %q, want %q", c.uid, got, c.was)
			continue
		}
		form.Set("url", "https://b.example/"+c.uid)
		if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != http.StatusSeeOther {
			t.Fatalf("%s: POST %d %s", c.uid, resp.Code, resp.Body.String())
		}
		got := strings.ReplaceAll(body(t, f, c.uid+".ics"), "\r\n", "\n")
		if !strings.Contains(got, "\nURL:https://b.example/"+c.uid+"\n") || !strings.Contains(got, "\nX-KEEP:1\n") {
			t.Errorf("%s after Save:\n%s", c.uid, got)
		}
	}

	note := decode[view.NotePage](t, f.h.do(get(f.path("notes")+"/n1", f.sess)))
	if l := note.Meta.Link; l == nil || l.Value != "https://b.example/n1" || l.Href != "https://b.example/n1" {
		t.Errorf("the note's Link = %+v", l)
	}
}
