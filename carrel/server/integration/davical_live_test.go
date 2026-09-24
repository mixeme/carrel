//go:build integration

package integration

import "testing"

// DAViCal is PHP over PostgreSQL, old and still deployed at hosters and in
// organisations, and — unlike Baikal, Nextcloud and Davis — not sabre/dav:
// an engine of its own, with its own reading of the RFCs. Calendars and
// address books share one home per principal, /caldav.php/<login>/. The
// WebDAV it speaks is access to those collections, not a file store, so
// there is no files suite.
//
// The stand's account is an ordinary principal, not the DAViCal admin: an
// administrator has every privilege on every collection, and a suite run as
// one would pass where a person's account fails. It is made through the
// admin screens' own form by testdata/davical/make-account.sh, which
// scripts/linux-gates.sh runs; the defaults below are what it makes.
const davicalDefaultURL = "http://127.0.0.1:8083/caldav.php/"

func davicalConfig(t *testing.T) (endpoint, user, pass string) {
	t.Helper()
	endpoint = envOr("CARREL_DAVICAL_URL", davicalDefaultURL)
	user, pass = envOr("CARREL_DAVICAL_USER", "carrel"), envOr("CARREL_DAVICAL_PASS", "carrel-pass")
	requireReachable(t, "DAViCal", endpoint,
		"it runs on the Linux machine, with scripts/linux-gates.sh; see carrel/server/testdata/davical/README.md")
	return endpoint, user, pass
}

func TestDAViCalRecords(t *testing.T) {
	endpoint, user, pass := davicalConfig(t)
	runRecords(t, endpoint, user, pass)
}

func TestDAViCalForms(t *testing.T) {
	endpoint, user, pass := davicalConfig(t)
	runForms(t, endpoint, user, pass)
}

func TestDAViCalCollections(t *testing.T) {
	endpoint, user, pass := davicalConfig(t)
	runCollections(t, endpoint, user, pass, escapesNamesTwice)
}

func TestDAViCalMove(t *testing.T) {
	endpoint, user, pass := davicalConfig(t)
	runMove(t, endpoint, user, pass)
}

func TestDAViCalSearch(t *testing.T) {
	endpoint, user, pass := davicalConfig(t)
	runSearch(t, endpoint, user, pass)
}

func TestDAViCalTimeline(t *testing.T) {
	endpoint, user, pass := davicalConfig(t)
	runTimeline(t, endpoint, user, pass)
}

func TestDAViCalCollectionDelete(t *testing.T) {
	endpoint, user, pass := davicalConfig(t)
	runCollectionDelete(t, endpoint, user, pass)
}
