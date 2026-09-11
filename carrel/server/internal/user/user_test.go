package user_test

import (
	"context"
	"testing"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/user"
)

func testService(t *testing.T) (*user.Service, *store.Store) {
	dir := t.TempDir()
	key, err := crypt.CreateServerKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(dir, key)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := conf.Load()
	if err != nil {
		t.Fatal(err)
	}
	cfg.DataDir = dir
	mgr := session.NewManager(cfg)
	return user.New(st, mgr, cfg), st
}

func TestSetupFirstAdmin(t *testing.T) {
	svc, st := testService(t)
	if err := svc.SetupFirstAdmin(context.Background(), "admin", "password123"); err != nil {
		t.Fatal(err)
	}
	if st.FirstRun() {
		t.Fatal("expected written state")
	}
	id, key, _, ok := svc.Authenticate(context.Background(), "admin", "password123")
	if !ok || id == "" || key == [32]byte{} {
		t.Fatalf("auth failed: ok=%v id=%s", ok, id)
	}
}

func TestInviteSecretLength(t *testing.T) {
	svc, _ := testService(t)
	_ = svc.SetupFirstAdmin(context.Background(), "admin", "password123")
	secret, err := svc.CreateInvite("bob", "bob@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(secret) < 32 {
		t.Fatalf("secret too short: %d", len(secret))
	}
	if !svc.InviteValid(context.Background(), secret) {
		t.Fatal("invite should be valid")
	}
	if err := svc.CompleteInvite(context.Background(), secret, "bobpass12"); err != nil {
		t.Fatal(err)
	}
	_, _, _, ok := svc.Authenticate(context.Background(), "bob", "bobpass12")
	if !ok {
		t.Fatal("bob should login")
	}
}

func TestLastAdminCannotDisable(t *testing.T) {
	svc, _ := testService(t)
	_ = svc.SetupFirstAdmin(context.Background(), "admin", "password123")
	id, _, _, _ := svc.Authenticate(context.Background(), "admin", "password123")
	if err := svc.DisableUser(id, id); err != user.ErrLastAdmin {
		t.Fatalf("expected ErrLastAdmin, got %v", err)
	}
}

func TestConsoleWipeLink(t *testing.T) {
	svc, _ := testService(t)
	_ = svc.SetupFirstAdmin(context.Background(), "admin", "password123")
	link, err := svc.IssueConsoleWipeLink("admin")
	if err != nil {
		t.Fatal(err)
	}
	if link == "" || len(link) < 20 {
		t.Fatalf("link = %q", link)
	}
}
