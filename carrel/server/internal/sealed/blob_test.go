package sealed

import (
	"carrel.dev/carrel/server/view"
	"encoding/json"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/crypt"
)

func TestConnectionCertFingerprintRoundTrip(t *testing.T) {
	login := "alice"
	dataKey, err := crypt.NewDataKey()
	if err != nil {
		t.Fatal(err)
	}
	want := Blob{
		Connections: []Connection{{
			ID: "c1", Server: "https://dav.example/", Login: "bob", Password: "secret",
			CertFingerprint: "abc123",
		}},
	}
	ct, err := Seal(dataKey, want, login)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Open(dataKey, ct, login)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Connections) != 1 || got.Connections[0].CertFingerprint != "abc123" {
		t.Fatalf("got %+v", got)
	}
	if _, err := Open(dataKey, ct, "bob"); err == nil {
		t.Fatal("blob must not open under another login AAD")
	}
}

// C-10, owner's decision 24.09.2026: two collections of one connection under
// one name are twins, and Open says so. A calendar and an address book of one
// name are not: they never share a rail. Another connection's collection of
// that name is not either. Twin is never stored.
func TestOpenMarksTwins(t *testing.T) {
	dataKey, err := crypt.NewDataKey()
	if err != nil {
		t.Fatal(err)
	}
	b := Blob{Collections: []Collection{
		{AccountID: "a", Path: "/cal/carrel-s-15/", Name: "Carrel S-15", Kinds: []view.Kind{view.KindEvent}},
		{AccountID: "a", Path: "/cal/carrel-s-15-2/", Name: "carrel s-15", Kinds: []view.Kind{view.KindEvent, view.KindTask}},
		{AccountID: "a", Path: "/card/carrel-s-15/", Name: "Carrel S-15", Kinds: []view.Kind{view.KindContact}},
		{AccountID: "b", Path: "/cal/carrel-s-15/", Name: "Carrel S-15", Kinds: []view.Kind{view.KindEvent}},
		{AccountID: "a", Path: "/cal/work/", Name: "Work", Kinds: []view.Kind{view.KindEvent}},
	}}
	ct, err := Seal(dataKey, b, "alice")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Open(dataKey, ct, "alice")
	if err != nil {
		t.Fatal(err)
	}
	want := []bool{true, true, false, false, false}
	for i, c := range got.Collections {
		if c.Twin != want[i] {
			t.Errorf("%s %s: twin %v, want %v", c.AccountID, c.Path, c.Twin, want[i])
		}
	}
	if strings.Contains(string(mustJSON(t, got)), "Twin") || strings.Contains(string(mustJSON(t, got)), "twin") {
		t.Error("Twin is written out")
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
