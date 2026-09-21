package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

func menuHref(t *testing.T, menu []view.MenuItem, id string) string {
	t.Helper()
	for _, m := range menu {
		if m.ID == id {
			return m.Href
		}
	}
	t.Fatalf("no menu item %q in %+v", id, menu)
	return ""
}

func lastAudit(h *harness) (what, detail string) {
	snap := h.st.Snapshot()
	e := snap.Audit[len(snap.Audit)-1]
	return e.What, e.Detail
}

// S-12.1–12.2: a new calendar takes its address from the name, carries name,
// colour and component set in one MKCALENDAR, joins the catalog at exactly the
// path discovery reports, and a second one with the same name gets a suffix.
func TestNewCalendarTakesItsAddressFromTheName(t *testing.T) {
	f := calendarFixture(t, nil)

	sheet := decode[view.CollectionNewSheet](t, f.h.do(get("/app/collections/new?kind=task&a="+f.id, f.sess)))
	if sheet.Kind != view.KindTask || sheet.Target.Chosen != f.id || sheet.Failure != nil {
		t.Fatalf("sheet = %+v", sheet)
	}
	var selected []string
	for _, o := range rowNamed(t, sheet.Form, "components").Options {
		if o.Selected {
			selected = append(selected, o.Value)
		}
	}
	if strings.Join(selected, " ") != "VTODO" {
		t.Errorf("components selected = %v, want the kind the sheet was opened for", selected)
	}
	if n := len(rowNamed(t, sheet.Form, "colour").Options); n != 8 {
		t.Errorf("palette = %d tones, want 8", n)
	}
	if hint := string(rowNamed(t, sheet.Form, "address").Hint); !strings.Contains(hint, davtest.CalendarHome) {
		t.Errorf("address hint = %q, want the home it goes into", hint)
	}

	form := url.Values{"account": {f.id}, "name": {"Дни рождения"}, "colour": {"#6B5C73"}, "components": {"VTODO", "VEVENT"}}
	rec := f.h.do(postForm("/app/collections/new?kind=event", f.sess, form))
	path := davtest.CalendarHome + "dni-rozhdeniya/"
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/app/calendar/"+f.id+"/"+url.PathEscape(path) {
		t.Fatalf("create: %d %q %s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	cal, ok := f.srv.Calendar(path)
	if !ok || cal.Name != "Дни рождения" || strings.Join(cal.SupportedComponentSet, " ") != "VEVENT VTODO" {
		t.Errorf("server calendar = %+v (found %v)", cal, ok)
	}
	if c := f.srv.Colour(path); c != "#6b5c73" {
		t.Errorf("calendar-color = %q", c)
	}
	entry, ok := catalogOf(t, f).Collection(f.id, path)
	if !ok || entry.Name != "Дни рождения" || entry.Colour != "#6b5c73" || !entry.Has(view.KindEvent) || !entry.Has(view.KindTask) || entry.Has(view.KindNote) {
		t.Errorf("catalog entry = %+v (found %v)", entry, ok)
	}
	if what, detail := lastAudit(f.h); what != "collection.create" || detail != f.id {
		t.Errorf("audit = %s %s", what, detail)
	}
	decode[view.AgendaPage](t, f.h.do(get(rec.Header().Get("Location"), f.sess)))

	rec = f.h.do(postForm("/app/collections/new?kind=event", f.sess, form))
	if loc := rec.Header().Get("Location"); !strings.HasSuffix(loc, url.PathEscape(davtest.CalendarHome+"dni-rozhdeniya-2/")) {
		t.Errorf("same name again went to %q, want the -2 address", loc)
	}

	if noticeTitled(rediscoverPage(t, f).Notices, "Nothing appeared or disappeared") == nil {
		t.Error("re-discovery sees the new calendars as new: the catalog path differs from the server's")
	}
}

// docs/servers.md, SV-19: a server that answers 201 and puts the collection at
// an address of its own, saying nothing about it. What goes into the catalog
// is where it went, not where it was asked to go: an entry that points at
// nothing opens nothing, and every write after it would go to the wrong place.
func TestCreateWhereTheServerPicksTheAddress(t *testing.T) {
	f := calendarFixture(t, nil)
	f.srv.CreateAssignsItsOwnAddress = true
	before := map[string]bool{}
	for _, c := range catalogOf(t, f).Collections {
		before[c.Path] = true
	}

	rec := f.h.do(postForm("/app/collections/new?kind=event", f.sess,
		url.Values{"account": {f.id}, "name": {"Trips"}, "components": {"VEVENT"}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	asked := davtest.CalendarHome + "trips/"
	if _, ok := f.srv.Calendar(asked); ok {
		t.Fatal("the fixture kept the address it was given: this test would hold nothing")
	}

	var placed string
	for _, c := range catalogOf(t, f).Collections {
		if !before[c.Path] {
			placed = c.Path
		}
	}
	if placed == "" || placed == asked {
		t.Fatalf("the catalog holds %q, want the address the server picked", placed)
	}
	if cal, ok := f.srv.Calendar(placed); !ok || cal.Name != "Trips" {
		t.Errorf("the catalog points at %q, which the server does not hold as Trips: %+v %v", placed, cal, ok)
	}
	if loc, want := rec.Header().Get("Location"), "/app/calendar/"+f.id+"/"+url.PathEscape(placed); loc != want {
		t.Errorf("went to %q, want %q", loc, want)
	}
	// And the page at that address opens.
	decode[view.AgendaPage](t, f.h.do(get(rec.Header().Get("Location"), f.sess)))
}

// An address typed by hand is kept as typed, refused when it is not one
// segment, and not suffixed when taken: the person chose it.
func TestNewAddressBookWithAddressByHand(t *testing.T) {
	f := contactsFixture(t, false)
	post := func(v url.Values) view.CollectionNewSheet {
		t.Helper()
		v.Set("account", f.id)
		return decode[view.CollectionNewSheet](t, f.h.do(postForm("/app/collections/new?kind=contact", f.sess, v)))
	}

	if sheet := post(url.Values{"name": {"Team"}, "address": {"../team"}}); rowNamed(t, sheet.Form, "address").Error == "" {
		t.Error("an address with a slash was not refused")
	}
	if sheet := post(url.Values{"name": {"Team"}, "address": {"people"}}); rowNamed(t, sheet.Form, "address").Error == "" ||
		rowNamed(t, sheet.Form, "name").Value != "Team" {
		t.Errorf("a taken address: %+v", sheet.Form)
	}
	if sheet := post(url.Values{"name": {""}}); rowNamed(t, sheet.Form, "name").Error == "" {
		t.Error("an empty name was not refused")
	}
	if n := f.srv.CountRequests("MKCOL", ""); n != 0 {
		t.Fatalf("%d MKCOL sent for refused forms", n)
	}

	rec := f.h.do(postForm("/app/collections/new?kind=contact", f.sess, url.Values{"account": {f.id}, "name": {"Team & Co"}, "address": {"Team_1"}}))
	path := davtest.AddressBookHome + "Team_1/"
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/app/contacts/"+f.id+"/"+url.PathEscape(path) {
		t.Fatalf("create: %d %q %s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	if book, ok := f.srv.AddressBook(path); !ok || book.Name != "Team & Co" {
		t.Errorf("server book = %+v (found %v)", book, ok)
	}
	if entry, ok := catalogOf(t, f).Collection(f.id, path); !ok || !entry.Has(view.KindContact) {
		t.Errorf("catalog entry = %+v (found %v)", entry, ok)
	}

	// From a connection card the kind is not known yet: the sheet asks.
	page := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
	sheet := decode[view.CollectionNewSheet](t, f.h.do(get(actionHref(t, page.Accounts[0].Actions, "collection.new"), f.sess)))
	if kind := rowNamed(t, sheet.Form, "kind"); len(kind.Options) != 2 || sheet.Target.Chosen != f.id {
		t.Errorf("kind row %+v, chosen %q", kind, sheet.Target.Chosen)
	}
}

// A refusal is the content of the sheet — method, address, code, body — and
// leaves nothing: no collection, no catalog entry, no journal line.
func TestRefusedCreateLeavesNothing(t *testing.T) {
	f := calendarFixture(t, nil)
	f.srv.Refuse("MKCALENDAR", http.StatusForbidden, `<?xml version="1.0"?><D:error xmlns:D="DAV:"><D:need-privileges/></D:error>`)
	before := len(catalogOf(t, f).Collections)

	sheet := decode[view.CollectionNewSheet](t, f.h.do(postForm("/app/collections/new?kind=event", f.sess,
		url.Values{"account": {f.id}, "name": {"Trips"}, "components": {"VEVENT"}})))
	n := sheet.Failure
	if n == nil || n.Title != "This account can read here but not create" || n.Details == nil ||
		!strings.HasPrefix(string(n.Details.Summary), "MKCALENDAR "+davtest.CalendarHome+"trips/") {
		t.Fatalf("failure = %+v", n)
	}
	var lines []string
	for _, l := range n.Details.Lines {
		lines = append(lines, string(l))
	}
	mustContain(t, strings.Join(lines, "\n"), "→ 403", "need-privileges")
	if rowNamed(t, sheet.Form, "name").Value != "Trips" {
		t.Error("the form lost what was entered")
	}
	if got := len(catalogOf(t, f).Collections); got != before || auditCount(f.h, "collection.create") != 0 {
		t.Errorf("catalog %d → %d entries, create lines %d", before, got, auditCount(f.h, "collection.create"))
	}
}

// Renaming writes the label and nothing else: displayname and colour in one
// PROPPATCH, no MOVE, the address as it was (S-12.2).
func TestRenameWritesTheLabelNotTheAddress(t *testing.T) {
	f := calendarFixture(t, nil)
	page := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
	var href string
	for _, row := range page.Accounts[0].Collections {
		if string(row.Path) == f.col {
			href = menuHref(t, row.Menu, "collection.rename")
		}
	}
	sheet := decode[view.CollectionRenameSheet](t, f.h.do(get(href, f.sess)))
	if !strings.HasSuffix(string(sheet.Address), f.col) || rowNamed(t, sheet.Form, "name").Value != "Work" || sheet.Kind != view.KindEvent {
		t.Fatalf("sheet = %+v", sheet)
	}

	u, _ := url.Parse(href)
	rec := f.h.do(postForm(u.Path+"?"+u.RawQuery, f.sess, url.Values{"name": {"Office"}, "colour": {"#5c736b"}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body.String())
	}
	if cal, _ := f.srv.Calendar(f.col); cal.Name != "Office" || f.srv.Colour(f.col) != "#5c736b" {
		t.Errorf("server = %q %q", cal.Name, f.srv.Colour(f.col))
	}
	if entry, ok := catalogOf(t, f).Collection(f.id, f.col); !ok || entry.Name != "Office" || entry.Colour != "#5c736b" {
		t.Errorf("catalog = %+v (found %v)", entry, ok)
	}
	if n := f.srv.CountRequests("MOVE", ""); n != 0 {
		t.Errorf("%d MOVE sent for a rename", n)
	}
	if what, detail := lastAudit(f.h); what != "collection.rename" || detail != f.id {
		t.Errorf("audit = %s %s", what, detail)
	}

	f.srv.Refuse("PROPPATCH", http.StatusForbidden, "")
	sheet = decode[view.CollectionRenameSheet](t, f.h.do(postForm(u.Path+"?"+u.RawQuery, f.sess, url.Values{"name": {"Elsewhere"}})))
	if sheet.Failure == nil || sheet.Failure.Title != "This account can read here but not change it" || rowNamed(t, sheet.Form, "name").Value != "Elsewhere" {
		t.Errorf("refused rename = %+v", sheet)
	}
	if entry, _ := catalogOf(t, f).Collection(f.id, f.col); entry.Name != "Office" {
		t.Errorf("a refused rename changed the catalog to %q", entry.Name)
	}

	for _, q := range []string{"a=" + f.id + "&c=" + url.QueryEscape(davtest.CalendarHome+"nope/"), "a=a-nobody&c=" + url.QueryEscape(f.col)} {
		if rec := f.h.do(get("/app/collections/rename?"+q, f.sess)); rec.Code != http.StatusNotFound {
			t.Errorf("rename?%s: %d, want 404", q, rec.Code)
		}
	}
}

// A connection that dies after the server has taken the MKCOL, and before
// the answer. The handler asks the home for its children first, so the
// write goes out on a connection the pool had kept idle and the guard sends
// it again (SV-9); the server, which already made the collection, answers
// 405. Until 21.09.2026 Carrel turned that into «The server would not make
// it» over a collection it had just made — the full live run caught it on
// Nextcloud (SV-39).
func TestACollectionMadeOnAConnectionThatDiedIsStillMade(t *testing.T) {
	f := calendarFixture(t, nil)

	f.srv.HangUpOnce("MKCALENDAR")
	form := url.Values{"account": {f.id}, "name": {"Дни рождения"}, "colour": {"#6B5C73"}, "components": {"VEVENT"}}
	rec := f.h.do(postForm("/app/collections/new?kind=event", f.sess, form))
	path := davtest.CalendarHome + "dni-rozhdeniya/"
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "/app/calendar/"+f.id+"/"+url.PathEscape(path) {
		t.Fatalf("create went to %q, want the collection the server made", loc)
	}
	if cal, ok := f.srv.Calendar(path); !ok || cal.Name != "Дни рождения" {
		t.Errorf("server calendar = %+v (found %v)", cal, ok)
	}
	if entry, ok := catalogOf(t, f).Collection(f.id, path); !ok || entry.Name != "Дни рождения" {
		t.Errorf("catalog entry = %+v (found %v)", entry, ok)
	}
}
