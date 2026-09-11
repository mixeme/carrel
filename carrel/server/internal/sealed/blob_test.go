package sealed

import (
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
