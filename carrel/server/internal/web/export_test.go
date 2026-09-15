package web_test

import (
	"net/http"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// download runs a GET that hands over a file and checks it did.
func download(t *testing.T, f davFixture, href, filename, media string) string {
	t.Helper()
	rec := f.h.do(get(href, f.sess))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: %d %s", href, rec.Code, rec.Body.String())
	}
	h := rec.Header()
	if got := h.Get("Content-Type"); !strings.HasPrefix(got, media) {
		t.Errorf("type = %q, want %q", got, media)
	}
	if got := h.Get("Content-Disposition"); !strings.Contains(got, "attachment") || !strings.Contains(got, filename) {
		t.Errorf("disposition = %q, want an attachment called %q", got, filename)
	}
	if h.Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(h.Get("Content-Security-Policy"), "sandbox") {
		t.Errorf("headers = %v", h)
	}
	return rec.Body.String()
}

// exportHref is the Export of a list screen.
func exportHref(t *testing.T, bar view.ListBar) string {
	t.Helper()
	return actionHref(t, bar.Actions, "collection.export")
}

// S-19: the export has no screen — the list offers it, and what comes back is
// the collection as a file.
func TestExportHandsOverTheBook(t *testing.T) {
	f := contactsFixture(t, false,
		"BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ада Лавлейс\nX-CUSTOM:keep\nEND:VCARD",
		"BEGIN:VCARD\nVERSION:4.0\nUID:grace\nFN:Grace Hopper\nEND:VCARD")
	rec := f.h.do(get(f.path("contacts"), f.sess))
	page := decode[view.ContactsListPage](t, rec)
	href := exportHref(t, page.Bar)

	body := download(t, f, href, "people.vcf", "text/vcard")
	if n := strings.Count(body, "BEGIN:VCARD"); n != 2 {
		t.Fatalf("cards = %d in\n%s", n, body)
	}
	for _, want := range []string{"Ада Лавлейс", "Grace Hopper", "X-CUSTOM:keep"} {
		if !strings.Contains(body, want) {
			t.Errorf("no %q in\n%s", want, body)
		}
	}
}

// A calendar goes out under one envelope, and a zone two records name is
// written once (RFC 5545 §3.6.5).
func TestExportCalendarWritesOneEnvelopeAndOneZone(t *testing.T) {
	event := func(uid, summary string) string {
		return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Other//Other//EN\r\n" +
			"BEGIN:VTIMEZONE\r\nTZID:Europe/Moscow\r\n" +
			"BEGIN:STANDARD\r\nDTSTART:19700101T000000\r\nTZOFFSETFROM:+0300\r\nTZOFFSETTO:+0300\r\nEND:STANDARD\r\n" +
			"END:VTIMEZONE\r\n" +
			"BEGIN:VEVENT\r\nUID:" + uid + "\r\nDTSTAMP:20260101T000000Z\r\n" +
			"DTSTART;TZID=Europe/Moscow:20260105T100000\r\nSUMMARY:" + summary + "\r\nX-KEEP:mine\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	}
	f := calendarFixture(t, nil, "one.ics", event("one", "Ковка"), "two.ics", event("two", "Отгрузка"))
	rec := f.h.do(get(f.path("calendar")+"?from=2026-01-05", f.sess))
	page := decode[view.AgendaPage](t, rec)
	href := exportHref(t, page.Bar)
	if !strings.Contains(href, "from=2026-01-05") || !strings.Contains(href, "to=2026-01-11") {
		t.Fatalf("the export of an agenda does not carry its week: %q", href)
	}

	body := download(t, f, href, "work.ics", "text/calendar")
	if n := strings.Count(body, "BEGIN:VCALENDAR"); n != 1 {
		t.Errorf("envelopes = %d in\n%s", n, body)
	}
	if n := strings.Count(body, "BEGIN:VTIMEZONE"); n != 1 {
		t.Errorf("zones = %d in\n%s", n, body)
	}
	if n := strings.Count(body, "BEGIN:VEVENT"); n != 2 {
		t.Errorf("events = %d in\n%s", n, body)
	}
	if !strings.Contains(body, "X-KEEP:mine") || !strings.Contains(body, "Ковка") {
		t.Errorf("an export lost something:\n%s", body)
	}
	if !strings.HasSuffix(body, "END:VCALENDAR\r\n") {
		t.Errorf("the envelope is not closed:\n%s", body)
	}
}

// The range of the agenda is the range of its export: a week asked for is a
// week handed over (S-19).
func TestExportCalendarTakesTheWindowOfTheScreen(t *testing.T) {
	ics := func(uid, day string) string {
		return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Other//Other//EN\r\n" +
			"BEGIN:VEVENT\r\nUID:" + uid + "\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:" + day + "T100000Z\r\n" +
			"SUMMARY:" + uid + "\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	}
	f := calendarFixture(t, nil, "in.ics", ics("inside", "20260107"), "out.ics", ics("outside", "20260401"))
	base := f.path("calendar") + "/export"

	week := download(t, f, base+"?from=2026-01-05&to=2026-01-11", "work.ics", "text/calendar")
	if !strings.Contains(week, "UID:inside") || strings.Contains(week, "UID:outside") {
		t.Errorf("the week holds the wrong records:\n%s", week)
	}
	whole := download(t, f, base, "work.ics", "text/calendar")
	if !strings.Contains(whole, "UID:inside") || !strings.Contains(whole, "UID:outside") {
		t.Errorf("without a range the collection goes whole:\n%s", whole)
	}
}

// Notes go out as Markdown with a header block, and what has no field of its
// own in that block is in the file all the same (S-19).
func TestExportNotesAreMarkdown(t *testing.T) {
	f := calendarFixture(t, nil, "n.ics",
		"BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Other//Other//EN\r\n"+
			"BEGIN:VJOURNAL\r\nUID:note-1\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20260105T100000Z\r\n"+
			"SUMMARY:Ковка\r\nCATEGORIES:work\r\nSTATUS:FINAL\r\nX-JTX-BOARD:2\r\n"+
			"DESCRIPTION:Первая строка\\nВторая\r\nEND:VJOURNAL\r\nEND:VCALENDAR\r\n")
	rec := f.h.do(get(f.path("notes"), f.sess))
	page := decode[view.NotesPage](t, rec)

	body := download(t, f, exportHref(t, page.Bar), "work.md", "text/markdown")
	for _, want := range []string{"---\n", "title: Ковка\n", "uid: note-1\n", "x-jtx-board: 2\n", "status: FINAL\n", "Первая строка\nВторая\n"} {
		if !strings.Contains(body, want) {
			t.Errorf("no %q in\n%s", want, body)
		}
	}
}

// A source that does not answer is a state on the screen, not a broken
// download (S-21).
func TestExportOfAnUnavailableCollectionSaysSo(t *testing.T) {
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada\nEND:VCARD")
	f.srv.Refuse("REPORT", http.StatusInternalServerError, "no")
	rec := f.h.do(get(f.path("contacts")+"/export", f.sess))
	page := decode[view.ContactsListPage](t, rec)
	if rec.Header().Get("Content-Disposition") != "" {
		t.Errorf("a file was handed over anyway: %v", rec.Header())
	}
	if !hasNotice(page.Notices, "People is unavailable") {
		t.Fatalf("notices = %+v", page.Notices)
	}
}

// Everything the rail offers for one collection is offered for every kind of
// them.
func TestRailOffersImportAndExport(t *testing.T) {
	f := calendarFixture(t, nil)
	page := decode[view.AgendaPage](t, f.h.do(get(f.path("calendar"), f.sess)))
	var found []string
	for _, sec := range page.Rail.Sections {
		for _, row := range sec.Rows {
			for _, item := range row.Menu {
				found = append(found, item.ID)
			}
		}
	}
	for _, want := range []string{"collection.import", "collection.export"} {
		if !contains(found, want) {
			t.Errorf("no %q in the rail menu: %v", want, found)
		}
	}
	list := f.path("calendar")
	if href := actionHref(t, page.Bar.Actions, "collection.import"); href != list+"/import" {
		t.Errorf("import = %q", href)
	}
}

func contains(all []string, want string) bool {
	for _, v := range all {
		if v == want {
			return true
		}
	}
	return false
}
