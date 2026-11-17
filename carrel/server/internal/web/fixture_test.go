package web_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"
	"github.com/google/go-cmp/cmp"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/dupes"
	"carrel.dev/carrel/server/internal/mail/mailtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/render"
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
	account string // "nextcloud" or "radicale · vps"
	slug    string // how the fixtures name it: kontakty, lichnyj, …
	name    string
	colour  string
	kinds   []view.Kind
	comps   []string
	file    string // under docs/visual/fixtures; empty for an empty collection
	// more is a second file into the same collection: «Работа» holds the
	// events of rabota.ics and the tasks of rabota-zadachi.ics, as the
	// set's README loads it.
	more string
	// readOnly and unticked are what the frames show of «Команда» and
	// «Рассылки»: a book this account may only read, and a collection that
	// stands in the catalog out of the combined views.
	readOnly bool
	unticked bool
}

// mockupSet is the table of docs/visual/fixtures/README.md.
var mockupSet = []mockupCollection{
	{account: "nextcloud", slug: "kontakty", name: "Контакты", colour: "#4A6B52",
		kinds: []view.Kind{view.KindContact}, file: "nextcloud-home/kontakty.vcf"},
	{account: "nextcloud", slug: "arhiv", name: "Архив", colour: "#A05A2C",
		kinds: []view.Kind{view.KindContact}, file: "nextcloud-home/arhiv.vcf"},
	{account: "nextcloud", slug: "lichnyj", name: "Личный", colour: "#4A6B52",
		kinds: []view.Kind{view.KindEvent}, comps: []string{"VEVENT"}, file: "nextcloud-home/lichnyj.ics"},
	// «Дела» before «Работа»: the order of the catalog is the frames' — the
	// tasks' rail «Дела, Работа» (s-tasks-3), the calendar's «Личный,
	// Работа, Дни рождения» (s-agenda-1) — one order for both.
	{account: "nextcloud", slug: "dela", name: "Дела", colour: "#4A6B52",
		kinds: []view.Kind{view.KindTask}, comps: []string{"VTODO"}, file: "nextcloud-home/dela.ics"},
	{account: "nextcloud", slug: "rabota", name: "Работа", colour: "#3F6E8C",
		kinds: []view.Kind{view.KindEvent, view.KindTask}, comps: []string{"VEVENT", "VTODO"}, file: "nextcloud-home/rabota.ics", more: "nextcloud-home/rabota-zadachi.ics"},
	// «Дни рождения» stands with its tick off, as every calendar frame draws
	// it and the set's README says (s-agenda-1…3).
	{account: "nextcloud", slug: "dni-rozhdeniya", name: "Дни рождения", colour: "#A05A2C",
		kinds: []view.Kind{view.KindEvent}, comps: []string{"VEVENT"}, file: "nextcloud-home/dni-rozhdeniya.ics", unticked: true},
	{account: "nextcloud", slug: "zapisi", name: "Записи", colour: "#4A6B52",
		kinds: []view.Kind{view.KindNote}, comps: []string{"VJOURNAL"}, file: "nextcloud-home/zapisi.ics"},
	// «Дневник» is ticked, as the frames of the notes draw it since the
	// mockup of 24.09.2026: its note stands in the combined notebook and
	// the rail counts it (s-notes-1). Until then it was drawn unticked.
	{account: "nextcloud", slug: "dnevnik", name: "Дневник", colour: "#6B4E7A",
		kinds: []view.Kind{view.KindNote}, comps: []string{"VJOURNAL"}, file: "nextcloud-home/dnevnik.ics"},
	// «Общие с семьёй» is the third notebook of every rail of the notes
	// since Typecase 55104b7 (from-typecase note 35, tasks 105, 127): this
	// account may only read it, and it stands with its tick off — «2 of 3
	// notebooks», «—» and «ro» (s-notes-1). Opened, it shows its two notes
	// (states-30, Typecase task 97).
	{account: "nextcloud", slug: "zapiski-obshchie", name: "Общие с семьёй", colour: "#3F6E8C",
		kinds: []view.Kind{view.KindNote}, comps: []string{"VJOURNAL"}, file: "nextcloud-home/zapiski-obshchie.ics", readOnly: true, unticked: true},
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
	// «Общий» is a calendar on a third server whose saved password has
	// stopped fitting: every calendar frame draws it refusing, «password ·
	// 401», with Retry (s-agenda-1…3). It is connected, ticked, and then the
	// stand refuses (mockupRefused).
	{account: "baikal", slug: "obshchij", name: "Общий", colour: "#9A9280",
		kinds: []view.Kind{view.KindEvent}, comps: []string{"VEVENT"}},
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
	// feeds are the two subscriptions of the set, by the address the frames
	// give them, with the stand that serves each (mockupFeeds).
	feeds map[string]*feedServer
	// archive is the size of a backup's archive this run wrote, as the
	// screens write it: the compression of a set whose identifiers are
	// random, so a tenth of a kilobyte either way; the frames draw a size,
	// any one (s-backup-8, -18).
	archive string
}

// mockupFeed is a subscription of the set: the file it serves, the address
// and name the frames give it, and whether the calendar ticks it.
type mockupFeed struct {
	file, url, name, id string
	colour              string // the ribbon's tone in the frames, chosen from the palette
	events              bool   // ticked in the calendar; nothing else is
	refused             bool   // the saved password stopped fitting (s-settings: 401)
}

var mockupFeedSet = []mockupFeed{
	{file: "feeds/ru-holidays.ics", url: "https://feeds.example/ru-holidays.ics", name: "Праздники РФ", id: "f-ru-holidays", colour: "#8C6239", events: true},
	{file: "feeds/league-season.ics", url: "https://league.example/season.ics", name: "Расписание лиги", id: "f-league", colour: "#6B4E7A", refused: true},
}

// mockupFileCols are the file collections of the set, as the frames of the
// files draw them (s-files-1…6): «Documents» and «Public» on a file server of
// their own, «Files» beside nextcloud's calendars. «Public» is read-only and
// unticked — «2 of 3».
var mockupFileCols = []struct {
	account, slug, name, local string
	readOnly, unticked         bool
}{
	{account: "sftpgo · vps", slug: "Documents", name: "Documents", local: "sftpgo-vps/Documents"},
	{account: "sftpgo · vps", slug: "Public", name: "Public", local: "sftpgo-vps/Public", readOnly: true, unticked: true},
	{account: "nextcloud", slug: "Files", name: "Files", local: "nextcloud-home/Files"},
}

// mockupFiles puts the file collections of the set in the catalog and their
// files on the stands, ticks the two the frames tick, and makes
// «Documents / carrel / attachments» the folder attachments go to. The file
// server is connected straight through the service: the stands speak CalDAV,
// and discovery would not take one for a file-only server; the README says
// how a person loads the same by hand.
func mockupFiles(t *testing.T, h *harness, sess *session.Session, root string, servers map[string]*davtest.Server) (map[string]string, map[string]string) {
	t.Helper()
	sftp := davtest.New(t)
	sftp.Login, sftp.Password = "alice", "secret"
	servers["sftpgo · vps"] = sftp
	key, _ := sess.DataKey()
	blob, _ := h.users.Catalog(sess.AccountID(), key)
	ids := map[string]string{}
	for _, c := range blob.Connections {
		ids[connLabelOf(c)] = c.ID
	}
	paths := map[string]string{}
	var sftpCols []sealed.Collection
	for _, fc := range mockupFileCols {
		srv := servers[fc.account]
		path := davtest.FileRoot + fc.slug + "/"
		paths[fc.slug] = path
		// The set's files are stubs of a few bytes; SeedSet gives the ones
		// the frames draw their sizes and times (s-files-1, -2, -5,
		// states-34), as it gives them on the acceptance stand.
		if err := srv.SeedSet(path, filepath.Join(root, filepath.FromSlash(fc.local)), fc.slug, mockupNow(t)); err != nil {
			t.Fatal(err)
		}
		col := sealed.Collection{Path: path, Name: fc.name, Kinds: []view.Kind{view.KindFile}, ReadOnly: fc.readOnly}
		if fc.account == "sftpgo · vps" {
			sftpCols = append(sftpCols, col)
			continue
		}
		col.AccountID = ids[fc.account]
		blob.Collections = append(blob.Collections, col)
	}
	sealedBlob, err := sealed.Seal(key, blob, sess.Login())
	if err != nil {
		t.Fatal(err)
	}
	if err := h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		st.UserByID(sess.AccountID()).SealedBlob = sealedBlob
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	sftpID, err := h.users.AddConnection(sess.AccountID(), key, sealed.Connection{
		Label: "sftpgo · vps", Server: sftp.URL, Login: "alice", Password: "secret", FileRoot: sftp.URL,
	}, sftpCols)
	if err != nil {
		t.Fatal(err)
	}
	ids["sftpgo · vps"] = sftpID

	var picks []sealed.Selection
	for _, fc := range mockupFileCols {
		if !fc.unticked {
			picks = append(picks, sealed.Selection{Kind: view.KindFile, AccountID: ids[fc.account], Path: paths[fc.slug]})
		}
	}
	if err := h.users.SetSelection(sess.AccountID(), key, view.KindFile, picks); err != nil {
		t.Fatal(err)
	}
	if err := h.users.SetAttachmentFolder(sess.AccountID(), key, sealed.AttachmentFolder{
		AccountID: sftpID, Path: paths["Documents"], Folder: []string{"carrel", "attachments"},
	}); err != nil {
		t.Fatal(err)
	}
	return ids, paths
}

// mockupPublished gives out the two links of the frames (s-publish-1) as a
// person does, through the sheet: «Личный» for 10–31 August, made the day
// before the set's day and opened a dozen times since, and
// «смета-подрядчик.pdf» without an expiry, never opened. The frames date
// them after the set's day; here they are before it (to-typecase, task 6).
//
// The token and the identifier of a link are issued when it is made; what
// it returns names them for the fixtures.
func mockupPublished(t *testing.T, h *harness, sess *session.Session, ids, paths map[string]string) map[string]string {
	t.Helper()
	at := func(day string) {
		h.srv.SetClock(mockupDayAt(t, day))
		h.users.SetClock(mockupDayAt(t, day))
	}
	give := func(q url.Values) string {
		rec := h.do(postForm("/app/publish", sess, q))
		if rec.Code != 200 {
			t.Fatalf("publish %v: %d %s", q, rec.Code, rec.Body.String())
		}
		link := string(decode[view.PublishSheet](t, rec).Link)
		if link == "" {
			t.Fatalf("publish %v: no link", q)
		}
		return strings.TrimPrefix(link[strings.Index(link, "/p/"):], "/p/")
	}
	names := map[string]string{}
	at("2026-08-09 19:20")
	calendar := give(url.Values{
		"k": {"calendar"}, "a": {ids["nextcloud"]}, "c": {davtest.CalendarHome + "lichnyj/"},
		"how": {"range"}, "from": {"2026-08-10"}, "to": {"2026-08-31"}, "download": {"on"},
	})
	at("2026-08-10 08:41")
	for i := 0; i < 12; i++ {
		if rec := h.do(get("/p/"+calendar, nil)); rec.Code != 200 {
			t.Fatalf("open the calendar's link: %d %s", rec.Code, rec.Body.String())
		}
	}
	at("2026-08-10 11:05")
	file := give(url.Values{
		"k": {"files"}, "a": {ids["sftpgo · vps"]}, "c": {paths["Documents"]},
		"f": {"carrel/attachments/2026-08/смета-подрядчик.pdf"}, "lives": {"never"},
	})
	h.srv.SetClock(mockupNow(t))
	h.users.SetClock(mockupNow(t))
	names[calendar], names[file] = "link-lichnyj-token", "link-smeta-token"
	for _, l := range h.users.PublishedLinks(sess.AccountID()) {
		names[l.ID] = "link-" + map[string]string{"event": "lichnyj", "file": "smeta"}[l.Target.Kind]
	}
	return names
}

// mockupFeeds subscribes to the two feeds of the set as a person does — the
// form and a name — and ticks «Праздники РФ» in the
// calendar and nowhere else, as the frames do (s-agenda-1, s-tasks-1). The
// league's feed asks for a password, is subscribed with it, and then stops
// taking it: the frames show it refusing.
func mockupFeeds(t *testing.T, h *harness, sess *session.Session, root string) (map[string]*feedServer, map[string]string) {
	t.Helper()
	servers, ids := map[string]*feedServer{}, map[string]string{}
	for _, one := range mockupFeedSet {
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(one.file)))
		if err != nil {
			t.Fatal(err)
		}
		login := ""
		if one.refused {
			login = "mix"
		}
		feed := newFeedServer(t, string(body), login)
		u, _ := url.Parse(one.url)
		form := url.Values{"url": {feed.URL + u.Path}, "name": {one.name}}
		if login != "" {
			form.Set("login", login)
			form.Set("password", "pw")
		}
		if rec := h.do(postForm("/app/settings/connections?op=feed.subscribe", sess, form)); rec.Code != 303 {
			t.Fatalf("subscribe %s: %d %s", one.name, rec.Code, rec.Body.String())
		}
		key, _ := sess.DataKey()
		blob, _ := h.users.Catalog(sess.AccountID(), key)
		sub := blob.Subscriptions[len(blob.Subscriptions)-1]
		col := sub.FeedCollection()
		// The ribbon's tone is chosen as a person chooses it, in «Rename and
		// colour»: the frames draw these two in tones of the palette.
		rename := "/app/collections/rename?" + url.Values{"a": {sub.ID}, "c": {col.Path}}.Encode()
		if rec := h.do(postForm(rename, sess, url.Values{"name": {one.name}, "colour": {one.colour}})); rec.Code != 303 {
			t.Fatalf("colour %s: %d %s", one.name, rec.Code, rec.Body.String())
		}
		if one.events {
			picks := blob.Selected
			picks = append(picks, sealed.Selection{Kind: view.KindEvent, AccountID: sub.ID, Path: col.Path})
			var events []sealed.Selection
			for _, p := range picks {
				if p.Kind == view.KindEvent {
					events = append(events, p)
				}
			}
			if err := h.users.SetSelection(sess.AccountID(), key, view.KindEvent, events); err != nil {
				t.Fatal(err)
			}
		}
		if one.refused {
			feed.mu.Lock()
			feed.login = "someone-else"
			feed.mu.Unlock()
		}
		servers[one.url] = feed
		ids[sub.ID] = one.id
	}
	return servers, ids
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
	a.compareAt(t, name, rec, 200)
}

// compareAt is compare for an answer whose status is not 200: the dead
// link's 404 (rest-1).
func (a *mockupApp) compareAt(t *testing.T, name string, rec *httptest.ResponseRecorder, status int) {
	t.Helper()
	if rec.Code != status {
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

var uidLineRe = regexp.MustCompile(`(?m)^UID:(.+)$`)

func newMockupApp(t *testing.T) *mockupApp {
	t.Helper()
	root := mockupRoot(t)
	servers := map[string]*davtest.Server{}
	at := mockupNow(t)
	for _, label := range []string{"nextcloud", "radicale · vps", "baikal"} {
		srv := davtest.New(t)
		srv.Login, srv.Password = "alice", "secret"
		// The stands keep the day of the frames: «changed on the server» is
		// theirs to say, and a wall clock there moves what a fixture holds.
		srv.Clock = func() time.Time { return at }
		servers[label] = srv
	}
	// Nextcloud and Baikal schedule (RFC 6638), Radicale does not; mix's
	// own address is the one the frames sign with (s-contacts-15).
	for _, label := range []string{"nextcloud", "baikal"} {
		servers[label].ScheduleOutbox = davtest.Principal + "outbox/"
		servers[label].UserAddresses = []string{"mailto:mix@example.org", davtest.Principal}
	}
	paths := map[string]string{}
	for _, col := range mockupSet {
		srv := servers[col.account]
		path := davtest.CalendarHome + col.slug + "/"
		if len(col.kinds) == 1 && col.kinds[0] == view.KindContact {
			path = davtest.AddressBookHome + col.slug + "/"
			srv.AddAddressBook(carddav.AddressBook{Path: path, Name: col.name, ReadOnly: col.readOnly})
		} else {
			srv.AddCalendar(caldav.Calendar{Path: path, Name: col.name, SupportedComponentSet: col.comps, ReadOnly: col.readOnly})
		}
		// The set's order is the frames' (Контакты before Архив, Личный
		// before Дни рождения): the stand lists its collections so, as a
		// server keeps the order they were made in (183 p. 3 of Typecase:
		// the catalog's order is what the screens show).
		srv.Ranked = append(srv.Ranked, path)
		srv.SetColour(path, col.colour)
		paths[col.slug] = path
		var records []string
		for _, file := range []string{col.file, col.more} {
			if file == "" {
				continue
			}
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
			if err != nil {
				t.Fatal(err)
			}
			records = append(records, davtest.SplitRecords(string(data))...)
		}
		for i, rec := range records {
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
	// davloom runs, as the rail's foot of every list frame says (components-36).
	h.cfg.DAVloomPort = 8081
	h.rebuild()
	// The day of the frames, in their zone, and the build they were drawn
	// against: a fixture is the same tomorrow (review Р-13).
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
	// account-1 and s-mail-1: the name mix signs with, the address confirmed
	// on 14 February, and a new one waiting for the link that went to it
	// that morning. Written straight into the record: the journal of the
	// frames (admin-audit) has none of it, and nobody opens that link.
	confirmed := mockupDayAt(t, "2026-02-14 11:42")
	mailed := mockupDayAt(t, "2026-08-10 11:42")
	if err := h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByLogin("mix")
		u.DisplayName = "Михаил Енученко"
		u.Email, u.EmailConfirmed, u.EmailConfirmedAt = "mix@example.org", true, &confirmed
		u.PendingEmail = "new@example.org"
		st.Tokens = append(st.Tokens, store.SecretToken{
			Kind: "email_confirm", Hash: make([]byte, 32), UserID: u.ID, Email: u.PendingEmail,
			CreatedAt: mailed, ExpiresAt: mailed.Add(48 * time.Hour), MailedAt: &mailed,
		})
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	// The account is behind us; everything the run does after this happens
	// on the day of the frames.
	h.users.SetClock(mockupNow(t))
	sess := h.login("mix", "password123")
	// The instance the frames sign into: the account menu names it, and
	// «Carrel» — the name of an instance nobody named — would say nothing.
	if rec := h.do(postForm("/admin/settings", sess, settingsForm(t, h, sess, map[string]string{
		"name": mockupInstance, "inviteTTL": "24", "idle": "2", "session": "7",
	}))); rec.Code != 303 {
		t.Fatalf("name the instance: %d %s", rec.Code, rec.Body.String())
	}
	// Key escrow is on, as account-1 (Deposit my key) and s-admin-6 draw
	// it; mix was made before it, so no copy is taken.
	if rec := h.do(postForm("/admin/escrow", sess, url.Values{
		"action": {"enable"}, "master": {"sixteen-char-pass!"}, "master2": {"sixteen-char-pass!"},
	})); rec.Code != 303 {
		t.Fatalf("turn escrow on: %d %s", rec.Code, rec.Body.String())
	}
	// A relay, and letters from people allowed: the frames offer a letter —
	// «Letter» under the arrow of New note (s-notes-7), Send on a card
	// (s-contacts-6), s-mail-1 «from mix@carrel.example», the login on the
	// instance's domain. Written straight into the record after the
	// instance's settings: the journal of the frames (admin-audit) has none
	// of it.
	if err := h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		st.Settings.SMTP = store.SMTPSettings{Host: "127.0.0.1", Port: 25, Security: "none", Username: "relay",
			Password: "relay-password-xyz", From: "noreply@carrel.example", Sender: "login"}
		people := true
		st.Settings.PeopleMail = &people
		st.Settings.MailMaxAttachMB, st.Settings.MailMaxMessageMB = 10, 25
		st.Settings.MailMaxRecipients, st.Settings.MailMaxPerDay = 20, 100
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"nextcloud", "radicale · vps", "baikal"} {
		rec := h.do(postForm("/app/settings/connections", sess, url.Values{
			"label": {label}, "server": {servers[label].URL}, "login": {"alice"}, "password": {"secret"},
		}))
		if rec.Code != 303 {
			t.Fatalf("connect %s: %d %s", label, rec.Code, rec.Body.String())
		}
	}
	tickMockup(t, h, sess)
	// «Контакты» is where a contact was last made: New contact's arrow says
	// «last used» by it (s-contacts-9). «Личный» is where an event was: New
	// event of the combined calendar writes there (states-3, states-6 draw
	// its arrow; the client's scan of the calendar, Б4).
	lastUsed := map[string]view.Kind{"kontakty": view.KindContact, "lichnyj": view.KindEvent}
	for _, c := range func() []sealed.Collection {
		key, _ := sess.DataKey()
		b, _ := h.users.Catalog(sess.AccountID(), key)
		return b.Collections
	}() {
		if k, ok := lastUsed[collectionSlugOf(c.Path)]; ok {
			key, _ := sess.DataKey()
			if err := h.users.SetLastUsed(sess.AccountID(), key, sealed.Selection{Kind: k, AccountID: c.AccountID, Path: c.Path}); err != nil {
				t.Fatal(err)
			}
		}
	}
	// The password of «baikal» stops fitting once it is connected and
	// ticked: what the frames show of it is the refusal.
	servers["baikal"].Refuse("REPORT", 401, "")
	linkMockupDuplicates(t, h, sess)
	feeds, feedIDs := mockupFeeds(t, h, sess, root)
	fileIDs, filePaths := mockupFiles(t, h, sess, root, servers)
	links := mockupPublished(t, h, sess, fileIDs, filePaths)

	app := &mockupApp{h: h, sess: sess, servers: servers, feeds: feeds, byAccount: feedIDs, byPath: map[string]string{}}
	app.byAccount[fileIDs["sftpgo · vps"]] = "a-sftp-vps"
	for issued, name := range links {
		app.byAccount[issued] = name
	}
	for slug, path := range filePaths {
		app.byPath[url.PathEscape(path)] = slug
	}
	key, _ := sess.DataKey()
	blob, _ := h.users.Catalog(sess.AccountID(), key)
	names := map[string]string{"nextcloud": "a-nc-home", "radicale · vps": "a-rad-vps", "baikal": "a-baikal"}
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
	// Connections counts the files of the storages in the background (the
	// owner's decision of 13.11.2026): the set is ready when they are
	// counted, and no walk runs under what a test does next.
	counted(t, app)
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

// linkMockupDuplicates links the people the set holds in several books
// (docs/visual/fixtures/README.md): the frames show one row with «linked ×N»
// and every source, and that is a decision of S-16, not a property of a file.
// Артём Белов is in two books; Анна Ковалёва in three, with her choice made —
// the work phone of «Команда», the second address of «Контакты» kept
// (from-typecase note 117 p. 2; s-contacts-33).
func linkMockupDuplicates(t *testing.T, h *harness, sess *session.Session) {
	t.Helper()
	key, _ := sess.DataKey()
	blob, _ := h.users.Catalog(sess.AccountID(), key)
	// members are the records of the set in these books under these UIDs
	// («komanda/4f2b-9c31-a0e7»), in this order: the folded row is the
	// group's first member (S-16), and the order of the catalog is not a
	// decision. Анна's copies share her UID, as «Copy» writes them.
	members := func(who string, at ...string) []sealed.RecordRef {
		var out []sealed.RecordRef
		for _, one := range at {
			slug, uid, _ := strings.Cut(one, "/")
			for _, c := range blob.Collections {
				if collectionSlugOf(c.Path) == slug && slices.Contains(mockupUIDs(t, slug), uid) {
					out = append(out, sealed.RecordRef{AccountID: c.AccountID, Col: c.Path, UID: uid})
				}
			}
		}
		if len(out) != len(at) {
			t.Fatalf("%s is not where the set puts them: %+v", who, out)
		}
		return out
	}
	groups := []sealed.LinkedGroup{
		// The frames show the «Команда» copy of Артём — its photo, its stamp.
		{ID: "g-artem", Kind: view.KindContact, Title: "Артём Белов", Members: members("Артём Белов", "komanda/d41a-6f30-92be", "kontakty/c3f7-8a21-1b09")},
		{ID: "g-anna", Kind: view.KindContact, Title: "Анна Ковалёва",
			Members: members("Анна Ковалёва", "kontakty/4f2b-9c31-a0e7", "komanda/4f2b-9c31-a0e7", "arhiv/4f2b-9c31-a0e7"),
			Dropped: map[string][]string{"TEL": {dupes.ValueKey("TEL", "+7 495 120-45-00")}}},
		// Two notes of «Записи» copied into «Дневник», linked: one row each
		// on every screen (s-dup-1, s-notes-1, -32; note 123 p. 6) — the
		// copy of the meeting shorter and without «релиз».
		{ID: "g-vstrecha", Kind: view.KindNote, Title: "Заметка со встречи 10.08", Members: members("Заметка со встречи 10.08", "zapisi/5c81-77aa-0d64", "dnevnik/9e31-40b7-c012")},
		{ID: "g-crdt", Kind: view.KindNote, Title: "Что почитать про CRDT", Members: members("Что почитать про CRDT", "zapisi/6d92-13ef-b408", "dnevnik/8e04-21ab-d517")},
	}
	// Анна and Алексей Смирнов share the work phone of the publisher: 60
	// points, and «not duplicates» — the decision s-dup-1 and s-dup-6 draw.
	pair := members("Анна Ковалёва and Алексей Смирнов", "kontakty/4f2b-9c31-a0e7", "kontakty/9a1c-03de-77f2")
	err := h.users.EditDuplicates(sess.AccountID(), key, "duplicates.link", "", func(d *sealed.Duplicates) error {
		d.Linked = append(d.Linked, groups...)
		d.Ignored = append(d.Ignored, sealed.IgnoredGroup{ID: dupes.Signature(view.KindContact, pair), Kind: view.KindContact, Members: pair,
			Title: "Анна Ковалёва · Алексей Смирнов"})
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
		// A folder's address as a query value (New folder of a tree: to=)
		// carries the escaped path escaped once more.
		s = strings.ReplaceAll(s, url.QueryEscape(path), slug)
		s = strings.ReplaceAll(s, path, slug)
	}
	// The stands listen on whatever port the machine had free; the frames
	// show an address. Same kind as an issued identifier.
	for label, name := range map[string]string{
		"nextcloud":      "https://nextcloud.example/dav.php",
		"radicale · vps": "https://radicale.example",
		"sftpgo · vps":   "https://files.example.org/dav",
		"baikal":         "https://baikal.example/dav.php",
	} {
		if srv := a.servers[label]; srv != nil {
			s = strings.ReplaceAll(s, srv.URL, name)
			// The DAVx5 link of a stand (s-plan-2): login@host.
			s = strings.ReplaceAll(s, "@"+strings.TrimPrefix(srv.URL, "http://"), "@"+strings.TrimPrefix(name, "https://"))
			// The host alone (Disconnect's «nextcloud · cloud.example.org»,
			// s-profile-26): the address's host.
			host, _, _ := strings.Cut(strings.TrimPrefix(name, "https://"), "/")
			s = strings.ReplaceAll(s, strings.TrimPrefix(srv.URL, "http://"), host)
		}
	}
	// A DAVx5 code is the picture of its link (s-plan-2), whose port is the
	// stand's: the frames draw a code, any one.
	s = regexp.MustCompile(`"qr":"data:image/png;base64,[A-Za-z0-9+/=]+"`).ReplaceAllString(s, `"qr":"data:image/png;base64,code"`)
	// A UID Carrel gives a record is random (model.NewUID, 32 hex digits):
	// the frames draw one, any one (s-io-9: «→ 9d04-77c1»).
	s = regexp.MustCompile(`→ [0-9a-f]{32}\b`).ReplaceAllString(s, "→ 9d04-77c1")
	// How long a feed took to read is the machine's (0 ms, 1 ms): the frames
	// draw a number, any one.
	s = regexp.MustCompile(`"took":"\d+ ms"`).ReplaceAllString(s, `"took":"0 ms"`)
	for frame, feed := range a.feeds {
		u, _ := url.Parse(frame)
		s = strings.ReplaceAll(s, feed.URL, u.Scheme+"://"+u.Host)
		// A feed's stamp is its host, and the stand's has a port of its own.
		s = strings.ReplaceAll(s, strings.TrimPrefix(feed.URL, "http://"), u.Host)
	}
	// The secret of an invitation is issued at the moment it is made, and
	// it stands inside an address rather than alone, so the rule for the
	// CSRF does not reach it. Secrets go first and whole: a random one can
	// hold a run that looks like an identifier below.
	s = regexp.MustCompile(`/invite/[A-Za-z0-9_-]{43}`).ReplaceAllString(s, "/invite/invite-token")
	s = regexp.MustCompile(`/forgot/[A-Za-z0-9_-]{43}`).ReplaceAllString(s, "/forgot/reset-token")
	s = regexp.MustCompile(`/password-history/[A-Za-z0-9_-]{43}`).ReplaceAllString(s, "/password-history/history-token")
	s = regexp.MustCompile(`/p/[A-Za-z0-9_-]{43}`).ReplaceAllString(s, "/p/link-token")
	// A link's identifier is a digest of its secret (Send by letter names it).
	s = regexp.MustCompile(`(compose|link)\?id=[0-9a-f]{16}`).ReplaceAllString(s, "${1}?id=link-id")
	s = regexp.MustCompile(`"[A-Za-z0-9_-]{43}"`).ReplaceAllString(s, `"csrf"`)
	// A temporary password a form offers (s-admin-1, s-admin-6) is made for
	// each answer.
	s = regexp.MustCompile(`"[a-z2-7]{4}(?:-[a-z2-7]{4}){4}"`).ReplaceAllString(s, `"temp-password"`)
	s = regexp.MustCompile(`t-[0-9A-Z]{10}`).ReplaceAllString(s, "t-task")
	s = regexp.MustCompile(`plan=r-[0-9A-Z]{10}`).ReplaceAllString(s, "plan=r-plan")
	s = regexp.MustCompile(`transfer=q-[0-9A-Z]{10}`).ReplaceAllString(s, "transfer=q-transfer")
	s = regexp.MustCompile(`u-[0-9A-Z]{10}`).ReplaceAllString(s, "u-mix")
	// Another session's public ID in its End (account-1).
	s = regexp.MustCompile(`op=end(\\u0026|&)s=[0-9a-f]{16}`).ReplaceAllString(s, "op=end${1}s=session")
	// How long a feed's fetch took on this machine (s-profile-1, -10).
	s = regexp.MustCompile(`(→ 200 · )\d+ ms`).ReplaceAllString(s, "${1}0 ms")
	s = regexp.MustCompile(`("took": ?")\d+ ms"`).ReplaceAllString(s, `${1}0 ms"`)
	// A backup job's and a run's ID (s-backup-1): the frames name them by
	// what they are.
	// Numbered in the order they appear: the forms of two jobs' rows are
	// two forms.
	jobs := map[string]string{}
	s = regexp.MustCompile(`\bj-[0-9A-Z]{10}\b`).ReplaceAllStringFunc(s, func(id string) string {
		if _, ok := jobs[id]; !ok {
			jobs[id] = "j-job"
			if n := len(jobs); n > 1 {
				jobs[id] += strconv.Itoa(n)
			}
		}
		return jobs[id]
	})
	s = regexp.MustCompile(`r-[0-9A-Z]{10}\b`).ReplaceAllString(s, "r-run")
	if a.archive != "" {
		s = strings.ReplaceAll(s, `"size":"`+a.archive+`"`, `"size":"4.4 KB"`)
		s = strings.ReplaceAll(s, `"text":"`+a.archive+` · "`, `"text":"4.4 KB · "`)
	}
	// The secret of a job's line, issued for each answer (s-backup-2).
	s = regexp.MustCompile(`cbk_[A-Za-z0-9_-]+`).ReplaceAllString(s, "cbk_secret")
	// A davloom device's ID (NewDeviceID): the frames name no device by it.
	// Numbered in the order they appear, as jobs are: two devices' Revoke
	// are two forms.
	devices := map[string]string{}
	s = regexp.MustCompile(`\bd-[0-9A-Z]{10}\b`).ReplaceAllStringFunc(s, func(id string) string {
		if _, ok := devices[id]; !ok {
			devices[id] = "d-device"
			if n := len(devices); n > 1 {
				devices[id] += strconv.Itoa(n)
			}
		}
		return devices[id]
	})
	// The one-time link to an Apple profile (s-plan-1) is made for each
	// issue.
	s = regexp.MustCompile(`profile\?t=[A-Za-z0-9_-]{43}`).ReplaceAllString(s, "profile?t=token")
	// The test objects of the DAV validator's second stage (s-admin-29).
	s = regexp.MustCompile(`carrel-check-[0-9a-f]{12}`).ReplaceAllString(s, "carrel-check")
	// A group of duplicates is named by a digest of the records in it, and
	// those carry the account identifier, which is issued.
	s = regexp.MustCompile(`g=[0-9a-f]{32}`).ReplaceAllString(s, "g=group")
	s = regexp.MustCompile(`group=[0-9a-f]{32}`).ReplaceAllString(s, "group=group")
	s = regexp.MustCompile(`"groupID": ?"[0-9a-f]{32}"`).ReplaceAllString(s, `"groupID":"group"`)
	// And so is the group itself and the hidden field of its form (the
	// candidate of the set, Typecase's note 44: a group still to decide).
	s = regexp.MustCompile(`"(id|value)": ?"[0-9a-f]{32}"`).ReplaceAllString(s, `"$1":"group"`)
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

// numLike is a moment in numbers, as the tables of the settings write it
// (Нс2: «2026-08-16 09:02»); a run's is the day of the frames in the same
// shape, mockupDay.
var numLike = regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}$`)

// tookLike is a duration as tookText writes it: «0 ms», «4.2 s».
var tookLike = regexp.MustCompile(`^(\d+ ms|\d+\.\d s)$`)

func steadyStamps(v any) {
	switch t := v.(type) {
	case map[string]any:
		for k, x := range t {
			// A link's last open is written by the service's clock, which
			// the set freezes (mockupPublished: 08:41): it is data, not a
			// stamp of the run (the client's scan ФД5).
			if _, link := t["opens"]; link && k == "lastSeen" {
				continue
			}
			// How long a feed's last read took is the run's machine: «0 ms»
			// on a quiet one, «0.1 s» under the gates' load (28.10.2026). It
			// is of the run, as a stamp is, and the set writes the quiet one.
			if s, ok := x.(string); ok && k == "took" && tookLike.MatchString(s) {
				if _, feed := t["checked"]; feed {
					t[k] = "0 ms"
					continue
				}
			}
			if s, ok := x.(string); ok && runStamps[k] && dateLike.MatchString(s) {
				t[k] = mockupStamp
				continue
			}
			if s, ok := x.(string); ok && runStamps[k] && numLike.MatchString(s) {
				t[k] = mockupDay
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
	// Анна is open in the panel, as the frame draws her (s-contacts-1).
	open := ""
	for _, g := range page.Groups {
		for _, r := range g.Rows {
			if r.Name == "Анна Ковалёва" {
				open = recordHref(r.Href)
			}
		}
	}
	// Written into the directory of CARREL_FIXTURE_WRITE as every fixture
	// is (compare): one shot of all the fixtures, none by hand.
	app.compare(t, "app-contacts/normal.json", app.h.do(get("/app/contacts?task="+page.Poll.TaskID+"&open="+url.QueryEscape(open), app.sess)))
	// On paper (print-2): the same list, no panel.
	app.compare(t, "app-contacts/print.json", app.h.do(get(onPaper("/app/contacts?task="+page.Poll.TaskID), app.sess)))
}

// C-14.2, the partial state of the same screen: the poll has not finished.
// One book answered from what the session already held, the other two are
// still being asked — the two stands are held open while the screen is
// taken (davtest.Stall).
func TestFixtureCombinedContactsPartial(t *testing.T) {
	app := newMockupApp(t)
	// The person has looked at every book before, an hour ago: the session
	// knows how many cards each holds — the sections of the rail keep those
	// numbers while a poll runs (states-8) —, and what it read then is too
	// old to answer from, so every book is asked again.
	app.h.srv.SetClock(mockupNow(t).Add(-time.Hour))
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	app.h.srv.SetClock(mockupNow(t))
	// «Контакты» is read alone first, so the combined poll finds it in the
	// session and answers from there: that is what «from cache» means.
	if rec := app.h.do(get(app.listPath(t, "contacts", "kontakty"), app.sess)); rec.Code != 200 {
		t.Fatalf("read the book alone: %d", rec.Code)
	}
	// «Команда» lives on the other stand, and that stand is held: its book
	// is still being asked while the two of nextcloud have answered.
	release := app.servers["radicale · vps"].Stall("REPORT")
	defer release()
	// A book the session has read is first asked whether it changed: that
	// question is held too.
	unchanged := app.servers["radicale · vps"].Stall("PROPFIND")
	defer unchanged()

	app.h.srv.SetClock(mockupNow(t))
	first := decode[view.ContactsListPage](t, app.h.do(get("/app/contacts", app.sess)))
	if first.Poll == nil {
		t.Fatal("the combined book has no poll")
	}
	// Two answered and the third being asked — not still waiting for its
	// turn, which a loaded machine shows for a moment.
	waitPoll(t, app.h, app.sess, first.Poll.TaskID, func(b view.PollBar) bool {
		done, asked := 0, 0
		for _, sp := range b.Sources {
			switch sp.State {
			case view.SourceDone, view.SourceEmpty:
				done++
			case view.SourceQuerying:
				asked++
			}
		}
		return done == 2 && asked == 1
	})
	app.compare(t, "app-contacts/partial.json", app.h.do(get("/app/contacts?task="+first.Poll.TaskID, app.sess)))
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

	// As s-conflict-1 (Рв6, Рв11): the card was opened without its work
	// phone, its note and its file-as line, under another display name and
	// address. The person gives it the name, the work phone, a new address
	// and the note of the set; meanwhile the other client gave it the same
	// name, an address of its own and the file-as line — FN agrees, TEL;work
	// and NOTE are only in the edit, EMAIL;work both changed, and
	// X-EVOLUTION-FILE-AS, which the form does not show, only the server.
	at := davtest.AddressBookHome + "kontakty/" + uid + ".vcf"
	set := setRecord(t, "nextcloud-home/kontakty.vcf", uid)
	base := strings.Replace(set, "FN:Анна Ковалёва\n", "FN:Ковалёва Анна\n", 1)
	base = strings.Replace(base, "TEL;TYPE=WORK:+7 495 120-45-00\n", "", 1)
	base = strings.Replace(base, "EMAIL;TYPE=WORK:a.kovaleva@example.org\n", "EMAIL;TYPE=WORK:anna@slovo.example\n", 1)
	base = regexp.MustCompile(`(?m)^NOTE:.*\n( .*\n)*`).ReplaceAllString(base, "")
	base = strings.Replace(base, "X-EVOLUTION-FILE-AS:Ковалёва\\, Анна\n", "", 1)
	app.servers["nextcloud"].Seed(at, base)

	form := formFields(decode[view.ContactEditPage](t, app.h.do(get(rec+"/edit", app.sess))).Form)
	server := strings.Replace(base, "FN:Ковалёва Анна\n", "FN:Анна Ковалёва\n", 1)
	server = strings.Replace(server, "EMAIL;TYPE=WORK:anna@slovo.example\n", "EMAIL;TYPE=WORK:anna.kovaleva@slovo.example\n", 1)
	server = strings.Replace(server, "X-ABLABEL:", "X-EVOLUTION-FILE-AS:Ковалёва\\, Анна\nX-ABLABEL:", 1)
	server = strings.Replace(server, "REV:20260814T090412Z\n", "REV:20260810T140200Z\n", 1)
	app.servers["nextcloud"].Seed(at, server)
	form.Set("fn", "Анна Ковалёва")
	form.Set("tel.1", "+7 495 120-45-00")
	form.Set("tel.1.type", "work")
	form.Set("email.0", "a.kovaleva@example.org")
	form.Set("note", "Пишет по вторникам, отвечает в тот же день.")
	if resp := app.h.do(postForm(rec+"/edit", app.sess, form)); resp.Code != 303 {
		t.Fatalf("POST over a moved version: %d %s", resp.Code, resp.Body.String())
	}

	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	app.compare(t, "app-conflict/normal.json", app.h.do(get(rec+"/conflict", app.sess)))
}

// mockupAnnaPhoto gives Анна Ковалёва the photograph the frames of her card
// and its form draw (s-contacts-6, -16, -3); the list and the timeline draw
// her initials (s-contacts-1, -7), and their fixtures are taken without it.
// It is put on the stand before anything is read: the version read is the
// version shown.
func mockupAnnaPhoto(t *testing.T, app *mockupApp) {
	t.Helper()
	path := davtest.AddressBookHome + "kontakty/4f2b-9c31-a0e7.vcf"
	body, _, ok := app.servers["nextcloud"].Object(path)
	if !ok {
		t.Fatal("Анна is not on the stand")
	}
	photo := "PHOTO;ENCODING=b;TYPE=JPEG:" + b64(jpegOf(t, halvesImage(120, 120))) + "\n"
	app.servers["nextcloud"].Seed(path, strings.Replace(strings.ReplaceAll(body, "\r\n", "\n"), "REV:", photo+"REV:", 1))
}

// mockupAnnaMeetings gives Anna a second, older address and the meetings
// that name her old ones, as s-contacts-15 counts them: in «Работа» on
// nextcloud — organised by mix or by others, coming or past —, and in
// «Общий» on baikal, which answers now and did not say whether it
// schedules («On baikal»).
func mockupAnnaMeetings(t *testing.T, app *mockupApp) {
	t.Helper()
	nc, baikal := app.servers["nextcloud"], app.servers["baikal"]
	card := davtest.AddressBookHome + "kontakty/4f2b-9c31-a0e7.vcf"
	body, _, ok := nc.Object(card)
	if !ok {
		t.Fatal("Анна is not on the stand")
	}
	nc.Seed(card, strings.Replace(body, "EMAIL;TYPE=WORK:a.kovaleva@example.org", "EMAIL;TYPE=WORK:a.kovaleva@example.org\r\nEMAIL;TYPE=HOME:anna@oldfirm.example", 1))
	meet := func(uid, summary, when, organiser string, names ...string) string {
		lines := []string{"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//T//EN", "BEGIN:VEVENT",
			"UID:" + uid, "DTSTAMP:20260801T000000Z", "DTSTART:" + when + "T100000Z", "DTEND:" + when + "T110000Z",
			"SUMMARY:" + summary, "ORGANIZER;CN=Organiser:mailto:" + organiser}
		for _, n := range names {
			lines = append(lines, "ATTENDEE;CN=Анна Ковалёва;PARTSTAT=ACCEPTED:mailto:"+n)
		}
		return strings.Join(append(lines, "END:VEVENT", "END:VCALENDAR", ""), "\r\n")
	}
	const (
		mine   = "mix@example.org"
		boss   = "boss@slovo.example"
		work   = "a.kovaleva@example.org"
		oldjob = "anna@oldfirm.example"
	)
	rabota := davtest.CalendarHome + "rabota/"
	for uid, ev := range map[string]string{
		"aa02": meet("aa02", "Ретро спринта", "20260814", boss, work),
		"aa03": meet("aa03", "Планирование Q4", "20260902", mine, work),
		"cc01": meet("cc01", "Встреча с подрядчиком", "20260818", boss, work, oldjob),
		"cc02": meet("cc02", "Юбилей отдела", "20260905", boss, oldjob),
		"dd01": meet("dd01", "Синк-ап по релизу", "20260803", mine, work),
		"dd02": meet("dd02", "Синк-ап по релизу", "20260727", mine, work, oldjob),
	} {
		nc.Seed(rabota+uid+".ics", ev)
	}
	obshchij := davtest.CalendarHome + "obshchij/"
	baikal.Seed(obshchij+"bb01.ics", meet("bb01", "Общий созвон", "20260812", mine, work))
	baikal.Seed(obshchij+"bb02.ics", meet("bb02", "Демо для заказчика", "20260821", mine, work))
	baikal.Accept("REPORT")
	// baikal did not say whether it schedules: what the connection holds of
	// it, the collections as they are.
	id := app.realAccount(t, "obshchij")
	key, _ := app.sess.DataKey()
	blob, _ := app.h.users.Catalog(app.sess.AccountID(), key)
	var found []sealed.Collection
	for _, c := range blob.Collections {
		if c.AccountID == id {
			found = append(found, c)
		}
	}
	if err := app.h.users.ApplyRediscovery(app.sess.AccountID(), key, id, user.Rediscovery{Found: found, Addresses: []string{mine}}); err != nil {
		t.Fatal(err)
	}
}

// C-14.2: the card of a contact, read (s-contacts-6): with her photograph,
// and with the timeline this session gathered in its inset (S-15.4).
func TestFixtureContactCard(t *testing.T) {
	app := newMockupApp(t)
	mockupAnnaPhoto(t, app)
	// The list was opened before the card: that is where the rail's counts
	// and «read 2 min ago» come from.
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	rec := app.listPath(t, "contacts", "kontakty") + "/4f2b-9c31-a0e7"
	tl := decode[view.ContactTimelinePage](t, app.h.do(get(rec+"/timeline", app.sess)))
	waitPoll(t, app.h, app.sess, tl.Poll.TaskID, func(b view.PollBar) bool { return !b.Running })
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	app.compare(t, "app-contact/normal.json", app.h.do(get(rec, app.sess)))
	app.compare(t, "app-contact/print.json", app.h.do(get(onPaper(rec), app.sess))) // print-3
}

// C-14.2: Anna's two addresses replaced on her card — the sheet asks
// whether to rewrite them where she is named (s-contacts-15), and the card
// says what came of it (s-contacts-16). Her meetings are in «Личный» and
// «Работа» on nextcloud, which schedules — some organised by others, some
// past — and in «Общий» on baikal, which did not say whether it schedules:
// the four groups of the frame.
func TestFixtureContactRewrite(t *testing.T) {
	app := newMockupApp(t)
	mockupAnnaPhoto(t, app)
	mockupAnnaMeetings(t, app)
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	rec := app.listPath(t, "contacts", "kontakty") + "/4f2b-9c31-a0e7"
	// Her timeline was gathered before the addresses changed, as on her
	// card (s-contacts-6): the outcome still counts what it found, by the
	// old addresses (s-contacts-16).
	tl := decode[view.ContactTimelinePage](t, app.h.do(get(rec+"/timeline", app.sess)))
	waitPoll(t, app.h, app.sess, tl.Poll.TaskID, func(b view.PollBar) bool { return !b.Running })
	form := formFields(decode[view.ContactEditPage](t, app.h.do(get(rec+"/edit", app.sess))).Form)
	form.Set("email.0", "a.kovaleva@slovo.example")
	form.Set("email.1", "anna@home.example")
	saved := app.h.do(postForm(rec+"/edit", app.sess, form))
	sheet := saved.Header().Get("Location")
	if saved.Code != 303 || !strings.HasPrefix(sheet, rec+"/rewrite?plan=") {
		t.Fatalf("save: %d %q", saved.Code, sheet)
	}
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	got := app.h.do(get(sheet, app.sess))
	app.compare(t, "app-contact-rewrite/normal.json", got)
	confirm := decode[view.RewriteSheet](t, got).Actions[0]
	// The server of a meeting organised by another refuses the change, as
	// in the frame's outcome (s-contacts-16): the card says what was not
	// rewritten.
	app.servers["nextcloud"].RefuseAt("PUT", davtest.CalendarHome+"rabota/aa02.ics", http.StatusForbidden, "")
	app.compare(t, "app-contact/outcome.json", app.h.do(postForm(confirm.Href, app.sess, url.Values{})))
}

// C-14.2: one calendar as an agenda — its own frame since Typecase 80d1319
// (s-agenda-17, from-typecase note 36): the calendar's row open in the rail,
// no record open in a panel.
func TestFixtureCalendarAgenda(t *testing.T) {
	app := newMockupApp(t)
	// Every ticked calendar carries its number in the frame (s-agenda-1): the
	// combined calendar was opened first, as a person comes to it.
	combined(t, app.h, app.sess, "/app/calendar?from=2026-08-10", func(p view.AgendaPage) *view.PollBar { return p.Poll })
	// The notes were read too: the panel's Related names its note (s-agenda-1).
	openList(t, app, "/app/notes")
	// The address books were read too: an attendee found in one is known
	// (s-agenda-1, -2: the mark of the book and a link; task 77).
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	here := app.listPath(t, "calendar", "lichnyj") + "?from=2026-08-10"
	// No panel: the frame of one calendar draws none (the panel is the
	// combined calendar's, s-agenda-1).
	app.compare(t, "app-agenda/normal.json", app.h.do(get(here, app.sess)))
	app.compare(t, "app-agenda/print.json", app.h.do(get(onPaper(here), app.sess))) // print-7
	// With a word: what is found in the whole calendar, by day (s-agenda-31).
	found := app.listPath(t, "calendar", "lichnyj") + "?filter=" + url.QueryEscape("синк")
	app.compare(t, "app-agenda/filtered.json", app.h.do(get(found, app.sess)))
}

// C-14.2: the suggestions of the attendees' field as s-agenda-12 draws them
// — «ве» typed, Вера of «Архив» found by her name in the books the session
// has read (task 77, server v0.2.139).
func TestFixtureAttendeeSuggest(t *testing.T) {
	app := newMockupApp(t)
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	app.compare(t, "app-contacts-suggest/normal.json", app.h.do(get("/app/contacts/suggest?q="+url.QueryEscape("ве"), app.sess)))
}

// C-14.2: the combined calendar as s-agenda-1 draws it — every ticked
// calendar polled, baikal refusing, the screen two minutes after the poll
// (the client's scan of the calendar, Б1).
func TestFixtureCombinedCalendar(t *testing.T) {
	app := newMockupApp(t)
	openList(t, app, "/app/notes") // the panel's Related names its note
	// The address books were read too: an attendee found in one is known
	// (s-agenda-1, -2: the mark of the book and a link; task 77).
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	page := combined(t, app.h, app.sess, "/app/calendar?from=2026-08-10", func(p view.AgendaPage) *view.PollBar { return p.Poll })
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	// «Синк-ап по релизу» open in the panel (s-agenda-1, В1).
	open := app.listPath(t, "calendar", "lichnyj") + "/8f1c-4a02-b7d3"
	app.compare(t, "app-agenda-all/normal.json", app.h.do(get("/app/calendar?from=2026-08-10&task="+page.Poll.TaskID+"&open="+url.QueryEscape(open), app.sess)))
	// The same week on paper (print-1, U-10).
	app.compare(t, "app-agenda-all/print.json", app.h.do(get("/app/calendar?from=2026-08-10&task="+page.Poll.TaskID+"&"+render.PrintParam+"="+render.PrintValue, app.sess)))
}

// C-14.2: the card of an event (s-agenda-2).
func TestFixtureEventCard(t *testing.T) {
	app := newMockupApp(t)
	linkMockupEvent(t, app)
	combined(t, app.h, app.sess, "/app/calendar?from=2026-08-10", func(p view.AgendaPage) *view.PollBar { return p.Poll })
	openList(t, app, "/app/notes") // Related names its note (s-agenda-2, Г8)
	// The address books were read too: an attendee found in one is known
	// (s-agenda-1, -2: the mark of the book and a link; task 77).
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	app.h.do(get(app.listPath(t, "calendar", "lichnyj")+"?from=2026-08-10", app.sess))
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	// The rail draws «Общий» refusing, as the poll found it; the copy there
	// is read when the card opens, and the frame draws it answered — «Where
	// differ» (s-agenda-2).
	app.servers["baikal"].Accept("REPORT")
	rec := app.listPath(t, "calendar", "lichnyj") + "/8f1c-4a02-b7d3"
	app.compare(t, "app-event/normal.json", app.h.do(get(rec, app.sess)))
}

// C-14.2: the sheet of deleting a record (note 69 of Typecase, task 166).
// normal — «Синк-ап по релизу» with its copies, its attendees on nextcloud,
// which schedules, its attachments — one written into it, one a link into
// the attachment folder —, its published link and the note related to it
// (s-agenda-26); single — the task «Отправить документы в МФЦ», a published
// link alone pointing at it (s-tasks-18).
func TestFixtureRecordDelete(t *testing.T) {
	app := newMockupApp(t)
	linkMockupEvent(t, app)
	attachMockupRecording(t, app)
	sftp := app.servers["sftpgo · vps"]
	at := davtest.FileRoot + "Documents/carrel/attachments/"
	sftp.SeedFile(at+"agenda-10-08.pdf", "%PDF")
	nc := app.servers["nextcloud"]
	ev := davtest.CalendarHome + "lichnyj/8f1c-4a02-b7d3.ics"
	body, _, ok := nc.Object(ev)
	if !ok {
		t.Fatal("the event is not on the stand")
	}
	link := "ATTACH;FMTTYPE=application/pdf;SIZE=4:" + sftp.URL + at + "agenda-10-08.pdf"
	nc.Seed(ev, strings.Replace(body, "END:VEVENT", link+"\r\nEND:VEVENT", 1))
	combined(t, app.h, app.sess, "/app/calendar?from=2026-08-10", func(p view.AgendaPage) *view.PollBar { return p.Poll })
	openList(t, app, "/app/notes")
	openList(t, app, "/app/tasks")
	app.servers["baikal"].Accept("REPORT")
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	del := func(t *testing.T, kind, col, uid string) *httptest.ResponseRecorder {
		t.Helper()
		page := app.h.do(get(app.listPath(t, kind, col)+"/"+uid, app.sess))
		var p struct {
			Actions []view.Action `json:"actions"`
		}
		if err := json.Unmarshal(page.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return app.h.do(postForm(actionHref(t, p.Actions, "record.delete"), app.sess, url.Values{}))
	}
	publish := func(t *testing.T, kind, col, uid string) {
		t.Helper()
		q := url.Values{"k": {kind}, "a": {app.realAccount(t, col)}, "c": {app.realPath(t, col)}, "uid": {uid}}
		sheet := decode[view.PublishSheet](t, app.h.do(get("/app/publish?"+q.Encode(), app.sess)))
		app.h.do(postForm(actionHref(t, sheet.Actions, "publish.create"), app.sess, url.Values{}))
	}
	t.Run("normal", func(t *testing.T) {
		publish(t, "calendar", "lichnyj", "8f1c-4a02-b7d3")
		app.compare(t, "app-record-delete/normal.json", del(t, "calendar", "lichnyj", "8f1c-4a02-b7d3"))
	})
	t.Run("single", func(t *testing.T) {
		publish(t, "tasks", "dela", "aa10-77bc-0e31")
		app.compare(t, "app-record-delete/single.json", del(t, "tasks", "dela", "aa10-77bc-0e31"))
	})
}

// linkMockupEvent puts two copies of «Синк-ап по релизу» — in radicale ·
// vps · Проекты the same, in baikal · Общий half an hour later and in
// another room — and links the three (S-16): «When» and «Where» on the
// group's card, «baikal · Общий · When, where differ» and «linked» at When
// and Where (s-dup-1, s-agenda-2, -25; from-typecase notes 57 and 117 p. 3;
// the client's scan Г9). The copies live in this test alone: the agenda
// frames draw no such rows. They hold neither the file nor the note it is
// related to: on the linked record both are marked «Личный» (s-agenda-25;
// Р-78, note 123 p. 10).
func linkMockupEvent(t *testing.T, app *mockupApp) {
	t.Helper()
	linkMockupCopies(t, app, view.KindEvent, "g-sinkap", mockupRecord{"nextcloud", "lichnyj", "8f1c-4a02-b7d3", "nextcloud-home/lichnyj.ics"},
		mockupCopy{rec: mockupRecord{"radicale · vps", "proekty", "4b2e-9d10-8f1c", ""}, edit: bareCopy},
		mockupCopy{rec: mockupRecord{"baikal", "obshchij", "6a7d-2c51-e930", ""}, edit: func(rec string) string {
			rec = bareCopy(rec)
			// 12:30 in Yekaterinburg is 10:30 in Moscow (s-dup-1).
			rec = strings.Replace(rec, "Yekaterinburg:20260810T120000", "Yekaterinburg:20260810T123000", 1)
			rec = strings.Replace(rec, "Yekaterinburg:20260810T130000", "Yekaterinburg:20260810T133000", 1)
			return strings.Replace(rec, "LOCATION:Переговорная 2", "LOCATION:Переговорная 3", 1)
		}})
}

// linkMockupTask puts a copy of «Собрать чемодан», due on the day without a
// time and of priority 3, in nextcloud · Работа and links the two (S-16),
// the priority of Работа chosen: «linked ×2», «2 sources» (s-tasks-1), «Due»
// and «Priority» on the group's card (s-dup-1), «Дела · Priority differs»,
// «Работа · Due differs» (s-tasks-17) and «linked» at Due and Priority
// (s-tasks-2; from-typecase notes 57 and 117; the client's scan ЗБ6, ЗГ5).
// The copy lives in the tests alone.
func linkMockupTask(t *testing.T, app *mockupApp) {
	t.Helper()
	linkMockupCopy(t, app, view.KindTask, "g-chemodan", chemodan, chemodanCopy.rec, chemodanCopy.edit)
	chooseMockup(t, app, "g-chemodan", "PRIORITY", "cc11-5a20-7e94")
}

// chemodan and chemodanCopy are «Собрать чемодан» of «Дела» and its copy in
// «Работа»: due on the day without a time, of priority 3, without the file
// and the trip, which are «Дела»'s (s-tasks-17).
var (
	chemodan     = mockupRecord{"nextcloud", "dela", "aa11-2d09-5f84", "nextcloud-home/dela.ics"}
	chemodanCopy = mockupCopy{rec: mockupRecord{"nextcloud", "rabota", "cc11-5a20-7e94", ""}, edit: func(rec string) string {
		rec = bareCopy(rec)
		rec = strings.Replace(rec, "DUE;TZID=Europe/Moscow:20260812T180000", "DUE;VALUE=DATE:20260812", 1)
		return strings.Replace(rec, "PRIORITY:5", "PRIORITY:3", 1)
	}}
)

// seedMockupWhole puts into the collections what they hold beyond what the
// lists of the frames draw: «Поездка в Казань» in «Личный» and the copy of
// «Собрать чемодан» in «Работа», unlinked. The settings count them — Личный
// 5, Работа 6 (s-profile-1), «5 events, one file» (s-profile-9; Typecase's
// note 178 p. 1); the agenda's week and the lists draw neither.
func seedMockupWhole(t *testing.T, app *mockupApp) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(mockupRoot(t), filepath.FromSlash(chemodan.file)))
	if err != nil {
		t.Fatal(err)
	}
	for _, rec := range davtest.SplitRecords(string(body)) {
		if strings.Contains(rec, "UID:"+chemodan.uid) {
			rec = chemodanCopy.edit(strings.Replace(rec, "UID:"+chemodan.uid, "UID:"+chemodanCopy.rec.uid, 1))
			app.servers[chemodanCopy.rec.account].Seed(davtest.CalendarHome+chemodanCopy.rec.slug+"/"+chemodanCopy.rec.uid+".ics", rec)
			return
		}
	}
	t.Fatalf("%s is not in %s", chemodan.uid, chemodan.file)
}

// bareCopy is a copy of a record of the set without its attachments and
// its relations: what one copy holds alone, the linked record marks with its
// collection (Р-78).
func bareCopy(rec string) string {
	var out []string
	skip := false
	for _, l := range strings.Split(rec, "\n") {
		switch {
		case strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t"):
			if skip {
				continue
			}
		case strings.HasPrefix(l, "ATTACH") || strings.HasPrefix(l, "RELATED-TO"):
			skip = true
			continue
		default:
			skip = false
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// chooseMockup answers a question of a linked group as the person would on
// its card: the value of the member with this UID (S-16).
func chooseMockup(t *testing.T, app *mockupApp, group, prop, uid string) {
	t.Helper()
	key, _ := app.sess.DataKey()
	if err := app.h.users.EditDuplicates(app.sess.AccountID(), key, "duplicates.choose", "", func(d *sealed.Duplicates) error {
		for i, g := range d.Linked {
			if g.ID != group {
				continue
			}
			for _, m := range g.Members {
				if m.UID == uid {
					if d.Linked[i].Prefs == nil {
						d.Linked[i].Prefs = map[string]sealed.RecordRef{}
					}
					d.Linked[i].Prefs[prop] = m
					return nil
				}
			}
		}
		return fmt.Errorf("no member %s in %s", uid, group)
	}); err != nil {
		t.Fatal(err)
	}
}

// attachMockupRecording links «Заметка со встречи 10.08» to the recording
// of the meeting in the attachment folder, Documents / carrel / attachments
// (s-notes-1, -2, -20: «запись-встречи.m4a · 24 MB · Documents / carrel»;
// the client's scan НГ4). The address of the storage is the stand's, known
// only here: the set's file cannot hold it.
func attachMockupRecording(t *testing.T, app *mockupApp) {
	t.Helper()
	sftp := app.servers["sftpgo · vps"]
	at := davtest.FileRoot + "Documents/carrel/attachments/2026-08/"
	sftp.SeedFile(at+"запись-встречи.m4a", "a stub of the recording")
	// Made after the meeting, not when the stand is loaded: August's
	// folder lists it (s-publish-12; note 180).
	sftp.SetModified(at+"запись-встречи.m4a", mockupDayAt(t, "2026-08-10 11:20"))
	nc := app.servers["nextcloud"]
	p := davtest.CalendarHome + "zapisi/5c81-77aa-0d64.ics"
	body, _, ok := nc.Object(p)
	if !ok {
		t.Fatal("the note is not on the stand")
	}
	link := "ATTACH;FMTTYPE=audio/mp4;SIZE=25165824:" + sftp.URL + davtest.FileRoot + "Documents/carrel/attachments/2026-08/" + url.PathEscape("запись-встречи.m4a")
	nc.Seed(p, strings.Replace(body, "END:VJOURNAL", link+"\r\nEND:VJOURNAL", 1))
}

// mockupRecord is a record of the set: where it lies and in which file of
// the set; a copy has no file.
type mockupRecord struct{ account, slug, uid, file string }

// mockupCopy is a copy of a record of the set: where it lies, and how it
// differs from the record (edit; nil — it holds the same).
type mockupCopy struct {
	rec  mockupRecord
	edit func(string) string
}

// linkMockupCopy puts a copy of a record of the set, changed by edit, under
// another UID into another collection, and links the two as the person
// would (S-16).
func linkMockupCopy(t *testing.T, app *mockupApp, kind view.Kind, group string, of, copy mockupRecord, edit func(string) string) {
	t.Helper()
	linkMockupCopies(t, app, kind, group, of, mockupCopy{rec: copy, edit: edit})
}

// linkMockupCopies puts copies of a record of the set, each changed by its
// edit, under other UIDs into other collections, and links them all with it
// as the person would (S-16).
func linkMockupCopies(t *testing.T, app *mockupApp, kind view.Kind, group string, of mockupRecord, copies ...mockupCopy) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(mockupRoot(t), filepath.FromSlash(of.file)))
	if err != nil {
		t.Fatal(err)
	}
	var orig string
	for _, one := range davtest.SplitRecords(string(body)) {
		if strings.Contains(one, "UID:"+of.uid) {
			orig = one
		}
	}
	if orig == "" {
		t.Fatalf("%s is not in %s", of.uid, of.file)
	}
	all := []mockupRecord{of}
	for _, cp := range copies {
		rec := strings.Replace(orig, "UID:"+of.uid, "UID:"+cp.rec.uid, 1)
		if cp.edit != nil {
			rec = cp.edit(rec)
		}
		at := davtest.CalendarHome + cp.rec.slug + "/" + cp.rec.uid + ".ics"
		if kind == view.KindContact {
			at = davtest.AddressBookHome + cp.rec.slug + "/" + cp.rec.uid + ".vcf"
		}
		app.servers[cp.rec.account].Seed(at, rec)
		all = append(all, cp.rec)
	}
	key, _ := app.sess.DataKey()
	blob, _ := app.h.users.Catalog(app.sess.AccountID(), key)
	var members []sealed.RecordRef
	for _, one := range all {
		for _, c := range blob.Collections {
			conn, _ := blob.Connection(c.AccountID)
			if collectionSlugOf(c.Path) == one.slug && connLabelOf(conn) == one.account {
				members = append(members, sealed.RecordRef{AccountID: c.AccountID, Col: c.Path, UID: one.uid})
			}
		}
	}
	if len(members) != len(all) {
		t.Fatalf("the collections are not in the catalog: %+v", members)
	}
	if err := app.h.users.EditDuplicates(app.sess.AccountID(), key, "duplicates.link", "", func(d *sealed.Duplicates) error {
		d.Linked = append(d.Linked, sealed.LinkedGroup{ID: group, Kind: kind, Members: members})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// lastUsed makes the collection of the set the one a record of kind was
// last made in: New's arrow says «last used» by it (s-tasks-5, s-notes-13).
func (a *mockupApp) lastUsed(t *testing.T, slug string, kind view.Kind) {
	t.Helper()
	key, _ := a.sess.DataKey()
	blob, _ := a.h.users.Catalog(a.sess.AccountID(), key)
	for _, c := range blob.Collections {
		if collectionSlugOf(c.Path) == slug {
			if err := a.h.users.SetLastUsed(a.sess.AccountID(), key, sealed.Selection{Kind: kind, AccountID: c.AccountID, Path: c.Path}); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no collection %s", slug)
}

// C-14.2: one list of tasks — its own frame since Typecase 80d1319
// (s-tasks-15, from-typecase note 36): «Собрать чемодан» linked with its
// copy in «Работа» («linked ×2», «2 sources»), no record open in a panel.
func TestFixtureTaskList(t *testing.T) {
	app := newMockupApp(t)
	linkMockupTask(t, app)
	// The calendar was read: the panel's Related names the trip.
	openList(t, app, "/app/calendar?from=2026-08-10")
	// Both task lists are counted in the frame (s-tasks-1): the combined
	// list was opened first, as a person comes to it.
	page := combined(t, app.h, app.sess, "/app/tasks", func(p view.TasksPage) *view.PollBar { return p.Poll })
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	// «Продлить страховку» just marked done (Р-42, s-tasks-15): the list it
	// comes back to holds it under Open, crossed.
	tickMockupTask(t, app, page, "Продлить страховку")
	list := app.listPath(t, "tasks", "dela")
	app.compare(t, "app-tasks/normal.json", app.h.do(get(list, app.sess)))
}

// C-14.2: the combined list of tasks as s-tasks-1 draws it (the client's
// scan of the tasks, ЗБ1): both lists, «Собрать чемодан» linked with its
// copy and open in the panel, New task writing into «Дела», last used.
func TestFixtureCombinedTasks(t *testing.T) {
	app := newMockupApp(t)
	linkMockupTask(t, app)
	app.lastUsed(t, "dela", view.KindTask)
	openList(t, app, "/app/calendar?from=2026-08-10")
	page := combined(t, app.h, app.sess, "/app/tasks", func(p view.TasksPage) *view.PollBar { return p.Poll })
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	// «Продлить страховку» just marked done from its row (Р-42; s-tasks-1,
	// -20 and Typecase's note 130 p. 1): it stays under Open, crossed, until
	// the list is read anew.
	tickMockupTask(t, app, page, "Продлить страховку")
	open := app.listPath(t, "tasks", "dela") + "/aa11-2d09-5f84"
	app.compare(t, "app-tasks-all/normal.json", app.h.do(get("/app/tasks?task="+page.Poll.TaskID+"&open="+url.QueryEscape(open), app.sess)))
	// The panel of a task of one list (s-tasks-20, Typecase's note 125
	// p. 9): «Отправить документы в МФЦ».
	one := app.listPath(t, "tasks", "dela") + "/aa10-77bc-0e31"
	app.compare(t, "app-tasks-all/panel-one.json", app.h.do(get("/app/tasks?task="+page.Poll.TaskID+"&open="+url.QueryEscape(one), app.sess)))
	// On paper (print-4): the filter Open once the lists are read again —
	// «Продлить страховку», ticked, is under Done now (Р-42).
	again := combined(t, app.h, app.sess, "/app/tasks?again=1", func(p view.TasksPage) *view.PollBar { return p.Poll })
	app.compare(t, "app-tasks-all/print.json", app.h.do(get(onPaper("/app/tasks?task="+again.Poll.TaskID), app.sess)))
}

// tickMockupTask marks a task done from its row of the list, as a person
// ticks it.
func tickMockupTask(t *testing.T, app *mockupApp, page view.TasksPage, summary string) {
	t.Helper()
	for _, g := range page.Groups {
		for _, r := range g.Rows {
			if string(r.Summary) != summary || r.Tick == nil {
				continue
			}
			u, err := url.Parse(r.Tick.Href)
			if err != nil {
				t.Fatal(err)
			}
			if rec := app.h.do(postForm(u.EscapedPath(), app.sess, u.Query())); rec.Code != http.StatusSeeOther {
				t.Fatalf("tick %s: %d %s", summary, rec.Code, rec.Body.String())
			}
			return
		}
	}
	t.Fatalf("no row %s to tick", summary)
}

// C-14.2: one notebook — its own frame since Typecase 80d1319 (s-notes-31,
// from-typecase note 36): «Что почитать про CRDT» linked with its copy in
// «Дневник» («linked ×2», «2 sources»), no record open in a panel.
func TestFixtureNoteList(t *testing.T) {
	app := newMockupApp(t)
	attachMockupRecording(t, app)
	// The calendar and the tasks were read: the panel's Related names
	// «Синк-ап» and «Дописать раздел про кэш».
	openList(t, app, "/app/calendar?from=2026-08-10")
	openList(t, app, "/app/tasks")
	// Both notebooks are ticked and counted in the frame (s-notes-1): the
	// combined notebook was opened first, as a person comes to it.
	combined(t, app.h, app.sess, "/app/notes", func(p view.NotesPage) *view.PollBar { return p.Poll })
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	list := app.listPath(t, "notes", "zapisi")
	app.compare(t, "app-notes/normal.json", app.h.do(get(list, app.sess)))
}

// C-14.2: the combined notebook as s-notes-1 draws it (the client's scan of
// the notes, НБ1): both notebooks, «Что почитать про CRDT» linked with its
// copy, «Заметка со встречи 10.08» open in the panel, New note writing into
// «Записи», last used; the recording in the attachment folder is the
// panel's second attachment (C25).
func TestFixtureCombinedNotes(t *testing.T) {
	app := newMockupApp(t)
	attachMockupRecording(t, app)
	app.lastUsed(t, "zapisi", view.KindNote)
	openList(t, app, "/app/calendar?from=2026-08-10")
	openList(t, app, "/app/tasks")
	page := combined(t, app.h, app.sess, "/app/notes", func(p view.NotesPage) *view.PollBar { return p.Poll })
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	open := app.listPath(t, "notes", "zapisi") + "/5c81-77aa-0d64"
	app.compare(t, "app-notes-all/normal.json", app.h.do(get("/app/notes?task="+page.Poll.TaskID+"&open="+url.QueryEscape(open), app.sess)))
	// The panel of a note of one notebook (s-notes-33, Typecase's note 125
	// p. 9): «Про дорогу до дачи» in «Дневник».
	one := app.listPath(t, "notes", "dnevnik") + "/7f03-5b26-cc19"
	app.compare(t, "app-notes-all/panel-one.json", app.h.do(get("/app/notes?task="+page.Poll.TaskID+"&open="+url.QueryEscape(one), app.sess)))
	// On paper (print-8; Typecase's note 159): the heading, the notebooks
	// that answered, the foot — how many notes, when, how many notebooks.
	paperAt := onPaper("/app/notes?task=" + page.Poll.TaskID)
	paper := decode[view.NotesPage](t, app.h.do(get(paperAt, app.sess))).Print
	if !paper.Paper || paper.Heading != "Notes · Combined notebook" || paper.Sources != "nextcloud · Записи, Дневник" ||
		!strings.HasPrefix(string(paper.Foot), "Carrel · notes · 4 notes · captured ") || !strings.HasSuffix(string(paper.Foot), " · 2 of 2 notebooks") {
		t.Errorf("the notes on paper = %+v", paper)
	}
	app.compare(t, "app-notes-all/print.json", app.h.do(get(paperAt, app.sess)))
}

// C-14.2: the card of a task (s-tasks-2): «Собрать чемодан», the task the
// frame draws — its copy in another list, the event it is related to, the
// file inside it (the client's scan ЗГ5).
func TestFixtureTaskCard(t *testing.T) {
	app := newMockupApp(t)
	linkMockupTask(t, app)
	// The calendar was read: Related names the trip.
	openList(t, app, "/app/calendar?from=2026-08-10")
	// The lists were opened before the record: that is where the counts
	// of the rail come from.
	combined(t, app.h, app.sess, "/app/tasks", func(p view.TasksPage) *view.PollBar { return p.Poll })
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	rec := app.listPath(t, "tasks", "dela") + "/aa11-2d09-5f84"
	app.compare(t, "app-task/normal.json", app.h.do(get(rec, app.sess)))
}

// C-14.2: the sheets of a linked record's copies — «Which copy to edit?»
// over the card of Анна Ковалёва in three books, «Команда» read only
// (s-contacts-32, one sheet for every kind; from-typecase note 117 p. 2) and «Which copy to mark done?» over the combined list of tasks it
// was opened from (s-tasks-16).
func TestFixtureCopiesSheets(t *testing.T) {
	app := newMockupApp(t)
	linkMockupTask(t, app)
	openList(t, app, "/app/calendar?from=2026-08-10")
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	list := combined(t, app.h, app.sess, "/app/tasks", func(p view.TasksPage) *view.PollBar { return p.Poll })
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	anna := app.listPath(t, "contacts", "kontakty") + "/4f2b-9c31-a0e7"
	app.compare(t, "app-record-copies/normal.json", app.h.do(get(anna+"/copies", app.sess)))
	rec := app.listPath(t, "tasks", "dela") + "/aa11-2d09-5f84"
	back := url.QueryEscape("/app/tasks?task=" + list.Poll.TaskID)
	app.compare(t, "app-task-done/normal.json", app.h.do(get(rec+"/done?from=all&back="+back, app.sess)))
}

// C-14.2: one note, read (s-notes-2): its copy in «Дневник», the recording
// in the attachment folder, the event and the task it is related to, read
// before (the client's scan НГ3–НГ5).
func TestFixtureNoteCard(t *testing.T) {
	app := newMockupApp(t)
	attachMockupRecording(t, app)
	openList(t, app, "/app/calendar?from=2026-08-10")
	openList(t, app, "/app/tasks")
	combined(t, app.h, app.sess, "/app/notes", func(p view.NotesPage) *view.PollBar { return p.Poll })
	app.h.do(get(app.listPath(t, "notes", "zapisi"), app.sess))
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	rec := app.listPath(t, "notes", "zapisi") + "/5c81-77aa-0d64"
	// s-notes-2 draws the full width the person chose (Typecase's note 79).
	app.compare(t, "app-note/normal.json", app.h.do(wideNotes(get(rec, app.sess))))
	app.compare(t, "app-note/print.json", app.h.do(get(onPaper(rec), app.sess))) // print-5
	source := rec + "?view=source"
	app.compare(t, "app-note/source.json", app.h.do(wideNotes(get(source, app.sess))))
}

// wideNotes is a request of a browser that chose «Open notes in full width»
// (Look.Wide): the note frames that draw the full width chosen are taken so
// (s-notes-2, -20, -30; Typecase's note 79).
func wideNotes(r *http.Request) *http.Request {
	r.AddCookie(&http.Cookie{Name: "carrel_look", Value: "wide=1"})
	return r
}

// C-14.2: the full form of a contact (s-contacts-3).
func TestFixtureContactEdit(t *testing.T) {
	app := newMockupApp(t)
	// The combined book was opened before the form, as a person comes to
	// it: that is where the counts of both books in the rail come from
	// (s-contacts-3 draws «Архив» with its number).
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	rec := app.listPath(t, "contacts", "kontakty") + "/4f2b-9c31-a0e7/edit"
	// Without a photograph: Add (s-contacts-4, the part «no-photo»).
	app.compare(t, "app-contact-edit/no-photo.json", app.h.do(get(rec, app.sess)))
	// With one: Replace, Crop, Remove (s-contacts-3).
	mockupAnnaPhoto(t, app)
	app.compare(t, "app-contact-edit/normal.json", app.h.do(get(rec, app.sess)))
}

// C-14.2: the form of an event (s-agenda-3).
func TestFixtureEventEdit(t *testing.T) {
	app := newMockupApp(t)
	combined(t, app.h, app.sess, "/app/calendar?from=2026-08-10", func(p view.AgendaPage) *view.PollBar { return p.Poll })
	app.h.do(get(app.listPath(t, "calendar", "lichnyj")+"?from=2026-08-10", app.sess))
	// The address books were read too: an attendee found in one is known
	// (s-agenda-1, -2: the mark of the book and a link; task 77).
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	// The notes were read: Related to names «Заметка со встречи 10.08»
	// with the mark of its notebook (s-agenda-3), as the task's names its
	// event.
	openList(t, app, "/app/notes")
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	rec := app.listPath(t, "calendar", "lichnyj") + "/8f1c-4a02-b7d3/edit"
	app.compare(t, "app-event-edit/normal.json", app.h.do(get(rec, app.sess)))
	// The full form of a new event in «Личный» (C-5.4 /new; the client's
	// scan Д9) — s-agenda-15 (Typecase task 77).
	app.compare(t, "app-event-new/normal.json", app.h.do(get(app.listPath(t, "calendar", "lichnyj")+"/new", app.sess)))
}

// C-14.2: the form of a task (s-tasks-3), and the full form of a new one
// (C-5.4 /new; the client's scan ЗД10 — no frame draws it yet, task 97).
func TestFixtureTaskEdit(t *testing.T) {
	app := newMockupApp(t)
	// The calendar was read: Related to names «Забрать документы».
	openList(t, app, "/app/calendar?from=2026-08-10")
	// The lists were opened before the record: that is where the counts
	// of the rail come from.
	combined(t, app.h, app.sess, "/app/tasks", func(p view.TasksPage) *view.PollBar { return p.Poll })
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	rec := app.listPath(t, "tasks", "dela") + "/aa10-77bc-0e31/edit"
	app.compare(t, "app-task-edit/normal.json", app.h.do(get(rec, app.sess)))
	app.compare(t, "app-task-edit/new.json", app.h.do(get(app.listPath(t, "tasks", "dela")+"/new", app.sess)))
}

// C-14.2: a note in edit (s-notes-20). The notebook was listed first, as a
// person comes to the note: that is where its neighbours in the rail come
// from.
func TestFixtureNoteEdit(t *testing.T) {
	app := newMockupApp(t)
	attachMockupRecording(t, app)
	// The calendar was read: Related to names «Синк-ап по релизу».
	openList(t, app, "/app/calendar?from=2026-08-10")
	// The tasks were read too: the note's second relation is the task
	// «Дописать раздел про кэш» of «Работа», and s-notes-20 draws it by its
	// name (Typecase's note 46).
	combined(t, app.h, app.sess, "/app/tasks", func(p view.TasksPage) *view.PollBar { return p.Poll })
	combined(t, app.h, app.sess, "/app/notes", func(p view.NotesPage) *view.PollBar { return p.Poll })
	app.h.do(get(app.listPath(t, "notes", "zapisi"), app.sess))
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	rec := app.listPath(t, "notes", "zapisi") + "/5c81-77aa-0d64/edit"
	app.compare(t, "app-note-edit/normal.json", app.h.do(wideNotes(get(rec, app.sess))))
	// The full form of a new note in «Записи» (C-5.4 /new; s-notes-30).
	app.compare(t, "app-note-edit/new.json", app.h.do(wideNotes(get(app.listPath(t, "notes", "zapisi")+"/new", app.sess))))
}

// C-14.2: the settings' pages as their frames draw them (appearance-1,
// account-1, attachments-1): the sections read, the folders too, davloom
// with its three devices and the jobs of the backups in the rail's foot.
func TestFixtureSettings(t *testing.T) {
	app := newMockupApp(t)
	settingsState(t, app)
	for _, one := range []struct{ dir, route string }{
		{"app-settings-account", "/app/settings/account"},
		{"app-settings-appearance", "/app/settings/appearance"},
		{"app-settings-attachments", "/app/settings/attachments"},
	} {
		t.Run(one.dir, func(t *testing.T) {
			app.compare(t, one.dir+"/normal.json", app.h.do(get(one.route, app.sess)))
		})
	}
	// Typecase's note 93: the folder of the attachments refused by its
	// server (attachments-6), and none chosen yet (attachments-5).
	sftp := app.servers["sftpgo · vps"]
	sftp.RefuseAt("PROPFIND", app.realPath(t, "Documents")+"carrel/attachments/", http.StatusUnauthorized, "")
	app.compare(t, "app-settings-attachments/unavailable.json", app.h.do(get("/app/settings/attachments", app.sess)))
	editBlob(t, app.h, app.sess, func(b *sealed.Blob) { b.Attachments = nil })
	app.compare(t, "app-settings-attachments/unset.json", app.h.do(get("/app/settings/attachments", app.sess)))
}

// settingsState is the instance the settings' frames stand on: the
// sections and folders read, the screen taken two minutes after; davloom's
// three devices, their last answer the morning of the frames; the backups'
// jobs and runs (s-publish-1, appearance-1).
func settingsState(t *testing.T, app *mockupApp) (work, home string) {
	t.Helper()
	settingsDevices(t, app)
	work, home = mockupBackups(t, app)
	// The calendars of nextcloud and radicale · vps declare their ceilings
	// (attachments-1: 2 MB and 1 MB).
	editBlob(t, app.h, app.sess, func(b *sealed.Blob) {
		ceiling := map[string]int64{}
		for _, c := range b.Connections {
			switch connLabelOf(c) {
			case "nextcloud":
				ceiling[c.ID] = 2 << 20
			case "radicale · vps":
				ceiling[c.ID] = 1 << 20
			}
		}
		for i := range b.Collections {
			if !b.Collections[i].Has(view.KindFile) {
				b.Collections[i].MaxResourceSize = ceiling[b.Collections[i].AccountID]
			}
		}
		// Attachments go to a subfolder per month (attachments-1).
		if b.Attachments != nil {
			b.Attachments.Monthly = true
		}
	})
	// Two sessions, this one marked (account-1): each the browser it came
	// from.
	app.sess.SetClient("Firefox · Windows", "203.0.113.7")
	app.h.login("mix", "password123").SetClient("Edge · Windows", "203.0.113.8")
	return work, home
}

// settingsDevices is the instance of the settings' frames before any job:
// the sections open, the folders read, three davloom devices last heard
// from at 09:41 (the rail's foot of s-backup-6).
func settingsDevices(t *testing.T, app *mockupApp) {
	t.Helper()
	openSections(t, app)
	openUnticked(t, app)
	readFolders(t, app)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	for _, id := range mockupDevices(t, app) {
		app.h.users.TouchDevice(id, mockupDayAt(t, "2026-08-16 09:41"))
	}
}

// C-14.2: the screens of the settings and of the administration. They hold
// no records of the set, so each is one call.
func TestFixtureSettingsAndAdmin(t *testing.T) {
	app := newMockupApp(t)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	for _, one := range []struct{ dir, route string }{
		// Account, Appearance, Attachments over the instance of their
		// frames: TestFixtureSettings.
		// Connections after the lists were read: TestFixtureConnections; a
		// new device: TestFixtureDevicesStates.
		// davloom with its devices: TestFixtureDavloomStates.
		// The escrow page's normal is the deposited copy (account-4):
		// TestFixtureAccountStates deposits it.
	} {
		t.Run(one.dir, func(t *testing.T) {
			app.compare(t, one.dir+"/normal.json", app.h.do(get(one.route, app.sess)))
		})
	}
	adminState(t, app)
	// The rail's foot of each frame: whether mail goes out, the last check
	// (Typecase's 195; adminRail).
	for _, one := range []struct {
		dir, route string
		mail       bool
	}{
		{"admin-users", "/admin/", false},
		// The link just made stands in its card (s-admin-1): the page Create
		// link answers with.
		{"admin-invites", "/admin/invites?invite=i-0000000001", true},
		{"admin-settings", "/admin/settings", false},
		// Read from the day its frame chose (s-admin-7: since 2026-08-01).
		{"admin-audit", "/admin/audit?since=2026-08-01", false},
		{"admin-dav", "/admin/dav", false},
		{"admin-escrow", "/admin/escrow", true},
	} {
		t.Run(one.dir, func(t *testing.T) {
			adminRail(t, app, one.mail, checksPassed)
			if one.dir == "admin-audit" {
				auditOfFrame(t, app)
			}
			app.compare(t, one.dir+"/normal.json", app.h.do(get(one.route, app.sess)))
		})
	}
	// Test discovery against a stand (Ад33; s-admin-5): the trace step by
	// step, with what the server answered at each.
	t.Run("admin-dav-report", func(t *testing.T) {
		adminRail(t, app, false, checksPassed)
		app.compare(t, "admin-dav/report.json", app.h.do(postForm("/admin/dav", app.sess, url.Values{
			"server": {app.servers["nextcloud"].URL}, "login": {"alice"}, "password": {"secret"},
		})))
	})
	// Discovery that stopped (s-admin-28): a password the stand refuses —
	// the step named, the trace up to it, no second stage.
	t.Run("admin-dav-broken", func(t *testing.T) {
		adminRail(t, app, false, checksPassed)
		app.compare(t, "admin-dav/broken.json", app.h.do(postForm("/admin/dav?op=test", app.sess, url.Values{
			"server": {app.servers["nextcloud"].URL}, "login": {"alice"}, "password": {"wrong-one"},
		})))
	})
	// The second stage agreed to (s-admin-29): one row a kind, what was
	// asked and what the stand answered; nothing is left on it.
	t.Run("admin-dav-exercised", func(t *testing.T) {
		adminRail(t, app, false, checksPassed)
		// The address book takes no writes from this account (s-admin-29:
		// the contact refused, 403).
		nc := app.servers["nextcloud"]
		nc.RefuseAt("PUT", davtest.AddressBookHome, http.StatusForbidden, "")
		defer nc.Accept("PUT " + davtest.AddressBookHome)
		app.compare(t, "admin-dav/exercised.json", app.h.do(postForm("/admin/dav?op=exercise", app.sess, url.Values{
			"server": {nc.URL}, "login": {"alice"}, "password": {"secret"}, "agree": {"on"},
		})))
	})
	// The relay's test (note 82 of Typecase): with the relay set, the letter
	// gone and the relay's answer under the test (s-admin-32); a port out of
	// range, refused at its field and under the test (s-admin-33). The relay
	// listens on a port the machine had free: the frames write 587.
	t.Run("admin-settings-test", func(t *testing.T) {
		adminRail(t, app, true, checksPassed)
		relay := app.h.relayed()
		defer func() {
			_ = app.h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
				st.Settings.SMTP = store.SMTPSettings{}
				return nil, nil
			})
		}()
		steady := func(rec *httptest.ResponseRecorder) *httptest.ResponseRecorder {
			out := httptest.NewRecorder()
			out.Code = rec.Code
			out.Body.Write(bytes.ReplaceAll(rec.Body.Bytes(), []byte(`"`+strconv.Itoa(relay.Port)+`"`), []byte(`"587"`)))
			return out
		}
		// Letters from users on as saved (s-admin-32): the sender and the
		// ceilings shown; off (s-admin-33), not at all (Typecase's note 88).
		people := func(on bool) {
			if err := app.h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
				st.Settings.PeopleMail = &on
				return nil, nil
			}); err != nil {
				t.Fatal(err)
			}
		}
		people(true)
		form := settingsForm(t, app.h, app.sess, map[string]string{"to": "mix@carrel.example"})
		app.compare(t, "admin-settings/sent.json", steady(app.h.do(postForm("/admin/settings/mail-test", app.sess, form))))
		people(false)
		form = settingsForm(t, app.h, app.sess, map[string]string{"to": "mix@carrel.example"})
		form.Set("port", "70000")
		app.compare(t, "admin-settings/refused.json", steady(app.h.do(postForm("/admin/settings/mail-test", app.sess, form))))
	})
	// Deleting an account (s-admin-34, note 83): the sheet over Users, anna
	// chosen by the item of her row.
	t.Run("admin-delete", func(t *testing.T) {
		adminRail(t, app, false, checksPassed)
		snap := app.h.st.Snapshot()
		app.compare(t, "admin-delete/normal.json", app.h.do(get("/admin/delete?target="+snap.UserByLogin("anna").ID, app.sess)))
	})
	// Key escrow (note 81 of Typecase): viktor's account recovered with the
	// master password — the temporary password once, and whether a letter
	// went (s-admin-30) —; his password's history by its link (s-auth-15);
	// the scheme off (s-admin-31).
	t.Run("admin-escrow-states", func(t *testing.T) {
		adminRail(t, app, true, checksPassed)
		// The history as the frame tells it, before this recovery adds to it.
		snap := app.h.st.Snapshot()
		viktor := snap.UserByLogin("viktor")
		app.compare(t, "password-history/normal.json", app.h.do(anon("/password-history/"+viktor.PasswordLog)))
		app.compare(t, "admin-escrow/recovered.json", app.h.do(postForm("/admin/escrow", app.sess, url.Values{
			"action": {"recover"}, "login": {"viktor"}, "master": {"sixteen-char-pass!"}, "temp": {"temp-pass-4821"},
		})))
		escrow := func(on bool) {
			if err := app.h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
				st.Settings.EscrowEnabled = on
				return nil, nil
			}); err != nil {
				t.Fatal(err)
			}
		}
		escrow(false)
		defer escrow(true)
		app.compare(t, "admin-escrow/off.json", app.h.do(get("/admin/escrow", app.sess)))
	})
	// The sheet that resets a password (Ад8): anna chosen (s-admin-13), the
	// password issued (s-admin-14), a password too short (s-admin-16) and
	// a login typed that is not hers (s-admin-20).
	snap := app.h.st.Snapshot()
	anna := snap.UserByLogin("anna").ID
	reset := func(password, confirm string) *httptest.ResponseRecorder {
		return app.h.do(postForm("/admin/reset", app.sess, url.Values{"target": {anna}, "password": {password}, "confirm": {confirm}}))
	}
	t.Run("admin-reset", func(t *testing.T) {
		app.compare(t, "admin-reset/normal.json", app.h.do(get("/admin/reset?target="+anna, app.sess)))
		app.compare(t, "admin-reset/refused.json", reset("short", "anna"))
		app.compare(t, "admin-reset/mismatch.json", reset("long-enough-temp", "ana"))
		app.compare(t, "admin-reset/issued.json", reset("long-enough-temp", "anna"))
	})
}

// adminState is the instance the administration's frames draw (Ад11, Ад15,
// Ад23, Ад36 of the scan of 15.10.2026): beside mix, anna, whose key is in
// escrow, and viktor, disabled (s-admin-2, -6); an invitation by link and
// one by letter the relay refused, and the registration open (s-admin-1);
// no relay, and people's letters off (s-admin-3). The settings' own screens
// were taken before it: they draw the relay the app's letters go through.
func adminState(t *testing.T, app *mockupApp) {
	t.Helper()
	h := app.h
	snap := h.st.Snapshot()
	mixID := snap.UserByLogin("mix").ID
	// Invited after escrow was turned on: their accounts are covered.
	h.invite(mixID, "anna", "annapass123")
	viktor := h.invite(mixID, "viktor", "viktorpass123")
	if err := h.users.DisableUser(mixID, viktor); err != nil {
		t.Fatalf("disable viktor: %v", err)
	}
	// The six of s-admin-2 (Typecase's note 84): olga an administrator with
	// her key in escrow, petr with his, ivan without a copy.
	olga := h.invite(mixID, "olga", "olgapass1234")
	if err := h.users.SetUserRole(mixID, olga, store.RoleAdmin); err != nil {
		t.Fatalf("olga an administrator: %v", err)
	}
	h.invite(mixID, "petr", "petrpass1234")
	ivan := h.invite(mixID, "ivan", "ivanpass1234")
	if err := h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(ivan)
		u.EscrowDeposited, u.EscrowDepositedAt = nil, nil
		// viktor's account was recovered once (s-admin-6), and his password
		// has the frame's history (s-auth-15): changed, opened with the
		// master password, changed again.
		at := mockupDayAt(t, "2026-08-14 18:40")
		v := st.UserByID(viktor)
		v.EscrowLastRecovery = &at
		store.NotePassword(v, mockupDayAt(t, "2026-05-03 10:15"), "password.change", "viktor")
		store.NotePassword(v, at, "escrow.recover", "mix")
		store.NotePassword(v, mockupDayAt(t, "2026-08-14 19:02"), "password.change", "viktor")
		return nil, nil
	}); err != nil {
		t.Fatalf("ivan without a copy: %v", err)
	}
	if _, err := h.users.Invite(mixID, "", store.RoleUser); err != nil {
		t.Fatalf("invite by link: %v", err)
	}
	if _, err := h.users.Invite(mixID, "oleg@example.org", store.RoleUser); err != nil {
		t.Fatalf("invite by letter: %v", err)
	}
	if err := h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		// Their IDs fixed: the rows' links name them, and a fixture is the
		// same each run.
		for i := range st.Invites {
			switch {
			case st.Invites[i].Email == "oleg@example.org":
				st.Invites[i].ID = "i-0000000002"
				st.Invites[i].Mailed, st.Invites[i].MailReply = "failed", "550 mailbox unavailable"
			case st.Invites[i].Login == "" && !st.Invites[i].Used:
				st.Invites[i].ID = "i-0000000001"
			}
		}
		st.Settings.SelfRegEnabled = true
		st.Settings.SMTP = store.SMTPSettings{}
		people := false
		st.Settings.PeopleMail = &people
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
}

// C-14.2: Connections as s-profile-1 draws it — after the lists were read
// two minutes ago: each card says when its server last answered, a
// collection its count, and «baikal», whose saved password stopped fitting,
// that its last answer was a refusal (Нс23).
func TestFixtureConnections(t *testing.T) {
	app := newMockupApp(t)
	seedMockupWhole(t, app)
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	// Over the instance of the settings' frames: the rail's foot of each
	// (s-profile-1).
	settingsState(t, app)
	// baikal refuses and was tested: its card carries the trace of what was
	// asked (s-profile-1).
	if rec := app.h.do(postForm("/app/settings/connections?"+url.Values{"op": {"test"}, "account": {app.realAccount(t, "obshchij")}}.Encode(), app.sess, url.Values{})); rec.Code >= 400 {
		t.Fatalf("test baikal: %d", rec.Code)
	}
	// The league's feed checked: it refuses (s-profile-1).
	league := ""
	for real, fx := range app.byAccount {
		if fx == "f-league" {
			league = real
		}
	}
	if rec := app.h.do(postForm("/app/settings/connections?"+url.Values{"op": {"feed.recheck"}, "feed": {league}}.Encode(), app.sess, url.Values{})); rec.Code >= 400 {
		t.Fatalf("check the league: %d", rec.Code)
	}
	app.compare(t, "app-settings-connections/normal.json", app.h.do(get("/app/settings/connections", app.sess)))
	// Edit of nextcloud, through which «Личный» is published (Нс76): the
	// question about the links waits hidden for a change of Server URL or
	// Username (s-profile-16, -22); Save with another login and no answer
	// brings the dialog back with the reason (s-profile-23).
	edit := "/app/settings/connections/" + url.PathEscape(app.realAccount(t, "lichnyj")) + "/edit"
	t.Run("edit", func(t *testing.T) {
		app.compare(t, "app-settings-connections/edit.json", app.h.do(get(edit, app.sess)))
	})
	// Нс30: Connect account (s-profile-2); its label taken — «nextcloud»
	// is (s-profile-11); the panel of a subscription (s-profile-10).
	t.Run("add", func(t *testing.T) {
		app.compare(t, "app-settings-connections/add.json", app.h.do(get("/app/settings/connections/new", app.sess)))
	})
	t.Run("label-taken", func(t *testing.T) {
		app.compare(t, "app-settings-connections/label-taken.json", app.h.do(postForm("/app/settings/connections/new", app.sess, url.Values{
			"label": {"nextcloud"}, "server": {app.servers["nextcloud"].URL}, "login": {"alice"}, "password": {"secret"}})))
	})
	// Typecase's note 92: the server refused the login and password — the
	// dialog with what was typed, the refusal and the trace over the
	// fields, the password empty (s-profile-24).
	t.Run("refused", func(t *testing.T) {
		app.compare(t, "app-settings-connections/refused.json", app.h.do(postForm("/app/settings/connections/new", app.sess, url.Values{
			"server": {app.servers["nextcloud"].URL}, "login": {"alice"}, "password": {"not-the-password"}})))
	})
	// The sheet of Disconnect over the connections (s-profile-26): what
	// Carrel removes, in numbers.
	t.Run("disconnect", func(t *testing.T) {
		q := url.Values{"account": {app.realAccount(t, "lichnyj")}}
		app.compare(t, "app-settings-connections-disconnect/normal.json", app.h.do(get("/app/settings/connections/disconnect?"+q.Encode(), app.sess)))
	})
	// The panel after Check the address (s-profile-10): what the feed
	// carries, its name, the trace of the check.
	t.Run("feed", func(t *testing.T) {
		holidays := app.feeds["https://feeds.example/ru-holidays.ics"]
		app.compare(t, "app-settings-connections/feed.json", app.h.do(postForm("/app/settings/connections?op=feed.check", app.sess,
			url.Values{"url": {holidays.URL + "/ru-holidays.ics"}, "asks": {""}})))
	})
	t.Run("edit-links", func(t *testing.T) {
		page := decode[view.ConnectionsPage](t, app.h.do(get(edit, app.sess)))
		form := formFields(*page.Add)
		form.Set("login", "mix")
		app.compare(t, "app-settings-connections/edit-links.json", app.h.do(postForm(edit, app.sess, form)))
	})
	// Typecase's note 103: nextcloud's password shown on the whole page
	// (s-profile-4) — the window open, its badge and Lock now in the head,
	// Hide password on the card, the password under the address with Copy.
	t.Run("shown", func(t *testing.T) {
		app.sess.SetUnlockUntil(mockupNow(t).Add(mockupSince).Add(4*time.Minute + 12*time.Second))
		defer app.sess.SetUnlockUntil(time.Time{})
		app.compare(t, "app-settings-connections/shown.json", app.h.do(get("/app/settings/connections?"+url.Values{
			"reveal": {app.realAccount(t, "lichnyj")}}.Encode(), app.sess)))
	})
	// Typecase's note 103: Update collection list on nextcloud after
	// «Поездки» appeared there (s-profile-27) — the line above the cards,
	// the new row unticked and marked new.
	t.Run("found", func(t *testing.T) {
		nc := app.servers["nextcloud"]
		trips := strings.Replace(app.realPath(t, "lichnyj"), "lichnyj", "poezdki", 1)
		nc.AddCalendar(caldav.Calendar{Path: trips, Name: "Поездки", SupportedComponentSet: []string{"VEVENT"}})
		nc.SetColour(trips, "#6B4E7A")
		app.byPath[url.PathEscape(trips)] = "poezdki"
		app.compare(t, "app-settings-connections/found.json", app.h.do(postForm("/app/settings/connections?"+url.Values{
			"op": {"rediscover"}, "account": {app.realAccount(t, "lichnyj")}}.Encode(), app.sess, url.Values{})))
	})
}

// Typecase's note 92: nothing connected yet (s-profile-25) — the head with
// Connect account and Subscribe to a feed, the rail without counts and
// without the states of davloom and the copies.
func TestFixtureConnectionsEmpty(t *testing.T) {
	app := newMockupApp(t)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	editBlob(t, app.h, app.sess, func(b *sealed.Blob) {
		b.Connections, b.Collections, b.Subscriptions = nil, nil, nil
	})
	// Nothing connected, nothing published from it either.
	if err := app.h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		st.Published = nil
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	app.compare(t, "app-settings-connections/empty.json", app.h.do(get("/app/settings/connections", app.sess)))
}

// C-14.3: the states of the account, of its key escrow and of the password
// change beside their normal ones (Нс9, Нс11, Нс13, Нс16, Нс17 of the scan
// of 15.10.2026). Each is a state the frames draw — a fragment of the Email
// or Key escrow block, the escrow page, a sheet over Account — and each is
// written straight into the record, as the account of the frames is: the
// journal of the frames (admin-audit) has none of it. One instance, in
// order: the states of the address first, then the escrow switched off and
// on, the deposit, and last the temporary password, after which the
// instance lets mix nowhere else.
func TestFixtureAccountStates(t *testing.T) {
	app := newMockupApp(t)
	// The page whole in s-mail-9: over the instance of the settings' frames.
	settingsState(t, app)
	mutate := func(t *testing.T, change func(st *store.State, u *store.User)) {
		t.Helper()
		if err := app.h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
			change(st, st.UserByLogin("mix"))
			return nil, nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	// letter is a link to confirm that went to the address at mailed, and
	// lives for two days.
	letter := func(st *store.State, u *store.User, to, mailedAt string) {
		mailed := mockupDayAt(t, mailedAt)
		st.Tokens = append(st.Tokens, store.SecretToken{
			Kind: "email_confirm", Hash: make([]byte, 32), UserID: u.ID, Email: to,
			CreatedAt: mailed, ExpiresAt: mailed.Add(48 * time.Hour), MailedAt: &mailed,
		})
	}
	noLetters := func(st *store.State) {
		var kept []store.SecretToken
		for _, tok := range st.Tokens {
			if tok.Kind != "email_confirm" {
				kept = append(kept, tok)
			}
		}
		st.Tokens = kept
	}
	account := func(t *testing.T, state string) {
		t.Helper()
		app.compare(t, "app-settings-account/"+state+".json", app.h.do(get("/app/settings/account", app.sess)))
	}

	// account-8: the address confirmed on 14 February, nothing waits.
	t.Run("email-confirmed", func(t *testing.T) {
		mutate(t, func(st *store.State, u *store.User) { u.PendingEmail = ""; noLetters(st) })
		account(t, "email-confirmed")
	})
	// account-10: a new address waits, and no link went to it.
	t.Run("email-unsent", func(t *testing.T) {
		mutate(t, func(st *store.State, u *store.User) { u.PendingEmail = "new@example.org"; noLetters(st) })
		account(t, "email-unsent")
	})
	// s-mail-9: the address was never confirmed, and nothing went to it.
	t.Run("email-unconfirmed", func(t *testing.T) {
		mutate(t, func(st *store.State, u *store.User) {
			u.PendingEmail, u.EmailConfirmed, u.EmailConfirmedAt = "", false, nil
			noLetters(st)
		})
		account(t, "email-unconfirmed")
	})
	// account-9: the link went to it the day before, and lives.
	t.Run("email-sent", func(t *testing.T) {
		mutate(t, func(st *store.State, u *store.User) { letter(st, u, u.Email, "2026-08-09 11:42") })
		account(t, "email-sent")
	})
	// The address of the frames again: confirmed, the new one waiting for
	// its letter of that morning (account-1, newMockupApp).
	confirmed := mockupDayAt(t, "2026-02-14 11:42")
	mutate(t, func(st *store.State, u *store.User) {
		noLetters(st)
		u.EmailConfirmed, u.EmailConfirmedAt, u.PendingEmail = true, &confirmed, "new@example.org"
		letter(st, u, u.PendingEmail, "2026-08-10 11:42")
	})

	// account-13 and account-14: the administrator switched the scheme
	// off — the block says so alone, the page says nothing else.
	t.Run("escrow-off", func(t *testing.T) {
		mutate(t, func(st *store.State, _ *store.User) { st.Settings.EscrowEnabled = false })
		defer mutate(t, func(st *store.State, _ *store.User) { st.Settings.EscrowEnabled = true })
		account(t, "escrow-off")
		app.compare(t, "app-settings-escrow/escrow-off.json", app.h.do(get("/app/settings/escrow", app.sess)))
	})
	// account-12: the page without a copy, and account-2: the deposit
	// sheet over Account.
	t.Run("escrow-no-copy", func(t *testing.T) {
		app.compare(t, "app-settings-escrow/no-copy.json", app.h.do(get("/app/settings/escrow", app.sess)))
		app.compare(t, "app-settings-escrow-deposit/normal.json", app.h.do(get("/app/settings/escrow/deposit", app.sess)))
	})
	// account-4, account-7, account-11: the copy deposited on 2 March.
	// Deposited by the service, not by the sheet: the sheet opens the
	// unlock window, and its time left (TopBar.UnlockLeft) is measured by
	// the real clock.
	t.Run("escrow-deposited", func(t *testing.T) {
		key, ok := app.sess.DataKey()
		if !ok {
			t.Fatal("no data key in the session")
		}
		app.h.users.SetClock(mockupDayAt(t, "2026-03-02 10:00"))
		if err := app.h.users.DepositEscrow(app.sess.AccountID(), key); err != nil {
			t.Fatal(err)
		}
		app.h.users.SetClock(mockupNow(t))
		app.compare(t, "app-settings-escrow/normal.json", app.h.do(get("/app/settings/escrow", app.sess)))
		account(t, "escrow-deposited")
		app.compare(t, "app-settings-escrow-withdraw/normal.json", app.h.do(get("/app/settings/escrow/withdraw", app.sess)))
		// account-16 (Typecase's note 99): the account recovered once by
		// olga, a line under Deposited.
		at := mockupDayAt(t, "2026-07-11 14:02")
		mutate(t, func(_ *store.State, u *store.User) {
			u.EscrowLastRecovery = &at
			store.NotePassword(u, at, "escrow.recover", "olga")
		})
		app.compare(t, "app-settings-escrow/history.json", app.h.do(get("/app/settings/escrow", app.sess)))
	})
	// account-3, the ordinary case: Back to settings, the settings' rail.
	t.Run("password", func(t *testing.T) {
		app.compare(t, "app-password/normal.json", app.h.do(get("/app/password", app.sess)))
	})
	// account-3, the forced case: signed in with a temporary password, the
	// red line on top and no way back.
	t.Run("password-forced", func(t *testing.T) {
		mutate(t, func(_ *store.State, u *store.User) { u.MustChangePassword = true })
		app.compare(t, "app-password/forced.json", app.h.do(get("/app/password", app.sess)))
	})
}

// C-14.2: the screens outside the shell. A person who has not signed in
// sees them, so they are taken without a session.
func TestFixtureOutsideTheShell(t *testing.T) {
	app := newMockupApp(t)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	// The instance of the frames lets people register (s-auth-1, -13 draw
	// «Create an account»).
	openRegistration(t, app.h)
	for _, one := range []struct{ dir, route string }{
		{"login", "/login"},
		{"forgot", "/forgot"},
		{"about", "/about"},
	} {
		t.Run(one.dir, func(t *testing.T) {
			app.compare(t, one.dir+"/normal.json", app.h.do(get(one.route, nil)))
		})
	}
	// refused: the sign-in with a wrong password (s-auth-13) — the login
	// back in its field, the password empty and in focus, the reason at it.
	t.Run("login-refused", func(t *testing.T) {
		app.compare(t, "login/refused.json", app.h.do(app.h.anonPostForm("/login", "/login", url.Values{"login": {"mix"}, "password": {"wrong-one"}})))
	})
	t.Run("admin-install", func(t *testing.T) {
		adminRail(t, app, true, checksNever)
		app.compare(t, "admin-install/normal.json", app.h.do(get("/admin/install", app.sess)))
	})
	// An address that is none (s-admin-26): the field in error, why under
	// the head, no table.
	t.Run("admin-install-refused", func(t *testing.T) {
		adminRail(t, app, true, checksFailed)
		app.compare(t, "admin-install/refused.json", app.h.do(postForm("/admin/install", app.sess, url.Values{"url": {"carrel.example"}})))
	})
	// The check run (Ад31; s-admin-4): Carrel asks itself at a listener of
	// the stand, bare http with no proxy — the certificate and the client
	// address broken, the timeout unknown, as the frame counts them. Last:
	// the run keeps the address it was given.
	t.Run("admin-install-report", func(t *testing.T) {
		// Behind the frame's proxy (s-admin-4): https with its certificate,
		// the client's address passed on and trusted, the event stream
		// buffered and a body over 20 MB cut.
		app.h.cfg.TrustedProxies = "127.0.0.1"
		app.h.rebuild()
		app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
		adminRail(t, app, true, checksNever)
		live := httptest.NewServer(app.h.srv.Handler())
		defer live.Close()
		proxy := frameProxy(t, live.URL)
		defer proxy.Close()
		app.h.srv.SetProbeTransport(proxy.Client().Transport)
		rec := app.h.do(postForm("/admin/install", app.sess, url.Values{"url": {proxy.URL}}))
		steadyInstall(rec, proxy.URL, app.h.cfg.DataDir)
		app.compare(t, "admin-install/report.json", rec)
	})
}

// frameProxy is the proxy s-admin-4 checks: https in front of live, the
// scheme and the client's address passed on, and two faults — the event
// stream held back past the check's two seconds, a body over 20 MB answered
// 413.
func frameProxy(t *testing.T, live string) *httptest.Server {
	t.Helper()
	to, err := url.Parse(live)
	if err != nil {
		t.Fatal(err)
	}
	rp := httputil.NewSingleHostReverseProxy(to)
	inner := rp.Director
	rp.Director = func(r *http.Request) {
		inner(r)
		r.Header.Set("X-Forwarded-Proto", "https")
	}
	proxy := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.ContentLength > 20<<20:
			_, _ = io.Copy(io.Discard, r.Body)
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		case strings.HasPrefix(r.URL.Path, "/app/search/progress/"):
			time.Sleep(2100 * time.Millisecond)
		}
		rp.ServeHTTP(w, r)
	}))
	proxy.StartTLS()
	return proxy
}

// steadyInstall gives the run of the installation check the frame's values
// where they are the machine's: the port the listener had free, how many
// milliseconds an answer took, the clocks' skew, the data folder and the
// room on its disk. What each row says stays the server's.
func steadyInstall(rec *httptest.ResponseRecorder, live, dataDir string) {
	s := rec.Body.String()
	s = strings.ReplaceAll(s, strings.TrimPrefix(strings.TrimPrefix(live, "https://"), "http://"), "carrel.example")
	// The test's certificate as the frame's issuer and its day (s-admin-4).
	s = regexp.MustCompile(`"[^"·]* · until \d{4}-\d\d-\d\d · chain`).ReplaceAllString(s, `"Let’s Encrypt R11 · until 2026-11-02 · chain`)
	s = regexp.MustCompile(`after [0-9.]+ s — the response`).ReplaceAllString(s, "after 30.0 s — the response")
	dir, _ := json.Marshal(dataDir)
	s = strings.ReplaceAll(s, strings.Trim(string(dir), `"`), "/var/lib/carrel")
	s = regexp.MustCompile(`· \d+ ms`).ReplaceAllString(s, "· 41 ms")
	s = regexp.MustCompile(`after \d+ ms`).ReplaceAllString(s, "after 2 ms")
	s = regexp.MustCompile(`"took": ?"[^"]*"`).ReplaceAllString(s, `"took":"4.2 s"`)
	s = regexp.MustCompile(`[0-9.]+[µm]?s between the proxy`).ReplaceAllString(s, "0s between the proxy")
	s = regexp.MustCompile(` · writable · [^"]* free`).ReplaceAllString(s, " · writable · 41 GB free")
	rec.Body.Reset()
	rec.Body.WriteString(s)
}

// C-14.2: the timeline of Анна Ковалёва (s-contacts-7). The timeline is a
// search over one person: it is gathered when the screen is opened, and the
// screen is taken once it has answered. The file with her name in its own
// («ковалёва-обложка-v2.pdf», 6 Aug, sftpgo · vps · Documents) and the note
// related to her card («Про сроки по сборнику», 21 Jul, nextcloud · Записи)
// are the set's (Typecase's notes 178 p. 3, 182 p. 5).
func TestFixtureContactTimeline(t *testing.T) {
	app := newMockupApp(t)
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	anna := app.listPath(t, "contacts", "kontakty") + "/4f2b-9c31-a0e7"
	first := decode[view.ContactTimelinePage](t, app.h.do(get(anna+"/timeline", app.sess)))
	if first.Poll.TaskID == "" {
		t.Fatal("the timeline gathered nothing")
	}
	waitPoll(t, app.h, app.sess, first.Poll.TaskID, nil)
	app.compare(t, "app-contact-timeline/normal.json", app.h.do(get(anna+"/timeline", app.sess)))
}

// C-14.2: the screens of one record that are neither its card nor its form,
// and the screens the shell opens over a list.
func TestFixtureMoreOfTheShell(t *testing.T) {
	app := newMockupApp(t)
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	book := app.listPath(t, "contacts", "kontakty")
	anna := book + "/4f2b-9c31-a0e7"
	// The combined notebook was read: the rails of the sheets after count
	// its notes (the quick sheets have a set of their own,
	// TestFixtureQuickSheets).
	combined(t, app.h, app.sess, "/app/notes", func(p view.NotesPage) *view.PollBar { return p.Poll })
	for _, one := range []struct{ dir, route string }{
		{"app-contact-new", book + "/new"},
		{"app-contact-import", book + "/import"},
		// Generate profile over Set up a new device (s-plan-3).
		{"app-settings-unlock", "/app/settings/unlock?for=profile&back=%2Fapp%2Fsettings%2Fdevices"},
	} {
		t.Run(one.dir, func(t *testing.T) {
			app.compare(t, one.dir+"/normal.json", app.h.do(get(one.route, app.sess)))
		})
	}
	// The password's sheet over Add device as typed (s-davloom-5) and before
	// the account is deleted (account-15): Typecase's note 86.
	t.Run("app-settings-unlock-davloom", func(t *testing.T) {
		form := url.Values{"op": {"issue"}, "name": {"Телефон · DAVx5"}, "access": {"write"}, "kind.event": {"on"}, "kind.contact": {"on"}}
		rec := app.h.do(postForm("/app/settings/davloom", app.sess, form))
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("Create password: %d", rec.Code)
		}
		app.compare(t, "app-settings-unlock/davloom.json", app.h.do(get(rec.Header().Get("Location"), app.sess)))
	})
	t.Run("app-settings-unlock-delete", func(t *testing.T) {
		app.compare(t, "app-settings-unlock/delete.json", app.h.do(get("/app/settings/unlock?for=delete&back=%2Fapp%2Fsettings%2Faccount", app.sess)))
	})
	// Move over the combined book it was opened from, as s-contacts-2 draws
	// it: the book as its poll answered.
	t.Run("app-contact-move", func(t *testing.T) {
		book := combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
		back := url.QueryEscape("/app/contacts?task=" + book.Poll.TaskID)
		app.compare(t, "app-contact-move/normal.json", app.h.do(get(anna+"/move?back="+back, app.sess)))
	})
	// New calendar from the rail of the calendar (s-profile-5): the calendar
	// has been read, «baikal» refused it, and the sheet leaves it out with
	// the hint under «On which server».
	t.Run("app-collections-new", func(t *testing.T) {
		combined(t, app.h, app.sess, "/app/calendar?from=2026-08-10", func(p view.AgendaPage) *view.PollBar { return p.Poll })
		// «Поездки» typed, as the frame draws it: the address shows what the
		// name gives (s-profile-5) — the form as typed, «Back to the form».
		app.compare(t, "app-collections-new/normal.json", app.h.do(postForm("/app/collections/new?kind=event&op=back", app.sess, url.Values{
			"account": {app.realAccount(t, "lichnyj")}, "name": {"Поездки"}, "colour": {"#6b4e7a"}, "components": {"VEVENT", "VTODO"},
		})))
		// Кл7: a new folder on the storage (s-profile-20) — its name is
		// its address, nothing else to ask —, and the server refusing to
		// make a calendar (s-profile-6): the sheet stays, the form as left.
		app.compare(t, "app-collections-new/folder.json", app.h.do(get("/app/collections/new?kind=file&a="+app.realAccount(t, "Documents"), app.sess)))
		app.servers["nextcloud"].Refuse("MKCALENDAR", http.StatusForbidden, "<D:error xmlns:D=\"DAV:\"><D:need-privileges/></D:error>")
		app.compare(t, "app-collections-new/refused.json", app.h.do(postForm("/app/collections/new?kind=event", app.sess, url.Values{
			"account": {app.realAccount(t, "lichnyj")}, "name": {"Поездки"}, "colour": {"#6b4e7a"}, "components": {"VEVENT", "VTODO"},
		})))
	})
	// Export from the combined calendar (s-agenda-30): the sheet over the
	// week its poll answered; last, as the week warms the counts of the rail.
	t.Run("app-export", func(t *testing.T) {
		week := combined(t, app.h, app.sess, "/app/calendar?from=2026-08-10", func(p view.AgendaPage) *view.PollBar { return p.Poll })
		back := url.QueryEscape("/app/calendar?from=2026-08-10&task=" + week.Poll.TaskID)
		app.compare(t, "app-export/normal.json", app.h.do(get("/app/calendar/export?back="+back, app.sess)))
	})
}

// The quick sheet from the top bar (U-6.16; s-notes-5, -8, -9, -10): on the
// collection a record of its kind was last made in, over the page New was
// pressed on — the note and the contact over the calendar, the event over
// the contacts, the task over the notes. A set of its own: what was last
// used marks New's arrow in every rail after it. The page under a sheet is
// read as its poll answered it: one that polls afresh is caught waiting or
// asking, as the race goes.
func TestFixtureQuickSheets(t *testing.T) {
	app := newMockupApp(t)
	app.lastUsed(t, "zapisi", view.KindNote)
	app.lastUsed(t, "dela", view.KindTask)
	under := map[string]func() string{
		"calendar": func() string {
			week := combined(t, app.h, app.sess, "/app/calendar?from=2026-08-10", func(p view.AgendaPage) *view.PollBar { return p.Poll })
			return "/app/calendar?from=2026-08-10&task=" + week.Poll.TaskID
		},
		"contacts": func() string {
			return "/app/contacts?task=" + combined(t, app.h, app.sess, "/app/contacts", contactsPoll).Poll.TaskID
		},
		"notes": func() string {
			return "/app/notes?task=" + combined(t, app.h, app.sess, "/app/notes", notesPoll).Poll.TaskID
		},
	}
	for _, one := range []struct{ state, kind, over string }{
		{"normal", "notes", "calendar"},
		{"contact", "contacts", "calendar"},
		{"event", "calendar", "contacts"},
		{"task", "tasks", "notes"},
	} {
		t.Run(one.state, func(t *testing.T) {
			rec := app.h.do(get("/app/"+one.kind+"/quick?back="+url.QueryEscape(under[one.over]()), app.sess))
			if rec.Code != http.StatusSeeOther {
				t.Fatalf("the quick %s from the top bar: %d, want the sheet on the collection last used", one.kind, rec.Code)
			}
			app.compare(t, "app-quick/"+one.state+".json", app.h.do(get(rec.Header().Get("Location"), app.sess)))
		})
	}
	// Nothing remembered (s-notes-34): the fields at once, «Write into»
	// not chosen, Save waiting for it; the form posts to the way in.
	t.Run("pick", func(t *testing.T) {
		editBlob(t, app.h, app.sess, func(blob *sealed.Blob) {
			blob.LastUsed = slices.DeleteFunc(blob.LastUsed, func(s sealed.Selection) bool { return s.Kind == view.KindNote })
		})
		rec := app.h.do(get("/app/notes/quick?back="+url.QueryEscape(under["calendar"]()), app.sess))
		sheet := decode[view.NewSheet](t, rec)
		if sheet.Target.Chosen != "" || sheet.Form.Action != "/app/notes/quick" || !sheet.Form.Actions[0].Waits {
			t.Fatalf("the sheet with nothing remembered: chosen %q, action %q, save %+v", sheet.Target.Chosen, sheet.Form.Action, sheet.Form.Actions[0])
		}
		app.compare(t, "app-quick/pick.json", rec)
		// Sent anyway without a choice: the sheet again, what was typed in it.
		again := app.h.do(postForm("/app/notes/quick", app.sess, url.Values{"description": {"Позвонить в МФЦ"}}))
		if again.Code != http.StatusUnprocessableEntity {
			t.Fatalf("the sheet sent without «where»: %d", again.Code)
		}
	})
	// Nowhere to write (s-notes-6): every notebook read-only — last, as
	// it takes them away from what comes after.
	t.Run("nowhere", func(t *testing.T) {
		editBlob(t, app.h, app.sess, func(blob *sealed.Blob) {
			for i := range blob.Collections {
				if blob.Collections[i].Has(view.KindNote) {
					blob.Collections[i].ReadOnly = true
				}
			}
		})
		app.compare(t, "app-quick/nowhere.json", app.h.do(get("/app/notes/quick?back="+url.QueryEscape(under["calendar"]()), app.sess)))
	})
}

// The link made (Пб13; s-publish-13): «Личный» for 10–31 August with its
// .ics — the sheet then shows the link whole in its one field and its day in
// figures. A set of its own: a new link counts in every rail after it.
func TestFixturePublishLinkMade(t *testing.T) {
	app := newMockupApp(t)
	// The link lives 30 days from the set's day, as the frame's.
	app.h.srv.SetClock(mockupNow(t))
	app.h.users.SetClock(mockupNow(t))
	app.compare(t, "app-publish/link.json", app.h.do(postForm("/app/publish", app.sess, url.Values{
		"k": {"calendar"}, "a": {app.realAccount(t, "lichnyj")}, "c": {app.realPath(t, "lichnyj")},
		"how": {"range"}, "from": {"2026-08-10"}, "to": {"2026-08-31"}, "download": {"on"},
	})))
	// A link without expiry on an instance that sends no letters
	// (s-publish-22): «No expiry. …» for its day, no Send by letter. The
	// relay is taken away for this one answer.
	var smtp store.SMTPSettings
	swap := func(to store.SMTPSettings) {
		t.Helper()
		if err := app.h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
			smtp, st.Settings.SMTP = st.Settings.SMTP, to
			return nil, nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	swap(store.SMTPSettings{})
	defer func() { swap(smtp) }()
	app.compare(t, "app-publish/link-never.json", app.h.do(postForm("/app/publish", app.sess, url.Values{
		"k": {"calendar"}, "a": {app.realAccount(t, "lichnyj")}, "c": {app.realPath(t, "lichnyj")},
		"how": {"whole"}, "download": {"on"}, "lives": {"never"},
	})))
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
		defer seedSearch(t, app)()
		at := func(q string) string { return "/app/search?q=" + url.QueryEscape(q) }
		search := func(q string, until func(view.PollBar) bool) view.SearchPage {
			t.Helper()
			first := decode[view.SearchPage](t, app.h.do(get(at(q)+"&again=1", app.sess)))
			if first.Poll.TaskID == "" {
				t.Fatalf("the search for %q asked nobody", q)
			}
			waitPoll(t, app.h, app.sess, first.Poll.TaskID, until)
			return first
		}
		// What s-find-1 found (note 114), and «дача», which nothing holds
		// (s-find-5); then the page without a query lists both (s-find-6).
		search("ковал", nil)
		app.compare(t, "app-search/normal.json", app.h.do(get(at("ковал"), app.sess)))
		// The frames after s-find-1 draw «Общий» answering: every source
		// answers «дача» (s-find-5, «14 of 14»).
		baikal := app.servers["baikal"]
		baikal.Accept("REPORT")
		search("дача", nil)
		app.compare(t, "app-search/nothing.json", app.h.do(get(at("дача"), app.sess)))
		app.compare(t, "app-search/noquery.json", app.h.do(get("/app/search", app.sess)))
		// A pass of «ковал» with some sources holding their answers, taken
		// once the rest have answered; then stopped and let go. A search
		// asks four at a time (conf.FanoutParallelism) and its sources race
		// for the places: no more than three are held, so the fourth place
		// lets the rest through whatever their order.
		held := func(stands []string, feed bool, answered int) (task string, release func()) {
			var lets []func()
			for _, name := range stands {
				prefix := davtest.CalendarHome
				if name == "sftpgo · vps" {
					prefix = davtest.FileRoot
				}
				lets = append(lets, app.servers[name].StallAt(prefix))
			}
			if feed {
				for _, f := range app.feeds {
					lets = append(lets, f.hold())
				}
			}
			release = func() {
				for _, l := range lets {
					l()
				}
			}
			// A failure on the way lets them go too: the stands close only
			// when nothing is held.
			t.Cleanup(release)
			first := search("ковал", func(b view.PollBar) bool {
				n := 0
				for _, sp := range b.Sources {
					if sp.State == view.SourceDone || sp.State == view.SourceEmpty {
						n++
					}
				}
				return n == answered
			})
			return first.Poll.TaskID, release
		}
		// Still running (s-find-4): «Общий», the feed and Documents hold
		// their answers — the five records of the frame, the rows of its
		// rail; then Stop (s-find-10) — what came stays, nothing refused.
		task, release := held([]string{"baikal", "sftpgo · vps"}, true, 11)
		app.compare(t, "app-search/running.json", app.h.do(get(at("ковал"), app.sess)))
		app.h.do(postForm("/app/search/progress/"+task+"/cancel", app.sess, url.Values{}))
		release()
		waitPoll(t, app.h, app.sess, task, nil)
		app.compare(t, "app-search/stopped.json", app.h.do(get(at("ковал"), app.sess)))
		// No source answered (s-find-9).
		for _, srv := range app.servers {
			srv.Refuse("REPORT", http.StatusServiceUnavailable, "offline")
			srv.Refuse("PROPFIND", http.StatusServiceUnavailable, "offline")
		}
		for _, f := range app.feeds {
			f.setDown(true)
		}
		search("договор", nil)
		for _, srv := range app.servers {
			srv.Accept("REPORT")
			srv.Accept("PROPFIND")
		}
		for _, f := range app.feeds {
			f.setDown(false)
		}
		// «Общий» goes on refusing the saved password, as the set has it,
		// and the calendar read again says so to what comes after.
		app.servers["baikal"].Refuse("REPORT", 401, "")
		combined(t, app.h, app.sess, "/app/calendar?from=2026-08-10", func(p view.AgendaPage) *view.PollBar { return p.Poll })
		app.compare(t, "app-search/noanswer.json", app.h.do(get(at("договор"), app.sess)))
		// The line's suggestions (note 118), four searches made (s-find-2):
		// with nothing typed the recent searches; «ковал» typed — what the
		// session holds that has it, its searches' finds among it, and the
		// recent searches that have it (s-find-3).
		search("смета ковалёвой", nil)
		app.compare(t, "app-search-suggest/normal.json", app.h.do(get("/app/search/suggest?q=", app.sess)))
		app.compare(t, "app-search-suggest/typed.json", app.h.do(get("/app/search/suggest?q="+url.QueryEscape("ковал"), app.sess)))
		// The session already runs as many searches and lists as it may
		// (s-find-8).
		slots := 0
		for app.sess.AcquireFanout() {
			slots++
		}
		app.compare(t, "app-search/toomany.json", app.h.do(get(at("смета")+"&again=1", app.sess)))
		for range slots {
			app.sess.ReleaseFanout()
		}
		// Nothing ticked (s-find-7).
		var ticked []sealed.Selection
		editBlob(t, app.h, app.sess, func(b *sealed.Blob) { ticked, b.Selected = b.Selected, nil })
		app.compare(t, "app-search/unticked.json", app.h.do(get(at("ковал")+"&again=1", app.sess)))
		editBlob(t, app.h, app.sess, func(b *sealed.Blob) { b.Selected = ticked })
	})
	// The publish sheets over the page they are opened from (note 105 of
	// Typecase): a contact over its card (s-publish-3), a calendar over
	// Connections (s-publish-2), an event over its card (s-publish-19), a
	// note over its page (s-publish-4), a folder over its parent (s-publish-5).
	// Only the calendar's frame is drawn on a local instance: the others on
	// one with a public address, no badge «local».
	for _, one := range []struct {
		state, slug, route string
		local              bool
	}{
		{"normal", "kontakty", "/app/publish?a=a-nc-home&c=kontakty&k=contacts&uid=4f2b-9c31-a0e7", false},
		{"calendar", "lichnyj", "/app/publish?a=a-nc-home&c=lichnyj&k=calendar", true},
		{"event", "lichnyj", "/app/publish?a=a-nc-home&c=lichnyj&k=calendar&uid=8f1c-4a02-b7d3", false},
		{"note", "zapisi", "/app/publish?a=a-nc-home&c=zapisi&k=notes&uid=5c81-77aa-0d64", false},
		{"folder", "Documents", "/app/publish?a=a-nc-home&c=Documents&k=files&f=carrel&dir=1", false},
	} {
		t.Run("app-publish/"+one.state, func(t *testing.T) {
			route := strings.ReplaceAll(one.route, "a-nc-home", app.realAccount(t, one.slug))
			route = strings.ReplaceAll(route, "c="+one.slug, "c="+url.QueryEscape(app.realPath(t, one.slug)))
			if !one.local {
				app.h.cfg.PublicURL = "https://carrel.example"
				defer func() { app.h.cfg.PublicURL = "" }()
			}
			app.compare(t, "app-publish/"+one.state+".json", app.h.do(get(route, app.sess)))
		})
	}
	// The sheets over Connections, where they are opened (Р-75), as
	// s-profile-7, -8, -9 draw them (Кл10, Кл15, Кл20; note 112): «Дни
	// рождения» renamed, «Личный» — with its meetings on nextcloud, which
	// schedules — moved and deleted. The calendar was read first, so its
	// count is known and «baikal» refused the saved password — the move
	// closes it, «password · 401», and chooses radicale, the one server that
	// suits. The sheets count all the collection holds (s-profile-7, -8,
	// -9): what the searches and the publish sheets above did not draw is
	// put in now.
	seedMockupWhole(t, app)
	combined(t, app.h, app.sess, "/app/calendar?from=2026-08-10", func(p view.AgendaPage) *view.PollBar { return p.Poll })
	for sheet, slug := range map[string]string{"rename": "dni-rozhdeniya", "move": "lichnyj"} {
		t.Run("app-collections-"+sheet, func(t *testing.T) {
			q := url.Values{"a": {app.realAccount(t, slug)}, "c": {app.realPath(t, slug)}, "back": {"/app/settings/connections"}}
			app.compare(t, "app-collections-"+sheet+"/normal.json", app.h.do(get("/app/collections/"+sheet+"?"+q.Encode(), app.sess)))
		})
	}
	// A folder of the storage renamed (s-profile-18) — Кл10.
	t.Run("app-collections-rename-folder", func(t *testing.T) {
		q := url.Values{"a": {app.realAccount(t, "Documents")}, "c": {app.realPath(t, "Documents")}}
		app.compare(t, "app-collections-rename/folder.json", app.h.do(get("/app/collections/rename?"+q.Encode(), app.sess)))
	})
	// Nobody that holds calendars answers (s-profile-21, Кл7): the sheet
	// says so, and Create is off with the same words.
	t.Run("app-collections-new-silent", func(t *testing.T) {
		// A poll asks the collection's tag first and reads what it has kept
		// when the tag is the same: both go unanswered.
		for _, name := range []string{"nextcloud", "radicale · vps"} {
			app.servers[name].Refuse("REPORT", 503, "the server is busy")
			app.servers[name].Refuse("PROPFIND", 503, "the server is busy")
			defer app.servers[name].Accept("REPORT")
			defer app.servers[name].Accept("PROPFIND")
		}
		// Every kind asked again: a server that answered for one kind
		// answers (silentNow).
		combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
		combined(t, app.h, app.sess, "/app/calendar?from=2026-08-10", func(p view.AgendaPage) *view.PollBar { return p.Poll })
		combined(t, app.h, app.sess, "/app/tasks", func(p view.TasksPage) *view.PollBar { return p.Poll })
		combined(t, app.h, app.sess, "/app/notes", func(p view.NotesPage) *view.PollBar { return p.Poll })
		app.compare(t, "app-collections-new/silent.json", app.h.do(get("/app/collections/new?kind=event", app.sess)))
	})
	// Deleting «Личный» and the folder «Documents» (s-profile-9, -19; Кл20):
	// what points at them — the backup job «Личное на домашний диск», which
	// archived them on 14 August, and the three devices of davloom —, last,
	// as the jobs and devices change every rail after them.
	mockupBackups(t, app)
	mockupDevices(t, app)
	app.h.users.SetClock(mockupNow(t))
	for state, slug := range map[string]string{"normal": "lichnyj", "folder": "Documents"} {
		t.Run("app-collections-delete/"+state, func(t *testing.T) {
			q := url.Values{"a": {app.realAccount(t, slug)}, "c": {app.realPath(t, slug)}, "back": {"/app/settings/connections"}}
			app.compare(t, "app-collections-delete/"+state+".json", app.h.do(get("/app/collections/delete?"+q.Encode(), app.sess)))
		})
	}
	_ = anna
}

// C-14.2: the states of the collections' sheets the frames of note 112
// draw — each opened from Connections (Р-75): New collection from an
// account's card (s-profile-29), the rename the server refused (-30), a
// feed's name and colour (-33), a folder of the storage moved (-34) and the
// whole storage moved (-35); on Connections, the delete the server refused
// (-31) and the move that kept the original (-32).
func TestFixtureCollectionSheets(t *testing.T) {
	app := newMockupApp(t)
	seedMockupWhole(t, app)
	settings := "/app/settings/connections"
	// From the account's card before any list was read: no server is
	// known to be silent, so the server field has no line under it
	// (s-profile-29).
	t.Run("app-collections-new/account", func(t *testing.T) {
		// «Поездки» typed, as the frame draws it — the form as typed.
		q := url.Values{"a": {app.realAccount(t, "lichnyj")}, "back": {settings}, "op": {"back"}}
		app.compare(t, "app-collections-new/account.json", app.h.do(postForm("/app/collections/new?"+q.Encode(), app.sess, url.Values{
			"account": {app.realAccount(t, "lichnyj")}, "kind": {"event"}, "name": {"Поездки"}, "colour": {"#6b4e7a"}, "components": {"VEVENT", "VTODO"},
		})))
	})
	combined(t, app.h, app.sess, "/app/calendar?from=2026-08-10", func(p view.AgendaPage) *view.PollBar { return p.Poll })
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	sheet := func(slug string, extra url.Values) url.Values {
		q := url.Values{"a": {app.realAccount(t, slug)}, "c": {app.realPath(t, slug)}, "back": {settings}}
		for k, v := range extra {
			q[k] = v
		}
		return q
	}
	t.Run("app-collections-rename/feed", func(t *testing.T) {
		q := url.Values{"back": {settings}}
		for _, s := range func() []sealed.Subscription {
			key, _ := app.sess.DataKey()
			b, _ := app.h.users.Catalog(app.sess.AccountID(), key)
			return b.Subscriptions
		}() {
			if s.Name == "Праздники РФ" {
				q.Set("a", s.ID)
				q.Set("c", s.FeedCollection().Path)
			}
		}
		app.compare(t, "app-collections-rename/feed.json", app.h.do(get("/app/collections/rename?"+q.Encode(), app.sess)))
	})
	t.Run("app-collections-rename/refused", func(t *testing.T) {
		app.servers["nextcloud"].Refuse("PROPPATCH", http.StatusForbidden, "<d:error xmlns:d=\"DAV:\"><s:message xmlns:s=\"http://sabredav.org/ns\">This calendar is maintained by the server</s:message></d:error>")
		defer app.servers["nextcloud"].Accept("PROPPATCH")
		q := sheet("dni-rozhdeniya", nil)
		app.compare(t, "app-collections-rename/refused.json", app.h.do(postForm("/app/collections/rename?"+q.Encode(), app.sess, url.Values{"name": {"Дни рождения"}, "colour": {"#a05a2c"}})))
	})
	t.Run("app-settings-connections/delete-refused", func(t *testing.T) {
		app.servers["nextcloud"].Refuse("DELETE", http.StatusForbidden, "<d:error xmlns:d=\"DAV:\"><d:need-privileges/></d:error>")
		defer app.servers["nextcloud"].Accept("DELETE")
		q := sheet("arhiv", nil)
		app.compare(t, "app-settings-connections/delete-refused.json", app.h.do(postForm("/app/collections/delete?"+q.Encode(), app.sess, url.Values{"name": {"Архив"}})))
	})
	// The storage of nextcloud is «Files» itself, as s-profile-17 and -35
	// draw it: the address the connection is made with.
	nc := app.realAccount(t, "Files")
	editBlob(t, app.h, app.sess, func(blob *sealed.Blob) {
		for i := range blob.Connections {
			if blob.Connections[i].ID == nc {
				blob.Connections[i].FileRoot = app.servers["nextcloud"].URL + app.realPath(t, "Files")
			}
		}
	})
	t.Run("app-collections-move/folder", func(t *testing.T) {
		q := sheet("Documents", url.Values{"to": {nc}})
		app.compare(t, "app-collections-move/folder.json", app.h.do(get("/app/collections/move?"+q.Encode(), app.sess)))
	})
	t.Run("app-collections-move/storage", func(t *testing.T) {
		q := sheet("Files", url.Values{"to": {app.realAccount(t, "Documents")}})
		app.compare(t, "app-collections-move/storage.json", app.h.do(get("/app/collections/move?"+q.Encode(), app.sess)))
	})
	// The move whose copy radicale did not keep whole (s-profile-32): the
	// original stays, both are there, the notice names what was lost.
	t.Run("app-settings-connections/move-kept", func(t *testing.T) {
		app.servers["radicale · vps"].Drop = []string{"ATTACH", "LOCATION"}
		q := sheet("lichnyj", nil)
		app.compare(t, "app-settings-connections/move-kept.json", app.h.do(postForm("/app/collections/move?"+q.Encode(), app.sess, url.Values{"to": {app.realAccount(t, "proekty")}})))
	})
}

// C-14.2: New calendar when no connected server holds calendars
// (s-profile-28): no fields, Connect account. A set of its own: the
// calendars' homes are taken away.
func TestFixtureCollectionNewNone(t *testing.T) {
	app := newMockupApp(t)
	editBlob(t, app.h, app.sess, func(blob *sealed.Blob) {
		for i := range blob.Connections {
			blob.Connections[i].CalendarHome = ""
		}
	})
	app.compare(t, "app-collections-new/none.json", app.h.do(get("/app/collections/new?kind=event", app.sess)))
}

// C-14.2 and C-14.3: the states of a list of contacts beside its normal
// one. Each is a state of the screen, not another screen.
func TestFixtureContactsStates(t *testing.T) {
	app := newMockupApp(t)
	// The combined book was opened first, as a person comes to it: the rail
	// of one book carries the section's count and tags (states-4, -10, -11;
	// the owner's decision of 08.10.2026 on the tags). «Команда» is the
	// book of the set, with its one card: states-4 draws it so since
	// Typecase's note 44 (before, the frame drew a second card, which lived
	// in this test alone).
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
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
		refuseReads(app.servers["nextcloud"])
		defer acceptReads(app.servers["nextcloud"])
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
		app.servers["nextcloud"].Drop = []string{"X-EVOLUTION-FILE-AS", "X-ABLABEL", "X-CARREL-UNKNOWN-FLAG"}
		form.Set("title", "Главный редактор")
		app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
		// The save went through and the form comes back with the report:
		// what the server did not keep is said where the change was made,
		// not on a screen the person has to go and find (C-9).
		app.compare(t, "app-contact-edit/loss.json", app.h.do(postForm(rec+"/edit", app.sess, form)))
	})
}

// C-14.2: the files, the links given out, and the link an invited person
// opens.
func TestFixtureFilesAndInvite(t *testing.T) {
	app := newMockupApp(t)
	openSections(t, app)
	readFolders(t, app)
	// The list of links draws davloom with its three devices, the last
	// answer the morning of the frames, and «Backups are running» in its
	// rail's foot (s-publish-1, -18, -20): the devices, jobs and runs of
	// the set.
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	for _, id := range mockupDevices(t, app) {
		app.h.users.TouchDevice(id, mockupDayAt(t, "2026-08-16 09:41"))
	}
	mockupBackups(t, app)
	for _, one := range []struct{ dir, state, route string }{
		{"app-files", "normal", "/app/files"},
		{"app-files-published", "normal", "/app/files/published"},
	} {
		t.Run(one.dir, func(t *testing.T) {
			app.compare(t, one.dir+"/"+one.state+".json", app.h.do(get(one.route, app.sess)))
		})
	}
	t.Run("app-mail-link", func(t *testing.T) {
		// «Send this link» of the calendar's link, over Published links
		// (s-mail-4).
		for _, l := range app.h.users.PublishedLinks(app.sess.AccountID()) {
			if l.Target.Kind == string(view.KindEvent) {
				app.compare(t, "app-mail-link/normal.json", app.h.do(get("/app/mail/link?id="+l.ID, app.sess)))
				return
			}
		}
		t.Fatal("no calendar's link in the set")
	})
	t.Run("invite", func(t *testing.T) {
		// The invitation of s-auth-12: sent by letter to anna@example.org —
		// the address shown and switched off —, the login the person's to
		// choose.
		letter, err := app.h.users.Invite(app.sess.AccountID(), "anna@example.org", store.RoleUser)
		if err != nil {
			t.Fatal(err)
		}
		app.compare(t, "invite/normal.json", app.h.do(get("/invite/"+letter, nil)))
		// An invitation by link (s-admin-1) has no address and no field of
		// it; its line under the title is the caption's of s-auth-12, which
		// draws no frame of its own.
		link, err := app.h.users.Invite(app.sess.AccountID(), "", store.RoleUser)
		if err != nil {
			t.Fatal(err)
		}
		app.compare(t, "invite/link.json", app.h.do(get("/invite/"+link, nil)))
	})
	t.Run("published-no-mail", func(t *testing.T) {
		// An instance that sends no letters (s-publish-17): the rows offer
		// no Send. The relay is taken away for this one answer.
		var smtp store.SMTPSettings
		swap := func(to store.SMTPSettings) {
			t.Helper()
			if err := app.h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
				smtp, st.Settings.SMTP = st.Settings.SMTP, to
				return nil, nil
			}); err != nil {
				t.Fatal(err)
			}
		}
		swap(store.SMTPSettings{})
		defer func() { swap(smtp) }()
		app.compare(t, "app-files-published/no-mail.json", app.h.do(get("/app/files/published", app.sess)))
	})
	t.Run("published-gone", func(t *testing.T) {
		// What the file's link opened is no longer there (s-publish-21): the
		// row stays for Revoke, with nothing to copy or send. It was opened
		// before it went, as the frame's row was.
		for token, name := range app.byAccount {
			if name == "link-smeta-token" {
				if rec := app.h.do(get("/p/"+token, nil)); rec.Code != http.StatusOK {
					t.Fatalf("open the file's link: %d", rec.Code)
				}
			}
		}
		// The frame draws it alone: the calendar's link is revoked first.
		for _, l := range app.h.users.PublishedLinks(app.sess.AccountID()) {
			var err error
			if l.Target.Kind == string(view.KindFile) {
				err = app.h.users.MarkLinkGone(l.ID)
			} else {
				err = app.h.users.RevokePublished(app.sess.AccountID(), l.ID)
			}
			if err != nil {
				t.Fatal(err)
			}
		}
		app.compare(t, "app-files-published/gone.json", app.h.do(get("/app/files/published", app.sess)))
	})
	t.Run("published-empty", func(t *testing.T) {
		// Every link revoked: the list says so (the client's scan ФД6).
		for _, l := range app.h.users.PublishedLinks(app.sess.AccountID()) {
			if rec := app.h.do(postForm("/app/publish?"+url.Values{"op": {"revoke"}, "id": {l.ID}}.Encode(), app.sess, url.Values{})); rec.Code != http.StatusSeeOther {
				t.Fatalf("revoke: %d %s", rec.Code, rec.Body)
			}
		}
		app.compare(t, "app-files-published/empty.json", app.h.do(get("/app/files/published", app.sess)))
	})
	t.Run("queue-refused", func(t *testing.T) {
		// A storage that writes only what it was told the length of
		// (s-files-24): the files sent with their size are uploaded, a name
		// taken waits for its choice, the one without a size is refused, and
		// its row says why — the four rows of s-files-2.
		srv := app.servers["sftpgo · vps"]
		srv.RefuseUnsizedPUT = true
		defer func() { srv.RefuseUnsizedPUT = false }()
		report := strings.Repeat(".", 4300000)
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		_ = w.WriteField("_csrf", app.sess.CSRFToken())
		poster := strings.Repeat(".", 8700000)
		_ = w.WriteField("size.отчёт-июль.pdf", strconv.Itoa(len(report)))
		_ = w.WriteField("size.плакат.png", strconv.Itoa(len(poster)))
		part, _ := w.CreateFormFile("file", "отчёт-июль.pdf")
		_, _ = part.Write([]byte(report))
		part, _ = w.CreateFormFile("file", "плакат.png")
		_, _ = part.Write([]byte(poster))
		// A name already taken, as s-files-2 draws it among the four (note
		// 83 of Typecase): its choice stands in its row.
		_ = w.WriteField("size.release-checklist.md", "7")
		part, _ = w.CreateFormFile("file", "release-checklist.md")
		_, _ = part.Write([]byte("# again"))
		part, _ = w.CreateFormFile("file", "архив-2025.zip")
		_, _ = part.Write([]byte("zip"))
		_ = w.Close()
		req := httptest.NewRequest(http.MethodPost, app.filesAt(t, "Documents", "upload", "carrel", "attachments", "2026-08"), &buf)
		req.Header.Set("Content-Type", w.FormDataContentType())
		req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: app.sess.ID()})
		if rec := app.h.do(req); rec.Code >= 400 {
			t.Fatalf("upload: %d %s", rec.Code, rec.Body)
		}
		for _, name := range []string{"отчёт-июль.pdf", "плакат.png"} {
			srv.SetModified(app.realPath(t, "Documents")+"carrel/attachments/2026-08/"+name, mockupNow(t))
		}
		// Over the three chosen of s-files-2, the frame of the queue under a
		// selection (its refused row is the fragment s-files-24).
		three := []string{"2026-08-10-sync-board.png", "release-checklist.md", "2026-08-04-whiteboard.jpg"}
		app.compare(t, "app-files-folder/refused.json", app.h.do(get(app.filesAt(t, "Documents", "", "carrel", "attachments", "2026-08")+"?"+url.Values{"select": three}.Encode(), app.sess)))
	})
	t.Run("app-files-picker", func(t *testing.T) {
		// «Upload files» of the menu (s-notes-11) over August's folder of the
		// attachments, which is there since 4 August (note 180 of Typecase).
		at := "/app/files/picker?" + url.Values{"for": {"upload"},
			"to": {app.filesAt(t, "Documents", "", "carrel", "attachments", "2026-08")}}.Encode()
		app.compare(t, "app-files-picker/normal.json", app.h.do(get(at, app.sess)))
	})
}

// openSections reads every section once, as a person who comes to the
// files has: the counts of the sections' rail come from there (s-files-1,
// -6; the client's scan ФА5).
func openSections(t *testing.T, app *mockupApp) {
	t.Helper()
	for _, path := range []string{"/app/contacts", "/app/calendar?from=2026-08-10", "/app/tasks", "/app/notes"} {
		openList(t, app, path)
	}
}

// openUnticked opens the lists that stand out of the combined views — «Дни
// рождения», «Общие с семьёй», «Рассылки», and the tasks of the feed
// «Праздники РФ» —, as a person looks at one now and then: the tables of
// the settings count what each holds (s-profile-1: «Objects» of every row).
func openUnticked(t *testing.T, app *mockupApp) {
	t.Helper()
	for _, one := range []struct{ segment, slug string }{
		{"calendar", "dni-rozhdeniya"}, {"notes", "zapiski-obshchie"}, {"contacts", "rassylki"},
	} {
		path := app.listPath(t, one.segment, one.slug)
		if one.segment == "calendar" {
			path += "?from=2026-08-10"
		}
		if rec := app.h.do(get(path, app.sess)); rec.Code != 200 {
			t.Fatalf("%s: %d", path, rec.Code)
		}
	}
	key, _ := app.sess.DataKey()
	blob, _ := app.h.users.Catalog(app.sess.AccountID(), key)
	for _, s := range blob.Subscriptions {
		if s.Name == "Праздники РФ" {
			path := "/app/tasks/" + s.ID + "/" + url.PathEscape(s.FeedCollection().Path)
			if rec := app.h.do(get(path, app.sess)); rec.Code != 200 {
				t.Fatalf("%s: %d", path, rec.Code)
			}
		}
	}
}

// readFolders reads the folders the files fixtures show at the set's hour:
// a screen is taken two minutes after, and its refresh says «2 min ago», as
// the frames draw it (s-files-1, -6).
func readFolders(t *testing.T, app *mockupApp) {
	t.Helper()
	for _, at := range []string{
		app.filesAt(t, "Documents", "", "carrel", "attachments", "2026-08"), app.filesAt(t, "Documents", "", "carrel", "attachments"),
		app.filesAt(t, "Documents", ""),
		app.filesAt(t, "Public", ""), app.filesAt(t, "Files", ""),
	} {
		if rec := app.h.do(get(at, app.sess)); rec.Code != 200 {
			t.Fatalf("%s: %d", at, rec.Code)
		}
	}
}

// filesAt is an address of a file collection of the set, as Carrel issued
// it: marker, then the path under the collection a segment at a time.
func (a *mockupApp) filesAt(t *testing.T, slug, marker string, segs ...string) string {
	t.Helper()
	h := "/app/files/" + a.realAccount(t, slug) + "/" + url.PathEscape(a.realPath(t, slug)) + "/"
	if marker != "" {
		h += marker + "/"
	}
	for _, s := range segs {
		h += url.PathEscape(s) + "/"
	}
	return h
}

// C-14.2: a folder of the files (ФБ5) and its states — one file chosen with
// its panel (s-files-1), tiles (s-files-5), a read-only storage (s-files-15),
// an empty folder, a storage that did not answer, three chosen with the
// queue of an upload (s-files-2) — and the sheets over it: move (s-files-4),
// copy (s-files-11), to another storage (s-files-12), New folder refused
// (s-files-13), rename, delete; From Files (s-files-16) and New folder of
// the upload picker refused (s-files-19).
func TestFixtureFilesFolderAndSheets(t *testing.T) {
	app := newMockupApp(t)
	openSections(t, app)
	readFolders(t, app)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	// The attachments' folders are by month (note 180 of Typecase): the
	// screens open August's, where the four files of the month lie.
	attachments := app.filesAt(t, "Documents", "", "carrel", "attachments")
	august := app.filesAt(t, "Documents", "", "carrel", "attachments", "2026-08")
	op := app.filesAt(t, "Documents", "op", "carrel", "attachments", "2026-08")
	three := []string{"2026-08-10-sync-board.png", "release-checklist.md", "2026-08-04-whiteboard.jpg"}
	sheet := func(name string, form url.Values) {
		t.Helper()
		app.compare(t, "app-files-op/"+name+".json", app.h.do(postForm(op+"?"+form.Encode(), app.sess, url.Values{})))
	}

	app.compare(t, "app-files-folder/normal.json", app.h.do(get(august+"?select="+url.QueryEscape(three[0]), app.sess)))
	// states-34 draws «Public» ticked with the other two, «3 of 3»: ticked
	// for this one answer — the other frames draw it unticked.
	app.tickFiles(t, "Documents", "Public", "Files")
	app.compare(t, "app-files-folder/readonly.json", app.h.do(get(app.filesAt(t, "Public", ""), app.sess)))
	app.tickFiles(t, "Documents", "Files")

	july := app.filesAt(t, "Documents", "", "carrel", "attachments", "2026-07")
	sheet("normal", url.Values{"op": {"move"}, "item": three, "to": {july}})
	sheet("copy", url.Values{"op": {"copy"}, "item": three, "to": {july}})
	sheet("elsewhere", url.Values{"op": {"move"}, "item": three, "to": {app.filesAt(t, "Files", "")}})
	sheet("rename", url.Values{"op": {"rename"}, "item": three[:1]})
	sheet("delete", url.Values{"op": {"delete"}, "item": three})
	// July's folder is deleted from where it lies, the attachments (s-files-27).
	app.compare(t, "app-files-op/delete-folder.json", app.h.do(postForm(app.filesAt(t, "Documents", "op", "carrel", "attachments")+"?"+
		url.Values{"op": {"delete"}, "item": {"2026-07"}}.Encode(), app.sess, url.Values{})))
	// New folder of the tree with a name taken in the chosen folder.
	app.compare(t, "app-files-op/refused.json", app.h.do(postForm(op+"?"+url.Values{"op": {"move"}, "item": three, "mkcol": {"1"},
		"to": {app.filesAt(t, "Documents", "", "carrel", "attachments")}}.Encode(), app.sess, url.Values{"name": {"2026-07"}})))

	// New folder of the head with a name taken (s-files-23).
	app.compare(t, "app-files-op/new.json", app.h.do(postForm(app.filesAt(t, "Documents", "mkcol", "carrel", "attachments"), app.sess, url.Values{"name": {"2026-07"}})))

	from := "/app/files/picker?" + url.Values{"for": {"attach"}, "k": {"tasks"}, "a": {app.realAccount(t, "dela")}, "c": {app.realPath(t, "dela")},
		"uid": {"aa11-2d09-5f84"}, "to": {august}}.Encode()
	app.compare(t, "app-files-picker/attach.json", app.h.do(get(from, app.sess)))
	app.compare(t, "app-files-picker/refused.json", app.h.do(postForm("/app/files/picker?"+url.Values{"for": {"upload"}, "mkcol": {"1"},
		"to": {attachments}}.Encode(), app.sess, url.Values{"name": {"2026-07"}})))

	// The queue (s-files-2): a report and a poster uploaded, a name already
	// taken, and a file the storage would not write without its size — the
	// four rows of the frame (note 83 of Typecase; its poster is still on
	// its way there, an answer of the server has every file's end).
	sftp := app.servers["sftpgo · vps"]
	sftp.RefuseUnsizedPUT = true
	rec := app.h.do(sizedUpload(app.filesAt(t, "Documents", "upload", "carrel", "attachments", "2026-08"), app.sess,
		upload{"отчёт-июль.pdf", strings.Repeat(".", 4300000)}, upload{"плакат.png", strings.Repeat(".", 8700000)},
		upload{"release-checklist.md", "# again"}, upload{"архив-2025.zip", ""}))
	sftp.RefuseUnsizedPUT = false
	if rec.Code >= 400 {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body)
	}
	// The stand stamps what arrives with the wall clock; the set's day stands.
	for _, name := range []string{"отчёт-июль.pdf", "плакат.png"} {
		app.servers["sftpgo · vps"].SetModified(app.realPath(t, "Documents")+"carrel/attachments/2026-08/"+name, mockupNow(t))
	}
	app.compare(t, "app-files-folder/long.json", app.h.do(get(august+"?"+url.Values{"select": three}.Encode(), app.sess)))

	// An empty folder (states-32): «черновики» of July's, under the
	// attachments (note 180 of Typecase) — on a stand of its own: the trees
	// of the sheets draw July's folder without folders in it.
	t.Run("empty", func(t *testing.T) {
		app := newMockupApp(t)
		openSections(t, app)
		readFolders(t, app)
		app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
		app.servers["sftpgo · vps"].SeedFolder(app.realPath(t, "Documents") + "carrel/attachments/2026-07/черновики/")
		app.compare(t, "app-files-folder/empty.json", app.h.do(get(app.filesAt(t, "Documents", "", "carrel", "attachments", "2026-07", "черновики"), app.sess)))
	})

	// A storage that did not answer (states-33): Documents, opened at its
	// root.
	app.servers["sftpgo · vps"].Refuse("PROPFIND", http.StatusServiceUnavailable, "the server is busy")
	app.compare(t, "app-files-folder/source-error.json", app.h.do(get(app.filesAt(t, "Documents", ""), app.sess)))
	app.servers["sftpgo · vps"].Accept("PROPFIND")

	// No storage ticked (states-35): the root says so and offers them all.
	if rec := app.h.do(postForm("/app/sources/files", app.sess, url.Values{})); rec.Code >= 400 {
		t.Fatalf("untick every storage: %d", rec.Code)
	}
	app.compare(t, "app-files/empty.json", app.h.do(get("/app/files", app.sess)))
}

// tickFiles ticks the file collections named and no other, as the rail's
// form does.
func (a *mockupApp) tickFiles(t *testing.T, slugs ...string) {
	t.Helper()
	var picks []sealed.Selection
	for _, slug := range slugs {
		picks = append(picks, sealed.Selection{Kind: view.KindFile, AccountID: a.realAccount(t, slug), Path: a.realPath(t, slug)})
	}
	key, _ := a.sess.DataKey()
	if err := a.h.users.SetSelection(a.sess.AccountID(), key, view.KindFile, picks); err != nil {
		t.Fatal(err)
	}
}

// C-14.2: a folder of pictures as tiles (s-files-5) — «2026-07» of the
// attachment folder: the chosen picture with its width and height, pictures
// with their sizes, a PDF, a folder nobody opened (no count), one opened and
// empty («0 items»), a picture too large to show. The files are the set's;
// the two folders are made here, as «Scans» is: an empty folder has no file
// to stand for it in the set, and the trees of the sheets (s-files-4,
// s-notes-11) draw «2026-07» without folders in it.
func TestFixtureFilesTiles(t *testing.T) {
	app := newMockupApp(t)
	july := davtest.FileRoot + "Documents/carrel/attachments/2026-07/"
	srv := app.servers["sftpgo · vps"]
	for _, name := range []string{"архив", "черновики"} {
		srv.SeedFolder(july + name + "/")
		srv.SetModified(july+name, mockupDayAt(t, "2026-07-03 11:00"))
	}
	srv.SetModified(strings.TrimSuffix(july, "/"), mockupDayAt(t, "2026-07-01 09:12"))
	openSections(t, app)
	readFolders(t, app)
	at := app.filesAt(t, "Documents", "", "carrel", "attachments", "2026-07")
	for _, read := range []string{at, at + url.PathEscape("черновики") + "/"} {
		if rec := app.h.do(get(read, app.sess)); rec.Code != 200 {
			t.Fatalf("%s: %d", read, rec.Code)
		}
	}
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	// The chosen tile is drawn without the panel: Details off.
	app.compare(t, "app-files-folder/tiles.json", app.h.do(get(at+"?view=tiles&details=0&select=whiteboard.jpg", app.sess)))
	// The same folder on paper, its folders first (print-6; note 180).
	app.compare(t, "app-files-folder/print.json", app.h.do(get(onPaper(at), app.sess)))
}

// C-14.2: the one page of every dead secret link (rest-1). What the link
// was is not in it, so one fixture stands for all of them.
func TestFixtureDeadLink(t *testing.T) {
	app := &mockupApp{h: newHarness(t), byAccount: map[string]string{}, byPath: map[string]string{}}
	app.h.admin("mix", "password123")
	app.compareAt(t, "dead-link/normal.json", app.h.do(get("/invite/"+strings.Repeat("A", 43), nil)), http.StatusNotFound)
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
	open.h.do(postForm("/admin/settings", sess, settingsForm(t, open.h, sess, map[string]string{
		"host": "127.0.0.1", "port": "25", "security": "none",
		"username": "relay", "password": "relay-password-xyz", "from": "noreply@carrel.example",
	})))
	if rec := open.h.do(postForm("/admin/invites", sess, url.Values{"action": {"selfreg"}, "selfReg": {"on"}})); rec.Code != 303 {
		t.Fatalf("open the registration: %d %s", rec.Code, rec.Body.String())
	}
	t.Run("register", func(t *testing.T) {
		open.compare(t, "register/normal.json", open.h.do(get("/register", nil)))
	})
	_ = admin
}

// C-14.2 and C-14.3: the duplicates as s-dup-1, -4, -5, -6 draw them (Дб10,
// Дб12, Дб21): nothing read yet; every section read, the groups of four
// kinds; the merge of an event; a merge whose delete the server refused.
func TestFixtureDuplicatesStates(t *testing.T) {
	t.Run("app-duplicates-empty", func(t *testing.T) {
		app := newMockupApp(t)
		app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
		app.compare(t, "app-duplicates/empty.json", app.h.do(get("/app/duplicates", app.sess)))
	})
	app := newMockupApp(t)
	// Every group s-dup-1 draws: the copies of the event, the task and the
	// notes linked as the set holds them (from-typecase note 117 p. 2), the
	// copy in «Общий» answering.
	linkMockupEvent(t, app)
	linkMockupTask(t, app)
	app.servers["baikal"].Accept("REPORT")
	// The duty of «Дежурства» has two reminders the one of «Работа» lacks
	// and one occurrence rewritten by hand, after the week the frames show:
	// the merge of s-dup-4 gains the reminders and does not carry the
	// occurrence.
	duty := davtest.CalendarHome + "dezhurstva/5e90-33c1-a8d2.ics"
	radicale := app.servers["radicale · vps"]
	if body, _, ok := radicale.Object(duty); ok {
		alarms := "BEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-P1D\r\nDESCRIPTION:Дежурство\r\nEND:VALARM\r\n" +
			"BEGIN:VALARM\r\nACTION:DISPLAY\r\nTRIGGER:-PT1H\r\nDESCRIPTION:Дежурство\r\nEND:VALARM\r\nEND:VEVENT"
		body = strings.Replace(body, "END:VEVENT", alarms, 1)
		moved := "BEGIN:VEVENT\r\nUID:5e90-33c1-a8d2\r\nDTSTAMP:20260601T061500Z\r\nRECURRENCE-ID;VALUE=DATE:20260824\r\n" +
			"DTSTART;VALUE=DATE:20260825\r\nDTEND;VALUE=DATE:20260826\r\nSUMMARY:Дежурство: первая линия\r\nEND:VEVENT\r\nEND:VCALENDAR"
		radicale.Seed(duty, strings.Replace(body, "END:VCALENDAR", moved, 1))
	} else {
		t.Fatal("the duty of «Дежурства» is not on the stand")
	}
	for _, list := range []string{"/app/contacts", "/app/calendar?from=2026-08-10", "/app/tasks", "/app/notes"} {
		openList(t, app, list)
	}
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	t.Run("app-duplicates", func(t *testing.T) {
		app.compare(t, "app-duplicates/normal.json", app.h.do(get("/app/duplicates", app.sess)))
	})
	// The merge of a group: its Merge, a button of the card's form when there
	// is something to choose (Дб9), sent as the card stands.
	mergeOf := func(t *testing.T, kind view.Kind) string {
		t.Helper()
		page := decode[view.DuplicatesPage](t, app.h.do(get("/app/duplicates", app.sess)))
		for _, g := range page.Groups {
			if g.Kind != kind || g.Decision != view.DecisionNone {
				continue
			}
			for _, a := range g.Actions {
				if a.ID != "duplicates.merge" || !a.Enabled {
					continue
				}
				return app.h.do(decideRequest(t, app.sess, g, a.ID, asItStands(g, nil))).Header().Get("Location")
			}
		}
		t.Fatalf("no %s group to merge among %d", kind, len(page.Groups))
		return ""
	}
	t.Run("app-duplicates-merge-event", func(t *testing.T) {
		app.compare(t, "app-duplicates-merge-event/normal.json", app.h.do(get(mergeOf(t, view.KindEvent), app.sess)))
	})
	t.Run("app-duplicates-merge-in-part", func(t *testing.T) {
		merge := decode[view.DuplicatesMergePage](t, app.h.do(get(mergeOf(t, view.KindContact), app.sess)))
		var confirm string
		for _, a := range merge.Actions {
			if a.ID == "duplicates.merge.confirm" {
				confirm = a.Href
			}
		}
		app.servers["nextcloud"].Refuse(http.MethodDelete, http.StatusForbidden, "<error/>")
		app.compare(t, "app-duplicates-merge/in-part.json", app.h.do(postForm(confirm, app.sess, url.Values{})))
	})
}

// C-14.2: the merge of two records, the letter, and the link a forgotten
// password leads to.
func TestFixtureMergeLetterAndReset(t *testing.T) {
	t.Run("app-duplicates-merge", func(t *testing.T) {
		app := newMockupApp(t)
		// Since Typecase's note 44 the set holds a pair to merge — Вера
		// Лебедева of «Архив» and «Лебедева Вера» of «Контакты», the
		// candidate s-dup-1 draws and s-dup-2 merges — beside the two Артём
		// Белов, who are linked and decided.
		combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
		// The merge stands on the ground of the screen it came from: every
		// collection compared, «Общий» answering (s-dup-2, Дб20).
		app.servers["baikal"].Accept("REPORT")
		for _, list := range []string{"/app/calendar?from=2026-08-10", "/app/tasks", "/app/notes"} {
			openList(t, app, list)
		}
		page := decode[view.DuplicatesPage](t, app.h.do(get("/app/duplicates?kind=contact", app.sess)))
		var merge string
		for _, g := range page.Groups {
			for _, a := range g.Actions {
				if a.ID == "duplicates.merge" && a.Enabled && strings.Contains(string(g.Rows[0].Title)+string(g.Rows[1].Title), "Лебедева") {
					// A button of the card's form (Дб9): it leads to the
					// merge with what the card chose, the card as it stands.
					merge = app.h.do(decideRequest(t, app.sess, g, a.ID, asItStands(g, nil))).Header().Get("Location")
				}
			}
		}
		if merge == "" {
			t.Fatalf("no pair of Вера Лебедева to merge among %d groups", len(page.Groups))
		}
		// «Архив» chosen to keep the merged record (s-dup-2): its link
		// among where it stays, as a person presses it.
		plan := decode[view.DuplicatesMergePage](t, app.h.do(get(merge, app.sess)))
		for _, a := range plan.Stays {
			if strings.HasSuffix(string(a.Label), "Архив") {
				merge = a.Href
			}
		}
		app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
		app.compare(t, "app-duplicates-merge/normal.json", app.h.do(get(merge, app.sess)))
	})
	t.Run("app-mail-compose", func(t *testing.T) {
		// The instance of the set has a relay and lets people send
		// (newMockupApp).
		app := newMockupApp(t)
		app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
		// The letter of s-mail-1 (С23): opened over the book, two people of
		// it typed by name, a card, an event and the estimate attached, a
		// folder as a link.
		// The books read, as the combined list reads them: the names typed
		// are found in them, and the book under the letter is that read,
		// done (as under Move, app-contact-move).
		book := combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
		back := url.QueryEscape("/app/contacts?task=" + book.Poll.TaskID)
		recipients := func() []string {
			var out []string
			for _, o := range letterTo(decode[view.MailSheet](t, app.h.do(get("/app/mail/compose?back="+back, app.sess)))) {
				out = append(out, o.Value)
			}
			return out
		}
		// Анна has two addresses since note 117 of Typecase: her name alone
		// asks which (s-mail-3); the letter is to her first.
		for _, who := range []string{"a.kovaleva@example.org", "Белов"} {
			if rec := app.h.do(postForm("/app/mail/send?op=add", app.sess, url.Values{"to": recipients(), "to.add": {who},
				"subject": {"Договор подряда и смета"}, "text": {"Пётр, ниже — договор и смета, которые мы обсуждали."}})); rec.Code != 200 {
				t.Fatalf("add %s: %d %s", who, rec.Code, rec.Body.String())
			}
		}
		attach := func(q url.Values) {
			t.Helper()
			sheet := decode[view.MailAttachSheet](t, app.h.do(get("/app/mail/attach?"+q.Encode(), app.sess)))
			if rec := app.h.do(postForm(actionHref(t, sheet.Actions, "mail.attach.add"), app.sess, formFields(sheet.Form))); rec.Code != 303 {
				t.Fatalf("attach %v: %d %s", q, rec.Code, rec.Body.String())
			}
		}
		attach(url.Values{"what": {"contact"}, "a": {app.realAccount(t, "kontakty")}, "c": {app.realPath(t, "kontakty")}, "uid": {"4f2b-9c31-a0e7"}})
		// The event from one calendar: a search of all of them would leave
		// the readings of the page under the letter half-way.
		events := decode[view.MailAttachSheet](t, app.h.do(get("/app/mail/attach?"+url.Values{"what": {"event"},
			"a": {app.realAccount(t, "lichnyj")}, "c": {app.realPath(t, "lichnyj")}}.Encode(), app.sess)))
		if len(events.Rows) == 0 {
			t.Fatal("no event to attach")
		}
		u, _ := url.Parse(events.Rows[0].Href)
		attach(u.Query())
		docs := url.Values{"a": {app.realAccount(t, "Documents")}, "c": {app.realPath(t, "Documents")}}
		attach(url.Values{"what": {"file"}, "a": docs["a"], "c": docs["c"], "dir": {"carrel/attachments/2026-08"}, "path": {"carrel/attachments/2026-08/смета-подрядчик.pdf"}})
		attach(url.Values{"what": {"folder"}, "a": docs["a"], "c": docs["c"], "dir": {"carrel"}})
		app.compare(t, "app-mail-compose/normal.json", app.h.do(get("/app/mail/compose", app.sess)))

		typed := url.Values{"to": recipients(), "subject": {"Договор подряда и смета"}, "text": {"Пётр, ниже — договор и смета, которые мы обсуждали."}}
		// The letter as the recipients get it (s-mail-14): its links one
		// to a line, so a second folder for the while; and Cancel of a
		// letter with text (s-mail-15).
		attach(url.Values{"what": {"folder"}, "a": docs["a"], "c": docs["c"], "dir": {"carrel/attachments"}})
		app.compare(t, "app-mail-preview/normal.json", app.h.do(postForm("/app/mail/send?op=preview", app.sess, typed)))
		if rec := app.h.do(postForm("/app/mail/send?op=remove&i=4", app.sess, typed)); rec.Code != 200 {
			t.Fatalf("remove the second folder: %d %s", rec.Code, rec.Body.String())
		}
		app.compare(t, "app-mail-close/normal.json", app.h.do(postForm("/app/mail/send?op=close", app.sess, typed)))

		// The relay of the set, in memory, as the instance has it: the
		// letter goes from the person's login.
		relay := mailtest.New(t)
		if err := app.h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
			st.Settings.SMTP.Host, st.Settings.SMTP.Port = relay.Host, relay.Port
			return nil, nil
		}); err != nil {
			t.Fatal(err)
		}
		// The relay takes no letter now (s-mail-17): the letter as it was,
		// its words first under To.
		relay.Defer = "451 4.7.0 Try again later"
		app.compare(t, "app-mail-compose/refused.json", app.h.do(postForm("/app/mail/send", app.sess, typed)))
		// It refuses one of the two (s-mail-18).
		relay.Defer, relay.Refuse, relay.RefuseReply = "", "a.belov@example.org", "550 5.1.1 Mailbox unavailable"
		sent := app.h.do(postForm("/app/mail/send", app.sess, typed))
		app.compare(t, "app-mail-sent/normal.json", sent)
		// Sent again to him, it goes: the letter closes to the book, which
		// says so once (s-mail-16). No fixture: the frame's rail is not the
		// set's (to-typecase.md, task 241).
		relay.Refuse = ""
		again := decode[view.MailSheet](t, app.h.do(get(actionHref(t, decode[view.MailSentSheet](t, sent).Actions, "mail.again"), app.sess)))
		if rec := app.h.do(postForm("/app/mail/send", app.sess, url.Values{"to": letterToValues(again), "subject": {letterValue(again, "subject")}, "text": {letterValue(again, "text")}})); rec.Code != 303 {
			t.Fatalf("sent again: %d %s", rec.Code, rec.Body.String())
		}
		page := decode[view.ContactsListPage](t, app.h.do(get(app.listPath(t, "contacts", "kontakty"), app.sess)))
		if len(page.Notices) != 1 || page.Notices[0].Role != view.NoticeAccent || page.Notices[0].Body != "The letter went to 1 recipient." {
			t.Errorf("notices = %+v", page.Notices)
		}
	})
	t.Run("app-mail-attach", func(t *testing.T) {
		// The attach sheets (С30): each frame its own state, a sheet of what
		// is attached. A record is searched for in all collections of its
		// kind, the first found chosen by its own link — the search kept.
		app := newMockupApp(t)
		app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
		// The letter over the book read, done: the sheets stand in its place.
		book := combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
		app.h.do(get("/app/mail/compose?back="+url.QueryEscape("/app/contacts?task="+book.Poll.TaskID), app.sess))
		sheet := func(q url.Values) view.MailAttachSheet {
			t.Helper()
			return decode[view.MailAttachSheet](t, app.h.do(get("/app/mail/attach?"+q.Encode(), app.sess)))
		}
		// A contact found by «белов» in two books, the first chosen —
		// «All» kept (s-mail-12); a note among all three of the set, the
		// first chosen (s-mail-5).
		for _, one := range []struct{ state, what, q string }{{"normal", "contact", "белов"}, {"note", "note", ""}} {
			found := sheet(url.Values{"what": {one.what}, "q": {one.q}})
			if len(found.Rows) == 0 {
				t.Fatalf("%s: nothing found by %q", one.what, one.q)
			}
			app.compare(t, "app-mail-attach/"+one.state+".json", app.h.do(get(found.Rows[0].Href, app.sess)))
		}
		app.compare(t, "app-mail-attach/calendar.json", app.h.do(get("/app/mail/attach?"+url.Values{"what": {"calendar"},
			"a": {app.realAccount(t, "lichnyj")}, "c": {app.realPath(t, "lichnyj")}, "span": {"dates"}, "from": {"2026-08-10"}, "to": {"2026-08-31"}}.Encode(), app.sess)))
		// The storage of s-mail-6, -13 on this stand alone: one storage on
		// the server, a sibling folder of the one open, the estimate's
		// folder with its three files, the attachments' six files under
		// their month.
		docs := url.Values{"a": {app.realAccount(t, "Documents")}, "c": {app.realPath(t, "Documents")}}
		editBlob(t, app.h, app.sess, func(b *sealed.Blob) {
			for i, c := range b.Collections {
				if c.Path == app.realPath(t, "Public") {
					b.Collections = append(b.Collections[:i], b.Collections[i+1:]...)
					break
				}
			}
		})
		sftp, root := app.servers["sftpgo · vps"], app.realPath(t, "Documents")
		sftp.SeedFolder(root + "Личное/")
		// The frames' ceiling of an attachment, 2 MB: the estimate goes as a
		// link (s-mail-6).
		if err := app.h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
			st.Settings.MailMaxAttachMB = 2
			return nil, nil
		}); err != nil {
			t.Fatal(err)
		}
		for name, size := range map[string]int{"carrel/Подряд/акт-смета-подрядчика.pdf": 4400 << 10, "carrel/Подряд/договор-подряда.pdf": 840 << 10,
			"carrel/Подряд/объект-фасад.jpg": 1600 << 10} {
			sftp.SeedFile(root+name, strings.Repeat(".", size))
			sftp.SetModified(root+name, mockupNow(t))
		}
		app.compare(t, "app-mail-attach/file.json", app.h.do(get("/app/mail/attach?"+url.Values{"what": {"file"}, "a": docs["a"], "c": docs["c"],
			"dir": {"carrel/Подряд"}, "path": {"carrel/Подряд/акт-смета-подрядчика.pdf"}}.Encode(), app.sess)))
		// A folder of a folder of Documents with a folder and six files in
		// it, the first of its folder, as s-mail-13 draws «Договор подряда»:
		// the attachments' folders are by month since note 180, a level
		// deeper than the frame's.
		deal := root + "carrel/agreement/"
		sftp.SeedFolder(deal + "Сканы/")
		sftp.SetModified(deal+"Сканы", mockupNow(t))
		for name, size := range map[string]int{"договор-подряда.pdf": 840 << 10, "приложение-1-смета.pdf": 4200 << 10, "приложение-2-график.pdf": 210 << 10,
			"акт-1.pdf": 180 << 10, "реквизиты.txt": 2 << 10, "объект-1.jpg": 1600 << 10} {
			sftp.SeedFile(deal+name, strings.Repeat(".", size))
			sftp.SetModified(deal+name, mockupNow(t))
		}
		app.compare(t, "app-mail-attach/folder.json", app.h.do(get("/app/mail/attach?"+url.Values{"what": {"folder"}, "a": docs["a"], "c": docs["c"],
			"dir": {"carrel/agreement"}}.Encode(), app.sess)))
	})
}

// C-14.2: the two steps a wipe link leads to, and the conflict of the three
// kinds beside the contact's.
func TestFixtureResetAndConflicts(t *testing.T) {
	t.Run("forgot-token", func(t *testing.T) {
		app := &mockupApp{h: newHarness(t), byAccount: map[string]string{}, byPath: map[string]string{}}
		app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
		app.h.srv.SetBuild(mockupVersion, mockupCommit)
		adminID := app.h.admin("mix", "password123")
		app.h.invite(adminID, "vera", "verapass123")
		// The administrator's reset no longer issues a link (s-admin-13):
		// the two steps are reached from the console, as S-10.4 has them.
		link, err := app.h.users.IssueConsoleWipeLink("vera")
		if err != nil {
			t.Fatal(err)
		}
		app.compare(t, "forgot-token/normal.json", app.h.do(get(link, nil)))
	})
}

// C-14.3: the steps of the pages outside the shell that have frames of their
// own (STATE_FRAME of check.py): the letter of a registration gone
// (s-auth-3), the four answers of /forgot (s-auth-6 to s-auth-9), the new
// password after a wipe (s-auth-11), and the address confirmed (rest-2), in
// the frames' data: mix, v@example.org. Each needs an instance of its own.
func TestFixtureStepsOutsideTheShell(t *testing.T) {
	fresh := func(t *testing.T) *mockupApp {
		app := &mockupApp{h: newHarness(t), byAccount: map[string]string{}, byPath: map[string]string{}}
		app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
		app.h.srv.SetBuild(mockupVersion, mockupCommit)
		return app
	}
	t.Run("register-sent", func(t *testing.T) {
		app := fresh(t)
		app.h.admin("mix", "password123")
		openRegistration(t, app.h)
		app.compare(t, "register/sent.json", app.h.do(app.h.anonPostForm("/register", "/register",
			url.Values{"login": {"viktor"}, "email": {"v@example.org"}, "password": {"password123"}, "password2": {"password123"}})))
	})
	t.Run("forgot", func(t *testing.T) {
		app := fresh(t)
		// mix has a confirmed address and deposited the key: both options.
		app.h.enableEscrow("alice", "password123", "sixteen-char-pass!")
		snap := app.h.st.Snapshot()
		app.h.invite(snap.UserByLogin("alice").ID, "mix", "mixpass123")
		app.h.confirmAddress("mix", "mix@example.org")
		app.h.relayed()
		ask := func(form url.Values) *httptest.ResponseRecorder {
			return app.h.do(app.h.anonPostForm("/forgot", "/forgot", form))
		}
		options := ask(url.Values{"login": {"mix"}})
		app.compare(t, "forgot/options.json", options)
		page := decode[view.ForgotPage](t, options)
		if len(page.Options) != 2 {
			t.Fatalf("options = %+v, want the wipe and the administrator", page.Options)
		}
		app.compare(t, "forgot/no-mail.json", ask(url.Values{"login": {"nobody"}}))
		app.compare(t, "forgot/wipe-sent.json", ask(formFields(page.Options[0].Form)))
		app.compare(t, "forgot/asked.json", ask(formFields(page.Options[1].Form)))
	})
	t.Run("forgot-token-password", func(t *testing.T) {
		app := fresh(t)
		app.h.admin("mix", "password123")
		link, err := app.h.users.IssueConsoleWipeLink("mix")
		if err != nil {
			t.Fatal(err)
		}
		confirm := decode[view.ForgotWipePage](t, app.h.do(get(link, nil)))
		app.compare(t, "forgot-token/password.json", app.h.do(app.h.anonPostForm(link, link, formFields(confirm.Form))))
	})
	t.Run("confirm-email", func(t *testing.T) {
		app := fresh(t)
		app.h.admin("mix", "password123")
		openRegistration(t, app.h)
		token, err := app.h.users.Register(context.Background(), "viktor", "v@example.org", "password123")
		if err != nil {
			t.Fatal(err)
		}
		app.compare(t, "confirm-email/normal.json", app.h.do(get("/confirm-email/"+token, nil)))
	})
}

// openList opens a combined list of the set and waits for its poll, as a
// person looking at it would.
func openList(t *testing.T, app *mockupApp, path string) {
	t.Helper()
	switch {
	case strings.HasPrefix(path, "/app/calendar"):
		combined(t, app.h, app.sess, path, func(p view.AgendaPage) *view.PollBar { return p.Poll })
	case strings.HasPrefix(path, "/app/tasks"):
		combined(t, app.h, app.sess, path, func(p view.TasksPage) *view.PollBar { return p.Poll })
	case strings.HasPrefix(path, "/app/notes"):
		combined(t, app.h, app.sess, path, func(p view.NotesPage) *view.PollBar { return p.Poll })
	default:
		combined(t, app.h, app.sess, path, contactsPoll)
	}
}

// resetLink digs the wipe link out of the answer that issued it.
// C-14.2: the conflict of an event and of a task. The screen is one for
// every kind (C-2.1), and each is checked on its own record.
func TestFixtureConflictsOfEveryKind(t *testing.T) {
	// Each as its frame (Рв11): the record of the set, opened, edited, and
	// meanwhile changed on the server — s-conflict-3: SUMMARY both changed,
	// LOCATION only in the edit, DTSTART moved alike on both sides;
	// s-conflict-4: SUMMARY both changed, DUE only on the server, STATUS
	// alike.
	for _, one := range []struct {
		dir, segment, list, slug, uid, file string
		base, server                        func(string) string
		edit                                map[string]string
	}{
		{"app-conflict-event", "calendar", "/app/calendar?from=2026-08-10", "lichnyj", "8f1c-4a02-b7d3", "nextcloud-home/lichnyj.ics",
			func(r string) string { return strings.Replace(r, "LOCATION:Переговорная 2\n", "", 1) },
			func(r string) string {
				r = strings.Replace(r, "SUMMARY:Синк-ап по релизу\n", "SUMMARY:Синк-ап перенесён\n", 1)
				return strings.Replace(r, ":20260810T120000\n", ":20260810T123000\n", 1)
			},
			map[string]string{"summary": "Синк-ап по релизу и демо", "location": "Переговорная 2", "start.time": "12:30"}},
		{"app-conflict-task", "tasks", "/app/tasks", "dela", "aa10-77bc-0e31", "nextcloud-home/dela.ics",
			func(r string) string { return strings.Replace(r, "DUE;VALUE=DATE:20260807\n", "", 1) },
			func(r string) string {
				r = strings.Replace(r, "SUMMARY:Отправить документы в МФЦ\n", "SUMMARY:Документы в МФЦ\nDUE:20260814T180000\nSTATUS:IN-PROCESS\n", 1)
				return r
			},
			map[string]string{"summary": "Отправить документы в МФЦ до пятницы", "status": "IN-PROCESS"}},
	} {
		t.Run(one.dir, func(t *testing.T) {
			app := newMockupApp(t)
			// Nobody arrives at a conflict from nowhere: the list was
			// opened first, and that is where the counts of the rail come
			// from (s-conflict-3, s-conflict-4), as for a contact.
			openList(t, app, one.list)
			at := davtest.CalendarHome + one.slug + "/" + one.uid + ".ics"
			base := one.base(setRecord(t, one.file, one.uid))
			app.servers["nextcloud"].Seed(at, base)
			rec := app.listPath(t, one.segment, one.slug) + "/" + one.uid
			page := decode[struct {
				Form      view.Form           `json:"form"`
				Attendees *view.AttendeeField `json:"attendees"`
			}](t, app.h.do(get(rec+"/edit", app.sess)))
			form := formFields(page.Form)
			// The people go back as a client sends them, each ticked.
			if a := page.Attendees; a != nil {
				for _, row := range a.Rows {
					form.Add(a.Name, row.Value)
				}
			}
			app.servers["nextcloud"].Seed(at, one.server(base))
			for k, v := range one.edit {
				form.Set(k, v)
			}
			if resp := app.h.do(postForm(rec+"/edit", app.sess, form)); resp.Code != 303 {
				t.Fatalf("POST over a moved version: %d %s", resp.Code, resp.Body.String())
			}
			app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
			app.compare(t, one.dir+"/normal.json", app.h.do(get(rec+"/conflict", app.sess)))
		})
	}
}

// setRecord is a record of the set as its own document, by its UID.
func setRecord(t *testing.T, file, uid string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(mockupRoot(t), filepath.FromSlash(file)))
	if err != nil {
		t.Fatal(err)
	}
	for _, one := range davtest.SplitRecords(string(body)) {
		if strings.Contains(one, "UID:"+uid+"\n") {
			return one
		}
	}
	t.Fatalf("%s is not in %s", uid, file)
	return ""
}

// C-14.2: the preview a photo is cropped in (s-contacts-5). It exists only
// while a photo is being added, so the screen is taken right after one is.
func TestFixturePhotoPreview(t *testing.T) {
	app := newMockupApp(t)
	// The lists were opened before the card, as a person opens them, and
	// that is where the counts of the rail come from.
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	// Анна keeps a photo, as s-contacts-6 and -3 draw her: the one chosen
	// replaces it, and Remove photo is offered (s-contacts-5).
	mockupAnnaPhoto(t, app)
	rec := app.listPath(t, "contacts", "kontakty") + "/4f2b-9c31-a0e7"
	add := actionHref(t, decode[view.ContactEditPage](t, app.h.do(get(rec+"/edit", app.sess))).Photo.Actions, "photo.replace")
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
		// The combined calendar was opened first: the rail of one calendar
		// carries the section's tags, whatever its own week holds (states-15).
		// The frame draws «Общий» answering, so here its password fits.
		app.servers["baikal"].Accept("REPORT")
		combined(t, app.h, app.sess, "/app/calendar?from=2026-08-10", func(p view.AgendaPage) *view.PollBar { return p.Poll })
		app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
		// A week of the set that holds nothing: an agenda is empty by the
		// window it shows, not by the calendar being empty. «Синк-ап по
		// релизу» repeats every Monday until 28 December, so the week is
		// after it (the client's scan Б16: the week of 2 November held its
		// Monday, and the «empty» state was not).
		week := app.listPath(t, "calendar", "lichnyj") + "?from=2027-01-04"
		page := decode[view.AgendaPage](t, app.h.do(get(week, app.sess)))
		if page.Bar.Loaded != 0 {
			t.Errorf("the empty week holds %d events", page.Bar.Loaded)
		}
		for _, d := range page.Days {
			if len(d.Rows) > 0 {
				t.Errorf("the empty week has rows on %s: %+v", d.Rubric, d.Rows)
			}
		}
		app.compare(t, "app-agenda/empty.json", app.h.do(get(week, app.sess)))
	})
	for _, one := range []struct{ dir, segment, slug string }{
		{"app-tasks", "tasks", "dela"},
		{"app-notes", "notes", "zapisi"},
	} {
		t.Run(one.dir+"-source-error", func(t *testing.T) {
			app := newMockupApp(t)
			// The section was read before its server stopped answering: the
			// rail keeps its tags over the list that is unavailable
			// (states-22, -29).
			openList(t, app, "/app/"+one.segment)
			refuseReads(app.servers["nextcloud"])
			defer acceptReads(app.servers["nextcloud"])
			app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
			app.compare(t, one.dir+"/source-error.json",
				app.h.do(get(app.listPath(t, one.segment, one.slug), app.sess)))
		})
	}
	t.Run("app-conflict-note", func(t *testing.T) {
		app := newMockupApp(t)
		openList(t, app, "/app/notes") // the counts of s-conflict-5, as above
		rec := app.listPath(t, "notes", "zapisi") + "/5c81-77aa-0d64"
		// The edit of a note is its page (s-notes-20); the form is its
		// document.
		doc := decode[view.NotePage](t, app.h.do(get(rec+"/edit", app.sess))).Doc
		save, err := url.Parse(actionHref(t, doc.Bar, "record.save"))
		if err != nil {
			t.Fatal(err)
		}
		// Everything the form holds goes back, as a client sends it: a
		// field left out is a field cleared, and the conflict would then
		// be about a date nobody touched.
		// As s-conflict-5 (Рв11): the title and the text both changed on
		// both sides, the tags alike.
		form := url.Values{
			"summary":     {"Заметка со встречи 10.08 — дополнено"},
			"description": {"Договорились: демо в пятницу, релиз в понедельник."},
			"categories":  {"встречи, релиз, итоги"},
		}
		for _, row := range doc.Fields {
			switch {
			case row.Name == "" || row.Name == "summary" || row.Name == "description" || row.Name == "categories":
			case row.Multiple:
				// Every relation chosen is a value (C-6.8).
				for _, o := range row.Options {
					if o.Selected {
						form.Add(row.Name, o.Value)
					}
				}
			default:
				form.Set(row.Name, string(row.Value))
			}
		}
		for k, v := range save.Query() {
			form[k] = v
		}
		server := setRecord(t, "nextcloud-home/zapisi.ics", "5c81-77aa-0d64")
		server = strings.Replace(server, "SUMMARY:Заметка со встречи 10.08\n", "SUMMARY:Встреча 10.08\n", 1)
		server = strings.Replace(server, "CATEGORIES:встречи,релиз\n", "CATEGORIES:встречи,релиз,итоги\n", 1)
		server = regexp.MustCompile(`(?m)^DESCRIPTION:.*\n( .*\n)*`).ReplaceAllString(server, "DESCRIPTION:Договорились: демо в пятницу.\n")
		app.servers["nextcloud"].Seed(davtest.CalendarHome+"zapisi/5c81-77aa-0d64.ics", server)
		if resp := app.h.do(postForm(save.EscapedPath(), app.sess, form)); resp.Code != 303 {
			t.Fatalf("POST over a moved version: %d %s", resp.Code, resp.Body.String())
		}
		app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
		app.compare(t, "app-conflict-note/normal.json", app.h.do(get(rec+"/conflict", app.sess)))
	})
}

// seriesExceptions gives «Синк-ап по релизу» a day left out and a meeting
// moved, as states-36 names them: 24 August skipped, 31 August at 1
// September, 15:00. The set's file holds neither: no other frame draws them.
func seriesExceptions(t *testing.T, app *mockupApp) {
	t.Helper()
	nc := app.servers["nextcloud"]
	p := davtest.CalendarHome + "lichnyj/8f1c-4a02-b7d3.ics"
	body, _, ok := nc.Object(p)
	if !ok {
		t.Fatal("the event is not on the stand")
	}
	rule := "RRULE:FREQ=WEEKLY;BYDAY=MO;UNTIL=20261228T235959Z"
	if !strings.Contains(body, rule) {
		t.Fatal("the series is not the set's: read the set again")
	}
	moved := strings.Join([]string{"BEGIN:VEVENT", "UID:8f1c-4a02-b7d3", "DTSTAMP:20260807T121500Z",
		"RECURRENCE-ID;TZID=Asia/Yekaterinburg:20260831T120000", "DTSTART;TZID=Asia/Yekaterinburg:20260901T150000",
		"DTEND;TZID=Asia/Yekaterinburg:20260901T160000", "SUMMARY:Синк-ап по релизу", "END:VEVENT", "END:VCALENDAR"}, "\r\n")
	body = strings.Replace(body, rule, rule+"\r\nEXDATE;TZID=Asia/Yekaterinburg:20260824T120000", 1)
	body = strings.Replace(body, "END:VCALENDAR", moved, 1)
	nc.Seed(p, body)
}

// privateProperties gives «Синк-ап по релизу» the two properties of other
// clients states-17 names as not kept: a private comment of Apple's
// calendar server and Outlook's busy status. The set's file holds neither:
// no other frame draws them.
func privateProperties(t *testing.T, app *mockupApp) {
	t.Helper()
	nc := app.servers["nextcloud"]
	p := davtest.CalendarHome + "lichnyj/8f1c-4a02-b7d3.ics"
	body, _, ok := nc.Object(p)
	if !ok {
		t.Fatal("the event is not on the stand")
	}
	at := strings.Index(body, "END:VEVENT")
	if at < 0 {
		t.Fatal("the event is not the set's: read the set again")
	}
	nc.Seed(p, body[:at]+"X-CALENDARSERVER-PRIVATE-COMMENT:взять ноутбук\r\nX-MICROSOFT-CDO-BUSYSTATUS:BUSY\r\n"+body[at:])
}

// C-14.3: what a server drops, on the forms of the other kinds, and the
// combined views while their poll is still running.
func TestFixtureLossAndPartialOfTheOtherKinds(t *testing.T) {
	for _, one := range []struct {
		dir, segment, slug, uid string
		drop                    []string
		state                   string                             // the fixture's state: loss, unless named
		prep                    func(t *testing.T, app *mockupApp) // what the record holds beyond the set
	}{
		// The two properties states-17 names, which the event carries
		// on the stand and the server does not keep; the rest is kept.
		{"app-event-edit", "calendar", "lichnyj", "8f1c-4a02-b7d3", []string{"X-CALENDARSERVER-PRIVATE-COMMENT", "X-MICROSOFT-CDO-BUSYSTATUS"}, "", privateProperties},
		{"app-task-edit", "tasks", "dela", "aa10-77bc-0e31", []string{"X-APPLE-SORT-ORDER", "X-OC-HIDESUBTASKS"}, "", nil},
		// The days of a series the server did not keep, said as days
		// (states-36, from-typecase note 62): the meeting of 24 August left
		// out, the one of 31 August moved to 1 September, 15:00.
		{"app-event-edit", "calendar", "lichnyj", "8f1c-4a02-b7d3", []string{"EXDATE", "RECURRENCE-ID"}, "loss-days", seriesExceptions},
	} {
		state := "loss"
		if one.state != "" {
			state = one.state
		}
		t.Run(one.dir+"-"+state, func(t *testing.T) {
			app := newMockupApp(t)
			if one.prep != nil {
				one.prep(t, app)
			}
			// The form was opened from the list, as in its frame: the names
			// of the related records and the numbers of the rail are known
			// (the task is related to an event).
			openList(t, app, "/app/calendar")
			if one.segment != "calendar" {
				openList(t, app, "/app/"+one.segment)
			}
			event := one.dir == "app-event-edit"
			if event {
				// The books and the notes were read too, as for the form
				// of s-agenda-3: the attendees carry the marks of their
				// books, Related to the name of its note.
				openList(t, app, "/app/contacts")
				openList(t, app, "/app/notes")
				// The change the person made is one the frame cannot
				// show: the meeting moved to «Переговорная 2» — the room
				// states-17 and -36 draw — from «Переговорная 1».
				nc := app.servers["nextcloud"]
				p := davtest.CalendarHome + one.slug + "/" + one.uid + ".ics"
				body, _, ok := nc.Object(p)
				if !ok || !strings.Contains(body, "LOCATION:Переговорная 2\r\n") {
					t.Fatal("the event is not the set's: read the set again")
				}
				nc.Seed(p, strings.Replace(body, "LOCATION:Переговорная 2\r\n", "LOCATION:Переговорная 1\r\n", 1))
			}
			rec := app.listPath(t, one.segment, one.slug) + "/" + one.uid
			form := formFields(decode[struct {
				Form view.Form `json:"form"`
			}](t, app.h.do(get(rec+"/edit", app.sess))).Form)
			app.servers["nextcloud"].Drop = one.drop
			if event {
				form.Set("location", "Переговорная 2")
			} else {
				form.Set("summary", string(form.Get("summary"))+" ·")
			}
			app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
			app.compare(t, one.dir+"/"+state+".json", app.h.do(postForm(rec+"/edit", app.sess, form)))
		})
	}
	// One collection is held while the others answer: «Проекты» of the
	// calendar on its stand (states-13), «Работа» of the tasks and
	// «Дневник» of the notes on nextcloud (states-19, -26). A hold is on
	// the collection's path, so the other lists of the same stand answer.
	for _, one := range []struct{ dir, segment, server, held, stamp string }{
		{"app-agenda-all", "calendar", "radicale · vps", "proekty", "radicale · vps"},
		{"app-tasks-all", "tasks", "nextcloud", "rabota", "nextcloud · Работа"},
		{"app-notes-all", "notes", "nextcloud", "dnevnik", "nextcloud · Дневник"},
	} {
		t.Run(one.dir+"-partial", func(t *testing.T) {
			app := newMockupApp(t)
			// The person has looked at every calendar before, an hour ago:
			// the session knows how many events each holds, and the
			// sections of the rail keep those numbers while a poll runs
			// (states-13); what it read then is too old to answer from.
			app.h.srv.SetClock(mockupNow(t).Add(-time.Hour))
			openList(t, app, "/app/"+one.segment)
			app.h.srv.SetClock(mockupNow(t))
			// One collection alone is held: «Дежурства», on the stand of
			// «Проекты», answers (states-13), and so do «Дела» beside
			// «Работа» and «Записи» beside «Дневник».
			release := app.servers[one.server].StallAt(app.realPath(t, one.held))
			defer release()
			first := app.h.do(get("/app/"+one.segment, app.sess))
			task := taskOf(t, first)
			if task == "" {
				t.Skip("this kind polls nothing on the other stand")
			}
			// The steady point: nobody is still in the queue, at least one
			// source is held, and at least one has answered with its count.
			// Everything that could answer has.
			//
			// «One has answered» is not a nicety. Without it the point is
			// also reached the moment the poll starts — nobody queued yet,
			// the held source already querying — and the screen taken then
			// is empty, not partial. Alone the test still passed; in a full
			// run, under load, it took that moment and the fixture rightly
			// said no (23.09.2026).
			waitPoll(t, app.h, app.sess, task, func(b view.PollBar) bool {
				held, answered := false, false
				for _, sp := range b.Sources {
					switch sp.State {
					case view.SourceWaiting:
						return false
					case view.SourceQuerying:
						// Held is the stalled server's; any other source
						// still querying is on its way and would be done
						// in one run and not in the next. With S-4.2 every
						// source asks the change tag before its read, and
						// the steady point that took any querying source
						// for a held one was taken early (24.09.2026).
						if !strings.HasPrefix(string(sp.Stamp), one.stamp) {
							return false
						}
						held = true
					case view.SourceDone:
						// A source that answered but has not yet given
						// its count is still arriving: its rows would
						// come and go between two runs.
						if sp.Count == nil {
							return false
						}
						answered = true
					}
				}
				return held && answered
			})
			app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
			if one.segment == "tasks" {
				// «Продлить страховку» just marked done from its row, as
				// in s-tasks-1: it stays under Open, crossed (states-19,
				// -20).
				tickMockupTask(t, app, decode[view.TasksPage](t, app.h.do(get("/app/tasks?task="+task, app.sess))), "Продлить страховку")
			}
			app.compare(t, one.dir+"/partial.json", app.h.do(get("/app/"+one.segment+"?task="+task, app.sess)))
		})
	}
}

// refuseReads makes a stand refuse what a list is read by: the question
// whether it changed, which a session that holds the list asks first
// (S-4.2), and the read itself. acceptReads takes the refusal back.
func refuseReads(srv *davtest.Server) {
	for _, m := range []string{"PROPFIND", "REPORT"} {
		srv.Refuse(m, 503, "the server is busy")
	}
}

func acceptReads(srv *davtest.Server) {
	for _, m := range []string{"PROPFIND", "REPORT"} {
		srv.Accept(m)
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

// An issued secret is random, and a run of it can look like an identifier
// (u-…, t-…). The secrets are folded first, whole, or a token that happens to
// hold such a run is shortened by the identifier's rule and no longer
// recognised: the fixture then shows the token itself, one run in many.
func TestNormaliseFoldsASecretBeforeItsInsides(t *testing.T) {
	a := &mockupApp{}
	token := "KvG1XJ8VuA0K4PAqez53u-ABCDEFGHJKiuu-9xLZMQ"
	token += strings.Repeat("x", 43-len(token))
	body := `{"csrf":"` + token + `","invite":"/invite/` + token + `","task":"t-ABCDEFGHJK"}`
	got := a.normalise(t, []byte(body))
	if got["csrf"] != "csrf" || got["invite"] != "/invite/invite-token" || got["task"] != "t-task" {
		t.Errorf("normalised = %v", got)
	}
}

// C-14.2 and C-14.3 (Им19 of the scan of import): the wizard on each of its
// steps, by kind — the preview of contacts (s-io-5), of events (s-io-6),
// of tasks (s-io-7) and of notes (s-io-8), the report (s-io-9) and the
// report with what was skipped (s-io-14), the first step of notes with its
// tree (s-io-4). Each kind's file holds what its frame draws: a record
// already here, one that cannot be read, one of another kind.
func TestFixtureImport(t *testing.T) {
	app := newMockupApp(t)
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	run := func(t *testing.T, preview *httptest.ResponseRecorder) *httptest.ResponseRecorder {
		t.Helper()
		var p view.ImportPage
		if err := json.Unmarshal(preview.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return app.h.do(postForm(actionHref(t, p.Actions, "import.run"), app.sess, url.Values{}))
	}

	t.Run("app-contact-import", func(t *testing.T) {
		book := app.listPath(t, "contacts", "kontakty")
		const vcf = "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:7c1e-2b40-9a11\r\nFN:Ирина Соколова\r\nN:Соколова;Ирина;;;\r\nTEL;TYPE=cell:+7 921 300-11-22\r\nEMAIL:i.sokolova@example.org\r\nEND:VCARD\r\n" +
			"BEGIN:VCARD\r\nVERSION:3.0\r\nUID:4f2b-9c31-a0e7\r\nFN:Анна Ковалева\r\nN:Ковалева;Анна;;;\r\nEMAIL:a.kovaleva@example.org\r\nEND:VCARD\r\n" +
			// Anna and Vera have their addresses, as the Email column of
			// s-io-5 draws them (Typecase's note 157).
			// What s-io-5 and -9 draw besides: a card named by its N in vCard
			// 2.1, one whose quoted-printable cannot be read, a UID the file
			// holds twice, a photo the server does not keep.
			"BEGIN:VCARD\r\nVERSION:2.1\r\nUID:8e20-5b1f-c7d4\r\nN;CHARSET=UTF-8;ENCODING=QUOTED-PRINTABLE:=D0=9B=D0=B5=D0=B1=D0=B5=D0=B4=D0=B5=D0=B2=D0=B0;=D0=92=D0=B5=D1=80=D0=B0;;;\r\nEMAIL:vera@example.net\r\nEND:VCARD\r\n" +
			"BEGIN:VCARD\r\nVERSION:2.1\r\nUID:3c90-aa41-0d2e\r\nNOTE;ENCODING=QUOTED-PRINTABLE:=ZZ broken\r\nEND:VCARD\r\n" +
			"BEGIN:VCARD\r\nVERSION:3.0\r\nUID:2b7e-11aa\r\nFN:Вера Дорошенко\r\nEMAIL:v.doroshenko@example.org\r\nEND:VCARD\r\n" +
			"BEGIN:VCARD\r\nVERSION:3.0\r\nUID:2b7e-11aa\r\nFN:Вера Дорошенко\r\nEMAIL:v.doroshenko@example.org\r\nTEL:+7 900 000-00-00\r\nEND:VCARD\r\n" +
			"BEGIN:VCARD\r\nVERSION:3.0\r\nUID:9f13-6be0-2a7c\r\nFN:Галина Мещерякова\r\nEMAIL:g.meshcheryakova@example.org\r\nPHOTO;ENCODING=b;TYPE=JPEG:/9j/4AAQSkZJRgABAQ==\r\nEND:VCARD\r\n"
		preview := app.h.do(importRequest(book+"/import", app.sess, false, "Разное (mix).vcf", []byte(vcf)))
		app.compare(t, "app-contact-import/preview.json", preview)
		// nextcloud keeps no photo of an imported card this once (s-io-9:
		// «Not kept by the server»).
		app.servers["nextcloud"].Drop = []string{"PHOTO"}
		report := run(t, preview)
		app.servers["nextcloud"].Drop = nil
		app.compare(t, "app-contact-import/report.json", report)
		// A file whose first line is not BEGIN:VCARD: nothing read (s-io-18).
		app.compare(t, "app-contact-import/refused.json", app.h.do(importRequest(book+"/import", app.sess, false, "Разное (mix).vcf",
			[]byte("PRODID:-//Other client//EN\r\n"+vcf))))
		// Dropped on the combined list: the first step with the file in hand
		// and «Into» empty (s-io-17).
		app.compare(t, "app-contacts-import/normal.json", app.h.do(importRequest("/app/contacts/import", app.sess, false, "Разное (mix).vcf", []byte(vcf))))
	})
	t.Run("app-event-import", func(t *testing.T) {
		cal := app.listPath(t, "calendar", "lichnyj")
		const ics = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Other client//EN\r\n" +
			"BEGIN:VEVENT\r\nUID:e7a1-standup\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:Планёрка\r\nDTSTART;TZID=Europe/Moscow:20260810T090000\r\nDTEND;TZID=Europe/Moscow:20260810T093000\r\nRRULE:FREQ=WEEKLY;BYDAY=MO;UNTIL=20261228T235959Z\r\nEND:VEVENT\r\n" +
			"BEGIN:VEVENT\r\nUID:e7a1-otpusk\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:Отпуск\r\nDTSTART;VALUE=DATE:20260713\r\nDTEND;VALUE=DATE:20260727\r\nEND:VEVENT\r\n" +
			// A day that is not, written as no client writes it (note 115,
			// s-io-6): its DTSTART on line 23.
			"BEGIN:VEVENT\r\nUID:e7a1-fevral\r\nDTSTAMP:20260801T000000Z\r\nDTEND;VALUE=DATE:20260301\r\nDTSTART;VALUE=DATE:2026-02-30\r\nEND:VEVENT\r\n" +
			// Already in «Личный»: the question about the records already
			// here stands beside «Into» (s-io-6).
			"BEGIN:VEVENT\r\nUID:8f1c-4a02-b7d3\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:Синк-ап по релизу\r\nDTSTART:20260810T110000Z\r\nDTEND:20260810T113000Z\r\nEND:VEVENT\r\n" +
			"BEGIN:VEVENT\r\nUID:e7a1-ekb\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:Созвон с Екатеринбургом\r\nDTSTART;TZID=Asia/Yekaterinburg:20260817T100000\r\nDTEND;TZID=Asia/Yekaterinburg:20260817T110000\r\nEND:VEVENT\r\n" +
			"BEGIN:VTODO\r\nUID:e7a1-bilety\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:Купить билеты\r\nEND:VTODO\r\n" +
			"BEGIN:VEVENT\r\nUID:e7a1-bez-imeni\r\nDTSTAMP:20260801T000000Z\r\nDTSTART;VALUE=DATE:20260504\r\nEND:VEVENT\r\n" +
			"END:VCALENDAR\r\n"
		// The file of the report (s-io-14): an event made, one already here,
		// one whose rule cannot be read, a task the calendar does not take.
		const reported = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Other client//EN\r\n" +
			"BEGIN:VEVENT\r\nUID:e7a1-kazan\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:Поездка в Казань\r\nDTSTART;VALUE=DATE:20260904\r\nDTEND;VALUE=DATE:20260907\r\nEND:VEVENT\r\n" +
			"BEGIN:VEVENT\r\nUID:8f1c-4a02-b7d3\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:Синк-ап по релизу\r\nDTSTART:20260810T110000Z\r\nDTEND:20260810T113000Z\r\nEND:VEVENT\r\n" +
			"BEGIN:VEVENT\r\nUID:e7a1-dvuhned\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:Разбор недели\r\nDTSTART:20260814T150000Z\r\nRRULE:FREQ=FORTNIGHTLY\r\nEND:VEVENT\r\n" +
			"BEGIN:VTODO\r\nUID:e7a1-bilety\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:Купить билеты\r\nEND:VTODO\r\n" +
			"END:VCALENDAR\r\n"
		// The first step of a calendar (s-io-15), the calendars read: the
		// rail counts them.
		openList(t, app, "/app/calendar")
		app.compare(t, "app-event-import/normal.json", app.h.do(get(cal+"/import", app.sess)))
		preview := app.h.do(importRequest(cal+"/import", app.sess, false, "Личный 2026.ics", []byte(ics)))
		app.compare(t, "app-event-import/preview.json", preview)
		app.compare(t, "app-event-import/report.json", run(t, app.h.do(importRequest(cal+"/import", app.sess, false, "Личный 2026.ics", []byte(reported)))))
	})
	t.Run("app-task-import", func(t *testing.T) {
		tasks := app.listPath(t, "tasks", "dela")
		const ics = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Other client//EN\r\n" +
			// Already in «Дела»: the question about the records already here
			// stands beside «Into» (s-io-7).
			"BEGIN:VTODO\r\nUID:aa10-77bc-0e31\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:Отправить документы в МФЦ\r\nDUE;VALUE=DATE:20260807\r\nEND:VTODO\r\n" +
			"BEGIN:VTODO\r\nUID:t9b2-kniga\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:Дочитать книгу\r\nPERCENT-COMPLETE:40\r\nPRIORITY:5\r\nEND:VTODO\r\n" +
			"BEGIN:VTODO\r\nUID:t9b2-smeta\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:Ответить подрядчику по смете\r\nEND:VTODO\r\n" +
			"BEGIN:VTODO\r\nUID:t9b2-pusto\r\nDTSTAMP:20260801T000000Z\r\nDUE;VALUE=DATE:20260828\r\nEND:VTODO\r\n" +
			"BEGIN:VEVENT\r\nUID:t9b2-podryadchik\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:Созвон с подрядчиком\r\nDTSTART;VALUE=DATE:20260817\r\nEND:VEVENT\r\n" +
			"BEGIN:VTODO\r\nUID:t9b2-otchet\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:Сдать отчёт\r\nDUE;VALUE=DATE:2026-13-01\r\nEND:VTODO\r\n" +
			"END:VCALENDAR\r\n"
		// The first step of a list of tasks (s-io-16), the lists read.
		openList(t, app, "/app/tasks")
		app.compare(t, "app-task-import/normal.json", app.h.do(get(tasks+"/import", app.sess)))
		app.compare(t, "app-task-import/preview.json", app.h.do(importRequest(tasks+"/import", app.sess, false, "tasks.ics", []byte(ics))))
	})
	t.Run("app-note-import", func(t *testing.T) {
		// The notebooks were read, as s-io-4 counts their notes.
		openList(t, app, "/app/notes")
		notes := app.listPath(t, "notes", "zapisi")
		// The folder chosen in the tree, a level down, as s-io-4 draws it:
		// Documents opened, «notes» chosen.
		node := func(nodes []view.FileTreeNode, label string) string {
			for _, n := range nodes {
				if n.Label == label {
					return n.Href
				}
				for _, c := range n.Children {
					if c.Label == label {
						return c.Href
					}
				}
			}
			t.Fatalf("no folder %q in %+v", label, nodes)
			return ""
		}
		app.servers["sftpgo · vps"].SeedFolder(app.realPath(t, "Documents") + "notes/")
		first := decode[view.ImportPage](t, app.h.do(get(notes+"/import", app.sess)))
		opened := decode[view.ImportPage](t, app.h.do(get(node(first.Source.Folder.Nodes, "Documents"), app.sess)))
		app.compare(t, "app-note-import/normal.json", app.h.do(get(node(opened.Source.Folder.Nodes, "notes"), app.sess)))
		// A zip of notes, as s-io-8 draws it: front matter, a heading in a
		// folder, a note already here by its UID, a file that is no
		// Markdown, an empty one; the files' times as the frame dates them.
		archive := zipAt(t, map[string]time.Time{
			"idei/crdt.md": time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC),
			"doroga.md":    time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC),
		},
			"2026-03-11-podryadchik.md", "---\r\ntitle: Встреча с подрядчиком\r\ndate: 2026-03-11\r\ncategories: работа\r\n---\r\n\r\nОбсудили смету и сроки.\r\n",
			"idei/crdt.md", "# Что почитать про CRDT\n\nСсылки.\n",
			"doroga.md", "---\r\ntitle: Про дорогу до дачи\r\nuid: 5c81-77aa-0d64\r\n---\r\n\r\nПоворот после моста.\r\n",
			"skan.pdf", "%PDF",
			"pustoj.md", "",
		)
		app.compare(t, "app-note-import/preview.json", app.h.do(importRequest(notes+"/import", app.sess, false, "заметки-2026.zip", archive)))
	})
	// The address book chosen does not answer the preview (s-io-19): what is
	// already there cannot be told, Import is off for it. Last: the server
	// stays refusing.
	t.Run("app-contact-import/unanswered", func(t *testing.T) {
		book := app.listPath(t, "contacts", "kontakty")
		// A card ready and one that cannot be read: the counts of s-io-19.
		const vcf = "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:7c1e-2b40-9a11\r\nFN:Ирина Соколова\r\nEMAIL:i.sokolova@example.org\r\nEND:VCARD\r\n" +
			"BEGIN:VCARD\r\nVERSION:3.0\r\nUID:8e20-5b1f-c7d4\r\nFN:Вера Лебедева\r\nEMAIL:vera@example.net\r\nEND:VCARD\r\n" +
			"BEGIN:VCARD\r\nVERSION:2.1\r\nUID:3c90-aa41-0d2e\r\nNOTE;ENCODING=QUOTED-PRINTABLE:=ZZ broken\r\nEND:VCARD\r\n"
		for _, m := range []string{"PROPFIND", "REPORT"} {
			app.servers["nextcloud"].Refuse(m, http.StatusServiceUnavailable, "")
		}
		app.compare(t, "app-contact-import/unanswered.json", app.h.do(importRequest(book+"/import", app.sess, false, "Разное (mix).vcf", []byte(vcf))))
	})
}

// sizedUpload is the upload of a page with scripts: each file's size sent
// before it (size.‹name›), except a file with no body — it goes as the page
// without scripts sends it, its size unsaid, and a storage that needs one
// refuses it (s-files-24).
func sizedUpload(path string, sess *session.Session, files ...upload) *http.Request {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("_csrf", sess.CSRFToken())
	for _, f := range files {
		body := f.body
		if body != "" {
			_ = w.WriteField("size."+f.name, strconv.Itoa(len(body)))
		} else {
			body = "zip"
		}
		part, _ := w.CreateFormFile("file", f.name)
		_, _ = part.Write([]byte(body))
	}
	_ = w.Close()
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: sess.ID()})
	return req
}

// The installation check as the rail's foot says it (s-admin-*): passed,
// failed (two, as s-admin-26 draws it), or not run since the start.
const (
	checksPassed = iota
	checksFailed
	checksNever
)

// adminRail is the state of the instance the rail's foot of a frame of the
// administration draws: mail going out or not configured, the last check
// passed, failed or not run (Typecase's 195 — s-admin-1, -6, -30…-33, -4,
// -25, -26 with mail; the rest without). Mail is a relay set in the state;
// nothing is sent.
func adminRail(t *testing.T, app *mockupApp, mail bool, check int) {
	t.Helper()
	if err := app.h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		st.Settings.SMTP = store.SMTPSettings{}
		if mail {
			st.Settings.SMTP = store.SMTPSettings{Host: "smtp.carrel.example", Port: 587, Security: "starttls",
				From: "noreply@carrel.example", Sender: "login"}
		}
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	at := mockupDayAt(t, "2026-08-16 09:12")
	var sent, checked time.Time
	failed := 0
	if mail {
		sent = at
	}
	switch check {
	case checksPassed:
		checked = at
	case checksFailed:
		checked, failed = at, 2
	}
	app.h.srv.SetRailMarks(sent, checked, failed)
}

// auditOfFrame is the journal s-admin-7 draws: a sign-in, three refused in
// a row as one line, a recovery with the master password, a change of the
// instance's settings — the newest first. The lines of making the set's
// accounts are the run's, not the frame's.
func auditOfFrame(t *testing.T, app *mockupApp) {
	t.Helper()
	at := func(day string) time.Time { return mockupDayAt(t, day) }
	if err := app.h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		st.Audit = []store.AuditEntry{
			{When: at("2026-08-12 10:02"), Who: "mix", What: "settings.instance", Detail: "smtp_host, smtp_port"},
			{When: at("2026-08-14 18:40"), Who: "mix", What: "escrow.recover", Whom: "viktor", Detail: "notice mailed"},
			{When: at("2026-08-14 22:07"), Who: "—", What: "login.fail", Whom: "anna", Detail: "3rd attempt · ip 203.0.113.9 · rate-limited"},
			{When: at("2026-08-15 09:20"), Who: "mix", What: "login.ok"},
		}
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
}

// onPaper is an address asked on paper (?print=1, U-10).
func onPaper(addr string) string {
	sep := "?"
	if strings.Contains(addr, "?") {
		sep = "&"
	}
	return addr + sep + render.PrintParam + "=" + render.PrintValue
}

// C-14.2: the page the installed application shows without a network
// (offline-1): one for everyone, asked without a session.
func TestFixtureOffline(t *testing.T) {
	app := &mockupApp{h: newHarness(t), byAccount: map[string]string{}, byPath: map[string]string{}}
	app.h.admin("mix", "password123")
	app.compare(t, "offline/normal.json", app.h.do(get(render.OfflinePath, nil)))
}
