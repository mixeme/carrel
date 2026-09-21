//go:build integration

package integration

import (
	"net"
	"net/url"
	"os"
	"testing"
	"time"
)

// Stalwart is the fourth engine in the lineup and the only one written in
// Rust: its CalDAV and CardDAV are young and share no ancestry with
// sabre/dav, Radicale or SoGo, so it is the server most likely to tell us
// something we do not know. It runs from `docker-compose.linux.yml` on the
// Linux machine, like Radicale, and is skipped everywhere else.
//
// Stalwart serves calendars under /dav/cal, address books under /dav/card,
// files under /dav/file and principals under /dav/pal, and answers
// /.well-known/caldav and /.well-known/carddav. Discovery is given the root
// and walks from there, exactly as it does for a server nobody prepared.
const (
	stalwartDefaultURL  = "http://127.0.0.1:8088/"
	stalwartDefaultUser = "carrel@carrel.test"
	stalwartDefaultPass = "carrel-pass"
)

// stalwartConfig is the address of the local stand and the account
// `testdata/stalwart/plan.ndjson` provisions. The environment overrides all
// three, so the same suite can be pointed at a Stalwart someone else runs.
func stalwartConfig(t *testing.T) (endpoint, user, pass string) {
	t.Helper()
	endpoint = envOr("CARREL_STALWART_URL", stalwartDefaultURL)
	user = envOr("CARREL_STALWART_USER", stalwartDefaultUser)
	pass = envOr("CARREL_STALWART_PASS", stalwartDefaultPass)
	requireReachable(t, "Stalwart", endpoint,
		"it runs on the Linux machine, with scripts/linux-gates.sh; see carrel/server/testdata/stalwart/README.md")
	return endpoint, user, pass
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

// requireReachable skips the test with an address and a reason when nothing
// answers, rather than failing: a server that is not running is not a fault
// of the code under test, and a run on a machine without it must stay
// green and say why.
func requireReachable(t *testing.T, name, endpoint, where string) {
	t.Helper()
	u, err := url.Parse(endpoint)
	if err != nil {
		t.Fatalf("%s address %q: %v", name, endpoint, err)
	}
	host := u.Host
	if u.Port() == "" {
		host = u.Hostname() + ":80"
		if u.Scheme == "https" {
			host = u.Hostname() + ":443"
		}
	}
	conn, err := net.DialTimeout("tcp", host, 2*time.Second)
	if err != nil {
		t.Skipf("no %s at %s (%v): %s", name, endpoint, err, where)
	}
	_ = conn.Close()
}

func TestStalwartRecords(t *testing.T) {
	endpoint, user, pass := stalwartConfig(t)
	runRecords(t, endpoint, user, pass)
}

func TestStalwartForms(t *testing.T) {
	endpoint, user, pass := stalwartConfig(t)
	runForms(t, endpoint, user, pass)
}

func TestStalwartCollections(t *testing.T) {
	endpoint, user, pass := stalwartConfig(t)
	runCollections(t, endpoint, user, pass)
}

func TestStalwartMove(t *testing.T) {
	endpoint, user, pass := stalwartConfig(t)
	runMove(t, endpoint, user, pass)
}

func TestStalwartSearch(t *testing.T) {
	endpoint, user, pass := stalwartConfig(t)
	runSearch(t, endpoint, user, pass)
}

func TestStalwartTimeline(t *testing.T) {
	endpoint, user, pass := stalwartConfig(t)
	runTimeline(t, endpoint, user, pass)
}

func TestStalwartCollectionDelete(t *testing.T) {
	endpoint, user, pass := stalwartConfig(t)
	runCollectionDelete(t, endpoint, user, pass)
}
