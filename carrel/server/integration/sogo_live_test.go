//go:build integration

package integration

import (
	"os"
	"testing"
)

// S-30.2 on SoGo. The credentials name one calendar and one address book, but
// discovery walks up from either to the principal and finds both homes, so this
// server takes the whole harness: records, collections, search, a move, import
// and export, and a collection deleted by name. Nothing here is new code —
// which is the point of the harness being parameterised by server.
//
// SoGo is a different lineage from sabre/dav and Radicale: a third answer to
// every REPORT, every precondition and every refusal the other two already
// shaped.

func sogoConfig(t *testing.T) (endpoint, user, pass string) {
	liveCredEnv()
	endpoint = os.Getenv("CARREL_SOGO_CAL_URL")
	user = os.Getenv("CARREL_SOGO_USER")
	pass = os.Getenv("CARREL_SOGO_PASS")
	if endpoint == "" || user == "" || pass == "" {
		t.Skip("CARREL_SOGO_CAL_URL, CARREL_SOGO_USER, CARREL_SOGO_PASS required")
	}
	return endpoint, user, pass
}

func TestSoGoRecords(t *testing.T) {
	endpoint, user, pass := sogoConfig(t)
	runRecords(t, endpoint, user, pass)
}

func TestSoGoCollections(t *testing.T) {
	endpoint, user, pass := sogoConfig(t)
	runCollections(t, endpoint, user, pass)
}

func TestSoGoSearch(t *testing.T) {
	endpoint, user, pass := sogoConfig(t)
	runSearch(t, endpoint, user, pass)
}

func TestSoGoMove(t *testing.T) {
	endpoint, user, pass := sogoConfig(t)
	runMove(t, endpoint, user, pass)
}

func TestSoGoImportExport(t *testing.T) {
	endpoint, user, pass := sogoConfig(t)
	runImportExport(t, endpoint, user, pass)
}

func TestSoGoCollectionDelete(t *testing.T) {
	endpoint, user, pass := sogoConfig(t)
	runCollectionDelete(t, endpoint, user, pass)
}

func TestSoGoVCard4Edit(t *testing.T) {
	endpoint, user, pass := sogoConfig(t)
	runVCard4Edit(t, endpoint, user, pass)
}
