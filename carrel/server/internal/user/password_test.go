package user_test

import (
	"context"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/user"
)

func testServiceWithMgr(t *testing.T) (*user.Service, *store.Store, *session.Manager) {
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
	return user.New(st, mgr, cfg), st, mgr
}

func TestChangePasswordBurnsWipeTokensAndEndsOtherSessions(t *testing.T) {
	svc, st, mgr := testServiceWithMgr(t)
	ctx := context.Background()
	if err := svc.SetupFirstAdmin(ctx, "alice", "password123"); err != nil {
		t.Fatal(err)
	}
	snap := st.Snapshot()
	alice := snap.UserByLogin("alice")
	id, dataKey, _, ok := svc.Authenticate(ctx, "alice", "password123")
	if !ok {
		t.Fatal("auth")
	}
	plain := sealed.Blob{Connections: []sealed.Connection{{ID: "a-alice00001", Server: "https://dav.example.com"}}}
	blob, err := sealed.Seal(dataKey, plain, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Mutate(func(s *store.State) ([]store.AuditEntry, error) {
		s.UserByLogin("alice").SealedBlob = blob
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}

	link, err := svc.IssueConsoleWipeLink("alice")
	if err != nil {
		t.Fatal(err)
	}
	token := strings.TrimPrefix(link, "/forgot/")

	other, err := mgr.Create(id, "alice", dataKey)
	if err != nil {
		t.Fatal(err)
	}
	current, err := mgr.Create(id, "alice", dataKey)
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.ChangePassword(ctx, alice.ID, current.ID(), "password123", "newpass1234", "newpass1234"); err != nil {
		t.Fatal(err)
	}

	if _, _, _, ok := svc.Authenticate(ctx, "alice", "password123"); ok {
		t.Fatal("old password still works")
	}
	_, newKey, _, ok := svc.Authenticate(ctx, "alice", "newpass1234")
	if !ok {
		t.Fatal("new password does not work")
	}
	snap = st.Snapshot()
	got, err := sealed.Open(newKey, snap.UserByLogin("alice").SealedBlob, "alice")
	if err != nil || len(got.Connections) != 1 {
		t.Fatal("blob unreadable after password change")
	}
	if _, ok := mgr.Get(other.ID()); ok {
		t.Fatal("other session still alive")
	}
	if _, ok := mgr.Get(current.ID()); !ok {
		t.Fatal("current session ended")
	}
	if _, _, valid := svc.ForgotWipeState(ctx, token); valid {
		t.Fatal("wipe token still valid after password change")
	}
	if snap.UserByLogin("alice").MustChangePassword {
		t.Fatal("must-change should be cleared")
	}
}

func TestChangePasswordWrongCurrent(t *testing.T) {
	svc, st, _ := testServiceWithMgr(t)
	ctx := context.Background()
	if err := svc.SetupFirstAdmin(ctx, "alice", "password123"); err != nil {
		t.Fatal(err)
	}
	snap := st.Snapshot()
	aliceID := snap.UserByLogin("alice").ID
	if err := svc.ChangePassword(ctx, aliceID, "", "wrongpass123", "newpass1234", "newpass1234"); err != user.ErrWrongPassword {
		t.Fatalf("want ErrWrongPassword, got %v", err)
	}
}
