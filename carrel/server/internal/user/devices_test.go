package user

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/conf"
)

// S-23: a profile link lives conf.DeviceProfileLinkTTL and opens once; a
// revoked one does not open; the state holds its hash and the connections,
// never the token itself.
func TestDeviceProfileLinkLifetime(t *testing.T) {
	f := newFixture(t)
	id := f.setupAdmin(t)

	token, _, err := f.svc.IssueDeviceProfile(id, []string{"c1", "c2"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tok := range f.st.Snapshot().Tokens {
		if strings.Contains(string(tok.Hash), token) || tok.Step != "c1,c2" {
			t.Errorf("stored token = %+v", tok)
		}
	}
	f.advance(conf.DeviceProfileLinkTTL)
	if _, ok := f.svc.TakeDeviceProfile(id, token); ok {
		t.Error("an expired link opened")
	}

	token, _, _ = f.svc.IssueDeviceProfile(id, []string{"c1"})
	f.advance(conf.DeviceProfileLinkTTL - 1)
	if got, ok := f.svc.TakeDeviceProfile(id, token); !ok || len(got) != 1 || got[0] != "c1" {
		t.Fatalf("a live link: %v %v", got, ok)
	}
	if _, ok := f.svc.TakeDeviceProfile(id, token); ok {
		t.Error("a used link opened again")
	}

	token, _, _ = f.svc.IssueDeviceProfile(id, []string{"c1"})
	if err := f.svc.RevokeDeviceProfiles(id); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.svc.TakeDeviceProfile(id, token); ok {
		t.Error("a revoked link opened")
	}
	if _, _, err := f.svc.IssueDeviceProfile(id, nil); err != ErrNothingToIssue {
		t.Errorf("a link to nothing: %v", err)
	}
}
