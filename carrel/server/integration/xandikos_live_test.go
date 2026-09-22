//go:build integration

package integration

import (
	"os"
	"testing"
)

// Xandikos is the second light engine beside Radicale and shares no code
// with it: Python, storing every collection as a git repository. It runs
// from `docker-compose.linux.yml` on the Linux machine and is skipped
// everywhere else.
//
// It asks for no credentials at all — the README calls a bare instance
// standalone, with authentication left to a proxy in front. Carrel wants a
// login, because an address without one is a subscription and not an
// account (owner's definition, 18.09.2026), but it does not want a
// password. So the suite signs in with a login and an empty password, and
// that is the first server in the lineup to exercise the case where
// nothing is asked of us.
const (
	xandikosDefaultURL  = "http://127.0.0.1:8081/"
	xandikosDefaultUser = "carrel"
)

func xandikosConfig(t *testing.T) (endpoint, user, pass string) {
	t.Helper()
	endpoint = envOr("CARREL_XANDIKOS_URL", xandikosDefaultURL)
	user = envOr("CARREL_XANDIKOS_USER", xandikosDefaultUser)
	pass = os.Getenv("CARREL_XANDIKOS_PASS") // empty on purpose
	requireReachable(t, "Xandikos", endpoint,
		"it runs on the Linux machine, with scripts/linux-gates.sh; see carrel/server/testdata/xandikos/README.md")
	return endpoint, user, pass
}

func TestXandikosRecords(t *testing.T) {
	endpoint, user, pass := xandikosConfig(t)
	runRecords(t, endpoint, user, pass)
}

func TestXandikosForms(t *testing.T) {
	endpoint, user, pass := xandikosConfig(t)
	runForms(t, endpoint, user, pass)
}

func TestXandikosCollections(t *testing.T) {
	endpoint, user, pass := xandikosConfig(t)
	runCollections(t, endpoint, user, pass)
}

func TestXandikosMove(t *testing.T) {
	endpoint, user, pass := xandikosConfig(t)
	runMove(t, endpoint, user, pass)
}

func TestXandikosSearch(t *testing.T) {
	endpoint, user, pass := xandikosConfig(t)
	runSearch(t, endpoint, user, pass)
}

func TestXandikosTimeline(t *testing.T) {
	endpoint, user, pass := xandikosConfig(t)
	runTimeline(t, endpoint, user, pass)
}

func TestXandikosCollectionDelete(t *testing.T) {
	endpoint, user, pass := xandikosConfig(t)
	runCollectionDelete(t, endpoint, user, pass)
}
