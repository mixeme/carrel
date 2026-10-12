package web_test

import (
	"net/http"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// An export does not leave out, without a word, a record the server handed
// over and Carrel could not read (Б3 of the audit of 11.10.2026,
// docs/plan/scans/silent-choices.md). Such a record goes into the .ics as the
// server holds it, byte for byte; when it cannot — the server will not give
// it, or the file is Markdown — nothing is handed over until the person says
// «Export without them».

// orphan is a record Carrel cannot read and the server serves: a changed
// instance of a series whose series is not in the resource.
func orphan(component, uid string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Other//Other//EN\r\n" +
		"BEGIN:" + component + "\r\nUID:" + uid + "\r\nDTSTAMP:20260101T000000Z\r\n" +
		"RECURRENCE-ID:20260107T100000Z\r\nDTSTART:20260107T120000Z\r\n" +
		"SUMMARY:Moved once\r\nX-KEEP:mine\r\nEND:" + component + "\r\nEND:VCALENDAR\r\n"
}

func plainEvent(uid string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Other//Other//EN\r\n" +
		"BEGIN:VEVENT\r\nUID:" + uid + "\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20260107T100000Z\r\n" +
		"SUMMARY:" + uid + "\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
}

func TestExportKeepsARecordCarrelCannotRead(t *testing.T) {
	f := calendarFixture(t, nil, "plain.ics", plainEvent("plain"), "orphan.ics", orphan("VEVENT", "lone"))
	// The premise: the list names it as unreadable.
	page := decode[view.AgendaPage](t, f.h.do(get(f.path("calendar")+"?from=2026-01-05", f.sess)))
	if !hasNotice(page.Notices, "1 record could not be read") {
		t.Fatalf("the stand's orphan is read after all: notices = %+v", page.Notices)
	}
	base := f.path("calendar") + "/export"
	for _, href := range []string{base, base + "?from=2026-01-05&to=2026-01-11", base + "?whole=1"} {
		body := download(t, f, href, "work.ics", "text/calendar")
		for _, want := range []string{"UID:plain", "UID:lone", "RECURRENCE-ID:20260107T100000Z", "X-KEEP:mine"} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: no %q in\n%s", href, want, body)
			}
		}
		if n := strings.Count(body, "BEGIN:VCALENDAR"); n != 1 || !strings.HasSuffix(body, "END:VCALENDAR\r\n") {
			t.Errorf("%s: envelopes = %d in\n%s", href, n, body)
		}
	}
}

// What cannot be put into the file is said before any file is handed over,
// and the file without it is one explicit step away.
func TestExportAsksBeforeLeavingARecordOut(t *testing.T) {
	t.Run("the server will not give the record", func(t *testing.T) {
		f := calendarFixture(t, nil, "plain.ics", plainEvent("plain"), "orphan.ics", orphan("VEVENT", "lone"))
		f.srv.Refuse(http.MethodGet, http.StatusInternalServerError, "no")
		rec := f.h.do(get(f.path("calendar")+"/export", f.sess))
		page := decode[view.AgendaPage](t, rec)
		if rec.Header().Get("Content-Disposition") != "" {
			t.Fatalf("a file was handed over without the record: %v", rec.Header())
		}
		if !hasNotice(page.Notices, "The file would lack 1 record") {
			t.Fatalf("notices = %+v", page.Notices)
		}
		var without string
		for _, n := range page.Notices {
			for _, a := range n.Actions {
				if a.Label == "Export without them" {
					without = a.Href
				}
			}
			if n.Details != nil && !strings.Contains(plainLines(n.Details.Lines), "orphan.ics") {
				t.Errorf("the notice does not name the record: %+v", n.Details)
			}
		}
		if without == "" {
			t.Fatalf("no «Export without them»: %+v", page.Notices)
		}
		body := download(t, f, without, "work.ics", "text/calendar")
		if !strings.Contains(body, "UID:plain") || strings.Contains(body, "UID:lone") {
			t.Errorf("the file without it:\n%s", body)
		}
	})
	t.Run("Markdown has no place for it", func(t *testing.T) {
		f := calendarFixture(t, nil, "n.ics",
			"BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Other//Other//EN\r\n"+
				"BEGIN:VJOURNAL\r\nUID:note-1\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20260105T100000Z\r\n"+
				"SUMMARY:Ковка\r\nEND:VJOURNAL\r\nEND:VCALENDAR\r\n",
			"orphan.ics", orphan("VJOURNAL", "lone"))
		rec := f.h.do(get(f.path("notes")+"/export", f.sess))
		page := decode[view.NotesPage](t, rec)
		if rec.Header().Get("Content-Disposition") != "" {
			t.Fatalf("a file was handed over without the record: %v", rec.Header())
		}
		if !hasNotice(page.Notices, "The file would lack 1 record") {
			t.Fatalf("notices = %+v", page.Notices)
		}
		body := download(t, f, f.path("notes")+"/export?without=1", "work.md", "text/markdown")
		if !strings.Contains(body, "title: Ковка") {
			t.Errorf("the file without it:\n%s", body)
		}
	})
}

func plainLines(lines []view.Text) string {
	var b strings.Builder
	for _, l := range lines {
		b.WriteString(string(l))
		b.WriteString("\n")
	}
	return b.String()
}
