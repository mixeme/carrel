//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/discover"
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

// docs/servers.md, SV-27: the server hands a structured value back with its
// trailing empty components dropped. Carrel's loss report must not call that a
// loss, and this test holds the server's side of it: red on the day SoGo keeps
// every component it was sent.
func TestSoGoTrimsEmptyNameComponents(t *testing.T) {
	endpoint, user, pass := sogoConfig(t)
	ctx := context.Background()
	g := testGuard(t)
	client, err := dav.NewClient(g, endpoint, user, pass, "")
	if err != nil {
		t.Fatal(err)
	}
	res, trace, err := discover.Run(ctx, g, endpoint, user, pass, "")
	if err != nil || res.AddressBookHome == "" {
		t.Fatalf("discovery: %v %+v %+v", err, res, trace)
	}
	uid := fmt.Sprintf("carrel-sv27-%d-%d", os.Getpid(), time.Now().Unix()%100000)
	file := strings.TrimSuffix(res.AddressBookHome, "/") + "/personal/" + uid + ".vcf"
	card := "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:" + uid +
		"\r\nFN:Ada Lovelace\r\nN:Lovelace;Ada;;;\r\nEND:VCARD\r\n"
	if _, err := client.PutCreate(ctx, file, dav.Typed(strings.NewReader(card), "text/vcard; charset=utf-8")); err != nil {
		t.Fatalf("seed %s: %v", file, err)
	}
	removeLater(t, freshClient(t, endpoint, user, pass), file)

	got := rawCard(ctx, t, endpoint, user, pass, file, "text/vcard; version=4.0")
	if !strings.Contains(got, "N:Lovelace;Ada\r\n") && !strings.HasSuffix(strings.TrimSpace(got), "N:Lovelace;Ada") {
		t.Errorf("the server kept the empty components, so SV-27 is over:\n%s", got)
	}
}
