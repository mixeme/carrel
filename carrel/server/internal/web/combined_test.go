package web_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

const (
	teamBook = davtest.AddressBookHome + "team/"
	homeCal  = davtest.CalendarHome + "home/"
)

type railFragment struct {
	Target string    `json:"target"`
	Body   view.Rail `json:"body"`
	Extra  []struct {
		Target string          `json:"target"`
		Body   json.RawMessage `json:"body"`
	} `json:"extra"`
}

// railRow finds a collection row of the rail by its label.
func railRow(t *testing.T, rail view.Rail, label string) (view.RailSection, view.SourceRow) {
	t.Helper()
	for _, sec := range rail.Sections {
		for _, row := range sec.Rows {
			if string(row.Label) == label {
				return sec, row
			}
		}
	}
	t.Fatalf("no rail row %q in %+v", label, rail.Sections)
	return view.RailSection{}, view.SourceRow{}
}

// tickRail posts the rail form of one kind the way a browser without scripts
// does: the hrefs of the ticked rows.
func tickRail(t *testing.T, h *harness, sess *session.Session, segment string, hrefs ...string) {
	t.Helper()
	rec := h.do(postForm("/app/sources/"+segment, sess, url.Values{"tick": hrefs}))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/app/"+segment {
		t.Fatalf("tick %s: %d %q", segment, rec.Code, rec.Header().Get("Location"))
	}
}

func selected(t *testing.T, h *harness, sess *session.Session, k view.Kind) []string {
	t.Helper()
	key, _ := sess.DataKey()
	blob, ok := h.users.Catalog(sess.AccountID(), key)
	if !ok {
		t.Fatal("no catalog")
	}
	var out []string
	for _, s := range blob.Selected {
		if s.Kind == k {
			out = append(out, s.AccountID+" "+s.Path)
		}
	}
	return out
}

// combined opens a combined view, waits for its poll, and reads the page of
// that task again.
func combined[T any](t *testing.T, h *harness, sess *session.Session, path string, poll func(T) *view.PollBar) T {
	t.Helper()
	first := decode[T](t, h.do(get(path, sess)))
	task := poll(first).TaskID
	if task == "" {
		t.Fatalf("%s started no task: %+v", path, poll(first))
	}
	waitPoll(t, h, sess, task, nil)
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	page := decode[T](t, h.do(get(path+sep+"task="+task, sess)))
	if poll(page).TaskID != task {
		t.Fatalf("the page after the poll is another task: %q, want %q", poll(page).TaskID, task)
	}
	return page
}

func contactsPoll(p view.ContactsListPage) *view.PollBar { return p.Poll }

func contactNames(p view.ContactsListPage) string {
	var out []string
	for _, g := range p.Groups {
		for _, r := range g.Rows {
			out = append(out, string(r.Name)+" · "+strings.SplitN(string(r.Stamp), " · ", 2)[1])
		}
	}
	return strings.Join(out, ", ")
}

// S-14: nothing ticked is an empty screen that asks nobody; the ticks go into
// the blob one kind at a time; the combined book merges two servers by name
// with the origin on every row; unticking takes a collection's records away;
// a row that is not a collection of the kind is not a tick.
func TestCombinedContactsFollowTheTicks(t *testing.T) {
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work"})
	srv.Seed(searchBook+"ada.vcf", adaCard())
	srv.Seed(searchBook+"bob.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:bob\nFN:Bob Builder\nEND:VCARD\n")
	other := searchServer(t)
	other.AddAddressBook(carddav.AddressBook{Path: teamBook, Name: "Team"})
	other.Seed(teamBook+"ann.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:ann\nFN:Ann Other\nEND:VCARD\n")

	f := connectFixture(t, srv, searchBook, nil)
	if rec := f.h.do(postForm("/app/settings/connections", f.sess, url.Values{
		"server": {other.URL}, "login": {other.Login}, "password": {other.Password},
	})); rec.Code != http.StatusSeeOther {
		t.Fatalf("second connection: %d %s", rec.Code, rec.Body.String())
	}
	key, _ := f.sess.DataKey()
	blob, _ := f.h.users.Catalog(f.sess.AccountID(), key)
	second := blob.Connections[1].ID
	teamPath := "/app/contacts/" + second + "/" + url.PathEscape(teamBook)

	asked := len(srv.Requests()) + len(other.Requests())
	empty := decode[view.ContactsListPage](t, f.h.do(get("/app/contacts", f.sess)))
	if empty.Poll == nil || empty.Poll.TaskID != "" || !strings.Contains(string(empty.Poll.Summary), "No collections are ticked") ||
		len(empty.Groups) != 0 || empty.Title != "Combined address book" {
		t.Fatalf("nothing ticked = %+v, poll %+v", empty.Groups, empty.Poll)
	}
	if len(srv.Requests())+len(other.Requests()) != asked {
		t.Error("an empty selection asked a server")
	}
	if combinedRow := empty.Rail.Sections[0].Rows[0]; combinedRow.Tick != view.TickOff || combinedRow.Href != "/app/contacts" || combinedRow.Count != nil {
		t.Errorf("combined row = %+v", combinedRow)
	}
	_, people := railRow(t, empty.Rail, "People")
	if people.Tick != view.TickOff || people.Href != f.path("contacts") || people.Count != nil {
		t.Errorf("people row = %+v", people)
	}
	actionHref(t, empty.Rail.Foot, "rail.apply")

	tickRail(t, f.h, f.sess, "calendar", "/app/calendar/"+f.id+"/"+url.PathEscape(searchWork))
	tickRail(t, f.h, f.sess, "contacts", people.Href, teamPath, "/app/contacts/a-foreign0001/x", f.path("calendar"))
	if got := selected(t, f.h, f.sess, view.KindContact); len(got) != 2 {
		t.Fatalf("contact ticks = %v", got)
	}
	if got := selected(t, f.h, f.sess, view.KindEvent); len(got) != 1 {
		t.Fatalf("ticking contacts touched the calendar ticks: %v", got)
	}

	page := combined(t, f.h, f.sess, "/app/contacts", contactsPoll)
	if got := contactNames(page); got != "Ada Lovelace · People, Ann Other · Team, Bob Builder · People" {
		t.Fatalf("rows = %s", got)
	}
	ann := page.Groups[0].Rows[1]
	if ann.Href != teamPath+"/ann" || ann.Colour == "" {
		t.Errorf("ann = %+v", ann)
	}
	if page.Bar.Total == nil || *page.Bar.Total != 3 || len(page.Poll.Sources) != 2 || page.Poll.Running {
		t.Errorf("bar %+v, poll %+v", page.Bar, page.Poll)
	}
	if !strings.Contains(string(page.Poll.Summary), "2 of 2 sources answered · 3 records") {
		t.Errorf("summary = %q", page.Poll.Summary)
	}
	if r := page.Top.Refresh; !r.Action.Enabled || r.Action.Href != "/app/contacts" || r.Answered == "" || r.Running {
		t.Errorf("refresh = %+v", r)
	}
	if again := action(*page.Poll, "poll.again"); again.Label != "Refresh" || again.Href != "/app/contacts" {
		t.Errorf("poll again = %+v", again)
	}
	if c := page.Rail.Sections[0].Rows[0]; c.Tick != view.TickOn || c.Count == nil || *c.Count != 3 {
		t.Errorf("combined row = %+v", c)
	}
	if _, p := railRow(t, page.Rail, "People"); p.Tick != view.TickOn || p.Count == nil || *p.Count != 2 {
		t.Errorf("people row = %+v", p)
	}
	if nav := page.Rail.Nav[0]; !nav.On || nav.Count == nil || *nav.Count != 3 {
		t.Errorf("nav = %+v", nav)
	}

	// With scripts the rail answers itself and the poll of the new selection.
	rec := f.h.do(fragment(postForm("/app/sources/contacts", f.sess, url.Values{"tick": {people.Href}}), "rail"))
	frag := decode[railFragment](t, rec)
	if _, team := railRow(t, frag.Body, "Team"); team.Tick != view.TickOff || team.Count != nil {
		t.Errorf("team after unticking = %+v", team)
	}
	if c := frag.Body.Sections[0].Rows[0]; c.Tick != view.TickPart {
		t.Errorf("combined row = %+v", c)
	}
	var bar view.PollBar
	if len(frag.Extra) != 1 || frag.Extra[0].Target != "poll" || json.Unmarshal(frag.Extra[0].Body, &bar) != nil || len(bar.Sources) != 1 {
		t.Fatalf("extra = %+v", frag.Extra)
	}
	if got := contactNames(combined(t, f.h, f.sess, "/app/contacts", contactsPoll)); got != "Ada Lovelace · People, Bob Builder · People" {
		t.Errorf("after unticking: %s", got)
	}

	tickRail(t, f.h, f.sess, "contacts")
	if got := selected(t, f.h, f.sess, view.KindContact); len(got) != 0 {
		t.Errorf("an empty form left ticks: %v", got)
	}
	if after := decode[view.ContactsListPage](t, f.h.do(get("/app/contacts", f.sess))); len(after.Groups) != 0 || after.Poll.TaskID != "" {
		t.Errorf("unticked everything, still shown: %+v", after.Groups)
	}
	if rec := f.h.do(postForm("/app/sources/files", f.sess, url.Values{})); rec.Code != http.StatusNotFound {
		t.Errorf("files have no combined view: %d", rec.Code)
	}
}

// S-14.1: events and tasks by time, notes by date, across calendars; a row is
// never grouped by the calendar it came from.
func TestCombinedAgendaTasksAndNotesMerge(t *testing.T) {
	srv := searchServer(t)
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work"})
	srv.AddCalendar(caldav.Calendar{Path: homeCal, Name: "Home"})
	srv.Seed(searchWork+"standup.ics", ics("BEGIN:VEVENT\nUID:standup\nDTSTAMP:20260101T000000Z\nDTSTART:20260908T100000Z\nSUMMARY:Standup\nEND:VEVENT"))
	srv.Seed(homeCal+"gym.ics", ics("BEGIN:VEVENT\nUID:gym\nDTSTAMP:20260101T000000Z\nDTSTART:20260908T090000Z\nSUMMARY:Gym\nEND:VEVENT"))
	srv.Seed(homeCal+"dinner.ics", ics("BEGIN:VEVENT\nUID:dinner\nDTSTAMP:20260101T000000Z\nDTSTART:20260909T180000Z\nSUMMARY:Dinner\nEND:VEVENT"))
	srv.Seed(homeCal+"later.ics", ics("BEGIN:VEVENT\nUID:later\nDTSTAMP:20260101T000000Z\nDTSTART:20260920T180000Z\nSUMMARY:Next month\nEND:VEVENT"))
	srv.Seed(searchWork+"report.ics", ics("BEGIN:VTODO\nUID:report\nDTSTAMP:20260101T000000Z\nDUE:20260910T120000Z\nSUMMARY:Report\nEND:VTODO"))
	srv.Seed(homeCal+"groceries.ics", ics("BEGIN:VTODO\nUID:groceries\nDTSTAMP:20260101T000000Z\nDUE:20260909T120000Z\nSUMMARY:Groceries\nEND:VTODO"))
	srv.Seed(searchWork+"old.ics", ics("BEGIN:VJOURNAL\nUID:old\nDTSTAMP:20260101T000000Z\nDTSTART:20260901T100000Z\nSUMMARY:Old\nEND:VJOURNAL"))
	srv.Seed(homeCal+"new.ics", ics("BEGIN:VJOURNAL\nUID:new\nDTSTAMP:20260101T000000Z\nDTSTART:20260905T100000Z\nSUMMARY:New\nEND:VJOURNAL"))

	f := connectFixture(t, srv, searchWork, nil)
	work, home := url.PathEscape(searchWork), url.PathEscape(homeCal)
	for _, seg := range []string{"calendar", "tasks", "notes"} {
		tickRail(t, f.h, f.sess, seg, "/app/"+seg+"/"+f.id+"/"+work, "/app/"+seg+"/"+f.id+"/"+home)
	}

	agenda := combined(t, f.h, f.sess, "/app/calendar?from=2026-09-07", func(p view.AgendaPage) *view.PollBar { return p.Poll })
	var events []string
	for _, d := range agenda.Days {
		for _, r := range d.Rows {
			events = append(events, string(r.Summary)+" · "+strings.SplitN(string(r.Stamp), " · ", 2)[1])
		}
	}
	if strings.Join(events, ", ") != "Gym · Home, Standup · Work, Dinner · Home" || len(agenda.Days) != 2 {
		t.Fatalf("agenda = %v (%d days)", events, len(agenda.Days))
	}
	if agenda.Bar.Range == nil || actionHref(t, agenda.Bar.Range.Presets, "range.next") != "/app/calendar?from=2026-09-14" {
		t.Errorf("range = %+v", agenda.Bar.Range)
	}
	if agenda.Rail.Sections[0].Rows[0].Count != nil {
		t.Error("the agenda reads a week, yet the rail counts the whole calendar")
	}

	tasks := combined(t, f.h, f.sess, "/app/tasks", func(p view.TasksPage) *view.PollBar { return p.Poll })
	if len(tasks.Groups) != 1 || len(tasks.Groups[0].Rows) != 2 ||
		tasks.Groups[0].Rows[0].Summary != "Groceries" || !strings.HasSuffix(string(tasks.Groups[0].Rows[0].Stamp), " · Home") ||
		tasks.Groups[0].Rows[1].Href != "/app/tasks/"+f.id+"/"+work+"/report" {
		t.Errorf("tasks = %+v", tasks.Groups)
	}
	if actionHref(t, tasks.Bar.Segments, "tasks.done") != "/app/tasks?show=done" {
		t.Errorf("segments = %+v", tasks.Bar.Segments)
	}

	notes := combined(t, f.h, f.sess, "/app/notes", func(p view.NotesPage) *view.PollBar { return p.Poll })
	if len(notes.Groups) != 1 || len(notes.Groups[0].Rows) != 2 || notes.Groups[0].Rows[0].Title != "New" || notes.Groups[0].Rows[1].Title != "Old" {
		t.Errorf("notes = %+v", notes.Groups)
	}
	if c := notes.Rail.Sections[0].Rows[0]; c.Count == nil || *c.Count != 2 || c.Tick != view.TickOn {
		t.Errorf("notes combined row = %+v", c)
	}
}

// S-21, S-22, S-6.1: a ticked collection re-discovery marked gone stays in the
// rail as gone and is not asked; a server that does not answer is unavailable,
// not gone, the others' records stay, and none of it is a 500.
func TestCombinedGoneIsNotUnavailable(t *testing.T) {
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.AddAddressBook(carddav.AddressBook{Path: teamBook, Name: "Team"})
	srv.Seed(searchBook+"ada.vcf", adaCard())
	f := connectFixture(t, srv, searchBook, nil)
	tickRail(t, f.h, f.sess, "contacts", f.path("contacts"), "/app/contacts/"+f.id+"/"+url.PathEscape(teamBook))

	// Team is gone after an applied re-discovery; a second connection points
	// at a port nothing listens on.
	key, _ := f.sess.DataKey()
	blob, _ := f.h.users.Catalog(f.sess.AccountID(), key)
	for i := range blob.Collections {
		if blob.Collections[i].Path == teamBook {
			blob.Collections[i].Gone = true
		}
	}
	const dead = "a-dead0000001"
	blob.Connections = append(blob.Connections, sealed.Connection{ID: dead, Server: "http://127.0.0.1:1/dav/"})
	blob.Collections = append(blob.Collections, sealed.Collection{AccountID: dead, Path: "/dav/lost/", Name: "Lost", Kinds: []view.Kind{view.KindContact}})
	blob.Selected = append(blob.Selected, sealed.Selection{Kind: view.KindContact, AccountID: dead, Path: "/dav/lost/"})
	sealedBlob, err := sealed.Seal(key, blob, f.sess.Login())
	if err != nil {
		t.Fatal(err)
	}
	if err := f.h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		st.UserByID(f.sess.AccountID()).SealedBlob = sealedBlob
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}

	page := combined(t, f.h, f.sess, "/app/contacts", contactsPoll)
	if len(page.Poll.Sources) != 2 {
		t.Fatalf("sources = %+v: a gone collection was asked or a live one dropped", page.Poll.Sources)
	}
	if n := srv.CountRequests("REPORT", teamBook); n != 0 {
		t.Errorf("REPORT to the gone collection: %d", n)
	}
	if lost := sourceNamed(t, *page.Poll, "Lost"); lost.State != view.SourceUnavailable || lost.Retry == nil {
		t.Errorf("lost = %+v", lost)
	}
	if got := contactNames(page); got != "Ada Lovelace · People" {
		t.Errorf("the answering source's records = %s", got)
	}
	sec, team := railRow(t, page.Rail, "Team")
	if team.State != view.CollectionGone || team.Tick != view.TickOn || team.Href != "" || team.Count != nil || sec.State != view.CollectionOK {
		t.Errorf("team = %+v in %+v", team, sec.State)
	}
	sec, lost := railRow(t, page.Rail, "Lost")
	if lost.State != view.CollectionUnavailable || sec.State != view.CollectionUnavailable {
		t.Errorf("lost = %+v in %+v", lost, sec.State)
	}
	// The note counts what can be shown: a gone collection is not among them.
	if !strings.Contains(string(page.Rail.Note), "2 of 2 collections ticked") {
		t.Errorf("note = %q", page.Rail.Note)
	}

	// A plain Stop or Retry returns to the combined view of that task.
	retry := sourceNamed(t, *page.Poll, "Lost").Retry.Href
	if rec := f.h.do(postForm(retry, f.sess, url.Values{})); rec.Code != http.StatusSeeOther ||
		rec.Header().Get("Location") != "/app/contacts?task="+page.Poll.TaskID {
		t.Errorf("retry: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	waitPoll(t, f.h, f.sess, page.Poll.TaskID, nil)

	// Unticking a gone collection works; ticking it again does not.
	teamPath := "/app/contacts/" + f.id + "/" + url.PathEscape(teamBook)
	tickRail(t, f.h, f.sess, "contacts", f.path("contacts"))
	tickRail(t, f.h, f.sess, "contacts", f.path("contacts"), teamPath)
	if got := selected(t, f.h, f.sess, view.KindContact); len(got) != 1 {
		t.Errorf("a gone collection was ticked again: %v", got)
	}
}

// S-14: creating from the combined view names no target; the sheet offers
// every collection of the kind, a read-only one closed with its reason.
func TestNewFromCombinedChoosesTheTarget(t *testing.T) {
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.AddAddressBook(carddav.AddressBook{Path: teamBook, Name: "Team", ReadOnly: true})
	f := connectFixture(t, srv, searchBook, nil)
	tickRail(t, f.h, f.sess, "contacts", f.path("contacts"), "/app/contacts/"+f.id+"/"+url.PathEscape(teamBook))

	page := combined(t, f.h, f.sess, "/app/contacts", contactsPoll)
	href := actionHref(t, page.Bar.Actions, "record.new")
	if href != f.path("contacts")+"/new?pick=1" || !page.Bar.Actions[0].Enabled {
		t.Fatalf("new = %+v", page.Bar.Actions)
	}
	type sheetFragment struct {
		Body view.NewSheet `json:"body"`
	}
	sheet := decode[sheetFragment](t, f.h.do(fragment(get(href, f.sess), "sheet"))).Body
	if sheet.Target.Chosen != "" || len(sheet.Target.Options) != 2 || sheet.Actions[0].Enabled || sheet.Actions[0].Reason == "" {
		t.Fatalf("sheet = %+v", sheet)
	}
	if o := sheet.Target.Options[1]; !o.ReadOnly || o.Reason == "" || o.ID != "/app/contacts/"+f.id+"/"+url.PathEscape(teamBook)+"/new" {
		t.Errorf("read-only option = %+v", o)
	}
	own := decode[sheetFragment](t, f.h.do(fragment(get(f.path("contacts")+"/new", f.sess), "sheet"))).Body
	if own.Target.Chosen != sheet.Target.Options[0].ID || !own.Actions[0].Enabled {
		t.Errorf("from one collection = %+v", own)
	}
	// Without scripts the same choice is the page, not the form of a guessed
	// collection (U-8).
	plain := decode[view.NewSheet](t, f.h.do(get(href, f.sess)))
	if plain.Kind != view.KindContact || plain.Target.Chosen != "" || len(plain.Target.Options) != 2 || plain.Actions[0].Enabled {
		t.Errorf("new without scripts = %+v", plain)
	}
}

// C-6.3: a count once shown does not disappear after a write in its
// collection. A new record adds one and a deleted one takes one away, and a
// list that then fails to read still shows the count the writes left.
func TestCountSurvivesWrites(t *testing.T) {
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nEND:VCARD\n")
	list := f.path("contacts")
	tickRail(t, f.h, f.sess, "contacts", list)
	count := func() string {
		t.Helper()
		page := decode[view.ContactsListPage](t, f.h.do(get(list, f.sess)))
		_, row := railRow(t, page.Rail, "People")
		if row.Count == nil {
			return "-"
		}
		return strconv.Itoa(*row.Count)
	}
	unread := func() string {
		t.Helper()
		f.srv.Refuse("REPORT", http.StatusServiceUnavailable, "")
		defer f.srv.Accept("REPORT")
		return count()
	}
	if n := count(); n != "1" {
		t.Fatalf("after the list = %s, want 1", n)
	}

	form := formFields(decode[view.ContactEditPage](t, f.h.do(get(list+"/new", f.sess))).Form)
	form.Set("fn", "Bob Builder")
	if resp := f.h.do(postForm(list+"/new", f.sess, form)); resp.Code != http.StatusSeeOther {
		t.Fatalf("new: %d %s", resp.Code, resp.Body.String())
	}
	if n := unread(); n != "2" {
		t.Errorf("after a new record = %s, want 2", n)
	}

	del := actionHref(t, decode[view.ContactPage](t, f.h.do(get(list+"/ada", f.sess))).Actions, "record.delete")
	if resp := f.h.do(postForm(del, f.sess, url.Values{})); resp.Code != http.StatusSeeOther {
		t.Fatalf("delete: %d %s", resp.Code, resp.Body.String())
	}
	if n := unread(); n != "1" {
		t.Errorf("after a delete = %s, want 1", n)
	}
}
