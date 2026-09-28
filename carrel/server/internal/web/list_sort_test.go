package web_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-webdav/caldav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

func listNames(p view.ContactsListPage) string {
	var out []string
	for _, g := range p.Groups {
		for _, r := range g.Rows {
			out = append(out, string(r.Name))
		}
	}
	return strings.Join(out, ",")
}

func segmentLabels(bar view.ListBar) string {
	var out []string
	for _, a := range bar.Segments {
		out = append(out, string(a.Label))
	}
	return strings.Join(out, "|")
}

func sortHref(t *testing.T, bar view.ListBar, id string) string {
	t.Helper()
	for _, a := range bar.Segments {
		if a.ID == id {
			u, _ := url.Parse(a.Href)
			return u.RequestURI()
		}
	}
	t.Fatalf("no segment %q in %+v", id, bar.Segments)
	return ""
}

// review Р-12 (s-contacts-1, s-notes-1): the order of a list is the
// server's — the client neither sorts nor reckons (C-3.1). A–Z groups by
// letter; Recently changed is one list, newest change first.
func TestContactAndNoteOrders(t *testing.T) {
	f := contactsFixture(t, false,
		"BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nREV:20260101T000000Z\nEND:VCARD\n",
		"BEGIN:VCARD\nVERSION:4.0\nUID:bob\nFN:Bob Builder\nREV:20260901T000000Z\nEND:VCARD\n",
		"BEGIN:VCARD\nVERSION:4.0\nUID:cat\nFN:Cat Stevens\nREV:20260501T000000Z\nEND:VCARD\n")
	list := f.path("contacts")

	page := decode[view.ContactsListPage](t, f.h.do(get(list, f.sess)))
	if got := segmentLabels(page.Bar); got != "A–Z|Recently changed" {
		t.Fatalf("segments = %q", got)
	}
	sameSegments(t, "contact orders", page.Bar.Segments, "s-contacts-1", 0)
	if names := listNames(page); names != "Ada Lovelace,Bob Builder,Cat Stevens" {
		t.Errorf("A–Z = %q", names)
	}
	if page.Groups[0].Rubric != "A" {
		t.Errorf("A–Z groups = %+v, want letters", page.Groups)
	}
	changed := decode[view.ContactsListPage](t, f.h.do(get(sortHref(t, page.Bar, "sort.changed"), f.sess)))
	if names := listNames(changed); names != "Bob Builder,Cat Stevens,Ada Lovelace" {
		t.Errorf("Recently changed = %q", names)
	}
	if len(changed.Groups) != 1 || changed.Groups[0].Rubric != "" {
		t.Errorf("Recently changed groups = %+v, want one list", changed.Groups)
	}

	n := calendarFixture(t, nil,
		"n1.ics", ics("BEGIN:VJOURNAL\nUID:n1\nDTSTAMP:20260101T000000Z\nDTSTART:20260110T090000Z\nSUMMARY:Zebra\nEND:VJOURNAL"),
		"n2.ics", ics("BEGIN:VJOURNAL\nUID:n2\nDTSTAMP:20260101T000000Z\nDTSTART:20260310T090000Z\nSUMMARY:Apple\nEND:VJOURNAL"))
	notes := n.path("notes")
	np := decode[view.NotesPage](t, n.h.do(get(notes, n.sess)))
	if got := segmentLabels(np.Bar); got != "Newest|Oldest|Title" {
		t.Fatalf("note segments = %q", got)
	}
	sameSegments(t, "note orders", np.Bar.Segments, "s-notes-1", 0)
	titles := func(p view.NotesPage) string {
		var out []string
		for _, g := range p.Groups {
			for _, r := range g.Rows {
				out = append(out, string(r.Title))
			}
		}
		return strings.Join(out, ",")
	}
	if titles(np) != "Apple,Zebra" {
		t.Errorf("Newest = %q", titles(np))
	}
	oldest := decode[view.NotesPage](t, n.h.do(get(sortHref(t, np.Bar, "sort.oldest"), n.sess)))
	if titles(oldest) != "Zebra,Apple" {
		t.Errorf("Oldest = %q", titles(oldest))
	}
	byTitle := decode[view.NotesPage](t, n.h.do(get(sortHref(t, np.Bar, "sort.title"), n.sess)))
	if titles(byTitle) != "Apple,Zebra" {
		t.Errorf("Title = %q", titles(byTitle))
	}
}

// review Р-12 (s-tasks-1): tasks are grouped by their due date — Overdue,
// This week, Later, No due date — and the other orders are one list.
func TestTaskGroupsAndOrders(t *testing.T) {
	now := time.Now()
	day := func(d int) string { return now.AddDate(0, 0, d).UTC().Format("20060102T150405Z") }
	f := calendarFixture(t, nil,
		"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Late one\nDUE:"+day(-3)+"\nPRIORITY:9\nEND:VTODO"),
		"t2.ics", ics("BEGIN:VTODO\nUID:t2\nDTSTAMP:20260101T000000Z\nSUMMARY:Soon\nDUE:"+day(1)+"\nPRIORITY:1\nEND:VTODO"),
		"t3.ics", ics("BEGIN:VTODO\nUID:t3\nDTSTAMP:20260101T000000Z\nSUMMARY:Someday\nEND:VTODO"),
		"t4.ics", ics("BEGIN:VTODO\nUID:t4\nDTSTAMP:20260101T000000Z\nSUMMARY:Next month\nDUE:"+day(40)+"\nPRIORITY:5\nEND:VTODO"))
	list := f.path("tasks")

	page := decode[view.TasksPage](t, f.h.do(get(list, f.sess)))
	if got := segmentLabels(page.Bar); !strings.Contains(got, "By due|By priority|Recently changed") {
		t.Fatalf("task segments = %q", got)
	}
	var rubrics []string
	for _, g := range page.Groups {
		rubrics = append(rubrics, string(g.Rubric))
	}
	if strings.Join(rubrics, "|") != "Overdue|This week|Later|No due date" {
		t.Fatalf("groups = %v", rubrics)
	}
	byPriority := decode[view.TasksPage](t, f.h.do(get(sortHref(t, page.Bar, "sort.priority"), f.sess)))
	var order []string
	for _, g := range byPriority.Groups {
		if g.Rubric != "" {
			t.Errorf("By priority still groups: %q", g.Rubric)
		}
		for _, r := range g.Rows {
			order = append(order, string(r.Summary))
		}
	}
	if strings.Join(order, ",") != "Soon,Next month,Late one,Someday" {
		t.Errorf("By priority = %v", order)
	}
}

// review Р-12 (s-auth-4): the first administrator may leave an address,
// so getting back in later is possible at all. It is not confirmed here —
// nothing was sent to it.
func TestSetupTakesAnOptionalEmail(t *testing.T) {
	h := newHarness(t)
	page := decode[view.SetupPage](t, h.do(get("/setup", nil)))
	row := rowNamed(t, page.Form, "email")
	if row.Kind != view.InputEmail || row.Required {
		t.Fatalf("email row = %+v, want an optional address", row)
	}
	if rec := h.do(h.anonPostForm("/setup", "/setup", url.Values{
		"login": {"owner"}, "email": {"owner@example.org"}, "password": {"password123"}, "password2": {"password123"},
	})); rec.Code != 303 {
		t.Fatalf("setup: %d %s", rec.Code, rec.Body.String())
	}
	snap := h.st.Snapshot()
	u := snap.UserByLogin("owner")
	if u == nil || u.Email != "owner@example.org" || u.EmailConfirmed {
		t.Errorf("the administrator = %+v, want the address unconfirmed", u)
	}
}

// review Р-12: a RELATED-TO whose target lives in another collection was
// a bare UID. It has the target's title when some list of this session
// holds it; the link stays inside the collection (S-11).
func TestRelatedTitleFromAnotherLoadedList(t *testing.T) {
	srv := searchServer(t)
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work", SupportedComponentSet: []string{"VEVENT", "VTODO"}})
	srv.AddCalendar(caldav.Calendar{Path: davtest.CalendarHome + "home/", Name: "Home", SupportedComponentSet: []string{"VEVENT", "VTODO"}})
	srv.Seed(searchWork+"e.ics", ics("BEGIN:VEVENT\nUID:e\nDTSTAMP:20260101T000000Z\nDTSTART:20260115T100000Z\nSUMMARY:Review\nRELATED-TO:far\nEND:VEVENT"))
	srv.Seed(davtest.CalendarHome+"home/far.ics", ics("BEGIN:VTODO\nUID:far\nDTSTAMP:20260101T000000Z\nSUMMARY:Buy the cable\nEND:VTODO"))
	f := connectFixture(t, srv, searchWork, nil)

	rec := f.path("calendar") + "/e"
	page := decode[view.EventPage](t, f.h.do(get(rec, f.sess)))
	if len(page.Related) != 1 || page.Related[0].Title != "far" {
		t.Fatalf("before the other list is read: %+v", page.Related)
	}
	// Read the other collection's tasks; now the UID has a name.
	f.h.do(get("/app/tasks/"+f.id+"/"+url.PathEscape(davtest.CalendarHome+"home/"), f.sess))
	page = decode[view.EventPage](t, f.h.do(get(rec, f.sess)))
	if len(page.Related) != 1 || page.Related[0].Title != "Buy the cable" || page.Related[0].Href != "" {
		t.Errorf("related = %+v, want the title without a link", page.Related)
	}
}

// review Р-12 (s-contacts-1): the combined view has Import and Export
// too, and each asks which collection — there is more than one, and the
// server does not guess (S-14).
func TestCombinedImportAndExportAsk(t *testing.T) {
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nEND:VCARD\n")
	tickAll(t, f.h, f.sess)

	page := combined(t, f.h, f.sess, "/app/contacts", contactsPoll)
	imp := actionByID(page.Bar.Actions, "collection.import")
	exp := actionByID(page.Bar.Actions, "collection.export")
	if imp == nil || !imp.Enabled || exp == nil || !exp.Enabled {
		t.Fatalf("combined bar = %+v", page.Bar.Actions)
	}
	// Import opens the wizard with its "Into" picker.
	rec := f.h.do(get(imp.Href, f.sess))
	if rec.Code != 303 {
		t.Fatalf("import entry: %d", rec.Code)
	}
	wizard := decode[view.ImportPage](t, f.h.do(get(rec.Header().Get("Location"), f.sess)))
	if len(wizard.Into.Options) == 0 || wizard.Into.Chosen == "" {
		t.Errorf("the wizard does not ask where: %+v", wizard.Into)
	}
	// Export opens the sheet of collections to hand over.
	sheet := decode[view.ExportSheet](t, f.h.do(get(exp.Href, f.sess)))
	if len(sheet.Target.Options) == 0 || !strings.HasSuffix(string(sheet.Target.Options[0].Label), "People") {
		t.Errorf("export sheet = %+v", sheet.Target)
	}
	if !strings.Contains(sheet.Target.Options[0].ID, "/export") {
		t.Errorf("the option does not hand anything over: %+v", sheet.Target.Options[0])
	}
	if actionByID(sheet.Actions, "export.cancel") == nil {
		t.Errorf("no way out of the sheet: %+v", sheet.Actions)
	}
}
