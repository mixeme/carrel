//go:build integration

package main

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"

	"carrel.dev/carrel/server/integration"
	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/discover"
	"carrel.dev/carrel/server/view"
)

// The acceptance stand on our test servers (U-15.1): what TestStand does
// with davtest, on the servers themselves — what an imitation cannot show
// (discovery, the paths and sizes a server keeps, what it stores of a
// record) shows here. The set goes in the way README's steps by hand give
// it: Connect, New collection with the set's names, colours and components,
// Import of each file; the files of the set are put on the file servers
// first, at their sizes, for an upload of 100 MB through a form is not what
// is under test.
//
// The accounts of the set are played by our test servers. By default
// nextcloud is Nextcloud, «radicale · vps» SOGo — a second server, for one
// principal cannot be connected twice —, baikal Baikal, and «sftpgo · vps» a
// folder carrel-set of SFTPGo. CARREL_STAND_ACCOUNTS gives others, by the
// set's label and liveServers' name: «nextcloud=davis,radicale ·
// vps=radicale,baikal=stalwart,sftpgo · vps=opencloud». The Linux machine's
// stands are reached through scripts/linux-stands.sh and its tunnel. The
// set is not only for the servers it names: through Carrel's screens a
// server shows what its live suites do not (the owner, 18.11.2026). What
// the stand makes it takes away when it ends: run it with
// CARREL_STAND_STOP, and end it by making that file, not by killing it.
//
//	CARREL_LIVE_STAND=3h CARREL_STAND_STOP=<file> go test -tags integration -run TestLiveStand -timeout 0 -v ./cmd/carrel
func TestLiveStand(t *testing.T) {
	wait := standHold(t, "CARREL_LIVE_STAND")
	port, root := standPort(t), standRoot(t)
	integration.LiveCredentials()

	cast := map[string]string{"nextcloud": "nextcloud", "radicale · vps": "sogo", "baikal": "baikal", "sftpgo · vps": "sftpgo"}
	for _, pair := range strings.Split(os.Getenv("CARREL_STAND_ACCOUNTS"), ",") {
		if label, name, ok := strings.Cut(pair, "="); ok {
			cast[strings.TrimSpace(label)] = strings.TrimSpace(name)
		}
	}
	play := func(label string) account {
		a := liveServer(t, cast[label])
		a.label = label
		if strings.HasPrefix(a.server, "http://127.0.0.1:") {
			// A stand of the Linux machine through the tunnel: Carrel's
			// guard lets it be reached, as the integration suites' does.
			t.Setenv("CARREL_DAV_SSRF_ALLOWLIST", "127.0.0.1")
		}
		t.Logf("%s is %s: %s", label, a.name, a.server)
		return a
	}
	accounts := []account{play("nextcloud"), play("radicale · vps"), play("baikal")}
	files := play("sftpgo · vps")
	files.server = strings.TrimSuffix(files.server, "/") + "/carrel-set/"

	cfg, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	guard := dav.NewGuard(cfg)
	ctx := context.Background()

	// What each account holds before: the stand takes away only what is new
	// and has a name of the set.
	setNames := map[string]bool{}
	for _, col := range standSet {
		setNames[col.name] = true
	}
	var ncFiles, filesName string
	for i, a := range accounts {
		res, _, err := discover.Run(ctx, guard, a.server, a.user, a.pass, "")
		if err != nil {
			t.Fatalf("%s: discovery: %v", a.label, err)
		}
		before := map[string]bool{}
		for _, c := range res.Collections {
			before[c.Path] = true
			// «Files» of the frames (s-files-6) is the file storage of
			// nextcloud's account, whichever server plays it.
			if i == 0 && ncFiles == "" && slices.Contains(c.Kinds, view.KindFile) {
				ncFiles, filesName = c.Path, c.DisplayName
			}
		}
		a := a
		t.Cleanup(func() {
			res, _, err := discover.Run(ctx, guard, a.server, a.user, a.pass, "")
			if err != nil {
				t.Errorf("%s: discovery for the cleanup: %v — the set's collections stay there", a.label, err)
				return
			}
			client, err := dav.NewClient(guard, a.server, a.user, a.pass, "")
			if err != nil {
				t.Error(err)
				return
			}
			for _, c := range res.Collections {
				if before[c.Path] || !setNames[c.DisplayName] {
					continue
				}
				if err := client.Delete(ctx, c.Path, ""); err != nil {
					t.Errorf("left on %s: %s (%v)", a.label, c.Path, err)
				}
			}
		})
	}

	// The files: «Documents» and «Public» in the folder of SFTPGo, «Files»
	// in Nextcloud's — at the sizes and with the times of the frames, as far
	// as a server takes a time from its client (X-OC-Mtime).
	day := standDay(t)
	put := func(a account, base string, set []davtest.SetFile) {
		for _, f := range set {
			if strings.HasSuffix(f.Rel, "/") {
				liveSend(t, a.user, a.pass, "MKCOL", base+escapePath(f.Rel), nil, 0, nil)
				continue
			}
			size := f.Size
			if size == 0 {
				size = int64(len(f.Data))
			}
			body := io.MultiReader(strings.NewReader(f.Data), io.LimitReader(zeros{}, size-int64(len(f.Data))))
			// A file the server does not take stays out, and the run goes
			// on: Davis's image refuses 88 MB with 500 (its PHP limits).
			if code := liveTry(t, a.user, a.pass, "PUT", base+escapePath(f.Rel), body, size,
				map[string]string{"X-OC-Mtime": strconv.FormatInt(f.At.Unix(), 10)}); code >= 300 {
				t.Logf("%s (%s): PUT %s of %d bytes → %d — the set's file stays out", a.label, a.name, f.Rel, size, code)
			}
		}
	}
	liveSend(t, files.user, files.pass, "DELETE", files.server, nil, 0, nil)
	liveSend(t, files.user, files.pass, "MKCOL", files.server, nil, 0, nil)
	t.Cleanup(func() { liveSend(t, files.user, files.pass, "DELETE", files.server, nil, 0, nil) })
	for _, folder := range []string{"Documents", "Public"} {
		set, err := davtest.SetFiles(filepath.Join(root, "sftpgo-vps", folder), folder, day)
		if err != nil {
			t.Fatal(err)
		}
		liveSend(t, files.user, files.pass, "MKCOL", files.server+folder+"/", nil, 0, nil)
		put(files, files.server+folder+"/", set)
	}
	if ncFiles == "" {
		// A server of calendars and contacts only (Radicale, Stalwart):
		// the frames of «Files» are not for it.
		t.Logf("nextcloud (%s): discovery names no file storage — «Files» of s-files-6 is not there", accounts[0].name)
	} else {
		// «Files» is the account's files home itself: what the set has
		// for it goes in at its top, beside what the account holds, and
		// only what was not there is taken away after.
		nc := accounts[0]
		u, _ := url.Parse(nc.server)
		u.Path = ncFiles
		set, err := davtest.SetFiles(filepath.Join(root, "nextcloud-home", "Files"), "Files", day)
		if err != nil {
			t.Fatal(err)
		}
		var fresh []davtest.SetFile
		for _, f := range set {
			at := u.String() + escapePath(f.Rel)
			if liveSend(t, nc.user, nc.pass, "PROPFIND", at, nil, 0, map[string]string{"Depth": "0"}) < 300 {
				t.Logf("nextcloud: %s is there already; the stand leaves it as it is", f.Rel)
				continue
			}
			fresh = append(fresh, f)
			t.Cleanup(func() { liveSend(t, nc.user, nc.pass, "DELETE", at, nil, 0, nil) })
		}
		put(nc, u.String(), fresh)
	}

	feeds, leaguePassword := standFeeds(t, root)

	t.Setenv("CARREL_TIMEZONE", "Europe/Moscow")
	b := startPlainOn(t, port, "127.0.0.1")
	standSignIn(b)
	for _, a := range accounts {
		b.open("/app/settings/connections")
		b.follow("Connect account")
		b.press("Connect", url.Values{"label": {a.label}, "server": {a.server}, "login": {a.user}, "password": {a.pass}}, nil)
		connected(t, b, a)
	}
	b.open("/app/files")
	b.follow("Connect account")
	b.press("Connect", url.Values{"label": {files.label}, "server": {files.server}, "login": {files.user}, "password": {files.pass}}, nil)
	connected(t, b, files)

	// The collections, made by Carrel in the order of the set — a server
	// lists them in the order they were made, and the rails of the frames
	// have that order —, then each file imported into its collection.
	ids := map[string]string{}
	for _, col := range standSet {
		kind := "event"
		switch {
		case col.comps == nil:
			kind = "contact"
		case len(col.comps) == 1 && col.comps[0] == "VTODO":
			kind = "task"
		case len(col.comps) == 1 && col.comps[0] == "VJOURNAL":
			kind = "note"
		}
		b.open("/app/collections/new?kind=" + kind)
		ids[col.account] = optionOf(t, b, "account", col.account)
		// The address of the set, typed: the frames draw the set's
		// (…/lichnyj/, …/arhiv/), and Carrel's own from the name is another
		// (lichnyi, arkhiv: architecture.md's table).
		set := url.Values{"account": {ids[col.account]}, "name": {col.name}, "colour": {col.colour}, "address": {col.slug}}
		for _, c := range col.comps {
			set.Add("components", c)
		}
		b.press("Create", set, nil)
		// A server with a trash bin keeps a deleted collection at its
		// address (SV-30): the set's of an earlier run, there for 30 days.
		// The person types another address, as the refusal asks.
		if takenAt.MatchString(b.page.text()) && strings.HasPrefix(b.page.url.Path, "/app/collections/new") {
			again := col.slug + "-" + strconv.FormatInt(time.Now().Unix()%100000, 10)
			t.Logf("%s · %s: «%s» — the address typed instead: %s", col.account, col.name, takenAt.FindString(b.page.text()), again)
			set.Set("address", again)
			b.press("Create", set, nil)
		}
		if strings.HasPrefix(b.page.url.Path, "/app/collections/new") {
			t.Fatalf("%s · %s not made: %.900s", col.account, col.name, b.page.text())
		}
		if col.readOnly {
			t.Logf("%s · %s: the account's own collection, written to — not read-only as the set has it", col.account, col.name)
		}
	}
	for _, col := range standSet {
		for _, file := range col.files {
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
			if err != nil {
				t.Fatal(err)
			}
			section := "calendar"
			switch {
			case col.comps == nil:
				section = "contacts"
			case strings.Contains(string(data), "BEGIN:VTODO"):
				section = "tasks"
			case strings.Contains(string(data), "BEGIN:VJOURNAL"):
				section = "notes"
			}
			// Работа holds events and tasks: each file goes in by the import
			// of its kind.
			at := regexp.MustCompile(`^/app/[a-z]+/`).ReplaceAllString(collectionHref(t, b, col, ids[col.account]), "/app/"+section+"/")
			b.open(at + "/import")
			if !strings.HasSuffix(b.page.url.Path, "/import") {
				t.Fatalf("%s/import went to %s: %.600s", at, b.page.url, b.page.text())
			}
			b.press("Preview file", nil, map[string][2]string{"file": {filepath.Base(file), string(data)}})
			run := b.page.find(func(n *html.Node) bool {
				return (n.Data == "button" || n.Data == "input") && importRun.MatchString(words(n)+attr(n, "value"))
			})
			if run == nil {
				t.Errorf("%s into %s: no Import after the preview: %.600s", file, col.name, b.page.text())
				continue
			}
			b.press(strings.TrimSpace(words(run)+attr(run, "value")), nil, nil)
			t.Logf("%s into %s · %s: %.300s", file, col.account, col.name, b.page.text())
		}
	}

	standRails(t, b, feeds.URL, func(value, name string) bool {
		for _, col := range standSet {
			// A set's collection in the section of its kind: a server that
			// keeps every component in a calendar (Xandikos, SV-43; SOGo,
			// SV-64) shows «Проекты» among the notebooks too.
			if strings.Contains(value, "/"+ids[col.account]+"/") && strings.Contains(name, col.name) && shownIn(col, value) {
				return true
			}
		}
		// The feeds, and the storages: «Files» is the files home of
		// nextcloud's account (s-files-6).
		if strings.HasPrefix(value, "/app/files/") && filesName != "" && strings.Contains(name, filesName) {
			return true
		}
		for _, n := range []string{"Праздники РФ", "Расписание лиги", "Documents", "Public"} {
			if strings.Contains(name, n) {
				return true
			}
		}
		for _, id := range ids {
			if strings.Contains(value, "/"+id+"/") {
				return false
			}
		}
		return false
	})
	leaguePassword.Store("changed")
	t.Log("baikal · Общий stays readable: the stand cannot make a real server refuse its password (README, step 6)")

	mock := standMockup(t, port, root)
	t.Logf("the stand on the test servers: %s (login %s); the mockup: %s/mockups/carrel-ui-mockups.html; holding %s", b.origin, standLogin, mock, wait)
	standWait(t, wait)
}

var (
	importRun = regexp.MustCompile(`^\s*Import \d`)
	takenAt   = regexp.MustCompile(`Something (?:already lives|deleted) at [^.]*\.`)
)

type account struct{ label, name, server, user, pass string }

// liveServer is one of our test servers by its name: those of
// credentials.txt, and the Linux machine's stands (scripts/linux-stands.sh)
// through the tunnel at CARREL_STANDS_HOST, 127.0.0.1 by default, with the
// accounts the stands make.
func liveServer(t *testing.T, name string) account {
	t.Helper()
	env := func(v string) string {
		s := os.Getenv(v)
		if s == "" {
			t.Skip(v + " is not in credentials.txt")
		}
		return s
	}
	host := os.Getenv("CARREL_STANDS_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	a := account{name: name}
	switch name {
	case "nextcloud", "baikal", "sftpgo", "opencloud":
		up := strings.ToUpper(name)
		a.server, a.user, a.pass = env("CARREL_"+up+"_URL"), env("CARREL_"+up+"_USER"), env("CARREL_"+up+"_PASS")
	case "sogo":
		a.server, a.user, a.pass = env("CARREL_SOGO_CAL_URL"), env("CARREL_SOGO_USER"), env("CARREL_SOGO_PASS")
	case "radicale":
		a.server, a.user, a.pass = "http://"+host+":15232/test/", "test", "test"
	case "stalwart":
		a.server, a.user, a.pass = "http://"+host+":18088/", "carrel@carrel.test", "carrel-pass"
	case "xandikos":
		a.server, a.user = "http://"+host+":18081/", "carrel"
	case "davis":
		a.server, a.user, a.pass = "http://"+host+":18082/dav/", "carrel", "carrel-pass"
	case "davis-files":
		// Davis's files home of the account, as a plain WebDAV address.
		a.server, a.user, a.pass = "http://"+host+":18082/dav/home/carrel/", "carrel", "carrel-pass"
	case "davical":
		a.server, a.user, a.pass = "http://"+host+":18083/caldav.php/", "carrel", "carrel-pass"
	default:
		t.Fatalf("no test server %q", name)
	}
	return a
}

// connected ends the run when Connect did not connect: the sheet is still
// there, and what it says is the finding.
func connected(t *testing.T, b *plainBrowser, a account) {
	t.Helper()
	if strings.HasSuffix(b.page.url.Path, "/new") || strings.Contains(b.page.text(), "Could not connect") {
		t.Fatalf("%s (%s, %s): Connect did not connect — %s: %.1500s", a.label, a.name, a.server, b.page.url, b.page.text())
	}
	t.Logf("%s (%s) connected: %s", a.label, a.name, b.page.url.Path)
}

// optionOf is the value of the option of the select named name whose words
// hold label.
func optionOf(t *testing.T, b *plainBrowser, name, label string) string {
	t.Helper()
	sel := b.page.find(func(n *html.Node) bool { return n.Data == "select" && attr(n, "name") == name })
	if sel == nil {
		t.Fatalf("%s: no select %q: %.600s", b.page.url.Path, name, b.page.text())
	}
	for _, o := range nodes(sel, "option") {
		if strings.Contains(words(o), label) {
			return optionValue(o)
		}
	}
	t.Fatalf("%s: no %q among the options of %q", b.page.url.Path, label, name)
	return ""
}

// collectionHref is where the rail of its section leads to a collection of
// the set: the link with its name, on the server of its account (id) — an
// account may hold one of the same name of its own.
func collectionHref(t *testing.T, b *plainBrowser, col standCollection, id string) string {
	t.Helper()
	section := sectionOf(col)
	b.open("/app/" + section)
	// The rail's row of a source is its tick, which sends its address.
	tick := b.page.find(func(n *html.Node) bool {
		return (n.Data == "button" || n.Data == "input") && attr(n, "name") == "toggle" &&
			strings.HasPrefix(attr(n, "value"), "/app/"+section+"/"+id+"/") && strings.Contains(tickName(n), col.name)
	})
	if tick == nil {
		t.Fatalf("/app/%s: no tick of %s on %s; buttons: %s", section, col.name, id, b.page.buttons())
	}
	return attr(tick, "value")
}

// sectionOf is the section a collection of the set is shown in: Работа,
// of events and tasks, in the calendar.
func sectionOf(col standCollection) string {
	switch {
	case col.comps == nil:
		return "contacts"
	case len(col.comps) == 1 && col.comps[0] == "VTODO":
		return "tasks"
	case len(col.comps) == 1 && col.comps[0] == "VJOURNAL":
		return "notes"
	}
	return "calendar"
}

// shownIn reports whether a rail's row of section (its tick's value) is
// where the set has col: a calendar of events and tasks (Работа) in the
// calendar and the tasks alike.
func shownIn(col standCollection, value string) bool {
	if col.comps == nil {
		return strings.HasPrefix(value, "/app/contacts/")
	}
	for _, c := range col.comps {
		section := map[string]string{"VEVENT": "calendar", "VTODO": "tasks", "VJOURNAL": "notes"}[c]
		if strings.HasPrefix(value, "/app/"+section+"/") {
			return true
		}
	}
	return false
}

// liveSend sends one request to a test server and answers with its status;
// a refusal other than a missing thing on DELETE and PROPFIND ends the run.
func liveSend(t *testing.T, user, pass, method, address string, body io.Reader, size int64, head map[string]string) int {
	t.Helper()
	code := liveTry(t, user, pass, method, address, body, size, head)
	if code >= 300 && !(code == http.StatusNotFound && (method == "DELETE" || method == "PROPFIND")) {
		t.Fatalf("%s %s: %d %s", method, address, code, http.StatusText(code))
	}
	return code
}

// liveTry is liveSend that leaves a refusal to its caller.
func liveTry(t *testing.T, user, pass, method, address string, body io.Reader, size int64, head map[string]string) int {
	t.Helper()
	req, err := http.NewRequest(method, address, body)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.ContentLength = size
	}
	for k, v := range head {
		req.Header.Set(k, v)
	}
	req.SetBasicAuth(user, pass)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, address, err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp.StatusCode
}

// escapePath escapes each segment of a slash path.
func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}

// zeros reads as zeros without end: what a stub is grown with.
type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}
