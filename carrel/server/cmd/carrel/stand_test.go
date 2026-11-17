package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"
	"golang.org/x/net/html"

	"carrel.dev/carrel/server/internal/davtest"
)

// The acceptance stand (U-15.1; docs/visual/fixtures/README.md): the
// records of the frames on the servers of a person — two of calendars and
// books, a third whose password stops fitting, a file server and two
// feeds — and an empty instance, served with its client's pages, to which
// they are given the way a person gives them: the first run, Connect, the
// ticks of the rails, the folder for attachments, the subscriptions. Then
// it holds, and the screens are opened beside the frames.
//
// It runs only when asked: CARREL_STAND is how long to hold (2h), and
// CARREL_STAND_PORT the port, 8099 when not said; the mockup is served
// under /mockups/ of a second port, one above.
//
//	CARREL_STAND=2h go test -run TestStand -timeout 0 ./cmd/carrel
func TestStand(t *testing.T) {
	hold := os.Getenv("CARREL_STAND")
	if hold == "" {
		t.Skip("CARREL_STAND=<how long> runs the acceptance stand")
	}
	wait, err := time.ParseDuration(hold)
	if err != nil {
		t.Fatalf("CARREL_STAND: %v", err)
	}
	port := 8099
	if p := os.Getenv("CARREL_STAND_PORT"); p != "" {
		if port, err = strconv.Atoi(p); err != nil {
			t.Fatalf("CARREL_STAND_PORT: %v", err)
		}
	}
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "docs", "visual", "fixtures"))
	if err != nil {
		t.Fatal(err)
	}

	servers := map[string]*davtest.Server{}
	for _, label := range []string{"nextcloud", "radicale · vps", "baikal"} {
		srv := davtest.New(t)
		srv.Login, srv.Password = "alice", "secret"
		servers[label] = srv
	}
	for _, col := range standSet {
		srv := servers[col.account]
		path := davtest.CalendarHome + col.slug + "/"
		if col.comps == nil {
			path = davtest.AddressBookHome + col.slug + "/"
			srv.AddAddressBook(carddav.AddressBook{Path: path, Name: col.name, ReadOnly: col.readOnly})
		} else {
			srv.AddCalendar(caldav.Calendar{Path: path, Name: col.name, SupportedComponentSet: col.comps, ReadOnly: col.readOnly})
		}
		// A server lists its collections in the order they were made: the
		// rails of the frames have that order.
		srv.Ranked = append(srv.Ranked, path)
		srv.SetColour(path, col.colour)
		for _, file := range col.files {
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
			if err != nil {
				t.Fatal(err)
			}
			for i, rec := range davtest.SplitRecords(string(data)) {
				name := "rec" + string(rune('a'+i))
				if m := standUID.FindStringSubmatch(rec); m != nil {
					name = strings.TrimSpace(m[1])
				}
				ext := ".ics"
				if col.comps == nil {
					ext = ".vcf"
				}
				srv.Seed(path+url.PathEscape(name)+ext, rec)
			}
		}
	}

	// The file server: «Documents» and «Public» as the set has them.
	files := davtest.New(t)
	files.Login, files.Password = "alice", "secret"
	for _, folder := range []string{"Documents", "Public"} {
		seedFolder(t, files, davtest.FileRoot+folder+"/", filepath.Join(root, "sftpgo-vps", folder))
	}

	// The feeds, served as files; the league's behind a password that is
	// changed once it is subscribed.
	var leaguePassword atomic.Value
	leaguePassword.Store("season")
	feeds := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "league") {
			if _, pass, ok := r.BasicAuth(); !ok || pass != leaguePassword.Load() {
				w.Header().Set("WWW-Authenticate", `Basic realm="league"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		http.FileServer(http.Dir(root)).ServeHTTP(w, r)
	}))
	t.Cleanup(feeds.Close)

	t.Setenv("CARREL_TIMEZONE", "Europe/Moscow")
	b := startPlainOn(t, port, "127.0.0.1")

	b.open("/login")
	b.press("Create administrator", url.Values{"login": {standLogin}, "password": {standPassword}, "password2": {standPassword}}, nil)
	b.press("Sign in", url.Values{"login": {standLogin}, "password": {standPassword}}, nil)
	for _, label := range []string{"nextcloud", "radicale · vps", "baikal"} {
		b.open("/app/settings/connections")
		b.follow("Connect account")
		b.press("Connect", url.Values{"label": {label}, "server": {servers[label].URL}, "login": {"alice"}, "password": {"secret"}}, nil)
	}
	b.open("/app/files")
	b.follow("Connect account")
	b.press("Connect", url.Values{"label": {"sftpgo · vps"}, "server": {files.URL + davtest.FileRoot}, "login": {"alice"}, "password": {"secret"}}, nil)

	// The subscriptions, before the ticks: the league's with its
	// password, which stops fitting once the rails are ticked.
	subscribe(t, b, "Праздники РФ", feeds.URL+"/feeds/ru-holidays.ics", "")
	subscribe(t, b, "Расписание лиги", feeds.URL+"/feeds/league-season.ics", "season")

	// Every rail ticked, then the ones the frames draw off taken off: the
	// holidays stand in the calendar only, the league nowhere.
	for _, section := range []string{"contacts", "calendar", "tasks", "notes"} {
		b.open("/app/" + section)
		if b.page.find(isButton("Tick every")) != nil {
			b.press("Tick every", nil, nil)
		}
	}
	b.open("/app/files")
	if b.page.find(isButton("Tick every")) != nil {
		b.press("Tick every", nil, nil)
	}
	for _, off := range []struct{ section, name string }{
		{"contacts", "Рассылки"}, {"calendar", "Дни рождения"}, {"calendar", "Расписание лиги"},
		{"tasks", "Праздники РФ"}, {"notes", "Общие с семьёй"}, {"files", "Public"},
	} {
		b.open("/app/" + off.section)
		untick(t, b, off.name)
	}
	leaguePassword.Store("changed")

	// «Общий» on baikal: connected and ticked, and then its password stops
	// fitting (README, step 6).
	servers["baikal"].Refuse("REPORT", 401, "")

	mock := httptest.NewUnstartedServer(http.StripPrefix("/mockups/", http.FileServer(http.Dir(filepath.Dir(root)))))
	if ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port+1)); err == nil {
		mock.Listener = ln
	}
	mock.Start()
	t.Cleanup(mock.Close)

	t.Logf("the stand: %s (login %s); the mockup: %s/mockups/carrel-ui-mockups.html; holding %s", b.origin, standLogin, mock.URL, wait)
	time.Sleep(wait)
}

// The stand's administrator: test values of an instance that lives for the
// run.
const (
	standLogin    = "mix"
	standPassword = "correct horse battery"
)

var standUID = regexp.MustCompile(`(?m)^UID:(.+)$`)

// standCollection is one collection of the set (the README's table): its
// server, address, name, colour, the components it holds — none for a
// book — and its files.
type standCollection struct {
	account, slug, name, colour string
	comps                       []string
	files                       []string
	readOnly                    bool
}

var standSet = []standCollection{
	{"nextcloud", "kontakty", "Контакты", "#4A6B52", nil, []string{"nextcloud-home/kontakty.vcf"}, false},
	{"nextcloud", "arhiv", "Архив", "#A05A2C", nil, []string{"nextcloud-home/arhiv.vcf"}, false},
	{"nextcloud", "lichnyj", "Личный", "#4A6B52", []string{"VEVENT"}, []string{"nextcloud-home/lichnyj.ics"}, false},
	{"nextcloud", "dela", "Дела", "#4A6B52", []string{"VTODO"}, []string{"nextcloud-home/dela.ics"}, false},
	{"nextcloud", "rabota", "Работа", "#3F6E8C", []string{"VEVENT", "VTODO"}, []string{"nextcloud-home/rabota.ics", "nextcloud-home/rabota-zadachi.ics"}, false},
	{"nextcloud", "dni-rozhdeniya", "Дни рождения", "#A05A2C", []string{"VEVENT"}, []string{"nextcloud-home/dni-rozhdeniya.ics"}, false},
	{"nextcloud", "zapisi", "Записи", "#4A6B52", []string{"VJOURNAL"}, []string{"nextcloud-home/zapisi.ics"}, false},
	{"nextcloud", "dnevnik", "Дневник", "#6B4E7A", []string{"VJOURNAL"}, []string{"nextcloud-home/dnevnik.ics"}, false},
	{"nextcloud", "zapiski-obshchie", "Общие с семьёй", "#3F6E8C", []string{"VJOURNAL"}, []string{"nextcloud-home/zapiski-obshchie.ics"}, true},
	{"radicale · vps", "komanda", "Команда", "#6B4E7A", nil, []string{"radicale-vps/komanda.vcf"}, true},
	{"radicale · vps", "rassylki", "Рассылки", "#6B4E7A", nil, nil, false},
	{"radicale · vps", "proekty", "Проекты", "#6B4E7A", []string{"VEVENT"}, []string{"radicale-vps/proekty.ics"}, false},
	{"radicale · vps", "dezhurstva", "Дежурства", "#8C6239", []string{"VEVENT"}, []string{"radicale-vps/dezhurstva.ics"}, false},
	{"baikal", "obshchij", "Общий", "#9A9280", []string{"VEVENT"}, nil, false},
}

// seedFolder puts a folder of the set on a file server, as it is on disk.
func seedFolder(t *testing.T, srv *davtest.Server, at, dir string) {
	t.Helper()
	srv.SeedFolder(at)
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || p == dir {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		target := at + filepath.ToSlash(rel)
		if d.IsDir() {
			srv.SeedFolder(target + "/")
			return nil
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		srv.SeedFile(target, string(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// untick takes a collection off its rail: the tick of its row, pressed.
func untick(t *testing.T, b *plainBrowser, name string) {
	t.Helper()
	btn := b.page.find(func(n *html.Node) bool {
		return (n.Data == "button" || n.Data == "input") && attr(n, "name") == "toggle" &&
			(strings.Contains(attr(n, "aria-label"), name) || strings.Contains(attr(n, "title"), name) || strings.Contains(words(n), name))
	})
	if btn == nil {
		t.Errorf("%s: no tick of %q; buttons: %s", b.page.url.Path, name, b.page.buttons())
		return
	}
	form := b.page.formOf(btn)
	fields := b.page.fieldsOf(form)
	fields.Set("toggle", attr(btn, "value"))
	u, _ := b.page.url.Parse(attr(form, "action"))
	if a := attr(btn, "formaction"); a != "" {
		u, _ = b.page.url.Parse(a)
	}
	req, _ := http.NewRequest(http.MethodPost, u.String(), strings.NewReader(fields.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	b.do(req)
}

// subscribe subscribes to a feed from the calendar's rail.
func subscribe(t *testing.T, b *plainBrowser, name, address, password string) {
	t.Helper()
	b.open("/app/calendar")
	b.follow("Subscribe to a feed")
	set := url.Values{"url": {address}, "name": {name}}
	if password != "" {
		set.Set("asks", "on")
		set.Set("login", "league")
		set.Set("password", password)
	}
	b.press("Subscribe", set, nil)
}

func isButton(text string) func(*html.Node) bool {
	return func(n *html.Node) bool {
		return (n.Data == "button" || n.Data == "input" && attr(n, "type") == "submit") &&
			(strings.Contains(words(n), text) || strings.Contains(attr(n, "title"), text))
	}
}
