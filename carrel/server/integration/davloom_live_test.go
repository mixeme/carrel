//go:build integration

package integration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/internal/proxy"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// S-27 live: a device talks to davloom as to one server, and davloom to a
// real one. Real servers write hrefs their own way — whole URLs, escaped
// segments — and a device must see only /c/… of its own. The test makes a
// calendar of its own on the far server and removes it after.
func TestBaikalThroughDavloom(t *testing.T) {
	endpoint, user, pass := baikalConfig(t)
	runDavloom(t, endpoint, user, pass)
}

func TestNextcloudThroughDavloom(t *testing.T) {
	endpoint, user, pass := nextcloudConfig(t)
	runDavloom(t, endpoint, user, pass)
}

func runDavloom(t *testing.T, endpoint, login, pass string) {
	ctx := context.Background()
	client := liveClient(t, endpoint, login, pass)
	calHome, _ := liveHomes(ctx, t, endpoint, login, pass)
	suffix := fmt.Sprintf("carrel-s27-%d-%d", os.Getpid(), time.Now().Unix()%100000)
	calPath := makeCollection(ctx, t, client, endpoint, login, pass, "MKCALENDAR", calHome+suffix+"/", `<?xml version="1.0" encoding="utf-8"?>
<C:mkcalendar xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:set><D:prop>
    <D:displayname>Carrel S-27</D:displayname>
    <C:supported-calendar-component-set><C:comp name="VEVENT"/></C:supported-calendar-component-set>
  </D:prop></D:set>
</C:mkcalendar>`)

	dir := t.TempDir()
	key, err := crypt.CreateServerKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dir, key)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.DataDir, cfg.DAVloomPort = dir, 8081
	users := user.New(st, session.NewManager(cfg), cfg)
	if err := users.SetupFirstAdmin(ctx, "owner", "password123"); err != nil {
		t.Fatal(err)
	}
	id, dataKey, _, _ := users.Authenticate(ctx, "owner", "password123")
	account, err := users.AddConnection(id, dataKey, sealed.Connection{Server: endpoint, Login: login, Password: pass},
		[]sealed.Collection{{Path: calPath, Name: "Carrel S-27", Kinds: []view.Kind{view.KindEvent}}})
	if err != nil {
		t.Fatal(err)
	}
	_, password, err := users.IssueDevice(ctx, id, dataKey, "Live", user.DeviceScope{
		Kinds: []view.Kind{view.KindEvent}, Cols: []store.DeviceScope{{AccountID: account, Path: calPath}}})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(proxy.New(cfg, users, testGuard(t), "live", "live").Handler())
	defer srv.Close()
	col := "/c/" + account + "/" + url.PathEscape(calPath) + "/"
	u, _ := url.Parse(endpoint)

	do := func(method, path, ctype, body string, header map[string]string) (int, http.Header, string) {
		t.Helper()
		req, _ := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		if ctype != "" {
			req.Header.Set("Content-Type", ctype)
		}
		for k, v := range header {
			req.Header.Set(k, v)
		}
		req.SetBasicAuth("owner", password)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, resp.Header, string(data)
	}
	// Nothing of the far server may reach the device: not its host, not the
	// collection's path there.
	foreign := func(step, body string) {
		t.Helper()
		escaped := strings.ReplaceAll(url.PathEscape(calPath), "%2F", "/")
		if strings.Contains(body, u.Host) || strings.Contains(body, calPath) || strings.Contains(body, escaped) {
			t.Errorf("%s: the far server shows through:\n%s", step, body)
		}
	}
	xmlType := `application/xml; charset="utf-8"`

	code, _, home := do("PROPFIND", "/cal/", xmlType, "", map[string]string{"Depth": "1"})
	if code != 207 || !strings.Contains(home, col) {
		t.Fatalf("home %d:\n%s", code, home)
	}

	event := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Carrel live//EN\r\nBEGIN:VEVENT\r\nUID:" + suffix +
		"\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20261001T100000Z\r\nDTEND:20261001T110000Z\r\nSUMMARY:Through davloom\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	code, _, _ = do("PUT", col+suffix+".ics", "text/calendar; charset=utf-8", event, map[string]string{"If-None-Match": "*"})
	if code/100 != 2 {
		t.Fatalf("put: %d", code)
	}

	code, _, listing := do("PROPFIND", col, xmlType, `<?xml version="1.0"?><d:propfind xmlns:d="DAV:"><d:prop><d:getetag/><d:resourcetype/></d:prop></d:propfind>`, map[string]string{"Depth": "1"})
	if code != 207 || !strings.Contains(listing, col+suffix+".ics") {
		t.Fatalf("listing %d:\n%s", code, listing)
	}
	foreign("listing", listing)

	query := `<?xml version="1.0"?><c:calendar-query xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav"><d:prop><d:getetag/><c:calendar-data/></d:prop>` +
		`<c:filter><c:comp-filter name="VCALENDAR"><c:comp-filter name="VEVENT"><c:time-range start="20260901T000000Z" end="20261101T000000Z"/></c:comp-filter></c:comp-filter></c:filter></c:calendar-query>`
	code, _, found := do("REPORT", col, xmlType, query, map[string]string{"Depth": "1"})
	if code != 207 || !strings.Contains(found, "Through davloom") {
		t.Fatalf("calendar-query %d:\n%s", code, found)
	}
	foreign("calendar-query", found)

	multiget := `<?xml version="1.0"?><c:calendar-multiget xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav"><d:prop><d:getetag/><c:calendar-data/></d:prop><d:href>` +
		col + suffix + `.ics</d:href></c:calendar-multiget>`
	code, _, got := do("REPORT", col, xmlType, multiget, map[string]string{"Depth": "1"})
	if code != 207 || !strings.Contains(got, "Through davloom") {
		t.Fatalf("multiget %d:\n%s", code, got)
	}
	foreign("multiget", got)

	etag := regexp.MustCompile(`<[a-zA-Z0-9]*:?getetag>([^<]+)<`).FindStringSubmatch(found)
	if etag == nil {
		t.Fatalf("no etag in:\n%s", found)
	}
	tag := strings.ReplaceAll(etag[1], "&quot;", `"`)
	code, _, _ = do("DELETE", col+suffix+".ics", "", "", map[string]string{"If-Match": tag})
	if code/100 != 2 {
		t.Errorf("delete with If-Match %s: %d", tag, code)
	}
}
