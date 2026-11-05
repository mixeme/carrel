package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// colMoveFixture is S-12.3's ground: Work on the first account with an event
// that has an overridden instance and a task, and a second account.
type colMoveFixture struct {
	davFixture
	other   *davtest.Server
	otherID string
}

var workSeries = ics("BEGIN:VEVENT\nUID:standup\nDTSTAMP:20260101T000000Z\nDTSTART:20261006T100000Z\nRRULE:FREQ=WEEKLY;COUNT=3\nSUMMARY:Standup\nX-CUSTOM:keep\nEND:VEVENT\n" +
	"BEGIN:VEVENT\nUID:standup\nDTSTAMP:20260101T000000Z\nRECURRENCE-ID:20261013T100000Z\nDTSTART:20261013T120000Z\nSUMMARY:Standup moved\nEND:VEVENT")

var workTask = ics("BEGIN:VTODO\nUID:report\nDTSTAMP:20260101T000000Z\nSUMMARY:Write report\nEND:VTODO")

func newColMoveFixture(t *testing.T) colMoveFixture {
	t.Helper()
	f := calendarFixture(t, nil, "standup.ics", workSeries, "report.ics", workTask)
	f.srv.SetColour(f.col, "#6b4e7a")
	other := davtest.New(t)
	other.Login, other.Password = "bob", "other"
	other.AddCalendar(caldav.Calendar{Path: davtest.CalendarHome + "home/", Name: "Home"})
	return colMoveFixture{davFixture: f, other: other, otherID: addAccount(t, f, other)}
}

func colSheetHref(f davFixture, to string, copying bool) string {
	q := url.Values{"a": {f.id}, "c": {f.col}}
	if to != "" {
		q.Set("to", to)
	}
	if copying {
		q.Set("copy", "1")
	}
	return "/app/collections/move?" + q.Encode()
}

func confirmCollection(t *testing.T, f colMoveFixture, copying bool) *http.Request {
	t.Helper()
	// The sheet's form as a browser sends it (Кл11): its hidden fields and
	// the server chosen under Target.Field.
	sheet := decode[view.CollectionMoveSheet](t, f.h.do(get(colSheetHref(f.davFixture, "", copying), f.sess)))
	c := sheet.Actions[0]
	if !c.Enabled || c.Method != "POST" || !c.Submits {
		t.Fatalf("confirm = %+v", c)
	}
	form := formFields(sheet.Form)
	form.Set(sheet.Target.Field, f.otherID)
	return postForm(sheet.Form.Action, f.sess, form)
}

func connectionRow(t *testing.T, page view.ConnectionsPage, account, path string) view.CollectionRow {
	t.Helper()
	for _, a := range page.Accounts {
		for _, row := range a.Collections {
			if string(row.Path) == path && strings.Contains(actionsOf(row.Menu), account) {
				return row
			}
		}
	}
	t.Fatalf("no row %s in %+v", path, page.Accounts)
	return view.CollectionRow{}
}

func actionsOf(menu []view.MenuItem) string {
	var b strings.Builder
	for _, m := range menu {
		b.WriteString(m.Href + " ")
	}
	return b.String()
}

func hasMenu(menu []view.MenuItem, id string) bool {
	for _, m := range menu {
		if m.ID == id {
			return true
		}
	}
	return false
}

// S-12.3: with one connection there is nowhere to go and no menu item; with a
// second one the sheet names both, the current closed, and says before the
// start that the address changes.
func TestCollectionMoveSheetSaysBeforeItStarts(t *testing.T) {
	alone := calendarFixture(t, nil)
	page := decode[view.ConnectionsPage](t, alone.h.do(get("/app/settings/connections", alone.sess)))
	if row := page.Accounts[0].Collections[0]; hasMenu(row.Menu, "collection.move") {
		t.Errorf("one connection, menu = %+v", row.Menu)
	}
	if rec := alone.h.do(get(colSheetHref(alone, "", false), alone.sess)); rec.Code != http.StatusNotFound {
		t.Errorf("sheet with nowhere to go: %d, want 404", rec.Code)
	}

	f := newColMoveFixture(t)
	page = decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
	row := connectionRow(t, page, f.id, f.col)
	sheet := decode[view.CollectionMoveSheet](t, f.h.do(get(menuHref(t, row.Menu, "collection.move"), f.sess)))
	if sheet.Kind != view.KindEvent || sheet.Copy || len(sheet.Target.Options) != 2 || !strings.HasSuffix(string(sheet.Subject), " · Work") {
		t.Fatalf("sheet = %+v", sheet)
	}
	if o := sheet.Target.Options[0]; !o.ReadOnly || o.Reason != "already there" {
		t.Errorf("current connection = %+v", o)
	}
	// Opened with nothing chosen, the server it can go to is (Кл12,
	// s-profile-8; until v0.2.274 Move was off, «Choose a server.»), and the
	// sheet is a form whose radio names it (Кл11).
	if c := sheet.Actions[0]; !c.Enabled || !c.Submits || !strings.Contains(plainText(sheet.Warning), "old address") ||
		sheet.Target.Chosen != sheet.Target.Options[1].ID || sheet.Target.Field != "to" || sheet.Target.Options[1].Value != f.otherID {
		t.Errorf("opened: confirm %+v, target %+v, warning %q", c, sheet.Target, sheet.Warning)
	}
	chosen := decode[view.CollectionMoveSheet](t, f.h.do(get(sheet.Target.Options[1].ID, f.sess)))
	if !chosen.Actions[0].Enabled || !strings.Contains(plainText(chosen.Warning), "old address") || chosen.Target.Chosen != sheet.Target.Options[1].ID {
		t.Errorf("chosen = %+v", chosen)
	}
	// A server chosen by its address that is no target says so on Move.
	here := decode[view.CollectionMoveSheet](t, f.h.do(get(sheet.Target.Options[0].ID, f.sess)))
	if c := here.Actions[0]; c.Enabled || c.Reason == "" {
		t.Errorf("the current server chosen: confirm %+v", c)
	}
	copySheet := decode[view.CollectionMoveSheet](t, f.h.do(get(colSheetHref(f.davFixture, f.otherID, true), f.sess)))
	if !copySheet.Copy || copySheet.Actions[0].ID != "collection.copy.confirm" || strings.Contains(plainText(copySheet.Warning), "deletes") {
		t.Errorf("copy sheet = %+v", copySheet)
	}
}

// A move makes the collection there with its name, colour and components,
// copies every record whole, deletes the original and carries the ticks.
func TestCollectionMoveCopiesThenDeletes(t *testing.T) {
	f := newColMoveFixture(t)
	key, _ := f.sess.DataKey()
	if err := f.h.users.SetSelection(f.sess.AccountID(), key, view.KindEvent, []sealed.Selection{{Kind: view.KindEvent, AccountID: f.id, Path: f.col}}); err != nil {
		t.Fatal(err)
	}

	resp := f.h.do(confirmCollection(t, f, false))
	newPath := davtest.CalendarHome + "work/"
	// Opened from nowhere in particular, done in Connections (note 112 p. 6).
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != "/app/settings/connections" {
		t.Fatalf("move: %d %q %s", resp.Code, resp.Header().Get("Location"), resp.Body.String())
	}
	cal, ok := f.other.Calendar(newPath)
	if !ok || cal.Name != "Work" || f.other.Colour(newPath) != "#6b4e7a" {
		t.Errorf("new calendar = %+v colour %q", cal, f.other.Colour(newPath))
	}
	series, _, ok := f.other.Object(newPath + "standup.ics")
	if !ok {
		t.Fatalf("paths there: %v", f.other.Paths(newPath))
	}
	mustContain(t, series, "RRULE:FREQ=WEEKLY;COUNT=3", "RECURRENCE-ID:20261013T100000Z", "SUMMARY:Standup moved", "X-CUSTOM:keep")
	if task, _, ok := f.other.Object(newPath + "report.ics"); !ok || !strings.Contains(task, "UID:report") {
		t.Errorf("the task did not travel: %q", task)
	}
	if _, ok := f.srv.Calendar(f.col); ok {
		t.Error("the original calendar is still there")
	}

	blob := catalogOf(t, f.davFixture)
	if _, ok := blob.Collection(f.id, f.col); ok {
		t.Error("the catalog still holds the original")
	}
	entry, ok := blob.Collection(f.otherID, newPath)
	if !ok || !entry.Has(view.KindEvent) || !entry.Has(view.KindTask) {
		t.Errorf("new entry = %+v (found %v)", entry, ok)
	}
	if len(blob.Selected) != 1 || blob.Selected[0].AccountID != f.otherID || blob.Selected[0].Path != newPath {
		t.Errorf("ticks = %+v, want the move's", blob.Selected)
	}
	if what, _ := lastAudit(f.h); what != "collection.move" {
		t.Errorf("audit = %s", what)
	}
}

// A taken address there gets a suffix; a copy leaves the original and ticks
// nothing new.
func TestCollectionCopyKeepsTheOriginal(t *testing.T) {
	f := newColMoveFixture(t)
	f.other.AddCalendar(caldav.Calendar{Path: davtest.CalendarHome + "work/", Name: "Their work"})
	resp := f.h.do(confirmCollection(t, f, true))
	newPath := davtest.CalendarHome + "work-2/"
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != "/app/settings/connections" {
		t.Fatalf("copy: %d %q %s", resp.Code, resp.Header().Get("Location"), resp.Body.String())
	}
	if _, _, ok := f.other.Object(newPath + "standup.ics"); !ok {
		t.Error("nothing copied")
	}
	if len(f.other.Paths(davtest.CalendarHome+"work/")) != 0 {
		t.Error("the taken calendar there was written into")
	}
	if _, _, ok := f.srv.Object(f.col + "standup.ics"); !ok {
		t.Error("the original lost its record")
	}
	blob := catalogOf(t, f.davFixture)
	if _, ok := blob.Collection(f.id, f.col); !ok {
		t.Error("the original left the catalog")
	}
	if len(blob.Selected) != 0 {
		t.Errorf("a copy ticked %+v", blob.Selected)
	}
}

// A refused record takes the new collection back: nothing is left there and
// the original is as it was.
func TestCollectionMoveRefusedTakesBackTheNew(t *testing.T) {
	f := newColMoveFixture(t)
	f.other.Refuse(http.MethodPut, http.StatusInsufficientStorage, "full")
	before := len(catalogOf(t, f.davFixture).Collections)
	page := decode[view.ConnectionsPage](t, f.h.do(confirmCollection(t, f, false)))
	n := noticeTitled(page.Notices, "Nothing was moved")
	if n == nil || n.Details == nil || len(n.Details.Lines) != 2 {
		t.Fatalf("notices = %+v", page.Notices)
	}
	if _, ok := f.other.Calendar(davtest.CalendarHome + "work/"); ok {
		t.Error("the half-made calendar was left there")
	}
	if _, _, ok := f.srv.Object(f.col + "standup.ics"); !ok {
		t.Error("the original lost a record")
	}
	if got := len(catalogOf(t, f.davFixture).Collections); got != before {
		t.Errorf("catalog %d → %d", before, got)
	}
}

// What the server there does not keep stops the delete: both stay, and the
// notice names the record and the property.
func TestCollectionMoveLossKeepsBoth(t *testing.T) {
	f := newColMoveFixture(t)
	f.other.Drop = []string{"X-CUSTOM"}
	page := decode[view.ConnectionsPage](t, f.h.do(confirmCollection(t, f, false)))
	n := noticeTitled(page.Notices, "Copied, and the original kept")
	if n == nil || n.Details == nil || !strings.Contains(string(n.Details.Lines[0]), "X-CUSTOM") ||
		!strings.HasSuffix(string(n.Body), "was not deleted.") {
		t.Fatalf("notices = %+v", page.Notices)
	}
	if _, ok := f.srv.Calendar(f.col); !ok {
		t.Error("the original was deleted")
	}
	blob := catalogOf(t, f.davFixture)
	if _, ok := blob.Collection(f.id, f.col); !ok {
		t.Error("the original left the catalog")
	}
	if _, ok := blob.Collection(f.otherID, davtest.CalendarHome+"work/"); !ok {
		t.Error("the copy is not in the catalog")
	}
}

// A read-only address book can be copied, not moved.
func TestReadOnlyCollectionCopiesNotMoves(t *testing.T) {
	f := contactsFixture(t, true, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada\nEND:VCARD\n")
	other := davtest.New(t)
	other.Login, other.Password = "bob", "other"
	other.AddAddressBook(carddav.AddressBook{Path: davtest.AddressBookHome + "work/", Name: "Work"})
	otherID := addAccount(t, f, other)

	sheet := decode[view.CollectionMoveSheet](t, f.h.do(get(colSheetHref(f, otherID, false), f.sess)))
	if sheet.Actions[0].Enabled {
		t.Errorf("move of a read-only book = %+v", sheet.Actions[0])
	}
	resp := f.h.do(postForm(colSheetHref(f, otherID, false), f.sess, url.Values{}))
	if resp.Code != http.StatusOK || len(other.Paths(davtest.AddressBookHome+"people/")) != 0 {
		t.Errorf("POST move of a read-only book: %d, paths there %v", resp.Code, other.Paths(davtest.AddressBookHome+"people/"))
	}
	resp = f.h.do(postForm(colSheetHref(f, otherID, true), f.sess, url.Values{}))
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("copy: %d %s", resp.Code, resp.Body.String())
	}
	if card, _, ok := other.Object(davtest.AddressBookHome + "people/ada.vcf"); !ok || !strings.Contains(card, "UID:ada") {
		t.Errorf("copied card = %q", card)
	}
	if _, _, ok := f.srv.Object(f.col + "ada.vcf"); !ok {
		t.Error("the read-only original lost its card")
	}
}
