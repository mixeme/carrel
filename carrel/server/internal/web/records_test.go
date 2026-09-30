package web_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// davFixture is a person with one connection to an in-memory DAV server,
// connected through POST /app/settings/connections like anyone else.
type davFixture struct {
	srv  *davtest.Server
	h    *harness
	sess *session.Session
	id   string
	col  string
}

func (f davFixture) path(kind string) string {
	return "/app/" + kind + "/" + f.id + "/" + url.PathEscape(f.col)
}

func connectFixture(t *testing.T, srv *davtest.Server, col string, configure func(*harness)) davFixture {
	t.Helper()
	h, sess := davHarness(t)
	if configure != nil {
		configure(h)
		h.rebuild()
	}
	rec := h.do(postForm("/app/settings/connections", sess, url.Values{
		"server": {srv.URL}, "login": {srv.Login}, "password": {srv.Password},
	}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("connect: %d %s", rec.Code, rec.Body.String())
	}
	key, _ := sess.DataKey()
	blob, _ := h.users.Catalog(sess.AccountID(), key)
	return davFixture{srv: srv, h: h, sess: sess, id: blob.Connections[0].ID, col: col}
}

var uidLine = regexp.MustCompile(`(?m)^UID:(\S+)`)

func contactsFixture(t *testing.T, readOnly bool, cards ...string) davFixture {
	t.Helper()
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	book := davtest.AddressBookHome + "people/"
	srv.AddAddressBook(carddav.AddressBook{Path: book, Name: "People", ReadOnly: readOnly})
	for _, c := range cards {
		srv.Seed(book+uidLine.FindStringSubmatch(c)[1]+".vcf", c)
	}
	return connectFixture(t, srv, book, nil)
}

// calendarFixture seeds name, body pairs into one calendar.
func calendarFixture(t *testing.T, configure func(*harness), files ...string) davFixture {
	t.Helper()
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	work := davtest.CalendarHome + "work/"
	srv.AddCalendar(caldav.Calendar{Path: work, Name: "Work"})
	for i := 0; i+1 < len(files); i += 2 {
		srv.Seed(work+files[i], files[i+1])
	}
	return connectFixture(t, srv, work, configure)
}

func ics(components string) string {
	return "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//Other client//EN\n" + components + "\nEND:VCALENDAR\n"
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %T: %v\n%s", v, err, rec.Body.String())
	}
	return v
}

// formFields posts a form back the way a browser would: every row as shown.
func formFields(f view.Form) url.Values {
	out := url.Values{}
	for _, set := range f.Sets {
		for _, row := range set.Rows {
			switch {
			case row.Name == "_csrf":
			case row.Kind == view.InputAdd, row.Kind == view.InputRemove:
				// A button that adds or removes a row is posted only when
				// pressed.
			case row.Name == "removed":
				// Each row Remove took out is named once (C-6.8).
				out.Add(row.Name, string(row.Value))
			case row.Kind == view.InputCheck && len(row.Options) > 0:
				// Several ticks under one name: each one ticked is a value.
				out.Del(row.Name)
				for _, o := range row.Options {
					if o.Selected {
						out.Add(row.Name, o.Value)
					}
				}
			default:
				out.Set(row.Name, string(row.Value))
			}
		}
	}
	return out
}

func actionHref(t *testing.T, actions []view.Action, id string) string {
	t.Helper()
	for _, a := range actions {
		if a.ID == id {
			return a.Href
		}
	}
	t.Fatalf("no action %q in %+v", id, actions)
	return ""
}

func hasNotice(notices []view.Notice, title string) bool {
	for _, n := range notices {
		if string(n.Title) == title {
			return true
		}
	}
	return false
}

// body is the record as the server holds it, unfolded: what Carrel writes is
// folded at 75 octets (docs/servers.md, SV-31), and an assertion here is
// about what the record holds, not about where the line was broken. The
// folding itself is held by the tests of the providers.
func body(t *testing.T, f davFixture, name string) string {
	t.Helper()
	b, _, ok := f.srv.Object(f.col + name)
	if !ok {
		t.Fatalf("%s: not on the server", name)
	}
	return unfolded(b)
}

// unfolded is what a parser does with a fold: a CRLF and one space or tab are
// not there (RFC 5545 §3.1, RFC 6350 §3.2).
func unfolded(doc string) string {
	doc = strings.ReplaceAll(doc, "\r\n ", "")
	return strings.ReplaceAll(doc, "\r\n\t", "")
}

func mustContain(t *testing.T, got string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("missing %q in\n%s", w, got)
		}
	}
}

func TestContactsListAndCard(t *testing.T) {
	f := contactsFixture(t, false,
		"BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nEMAIL;TYPE=work:ada@example.com\nTEL:+44 20 1\nORG:Analytical;Engines\nCATEGORIES:math,poetry\nX-CUSTOM:keep\nEND:VCARD\n",
		"BEGIN:VCARD\nVERSION:4.0\nUID:bob\nFN:bob builder\nEND:VCARD\n",
		"BEGIN:VCARD\nVERSION:4.0\nUID:num\nFN:42 Club\nEND:VCARD\n",
	)
	list := f.path("contacts")

	page := decode[view.ContactsListPage](t, f.h.do(get(list, f.sess)))
	var rubrics, names []string
	for _, g := range page.Groups {
		rubrics = append(rubrics, string(g.Rubric))
		for _, row := range g.Rows {
			names = append(names, string(row.Name))
		}
	}
	if strings.Join(rubrics, ",") != "A,B,#" || strings.Join(names, ",") != "Ada Lovelace,bob builder,42 Club" {
		t.Fatalf("groups %v, names %v", rubrics, names)
	}
	ada := page.Groups[0].Rows[0]
	if ada.Href != list+"/ada" || ada.Email != "ada@example.com" || ada.Phone != "+44 20 1" ||
		ada.Org != "Analytical, Engines" || ada.Avatar.Initials != "AL" || ada.Avatar.Colour == "" {
		t.Errorf("row = %+v", ada)
	}
	if page.Poll != nil || page.Bar.Total == nil || *page.Bar.Total != 3 {
		t.Errorf("one collection is not a fanout: poll %v, total %v", page.Poll, page.Bar.Total)
	}
	if n := f.srv.CountRequests("GET", f.col); n != 0 {
		t.Errorf("GET requests = %d: bodies from the REPORT were fetched again", n)
	}

	card := decode[view.ContactPage](t, f.h.do(get(list+"/ada", f.sess)))
	if card.Name != "Ada Lovelace" || card.ReadOnly || !strings.Contains(string(card.Source.Stamp), "People") {
		t.Errorf("card = %+v", card)
	}
	var email *view.Field
	for i := range card.Fields {
		if card.Fields[i].Label == "Email · work" {
			email = &card.Fields[i]
		}
	}
	if email == nil || email.Value != "ada@example.com" || email.Href != "mailto:ada@example.com" {
		t.Errorf("fields = %+v", card.Fields)
	}
	if len(card.Tags) != 2 || len(card.Foreign) != 1 || card.Foreign[0].Label != "X-CUSTOM" {
		t.Errorf("tags %+v, foreign %+v", card.Tags, card.Foreign)
	}
	if !strings.Contains(actionHref(t, card.Actions, "record.delete"), "etag=") {
		t.Error("delete does not carry the version it was shown at")
	}
	actionHref(t, card.Actions, "record.edit")

	if rec := f.h.do(get(list+"/nobody", f.sess)); rec.Code != http.StatusNotFound {
		t.Errorf("missing record: %d", rec.Code)
	}
	if rec := f.h.do(get(f.path("calendar"), f.sess)); rec.Code != http.StatusNotFound {
		t.Errorf("an address book opened as a calendar: %d", rec.Code)
	}
}

func agendaLines(p view.AgendaPage) []string {
	var out []string
	for _, d := range p.Days {
		for _, r := range d.Rows {
			out = append(out, strings.Join([]string{string(d.Rubric), string(r.Time), string(r.Summary), string(r.Zone)}, "|"))
		}
	}
	return out
}

// S-11: the week goes to the server as the window; series expand here; an
// instance rewritten by another client replaces the instant it names; the
// original zone shows only where it differs from the instance zone (C-10).
func TestAgendaWeekOverridesAndZones(t *testing.T) {
	f := calendarFixture(t, func(h *harness) { h.cfg.Timezone = "Europe/Berlin" },
		"series.ics", ics("BEGIN:VEVENT\nUID:series\nDTSTAMP:20260101T000000Z\nDTSTART:20260112T090000Z\nDTEND:20260112T093000Z\n"+
			"RRULE:FREQ=WEEKLY;COUNT=4\nSUMMARY:Standup\nEND:VEVENT\n"+
			"BEGIN:VEVENT\nUID:series\nDTSTAMP:20260101T000000Z\nRECURRENCE-ID:20260119T090000Z\nDTSTART:20260119T110000Z\n"+
			"DTEND:20260119T113000Z\nSUMMARY:Standup moved\nEND:VEVENT"),
		"ny.ics", ics("BEGIN:VEVENT\nUID:ny\nDTSTAMP:20260101T000000Z\nDTSTART;TZID=America/New_York:20260114T100000\n"+
			"DTEND;TZID=America/New_York:20260114T110000\nSUMMARY:Call New York\nEND:VEVENT"),
		"day.ics", ics("BEGIN:VEVENT\nUID:day\nDTSTAMP:20260101T000000Z\nDTSTART;VALUE=DATE:20260113\nDTEND;VALUE=DATE:20260114\nSUMMARY:Holiday\nEND:VEVENT"),
		"later.ics", ics("BEGIN:VEVENT\nUID:later\nDTSTAMP:20260101T000000Z\nDTSTART:20260220T100000Z\nSUMMARY:Later\nEND:VEVENT"),
	)
	agenda := f.path("calendar")

	page := decode[view.AgendaPage](t, f.h.do(get(agenda+"?from=2026-01-12", f.sess)))
	want := []string{
		"Monday, 12 January|10:00|Standup|",
		"Tuesday, 13 January|All day|Holiday|",
		"Wednesday, 14 January|16:00|Call New York|America/New_York",
	}
	if got := agendaLines(page); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("week of 12 January:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if page.Bar.Range == nil || page.Bar.Range.From != "12 January 2026" || page.Bar.Range.To != "18 January 2026" {
		t.Errorf("range = %+v", page.Bar.Range)
	}

	page = decode[view.AgendaPage](t, f.h.do(get(agenda+"?from=2026-01-19", f.sess)))
	if got := agendaLines(page); strings.Join(got, "\n") != "Monday, 19 January|12:00|Standup moved|" {
		t.Errorf("week of 19 January: %v", got)
	}
}

func TestEventPageRelatedRemindersAttendees(t *testing.T) {
	f := calendarFixture(t, nil,
		"task.ics", ics("BEGIN:VTODO\nUID:task-1\nDTSTAMP:20260101T000000Z\nSUMMARY:Prepare slides\nEND:VTODO"),
		"meet.ics", ics("BEGIN:VEVENT\nUID:meet\nDTSTAMP:20260101T000000Z\nDTSTART:20260115T100000Z\nDTEND:20260115T110000Z\n"+
			"RRULE:FREQ=WEEKLY;BYDAY=TH;COUNT=5\nSUMMARY:Review\nATTENDEE;CN=Ada;PARTSTAT=ACCEPTED:mailto:ada@example.com\n"+
			"RELATED-TO:task-1\nRELATED-TO:elsewhere\nBEGIN:VALARM\nACTION:DISPLAY\nDESCRIPTION:x\nTRIGGER:-PT15M\nEND:VALARM\nEND:VEVENT"),
	)
	events, tasks := f.path("calendar"), f.path("tasks")

	ev := decode[view.EventPage](t, f.h.do(get(events+"/meet", f.sess)))
	if len(ev.Related) != 2 || ev.Related[0].Href != "" || ev.Related[1].Href != "" || ev.Related[0].Title != "task-1" {
		t.Errorf("before any list is loaded, related = %+v", ev.Related)
	}
	if len(ev.Reminders) != 1 || ev.Reminders[0].Label != "15 minutes before" {
		t.Errorf("reminders = %+v", ev.Reminders)
	}
	if len(ev.Attendees) != 1 || ev.Attendees[0] != (view.AttendeeRow{Name: "Ada", Email: "ada@example.com", Status: "Accepted"}) {
		t.Errorf("attendees = %+v", ev.Attendees)
	}
	if !strings.Contains(string(ev.Recurrence.Raw), "FREQ=WEEKLY") || ev.Recurrence.Summary != "Weekly on Thursday, 5 times" ||
		ev.Recurrence.Note != view.Text(frameSeriesLine(t)) {
		t.Errorf("recurrence = %+v", ev.Recurrence)
	}
	if len(ev.Fields) == 0 || ev.Fields[0].Value != "Thursday, 15 January 2026, 10:00 – 11:00" {
		t.Errorf("fields = %+v", ev.Fields)
	}

	_ = decode[view.TasksPage](t, f.h.do(get(tasks+"?show=all", f.sess)))
	reports := f.srv.CountRequests("REPORT", f.col)
	ev = decode[view.EventPage](t, f.h.do(get(events+"/meet", f.sess)))
	if n := f.srv.CountRequests("REPORT", f.col) - reports; n != 1 {
		t.Errorf("the record page made %d REPORTs; related records come from the loaded list", n)
	}
	if len(ev.Related) != 2 || ev.Related[0].Href != tasks+"/task-1" || ev.Related[0].Kind != view.KindTask ||
		ev.Related[0].Title != "Prepare slides" || ev.Related[1].Href != "" || ev.Related[1].Title != "elsewhere" {
		t.Errorf("with the task list loaded, related = %+v", ev.Related)
	}

	_ = decode[view.AgendaPage](t, f.h.do(get(events+"?from=2026-01-12", f.sess)))
	task := decode[view.TaskPage](t, f.h.do(get(tasks+"/task-1", f.sess)))
	if len(task.Related) != 1 || task.Related[0].Href != events+"/meet" || task.Related[0].Why != "Refers to this" {
		t.Errorf("reverse link = %+v", task.Related)
	}
}

// S-7: the patch touches what the form changed and nothing else; a kept row
// keeps its parameters; foreign properties survive.
func TestEditContactWritesOnlyWhatChanged(t *testing.T) {
	f := contactsFixture(t, false,
		"BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nEMAIL;TYPE=work:ada@old.example\nEMAIL:second@example.com\nTEL:+44 20 1\nX-CUSTOM:keep\nEND:VCARD\n")
	rec := f.path("contacts") + "/ada"

	edit := decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
	form := formFields(edit.Form)
	if form.Get("etag") == "" || form.Get("email.0") != "ada@old.example" || form.Get("email.1") != "second@example.com" {
		t.Fatalf("form = %v", form)
	}
	form.Set("fn", "Ada King")
	form.Set("email.0", "ada@new.example")
	resp := f.h.do(postForm(rec+"/edit", f.sess, form))
	// An address replaced is asked about (S-15.4); nothing names it here,
	// so the question comes back to the card at once.
	if resp.Code != http.StatusSeeOther || !strings.HasPrefix(resp.Header().Get("Location"), rec+"/rewrite?plan=") {
		t.Fatalf("POST: %d %s %s", resp.Code, resp.Header().Get("Location"), resp.Body.String())
	}
	if next := f.h.do(get(resp.Header().Get("Location"), f.sess)); next.Code != http.StatusSeeOther || next.Header().Get("Location") != rec {
		t.Fatalf("the question with nothing to ask: %d %s", next.Code, next.Header().Get("Location"))
	}
	got := body(t, f, "ada.vcf")
	mustContain(t, got, "FN:Ada King", "EMAIL;TYPE=work:ada@new.example", "EMAIL:second@example.com", "TEL:+44 20 1", "X-CUSTOM:keep", "UID:ada")
	if strings.Contains(got, "ada@old.example") {
		t.Errorf("old address still there:\n%s", got)
	}

	// Saving an unchanged form writes nothing.
	puts := f.srv.CountRequests("PUT", f.col)
	edit = decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
	if resp := f.h.do(postForm(rec+"/edit", f.sess, formFields(edit.Form))); resp.Code != http.StatusSeeOther {
		t.Fatalf("unchanged POST: %d", resp.Code)
	}
	if n := f.srv.CountRequests("PUT", f.col) - puts; n != 0 {
		t.Errorf("an unchanged form sent %d PUT", n)
	}
}

// U-6.16: the way in from the shell. The create menu points at it; the first
// time there is nothing to go by, so the sheet asks where and shows no fields;
// once a record has been made somewhere, the next one starts there — the
// person's own last choice, remembered, not guessed.
func TestQuickSheetFromTheShellRemembersWhereRecordsGo(t *testing.T) {
	f := contactsFixture(t, false)
	entry := "/app/contacts/quick"

	page := decode[view.ContactsListPage](t, f.h.do(get(f.path("contacts"), f.sess)))
	if href := actionHref(t, menuActions(page.Top.New.Arrow), "new.contacts"); href != entry {
		t.Errorf("the create menu points at %q, want %q", href, entry)
	}

	// Nothing made yet: the sheet asks where, and carries no fields.
	sheet := decode[view.NewSheet](t, f.h.do(get(entry, f.sess)))
	if len(sheet.Form.Sets) != 0 {
		t.Errorf("the sheet offered fields before a collection was chosen: %+v", sheet.Form)
	}
	if len(sheet.Target.Options) == 0 {
		t.Fatalf("the sheet asks where, but offers nothing: %+v", sheet.Target)
	}

	// Make one through the sheet of the collection itself.
	quick := f.path("contacts") + "/new/quick"
	form := formFields(decode[view.NewSheet](t, f.h.do(get(quick, f.sess))).Form)
	form.Set("fn", "Ada Lovelace")
	if resp := f.h.do(postForm(quick, f.sess, form)); resp.Code != http.StatusSeeOther {
		t.Fatalf("save: %d %s", resp.Code, resp.Body.String())
	}

	// Now the way in goes straight there.
	resp := f.h.do(get(entry, f.sess))
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != quick {
		t.Fatalf("the shell entry went to %d %q, want %q", resp.Code, resp.Header().Get("Location"), quick)
	}
}

// menuActions is the actions of a menu, for actionHref.
func menuActions(items []view.MenuItem) []view.Action {
	out := make([]view.Action, 0, len(items))
	for _, it := range items {
		out = append(out, it.Action)
	}
	return out
}

// U-6.16: the quick sheet holds a few of the full form's own fields, saves on
// Save, and only then. "Open the card" carries what was typed into the full
// form at /new and writes nothing: until Save there is no record, so changing
// your mind after opening the card leaves nothing behind.
func TestQuickSheetSavesOnlyOnSave(t *testing.T) {
	f := contactsFixture(t, false)
	quick := f.path("contacts") + "/new/quick"

	sheet := decode[view.NewSheet](t, f.h.do(get(quick, f.sess)))
	if len(sheet.Form.Sets) == 0 {
		t.Fatalf("the sheet carries no form: %+v", sheet)
	}
	for _, name := range []string{"fn", "tel.0", "email.0"} {
		rowNamed(t, sheet.Form, name)
	}
	// "Open the card" carries its own address, so the browser posts the same
	// form there: what was typed travels in the body, never in the address.
	if href := actionHref(t, sheet.Form.Actions, "record.open"); !strings.HasSuffix(href, "/new/quick?op=open") {
		t.Errorf("Open the card goes to %q", href)
	}

	form := formFields(sheet.Form)
	form.Set("fn", "Ada Lovelace")
	puts := f.srv.CountRequests("PUT", f.col)

	edit := decode[view.ContactEditPage](t, f.h.do(postForm(quick+"?op=open", f.sess, form)))
	if got := string(rowNamed(t, edit.Form, "fn").Value); got != "Ada Lovelace" {
		t.Errorf("the full form opened with %q, want what was typed in the sheet", got)
	}
	if n := f.srv.CountRequests("PUT", f.col) - puts; n != 0 {
		t.Errorf("opening the card wrote %d record(s); until Save there is none", n)
	}

	if resp := f.h.do(postForm(quick, f.sess, form)); resp.Code != http.StatusSeeOther {
		t.Fatalf("save from the sheet: %d %s", resp.Code, resp.Body.String())
	}
	if n := f.srv.CountRequests("PUT", f.col) - puts; n != 1 {
		t.Errorf("Save wrote %d records, want one", n)
	}
}

// U-6.1: the contact editor holds three name fields — Display name, Given and
// Family. Given and Family are the two components of N a person types; the
// three the form does not offer — middle name, prefix, suffix — come back
// untouched (S-7). An empty display name is assembled from the two on save.
// Without N a server that builds FN from it leaves the contact with no name at
// all (docs/servers.md, SV-21).
func TestContactNameFieldsWriteN(t *testing.T) {
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nN:Lovelace;Ada;Byron;;Countess\nEND:VCARD\n")
	rec := f.path("contacts") + "/ada"

	edit := decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
	if got := string(rowNamed(t, edit.Form, "n.given").Value); got != "Ada" {
		t.Errorf("Given = %q, want the given name out of N", got)
	}
	if got := string(rowNamed(t, edit.Form, "n.family").Value); got != "Lovelace" {
		t.Errorf("Family = %q, want the family name out of N", got)
	}

	form := formFields(edit.Form)
	form.Set("n.family", "King")
	if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != http.StatusSeeOther {
		t.Fatalf("save: %d %s", resp.Code, resp.Body.String())
	}
	// Byron and Countess are nobody's to drop: no field offers them.
	mustContain(t, body(t, f, "ada.vcf"), "N:King;Ada;Byron;;Countess")

	form = formFields(decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form)
	form.Set("fn", "")
	form.Set("n.given", "Augusta")
	if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != http.StatusSeeOther {
		t.Fatalf("save without a display name: %d %s", resp.Code, resp.Body.String())
	}
	mustContain(t, body(t, f, "ada.vcf"), "FN:Augusta King", "N:King;Augusta;Byron;;Countess")

	// The other way round (U-6.16): one display name and both parts emptied —
	// it is cut left to right, and the components no field offers stay put.
	// The two rules must not undo each other: what was assembled a moment ago
	// is not re-cut, and what is cut here is not assembled back.
	form = formFields(decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form)
	form.Set("fn", "Grace Hopper")
	form.Set("n.given", "")
	form.Set("n.family", "")
	if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != http.StatusSeeOther {
		t.Fatalf("save with a display name alone: %d %s", resp.Code, resp.Body.String())
	}
	mustContain(t, body(t, f, "ada.vcf"), "FN:Grace Hopper", "N:Hopper;Grace;Byron;;Countess")
}

// A card another client wrote without N keeps having none when the person
// edits something else entirely: the cut happens on a changed display name,
// not on every save. What nobody touched is not written (S-7, U-6.1).
func TestEditingAnotherFieldDoesNotGrowAName(t *testing.T) {
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nEND:VCARD\n")
	rec := f.path("contacts") + "/ada"

	form := formFields(decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form)
	form.Set("title", "Countess")
	if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != http.StatusSeeOther {
		t.Fatalf("save: %d %s", resp.Code, resp.Body.String())
	}
	got := body(t, f, "ada.vcf")
	mustContain(t, got, "TITLE:Countess", "FN:Ada Lovelace")
	// A property, not a substring: "N:" lives inside BEGIN: and VERSION: too.
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "N:") {
			t.Errorf("editing the title wrote a name nobody entered:\n%s", got)
			break
		}
	}
}

// S-8: a write against a version that moved on is not sent over it. The
// screen lists only what the person changed and the server holds otherwise;
// the chosen combination lands on the server version with its If-Match.
func TestConflictScreenAndCombination(t *testing.T) {
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nNOTE:first note\nEND:VCARD\n")
	rec := f.path("contacts") + "/ada"

	form := formFields(decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form)
	f.srv.Seed(f.col+"ada.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Byron\nNOTE:server note\nEND:VCARD\n")
	form.Set("fn", "Ada King")
	form.Set("title", "Countess")
	resp := f.h.do(postForm(rec+"/edit", f.sess, form))
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != rec+"/conflict" {
		t.Fatalf("POST over a moved version: %d %q", resp.Code, resp.Header().Get("Location"))
	}
	mustContain(t, body(t, f, "ada.vcf"), "FN:Ada Byron", "NOTE:server note")

	page := decode[view.ConflictPage](t, f.h.do(get(rec+"/conflict", f.sess)))
	// The name was changed, so N is part of that change and the screen says so
	// (U-6.1): the card on the server carries none, and this one now would.
	want := []view.ConflictRow{
		{Property: "FN", Mine: "Ada King", Theirs: "Ada Byron", Choice: view.ChoiceUnset},
		{Property: "N", Mine: "Ada, King", Theirs: "Empty", Choice: view.ChoiceUnset},
		{Property: "TITLE", Mine: "Countess", Theirs: "Empty", Choice: view.ChoiceUnset},
	}
	if len(page.Rows) != len(want) || page.Rows[0] != want[0] || page.Rows[1] != want[1] || page.Rows[2] != want[2] {
		t.Fatalf("rows = %+v", page.Rows)
	}
	if page.ReadAt == "" || page.Kind != view.KindContact {
		t.Errorf("page = %+v", page)
	}
	// C-6.3: the conflict screen is left for the list; it is not printed.
	if nav := page.Rail.Nav; len(nav) != 1 || nav[0].ID != "nav.back" || nav[0].Href != f.path("contacts") || page.Print != (view.Print{}) {
		t.Errorf("conflict rail %+v, print %+v", nav, page.Print)
	}

	resp = f.h.do(postForm(actionHref(t, page.Actions, "conflict.combine"), f.sess, url.Values{"choice.0": {"theirs"}}))
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != rec {
		t.Fatalf("combine: %d %q %s", resp.Code, resp.Header().Get("Location"), resp.Body.String())
	}
	mustContain(t, body(t, f, "ada.vcf"), "FN:Ada Byron", "TITLE:Countess", "NOTE:server note")
	if resp := f.h.do(get(rec+"/conflict", f.sess)); resp.Code != http.StatusSeeOther {
		t.Errorf("the draft outlived the written combination: %d", resp.Code)
	}
}

// S-11: reminders are the VALARM set of the record. A save that does not
// touch them leaves the other client's alarm as it was; a changed set is
// rewritten; a trigger the form did not offer is refused.
func TestRemindersKeepUntouchedAlarms(t *testing.T) {
	f := calendarFixture(t, nil, "meet.ics", ics("BEGIN:VEVENT\nUID:meet\nDTSTAMP:20260101T000000Z\nDTSTART:20260115T100000Z\n"+
		"SUMMARY:Review\nBEGIN:VALARM\nACTION:AUDIO\nTRIGGER:-PT10M\nX-KEEP:yes\nEND:VALARM\nEND:VEVENT"))
	rec := f.path("calendar") + "/meet"
	editForm := func() url.Values {
		return formFields(decode[view.EventEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form)
	}

	form := editForm()
	if form.Get("reminder.0") != "-PT10M" || form.Get("start.date") != "2026-01-15" || form.Get("start.time") != "10:00" {
		t.Fatalf("form = %v", form)
	}
	form.Set("summary", "Review, again")
	if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != http.StatusSeeOther {
		t.Fatalf("POST: %d %s", resp.Code, resp.Body.String())
	}
	mustContain(t, body(t, f, "meet.ics"), `SUMMARY:Review\, again`, "X-KEEP:yes", "ACTION:AUDIO", "TRIGGER:-PT10M", "DTSTART:20260115T100000Z")

	form = editForm()
	form.Set("reminder.0", "")
	form.Set("reminder.1", "-PT1H")
	if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != http.StatusSeeOther {
		t.Fatalf("POST: %d %s", resp.Code, resp.Body.String())
	}
	got := body(t, f, "meet.ics")
	mustContain(t, got, "TRIGGER:-PT1H")
	if strings.Contains(got, "TRIGGER:-PT10M") || strings.Contains(got, "X-KEEP") {
		t.Errorf("the removed alarm is still there:\n%s", got)
	}

	form = editForm()
	form.Set("reminder.1", "-PT3H")
	page := decode[view.EventEditPage](t, f.h.do(postForm(rec+"/edit", f.sess, form)))
	if len(page.Form.Errors) == 0 || page.Form.Errors[0].Name != "reminder" {
		t.Errorf("errors = %+v", page.Form.Errors)
	}
	if body(t, f, "meet.ics") != got {
		t.Error("a refused form changed the server")
	}
}

func TestDeleteNeedsTheVersionShown(t *testing.T) {
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:bob\nFN:Bob\nEND:VCARD\n")
	list := f.path("contacts")

	del := actionHref(t, decode[view.ContactPage](t, f.h.do(get(list+"/bob", f.sess))).Actions, "record.delete")
	f.srv.Seed(f.col+"bob.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:bob\nFN:Robert\nEND:VCARD\n")
	page := decode[view.ContactPage](t, f.h.do(postForm(del, f.sess, url.Values{})))
	if page.Name != "Robert" || !hasNotice(page.Notices, "Changed elsewhere, so nothing was deleted") {
		t.Errorf("stale delete page = %+v", page)
	}
	body(t, f, "bob.vcf")

	del = actionHref(t, page.Actions, "record.delete")
	resp := f.h.do(postForm(del, f.sess, url.Values{}))
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != list {
		t.Fatalf("delete: %d %q", resp.Code, resp.Header().Get("Location"))
	}
	if _, _, ok := f.srv.Object(f.col + "bob.vcf"); ok {
		t.Error("still on the server")
	}
}

func TestNewEventIsSkeletonPlusPatch(t *testing.T) {
	f := calendarFixture(t, nil)
	events := f.path("calendar")

	page := decode[view.EventEditPage](t, f.h.do(get(events+"/new", f.sess)))
	form := formFields(page.Form)
	form.Set("summary", "No date")
	refused := decode[view.EventEditPage](t, f.h.do(postForm(events+"/new", f.sess, form)))
	if len(refused.Form.Errors) == 0 || refused.Form.Errors[0].Name != "start" || len(f.srv.Paths(f.col)) != 0 {
		t.Fatalf("an event without a start: errors %+v, stored %v", refused.Form.Errors, f.srv.Paths(f.col))
	}

	form.Set("summary", "Dentist")
	form.Set("start.date", "2026-03-02")
	form.Set("start.time", "09:30")
	form.Set("reminder.0", "-PT1H")
	resp := f.h.do(postForm(events+"/new", f.sess, form))
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("POST: %d %s", resp.Code, resp.Body.String())
	}
	m := regexp.MustCompile(`^` + regexp.QuoteMeta(events) + `/([0-9a-f]{32})$`).FindStringSubmatch(resp.Header().Get("Location"))
	if m == nil {
		t.Fatalf("Location = %q, want the record under a 32-hex UID", resp.Header().Get("Location"))
	}
	mustContain(t, body(t, f, m[1]+".ics"), "UID:"+m[1], "SUMMARY:Dentist", "DTSTART:20260302T093000Z", "TRIGGER:-PT1H", "DTSTAMP:", "PRODID:")

	sheet := f.h.do(fragment(get(events+"/new", f.sess), "sheet"))
	var payload struct {
		Target string        `json:"target"`
		Body   view.NewSheet `json:"body"`
	}
	if err := json.Unmarshal(sheet.Body.Bytes(), &payload); err != nil || payload.Target != "sheet" ||
		payload.Body.Kind != view.KindEvent || len(payload.Body.Target.Options) != 1 {
		t.Errorf("new sheet = %+v (%v)", payload, err)
	}
}

// docs/servers.md, SV-25: a server that writes its own UID over the one a new
// card was sent with. The record is read back at the address it was written
// to, the identity found there becomes the record's, and the person lands on
// it — the UID Carrel gave was never theirs to lose, so it is no loss (S-7.1).
func TestCreateWhereTheServerRewritesTheUID(t *testing.T) {
	f := contactsFixture(t, false)
	f.srv.CreateRewritesUID = true
	list := f.path("contacts")

	form := formFields(decode[view.ContactEditPage](t, f.h.do(get(list+"/new", f.sess))).Form)
	form.Set("fn", "Ada Lovelace")
	resp := f.h.do(postForm(list+"/new", f.sess, form))
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("POST: %d %s", resp.Code, resp.Body.String())
	}
	loc := resp.Header().Get("Location")
	if !strings.HasPrefix(loc, list+"/sv25-") {
		t.Fatalf("Location = %q, want the record under the UID the server kept", loc)
	}
	page := decode[view.ContactPage](t, f.h.do(get(loc, f.sess)))
	if page.Name != "Ada Lovelace" || len(page.Notices) != 0 {
		t.Errorf("the card at %s: name %q, notices %+v", loc, page.Name, page.Notices)
	}
}

// The same server, and nothing to be found under either identity or address:
// the truth and a way on, not a redirect into nothing (S-7.1).
func TestCreateWhereTheServerShowsNothing(t *testing.T) {
	f := contactsFixture(t, false)
	f.srv.CreateHidesTheRecord = true
	list := f.path("contacts")

	form := formFields(decode[view.ContactEditPage](t, f.h.do(get(list+"/new", f.sess))).Form)
	form.Set("fn", "Ada Lovelace")
	page := decode[view.ContactPage](t, f.h.do(postForm(list+"/new", f.sess, form)))
	if !hasNotice(page.Notices, "Saved, but the server does not show it") {
		t.Fatalf("notices = %+v", page.Notices)
	}
	var opens string
	for _, n := range page.Notices {
		for _, a := range n.Actions {
			if a.ID == "collection.open" {
				opens = a.Href
			}
		}
	}
	if opens != list {
		t.Errorf("the notice leads to %q, want the collection", opens)
	}
}

// S-7.1: a server that drops a property is named after the write, once per
// server and property.
func TestLossIsReportedOncePerServer(t *testing.T) {
	f := contactsFixture(t, false,
		"BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada\nX-CUSTOM:keep\nEND:VCARD\n",
		"BEGIN:VCARD\nVERSION:4.0\nUID:bob\nFN:Bob\nX-CUSTOM:keep\nEND:VCARD\n")
	f.srv.Drop = []string{"X-CUSTOM"}
	list := f.path("contacts")

	for i, uid := range []string{"ada", "bob"} {
		form := formFields(decode[view.ContactEditPage](t, f.h.do(get(list+"/"+uid+"/edit", f.sess))).Form)
		form.Set("fn", "Renamed")
		page := decode[view.ContactEditPage](t, f.h.do(postForm(list+"/"+uid+"/edit", f.sess, form)))
		if page.Loss == nil || len(page.Loss.Properties) != 1 || page.Loss.Properties[0] != "X-CUSTOM" {
			t.Fatalf("%s: loss = %+v", uid, page.Loss)
		}
		if page.Loss.Aggregated != (i == 1) {
			t.Errorf("%s: aggregated = %v, want %v", uid, page.Loss.Aggregated, i == 1)
		}
	}
}

// C-9.1, S-21: a source that refuses is a screen with the answer and a retry
// of that source, never a 500.
func TestUnavailableSourceIsAScreen(t *testing.T) {
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada\nEND:VCARD\n")
	f.srv.Password = "changed on the server"
	list := f.path("contacts")

	page := decode[view.ContactsListPage](t, f.h.do(get(list, f.sess)))
	if len(page.Groups) != 0 || len(page.Notices) == 0 || page.Notices[0].Details == nil {
		t.Fatalf("page = %+v", page)
	}
	if !strings.Contains(strings.Join(func() []string {
		var l []string
		for _, line := range page.Notices[0].Details.Lines {
			l = append(l, string(line))
		}
		return l
	}(), " "), "401") {
		t.Errorf("details = %+v, want the server's answer", page.Notices[0].Details)
	}
	if actionHref(t, page.Notices[0].Actions, "source.retry") != list {
		t.Error("retry is not this source")
	}
	card := decode[view.ContactPage](t, f.h.do(get(list+"/ada", f.sess)))
	if len(card.Notices) == 0 || len(card.Actions) != 0 {
		t.Errorf("record page = %+v", card)
	}
}

func TestReadOnlyCollectionHasNoEditing(t *testing.T) {
	f := contactsFixture(t, true, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada\nEND:VCARD\n")
	list := f.path("contacts")

	card := decode[view.ContactPage](t, f.h.do(get(list+"/ada", f.sess)))
	if got := ids(menuActions(card.Menu)); !card.ReadOnly || len(card.Actions) != 0 || !equalStrings(got, []string{"record.copy", "record.export", "publish"}) {
		t.Errorf("card: read-only %v, actions %v, menu %v", card.ReadOnly, ids(card.Actions), got)
	}
	// New is closed with its reason and the wizard that would write here is
	// not offered at all; Export is, because reading is what read-only
	// allows (the collection is published from Connections).
	page := decode[view.ContactsListPage](t, f.h.do(get(list, f.sess)))
	if len(page.Bar.Actions) != 2 || page.Bar.Actions[0].Enabled || page.Bar.Actions[0].Reason == "" {
		t.Errorf("new action = %+v", page.Bar.Actions)
	}
	if page.Bar.Actions[1].ID != "collection.export" {
		t.Errorf("bar = %+v", page.Bar.Actions)
	}
	before := body(t, f, "ada.vcf")
	for _, req := range []*http.Request{
		get(list+"/ada/edit", f.sess),
		postForm(list+"/ada/edit", f.sess, url.Values{"fn": {"X"}, "etag": {"e1"}}),
		postForm(list+"/ada/delete", f.sess, url.Values{"etag": {"e1"}}),
		postForm(list+"/new", f.sess, url.Values{"fn": {"X"}}),
	} {
		if resp := f.h.do(req); resp.Code != http.StatusNotFound {
			t.Errorf("%s %s: %d, want 404", req.Method, req.URL.Path, resp.Code)
		}
	}
	if body(t, f, "ada.vcf") != before {
		t.Error("a read-only collection was written")
	}
}

func TestTasksAndNotes(t *testing.T) {
	f := calendarFixture(t, nil,
		"a.ics", ics("BEGIN:VTODO\nUID:open-1\nDTSTAMP:20260101T000000Z\nSUMMARY:Write report\nDUE:20260110T120000Z\nEND:VTODO"),
		"b.ics", ics("BEGIN:VTODO\nUID:done-1\nDTSTAMP:20260101T000000Z\nSUMMARY:Old\nSTATUS:COMPLETED\nEND:VTODO"),
		"n.ics", ics("BEGIN:VJOURNAL\nUID:note-1\nDTSTAMP:20260101T000000Z\nDTSTART;VALUE=DATE:20260105\nSUMMARY:Ideas\n"+
			"DESCRIPTION:# Ideas\\n\\nFirst **bold** line\nEND:VJOURNAL"),
	)
	tasks, notes := f.path("tasks"), f.path("notes")
	// A row says its day the short way of a list, without the year of
	// this year (C-10).
	f.h.srv.SetClock(time.Date(2026, 1, 20, 9, 0, 0, 0, time.UTC))

	open := decode[view.TasksPage](t, f.h.do(get(tasks, f.sess)))
	if len(open.Groups) != 1 || len(open.Groups[0].Rows) != 1 {
		t.Fatalf("open tasks = %+v", open.Groups)
	}
	row := open.Groups[0].Rows[0]
	if row.Summary != "Write report" || row.Done || !row.Overdue || row.Due != "10 Jan 12:00" {
		t.Errorf("row = %+v", row)
	}
	var segments []string
	for _, s := range open.Bar.Segments {
		segments = append(segments, string(s.Label))
	}
	// The orders of the list stand beside the segments (review Р-12).
	if strings.Join(segments, ",") != "Open 1,Done 1,All 2,By due,By priority,Recently changed" {
		t.Errorf("segments = %v", segments)
	}
	sameSegments(t, "tasks", segmentsOf(open.Bar.Segments, "tasks."), "s-tasks-1", 0)
	sameSegments(t, "task orders", segmentsOf(open.Bar.Segments, "sort."), "s-tasks-1", 1)
	done := decode[view.TasksPage](t, f.h.do(get(tasks+"?show=done", f.sess)))
	if len(done.Groups) != 1 || !done.Groups[0].Rows[0].Done || done.Groups[0].Rows[0].Summary != "Old" {
		t.Errorf("done tasks = %+v", done.Groups)
	}

	list := decode[view.NotesPage](t, f.h.do(get(notes, f.sess)))
	if len(list.Groups) != 1 || list.Groups[0].Rows[0].Title != "Ideas" ||
		list.Groups[0].Rows[0].Excerpt != "First **bold** line" || list.Groups[0].Rows[0].Date != "5 Jan" {
		t.Errorf("notes = %+v", list.Groups)
	}
	note := decode[view.NotePage](t, f.h.do(get(notes+"/note-1", f.sess)))
	if len(note.Doc.Rendered) == 0 || note.Doc.Rendered[0].Kind != view.BlockHeading || note.Doc.Body != "# Ideas\n\nFirst **bold** line" {
		t.Errorf("note doc = %+v", note.Doc)
	}
	actionHref(t, note.Doc.Bar, "record.edit")
	actionHref(t, note.Meta.Actions, "record.delete")

	form := formFields(decode[view.TaskEditPage](t, f.h.do(get(tasks+"/open-1/edit", f.sess))).Form)
	form.Set("done", "on")
	if resp := f.h.do(postForm(tasks+"/open-1/edit", f.sess, form)); resp.Code != http.StatusSeeOther {
		t.Fatalf("POST: %d %s", resp.Code, resp.Body.String())
	}
	mustContain(t, body(t, f, "a.ics"), "STATUS:COMPLETED", "COMPLETED:", "DUE:20260110T120000Z")
}

// The pool question, answered with numbers instead of reasoning
// (docs/plan/server.md, «Пул соединений»). Guard.httpClient builds a
// transport for every dav.Client, so a client's first request always dials
// and only what follows it on that same client can come off the pool. The
// rule that a dropped write is sent again applies to the second kind only,
// so how much that rule is worth depends entirely on the mix.
//
// This walks an ordinary visit — look at a list, open a card, make a
// record, edit it, delete it — and prints what the guard did. It asserts
// only what must not silently change: that writes happen and that the
// counters move. The numbers are for the decision, and they are in the log.
func TestWhatThePoolActuallyDoes(t *testing.T) {
	f := contactsFixture(t, false,
		"BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nEMAIL:ada@example.com\nEND:VCARD\n")
	list := f.path("contacts")

	f.h.do(get(list, f.sess))
	f.h.do(get(list+"/ada", f.sess))
	form := formFields(decode[view.ContactEditPage](t, f.h.do(get(list+"/new", f.sess))).Form)
	form.Set("fn", "Grace Hopper")
	made := f.h.do(postForm(list+"/new", f.sess, form))
	if made.Code != http.StatusSeeOther {
		t.Fatalf("new: %d %s", made.Code, made.Body.String())
	}
	at := made.Header().Get("Location")
	edit := formFields(decode[view.ContactEditPage](t, f.h.do(get(at+"/edit", f.sess))).Form)
	edit.Set("fn", "Grace M. Hopper")
	f.h.do(postForm(at+"/edit", f.sess, edit))
	f.h.do(get(list, f.sess))

	clients, dialled, requests, writes, fromIdle, resent := f.h.srv.DAVCounts()
	t.Logf("pool over one visit: %d clients built, %d connections dialled, %d requests, %d of them writes, %d of those off an idle connection, %d sent again",
		clients, dialled, requests, writes, fromIdle, resent)
	if writes == 0 || requests == 0 {
		t.Fatalf("the visit wrote nothing: %d requests, %d writes", requests, writes)
	}
	if dialled == 0 {
		t.Error("no connection was counted: the tally is not wired to the dialler")
	}
}
