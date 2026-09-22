//go:build integration

package integration

import (
	"os"
	"testing"
)

// Davis is sabre/dav again — the same engine as Baikal and Nextcloud — so
// it is in the lineup for completeness rather than for news: what it tells
// us, those two have mostly told us already. It earns its place cheaply,
// as one container on the Linux machine.
//
// It is the one stand whose account is made by hand. Davis has no way to
// create a DAV user without its admin screens: the API reads users and
// never writes them, there is no console command, and the row cannot
// honestly be written straight into the database — the password is a
// PHP password_hash despite the column being called digesta1, and making
// a user writes four entities at once (user, principal, a default
// calendar with its instance, an address book). Reproducing that in SQL
// would be reimplementing the application, and it would rot at its next
// release. So the container comes up on its own, a person makes one user
// in a minute, and the suite is told about it through the environment.
const davisDefaultURL = "http://127.0.0.1:8082/dav/"

func davisConfig(t *testing.T) (endpoint, user, pass string) {
	t.Helper()
	endpoint = envOr("CARREL_DAVIS_URL", davisDefaultURL)
	user, pass = os.Getenv("CARREL_DAVIS_USER"), os.Getenv("CARREL_DAVIS_PASS")
	requireReachable(t, "Davis", endpoint,
		"it runs on the Linux machine, with scripts/linux-gates.sh; see carrel/server/testdata/davis/README.md")
	if user == "" || pass == "" {
		t.Skip("CARREL_DAVIS_USER and CARREL_DAVIS_PASS required: Davis makes its DAV accounts " +
			"through its admin screens only, so the one this suite uses is made by hand once — " +
			"see carrel/server/testdata/davis/README.md")
	}
	return endpoint, user, pass
}

func TestDavisRecords(t *testing.T) {
	endpoint, user, pass := davisConfig(t)
	runRecords(t, endpoint, user, pass)
}

func TestDavisForms(t *testing.T) {
	endpoint, user, pass := davisConfig(t)
	runForms(t, endpoint, user, pass)
}

func TestDavisCollections(t *testing.T) {
	endpoint, user, pass := davisConfig(t)
	runCollections(t, endpoint, user, pass)
}

func TestDavisMove(t *testing.T) {
	endpoint, user, pass := davisConfig(t)
	runMove(t, endpoint, user, pass)
}

func TestDavisSearch(t *testing.T) {
	endpoint, user, pass := davisConfig(t)
	runSearch(t, endpoint, user, pass)
}

func TestDavisTimeline(t *testing.T) {
	endpoint, user, pass := davisConfig(t)
	runTimeline(t, endpoint, user, pass)
}

func TestDavisCollectionDelete(t *testing.T) {
	endpoint, user, pass := davisConfig(t)
	runCollectionDelete(t, endpoint, user, pass)
}
