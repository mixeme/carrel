package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

func colDeleteHref(f davFixture) string {
	return "/app/collections/delete?" + url.Values{"a": {f.id}, "c": {f.col}}.Encode()
}

func confirmDelete(f davFixture, name string) *http.Request {
	return postForm(colDeleteHref(f), f.sess, url.Values{"name": {name}})
}

// S-12.4: the rail and the connections page offer Delete…; the sheet says
// what goes, that Carrel keeps no copy and what a restore would not do, and
// offers a file of everything in it first — a task in a calendar of events too.
func TestCollectionDeleteSheetSaysWhatGoes(t *testing.T) {
	f := calendarFixture(t, nil, "standup.ics", workSeries, "report.ics", workTask)

	page := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
	row := page.Accounts[0].Collections[0]
	item := row.Menu[len(row.Menu)-1]
	if item.ID != "collection.delete" || !item.Danger || !item.Separator || item.Href != colDeleteHref(f) {
		t.Fatalf("connections menu = %+v", row.Menu)
	}
	rail := decode[view.AgendaPage](t, f.h.do(get(f.path("calendar"), f.sess)))
	found := false
	for _, sec := range rail.Rail.Sections {
		for _, r := range sec.Rows {
			found = found || hasMenu(r.Menu, "collection.delete")
		}
	}
	if !found {
		t.Error("the rail has no Delete…")
	}

	sheet := decode[view.CollectionDeleteSheet](t, f.h.do(get(colDeleteHref(f), f.sess)))
	if sheet.Kind != view.KindEvent || !strings.HasPrefix(string(sheet.Subject), "Work · ") {
		t.Errorf("sheet = %+v", sheet)
	}
	if w := string(sheet.Warning); !strings.Contains(w, "all 2 records") || !strings.Contains(w, "Carrel holds no copy") {
		t.Errorf("warning = %q", w)
	}
	if sheet.References == nil || len(sheet.References) != 0 {
		t.Errorf("references = %#v, want an empty list", sheet.References)
	}
	if !strings.Contains(string(sheet.Archive), "new calendar at a new address") {
		t.Errorf("archive = %q", sheet.Archive)
	}
	if c := sheet.Actions[0]; c.ID != "collection.delete.confirm" || c.Method != "POST" || !c.Danger {
		t.Errorf("confirm = %+v", c)
	}
	if sheet.Export == nil || string(sheet.Export.Label) != "Download .ics first" || !strings.Contains(string(sheet.ExportNote), "2 records") {
		t.Fatalf("export = %+v, note %q", sheet.Export, sheet.ExportNote)
	}
	body := download(t, f, sheet.Export.Href, "work.ics", "text/calendar")
	if strings.Count(body, "BEGIN:VCALENDAR") != 1 || !strings.Contains(body, "UID:standup") || !strings.Contains(body, "BEGIN:VTODO") ||
		!strings.Contains(body, "RECURRENCE-ID:20261013T100000Z") {
		t.Errorf("the file before deleting does not hold everything:\n%s", body)
	}
	if n := strings.Count(body, "BEGIN:VTODO"); n != 1 {
		t.Errorf("tasks = %d, a record read twice", n)
	}
}

// A book goes out as .vcf; an empty calendar has nothing to download.
func TestCollectionDeleteSheetExportByKind(t *testing.T) {
	book := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada\nEND:VCARD\n")
	sheet := decode[view.CollectionDeleteSheet](t, book.h.do(get(colDeleteHref(book), book.sess)))
	if sheet.Export == nil || string(sheet.Export.Label) != "Download .vcf first" || sheet.Export.Href != book.path("contacts")+"/export" {
		t.Errorf("book export = %+v", sheet.Export)
	}
	if !strings.Contains(string(sheet.Warning), "address book and all 1 contact in it") {
		t.Errorf("warning = %q", sheet.Warning)
	}

	empty := calendarFixture(t, nil)
	sheet = decode[view.CollectionDeleteSheet](t, empty.h.do(get(colDeleteHref(empty), empty.sess)))
	if sheet.Export != nil || sheet.ExportNote == "" {
		t.Errorf("empty calendar: export %+v, note %q", sheet.Export, sheet.ExportNote)
	}
}

// A name that is not the one shown sends nothing to the server.
func TestCollectionDeleteNeedsTheName(t *testing.T) {
	f := calendarFixture(t, nil, "report.ics", workTask)
	for _, typed := range []string{"", "work", "Work 2"} {
		sheet := decode[view.CollectionDeleteSheet](t, f.h.do(confirmDelete(f, typed)))
		row := sheet.Form.Sets[0].Rows[1]
		if row.Error == "" || string(row.Value) != typed {
			t.Errorf("typed %q: row = %+v", typed, row)
		}
	}
	if n := f.srv.CountRequests(http.MethodDelete, f.col); n != 0 {
		t.Errorf("DELETE sent %d times without the name", n)
	}
	if _, ok := f.srv.Calendar(f.col); !ok {
		t.Error("the calendar is gone")
	}
}

// The name typed deletes the collection on the server and takes it and its
// ticks out of the catalog; the journal says so.
func TestCollectionDeleteDeletesAndForgets(t *testing.T) {
	f := calendarFixture(t, nil, "report.ics", workTask)
	key, _ := f.sess.DataKey()
	if err := f.h.users.SetSelection(f.sess.AccountID(), key, view.KindEvent, []sealed.Selection{{Kind: view.KindEvent, AccountID: f.id, Path: f.col}}); err != nil {
		t.Fatal(err)
	}
	resp := f.h.do(confirmDelete(f, "  Work "))
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != "/app/settings/connections" {
		t.Fatalf("delete: %d %q %s", resp.Code, resp.Header().Get("Location"), resp.Body.String())
	}
	if _, ok := f.srv.Calendar(f.col); ok {
		t.Error("the calendar is still on the server")
	}
	blob := catalogOf(t, f)
	if _, ok := blob.Collection(f.id, f.col); ok {
		t.Error("the catalog still holds it")
	}
	if len(blob.Selected) != 0 {
		t.Errorf("ticks = %+v", blob.Selected)
	}
	if what, _ := lastAudit(f.h); what != "collection.delete" {
		t.Errorf("audit = %s", what)
	}
	if rec := f.h.do(get(colDeleteHref(f), f.sess)); rec.Code != http.StatusNotFound {
		t.Errorf("sheet after delete: %d, want 404", rec.Code)
	}
}

// The name is compared as shown, not as the keyboard composed it.
func TestCollectionDeleteNameInAnyUnicodeForm(t *testing.T) {
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	col := davtest.CalendarHome + "yolka/"
	srv.AddCalendar(caldav.Calendar{Path: col, Name: "Йолка"})
	f := connectFixture(t, srv, col, nil)
	resp := f.h.do(confirmDelete(f, "Йолка"))
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("decomposed name: %d %s", resp.Code, resp.Body.String())
	}
	if _, ok := srv.Calendar(col); ok {
		t.Error("not deleted")
	}
}

// A refusal leaves the collection where it was, in the catalog too, and says
// why with the method, address and code.
func TestCollectionDeleteRefusedKeepsIt(t *testing.T) {
	f := calendarFixture(t, nil, "report.ics", workTask)
	f.srv.Refuse(http.MethodDelete, http.StatusForbidden, "no")
	page := decode[view.ConnectionsPage](t, f.h.do(confirmDelete(f, "Work")))
	n := noticeTitled(page.Notices, "This account can read here but not delete")
	if n == nil || n.Details == nil || !strings.Contains(string(n.Details.Summary), "DELETE") {
		t.Fatalf("notices = %+v", page.Notices)
	}
	if _, ok := catalogOf(t, f).Collection(f.id, f.col); !ok {
		t.Error("the catalog lost it")
	}
}

// A server that no longer has it is taken at its word: it leaves the catalog.
func TestCollectionDeleteAlreadyGone(t *testing.T) {
	f := calendarFixture(t, nil)
	f.srv.Refuse(http.MethodDelete, http.StatusNotFound, "")
	if resp := f.h.do(confirmDelete(f, "Work")); resp.Code != http.StatusSeeOther {
		t.Fatalf("delete of a gone calendar: %d %s", resp.Code, resp.Body.String())
	}
	if _, ok := catalogOf(t, f).Collection(f.id, f.col); ok {
		t.Error("the catalog still holds it")
	}
}
