// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"errors"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/crypto"
)

const masterPassword = "master password for escrow"

// coveredUser sets escrow up and returns a user created under it, together
// with the DEK their password opens.
func coveredUser(t *testing.T, s *Store, admin *User, login string) (*User, crypto.Key) {
	t.Helper()
	if err := s.EnableEscrow(actorOf(admin), masterPassword); err != nil {
		t.Fatalf("EnableEscrow: %v", err)
	}
	_, token, err := s.CreateInvite(actorOf(admin), login, login+"@example.org", RoleUser, 0)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	if _, err := s.AcceptInvite(token, testPassword, ""); err != nil {
		t.Fatalf("AcceptInvite: %v", err)
	}
	user, dek, err := s.Authenticate(login, testPassword)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if len(user.EscrowDEK) == 0 {
		t.Fatalf("%s was created under active escrow with no deposited copy", login)
	}
	return user, dek
}

// The whole point of the scheme (§5.4): the administrator gets the account
// back without the password, and the data comes back with it.
func TestRecoveryKeepsTheData(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	user, before := coveredUser(t, s, admin, "ada")
	defer before.Zero()

	const temp = "temporary handover"
	recovered, err := s.RecoverUser(actorOf(admin), user.ID, masterPassword, temp)
	if err != nil {
		t.Fatalf("RecoverUser: %v", err)
	}
	if !recovered.MustChangePassword {
		t.Error("a password the administrator knows must be changed at next login")
	}
	if recovered.EscrowRecoveredAt.IsZero() {
		t.Error("the recovery was not stamped on the account")
	}
	if recovered.Email == "" {
		t.Error("the caller cannot send the mandatory notice without the address")
	}

	// The old password is gone and the new one opens the very same key, so
	// everything sealed under it is still readable.
	if _, _, err := s.Authenticate("ada", testPassword); !errors.Is(err, ErrAuth) {
		t.Errorf("the recovered account still takes the old password: %v", err)
	}
	_, after, err := s.Authenticate("ada", temp)
	if err != nil {
		t.Fatalf("Authenticate with the temporary password: %v", err)
	}
	defer after.Zero()
	if !crypto.Equal(before, after) {
		t.Fatal("recovery replaced the DEK; the connections it was meant to save are gone")
	}

	// The user signs in and picks their own password: still the same key, and
	// still recoverable, because the copy is of the key and not of a password.
	const chosen = "chosen by the user"
	if err := s.ChangePassword(user.ID, temp, chosen); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	_, final, err := s.Authenticate("ada", chosen)
	if err != nil {
		t.Fatalf("Authenticate after the user's own change: %v", err)
	}
	defer final.Zero()
	if !crypto.Equal(before, final) {
		t.Error("the password change after a recovery lost the DEK")
	}
	stored, err := s.User(user.ID)
	if err != nil {
		t.Fatalf("User: %v", err)
	}
	if len(stored.EscrowDEK) == 0 {
		t.Error("the deposited copy did not survive the recovery")
	}
	if stored.MustChangePassword {
		t.Error("the forced-change flag outlived the change")
	}
}

// Recovery is exactly as hard as the master password, which is not the
// administrator's login password (§5.4).
func TestRecoveryNeedsTheMasterPassword(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	user, dek := coveredUser(t, s, admin, "ada")
	defer dek.Zero()

	for _, wrong := range []string{testPassword, "master password for escrov", ""} {
		_, err := s.RecoverUser(actorOf(admin), user.ID, wrong, "temporary handover")
		if !errors.Is(err, crypto.ErrWrongMasterPassword) {
			t.Errorf("RecoverUser(%q): got %v, want ErrWrongMasterPassword", wrong, err)
		}
	}
	// A refused attempt leaves the account exactly as it was.
	if _, _, err := s.Authenticate("ada", testPassword); err != nil {
		t.Errorf("a failed recovery disturbed the account: %v", err)
	}
	if _, _, err := s.Authenticate("ada", "temporary handover"); !errors.Is(err, ErrAuth) {
		t.Error("a failed recovery installed the temporary password anyway")
	}
}

// Enabling escrow is not retroactive, so there is nothing to recover for an
// account that predates it (§5.4, §21).
func TestRecoveryRefusedWithoutADeposit(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)

	if _, err := s.RecoverUser(actorOf(admin), admin.ID, masterPassword, "temporary handover"); !errors.Is(err, ErrEscrowNotConfigured) {
		t.Errorf("recovery with no key pair: got %v, want ErrEscrowNotConfigured", err)
	}
	if err := s.EnableEscrow(actorOf(admin), masterPassword); err != nil {
		t.Fatalf("EnableEscrow: %v", err)
	}
	if _, err := s.RecoverUser(actorOf(admin), admin.ID, masterPassword, "temporary handover"); !errors.Is(err, ErrEscrowNotDeposited) {
		t.Errorf("recovery of an account that predates escrow: got %v, want ErrEscrowNotDeposited", err)
	}
}

// An existing user joins the scheme from their profile, and only they can do
// it: their DEK cannot be opened without their password (§5.4).
func TestOptInThenOptOut(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	a := actorOf(admin)
	if err := s.EnableEscrow(a, masterPassword); err != nil {
		t.Fatalf("EnableEscrow: %v", err)
	}

	// The administrator predates escrow, so nothing about them is recoverable
	// until they say otherwise.
	if err := s.EscrowOptIn(a, admin.ID, "not their password"); !errors.Is(err, ErrAuth) {
		t.Errorf("opt-in with a wrong password: got %v, want ErrAuth", err)
	}
	if stored, _ := s.User(admin.ID); len(stored.EscrowDEK) != 0 {
		t.Fatal("a refused opt-in deposited a copy")
	}

	if err := s.EscrowOptIn(a, admin.ID, testPassword); err != nil {
		t.Fatalf("EscrowOptIn: %v", err)
	}
	if err := s.EscrowOptIn(a, admin.ID, testPassword); !errors.Is(err, ErrEscrowDeposited) {
		t.Errorf("second opt-in: got %v, want ErrEscrowDeposited", err)
	}

	// The copy is of the real key: a recovery on it returns the same DEK.
	_, before, err := s.Authenticate("root", testPassword)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	defer before.Zero()
	if _, err := s.RecoverUser(a, admin.ID, masterPassword, "temporary handover"); err != nil {
		t.Fatalf("RecoverUser after opt-in: %v", err)
	}
	_, after, err := s.Authenticate("root", "temporary handover")
	if err != nil {
		t.Fatalf("Authenticate after recovery: %v", err)
	}
	defer after.Zero()
	if !crypto.Equal(before, after) {
		t.Error("the voluntarily deposited copy did not hold the account's own DEK")
	}

	// Withdrawing makes the account unrecoverable again, which is what lets
	// the destructive reset back in.
	if err := s.EscrowOptOut(a, admin.ID); err != nil {
		t.Fatalf("EscrowOptOut: %v", err)
	}
	if stored, _ := s.User(admin.ID); len(stored.EscrowDEK) != 0 {
		t.Fatal("opting out left the copy in place")
	}
	if err := s.EscrowOptOut(a, admin.ID); !errors.Is(err, ErrEscrowNotDeposited) {
		t.Errorf("second opt-out: got %v, want ErrEscrowNotDeposited", err)
	}
	if err := s.ResetPassword(a, admin.ID, "temporary handover again"); err != nil {
		t.Errorf("reset is still refused after the copy was withdrawn: %v", err)
	}
}

// The administrator may take the withdrawal away, but the user has to be able
// to see that they did (§5.4) — here, that the refusal is its own error and
// not silence.
func TestOptOutCanBeForbidden(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	user, dek := coveredUser(t, s, admin, "ada")
	defer dek.Zero()

	if err := s.SetEscrowOptOutPolicy(actorOf(admin), true); err != nil {
		t.Fatalf("SetEscrowOptOutPolicy: %v", err)
	}
	if !s.Settings().Escrow.ForbidOptOut {
		t.Fatal("the policy was not stored")
	}
	if err := s.EscrowOptOut(actorOf(admin), user.ID); !errors.Is(err, ErrEscrowOptOutForbidden) {
		t.Fatalf("forbidden opt-out: got %v, want ErrEscrowOptOutForbidden", err)
	}
	if stored, _ := s.User(user.ID); len(stored.EscrowDEK) == 0 {
		t.Error("the copy went away despite the refusal")
	}

	if err := s.SetEscrowOptOutPolicy(actorOf(admin), false); err != nil {
		t.Fatalf("SetEscrowOptOutPolicy: %v", err)
	}
	if err := s.EscrowOptOut(actorOf(admin), user.ID); err != nil {
		t.Errorf("opt-out after the policy was lifted: %v", err)
	}
}

// Opting in needs somewhere to put the copy: with deposit switched off there
// is no scheme to join, even though the key pair is still on the volume.
func TestOptInRequiresActiveEscrow(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	a := actorOf(admin)

	if err := s.EscrowOptIn(a, admin.ID, testPassword); !errors.Is(err, ErrEscrowNotConfigured) {
		t.Errorf("opt-in with no key pair: got %v, want ErrEscrowNotConfigured", err)
	}
	if err := s.EnableEscrow(a, masterPassword); err != nil {
		t.Fatalf("EnableEscrow: %v", err)
	}
	if err := s.DisableEscrow(a); err != nil {
		t.Fatalf("DisableEscrow: %v", err)
	}
	if err := s.EscrowOptIn(a, admin.ID, testPassword); !errors.Is(err, ErrEscrowNotConfigured) {
		t.Errorf("opt-in while deposit is off: got %v, want ErrEscrowNotConfigured", err)
	}
}

// Turning deposit back on uses the key pair already on the volume, so the
// copies taken before it was switched off keep working — and it asks for the
// master password, because an administrator who cannot produce it would be
// switching on a scheme that recovers nothing.
func TestResumeEscrow(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	a := actorOf(admin)
	user, dek := coveredUser(t, s, admin, "ada")
	defer dek.Zero()

	if err := s.ResumeEscrow(a, masterPassword); err != nil {
		t.Fatalf("resume while already enabled: %v", err)
	}
	if err := s.DisableEscrow(a); err != nil {
		t.Fatalf("DisableEscrow: %v", err)
	}
	if err := s.ResumeEscrow(a, "not the master password"); !errors.Is(err, crypto.ErrWrongMasterPassword) {
		t.Fatalf("resume with a wrong master password: got %v, want ErrWrongMasterPassword", err)
	}
	if s.Settings().Escrow.Enabled {
		t.Fatal("a refused resume switched deposit on anyway")
	}

	if err := s.ResumeEscrow(a, masterPassword); err != nil {
		t.Fatalf("ResumeEscrow: %v", err)
	}
	if !s.Settings().Escrow.Active() {
		t.Error("escrow is not active after being resumed")
	}
	if _, err := s.RecoverUser(a, user.ID, masterPassword, "temporary handover"); err != nil {
		t.Errorf("a copy taken before the pause is no longer recoverable: %v", err)
	}
}

// Generating a second key pair would orphan every copy taken under the first.
func TestEnableEscrowRefusedTwice(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)

	if err := s.EnableEscrow(actorOf(admin), masterPassword); err != nil {
		t.Fatalf("EnableEscrow: %v", err)
	}
	if err := s.EnableEscrow(actorOf(admin), "another master password"); !errors.Is(err, ErrEscrowConfigured) {
		t.Errorf("second setup: got %v, want ErrEscrowConfigured", err)
	}
	if err := s.EnableEscrow(actorOf(admin), "short"); !errors.Is(err, crypto.ErrMasterPasswordTooShort) {
		t.Errorf("short master password: got %v, want ErrMasterPasswordTooShort", err)
	}
}

// Only the private key is re-encrypted, so nobody has to deposit again (§5.4).
func TestChangeMasterPasswordKeepsDeposits(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	a := actorOf(admin)
	user, dek := coveredUser(t, s, admin, "ada")
	defer dek.Zero()

	const next = "a different master password"
	if err := s.ChangeEscrowMasterPassword(a, "not the master password", next); !errors.Is(err, crypto.ErrWrongMasterPassword) {
		t.Fatalf("change with a wrong current password: got %v, want ErrWrongMasterPassword", err)
	}
	if err := s.ChangeEscrowMasterPassword(a, masterPassword, next); err != nil {
		t.Fatalf("ChangeEscrowMasterPassword: %v", err)
	}

	if _, err := s.RecoverUser(a, user.ID, masterPassword, "temporary handover"); !errors.Is(err, crypto.ErrWrongMasterPassword) {
		t.Error("the old master password still recovers accounts")
	}
	recovered, err := s.RecoverUser(a, user.ID, next, "temporary handover")
	if err != nil {
		t.Fatalf("recovery under the new master password: %v", err)
	}
	if !crypto.Equal(recoveredDEK(t, s, recovered.Login, "temporary handover"), dek) {
		t.Error("the deposited copy stopped matching the account's DEK")
	}
}

func recoveredDEK(t *testing.T, s *Store, login, password string) crypto.Key {
	t.Helper()
	_, dek, err := s.Authenticate(login, password)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	t.Cleanup(dek.Zero)
	return dek
}

// Every recovery is in the log, and what is in the log is who did it to whom —
// never the passwords involved (§5.4, §5.5).
func TestEscrowActionsAreAudited(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	a := actorOf(admin)
	user, dek := coveredUser(t, s, admin, "ada")
	defer dek.Zero()

	const temp = "temporary handover"
	if _, err := s.RecoverUser(a, user.ID, masterPassword, temp); err != nil {
		t.Fatalf("RecoverUser: %v", err)
	}
	if err := s.EscrowOptOut(a, user.ID); err != nil {
		t.Fatalf("EscrowOptOut: %v", err)
	}
	if err := s.EscrowOptIn(a, user.ID, temp); err != nil {
		t.Fatalf("EscrowOptIn: %v", err)
	}

	for _, action := range []string{ActionEscrowEnable, ActionEscrowRecover, ActionEscrowOptOut, ActionEscrowOptIn} {
		entries := s.Audit(AuditFilter{Action: action})
		if len(entries) == 0 {
			t.Errorf("no audit entry for %s", action)
		}
	}
	recovery := s.Audit(AuditFilter{Action: ActionEscrowRecover})
	if len(recovery) != 1 {
		t.Fatalf("recorded %d recoveries, want 1", len(recovery))
	}
	if recovery[0].ActorLogin != admin.Login || recovery[0].TargetLogin != user.Login {
		t.Errorf("recovery entry = %+v, want root recovering ada", recovery[0])
	}
	for _, e := range s.Audit(AuditFilter{}) {
		if e.Detail == masterPassword || e.Detail == temp || e.Detail == testPassword {
			t.Errorf("a password reached the audit log: %+v", e)
		}
	}
}
