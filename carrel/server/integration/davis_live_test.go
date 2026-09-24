//go:build integration

package integration

import (
	"fmt"
	"net/url"
	"os"
	"testing"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/provider/file"
)

// Davis is sabre/dav again — the same engine as Baikal and Nextcloud — so
// it is in the lineup for completeness rather than for news: what it tells
// us, those two have mostly told us already. It earns its place cheaply,
// as one container on the Linux machine.
//
// Davis has no way to create a DAV user but its admin screens: the API
// reads users and never writes them, there is no console command, and the
// row cannot honestly be written straight into the database — the password
// is a PHP password_hash despite the column being called digesta1, and
// making a user writes four entities at once (user, principal, a default
// calendar with its instance, an address book). So the stand's account is
// made through the admin form itself, by testdata/davis/make-account.sh,
// which scripts/linux-gates.sh runs; the defaults below are what it makes.
const davisDefaultURL = "http://127.0.0.1:8082/dav/"

func davisConfig(t *testing.T) (endpoint, user, pass string) {
	t.Helper()
	endpoint = envOr("CARREL_DAVIS_URL", davisDefaultURL)
	user, pass = envOr("CARREL_DAVIS_USER", "carrel"), envOr("CARREL_DAVIS_PASS", "carrel-pass")
	requireReachable(t, "Davis", endpoint,
		"it runs on the Linux machine, with scripts/linux-gates.sh; see carrel/server/testdata/davis/README.md")
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

// S-30.2 / S-17: Davis serves files too, from a home of each account's own
// under /dav/home/ (WEBDAV_HOMES_DIR in docker-compose.linux.yml). The
// client sits at the origin and the root is the whole path, as for Stalwart.
func TestDavisFiles(t *testing.T) {
	endpoint, user, pass := davisConfig(t)
	u, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	client, err := dav.NewClient(testGuard(t), u.Scheme+"://"+u.Host+"/", user, pass, "")
	if err != nil {
		t.Fatal(err)
	}
	p, err := file.New(client)
	if err != nil {
		t.Fatal(err)
	}
	root := fmt.Sprintf("/dav/home/%s/carrel-files-%d/", user, os.Getpid())
	t.Logf("root: %s", root)
	runFiles(t, client, p, root)
}
