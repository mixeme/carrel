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
	if n := len(rowNamed(t, sheet.Form, "colour").Options); n != 6 {
		t.Errorf("palette = %d tones, want the six of C-10", n)
	}
	// The hint in pieces since Кл3, the address in mono.
	address := rowNamed(t, sheet.Form, "address")
	if len(address.HintRuns) == 0 || address.HintRuns[0].Mark != view.RunMono || !strings.Contains(string(address.HintRuns[0].Text), davtest.CalendarHome) {
		t.Errorf("address hint = %+v, want the home it goes into, in mono", address.HintRuns)
	}

	form := url.Values{"account": {f.id}, "name": {"Дни рождения"}, "colour": {"#6B4E7A"}, "components": {"VTODO", "VEVENT"}}
	rec := f.h.do(postForm("/app/collections/new?kind=event", f.sess, form))
	path := davtest.CalendarHome + "dni-rozhdeniya/"
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/app/calendar/"+f.id+"/"+url.PathEscape(path) {
		t.Fatalf("create: %d %q %s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	cal, ok := f.srv.Calendar(path)
	if !ok || cal.Name != "Дни рождения" || strings.Join(cal.SupportedComponentSet, " ") != "VEVENT VTODO" {
		t.Errorf("server calendar = %+v (found %v)", cal, ok)
	}
	if c := f.srv.Colour(path); c != "#6b4e7a" {
		t.Errorf("calendar-color = %q", c)
	}
	entry, ok := catalogOf(t, f).Collection(f.id, path)
	if !ok || entry.Name != "Дни рождения" || entry.Colour != "#6b4e7a" || !entry.Has(view.KindEvent) || !entry.Has(view.KindTask) || entry.Has(view.KindNote) {
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

// docs/servers.md, SV-41: a server that makes the calendar and drops the
// colour the MKCALENDAR carried, saying so in a propstat of the answer and
// still calling it 201. The colour is sent again on its own, and the catalog
// keeps what the server holds — a swatch kept here alone would be one no
// other client draws, while C-10 went on calling it the colour the server
// sent.
func TestCreateSettlesAColourTheServerDroppedAtCreation(t *testing.T) {
	f := calendarFixture(t, nil)
	f.srv.CreateDropsColour = true

	rec := f.h.do(postForm("/app/collections/new?kind=event", f.sess,
		url.Values{"account": {f.id}, "name": {"Trips"}, "colour": {"#6B4E7A"}, "components": {"VEVENT"}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	path := davtest.CalendarHome + "trips/"
	if n := f.srv.CountRequests("PROPPATCH", path); n != 1 {
		t.Errorf("PROPPATCH requests = %d, want the one that sends the dropped colour again", n)
	}
	if c := f.srv.Colour(path); c != "#6b4e7a" {
		t.Errorf("the server holds %q, want the colour sent again after the create dropped it", c)
	}
	entry, ok := catalogOf(t, f).Collection(f.id, path)
	if !ok || entry.Colour != "#6b4e7a" {
		t.Errorf("catalog colour = %q (found %v), want the one the server holds", entry.Colour, ok)
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
	// The address from the server's root, its start an ellipsis; nothing
	// under the colour (s-profile-7; Кл2, Кл9).
	if string(sheet.Address) != "…"+f.col || rowNamed(t, sheet.Form, "colour").Hint != "" {
		t.Errorf("address %q, colour's hint %q", sheet.Address, rowNamed(t, sheet.Form, "colour").Hint)
	}

	u, _ := url.Parse(href)
	rec := f.h.do(postForm(u.Path+"?"+u.RawQuery, f.sess, url.Values{"name": {"Office"}, "colour": {"#3f6e8c"}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body.String())
	}
	if cal, _ := f.srv.Calendar(f.col); cal.Name != "Office" || f.srv.Colour(f.col) != "#3f6e8c" {
		t.Errorf("server = %q %q", cal.Name, f.srv.Colour(f.col))
	}
	if entry, ok := catalogOf(t, f).Collection(f.id, f.col); !ok || entry.Name != "Office" || entry.Colour != "#3f6e8c" {
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
	form := url.Values{"account": {f.id}, "name": {"Дни рождения"}, "colour": {"#6B4E7A"}, "components": {"VEVENT"}}
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

// docs/servers.md, SV-43: a server that holds every component in every
// calendar whatever the create asked for, with nothing to send again — the
// set is protected, so a PROPPATCH of it is refused. The kinds decide which
// screens a collection appears on, so keeping the narrower set the person
// chose would hide records that are really there: a task written into that
// calendar by another client would show nowhere. The catalog takes the
// server's answer, the way discovery always has.
func TestCreateTakesTheKindsTheServerReports(t *testing.T) {
	f := calendarFixture(t, nil)
	f.srv.CreateIgnoresComponentSet = true

	rec := f.h.do(postForm("/app/collections/new?kind=event", f.sess,
		url.Values{"account": {f.id}, "name": {"Trips"}, "components": {"VEVENT"}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	path := davtest.CalendarHome + "trips/"
	entry, ok := catalogOf(t, f).Collection(f.id, path)
	if !ok {
		t.Fatalf("not in the catalog: %+v", catalogOf(t, f).Collections)
	}
	if !entry.Has(view.KindEvent) || !entry.Has(view.KindTask) || !entry.Has(view.KindNote) {
		t.Errorf("catalog kinds = %v, want the three the server reports", entry.Kinds)
	}
	if strings.Join(entry.Components, " ") != "VEVENT VTODO VJOURNAL VFREEBUSY" {
		t.Errorf("catalog components = %v, want what the server holds", entry.Components)
	}
}

// After a refusal the sheet is the server's answer, and its foot is the way
// on (s-profile-6): «Back to the form» — the form as it was left, no
// refusal, nothing sent — and «Try again», the same form sent once more.
func TestRefusedCreateOffersBackAndAgain(t *testing.T) {
	f := calendarFixture(t, nil)
	f.srv.Refuse("MKCALENDAR", http.StatusForbidden, `<?xml version="1.0"?><D:error xmlns:D="DAV:"><D:need-privileges/></D:error>`)
	form := url.Values{"account": {f.id}, "name": {"Trips"}, "components": {"VEVENT"}}
	sheet := decode[view.CollectionNewSheet](t, f.h.do(postForm("/app/collections/new?kind=event", f.sess, form)))
	var labels []string
	for _, a := range sheet.Actions {
		labels = append(labels, string(a.Label))
	}
	if strings.Join(labels, "|") != "Back to the form|Try again|Cancel" || !sheet.Actions[0].Primary || sheet.Actions[1].Primary {
		t.Fatalf("actions after a refusal = %+v", sheet.Actions)
	}
	for _, a := range sheet.Actions[:2] {
		if a.Method != "POST" || !a.Enabled || a.Href == "" {
			t.Errorf("%s does not send the form: %+v", a.Label, a)
		}
	}
	tries := f.srv.CountRequests("MKCALENDAR", "")
	back := decode[view.CollectionNewSheet](t, f.h.do(postForm(sheet.Actions[0].Href, f.sess, form)))
	if back.Failure != nil || rowNamed(t, back.Form, "name").Value != "Trips" || len(back.Actions) != 2 || back.Actions[0].Label != "Create" {
		t.Errorf("back to the form = failure %+v, actions %+v", back.Failure, back.Actions)
	}
	if n := f.srv.CountRequests("MKCALENDAR", "") - tries; n != 0 {
		t.Errorf("Back to the form sent %d requests", n)
	}
	again := decode[view.CollectionNewSheet](t, f.h.do(postForm(sheet.Actions[1].Href, f.sess, form)))
	if again.Failure == nil || f.srv.CountRequests("MKCALENDAR", "")-tries != 1 {
		t.Errorf("Try again: failure %+v, sent %d", again.Failure, f.srv.CountRequests("MKCALENDAR", "")-tries)
	}
}
