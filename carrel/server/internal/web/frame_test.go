package web_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/view"
)

// backRow is the one row of a back rail, after checking the rail is one.
func backRow(t *testing.T, rail view.Rail) view.NavRow {
	t.Helper()
	if len(rail.Nav) != 1 || rail.Nav[0].ID != "nav.back" || rail.Nav[0].Count != nil || rail.Nav[0].On {
		t.Fatalf("not a back rail: nav %+v", rail.Nav)
	}
	// Empty, not absent: an empty list goes out as [] (C-6.1).
	if len(rail.Tags) != 0 || len(rail.Foot) != 0 {
		t.Errorf("a back rail with tags %+v or a foot %+v", rail.Tags, rail.Foot)
	}
	for _, sec := range rail.Sections {
		for _, row := range sec.Rows {
			if strings.HasPrefix(string(row.Label), "Combined") {
				t.Errorf("a back rail with the combined row: %+v", row)
			}
		}
	}
	return rail.Nav[0]
}

// tagLine is the tags as labels, the one that is on marked with a star.
func tagLine(tags []view.Tag) string {
	out := make([]string, 0, len(tags))
	for _, tg := range tags {
		label := string(tg.Label)
		if tg.On {
			label += "*"
		}
		out = append(out, label)
	}
	return strings.Join(out, ", ")
}

func tagHref(t *testing.T, tags []view.Tag, label string) string {
	t.Helper()
	for _, tg := range tags {
		if string(tg.Label) == label {
			return tg.Href
		}
	}
	t.Fatalf("no tag %q in %+v", label, tags)
	return ""
}

// C-6.3: a record, its edit, a new record and the import screen carry the
// back rail: one way out, the same collections with their counts, and no
// combined row, tags or foot. A record read is printed with where it lives
// and when; an edit is not printed.
func TestBackRailOnRecordScreens(t *testing.T) {
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nEND:VCARD\n")
	list := f.path("contacts")
	rec := list + "/ada"
	tickRail(t, f.h, f.sess, "contacts", list)
	decode[view.ContactsListPage](t, f.h.do(get(list, f.sess)))

	card := decode[view.ContactPage](t, f.h.do(get(rec, f.sess)))
	if back := backRow(t, card.Rail); back.Label != "Back to list" || back.Href != list {
		t.Errorf("record back = %+v", back)
	}
	if _, people := railRow(t, card.Rail, "People"); people.Tick != view.TickOn || people.Count == nil || *people.Count != 1 || people.Href != list {
		t.Errorf("people on the record = %+v", people)
	}
	if p := card.Print; p.Title != "Ada Lovelace" || !strings.HasSuffix(string(p.Range), " · People") ||
		!strings.Contains(string(p.Taken), time.Now().Format("2006")) {
		t.Errorf("record print = %+v", p)
	}

	edit := decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
	if back := backRow(t, edit.Rail); back.Label != "Back to the card" || back.Href != rec {
		t.Errorf("edit back = %+v", back)
	}
	if edit.Print != (view.Print{}) {
		t.Errorf("edit print = %+v", edit.Print)
	}
	fresh := decode[view.ContactEditPage](t, f.h.do(get(list+"/new", f.sess)))
	if back := backRow(t, fresh.Rail); back.Label != "Back to list" || back.Href != list {
		t.Errorf("new back = %+v", back)
	}
	imp := decode[view.ImportPage](t, f.h.do(get(list+"/import", f.sess)))
	if back := backRow(t, imp.Rail); back.Href != list {
		t.Errorf("import back = %+v", back)
	}
}

// C-6.3: the rail's tags are the categories of what the list read, the most
// used first, "all" before them. A tag cuts the list without case and without
// touching the rail's count; the other tags stay; a tag nobody has stays
// marked so it can be cleared. The card's categories link to the filter.
func TestTagFilterOnContacts(t *testing.T) {
	f := contactsFixture(t, false,
		"BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nCATEGORIES:math,poetry\nEND:VCARD\n",
		"BEGIN:VCARD\nVERSION:4.0\nUID:bob\nFN:Bob Builder\nCATEGORIES:math\nEND:VCARD\n",
		"BEGIN:VCARD\nVERSION:4.0\nUID:cy\nFN:Cy Twombly\nCATEGORIES:работа\nEND:VCARD\n")
	list := f.path("contacts")
	tickRail(t, f.h, f.sess, "contacts", list)

	page := decode[view.ContactsListPage](t, f.h.do(get(list, f.sess)))
	if got := tagLine(page.Rail.Tags); got != "all*, math, poetry, работа" {
		t.Fatalf("tags = %s", got)
	}
	if h := tagHref(t, page.Rail.Tags, "all"); h != list {
		t.Errorf("all = %q", h)
	}
	if h := tagHref(t, page.Rail.Tags, "работа"); h != list+"?tag=%D1%80%D0%B0%D0%B1%D0%BE%D1%82%D0%B0" {
		t.Errorf("работа = %q", h)
	}
	if p := page.Print; p.Title != "People" || p.Range != "3 cards" || p.Taken == "" {
		t.Errorf("print = %+v", p)
	}

	filtered := decode[view.ContactsListPage](t, f.h.do(get(list+"?tag=POETRY", f.sess)))
	if len(filtered.Groups) != 1 || len(filtered.Groups[0].Rows) != 1 || filtered.Groups[0].Rows[0].Name != "Ada Lovelace" {
		t.Fatalf("filtered = %+v", filtered.Groups)
	}
	if got := tagLine(filtered.Rail.Tags); got != "all, math, poetry*, работа" {
		t.Errorf("filtered tags = %s", got)
	}
	if filtered.Bar.Loaded != 1 || filtered.Bar.Total == nil || *filtered.Bar.Total != 3 {
		t.Errorf("bar = %+v", filtered.Bar)
	}
	if _, people := railRow(t, filtered.Rail, "People"); people.Count == nil || *people.Count != 3 {
		t.Errorf("the filter changed the collection's count: %+v", people)
	}
	if filtered.Print.Range != "1 card · POETRY" {
		t.Errorf("filtered print = %+v", filtered.Print)
	}

	nobody := decode[view.ContactsListPage](t, f.h.do(get(list+"?tag=chess", f.sess)))
	if len(nobody.Groups) != 0 || tagLine(nobody.Rail.Tags) != "all, math, poetry, работа, chess*" {
		t.Errorf("a tag nobody has: %+v, tags %s", nobody.Groups, tagLine(nobody.Rail.Tags))
	}

	card := decode[view.ContactPage](t, f.h.do(get(list+"/ada", f.sess)))
	if tagLine(card.Tags) != "math, poetry" || tagHref(t, card.Tags, "poetry") != list+"?tag=poetry" {
		t.Errorf("card tags = %+v", card.Tags)
	}
}

// The filter on tasks, notes and the agenda: rows' categories link to it, and
// the segments and the week keep it.
func TestTagFilterOnCalendars(t *testing.T) {
	f := calendarFixture(t, nil,
		"a.ics", ics("BEGIN:VTODO\nUID:a\nDTSTAMP:20260101T000000Z\nDUE:20260910T120000Z\nCATEGORIES:home\nSUMMARY:Groceries\nEND:VTODO"),
		"b.ics", ics("BEGIN:VTODO\nUID:b\nDTSTAMP:20260101T000000Z\nDUE:20260911T120000Z\nCATEGORIES:work\nSUMMARY:Report\nEND:VTODO"),
		"n.ics", ics("BEGIN:VJOURNAL\nUID:n\nDTSTAMP:20260101T000000Z\nDTSTART:20260905T100000Z\nCATEGORIES:home,garden\nSUMMARY:Seeds\nEND:VJOURNAL"),
		"m.ics", ics("BEGIN:VJOURNAL\nUID:m\nDTSTAMP:20260101T000000Z\nDTSTART:20260906T100000Z\nSUMMARY:Minutes\nEND:VJOURNAL"),
		"e.ics", ics("BEGIN:VEVENT\nUID:e\nDTSTAMP:20260101T000000Z\nDTSTART:20260908T100000Z\nCATEGORIES:home\nSUMMARY:Plumber\nEND:VEVENT"),
		"g.ics", ics("BEGIN:VEVENT\nUID:g\nDTSTAMP:20260101T000000Z\nDTSTART:20260909T100000Z\nCATEGORIES:work\nSUMMARY:Standup\nEND:VEVENT"),
	)

	tasksList := f.path("tasks")
	tasks := decode[view.TasksPage](t, f.h.do(get(tasksList+"?tag=home", f.sess)))
	if len(tasks.Groups) != 1 || len(tasks.Groups[0].Rows) != 1 || tasks.Groups[0].Rows[0].Summary != "Groceries" {
		t.Fatalf("tasks = %+v", tasks.Groups)
	}
	if tg := tasks.Groups[0].Rows[0].Tags; len(tg) != 1 || !tg[0].On || tg[0].Href != tasksList+"?tag=home" {
		t.Errorf("task row tags = %+v", tg)
	}
	if h := actionHref(t, tasks.Bar.Segments, "tasks.done"); h != tasksList+"?show=done&tag=home" {
		t.Errorf("done segment = %q", h)
	}
	if got := tagLine(tasks.Rail.Tags); got != "all, home*, work" {
		t.Errorf("task tags = %s", got)
	}
	if tasks.Print.Range != "Open · 1 task · home" {
		t.Errorf("task print = %+v", tasks.Print)
	}

	notesList := f.path("notes")
	notes := decode[view.NotesPage](t, f.h.do(get(notesList+"?tag=garden", f.sess)))
	if len(notes.Groups) != 1 || len(notes.Groups[0].Rows) != 1 || notes.Groups[0].Rows[0].Title != "Seeds" {
		t.Fatalf("notes = %+v", notes.Groups)
	}
	if tagHref(t, notes.Groups[0].Rows[0].Tags, "home") != notesList+"?tag=home" {
		t.Errorf("note row tags = %+v", notes.Groups[0].Rows[0].Tags)
	}
	if got := tagLine(notes.Rail.Tags); got != "all, garden*, home" {
		t.Errorf("note tags = %s", got)
	}

	cal := f.path("calendar")
	agenda := decode[view.AgendaPage](t, f.h.do(get(cal+"?from=2026-09-07&tag=home", f.sess)))
	// Seven days: the filter cuts the rows, not the days (views-tail.md).
	if agenda.Bar.Loaded != 1 || len(agenda.Days) != 7 || agenda.Days[1].Rows[0].Summary != "Plumber" {
		t.Fatalf("agenda = %+v", agenda.Days)
	}
	if agenda.Bar.Range == nil || actionHref(t, agenda.Bar.Range.Presets, "range.next") != cal+"?from=2026-09-14&tag=home" ||
		actionHref(t, agenda.Bar.Range.Presets, "range.current") != cal+"?tag=home" {
		t.Errorf("range = %+v", agenda.Bar.Range)
	}
	if agenda.Print.Range != "7 September 2026 – 13 September 2026 · 1 event · home" {
		t.Errorf("agenda print = %+v", agenda.Print)
	}
	event := decode[view.EventPage](t, f.h.do(get(cal+"/e", f.sess)))
	if back := backRow(t, event.Rail); back.Label != "Back to calendar" || back.Href != cal {
		t.Errorf("event back = %+v", back)
	}
}

// S-14: in the combined view the filter's links name the poll's task, so a
// category is chosen from what was polled and no server is asked again; the
// refresh of a filtered view stays filtered. The print header counts the
// books and, once the poll is over, the cards.
func TestCombinedTagFilterReadsThePoll(t *testing.T) {
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.AddAddressBook(carddav.AddressBook{Path: teamBook, Name: "Team"})
	srv.Seed(searchBook+"ada.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nCATEGORIES:math\nEND:VCARD\n")
	srv.Seed(searchBook+"bob.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:bob\nFN:Bob Builder\nEND:VCARD\n")
	srv.Seed(teamBook+"ann.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:ann\nFN:Ann Other\nCATEGORIES:Math\nEND:VCARD\n")
	f := connectFixture(t, srv, searchBook, nil)
	tickRail(t, f.h, f.sess, "contacts", f.path("contacts"), "/app/contacts/"+f.id+"/"+url.PathEscape(teamBook))

	page := combined(t, f.h, f.sess, "/app/contacts", contactsPoll)
	if p := page.Print; p.Title != "Combined address book" || p.Range != "2 books · 3 cards" || p.Taken == "" {
		t.Errorf("print = %+v", p)
	}
	task := page.Poll.TaskID
	if got := tagLine(page.Rail.Tags); got != "all*, math" && got != "all*, Math" {
		t.Fatalf("tags = %s", got)
	}
	label := string(page.Rail.Tags[1].Label)
	want := url.Values{"tag": {label}, "task": {task}}
	if h := page.Rail.Tags[1].Href; h != "/app/contacts?"+want.Encode() {
		t.Fatalf("tag href = %q", h)
	}

	asked := len(srv.Requests())
	filtered := decode[view.ContactsListPage](t, f.h.do(get(page.Rail.Tags[1].Href, f.sess)))
	if len(srv.Requests()) != asked || filtered.Poll.TaskID != task {
		t.Errorf("choosing a category polled again: %d requests, task %q", len(srv.Requests())-asked, filtered.Poll.TaskID)
	}
	if got := contactNames(filtered); got != "Ada Lovelace · People, Ann Other · Team" {
		t.Errorf("filtered = %s", got)
	}
	if filtered.Print.Range != view.Text("2 books · 2 cards · "+label) {
		t.Errorf("filtered print = %+v", filtered.Print)
	}
	if h := filtered.Top.Refresh.Action.Href; h != withTag("/app/contacts", label) {
		t.Errorf("refresh = %q", h)
	}
	if h := tagHref(t, filtered.Rail.Tags, "all"); h != "/app/contacts?task="+url.QueryEscape(task) {
		t.Errorf("all = %q", h)
	}
}

func withTag(path, tag string) string {
	return path + "?tag=" + url.QueryEscape(tag)
}
