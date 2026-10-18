package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// A note with a file written into it, a tag and a property another client
// wrote: what of it a guest gets (Пб3).
const attachedNote = "BEGIN:VJOURNAL\nUID:n2\nDTSTAMP:20260801T000000Z\nSUMMARY:Sync notes\nDTSTART;VALUE=DATE:20260810\n" +
	"DESCRIPTION:Freeze the branch on Wednesday.\nCATEGORIES:secret-tag\nX-OTHER-CLIENT:private\n" +
	"ATTACH;ENCODING=BASE64;VALUE=BINARY;FMTTYPE=text/plain;FILENAME=board.txt:aGVsbG8gYm9hcmQ=\nEND:VJOURNAL"

// publishNote makes the link of n2 with the sheet's form as posted.
func publishNote(t *testing.T, f davFixture, form url.Values) string {
	t.Helper()
	sheet := decode[view.PublishSheet](t, f.h.do(get(f.publishHref("notes", searchWork, "n2"), f.sess)))
	form.Set("lives", "30")
	got := decode[view.PublishSheet](t, f.h.do(postForm(actionHref(t, sheet.Actions, "publish.create"), f.sess, form)))
	if !strings.HasPrefix(string(got.Link), "/p/") {
		t.Fatalf("no link: %+v", got)
	}
	return string(got.Link)
}

// The sheet asks of the attachments (s-publish-4): how many, and whether
// they go too, between what is published and its terms; the .md is a tick.
func TestNoteSheetAsksOfItsAttachments(t *testing.T) {
	f := publishFixture(t)
	f.srv.Seed(searchWork+"n2.ics", ics(attachedNote))
	sheet := decode[view.PublishSheet](t, f.h.do(get(f.publishHref("notes", searchWork, "n2"), f.sess)))
	var legend, aside string
	var tick, md bool
	for _, set := range sheet.Form.Sets {
		if set.Legend == "Attachments" {
			legend, aside = string(set.Legend), string(set.LegendAside)
		}
		for _, row := range set.Rows {
			tick = tick || row.Name == "attachments" && row.Kind == view.InputCheck && row.Value == ""
			md = md || row.Name == "download" && row.Value == "on" && len(row.LabelRuns) == 3
		}
	}
	if legend == "" || !strings.HasPrefix(aside, "1 file · ") || !tick || !md {
		t.Errorf("sheet = %+v", sheet.Form.Sets)
	}
	// A note without attachments is not asked.
	plain := decode[view.PublishSheet](t, f.h.do(get(f.publishHref("notes", searchWork, "n1"), f.sess)))
	for _, set := range plain.Form.Sets {
		if set.Legend == "Attachments" {
			t.Errorf("asked of nothing: %+v", plain.Form.Sets)
		}
	}
}

// Unticked, the page says how many files there are and not their names;
// the guest's .md is the title, the text and the day — no attachment, no
// tag, nothing another client wrote.
func TestNoteAttachmentsStayUnlessTicked(t *testing.T) {
	f := publishFixture(t)
	f.srv.Seed(searchWork+"n2.ics", ics(attachedNote))
	link := publishNote(t, f, url.Values{"download": {"on"}})
	page := publicPage(t, f, link)
	if len(page.Attachments) != 0 || !strings.Contains(string(page.AttachNote), "1 file is attached") {
		t.Errorf("page = %+v / %q", page.Attachments, page.AttachNote)
	}
	md := f.h.do(anon(link + "/download")).Body.String()
	if !strings.Contains(md, "Freeze the branch") {
		t.Errorf("the .md lost the text:\n%s", md)
	}
	for _, leak := range []string{"ATTACH", "board.txt", "aGVsbG8", "secret-tag", "X-OTHER-CLIENT", "private"} {
		if strings.Contains(md, leak) {
			t.Errorf("the guest's .md carries %q:\n%s", leak, md)
		}
	}
	// The .md taken off: the page stays, the file goes.
	off := publishNote(t, f, url.Values{})
	if rec := f.h.do(anon(off + "/download")); rec.Code != http.StatusNotFound {
		t.Errorf("a .md not allowed: %d", rec.Code)
	}
}

// Ticked, each file has its own link, named on the note's page by its name
// and size; it is revoked with the note's and is no row of the list; a file
// a removal shifted into its place is not opened by it.
func TestNoteAttachmentsGoWithTheirOwnLinks(t *testing.T) {
	f := publishFixture(t)
	f.srv.Seed(searchWork+"n2.ics", ics(attachedNote))
	link := publishNote(t, f, url.Values{"download": {"on"}, "attachments": {"on"}})
	page := publicPage(t, f, link)
	if len(page.Attachments) != 1 || page.Attachments[0].Name != "board.txt" || page.AttachNote != "" {
		t.Fatalf("attachments = %+v / %q", page.Attachments, page.AttachNote)
	}
	child := page.Attachments[0].Href
	if !strings.HasPrefix(child, "/p/") || child == link {
		t.Fatalf("child link = %q", child)
	}
	file := publicPage(t, f, child)
	if file.File == nil || file.File.Name != "board.txt" || len(file.Notices) != 0 {
		t.Fatalf("file page = %+v", file)
	}
	if got := f.h.do(anon(child + "/download")).Body.String(); got != "hello board" {
		t.Errorf("file = %q", got)
	}
	if n := len(f.h.users.PublishedLinks(f.sess.AccountID())); n != 1 {
		t.Errorf("rows of the list: %d", n)
	}

	// Another file where the published one was: the child says it is gone.
	f.srv.Seed(searchWork+"n2.ics", ics(strings.Replace(attachedNote, "FILENAME=board.txt:aGVsbG8gYm9hcmQ=", "FILENAME=salary.txt:c2VjcmV0", 1)))
	shifted := publicPage(t, f, child)
	if shifted.File != nil || len(shifted.Notices) == 0 {
		t.Errorf("a shifted place opened: %+v", shifted)
	}
	if rec := f.h.do(anon(child + "/download")); strings.Contains(rec.Body.String(), "secret") {
		t.Errorf("the file shifted in went out: %q", rec.Body.String())
	}

	// Revoked with the note's.
	f.srv.Seed(searchWork+"n2.ics", ics(attachedNote))
	id := f.h.users.PublishedLinks(f.sess.AccountID())[0].ID
	if err := f.h.users.RevokePublished(f.sess.AccountID(), id); err != nil {
		t.Fatal(err)
	}
	if rec := f.h.do(anon(child)); rec.Code != http.StatusNotFound {
		t.Errorf("the child outlived its note: %d", rec.Code)
	}
}

// A note's file that lies in a file collection of the catalog is published
// as that file is: its own page, read from where it lies.
func TestNoteLinkedAttachmentIsPublishedAsTheFile(t *testing.T) {
	f := attachFixture(t, 1<<20, []string{"carrel"})
	work := davtest.CalendarHome + "work/"
	f.srv.SeedFile(attachFiles+"carrel/plan.txt", "the plan")
	f.srv.Seed(work+"n3.ics", ics("BEGIN:VJOURNAL\nUID:n3\nDTSTAMP:20260801T000000Z\nSUMMARY:Plan\nDTSTART;VALUE=DATE:20260810\n"+
		"ATTACH;FMTTYPE=text/plain;FILENAME=plan.txt:"+f.srv.URL+attachFiles+"carrel/plan.txt\nEND:VJOURNAL"))
	sheet := decode[view.PublishSheet](t, f.h.do(get(f.publishHref("notes", work, "n3"), f.sess)))
	got := decode[view.PublishSheet](t, f.h.do(postForm(actionHref(t, sheet.Actions, "publish.create"), f.sess,
		url.Values{"lives": {"30"}, "download": {"on"}, "attachments": {"on"}})))
	page := publicPage(t, f, string(got.Link))
	if len(page.Attachments) != 1 || page.Attachments[0].Name != "plan.txt" {
		t.Fatalf("attachments = %+v / %q", page.Attachments, page.AttachNote)
	}
	file := publicPage(t, f, page.Attachments[0].Href)
	if file.File == nil || file.File.Name != "plan.txt" {
		t.Fatalf("file page = %+v", file)
	}
	if body := f.h.do(anon(page.Attachments[0].Href + "/download")).Body.String(); body != "the plan" {
		t.Errorf("file = %q", body)
	}
}
