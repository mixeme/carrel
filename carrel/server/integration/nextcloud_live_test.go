//go:build integration

package integration

import (
	"os"
	"testing"
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
