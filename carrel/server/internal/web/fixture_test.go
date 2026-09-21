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
	mockupZone = "Europe/Moscow"
	mockupDay  = "2026-08-10 17:04"
	// The day the frames say the account was made: «mix · created 11
	// February 2026». It is deliberately not the day of the frames —
	// an account a person has had for a while looks like one.
	mockupCreatedDay = "2026-02-11 09:00"
	mockupVersion    = "0.0.1"
	mockupCommit     = "786949a"
	mockupInstance   = "carrel.example"
	// The frames were drawn two minutes after the poll finished: their
	// refresh button says «2 min ago». A screen is taken at mockupSeen,
	// the poll behind it ran at mockupDay.
	mockupSince = 2 * time.Minute
)

func mockupNow(t *testing.T) time.Time { return mockupDayAt(t, mockupDay) }

func mockupDayAt(t *testing.T, day string) time.Time {
	t.Helper()
	tz, err := time.LoadLocation(mockupZone)
	if err != nil {
		t.Fatal(err)
	}
	at, err := time.ParseInLocation("2006-01-02 15:04", day, tz)
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
	// «Дневник» stands in the catalog with its tick off, as the
	// frames of the notes draw it (s-notes-1, s-notes-2).
	{account: "nextcloud · home", slug: "dnevnik", name: "Дневник", colour: "#6B4E7A",
		kinds: []view.Kind{view.KindNote}, comps: []string{"VJOURNAL"}, file: "nextcloud-home/dnevnik.ics", unticked: true},
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
func (a *mockupApp) listPath(t *testing.T, segment, slug string) string {
	t.Helper()
	key, _ := a.sess.DataKey()
	blob, _ := a.h.users.Catalog(a.sess.AccountID(), key)
	for _, c := range blob.Collections {
		if collectionSlugOf(c.Path) == slug {
			return "/app/" + segment + "/" + c.AccountID + "/" + url.PathEscape(c.Path)
		}
	}
	t.Fatalf("the set has no %q", slug)
	return ""
}

// realAccount and realPath are what Carrel issued for a collection of the
// set: a sheet is addressed by query, not by path, and the normaliser turns
// the answer back into the names the fixtures use.
func (a *mockupApp) realAccount(t *testing.T, slug string) string {
	t.Helper()
	key, _ := a.sess.DataKey()
	blob, _ := a.h.users.Catalog(a.sess.AccountID(), key)
	for _, c := range blob.Collections {
		if collectionSlugOf(c.Path) == slug {
			return c.AccountID
		}
	}
	t.Fatalf("the set has no %q", slug)
	return ""
}

func (a *mockupApp) realPath(t *testing.T, slug string) string {
	t.Helper()
	key, _ := a.sess.DataKey()
	blob, _ := a.h.users.Catalog(a.sess.AccountID(), key)
	for _, c := range blob.Collections {
		if collectionSlugOf(c.Path) == slug {
			return c.Path
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
	// The head is everything before the first component of the set: the
	// calendar's own lines and its VTIMEZONEs. Cutting at the first
	// «BEGIN:V» left the zones behind, and an event whose TZID nothing
	// defines is an event nobody can place.
	cut := len(body)
	for _, comp := range []string{"BEGIN:VEVENT", "BEGIN:VTODO", "BEGIN:VJOURNAL"} {
		if at := strings.Index(body, comp); at >= 0 && at < cut {
			cut = at
		}
	}
	head := body[:cut]
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
	// The account is made below, and its own date shows on the account
	// screen: «Account mix · created 21 September 2026». The screens' clock
	// does not reach into the record, so the service that writes it is
	// frozen too — otherwise the fixture is right only on the day it was
	// taken, and the gates go red overnight.
	h.users.SetClock(mockupDayAt(t, mockupCreatedDay))
	h.srv.SetBuild(mockupVersion, mockupCommit)
	h.srv.SetPatience()
	h.admin("mix", "password123")
	// The account is behind us; everything the run does after this happens
	// on the day of the frames.
	h.users.SetClock(mockupNow(t))
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
	// The stands listen on whatever port the machine had free; the frames
	// show an address. Same kind as an issued identifier.
	for label, name := range map[string]string{
		"nextcloud · home": "https://nextcloud.example/dav.php",
		"radicale · vps":   "https://radicale.example",
	} {
		if srv := a.servers[label]; srv != nil {
			s = strings.ReplaceAll(s, srv.URL, name)
		}
	}
	s = regexp.MustCompile(`t-[0-9A-Z]{10}`).ReplaceAllString(s, "t-task")
	s = regexp.MustCompile(`u-[0-9A-Z]{10}`).ReplaceAllString(s, "u-mix")
	// The secret of an invitation is issued at the moment it is made, and
	// it stands inside an address rather than alone, so the rule for the
	// CSRF does not reach it.
	s = regexp.MustCompile(`/invite/[A-Za-z0-9_-]{43}`).ReplaceAllString(s, "/invite/invite-token")
	s = regexp.MustCompile(`/forgot/[A-Za-z0-9_-]{43}`).ReplaceAllString(s, "/forgot/reset-token")
	// A group of duplicates is named by a digest of the records in it, and
	// those carry the account identifier, which is issued.
	s = regexp.MustCompile(`g=[0-9a-f]{32}`).ReplaceAllString(s, "g=group")
	s = regexp.MustCompile(`"groupID": ?"[0-9a-f]{32}"`).ReplaceAllString(s, `"groupID":"group"`)
	s = regexp.MustCompile(`"[A-Za-z0-9_-]{43}"`).ReplaceAllString(s, `"csrf"`)
	// When the run itself did something — started this session, created
	// this account, wrote this journal line — the stamp is the wall clock's
	// and cannot be frozen: a session begun on the day of the frames would
	// be an expired session today. Those stamps are of the same kind as an
	// issued identifier, and they are replaced by the day of the frames, so
	// a fixture still teaches the shape of a date without moving.

	var out map[string]any
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		t.Fatalf("decode the answer: %v", err)
	}
	steadyStamps(out)
	return out
}

// runStamps are the fields that say when the run itself did something: this
// session began, this account was made, this line went into the journal.
// Their times are the wall clock's and cannot be frozen — a session begun on
// the day of the frames would be an expired session today — so they are of
// the same kind as an issued identifier and are replaced by the day of the
// frames. The shape of a date is still taught; it just stops moving.
var runStamps = map[string]bool{
	"started": true, "lastSeen": true, "created": true, "lastLogin": true,
	"lastUsed": true, "expires": true, "issued": true, "when": true,
}

var dateLike = regexp.MustCompile(
	`^\d{1,2} (January|February|March|April|May|June|July|August|September|October|November|December)( \d{4})?, \d{2}:\d{2}$`)

const mockupStamp = "10 August 2026, 17:04"

func steadyStamps(v any) {
	switch t := v.(type) {
	case map[string]any:
		for k, x := range t {
			if s, ok := x.(string); ok && runStamps[k] && dateLike.MatchString(s) {
				t[k] = mockupStamp
				continue
			}
			steadyStamps(x)
		}
	case []any:
		for _, x := range t {
			steadyStamps(x)
		}
	}
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
	if rec := app.h.do(get(app.listPath(t, "contacts", "kontakty"), app.sess)); rec.Code != 200 {
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
	rec := app.listPath(t, "contacts", "kontakty") + "/" + uid
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

// C-14.2: the card of a contact, read (s-contacts-6).
func TestFixtureContactCard(t *testing.T) {
	app := newMockupApp(t)
	// The list was opened before the card: that is where the rail's counts
	// and «read 2 min ago» come from.
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	rec := app.listPath(t, "contacts", "kontakty") + "/4f2b-9c31-a0e7"
	app.compare(t, "app-contact/normal.json", app.h.do(get(rec, app.sess)))
}

// C-14.2: one calendar as an agenda (s-agenda-1).
func TestFixtureCalendarAgenda(t *testing.T) {
	app := newMockupApp(t)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	here := app.listPath(t, "calendar", "lichnyj") + "?from=2026-08-10"
	app.compare(t, "app-agenda/normal.json", app.h.do(get(here, app.sess)))
}

// C-14.2: the card of an event (s-agenda-2).
func TestFixtureEventCard(t *testing.T) {
	app := newMockupApp(t)
	app.h.do(get(app.listPath(t, "calendar", "lichnyj")+"?from=2026-08-10", app.sess))
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	rec := app.listPath(t, "calendar", "lichnyj") + "/8f1c-4a02-b7d3"
	app.compare(t, "app-event/normal.json", app.h.do(get(rec, app.sess)))
}

// C-14.2: one list of tasks (s-tasks-1).
func TestFixtureTaskList(t *testing.T) {
	app := newMockupApp(t)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	app.compare(t, "app-tasks/normal.json", app.h.do(get(app.listPath(t, "tasks", "dela"), app.sess)))
}

// C-14.2: one notebook (s-notes-1).
func TestFixtureNoteList(t *testing.T) {
	app := newMockupApp(t)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	app.compare(t, "app-notes/normal.json", app.h.do(get(app.listPath(t, "notes", "zapisi"), app.sess)))
}

// C-14.2: the card of a task (s-tasks-2).
func TestFixtureTaskCard(t *testing.T) {
	app := newMockupApp(t)
	// The lists were opened before the record: that is where the counts
	// of the rail come from.
	combined(t, app.h, app.sess, "/app/tasks", func(p view.TasksPage) *view.PollBar { return p.Poll })
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	rec := app.listPath(t, "tasks", "dela") + "/aa10-77bc-0e31"
	app.compare(t, "app-task/normal.json", app.h.do(get(rec, app.sess)))
}

// C-14.2: one note, read (s-notes-2).
func TestFixtureNoteCard(t *testing.T) {
	app := newMockupApp(t)
	app.h.do(get(app.listPath(t, "notes", "zapisi"), app.sess))
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	rec := app.listPath(t, "notes", "zapisi") + "/5c81-77aa-0d64"
	app.compare(t, "app-note/normal.json", app.h.do(get(rec, app.sess)))
}

// C-14.2: the full form of a contact (s-contacts-3).
func TestFixtureContactEdit(t *testing.T) {
	app := newMockupApp(t)
	app.h.do(get(app.listPath(t, "contacts", "kontakty"), app.sess))
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	rec := app.listPath(t, "contacts", "kontakty") + "/4f2b-9c31-a0e7/edit"
	app.compare(t, "app-contact-edit/normal.json", app.h.do(get(rec, app.sess)))
}

// C-14.2: the form of an event (s-agenda-3).
func TestFixtureEventEdit(t *testing.T) {
	app := newMockupApp(t)
	app.h.do(get(app.listPath(t, "calendar", "lichnyj")+"?from=2026-08-10", app.sess))
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	rec := app.listPath(t, "calendar", "lichnyj") + "/8f1c-4a02-b7d3/edit"
	app.compare(t, "app-event-edit/normal.json", app.h.do(get(rec, app.sess)))
}

// C-14.2: the form of a task (s-tasks-3).
func TestFixtureTaskEdit(t *testing.T) {
	app := newMockupApp(t)
	// The lists were opened before the record: that is where the counts
	// of the rail come from.
	combined(t, app.h, app.sess, "/app/tasks", func(p view.TasksPage) *view.PollBar { return p.Poll })
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	rec := app.listPath(t, "tasks", "dela") + "/aa10-77bc-0e31/edit"
	app.compare(t, "app-task-edit/normal.json", app.h.do(get(rec, app.sess)))
}

// C-14.2: a note in edit (s-notes-3).
func TestFixtureNoteEdit(t *testing.T) {
	app := newMockupApp(t)
	app.h.do(get(app.listPath(t, "notes", "zapisi"), app.sess))
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	rec := app.listPath(t, "notes", "zapisi") + "/5c81-77aa-0d64/edit"
	app.compare(t, "app-note-edit/normal.json", app.h.do(get(rec, app.sess)))
}

// C-14.2: the screens of the settings and of the administration. They hold
// no records of the set, so each is one call.
func TestFixtureSettingsAndAdmin(t *testing.T) {
	app := newMockupApp(t)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	for _, one := range []struct{ dir, route string }{
		{"app-settings-account", "/app/settings/account"},
		{"app-settings-connections", "/app/settings/connections"},
		{"app-settings-appearance", "/app/settings/appearance"},
		{"app-settings-attachments", "/app/settings/attachments"},
		{"app-settings-devices", "/app/settings/devices"},
		{"app-settings-backup", "/app/settings/backup"},
		{"app-settings-davloom", "/app/settings/davloom"},
		{"app-settings-escrow", "/app/settings/escrow"},
		{"admin-users", "/admin/"},
		{"admin-invites", "/admin/invites"},
		{"admin-settings", "/admin/settings"},
		{"admin-audit", "/admin/audit"},
		{"admin-dav", "/admin/dav"},
		{"admin-escrow", "/admin/escrow"},
	} {
		t.Run(one.dir, func(t *testing.T) {
			app.compare(t, one.dir+"/normal.json", app.h.do(get(one.route, app.sess)))
		})
	}
}

// C-14.2: the screens outside the shell. A person who has not signed in
// sees them, so they are taken without a session.
func TestFixtureOutsideTheShell(t *testing.T) {
	app := newMockupApp(t)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	for _, one := range []struct{ dir, route string }{
		{"login", "/login"},
		{"forgot", "/forgot"},
		{"about", "/about"},
	} {
		t.Run(one.dir, func(t *testing.T) {
			app.compare(t, one.dir+"/normal.json", app.h.do(get(one.route, nil)))
		})
	}
	t.Run("admin-install", func(t *testing.T) {
		app.compare(t, "admin-install/normal.json", app.h.do(get("/admin/install", app.sess)))
	})
}

// C-14.2: the screens of one record that are neither its card nor its form,
// and the screens the shell opens over a list.
func TestFixtureMoreOfTheShell(t *testing.T) {
	app := newMockupApp(t)
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	book := app.listPath(t, "contacts", "kontakty")
	anna := book + "/4f2b-9c31-a0e7"
	// The timeline is a search over one person: it is gathered when the
	// screen is opened, and the screen is taken once it has answered.
	t.Run("app-contact-timeline", func(t *testing.T) {
		first := decode[view.ContactTimelinePage](t, app.h.do(get(anna+"/timeline", app.sess)))
		if first.Poll.TaskID == "" {
			t.Fatal("the timeline gathered nothing")
		}
		waitPoll(t, app.h, app.sess, first.Poll.TaskID, nil)
		app.compare(t, "app-contact-timeline/normal.json", app.h.do(get(anna+"/timeline", app.sess)))
	})
	for _, one := range []struct{ dir, route string }{
		{"app-duplicates", "/app/duplicates?kind=contact"},
		{"app-contact-new", book + "/new"},
		{"app-contact-import", book + "/import"},
		{"app-contact-move", anna + "/move"},
		{"app-quick", "/app/notes/quick"},
		{"app-collections-new", "/app/collections/new?kind=contact"},
		{"app-settings-unlock", "/app/settings/unlock"},
	} {
		t.Run(one.dir, func(t *testing.T) {
			app.compare(t, one.dir+"/normal.json", app.h.do(get(one.route, app.sess)))
		})
	}
}

// C-14.2: the search, and the sheets a collection and a record open.
func TestFixtureSearchAndSheets(t *testing.T) {
	app := newMockupApp(t)
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	book := app.listPath(t, "contacts", "kontakty")
	anna := book + "/4f2b-9c31-a0e7"

	// A search asks every ticked collection and the screen is taken once
	// they have answered, like a combined list.
	t.Run("app-search", func(t *testing.T) {
		first := decode[view.SearchPage](t, app.h.do(get("/app/search?q=Ковалёва", app.sess)))
		if first.Poll.TaskID == "" {
			t.Fatal("the search asked nobody")
		}
		waitPoll(t, app.h, app.sess, first.Poll.TaskID, nil)
		app.compare(t, "app-search/normal.json", app.h.do(get("/app/search?q=Ковалёва", app.sess)))
	})
	for _, one := range []struct{ dir, route string }{
		{"app-collections-rename", "/app/collections/rename?a=a-nc-home&c=kontakty"},
		{"app-collections-move", "/app/collections/move?a=a-nc-home&c=kontakty"},
		{"app-collections-delete", "/app/collections/delete?a=a-nc-home&c=kontakty"},
		{"app-publish", "/app/publish?a=a-nc-home&c=kontakty&k=contacts&uid=4f2b-9c31-a0e7"},
		{"app-password", "/app/password"},
	} {
		t.Run(one.dir, func(t *testing.T) {
			route := strings.ReplaceAll(one.route, "a-nc-home", app.realAccount(t, "kontakty"))
			route = strings.ReplaceAll(route, "c=kontakty", "c="+app.realPath(t, "kontakty"))
			app.compare(t, one.dir+"/normal.json", app.h.do(get(route, app.sess)))
		})
	}
	_ = anna
}

// C-14.2 and C-14.3: the states of a list of contacts beside its normal
// one. Each is a state of the screen, not another screen.
func TestFixtureContactsStates(t *testing.T) {
	app := newMockupApp(t)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))

	// empty: «Рассылки» holds nothing at all.
	t.Run("empty", func(t *testing.T) {
		app.compare(t, "app-contacts-one/empty.json",
			app.h.do(get(app.listPath(t, "contacts", "rassylki"), app.sess)))
	})
	// readonly: «Команда» is a book this account may only read.
	t.Run("readonly", func(t *testing.T) {
		app.compare(t, "app-contacts-one/readonly.json",
			app.h.do(get(app.listPath(t, "contacts", "komanda"), app.sess)))
	})
	// source-error: the stand refuses, and the screen says so instead of
	// showing an empty list (C-8).
	t.Run("source-error", func(t *testing.T) {
		app.servers["nextcloud · home"].Refuse("REPORT", 503, "the server is busy")
		defer app.servers["nextcloud · home"].Accept("REPORT")
		app.compare(t, "app-contacts-one/source-error.json",
			app.h.do(get(app.listPath(t, "contacts", "kontakty"), app.sess)))
	})
}

// C-14.3: the states of the combined address book.
func TestFixtureCombinedContactsStates(t *testing.T) {
	// empty: nothing is ticked, so there is nothing to poll and the screen
	// says so rather than showing an empty list (C-8).
	t.Run("empty", func(t *testing.T) {
		app := newMockupApp(t)
		rec := app.h.do(postForm("/app/sources/contacts", app.sess, url.Values{}))
		if rec.Code != 303 {
			t.Fatalf("untick everything: %d", rec.Code)
		}
		app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
		app.compare(t, "app-contacts/empty.json", app.h.do(get("/app/contacts", app.sess)))
	})
	// unavailable: one of the two stands refuses, and the poll says which
	// book is missing from the answer instead of quietly dropping it.
	t.Run("unavailable", func(t *testing.T) {
		app := newMockupApp(t)
		app.servers["radicale · vps"].Refuse("REPORT", 503, "the server is busy")
		defer app.servers["radicale · vps"].Accept("REPORT")
		app.h.srv.SetClock(mockupNow(t))
		page := combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
		app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
		app.compare(t, "app-contacts/unavailable.json",
			app.h.do(get("/app/contacts?task="+page.Poll.TaskID, app.sess)))
	})
}

// C-14.3: a combined view of every other kind with nothing ticked, and the
// report of what a server dropped (C-9).
func TestFixtureMoreStates(t *testing.T) {
	for _, one := range []struct{ dir, segment string }{
		{"app-agenda-all", "calendar"},
		{"app-tasks-all", "tasks"},
		{"app-notes-all", "notes"},
	} {
		t.Run(one.dir, func(t *testing.T) {
			app := newMockupApp(t)
			if rec := app.h.do(postForm("/app/sources/"+one.segment, app.sess, url.Values{})); rec.Code != 303 {
				t.Fatalf("untick everything: %d", rec.Code)
			}
			app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
			app.compare(t, one.dir+"/empty.json", app.h.do(get("/app/"+one.segment, app.sess)))
		})
	}

	// loss: the stand keeps a card the way go-vcard writes it back, so the
	// properties it does not know are gone after a write, and C-9 says so.
	t.Run("loss", func(t *testing.T) {
		app := newMockupApp(t)
		rec := app.listPath(t, "contacts", "kontakty") + "/4f2b-9c31-a0e7"
		form := formFields(decode[view.ContactEditPage](t, app.h.do(get(rec+"/edit", app.sess))).Form)
		app.servers["nextcloud · home"].Drop = []string{"X-EVOLUTION-FILE-AS", "X-ABLABEL", "X-CARREL-UNKNOWN-FLAG"}
		form.Set("title", "Главный редактор")
		app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
		// The save went through and the form comes back with the report:
		// what the server did not keep is said where the change was made,
		// not on a screen the person has to go and find (C-9).
		app.compare(t, "app-contact-edit/loss.json", app.h.do(postForm(rec+"/edit", app.sess, form)))
	})
}

// C-14.2: the files, and the link an invited person opens. The mockup set
// holds no folder of files, so the three screens of the files stand in
// their empty state — which is the truth of this set, not a gap in them.
func TestFixtureFilesAndInvite(t *testing.T) {
	app := newMockupApp(t)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	for _, one := range []struct{ dir, state, route string }{
		{"app-files", "empty", "/app/files"},
		{"app-files-published", "empty", "/app/files/published"},
		{"app-files-picker", "empty", "/app/files/picker?for=upload"},
	} {
		t.Run(one.dir, func(t *testing.T) {
			app.compare(t, one.dir+"/"+one.state+".json", app.h.do(get(one.route, app.sess)))
		})
	}
	t.Run("invite", func(t *testing.T) {
		secret, err := app.h.users.CreateInvite(app.sess.AccountID(), "vera", "")
		if err != nil {
			t.Fatal(err)
		}
		app.compare(t, "invite/normal.json", app.h.do(get("/invite/"+secret, nil)))
	})
}

// C-14.2: the screens of an instance nobody has set up yet, and the ones a
// person reaches without an account. They need an empty instance, so they
// do not use the mockup set.
func TestFixtureBeforeAnyone(t *testing.T) {
	fresh := &mockupApp{h: newHarness(t), byAccount: map[string]string{}, byPath: map[string]string{}}
	fresh.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	fresh.h.srv.SetBuild(mockupVersion, mockupCommit)
	t.Run("setup", func(t *testing.T) {
		fresh.compare(t, "setup/normal.json", fresh.h.do(get("/setup", nil)))
	})

	open := &mockupApp{h: newHarness(t), byAccount: map[string]string{}, byPath: map[string]string{}}
	open.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	open.h.srv.SetBuild(mockupVersion, mockupCommit)
	admin := open.h.admin("mix", "password123")
	sess := open.h.login("mix", "password123")
	// Registration needs a relay: without one the letter that confirms an
	// address never goes, and nobody who registered could sign in (S-26).
	open.h.do(postForm("/admin/settings", sess, url.Values{
		"host": {"127.0.0.1"}, "port": {"25"}, "security": {"none"},
		"username": {"relay"}, "password": {"relay-password-xyz"}, "from": {"noreply@carrel.example"},
	}))
	if rec := open.h.do(postForm("/admin/", sess, url.Values{"action": {"selfreg"}, "selfReg": {"on"}})); rec.Code != 303 {
		t.Fatalf("open the registration: %d %s", rec.Code, rec.Body.String())
	}
	t.Run("register", func(t *testing.T) {
		open.compare(t, "register/normal.json", open.h.do(get("/register", nil)))
	})
	_ = admin
}

// C-14.2: the merge of two records, the letter, and the link a forgotten
// password leads to.
func TestFixtureMergeLetterAndReset(t *testing.T) {
	t.Run("app-duplicates-merge", func(t *testing.T) {
		app := newMockupApp(t)
		// The two Артём Белов are linked in the set, and a linked group is
		// decided: nothing to merge. Here they are undecided again, which
		// is the state the merge screen exists for.
		key, _ := app.sess.DataKey()
		err := app.h.users.EditDuplicates(app.sess.AccountID(), key, "", "", func(d *sealed.Duplicates) error {
			d.Linked = nil
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
		page := decode[view.DuplicatesPage](t, app.h.do(get("/app/duplicates?kind=contact", app.sess)))
		if len(page.Groups) != 1 {
			t.Fatalf("groups = %d, want the one pair of the set", len(page.Groups))
		}
		app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
		app.compare(t, "app-duplicates-merge/normal.json",
			app.h.do(get(actionOf(t, page.Groups[0], "duplicates.merge").Href, app.sess)))
	})
	t.Run("app-mail-compose", func(t *testing.T) {
		app := newMockupApp(t)
		// A letter needs a relay; the instance of the other fixtures has
		// none, and adding one there would put «Letter» in every menu.
		app.h.do(postForm("/admin/settings", app.sess, url.Values{
			"host": {"127.0.0.1"}, "port": {"25"}, "security": {"none"},
			"username": {"relay"}, "password": {"relay-password-xyz"}, "from": {"noreply@carrel.example"},
		}))
		// And letters from people have to be allowed: a relay alone is for
		// the instance's own letters (S-26).
		app.h.do(postForm("/admin/settings", app.sess, url.Values{
			"op": {"limits"}, "people": {"on"},
			"attach": {"10"}, "letter": {"25"}, "recipients": {"20"}, "perDay": {"100"},
		}))
		app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
		app.compare(t, "app-mail-compose/normal.json", app.h.do(get("/app/mail/compose", app.sess)))
	})
}

// C-14.2: the link a reset password leads to, and the conflict of the
// three kinds beside the contact's.
func TestFixtureResetAndConflicts(t *testing.T) {
	t.Run("forgot-token", func(t *testing.T) {
		app := &mockupApp{h: newHarness(t), byAccount: map[string]string{}, byPath: map[string]string{}}
		app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
		app.h.srv.SetBuild(mockupVersion, mockupCommit)
		adminID := app.h.admin("mix", "password123")
		app.h.invite(adminID, "vera", "verapass123")
		sess := app.h.login("mix", "password123")
		page := decode[view.AdminUsersPage](t, app.h.do(get("/admin/", sess)))
		var target string
		for _, r := range page.Rows {
			if r.Login != "vera" {
				continue
			}
			for _, it := range r.Menu {
				if it.ID == "admin.user.reset" {
					target = it.Href
				}
			}
		}
		if target == "" {
			t.Fatal("no reset for the invited person")
		}
		rec := app.h.do(postForm(target, sess, url.Values{}))
		link := resetLink(t, rec)
		app.compare(t, "forgot-token/normal.json", app.h.do(get(link, nil)))
	})
}

// resetLink digs the wipe link out of the answer that issued it.
func resetLink(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	m := regexp.MustCompile(`/forgot/[A-Za-z0-9_-]{43}`).FindString(rec.Body.String())
	if m == "" {
		t.Fatalf("no reset link in the answer: %s", rec.Body.String()[:400])
	}
	return m
}

// C-14.2: the conflict of an event and of a task. The screen is one for
// every kind (C-2.1), and each is checked on its own record.
func TestFixtureConflictsOfEveryKind(t *testing.T) {
	for _, one := range []struct{ dir, segment, slug, uid, now, body string }{
		{"app-conflict-event", "calendar", "lichnyj", "8f1c-4a02-b7d3", "Синк-ап по релизу и демо",
			"BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//set//EN\nBEGIN:VEVENT\nUID:8f1c-4a02-b7d3\nDTSTAMP:20260801T000000Z\nSUMMARY:Синк-ап перенесён\nDTSTART:20260810T090000Z\nDTEND:20260810T100000Z\nEND:VEVENT\nEND:VCALENDAR\n"},
		{"app-conflict-task", "tasks", "dela", "aa10-77bc-0e31", "Отправить документы в МФЦ до пятницы",
			"BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//set//EN\nBEGIN:VTODO\nUID:aa10-77bc-0e31\nDTSTAMP:20260801T000000Z\nSUMMARY:Документы в МФЦ\nEND:VTODO\nEND:VCALENDAR\n"},
	} {
		t.Run(one.dir, func(t *testing.T) {
			app := newMockupApp(t)
			rec := app.listPath(t, one.segment, one.slug) + "/" + one.uid
			form := formFields(decode[view.EventEditPage](t, app.h.do(get(rec+"/edit", app.sess))).Form)
			app.servers["nextcloud · home"].Seed(davtest.CalendarHome+one.slug+"/"+one.uid+".ics", one.body)
			form.Set("summary", one.now)
			if resp := app.h.do(postForm(rec+"/edit", app.sess, form)); resp.Code != 303 {
				t.Fatalf("POST over a moved version: %d", resp.Code)
			}
			app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
			app.compare(t, one.dir+"/normal.json", app.h.do(get(rec+"/conflict", app.sess)))
		})
	}
}

// C-14.2: the preview a photo is cropped in (s-contacts-5). It exists only
// while a photo is being added, so the screen is taken right after one is.
func TestFixturePhotoPreview(t *testing.T) {
	app := newMockupApp(t)
	// The lists were opened before the card, as a person opens them, and
	// that is where the counts of the rail come from.
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	rec := app.listPath(t, "contacts", "kontakty") + "/4f2b-9c31-a0e7"
	add := actionHref(t, decode[view.ContactEditPage](t, app.h.do(get(rec+"/edit", app.sess))).Photo.Actions, "photo.add")
	// The upload answers with a redirect to the preview; the screen is
	// the page it leads to.
	if resp := app.h.do(uploadRequest(add, app.sess, false, "anna.jpg", jpegOf(t, halvesImage(240, 180)))); resp.Code != 303 {
		t.Fatalf("upload: %d %s", resp.Code, resp.Body.String())
	}
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	app.compare(t, "app-photo-preview/normal.json", app.h.do(get(rec+"/photo-preview", app.sess)))
}

// C-14.3: the states left of the other kinds — a week with nothing in it,
// a stand that refuses, and the conflict of a note.
func TestFixtureStatesOfTheOtherKinds(t *testing.T) {
	t.Run("agenda-empty", func(t *testing.T) {
		app := newMockupApp(t)
		app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
		// A week of the set that holds nothing: an agenda is empty by the
		// window it shows, not by the calendar being empty.
		app.compare(t, "app-agenda/empty.json",
			app.h.do(get(app.listPath(t, "calendar", "lichnyj")+"?from=2026-11-02", app.sess)))
	})
	for _, one := range []struct{ dir, segment, slug string }{
		{"app-tasks", "tasks", "dela"},
		{"app-notes", "notes", "zapisi"},
	} {
		t.Run(one.dir+"-source-error", func(t *testing.T) {
			app := newMockupApp(t)
			app.servers["nextcloud · home"].Refuse("REPORT", 503, "the server is busy")
			defer app.servers["nextcloud · home"].Accept("REPORT")
			app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
			app.compare(t, one.dir+"/source-error.json",
				app.h.do(get(app.listPath(t, one.segment, one.slug), app.sess)))
		})
	}
	t.Run("app-conflict-note", func(t *testing.T) {
		app := newMockupApp(t)
		rec := app.listPath(t, "notes", "zapisi") + "/5c81-77aa-0d64"
		// The edit of a note answers with the document itself, not with
		// the page around it (C-7).
		doc := decode[view.NoteDoc](t, app.h.do(get(rec+"/edit", app.sess)))
		save, err := url.Parse(actionHref(t, doc.Bar, "record.save"))
		if err != nil {
			t.Fatal(err)
		}
		// Everything the form holds goes back, as a client sends it: a
		// field left out is a field cleared, and the conflict would then
		// be about a date nobody touched.
		form := url.Values{
			"summary":     {"Заметка со встречи 10.08 — дополнено"},
			"description": {string(doc.Body)},
		}
		for _, row := range doc.Fields {
			if row.Name != "" && row.Name != "summary" && row.Name != "description" {
				form.Set(row.Name, string(row.Value))
			}
		}
		for k, v := range save.Query() {
			form[k] = v
		}
		app.servers["nextcloud · home"].Seed(davtest.CalendarHome+"zapisi/5c81-77aa-0d64.ics",
			"BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//set//EN\nBEGIN:VJOURNAL\nUID:5c81-77aa-0d64\nDTSTAMP:20260801T000000Z\nSUMMARY:Встреча 10.08\nDESCRIPTION:Правка с другого клиента\nDTSTART:20260810T080500Z\nEND:VJOURNAL\nEND:VCALENDAR\n")
		if resp := app.h.do(postForm(save.EscapedPath(), app.sess, form)); resp.Code != 303 {
			t.Fatalf("POST over a moved version: %d %s", resp.Code, resp.Body.String())
		}
		app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
		app.compare(t, "app-conflict-note/normal.json", app.h.do(get(rec+"/conflict", app.sess)))
	})
}

// C-14.3: what a server drops, on the forms of the other kinds, and the
// combined views while their poll is still running.
func TestFixtureLossAndPartialOfTheOtherKinds(t *testing.T) {
	for _, one := range []struct {
		dir, segment, slug, uid string
		drop                    []string
	}{
		{"app-event-edit", "calendar", "lichnyj", "8f1c-4a02-b7d3", []string{"CATEGORIES", "ORGANIZER"}},
		{"app-task-edit", "tasks", "dela", "aa10-77bc-0e31", []string{"CATEGORIES"}},
	} {
		t.Run(one.dir+"-loss", func(t *testing.T) {
			app := newMockupApp(t)
			rec := app.listPath(t, one.segment, one.slug) + "/" + one.uid
			form := formFields(decode[view.EventEditPage](t, app.h.do(get(rec+"/edit", app.sess))).Form)
			app.servers["nextcloud · home"].Drop = one.drop
			form.Set("summary", string(form.Get("summary"))+" ·")
			app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
			app.compare(t, one.dir+"/loss.json", app.h.do(postForm(rec+"/edit", app.sess, form)))
		})
	}
	// Only the calendar: its collections stand on both stands, so one can
	// be held while the other answers. The tasks and the notes of this set
	// live on nextcloud alone — holding it holds everything, and what the
	// screen shows then is not a state but a moment. The partial state is
	// covered here and by the contacts.
	for _, one := range []struct{ dir, segment string }{
		{"app-agenda-all", "calendar"},
	} {
		t.Run(one.dir+"-partial", func(t *testing.T) {
			app := newMockupApp(t)
			release := app.servers["radicale · vps"].Stall("REPORT")
			defer release()
			app.h.srv.SetClock(mockupNow(t))
			first := app.h.do(get("/app/"+one.segment, app.sess))
			task := taskOf(t, first)
			if task == "" {
				t.Skip("this kind polls nothing on the other stand")
			}
			// The steady point: nobody is still in the queue and at least
			// one source is held. Everything that could answer has.
			waitPoll(t, app.h, app.sess, task, func(b view.PollBar) bool {
				held := false
				for _, sp := range b.Sources {
					switch sp.State {
					case view.SourceWaiting:
						return false
					case view.SourceQuerying:
						held = true
					case view.SourceDone:
						// A source that answered but has not yet given
						// its count is still arriving: its rows would
						// come and go between two runs.
						if sp.Count == nil {
							return false
						}
					}
				}
				return held
			})
			app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
			app.compare(t, one.dir+"/partial.json", app.h.do(get("/app/"+one.segment+"?task="+task, app.sess)))
		})
	}
}

// taskOf digs the poll's task out of whatever list answered.
func taskOf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Poll *struct {
			TaskID string `json:"taskID"`
		} `json:"poll"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Poll == nil {
		return ""
	}
	return body.Poll.TaskID
}
