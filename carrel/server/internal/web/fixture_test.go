package web_test

import (
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"
	"github.com/google/go-cmp/cmp"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// C-14.2: every handler is checked against its fixture, in every state the
// screen can show. The fixture is the frame written down (C-3.2); the
// records behind it are the mockup set of docs/visual/fixtures, so the two
// sides compare the same data and a difference means the screen, not the
// data.
//
// What never matches and is normalised away: the identifiers Carrel issues
// (a-…, t-…), the CSRF of the session, and the version the binary was built
// with. Nothing else: a fixture that needs another exception is a finding.

// The frames stand on one day, in one zone, of one build.
const (
	mockupZone     = "Europe/Moscow"
	mockupDay      = "2026-08-10 17:04"
	mockupVersion  = "0.0.1"
	mockupCommit   = "786949a"
	mockupInstance = "carrel.example"
	// The frames were drawn two minutes after the poll finished: their
	// refresh button says «2 min ago». A screen is taken at mockupSeen,
	// the poll behind it ran at mockupDay.
	mockupSince = 2 * time.Minute
)

func mockupNow(t *testing.T) time.Time {
	t.Helper()
	tz, err := time.LoadLocation(mockupZone)
	if err != nil {
		t.Fatal(err)
	}
	at, err := time.ParseInLocation("2006-01-02 15:04", mockupDay, tz)
	if err != nil {
		t.Fatal(err)
	}
	return at
}

// mockupCollection is one collection of the mockup set: where it lives, what
// it is called, its colour, the kinds it holds and the file of its records.
type mockupCollection struct {
	account string // "nextcloud · home" or "radicale · vps"
	slug    string // how the fixtures name it: kontakty, lichnyj, …
	name    string
	colour  string
	kinds   []view.Kind
	comps   []string
	file    string // under docs/visual/fixtures; empty for an empty collection
	// readOnly and unticked are what the frames show of «Команда» and
	// «Рассылки»: a book this account may only read, and a collection that
	// stands in the catalog out of the combined views.
	readOnly bool
	unticked bool
}

// mockupSet is the table of docs/visual/fixtures/README.md.
var mockupSet = []mockupCollection{
	{account: "nextcloud · home", slug: "kontakty", name: "Контакты", colour: "#4A6B52",
		kinds: []view.Kind{view.KindContact}, file: "nextcloud-home/kontakty.vcf"},
	{account: "nextcloud · home", slug: "arhiv", name: "Архив", colour: "#A05A2C",
		kinds: []view.Kind{view.KindContact}, file: "nextcloud-home/arhiv.vcf"},
	{account: "nextcloud · home", slug: "lichnyj", name: "Личный", colour: "#4A6B52",
		kinds: []view.Kind{view.KindEvent}, comps: []string{"VEVENT"}, file: "nextcloud-home/lichnyj.ics"},
	{account: "nextcloud · home", slug: "rabota", name: "Работа", colour: "#3F6E8C",
		kinds: []view.Kind{view.KindEvent, view.KindTask}, comps: []string{"VEVENT", "VTODO"}, file: "nextcloud-home/rabota.ics"},
	{account: "nextcloud · home", slug: "dni-rozhdeniya", name: "Дни рождения", colour: "#A05A2C",
		kinds: []view.Kind{view.KindEvent}, comps: []string{"VEVENT"}, file: "nextcloud-home/dni-rozhdeniya.ics"},
	{account: "nextcloud · home", slug: "dela", name: "Дела", colour: "#4A6B52",
		kinds: []view.Kind{view.KindTask}, comps: []string{"VTODO"}, file: "nextcloud-home/dela.ics"},
	{account: "nextcloud · home", slug: "zapisi", name: "Записи", colour: "#4A6B52",
		kinds: []view.Kind{view.KindNote}, comps: []string{"VJOURNAL"}, file: "nextcloud-home/zapisi.ics"},
	{account: "nextcloud · home", slug: "dnevnik", name: "Дневник", colour: "#6B4E7A",
		kinds: []view.Kind{view.KindNote}, comps: []string{"VJOURNAL"}, file: "nextcloud-home/dnevnik.ics"},
	// «Команда» is a book this account may only read, and «Рассылки» stands
	// in the catalog out of the combined views: both are what the frames show.
	{account: "radicale · vps", slug: "komanda", name: "Команда", colour: "#6B4E7A",
		kinds: []view.Kind{view.KindContact}, file: "radicale-vps/komanda.vcf", readOnly: true},
	{account: "radicale · vps", slug: "rassylki", name: "Рассылки", colour: "#6B4E7A",
		kinds: []view.Kind{view.KindContact}, unticked: true},
	{account: "radicale · vps", slug: "proekty", name: "Проекты", colour: "#6B4E7A",
		kinds: []view.Kind{view.KindEvent}, comps: []string{"VEVENT"}, file: "radicale-vps/proekty.ics"},
	{account: "radicale · vps", slug: "dezhurstva", name: "Дежурства", colour: "#8C6239",
		kinds: []view.Kind{view.KindEvent}, comps: []string{"VEVENT"}, file: "radicale-vps/dezhurstva.ics"},
}

// mockupApp is the instance of the fixtures: two connections labelled as the
// frames label them, the collections of the set, and everything ticked.
type mockupApp struct {
	h    *harness
	sess *session.Session
	// byAccount and byPath turn what Carrel issued into what the fixtures
	// name: a-nc-home, a-rad-vps, and the last segment of each collection.
	byAccount map[string]string
	byPath    map[string]string
	// servers are the two stands behind the set, by the label of their
	// connection: a state where a source is still being asked is made by
	// holding one of them (davtest.Stall).
	servers map[string]*davtest.Server
}

// listPath is the address of one collection of the set, as Carrel issued it.
func (a *mockupApp) listPath(t *testing.T, slug string) string {
	t.Helper()
	key, _ := a.sess.DataKey()
	blob, _ := a.h.users.Catalog(a.sess.AccountID(), key)
	for _, c := range blob.Collections {
		if collectionSlugOf(c.Path) == slug {
			return "/app/contacts/" + c.AccountID + "/" + url.PathEscape(c.Path)
		}
	}
	t.Fatalf("the set has no %q", slug)
	return ""
}

// compare is one pair: the answer of a screen against its fixture. With
// CARREL_FIXTURE_WRITE it also writes the answer out, for reading beside
// the frame while a fixture is written — never as the fixture itself.
func (a *mockupApp) compare(t *testing.T, name string, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != 200 {
		t.Fatalf("%s: %d %s", name, rec.Code, rec.Body.String())
	}
	got := a.normalise(t, rec.Body.Bytes())
	if out := os.Getenv("CARREL_FIXTURE_WRITE"); out != "" {
		body, err := json.MarshalIndent(got, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(out, name)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, name), append(body, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if diff := cmp.Diff(loadFixture(t, name), got); diff != "" {
		t.Errorf("%s differs from its fixture (-fixture +screen):\n%s", name, diff)
	}
}

func mockupRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "docs", "visual", "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("the mockup set is not where it was: %v", err)
	}
	return root
}

// splitRecords cuts a file of the set into one body per record: the DAV
// servers of the world hold one object per address, and so does davtest.
func splitRecords(body string) []string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	if strings.Contains(body, "BEGIN:VCARD") {
		var out []string
		for _, part := range strings.SplitAfter(body, "END:VCARD\n") {
			if strings.Contains(part, "BEGIN:VCARD") {
				out = append(out, strings.TrimLeft(part, "\n"))
			}
		}
		return out
	}
	head := body[:strings.Index(body, "BEGIN:V")]
	var out []string
	for _, comp := range []string{"VEVENT", "VTODO", "VJOURNAL"} {
		for _, part := range strings.SplitAfter(body, "END:"+comp+"\n") {
			if !strings.Contains(part, "BEGIN:"+comp) {
				continue
			}
			at := strings.Index(part, "BEGIN:"+comp)
			out = append(out, head+part[at:]+"END:VCALENDAR\n")
		}
	}
	return out
}

var uidLineRe = regexp.MustCompile(`(?m)^UID:(.+)$`)

func newMockupApp(t *testing.T) *mockupApp {
	t.Helper()
	root := mockupRoot(t)
	servers := map[string]*davtest.Server{}
	at := mockupNow(t)
	for _, label := range []string{"nextcloud · home", "radicale · vps"} {
		srv := davtest.New(t)
		srv.Login, srv.Password = "alice", "secret"
		// The stands keep the day of the frames: «changed on the server» is
		// theirs to say, and a wall clock there moves what a fixture holds.
		srv.Clock = func() time.Time { return at }
		servers[label] = srv
	}
	paths := map[string]string{}
	for _, col := range mockupSet {
		srv := servers[col.account]
		path := davtest.CalendarHome + col.slug + "/"
		if len(col.kinds) == 1 && col.kinds[0] == view.KindContact {
			path = davtest.AddressBookHome + col.slug + "/"
			srv.AddAddressBook(carddav.AddressBook{Path: path, Name: col.name, ReadOnly: col.readOnly})
		} else {
			srv.AddCalendar(caldav.Calendar{Path: path, Name: col.name, SupportedComponentSet: col.comps})
		}
		srv.SetColour(path, col.colour)
		paths[col.slug] = path
		if col.file == "" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(col.file)))
		if err != nil {
			t.Fatal(err)
		}
		for i, rec := range splitRecords(string(body)) {
			name := "rec" + string(rune('a'+i))
			if m := uidLineRe.FindStringSubmatch(rec); m != nil {
				name = strings.TrimSpace(m[1])
			}
			ext := ".ics"
			if strings.Contains(rec, "BEGIN:VCARD") {
				ext = ".vcf"
			}
			srv.Seed(path+url.PathEscape(name)+ext, rec)
		}
	}

	h := newHarness(t)
	h.cfg.DAVSSRFAllowlist = "127.0.0.1"
	h.cfg.Timezone = mockupZone
	h.rebuild()
	// The day of the frames, in their zone, and the build they were drawn
	// against: a fixture is the same tomorrow (fixtures-tail.md).
	h.srv.SetClock(mockupNow(t))
	h.srv.SetBuild(mockupVersion, mockupCommit)
	h.admin("mix", "password123")
	sess := h.login("mix", "password123")
	// The instance the frames sign into: the account menu names it, and
	// «Carrel» — the name of an instance nobody named — would say nothing.
	if rec := h.do(postForm("/admin/settings", sess, url.Values{
		"op": {"instance"}, "name": {mockupInstance},
		"inviteTTL": {"24"}, "idle": {"2"}, "session": {"7"},
	})); rec.Code != 303 {
		t.Fatalf("name the instance: %d %s", rec.Code, rec.Body.String())
	}
	for _, label := range []string{"nextcloud · home", "radicale · vps"} {
		rec := h.do(postForm("/app/settings/connections", sess, url.Values{
			"label": {label}, "server": {servers[label].URL}, "login": {"alice"}, "password": {"secret"},
		}))
		if rec.Code != 303 {
			t.Fatalf("connect %s: %d %s", label, rec.Code, rec.Body.String())
		}
	}
	tickMockup(t, h, sess)
	linkMockupDuplicates(t, h, sess)

	app := &mockupApp{h: h, sess: sess, servers: servers, byAccount: map[string]string{}, byPath: map[string]string{}}
	key, _ := sess.DataKey()
	blob, _ := h.users.Catalog(sess.AccountID(), key)
	names := map[string]string{"nextcloud · home": "a-nc-home", "radicale · vps": "a-rad-vps"}
	seen := 0
	for _, col := range mockupSet {
		for _, c := range blob.Collections {
			conn, _ := blob.Connection(c.AccountID)
			if c.Path != paths[col.slug] || connLabelOf(conn) != col.account {
				continue
			}
			app.byAccount[c.AccountID] = names[col.account]
			app.byPath[url.PathEscape(c.Path)] = col.slug
			seen++
		}
	}
	if seen != len(mockupSet) {
		t.Fatalf("the set did not reach the catalog: %d of %d", seen, len(mockupSet))
	}
	return app
}

// tickMockup ticks what the frames show ticked: everything but «Рассылки»,
// which the rail shows in the catalog and out of the combined views.
func tickMockup(t *testing.T, h *harness, sess *session.Session) {
	t.Helper()
	skip := map[string]bool{}
	for _, col := range mockupSet {
		if col.unticked {
			skip[col.slug] = true
		}
	}
	key, _ := sess.DataKey()
	blob, ok := h.users.Catalog(sess.AccountID(), key)
	if !ok {
		t.Fatal("no catalog")
	}
	var picks []sealed.Selection
	for _, c := range blob.Collections {
		if skip[collectionSlugOf(c.Path)] {
			continue
		}
		for _, k := range c.Kinds {
			picks = append(picks, sealed.Selection{Kind: k, AccountID: c.AccountID, Path: c.Path})
		}
	}
	for _, k := range []view.Kind{view.KindContact, view.KindEvent, view.KindTask, view.KindNote} {
		if err := h.users.SetSelection(sess.AccountID(), key, k, picks); err != nil {
			t.Fatal(err)
		}
	}
}

// collectionSlugOf is the last segment of a collection's address — how the
// set and the fixtures name it.
func collectionSlugOf(path string) string {
	return strings.Trim(path[strings.LastIndex(strings.TrimSuffix(path, "/"), "/")+1:], "/")
}

// linkMockupDuplicates links the two Артём Белов the set holds in two books
// (docs/visual/fixtures/README.md): the frames show one row with «linked ×2»
// and both sources, and that is a decision of S-16, not a property of a file.
func linkMockupDuplicates(t *testing.T, h *harness, sess *session.Session) {
	t.Helper()
	key, _ := sess.DataKey()
	blob, _ := h.users.Catalog(sess.AccountID(), key)
	var members []sealed.RecordRef
	for _, c := range blob.Collections {
		slug := collectionSlugOf(c.Path)
		if slug != "kontakty" && slug != "komanda" {
			continue
		}
		for _, uid := range mockupUIDs(t, slug) {
			if strings.HasPrefix(uid, "c3f7") || strings.HasPrefix(uid, "d41a") {
				members = append(members, sealed.RecordRef{AccountID: c.AccountID, Col: c.Path, UID: uid})
			}
		}
	}
	if len(members) != 2 {
		t.Fatalf("the two Артём Белов are not where the set puts them: %+v", members)
	}
	// The frames show the «Команда» copy — its photo, its stamp — and the
	// folded row is the group's first member (S-16), so that copy stands
	// first here. The order of the catalog is not a decision.
	sort.SliceStable(members, func(i, j int) bool { return strings.HasPrefix(members[i].UID, "d41a") })
	err := h.users.EditDuplicates(sess.AccountID(), key, "duplicates.link", "", func(d *sealed.Duplicates) error {
		d.Linked = append(d.Linked, sealed.LinkedGroup{ID: "g-artem", Kind: view.KindContact, Members: members})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// mockupUIDs are the UIDs of one file of the set, in the order it holds them.
func mockupUIDs(t *testing.T, slug string) []string {
	t.Helper()
	for _, col := range mockupSet {
		if col.slug != slug || col.file == "" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(mockupRoot(t), filepath.FromSlash(col.file)))
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, m := range uidLineRe.FindAllStringSubmatch(string(body), -1) {
			out = append(out, strings.TrimSpace(m[1]))
		}
		return out
	}
	return nil
}

// connLabelOf is the label of a connection as the screens show it.
func connLabelOf(c sealed.Connection) string {
	if c.Label != "" {
		return c.Label
	}
	return c.Server
}

// normalise takes out of an answer what cannot be the same twice: the
// identifiers Carrel issues, the session's CSRF, and the build's version.
func (a *mockupApp) normalise(t *testing.T, body []byte) map[string]any {
	t.Helper()
	s := string(body)
	for id, name := range a.byAccount {
		s = strings.ReplaceAll(s, id, name)
	}
	for path, slug := range a.byPath {
		s = strings.ReplaceAll(s, path, slug)
	}
	s = regexp.MustCompile(`t-[0-9A-Z]{10}`).ReplaceAllString(s, "t-task")
	s = regexp.MustCompile(`"[A-Za-z0-9_-]{43}"`).ReplaceAllString(s, `"csrf"`)
	var out map[string]any
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		t.Fatalf("decode the answer: %v", err)
	}
	return out
}

func loadFixture(t *testing.T, name string) map[string]any {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "docs", "spec", "fixtures", name))
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(root)
	if err != nil {
		t.Fatalf("read the fixture: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode the fixture: %v", err)
	}
	return out
}

// keys is every path of a JSON tree, for a first look at how far apart the
// two sides are before the whole diff is read.
func keys(prefix string, v any, out *[]string) {
	switch t := v.(type) {
	case map[string]any:
		names := make([]string, 0, len(t))
		for k := range t {
			names = append(names, k)
		}
		sort.Strings(names)
		for _, k := range names {
			keys(prefix+"."+k, t[k], out)
		}
	case []any:
		for i, e := range t {
			if i > 2 {
				break
			}
			keys(prefix+"[]", e, out)
		}
	default:
		*out = append(*out, prefix)
	}
}

// C-14.2, the first pair: the combined address book against
// app-contacts/normal.json. It was shown red first, with the fixture written
// from the frame and nothing yet made to answer it; it is a gate now, and a
// screen that drifts from its fixture fails the build.
func TestFixtureCombinedContacts(t *testing.T) {
	app := newMockupApp(t)
	page := combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	if page.Poll == nil {
		t.Fatal("the combined book has no poll")
	}
	// The frame is the screen two minutes after the poll: its refresh
	// button says «2 min ago». The poll itself keeps the day of the set.
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	rec := app.h.do(get("/app/contacts?task="+page.Poll.TaskID, app.sess))
	got := app.normalise(t, rec.Body.Bytes())
	want := loadFixture(t, "app-contacts/normal.json")
	if out := os.Getenv("CARREL_FIXTURE_WRITE"); out != "" {
		// The answer of the screen, normalised, for reading beside the frame
		// while a fixture is written. It is not the fixture: one written
		// from the answer would only agree with itself (fixtures-tail.md).
		body, err := json.MarshalIndent(got, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(out, append(body, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Logf("the screen is written to %s", out)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("the screen differs from its fixture (-fixture +screen):\n%s", diff)
	}
}

// C-14.2, the partial state of the same screen: the poll has not finished.
// One book answered from what the session already held, the other two are
// still being asked — the two stands are held open while the screen is
// taken (davtest.Stall).
func TestFixtureCombinedContactsPartial(t *testing.T) {
	app := newMockupApp(t)
	// «Контакты» is read alone first, so the combined poll finds it in the
	// session and answers from there: that is what «from cache» means.
	if rec := app.h.do(get(app.listPath(t, "kontakty"), app.sess)); rec.Code != 200 {
		t.Fatalf("read the book alone: %d", rec.Code)
	}
	// «Команда» lives on the other stand, and that stand is held: its book
	// is still being asked while the two of nextcloud have answered.
	release := app.servers["radicale · vps"].Stall("REPORT")
	defer release()

	app.h.srv.SetClock(mockupNow(t))
	first := decode[view.ContactsListPage](t, app.h.do(get("/app/contacts", app.sess)))
	if first.Poll == nil {
		t.Fatal("the combined book has no poll")
	}
	waitPoll(t, app.h, app.sess, first.Poll.TaskID, func(b view.PollBar) bool {
		done := 0
		for _, sp := range b.Sources {
			if sp.State == view.SourceDone || sp.State == view.SourceEmpty {
				done++
			}
		}
		return done == 2
	})
	rec := app.h.do(get("/app/contacts?task="+first.Poll.TaskID, app.sess))
	got := app.normalise(t, rec.Body.Bytes())
	if out := os.Getenv("CARREL_FIXTURE_WRITE"); out != "" {
		body, err := json.MarshalIndent(got, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(out, append(body, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Logf("the screen is written to %s", out)
	}
	if diff := cmp.Diff(loadFixture(t, "app-contacts/partial.json"), got); diff != "" {
		t.Errorf("the screen differs from its fixture (-fixture +screen):\n%s", diff)
	}
}

// C-14.2: the conflict screen (S-8). The person edits Анна Ковалёва, the
// server moves on under them, and the write is not sent over it.
func TestFixtureContactConflict(t *testing.T) {
	app := newMockupApp(t)
	const uid = "4f2b-9c31-a0e7"
	rec := app.listPath(t, "kontakty") + "/" + uid
	// Nobody arrives at a conflict from nowhere: the list was opened, the
	// card after it. That is also where the counts in the rail come from.
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)

	form := formFields(decode[view.ContactEditPage](t, app.h.do(get(rec+"/edit", app.sess))).Form)
	// The other client saved a different address and a file-as line Carrel
	// does not model; it keeps the rest of the card as the set holds it.
	app.servers["nextcloud · home"].Seed(davtest.AddressBookHome+"kontakty/"+uid+".vcf",
		"BEGIN:VCARD\nVERSION:3.0\nUID:"+uid+"\nN:Ковалёва;Анна;;;\nFN:Анна Ковалёва\n"+
			"ORG:Издательство «Слово»\nTITLE:Выпускающий редактор\n"+
			"TEL;TYPE=CELL:+7 912 445-10-88\nEMAIL;TYPE=WORK:anna.kovaleva@slovo.example\n"+
			"BDAY:1987-03-14\nCATEGORIES:работа,издательство\n"+
			"X-EVOLUTION-FILE-AS:Ковалёва\\, Анна\nREV:20260810T140200Z\nEND:VCARD\n")
	form.Set("email", "anna@slovo.example")
	form.Set("title", "Главный редактор")
	form.Set("note", "Пишет по вторникам, отвечает в тот же день. Просила писать на рабочий адрес.")
	if resp := app.h.do(postForm(rec+"/edit", app.sess, form)); resp.Code != 303 {
		t.Fatalf("POST over a moved version: %d %s", resp.Code, resp.Body.String())
	}

	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	got := app.normalise(t, app.h.do(get(rec+"/conflict", app.sess)).Body.Bytes())
	if out := os.Getenv("CARREL_FIXTURE_WRITE"); out != "" {
		body, err := json.MarshalIndent(got, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(out, append(body, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Logf("the screen is written to %s", out)
	}
	if diff := cmp.Diff(loadFixture(t, "app-conflict/normal.json"), got); diff != "" {
		t.Errorf("the screen differs from its fixture (-fixture +screen):\n%s", diff)
	}
}
