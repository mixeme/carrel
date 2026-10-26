package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-webdav/caldav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

func colDeleteHref(f davFixture) string {
	return "/app/collections/delete?" + url.Values{"a": {f.id}, "c": {f.col}}.Encode()
}

func confirmDelete(f davFixture, name string) *http.Request {
	return postForm(colDeleteHref(f), f.sess, url.Values{"name": {name}})
}

// S-12.4: the connections page offers Delete… (Р-75: the rail does not); the sheet says
// what goes, that Carrel keeps no copy and what a restore would not do, and
// offers a file of everything in it first — a task in a calendar of events too.
func TestCollectionDeleteSheetSaysWhatGoes(t *testing.T) {
	f := calendarFixture(t, nil, "standup.ics", workSeries, "report.ics", workTask)

	page := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
	row := page.Accounts[0].Collections[0]
	item := row.Menu[len(row.Menu)-1]
	fromSettings := "/app/collections/delete?" + url.Values{"a": {f.id}, "c": {f.col}, "back": {"/app/settings/connections"}}.Encode()
	if item.ID != "collection.delete" || !item.Danger || !item.Separator || item.Href != fromSettings {
		t.Fatalf("connections menu = %+v", row.Menu)
	}

	sheet := decode[view.CollectionDeleteSheet](t, f.h.do(get(colDeleteHref(f), f.sess)))
	// The server, then the collection: «nextcloud · Дни рождения» (s-profile-9).
	if sheet.Kind != view.KindEvent || !strings.HasSuffix(string(sheet.Subject), " · Work") {
		t.Errorf("sheet = %+v", sheet)
	}
	if w := plainText(sheet.Warning); w != "Carrel keeps no copy: this cannot be undone." {
		t.Errorf("warning = %q", w)
	}
	if sheet.References == nil || len(sheet.References) != 0 {
		t.Errorf("references = %#v, want an empty list", sheet.References)
	}
	if !strings.Contains(plainText(sheet.Archive), "new calendar at a new address") {
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
	if plainText(sheet.Warning) != "Carrel keeps no copy: this cannot be undone." || !strings.HasPrefix(string(sheet.ExportNote), "1 contact,") {
		t.Errorf("warning = %q, export note %q", sheet.Warning, sheet.ExportNote)
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
	// The sheet closes and the answer stands on Connections, where the
	// collection still is (s-profile-31, note 112 p. 6).
	page := decode[view.ConnectionsPage](t, f.h.do(confirmDelete(f, "Work")))
	var n *view.Notice
	for i := range page.Notices {
		n = &page.Notices[i]
	}
	if n == nil || n.Title != "Work was not deleted." || !strings.Contains(string(n.Body), "refused: this account can read here but not write. Work is as it was.") ||
		n.Details == nil || !strings.Contains(string(n.Details.Summary), "DELETE") {
		t.Fatalf("notice = %+v", n)
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

var (
	frameArchiveMsg = regexp.MustCompile(`(?s)<div class="tc-msg tc-msg--flush">(.*?)</div>`)
	frameArchiveDay = regexp.MustCompile(`^Last archived (\d{4}-\d\d-\d\d);`)
)

// pointers are what s-profile-9 or s-profile-19 says points at the
// collection, and the archive line under them.
func pointers(t *testing.T, id string) (refs []string, archive string) {
	t.Helper()
	frame := mockupFrame(t, id)
	for _, m := range frameRefItem.FindAllStringSubmatch(frame, -1) {
		refs = append(refs, frameText(m[1]))
	}
	return refs, frameOne(t, frameArchiveMsg, frame, "archive line")[1]
}

// lastArchived writes a whole run of the job, finished on day.
func lastArchived(t *testing.T, f davFixture, job, day string) {
	t.Helper()
	at, err := time.Parse("2006-01-02 15:04", day+" 12:00")
	if err != nil {
		t.Fatal(err)
	}
	if err := f.h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		st.Backups = append(st.Backups, store.BackupState{JobID: job, UserID: f.sess.AccountID(),
			Runs: []store.BackupRun{{ID: "r-1", Started: at.Add(-time.Minute), Finished: &at, State: user.RunDone, Mode: sealed.BackupToDownload}}})
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
}

// s-profile-9 (from-typecase note 20): what points at the collection is
// counted by kind — «1 published link», «3 devices through davloom» — and a
// backup job is named; the archive line says when it was last in an archive
// and that a restore makes a new collection (note 135). The warning is the
// frame's, less the sentence of a server that schedules.
func TestCollectionDeleteSheetIsTheFrame(t *testing.T) {
	frame := mockupFrame(t, "s-profile-9")
	subject := frameOne(t, frameSheetSub, frame, "subject")[1]
	warning := frameOne(t, frameSheetMsg, frame, "message")[1]
	refs, archive := pointers(t, "s-profile-9")
	if len(refs) != 3 {
		t.Fatalf("s-profile-9 references %q: read it again", refs)
	}
	links := regexp.MustCompile(`^(\d+) published links? — `).FindStringSubmatch(refs[0])
	devices := regexp.MustCompile(`^(\d+) devices through davloom — `).FindStringSubmatch(refs[len(refs)-1])
	day := frameArchiveDay.FindStringSubmatch(archive)
	host := strings.Split(subject, " · ")[0]
	schedules := " " + host + " may send the attendees of its meetings a cancellation."
	if links == nil || devices == nil || day == nil || !strings.HasSuffix(warning, schedules) {
		t.Fatalf("s-profile-9 references %q, archive %q, head %q, warning %q: read it again", refs, archive, subject, warning)
	}
	job := regexp.MustCompile(`«([^»]+)»`).FindStringSubmatch(refs[1])
	nLinks, _ := strconv.Atoi(links[1])
	nDevices, _ := strconv.Atoi(devices[1])

	f := calendarFixture(t, nil, "standup.ics", workSeries)
	key, _ := f.sess.DataKey()
	conn, _ := catalogOf(t, f).Connection(f.id)
	for i := range nLinks {
		if _, err := f.h.users.Publish(f.sess.AccountID(), key, conn,
			user.PublishTarget{AccountID: f.id, Kind: "calendar", Col: f.col, Title: "Work " + strconv.Itoa(i)}, nil); err != nil {
			t.Fatal(err)
		}
	}
	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		b.BackupJobs = append(b.BackupJobs, sealed.BackupJob{ID: "j-1", Name: job[1],
			Sources: []sealed.Selection{{Kind: view.KindEvent, AccountID: f.id, Path: f.col}}})
	})
	lastArchived(t, f, "j-1", day[1])
	if err := f.h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(f.sess.AccountID())
		for i := range nDevices {
			u.ProxyDevices = append(u.ProxyDevices, store.ProxyDevice{ID: "d-" + strconv.Itoa(i), Name: "Phone " + strconv.Itoa(i),
				ScopeKinds: []string{string(view.KindEvent)}, ScopeCols: []store.DeviceScope{{AccountID: f.id, Path: f.col}}})
		}
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}

	sheet := decode[view.CollectionDeleteSheet](t, f.h.do(get(colDeleteHref(f), f.sess)))
	if want := strings.TrimSuffix(warning, schedules); plainText(sheet.Warning) != want {
		t.Errorf("warning = %q, the frame %q", sheet.Warning, want)
	}
	if !equalStrings(refLines(sheet.References), refs) {
		t.Errorf("references = %q,\nthe frame    %q", sheet.References, refs)
	}
	if plainText(sheet.Archive) != archive {
		t.Errorf("archive = %q,\nthe frame %q", sheet.Archive, archive)
	}
}

// s-profile-9's caption (from-typecase note 22) gives the lines of the other
// cases in the server's words: the archive line of a collection no job copies
// and of one no job has copied whole yet, one device, a job whose copies are
// written into the collection.
func TestCollectionDeleteSheetCaptionCases(t *testing.T) {
	frame := mockupFrame(t, "s-profile-9")
	_, caption, _ := strings.Cut(frame, "<figcaption>")
	_, cases, _ := strings.Cut(caption, "Строка архива — по делу")
	cases, _, _ = strings.Cut(cases, "Download .ics first")
	var said []string
	for _, m := range regexp.MustCompile(`«((?:[^«»]|«[^«»]*»)*)»`).FindAllStringSubmatch(cases, -1) {
		said = append(said, frameText(m[1]))
	}
	if len(said) != 4 || !strings.HasSuffix(said[1], "…") || !strings.Contains(said[3], "«…»") {
		t.Fatalf("s-profile-9 caption cases = %q: read it again", said)
	}
	noArchive, notWhole, oneDevice, writesHere := said[0], strings.TrimSuffix(said[1], "…"), said[2], said[3]

	f := calendarFixture(t, nil, "standup.ics", workSeries)
	sheet := func() view.CollectionDeleteSheet {
		return decode[view.CollectionDeleteSheet](t, f.h.do(get(colDeleteHref(f), f.sess)))
	}
	if got := plainText(sheet().Archive); got != noArchive {
		t.Errorf("no job copies it: archive = %q,\nthe caption %q", got, noArchive)
	}

	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		b.BackupJobs = append(b.BackupJobs,
			sealed.BackupJob{ID: "j-1", Name: "Личное", Sources: []sealed.Selection{{Kind: view.KindEvent, AccountID: f.id, Path: f.col}}},
			sealed.BackupJob{ID: "j-2", Name: "Сюда", DestAccount: f.id, DestPath: f.col})
	})
	if err := f.h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(f.sess.AccountID())
		u.ProxyDevices = append(u.ProxyDevices, store.ProxyDevice{ID: "d-1", Name: "Phone",
			ScopeKinds: []string{string(view.KindEvent)}, ScopeCols: []store.DeviceScope{{AccountID: f.id, Path: f.col}}})
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	got := sheet()
	if !strings.HasPrefix(plainText(got.Archive), notWhole) {
		t.Errorf("a job copies it, none whole yet: archive = %q,\nthe caption %q…", got.Archive, notWhole)
	}
	refs := refLines(got.References)
	if want := strings.Replace(writesHere, "«…»", "«Сюда»", 1); !slices.Contains(refs, want) {
		t.Errorf("references = %q, want %q", refs, want)
	}
	if !slices.Contains(refs, oneDevice) {
		t.Errorf("references = %q, want %q", refs, oneDevice)
	}
}

// Кл4 of the scan of collections, note 112 p. 6: a sheet goes back where it
// was opened once its work is done, else to Connections; a deletion opened
// from the collection's own list, which goes with it, to its section.
func TestCollectionSheetsGoBackWhereTheyCameFrom(t *testing.T) {
	f := calendarFixture(t, nil, "report.ics", workTask)
	q := url.Values{"a": {f.id}, "c": {f.col}}
	from := "/app/tasks"
	q.Set("back", from)
	rec := f.h.do(postForm("/app/collections/rename?"+q.Encode(), f.sess, url.Values{"name": {"Работа"}}))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != from {
		t.Errorf("rename: %d → %q, want %q", rec.Code, rec.Header().Get("Location"), from)
	}
	q.Del("back")
	rec = f.h.do(postForm("/app/collections/rename?"+q.Encode(), f.sess, url.Values{"name": {"Работа"}}))
	if loc := rec.Header().Get("Location"); rec.Code != http.StatusSeeOther || loc != "/app/settings/connections" {
		t.Errorf("rename without back: %d → %q, want Connections", rec.Code, loc)
	}
	// Opened from its own list, which goes with it.
	q.Set("back", f.path("calendar"))
	rec = f.h.do(postForm("/app/collections/delete?"+q.Encode(), f.sess, url.Values{"name": {"Работа"}}))
	if l := rec.Header().Get("Location"); rec.Code != http.StatusSeeOther || l == q.Get("back") || strings.Contains(l, "/settings/") {
		t.Errorf("delete from its own list: %d → %q", rec.Code, l)
	}
}
