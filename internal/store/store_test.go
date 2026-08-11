// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/crypto"
)

// fastParams keep Argon2id cheap. The production profiles are exercised in the
// crypto package; here the point is the persistence logic around them.
func fastParams() crypto.Params {
	return crypto.Params{Time: 1, Memory: 8 * 1024, Threads: 1, KeyLen: crypto.KeyLen}
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

func openTest(t *testing.T, dir string) (*Store, *clock) {
	t.Helper()
	c := &clock{t: time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)}
	s, err := OpenWith(dir, Options{Auth: fastParams(), KEK: fastParams(), Master: fastParams(), Now: c.now})
	if err != nil {
		t.Fatalf("OpenWith: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, c
}

const testPassword = "correct horse battery"

func mustAdmin(t *testing.T, s *Store) *User {
	t.Helper()
	u, err := s.CreateFirstAdmin("root", "root@example.org", testPassword, "10.0.0.1")
	if err != nil {
		t.Fatalf("CreateFirstAdmin: %v", err)
	}
	return u
}

func actorOf(u *User) Actor { return Actor{ID: u.ID, Login: u.Login, IP: "10.0.0.1"} }

func TestBootstrapDetection(t *testing.T) {
	dir := t.TempDir()
	s, _ := openTest(t, dir)

	if !s.NeedsBootstrap() {
		t.Fatal("empty volume must ask for the first administrator")
	}
	// Nothing is written before the first change: the server key alone is not
	// a configured instance.
	if _, err := os.Stat(filepath.Join(dir, StateFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("state file exists before any change: %v", err)
	}

	mustAdmin(t, s)
	if s.NeedsBootstrap() {
		t.Fatal("bootstrap still requested after the administrator was created")
	}
	if _, err := s.CreateFirstAdmin("second", "", testPassword, ""); !errors.Is(err, ErrNotBootstrap) {
		t.Fatalf("second bootstrap: got %v, want ErrNotBootstrap", err)
	}
}

func TestReopenPreservesState(t *testing.T) {
	dir := t.TempDir()
	s, _ := openTest(t, dir)
	admin := mustAdmin(t, s)
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, _ := openTest(t, dir)
	if reopened.NeedsBootstrap() {
		t.Fatal("reopened volume asks for bootstrap")
	}
	got, err := reopened.UserByLogin("root")
	if err != nil {
		t.Fatalf("UserByLogin: %v", err)
	}
	if got.ID != admin.ID {
		t.Errorf("user ID = %q after reload, want %q", got.ID, admin.ID)
	}
	// The password still opens the DEK, so the whole key schedule survived
	// the round trip through the file.
	_, dek, err := reopened.Authenticate("root", testPassword)
	if err != nil {
		t.Fatalf("Authenticate after reload: %v", err)
	}
	dek.Zero()
}

func TestStateFileIsEncrypted(t *testing.T) {
	dir := t.TempDir()
	s, _ := openTest(t, dir)
	mustAdmin(t, s)

	raw, err := os.ReadFile(filepath.Join(dir, StateFile))
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	for _, needle := range []string{"root", "root@example.org", "users"} {
		if containsBytes(raw, needle) {
			t.Errorf("state file leaks %q in the clear", needle)
		}
	}
}

func containsBytes(haystack []byte, needle string) bool {
	n := []byte(needle)
	for i := 0; i+len(n) <= len(haystack); i++ {
		match := true
		for j := range n {
			if haystack[i+j] != n[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func TestWriteIsAtomic(t *testing.T) {
	dir := t.TempDir()
	s, _ := openTest(t, dir)
	mustAdmin(t, s)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		switch e.Name() {
		case StateFile, crypto.ServerKeyFile:
		default:
			t.Errorf("leftover file after write: %s", e.Name())
		}
	}
}

func TestAuthenticate(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)

	u, dek, err := s.Authenticate("ROOT", testPassword)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	defer dek.Zero()
	if u.ID != admin.ID {
		t.Errorf("authenticated %q, want %q", u.Login, admin.Login)
	}
	if len(dek) != crypto.KeyLen {
		t.Errorf("DEK is %d bytes, want %d", len(dek), crypto.KeyLen)
	}

	if _, _, err := s.Authenticate("root", "wrong password here"); !errors.Is(err, ErrAuth) {
		t.Errorf("wrong password: got %v, want ErrAuth", err)
	}
	// An unknown login must fail exactly like a wrong password (§24.3).
	if _, _, err := s.Authenticate("nobody", testPassword); !errors.Is(err, ErrAuth) {
		t.Errorf("unknown login: got %v, want ErrAuth", err)
	}
}

func TestDisabledUserCannotLogIn(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)

	inv, token, err := s.CreateInvite(actorOf(admin), "ada", "ada@example.org", RoleUser, 0)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	_ = inv
	user, err := s.AcceptInvite(token, testPassword, "10.0.0.2")
	if err != nil {
		t.Fatalf("AcceptInvite: %v", err)
	}

	if err := s.SetDisabled(actorOf(admin), user.ID, true); err != nil {
		t.Fatalf("SetDisabled: %v", err)
	}
	if _, _, err := s.Authenticate("ada", testPassword); !errors.Is(err, ErrUserDisabled) {
		t.Fatalf("disabled login: got %v, want ErrUserDisabled", err)
	}
}

func TestChangePasswordKeepsDEK(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)

	_, before, err := s.Authenticate("root", testPassword)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	defer before.Zero()

	const next = "a different passphrase"
	if err := s.ChangePassword(admin.ID, testPassword, next); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if err := s.ChangePassword(admin.ID, "not the password", next); !errors.Is(err, ErrAuth) {
		t.Errorf("change with wrong current password: got %v, want ErrAuth", err)
	}

	_, after, err := s.Authenticate("root", next)
	if err != nil {
		t.Fatalf("Authenticate with new password: %v", err)
	}
	defer after.Zero()

	// Only the wrapping changed: data sealed under the DEK stays readable.
	if !crypto.Equal(before, after) {
		t.Error("password change replaced the DEK; connections would be lost")
	}
	if _, _, err := s.Authenticate("root", testPassword); !errors.Is(err, ErrAuth) {
		t.Errorf("old password still works: %v", err)
	}
}

func TestResetPasswordReplacesDEK(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)

	_, before, err := s.Authenticate("root", testPassword)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	defer before.Zero()

	const temp = "temporary password"
	if err := s.ResetPassword(actorOf(admin), admin.ID, temp); err != nil {
		t.Fatalf("ResetPassword: %v", err)
	}

	u, after, err := s.Authenticate("root", temp)
	if err != nil {
		t.Fatalf("Authenticate after reset: %v", err)
	}
	defer after.Zero()

	if crypto.Equal(before, after) {
		t.Error("reset kept the DEK; it is supposed to be a destructive operation")
	}
	if !u.MustChangePassword {
		t.Error("reset must force a password change at next login")
	}
}

func TestResetRefusedWhenEscrowCovers(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)

	if err := s.EnableEscrow(actorOf(admin), "master password for escrow"); err != nil {
		t.Fatalf("EnableEscrow: %v", err)
	}
	_, token, err := s.CreateInvite(actorOf(admin), "ada", "", RoleUser, 0)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	user, err := s.AcceptInvite(token, testPassword, "")
	if err != nil {
		t.Fatalf("AcceptInvite: %v", err)
	}
	if len(user.EscrowDEK) == 0 {
		t.Fatal("user created under active escrow has no deposited DEK copy")
	}

	if err := s.ResetPassword(actorOf(admin), user.ID, "temporary password"); !errors.Is(err, ErrEscrowActive) {
		t.Fatalf("reset under escrow: got %v, want ErrEscrowActive", err)
	}
}

func TestEscrowOffByDefault(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)

	if s.Settings().Escrow.Active() {
		t.Fatal("escrow must be off on a fresh volume")
	}
	if len(admin.EscrowDEK) != 0 {
		t.Error("a recoverable DEK copy exists although escrow is off")
	}
}

func TestEscrowAppliesOnlyToLaterUsers(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)

	if err := s.EnableEscrow(actorOf(admin), "master password for escrow"); err != nil {
		t.Fatalf("EnableEscrow: %v", err)
	}
	// The administrator existed before escrow was turned on, and their DEK
	// cannot be deposited without their password (§5.4).
	stored, err := s.User(admin.ID)
	if err != nil {
		t.Fatalf("User: %v", err)
	}
	if len(stored.EscrowDEK) != 0 {
		t.Error("escrow was applied retroactively")
	}
}

func TestLastAdminGuard(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	a := actorOf(admin)

	if err := s.SetRole(a, admin.ID, RoleUser); !errors.Is(err, ErrLastAdmin) {
		t.Errorf("demote last admin: got %v, want ErrLastAdmin", err)
	}
	if err := s.DeleteUser(a, admin.ID); !errors.Is(err, ErrLastAdmin) {
		t.Errorf("delete last admin: got %v, want ErrLastAdmin", err)
	}
	if err := s.SetDisabled(a, admin.ID, true); !errors.Is(err, ErrLastAdmin) {
		t.Errorf("disable last admin: got %v, want ErrLastAdmin", err)
	}

	// With a second administrator the guard steps aside.
	second, err := s.CreateUserWithPassword(a, "backup", "", RoleAdmin, testPassword)
	if err != nil {
		t.Fatalf("CreateUserWithPassword: %v", err)
	}
	if err := s.SetRole(a, admin.ID, RoleUser); err != nil {
		t.Errorf("demote with a second admin present: %v", err)
	}
	if err := s.SetRole(a, second.ID, RoleUser); !errors.Is(err, ErrLastAdmin) {
		t.Errorf("demote the now-last admin: got %v, want ErrLastAdmin", err)
	}
}

func TestTemporaryPasswordForcesChange(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)

	u, err := s.CreateUserWithPassword(actorOf(admin), "ada", "", RoleUser, "handed over by hand")
	if err != nil {
		t.Fatalf("CreateUserWithPassword: %v", err)
	}
	if !u.MustChangePassword {
		t.Fatal("temporary password must force a change at first login")
	}
	if err := s.ChangePassword(u.ID, "handed over by hand", "chosen by the user"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	after, err := s.User(u.ID)
	if err != nil {
		t.Fatalf("User: %v", err)
	}
	if after.MustChangePassword {
		t.Error("flag survived the password change")
	}
}

func TestLoginValidation(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	mustAdmin(t, s)

	for _, bad := range []string{"", "-leading", "trailing-", "has space", "sym$bol"} {
		if err := ValidateLogin(NormalizeLogin(bad)); err == nil {
			t.Errorf("ValidateLogin(%q) accepted an invalid login", bad)
		}
	}
	for _, good := range []string{"ada.lovelace_1", "Upper", " padded "} {
		if err := ValidateLogin(NormalizeLogin(good)); err != nil {
			t.Errorf("ValidateLogin(%q) rejected a valid login: %v", good, err)
		}
	}
}

func TestDuplicateLoginRejected(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	a := actorOf(admin)

	if _, err := s.CreateUserWithPassword(a, "ROOT", "", RoleUser, testPassword); !errors.Is(err, ErrLoginTaken) {
		t.Errorf("duplicate login differing in case: got %v, want ErrLoginTaken", err)
	}
	if _, _, err := s.CreateInvite(a, "ada", "", RoleUser, 0); err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	// A pending invite reserves the login too, or two people could claim it.
	if _, err := s.CreateUserWithPassword(a, "ada", "", RoleUser, testPassword); !errors.Is(err, ErrLoginTaken) {
		t.Errorf("login held by a pending invite: got %v, want ErrLoginTaken", err)
	}
}
