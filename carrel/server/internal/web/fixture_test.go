package web_test

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

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
}

// mockupSet is the table of docs/visual/fixtures/README.md.
var mockupSet = []mockupCollection{
	{"nextcloud · home", "kontakty", "Контакты", "#4A6B52", []view.Kind{view.KindContact}, nil, "nextcloud-home/kontakty.vcf"},
	{"nextcloud · home", "arhiv", "Архив", "#A05A2C", []view.Kind{view.KindContact}, nil, "nextcloud-home/arhiv.vcf"},
	{"nextcloud · home", "lichnyj", "Личный", "#4A6B52", []view.Kind{view.KindEvent}, []string{"VEVENT"}, "nextcloud-home/lichnyj.ics"},
	{"nextcloud · home", "rabota", "Работа", "#3F6E8C", []view.Kind{view.KindEvent, view.KindTask}, []string{"VEVENT", "VTODO"}, "nextcloud-home/rabota.ics"},
	{"nextcloud · home", "dni-rozhdeniya", "Дни рождения", "#A05A2C", []view.Kind{view.KindEvent}, []string{"VEVENT"}, "nextcloud-home/dni-rozhdeniya.ics"},
	{"nextcloud · home", "dela", "Дела", "#4A6B52", []view.Kind{view.KindTask}, []string{"VTODO"}, "nextcloud-home/dela.ics"},
	{"nextcloud · home", "zapisi", "Записи", "#4A6B52", []view.Kind{view.KindNote}, []string{"VJOURNAL"}, "nextcloud-home/zapisi.ics"},
	{"nextcloud · home", "dnevnik", "Дневник", "#6B4E7A", []view.Kind{view.KindNote}, []string{"VJOURNAL"}, "nextcloud-home/dnevnik.ics"},
	{"radicale · vps", "komanda", "Команда", "#6B4E7A", []view.Kind{view.KindContact}, nil, "radicale-vps/komanda.vcf"},
	{"radicale · vps", "rassylki", "Рассылки", "#6B4E7A", []view.Kind{view.KindContact}, nil, ""},
	{"radicale · vps", "proekty", "Проекты", "#6B4E7A", []view.Kind{view.KindEvent}, []string{"VEVENT"}, "radicale-vps/proekty.ics"},
	{"radicale · vps", "dezhurstva", "Дежурства", "#8C6239", []view.Kind{view.KindEvent}, []string{"VEVENT"}, "radicale-vps/dezhurstva.ics"},
}

// mockupApp is the instance of the fixtures: two connections labelled as the
// frames label them, the collections of the set, and everything ticked.
type mockupApp struct {
	h     *harness
	sess  *session.Session
	byCol map[string]string // slug → {account}/{col} as an address holds them
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
	for _, label := range []string{"nextcloud · home", "radicale · vps"} {
		srv := davtest.New(t)
		srv.Login, srv.Password = "alice", "secret"
		servers[label] = srv
	}
	paths := map[string]string{}
	for _, col := range mockupSet {
		srv := servers[col.account]
		path := davtest.CalendarHome + col.slug + "/"
		if len(col.kinds) == 1 && col.kinds[0] == view.KindContact {
			path = davtest.AddressBookHome + col.slug + "/"
			srv.AddAddressBook(carddav.AddressBook{Path: path, Name: col.name})
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
	h.rebuild()
	h.admin("mix", "password123")
	sess := h.login("mix", "password123")
	for _, label := range []string{"nextcloud · home", "radicale · vps"} {
		rec := h.do(postForm("/app/settings/connections", sess, url.Values{
			"label": {label}, "server": {servers[label].URL}, "login": {"alice"}, "password": {"secret"},
		}))
		if rec.Code != 303 {
			t.Fatalf("connect %s: %d %s", label, rec.Code, rec.Body.String())
		}
	}
	tickAll(t, h, sess)

	app := &mockupApp{h: h, sess: sess, byCol: map[string]string{}}
	key, _ := sess.DataKey()
	blob, _ := h.users.Catalog(sess.AccountID(), key)
	for _, col := range mockupSet {
		for _, c := range blob.Collections {
			conn, _ := blob.Connection(c.AccountID)
			if c.Path == paths[col.slug] && connLabelOf(conn) == col.account {
				app.byCol[col.slug] = c.AccountID + "/" + url.PathEscape(c.Path)
			}
		}
	}
	if len(app.byCol) != len(mockupSet) {
		t.Fatalf("the set did not reach the catalog: %d of %d", len(app.byCol), len(mockupSet))
	}
	return app
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
	for slug, addr := range a.byCol {
		s = strings.ReplaceAll(s, addr, "a-"+slug)
	}
	s = regexp.MustCompile(`a-[0-9A-Z]{10}`).ReplaceAllString(s, "a-account")
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
// app-contacts/normal.json. It is shown red on purpose — the fixture was
// written from the frame and nothing has yet been made to answer it.
func TestFixtureCombinedContacts(t *testing.T) {
	if os.Getenv("CARREL_FIXTURE_DIFF") == "" {
		t.Skip("set CARREL_FIXTURE_DIFF=1 to compare the screens with docs/spec/fixtures (C-14.2)")
	}
	app := newMockupApp(t)
	page := combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	if page.Poll == nil {
		t.Fatal("the combined book has no poll")
	}
	rec := app.h.do(get("/app/contacts?task="+page.Poll.TaskID, app.sess))
	got := app.normalise(t, rec.Body.Bytes())
	want := loadFixture(t, "app-contacts/normal.json")
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("the screen differs from its fixture (-fixture +screen):\n%s", diff)
	}
}
