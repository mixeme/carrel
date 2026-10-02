//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/discover"
	"carrel.dev/carrel/server/internal/provider/file"
)

// S-30.2 on Nextcloud. sabre/dav underneath, like Baikal, so the interest is
// not the library but the deployment: shared hosting, /remote.php/dav/ as the
// entered address, and a principal whose own path holds spaces
// ("MIX_1 for tests"). Every address in this account therefore arrives
// percent-encoded, which is exactly the shape that breaks a client that builds
// addresses by hand — so the whole harness runs here, and what it exercises is
// escaping as much as protocol.
//
// The account has no calendars of its own: its calendar home holds only the
// scheduling inbox, outbox and trash bin, and Carrel lists none of them as a
// calendar (probe 17.09.2026). The harness makes its own calendar and its own
// address book, so nothing here depends on that.

func nextcloudConfig(t *testing.T) (endpoint, user, pass string) {
	liveCredEnv()
	endpoint = os.Getenv("CARREL_NEXTCLOUD_URL")
	user = os.Getenv("CARREL_NEXTCLOUD_USER")
	pass = os.Getenv("CARREL_NEXTCLOUD_PASS")
	if endpoint == "" || user == "" || pass == "" {
		t.Skip("CARREL_NEXTCLOUD_URL, CARREL_NEXTCLOUD_USER, CARREL_NEXTCLOUD_PASS required")
	}
	return endpoint, user, pass
}

func TestNextcloudRecords(t *testing.T) {
	endpoint, user, pass := nextcloudConfig(t)
	runRecords(t, endpoint, user, pass)
}

func TestNextcloudCollections(t *testing.T) {
	endpoint, user, pass := nextcloudConfig(t)
	runCollections(t, endpoint, user, pass)
}

func TestNextcloudSearch(t *testing.T) {
	endpoint, user, pass := nextcloudConfig(t)
	runSearch(t, endpoint, user, pass)
}

func TestNextcloudMove(t *testing.T) {
	endpoint, user, pass := nextcloudConfig(t)
	runMove(t, endpoint, user, pass)
}

func TestNextcloudImportExport(t *testing.T) {
	endpoint, user, pass := nextcloudConfig(t)
	runImportExport(t, endpoint, user, pass)
}

func TestNextcloudCollectionDelete(t *testing.T) {
	endpoint, user, pass := nextcloudConfig(t)
	runCollectionDelete(t, endpoint, user, pass)
}

// vCard 4.0 through sabre/dav: the card is asked for in 4.0 and comes back
// whole, converted only where the server converts it (docs/servers.md, SV-1).
func TestNextcloudVCard4Edit(t *testing.T) {
	endpoint, user, pass := nextcloudConfig(t)
	runVCard4Edit(t, endpoint, user, pass)
}

// The photo: sabre/dav keeps a data: URI as sent, like Baikal.
func TestNextcloudPhoto(t *testing.T) {
	endpoint, user, pass := nextcloudConfig(t)
	runPhoto(t, endpoint, user, pass, true)
}

// S-30.2 / S-17: plain WebDAV on the same engine that serves the calendars
// and the address books. Until 23.09.2026 files were checked only on SFTPGo
// and OpenCloud, which serve nothing else; Nextcloud, Stalwart and Davis all
// offer the three together and none of them was run against its files
// (docs/servers.md, «Что сервер умеет и что мы у него проверяем»).
//
// Here it is worth more than the coverage: this account's own path holds a
// space, so every address arrives percent-encoded. A file layer that builds
// addresses by hand breaks on exactly that.
func TestNextcloudFiles(t *testing.T) {
	client, p, home := nextcloudFiles(t)
	root := fmt.Sprintf("%scarrel-files-%d/", home, os.Getpid())
	t.Logf("root: %s", root)
	runFiles(t, client, p, root)
}

// nextcloudFiles dials the files of the account and says where its home
// is: /remote.php/dav/files/<user id>/.
func nextcloudFiles(t *testing.T) (*dav.Client, *file.Provider, string) {
	t.Helper()
	endpoint, user, pass := nextcloudConfig(t)
	g := testGuard(t)
	res, trace, err := discover.Run(context.Background(), g, endpoint, user, pass, "")
	if err != nil || res.Principal == "" {
		t.Fatalf("discovery: %v %+v %+v", err, res, trace)
	}
	// Nextcloud's layout is fixed: /remote.php/dav/files/<user id>/, and the
	// user id is the last segment of the principal — «MIX_1 for tests» here,
	// spaces and all. The login is an email address and is not it.
	uid := strings.Trim(res.Principal, "/")
	uid = uid[strings.LastIndex(uid, "/")+1:]
	// The client sits at the origin and the root is the whole path, the way
	// SFTPGo's does. A client rooted at the files home and a root starting
	// with a slash do not compose: the request leaves the home altogether
	// and answers 404 with an HTML page (my own mistake, 23.09.2026).
	origin, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	origin.Path, origin.RawQuery, origin.Fragment = "/", "", ""
	client, err := dav.NewClient(g, origin.String(), user, pass, "")
	if err != nil {
		t.Fatal(err)
	}
	p, err := file.New(client)
	if err != nil {
		t.Fatal(err)
	}
	return client, p, fmt.Sprintf("/remote.php/dav/files/%s/", uid)
}
