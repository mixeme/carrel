package web_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Р21 of scans/kinds-parity.md (the rule of 12.10.2026 «kinds are equal»):
// two tasks found duplicates that hold different places are asked which
// place stays, as two events are.
func TestDuplicateTasksAreAskedTheirPlace(t *testing.T) {
	f := calendarFixture(t, nil,
		"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Pay rent\nDUE;VALUE=DATE:20260815\nLOCATION:Bank\nEND:VTODO"),
		"t2.ics", ics("BEGIN:VTODO\nUID:t2\nDTSTAMP:20260101T000000Z\nSUMMARY:Pay rent\nDUE;VALUE=DATE:20260815\nLOCATION:Online\nEND:VTODO"))
	f.h.do(get(f.path("tasks"), f.sess))
	page := decode[view.DuplicatesPage](t, f.h.do(get("/app/duplicates", f.sess)))
	if len(page.Groups) != 1 {
		t.Fatalf("groups = %+v", page.Groups)
	}
	var props []string
	for _, c := range page.Groups[0].Choices {
		props = append(props, c.Name)
	}
	if !strings.Contains(strings.Join(props, ","), "choice.LOCATION") {
		t.Errorf("choices = %v, want the place asked", props)
	}
}

// Note 69 (task 168): the owner took New task off the cards — a task is
// made in the tasks. Р15 of scans/kinds-parity.md had given it to every
// kind; equal now is that none of the cards has it.
func TestNoCardHasNewTask(t *testing.T) {
	f := calendarFixture(t, nil,
		"n1.ics", ics("BEGIN:VJOURNAL\nUID:n1\nDTSTAMP:20260101T000000Z\nSUMMARY:Plan\nEND:VJOURNAL"),
		"e1.ics", ics("BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260101T000000Z\nDTSTART:20260115T100000Z\nSUMMARY:Review\nEND:VEVENT"))
	note := decode[view.NotePage](t, f.h.do(get(f.path("notes")+"/n1", f.sess)))
	event := decode[view.EventPage](t, f.h.do(get(f.path("calendar")+"/e1", f.sess)))
	for _, a := range append(append([]view.Action(nil), note.Doc.Bar...), event.Actions...) {
		if a.Label == "New task" {
			t.Errorf("a card has New task: %+v", a)
		}
	}
}

// Р17 of scans/kinds-parity.md: what Carrel does not know of a task is
// shown in its form as it came, as an event's is — it was shown nowhere.
func TestATaskFormShowsWhatIsKeptAsSent(t *testing.T) {
	f := calendarFixture(t, nil, "t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Slides\nPRIORITY:1\nX-OC-HIDESUBTASKS:1\nCLASS:PRIVATE\nEND:VTODO"))
	page := decode[view.TaskEditPage](t, f.h.do(get(f.path("tasks")+"/t1/edit", f.sess)))
	var names []string
	for _, fl := range page.Foreign {
		names = append(names, string(fl.Label)+"="+string(fl.Value))
	}
	if strings.Join(names, ",") != "CLASS=PRIVATE,X-OC-HIDESUBTASKS=1" {
		t.Errorf("kept as sent = %v", names)
	}
}

// Р18 of scans/kinds-parity.md: the form of an event says where the event
// is and when it was read, as the task's form does.
func TestAnEventFormHasItsSource(t *testing.T) {
	f := calendarFixture(t, nil, "e1.ics", ics("BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260101T000000Z\nDTSTART:20260115T100000Z\nSUMMARY:Review\nEND:VEVENT"))
	page := decode[view.EventEditPage](t, f.h.do(get(f.path("calendar")+"/e1/edit", f.sess)))
	if !strings.HasSuffix(string(page.Source.Path), "e1.ics") || page.Source.Stamp == "" {
		t.Errorf("source = %+v", page.Source)
	}
}

// Р10 of scans/kinds-parity.md: a task's list offers Where, as the agenda
// does, and a row carries the task's LOCATION.
func TestTasksHaveAWhereColumn(t *testing.T) {
	f := calendarFixture(t, nil, "t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Slides\nLOCATION:Room 4\nEND:VTODO"))
	page := decode[view.TasksPage](t, f.h.do(get(f.path("tasks")+"?seg=all", f.sess)))
	offered := false
	for _, it := range page.Bar.Columns.Items {
		offered = offered || it.ID == "where"
	}
	if !offered || len(page.Groups) == 0 || page.Groups[0].Rows[0].Where != "Room 4" {
		t.Errorf("columns %+v, groups %+v", page.Bar.Columns.Items, page.Groups)
	}
}

// Р20 of scans/kinds-parity.md: an event and a task hand over one record
// whole, as .ics; a note gave only its text, as Markdown, and its other
// properties had no way out. Its menu now has «Export .ics» too: the
// journal as the server holds it.
func TestANoteExportsAsCalendar(t *testing.T) {
	f := calendarFixture(t, nil, "n1.ics", ics("BEGIN:VJOURNAL\nUID:n1\nDTSTAMP:20260101T000000Z\nSUMMARY:Plan\nDESCRIPTION:Steps\nX-KEEP:1\nEND:VJOURNAL"))
	page := decode[view.NotePage](t, f.h.do(get(f.path("notes")+"/n1", f.sess)))
	var href string
	for _, a := range menuActions(page.Meta.Menu) {
		if string(a.Label) == "Export .ics" {
			href = a.Href
		}
	}
	if href == "" {
		t.Fatalf("menu = %+v, want Export .ics", menuActions(page.Meta.Menu))
	}
	resp := f.h.do(get(href, f.sess))
	got := strings.ReplaceAll(resp.Body.String(), "\r\n", "\n")
	if resp.Code != 200 || !strings.HasPrefix(resp.Header().Get("Content-Type"), "text/calendar") ||
		!strings.Contains(resp.Header().Get("Content-Disposition"), ".ics") ||
		!strings.Contains(got, "BEGIN:VJOURNAL") || !strings.Contains(got, "X-KEEP:1") {
		t.Errorf("%d %v\n%s", resp.Code, resp.Header(), got)
	}
	// The bar's Export Markdown stays what it was.
	if a := actionByID(page.Doc.Bar, "note.export"); a == nil || strings.Contains(a.Href, "as=ics") {
		t.Errorf("Export Markdown = %+v", a)
	}
}

// Р3 of scans/kinds-parity.md: an event's people are edited and shown; a
// task's and a note's were neither, though VTODO and VJOURNAL hold ATTENDEE
// as VEVENT does (RFC 5545 §3.6.2, §3.6.3) — the task's card named its
// organiser in a field, and ATTENDEE was kept as foreign. Now a task's and a
// note's edit carry the field of attendees, their cards name the people,
// the organiser first, as an event's, and a form that does not draw the
// field leaves them be.
func TestTasksAndNotesHaveAttendees(t *testing.T) {
	f := calendarFixture(t, nil,
		"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Slides\nATTENDEE;CN=Bob;PARTSTAT=ACCEPTED:mailto:bob@example.org\nX-KEEP:1\nEND:VTODO"),
		"t2.ics", ics("BEGIN:VTODO\nUID:t2\nDTSTAMP:20260101T000000Z\nSUMMARY:Review\nORGANIZER;CN=Anna:mailto:anna@example.org\nATTENDEE:mailto:bob@example.org\nEND:VTODO"),
		"n1.ics", ics("BEGIN:VJOURNAL\nUID:n1\nDTSTAMP:20260101T000000Z\nSUMMARY:Minutes\nATTENDEE;CN=Bob:mailto:bob@example.org\nX-KEEP:1\nEND:VJOURNAL"))
	people := func(rows []view.AttendeeRow) string {
		var out []string
		for _, r := range rows {
			out = append(out, string(r.Name)+"/"+string(r.Status))
		}
		return strings.Join(out, ",")
	}
	tasks := f.path("tasks")

	card := decode[view.TaskPage](t, f.h.do(get(tasks+"/t1", f.sess)))
	if got := people(card.Attendees); got != "Bob/accepted" {
		t.Errorf("the task's card names %q", got)
	}
	other := decode[view.TaskPage](t, f.h.do(get(tasks+"/t2", f.sess)))
	if got := people(other.Attendees); got != "Anna/organiser,bob@example.org/no reply" {
		t.Errorf("an organised task's card names %q", got)
	}
	for _, fl := range other.Fields {
		if fl.Label == "Organiser" {
			t.Errorf("the organiser is a field still, as well as the first of the people: %+v", other.Fields)
		}
	}
	edit := decode[view.TaskEditPage](t, f.h.do(get(tasks+"/t1/edit", f.sess)))
	if got := pageTags(edit.Attendees); !slices.Equal(got, []string{"attendee=mailto:bob@example.org"}) || edit.Attendees.Add == nil {
		t.Errorf("the task's field = %q, %+v", got, edit.Attendees)
	}
	for _, fl := range edit.Foreign {
		if fl.Label == "ATTENDEE" {
			t.Errorf("ATTENDEE is kept as foreign: %+v", edit.Foreign)
		}
	}
	form := formFields(edit.Form)
	if resp := f.h.do(postForm(tasks+"/t1/edit", f.sess, form)); resp.Code != 303 {
		t.Fatalf("POST without the field: %d %s", resp.Code, resp.Body.String())
	}
	if got := unfolded(body(t, f, "t1.ics")); !strings.Contains(got, "mailto:bob@example.org") {
		t.Fatalf("a form without the field took Bob away:\n%s", got)
	}
	form = formFields(decode[view.TaskEditPage](t, f.h.do(get(tasks+"/t1/edit", f.sess))).Form)
	form["attendee"] = []string{"mailto:bob@example.org"}
	form.Set("attendee.add", "carol@example.org")
	if resp := f.h.do(postForm(tasks+"/t1/edit", f.sess, form)); resp.Code != 303 {
		t.Fatalf("POST: %d %s", resp.Code, resp.Body.String())
	}
	got := unfolded(body(t, f, "t1.ics"))
	mustContain(t, got, "ATTENDEE;CN=Bob;PARTSTAT=ACCEPTED:mailto:bob@example.org", "ATTENDEE;PARTSTAT=NEEDS-ACTION;RSVP=TRUE:mailto:carol@example.org", "X-KEEP:1")

	page := decode[view.NotePage](t, f.h.do(get(f.path("notes")+"/n1", f.sess)))
	if got := people(page.Meta.Attendees); got != "Bob/no reply" {
		t.Errorf("the note's page names %q", got)
	}
	nedit := decode[view.NotePage](t, f.h.do(get(f.path("notes")+"/n1/edit", f.sess)))
	if nedit.Doc.Attendees == nil || !slices.Equal(pageTags(*nedit.Doc.Attendees), []string{"attendee=mailto:bob@example.org"}) {
		t.Errorf("the note's field = %+v", nedit.Doc.Attendees)
	}
	if strings.Contains(fmt.Sprint(nedit.Doc.Foreign), "ATTENDEE") {
		t.Errorf("ATTENDEE is kept as foreign: %v", nedit.Doc.Foreign)
	}
	nform := noteFormOf(t, f, "n1")
	nform["attendee"] = []string{}
	nform.Set("attendee.add", "")
	if resp := f.h.do(postForm(f.path("notes")+"/n1/edit", f.sess, nform)); resp.Code != 303 {
		t.Fatalf("POST: %d %s", resp.Code, resp.Body.String())
	}
	if got := body(t, f, "n1.ics"); strings.Contains(got, "ATTENDEE") || !strings.Contains(got, "X-KEEP:1") {
		t.Errorf("Bob unticked is still there:\n%s", got)
	}
}

// Р6 of scans/kinds-parity.md: an event's form chooses the zone its times
// are written in and shows them in it (s-agenda-3, -8); a task's showed its
// times in the instance zone and had none to choose, though they are written
// with TZID as an event's are. Now a task's When ends with Time zone, as the
// event's: the zone its times came in — its start's, else its due date's —
// «as sent», its times shown in it; another rewrites the start and the due
// date at the clock the form names, in that zone, with its VTIMEZONE.
func TestATaskChoosesItsZone(t *testing.T) {
	f := calendarFixture(t, nil,
		"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Call\n"+
			"DTSTART;TZID=Asia/Yekaterinburg:20260814T090000\nDUE;TZID=Asia/Yekaterinburg:20260815T120000\nX-KEEP:1\nEND:VTODO"),
		"t2.ics", ics("BEGIN:VTODO\nUID:t2\nDTSTAMP:20260101T000000Z\nSUMMARY:Pay\nDUE;TZID=Europe/Moscow:20260815T180000\nEND:VTODO"))
	tasks := f.path("tasks")
	edit := func(uid string) view.TaskEditPage {
		t.Helper()
		return decode[view.TaskEditPage](t, f.h.do(get(tasks+"/"+uid+"/edit", f.sess)))
	}
	save := func(uid string, set map[string]string) string {
		t.Helper()
		form := formFields(edit(uid).Form)
		for k, v := range set {
			form.Set(k, v)
		}
		if resp := f.h.do(postForm(tasks+"/"+uid+"/edit", f.sess, form)); resp.Code != 303 {
			t.Fatalf("POST %v: %d %s", set, resp.Code, resp.Body.String())
		}
		return unfolded(body(t, f, uid+".ics"))
	}

	page := edit("t1")
	zone := rowNamed(t, page.Form, "tz")
	if zone.Value != "Asia/Yekaterinburg" || zone.ValueAside != "as sent" || len(zone.Options) < 3 ||
		zone.Options[0].Value != "Asia/Yekaterinburg" || zone.Options[0].Group != "As sent" {
		t.Fatalf("zone row = %+v", zone)
	}
	if form := formFields(page.Form); form.Get("start.time") != "09:00" || form.Get("due.time") != "12:00" {
		t.Errorf("times shown at %s and %s, want the task zone's 09:00 and 12:00", form.Get("start.time"), form.Get("due.time"))
	}
	got := save("t1", map[string]string{"summary": "Call again"})
	mustContain(t, got, "DTSTART;TZID=Asia/Yekaterinburg:20260814T090000", "DUE;TZID=Asia/Yekaterinburg:20260815T120000")
	got = save("t1", map[string]string{"tz": "Europe/London"})
	mustContain(t, got, "DTSTART;TZID=Europe/London:20260814T090000", "DUE;TZID=Europe/London:20260815T120000",
		"BEGIN:VTIMEZONE", "TZID:Europe/London", "SUMMARY:Call again", "X-KEEP:1")

	// A task with a due date and no start: the due date's zone.
	page = edit("t2")
	if zone := rowNamed(t, page.Form, "tz"); zone.Value != "Europe/Moscow" {
		t.Errorf("a due date's zone = %+v", zone)
	}
	if form := formFields(page.Form); form.Get("due.time") != "18:00" {
		t.Errorf("due shown at %s, want 18:00", form.Get("due.time"))
	}
	mustContain(t, save("t2", map[string]string{"tz": "UTC"}), "DUE:20260815T180000Z")

	form := formFields(edit("t1").Form)
	form.Set("tz", "Mars/Olympus")
	bad := decode[view.TaskEditPage](t, f.h.do(postForm(tasks+"/t1/edit", f.sess, form)))
	if len(bad.Form.Errors) == 0 || bad.Form.Errors[0].Name != "tz" {
		t.Errorf("errors = %+v, want tz refused", bad.Form.Errors)
	}
}

// Following Р3 of scans/kinds-parity.md, as Р13 followed Р4 and Р7: the
// agenda offers its people as a column; a task's and a notebook's list
// offered none, now that a task and a note have attendees. Each offers
// «Attendees», off until chosen, and a row carries how many — the
// organiser one of them, as the agenda's badge counts.
func TestAttendeesColumnForEveryKind(t *testing.T) {
	f := calendarFixture(t, nil,
		"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Slides\nORGANIZER:mailto:anna@example.org\nATTENDEE:mailto:bob@example.org\nEND:VTODO"),
		"n1.ics", ics("BEGIN:VJOURNAL\nUID:n1\nDTSTAMP:20260101T000000Z\nSUMMARY:Minutes\nATTENDEE:mailto:bob@example.org\nEND:VJOURNAL"))
	offered := func(cols *view.Columns) bool {
		for _, it := range cols.Items {
			if it.ID == "attendees" {
				return !it.On
			}
		}
		return false
	}
	tasks := decode[view.TasksPage](t, f.h.do(get(f.path("tasks")+"?seg=all", f.sess)))
	if tasks.Bar.Columns == nil || !offered(tasks.Bar.Columns) || len(tasks.Groups) == 0 || tasks.Groups[0].Rows[0].Attendees != "2 attendees" {
		t.Errorf("tasks: columns %+v, groups %+v", tasks.Bar.Columns, tasks.Groups)
	}
	notes := decode[view.NotesPage](t, f.h.do(get(f.path("notes"), f.sess)))
	if notes.Bar.Columns == nil || !offered(notes.Bar.Columns) || len(notes.Groups) == 0 || notes.Groups[0].Rows[0].Attendees != "1 attendee" {
		t.Errorf("notes: columns %+v, groups %+v", notes.Bar.Columns, notes.Groups)
	}
}
