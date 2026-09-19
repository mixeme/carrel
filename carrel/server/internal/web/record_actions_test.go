package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// forms-tail.md: Send on the contact card (s-contacts-6) puts the card into
// a letter — the attach sheet already writes a record (S-26.2) — and opens
// the letter; without mail on the instance there is no Send.
func TestSendAContactFromItsCard(t *testing.T) {
	f, _ := letterFixture(t)
	rec := f.path("contacts") + "/ada"

	page := decode[view.ContactPage](t, f.h.do(get(rec, f.sess)))
	send := actionByID(page.Actions, "record.send")
	if send == nil || send.Method != "POST" {
		t.Fatalf("contact actions = %+v, want Send", page.Actions)
	}
	resp := f.h.do(postHref(f, send.Href))
	if resp.Code != 303 || resp.Header().Get("Location") != "/app/mail/compose" {
		t.Fatalf("Send: %d %q %s", resp.Code, resp.Header().Get("Location"), resp.Body.String())
	}
	sheet := decode[view.MailSheet](t, f.h.do(get("/app/mail/compose", f.sess)))
	if len(sheet.Attach) != 1 || sheet.Attach[0].Title != "Ada Lovelace" {
		t.Errorf("letter attachments = %+v", sheet.Attach)
	}

	plain := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:bob\nFN:Bob\nEND:VCARD\n")
	page = decode[view.ContactPage](t, plain.h.do(get(plain.path("contacts")+"/bob", plain.sess)))
	if actionByID(page.Actions, "record.send") != nil {
		t.Error("Send is offered where no letter can be written")
	}
}

// New task on the event card (s-agenda-2) is the quick sheet of a task in
// the same calendar, carrying RELATED-TO the event.
func TestNewTaskFromAnEvent(t *testing.T) {
	f := calendarFixture(t, nil, "meet.ics", ics("BEGIN:VEVENT\nUID:meet\nDTSTAMP:20260101T000000Z\nDTSTART:20260115T100000Z\nSUMMARY:Review\nEND:VEVENT"))
	page := decode[view.EventPage](t, f.h.do(get(f.path("calendar")+"/meet", f.sess)))
	act := actionByID(page.Actions, "event.task")
	if act == nil || act.Method != "GET" {
		t.Fatalf("event actions = %+v, want New task", page.Actions)
	}
	u, _ := url.Parse(act.Href)
	sheet := decode[view.NewSheet](t, f.h.do(get(u.RequestURI(), f.sess)))
	form := formFields(sheet.Form)
	if form.Get("related") != "meet" {
		t.Fatalf("quick sheet form = %v, want related to the event", form)
	}
	form.Set("summary", "Prepare slides")
	quick, _ := url.Parse(sheet.Form.Action)
	if resp := f.h.do(postForm(quick.EscapedPath(), f.sess, form)); resp.Code != 303 {
		t.Fatalf("save: %d %s", resp.Code, resp.Body.String())
	}
	found := false
	for _, p := range f.srv.Paths(f.col) {
		b, _, _ := f.srv.Object(p)
		b = unfolded(b)
		found = found || strings.Contains(b, "BEGIN:VTODO") && strings.Contains(b, "RELATED-TO:meet") && strings.Contains(b, "SUMMARY:Prepare slides")
	}
	if !found {
		t.Error("no task related to the event was written")
	}
}

// Export Markdown on a note (s-notes-2): one note, the format of the
// notebook export (S-19).
func TestExportOneNoteAsMarkdown(t *testing.T) {
	f := calendarFixture(t, nil,
		"n1.ics", ics("BEGIN:VJOURNAL\nUID:n1\nDTSTAMP:20260101T000000Z\nSUMMARY:Plan\nDESCRIPTION:First line\\n\\n- item\nEND:VJOURNAL"),
		"n2.ics", ics("BEGIN:VJOURNAL\nUID:n2\nDTSTAMP:20260101T000000Z\nSUMMARY:Other\nDESCRIPTION:Not this one\nEND:VJOURNAL"))
	page := decode[view.NotePage](t, f.h.do(get(f.path("notes")+"/n1", f.sess)))
	act := actionByID(page.Meta.Actions, "note.export")
	if act == nil || act.Method != "GET" {
		t.Fatalf("note actions = %+v, want Export Markdown", page.Meta.Actions)
	}
	u, _ := url.Parse(act.Href)
	resp := f.h.do(get(u.RequestURI(), f.sess))
	if resp.Code != 200 || !strings.HasPrefix(resp.Header().Get("Content-Type"), "text/markdown") ||
		!strings.Contains(resp.Header().Get("Content-Disposition"), "plan.md") {
		t.Fatalf("export: %d %v", resp.Code, resp.Header())
	}
	got := resp.Body.String()
	if !strings.Contains(got, "Plan") || !strings.Contains(got, "- item") || strings.Contains(got, "Not this one") {
		t.Errorf("export body:\n%s", got)
	}
	if resp := f.h.do(get(f.path("notes")+"/export?uid=nosuch", f.sess)); resp.Code != 404 {
		t.Errorf("unknown note: %d, want 404", resp.Code)
	}
}

// views-tail.md, owner's decision 19.09.2026: the note has no sheet of its
// own (C-2.1, equality of collections), and "nowhere to write" (s-notes-6)
// is the common sheet's too: no collection of the kind to write to, and the
// sheet says so with the way out, not a 404.
func TestQuickSheetWithNowhereToWrite(t *testing.T) {
	h, sess := davHarness(t)
	for _, kind := range []string{"notes", "contacts"} {
		sheet := decode[view.NewSheet](t, h.do(get("/app/"+kind+"/quick", sess)))
		if len(sheet.Target.Options) != 0 || len(sheet.Form.Sets) != 0 {
			t.Errorf("%s: sheet = %+v, want nowhere to write", kind, sheet)
		}
		if a := actionByID(sheet.Actions, "connection.add"); a == nil || a.Href != "/app/settings/connections" {
			t.Errorf("%s: actions = %+v, want Connect account", kind, sheet.Actions)
		}
	}
}
