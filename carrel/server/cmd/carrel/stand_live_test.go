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
// The accounts are the test servers' of credentials.txt, never a person's
// mailbox (Yandex, Google, iCloud, Mail.ru: the set's events have
// attendees): nextcloud is Nextcloud, «radicale · vps» SOGo — a second
// server, for one principal cannot be connected twice —, baikal Baikal, and
// «sftpgo · vps» a folder carrel-set of SFTPGo. What the stand makes there it
// takes away when it ends: run it with CARREL_STAND_STOP, and end it by
// making that file, not by killing it.
//
//	CARREL_LIVE_STAND=3h CARREL_STAND_STOP=<file> go test -tags integration -run TestLiveStand -timeout 0 -v ./cmd/carrel
func TestLiveStand(t *testing.T) {
	wait := standHold(t, "CARREL_LIVE_STAND")
	port, root := standPort(t), standRoot(t)
	integration.LiveCredentials()

	type account struct{ label, server, user, pass string }
	env := func(name string) string {
		v := os.Getenv(name)
		if v == "" {
			t.Skip(name + " is not in credentials.txt")
		}
		return v
	}
	accounts := []account{
		{"nextcloud", env("CARREL_NEXTCLOUD_URL"), env("CARREL_NEXTCLOUD_USER"), env("CARREL_NEXTCLOUD_PASS")},
		{"radicale · vps", env("CARREL_SOGO_CAL_URL"), env("CARREL_SOGO_USER"), env("CARREL_SOGO_PASS")},
		{"baikal", env("CARREL_BAIKAL_URL"), env("CARREL_BAIKAL_USER"), env("CARREL_BAIKAL_PASS")},
	}
	files := account{"sftpgo · vps", strings.TrimSuffix(env("CARREL_SFTPGO_URL"), "/") + "/carrel-set/", env("CARREL_SFTPGO_USER"), env("CARREL_SFTPGO_PASS")}

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
	var ncFiles string
	for _, a := range accounts {
		res, _, err := discover.Run(ctx, guard, a.server, a.user, a.pass, "")
		if err != nil {
			t.Fatalf("%s: discovery: %v", a.label, err)
		}
		before := map[string]bool{}
		for _, c := range res.Collections {
			before[c.Path] = true
			if m := nextcloudFiles.FindStringSubmatch(c.Path); m != nil {
				ncFiles = m[1]
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
			liveSend(t, a.user, a.pass, "PUT", base+escapePath(f.Rel), body, size,
				map[string]string{"X-OC-Mtime": strconv.FormatInt(f.At.Unix(), 10)})
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
		t.Error("nextcloud: discovery names no storage «Files» (s-files-6)")
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
	}
	b.open("/app/files")
	b.follow("Connect account")
	b.press("Connect", url.Values{"label": {files.label}, "server": {files.server}, "login": {files.user}, "password": {files.pass}}, nil)

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
		set := url.Values{"account": {ids[col.account]}, "name": {col.name}, "colour": {col.colour}}
		for _, c := range col.comps {
			set.Add("components", c)
		}
		b.press("Create", set, nil)
		// A server with a trash bin keeps a deleted collection at its
		// address (SV-30): the set's of an earlier run, there for 30 days.
		// The person types another address, as the refusal asks.
		if m := takenAt.FindStringSubmatch(b.page.text()); m != nil && strings.HasPrefix(b.page.url.Path, "/app/collections/new") {
			again := m[1] + "-" + strconv.FormatInt(time.Now().Unix()%100000, 10)
			t.Logf("%s · %s: «%s» — the address typed instead: %s", col.account, col.name, m[0], again)
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
			if strings.Contains(value, "/"+ids[col.account]+"/") && strings.Contains(name, col.name) {
				return true
			}
		}
		// The feeds, and the storages: «Files» is the files home of
		// nextcloud's account (s-files-6).
		if strings.HasPrefix(value, "/app/files/") && strings.Contains(name, "Files") {
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
	nextcloudFiles = regexp.MustCompile(`^(.*/remote\.php/dav/files/[^/]+/)`)
	importRun      = regexp.MustCompile(`^\s*Import \d`)
	takenAt        = regexp.MustCompile(`Something already lives at (\S+)/ on this server[^.]*\.`)
)

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
	section := "calendar"
	switch {
	case col.comps == nil:
		section = "contacts"
	case len(col.comps) == 1 && col.comps[0] == "VTODO":
		section = "tasks"
	case len(col.comps) == 1 && col.comps[0] == "VJOURNAL":
		section = "notes"
	}
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

// liveSend sends one request to a test server and answers with its status;
// a refusal other than a missing thing on DELETE and PROPFIND ends the run.
func liveSend(t *testing.T, user, pass, method, address string, body io.Reader, size int64, head map[string]string) int {
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
	if resp.StatusCode >= 300 && !(resp.StatusCode == http.StatusNotFound && (method == "DELETE" || method == "PROPFIND")) {
		t.Fatalf("%s %s: %s", method, address, resp.Status)
	}
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
