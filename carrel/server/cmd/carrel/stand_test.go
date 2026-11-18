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
	wait := standHold(t, "CARREL_STAND")
	port, root := standPort(t), standRoot(t)

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

	// The file server: «Documents» and «Public» as the set has them, with
	// the sizes and times the frames draw (davtest.SetShapes) — the day
	// the frames were drawn, not the day the stand is started.
	files := davtest.New(t)
	files.Login, files.Password = "alice", "secret"
	for _, folder := range []string{"Documents", "Public"} {
		if err := files.SeedSet(davtest.FileRoot+folder+"/", filepath.Join(root, "sftpgo-vps", folder), folder, standDay(t)); err != nil {
			t.Fatal(err)
		}
	}

	feeds, leaguePassword := standFeeds(t, root)

	t.Setenv("CARREL_TIMEZONE", "Europe/Moscow")
	b := startPlainOn(t, port, "127.0.0.1")

	standSignIn(b)
	for _, label := range []string{"nextcloud", "radicale · vps", "baikal"} {
		b.open("/app/settings/connections")
		b.follow("Connect account")
		b.press("Connect", url.Values{"label": {label}, "server": {servers[label].URL}, "login": {"alice"}, "password": {"secret"}}, nil)
	}
	b.open("/app/files")
	b.follow("Connect account")
	b.press("Connect", url.Values{"label": {"sftpgo · vps"}, "server": {files.URL + davtest.FileRoot}, "login": {"alice"}, "password": {"secret"}}, nil)

	standRails(t, b, feeds.URL, nil)
	leaguePassword.Store("changed")

	// «Общий» on baikal: connected and ticked, and then its password stops
	// fitting (README, step 6).
	servers["baikal"].Refuse("REPORT", 401, "")

	mock := standMockup(t, port, root)
	t.Logf("the stand: %s (login %s); the mockup: %s/mockups/carrel-ui-mockups.html; holding %s", b.origin, standLogin, mock, wait)
	standWait(t, wait)
}

// standHold is how long a stand holds, from its variable (2h); a stand not
// asked for is skipped.
func standHold(t *testing.T, name string) time.Duration {
	t.Helper()
	hold := os.Getenv(name)
	if hold == "" {
		t.Skip(name + "=<how long> runs the acceptance stand")
	}
	wait, err := time.ParseDuration(hold)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return wait
}

// standPort is CARREL_STAND_PORT, 8099 when not said.
func standPort(t *testing.T) int {
	t.Helper()
	port := 8099
	if p := os.Getenv("CARREL_STAND_PORT"); p != "" {
		var err error
		if port, err = strconv.Atoi(p); err != nil {
			t.Fatalf("CARREL_STAND_PORT: %v", err)
		}
	}
	return port
}

// standRoot is the folder of the set, docs/visual/fixtures.
func standRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "docs", "visual", "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// standDay is the day the frames were drawn: the files of the set changed
// then, not the day the stand is started.
func standDay(t *testing.T) time.Time {
	t.Helper()
	zone, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	return time.Date(2026, 8, 10, 17, 4, 0, 0, zone)
}

// standFeeds serves the feeds as files; the league's behind a password that
// is changed once it is subscribed.
func standFeeds(t *testing.T, root string) (*httptest.Server, *atomic.Value) {
	t.Helper()
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

	return feeds, &leaguePassword
}

// standSignIn makes the instance's administrator on its first run and signs
// in.
func standSignIn(b *plainBrowser) {
	b.open("/login")
	b.press("Create administrator", url.Values{"login": {standLogin}, "password": {standPassword}, "password2": {standPassword}}, nil)
	b.press("Sign in", url.Values{"login": {standLogin}, "password": {standPassword}}, nil)
}

// standRails subscribes to the feeds and ticks the rails as the frames draw
// them: every rail ticked, then the ones the frames draw off taken off. A
// real account holds collections of its own beside the set's: with keep,
// every source it does not keep — by the address its tick sends and its
// name — is taken off as well.
func standRails(t *testing.T, b *plainBrowser, feeds string, keep func(value, name string) bool) {
	t.Helper()
	// The subscriptions, before the ticks: the league's with its
	// password, which stops fitting once the rails are ticked.
	subscribe(t, b, "Праздники РФ", feeds+"/feeds/ru-holidays.ics", "")
	subscribe(t, b, "Расписание лиги", feeds+"/feeds/league-season.ics", "season")

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
		// A real account may hold one of the same name of its own: the
		// set's is the one keep keeps.
		untickWhere(t, b, off.name, func(n *html.Node) bool {
			return strings.Contains(tickName(n), off.name) && (keep == nil || keep(attr(n, "value"), tickName(n)))
		})
	}
	if keep == nil {
		return
	}
	for _, section := range []string{"contacts", "calendar", "tasks", "notes", "files"} {
		b.open("/app/" + section)
		var off, names []string // the addresses the ticks send, and their names
		for _, n := range b.page.all(func(n *html.Node) bool {
			return (n.Data == "button" || n.Data == "input") && attr(n, "name") == "toggle"
		}) {
			// The row of the whole section sends the section's root; a
			// source's row its own address under it.
			if strings.Count(strings.Trim(attr(n, "value"), "/"), "/") < 2 {
				continue
			}
			if !keep(attr(n, "value"), tickName(n)) {
				off = append(off, attr(n, "value"))
				names = append(names, tickName(n))
			}
		}
		t.Logf("%s: not the set's, taken off: %q", section, names)
		for _, value := range off {
			b.open("/app/" + section)
			untickWhere(t, b, value, func(n *html.Node) bool { return attr(n, "value") == value })
		}
	}
}

// tickName is the name of the source a rail's tick is: its label, else its
// hint, else its words.
func tickName(n *html.Node) string {
	if name := attr(n, "aria-label"); name != "" {
		return name
	}
	if name := attr(n, "title"); name != "" {
		return name
	}
	return words(n)
}

// standMockup serves the mockup under /mockups/ of the port above the
// stand's and answers with its address.
func standMockup(t *testing.T, port int, root string) string {
	t.Helper()
	mock := httptest.NewUnstartedServer(http.StripPrefix("/mockups/", http.FileServer(http.Dir(filepath.Dir(root)))))
	if ln, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port+1)); err == nil {
		mock.Listener = ln
	}
	mock.Start()
	t.Cleanup(mock.Close)
	return mock.URL
}

// standWait holds the stand for wait, or until the file CARREL_STAND_STOP
// names appears: a stand stopped that way still runs its cleanups, which a
// killed one does not — and a stand on real servers takes away what it made
// on them there.
func standWait(t *testing.T, wait time.Duration) {
	stop := os.Getenv("CARREL_STAND_STOP")
	for end := time.Now().Add(wait); time.Now().Before(end); time.Sleep(time.Second) {
		if stop == "" {
			continue
		}
		if _, err := os.Stat(stop); err == nil {
			t.Logf("stopped by %s", stop)
			return
		}
	}
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

// untick takes a collection off its rail: the tick of its row, pressed.
func untick(t *testing.T, b *plainBrowser, name string) {
	t.Helper()
	untickWhere(t, b, name, func(n *html.Node) bool { return strings.Contains(tickName(n), name) })
}

// untickWhere presses the first tick of the rail match picks; what is the
// tick of which source is said as what.
func untickWhere(t *testing.T, b *plainBrowser, what string, match func(*html.Node) bool) {
	t.Helper()
	btn := b.page.find(func(n *html.Node) bool {
		return (n.Data == "button" || n.Data == "input") && attr(n, "name") == "toggle" && match(n)
	})
	if btn == nil {
		t.Errorf("%s: no tick of %q; buttons: %s", b.page.url.Path, what, b.page.buttons())
		return
	}
	// A tick turns its row over: pressed only while it is on, and looked at
	// again after — a row already off would come back on.
	at := b.page.url.RequestURI()
	for try := 0; attr(btn, "aria-pressed") != "false"; try++ {
		if try == 3 {
			t.Errorf("%s: the tick of %q stays on", at, what)
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
		b.open(at)
		if btn = b.page.find(func(n *html.Node) bool {
			return (n.Data == "button" || n.Data == "input") && attr(n, "name") == "toggle" && match(n)
		}); btn == nil {
			return
		}
	}
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
