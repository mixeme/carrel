// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/crypto"
	"gitea.mixdep.ru/mix/carrel/internal/ratelimit"
	"gitea.mixdep.ru/mix/carrel/internal/store"
)

const masterPassword = "master password for escrow"

// enableEscrow turns the scheme on through the panel, as an administrator
// would.
func (a *app) enableEscrow() {
	a.t.Helper()
	rec := a.post("/admin/", url.Values{
		fieldAction:         {"enable_escrow"},
		fieldMasterPassword: {masterPassword},
		fieldConfirmMaster:  {masterPassword},
	})
	if rec.Code != http.StatusOK {
		a.t.Fatalf("enable escrow: status %d (body: %s)", rec.Code, rec.Body.String())
	}
	if !a.Store.Settings().Escrow.Active() {
		a.t.Fatal("escrow is not active after the panel enabled it")
	}
}

// coveredUser adds an account while escrow is on, so it carries a deposited
// copy of its DEK.
func (a *app) coveredUser(login, email string) *store.User {
	a.t.Helper()
	admin, err := a.Store.UserByLogin("root")
	if err != nil {
		a.t.Fatalf("UserByLogin: %v", err)
	}
	user, err := a.Store.CreateUserWithPassword(
		store.Actor{ID: admin.ID, Login: admin.Login}, login, email, store.RoleUser, testPassword)
	if err != nil {
		a.t.Fatalf("CreateUserWithPassword: %v", err)
	}
	if len(user.EscrowDEK) == 0 {
		a.t.Fatalf("%s was created under active escrow with no deposited copy", login)
	}
	return user
}

// signIn logs in from a fresh cookie jar and returns the session, which stays
// live in the manager after the jar is dropped.
func (a *app) signIn(login, password string) string {
	a.t.Helper()
	a.cookies = map[string]string{}
	a.get("/login")
	rec := a.post("/login", url.Values{fieldLogin: {login}, fieldPassword: {password}})
	if rec.Code != http.StatusSeeOther {
		a.t.Fatalf("sign in as %s: status %d (body: %s)", login, rec.Code, rec.Body.String())
	}
	return a.cookies[SessionCookie]
}

// With escrow off — the default on a fresh volume — nothing about an account
// is recoverable, and the profile says exactly that (§5.4, §21).
func TestEscrowOffByDefault(t *testing.T) {
	a := newApp(t, nil)
	a.setupAdmin("root", "root@example.org", testPassword)

	panel := a.get("/admin/").Body.String()
	if !strings.Contains(panel, `value="enable_escrow"`) {
		t.Error("the panel does not offer to enable key escrow")
	}
	if strings.Contains(panel, `value="recover_user"`) {
		t.Error("the panel offers a recovery with no key pair to recover with")
	}

	profile := a.get("/app/").Body.String()
	if !strings.Contains(profile, "No copy of your data key is deposited") {
		t.Errorf("the profile does not say the account is unrecoverable:\n%s", profile)
	}
	if strings.Contains(profile, `value="opt_in"`) {
		t.Error("the profile offers to deposit a key into a scheme that is off")
	}
}

// Turning the scheme on covers accounts created afterwards, and the people it
// covers are told so — in the profile, permanently, and once at first login
// (§5.4).
func TestEscrowCoversNewAccountsAndSaysSo(t *testing.T) {
	a := newApp(t, nil)
	a.setupAdmin("root", "root@example.org", testPassword)
	a.enableEscrow()
	a.coveredUser("ada", "ada@example.org")

	a.signIn("ada", testPassword)
	first := a.get("/app/").Body.String()
	if !strings.Contains(first, escrowNotice) {
		t.Errorf("no first-login notice for a covered account:\n%s", first)
	}
	if !strings.Contains(first, "A copy of your data key is deposited") {
		t.Error("the profile does not show the deposit status")
	}
	if !strings.Contains(first, `value="opt_out"`) {
		t.Error("the profile does not offer to withdraw the deposited copy")
	}

	// The notice is a notice, not a nag.
	if second := a.get("/app/").Body.String(); strings.Contains(second, escrowNotice) {
		t.Error("the first-login notice was shown again")
	}
	// And it stays gone across sessions, because it was recorded as delivered.
	a.signIn("ada", testPassword)
	if again := a.get("/app/").Body.String(); strings.Contains(again, escrowNotice) {
		t.Error("the notice came back at the next sign-in")
	}
}

// An account that predates the scheme joins it only by its owner's own hand,
// with their own password (§5.4).
func TestProfileOptInAndOptOut(t *testing.T) {
	a := newApp(t, nil)
	a.setupAdmin("root", "root@example.org", testPassword)
	a.enableEscrow()

	if !strings.Contains(a.get("/app/").Body.String(), `value="opt_in"`) {
		t.Fatal("the profile does not offer to join the scheme")
	}

	rec := a.post("/app/escrow", url.Values{fieldAction: {"opt_in"}, fieldPassword: {badPassword}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("opt-in with a wrong password: status %d, want 400", rec.Code)
	}
	admin, err := a.Store.UserByLogin("root")
	if err != nil {
		t.Fatalf("UserByLogin: %v", err)
	}
	if len(admin.EscrowDEK) != 0 {
		t.Fatal("a refused opt-in deposited a copy anyway")
	}

	rec = a.post("/app/escrow", url.Values{fieldAction: {"opt_in"}, fieldPassword: {testPassword}})
	if rec.Code != http.StatusOK {
		t.Fatalf("opt-in: status %d (body: %s)", rec.Code, rec.Body.String())
	}
	if admin, _ = a.Store.UserByLogin("root"); len(admin.EscrowDEK) == 0 {
		t.Fatal("opting in deposited nothing")
	}

	rec = a.post("/app/escrow", url.Values{fieldAction: {"opt_out"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("opt-out: status %d (body: %s)", rec.Code, rec.Body.String())
	}
	if admin, _ = a.Store.UserByLogin("root"); len(admin.EscrowDEK) != 0 {
		t.Error("opting out left the copy in place")
	}
}

// The administrator may forbid withdrawal, but the user has to see that they
// did rather than just find the button missing (§5.4).
func TestForbiddenOptOutIsVisible(t *testing.T) {
	a := newApp(t, nil)
	a.setupAdmin("root", "root@example.org", testPassword)
	a.enableEscrow()
	a.coveredUser("ada", "ada@example.org")

	rec := a.post("/admin/", url.Values{fieldAction: {"escrow_policy"}, fieldForbidOptOut: {"1"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("set policy: status %d", rec.Code)
	}

	a.signIn("ada", testPassword)
	profile := a.get("/app/").Body.String()
	if !strings.Contains(profile, "withdrawing a deposited key") {
		t.Errorf("the profile hides the policy instead of showing it:\n%s", profile)
	}
	if strings.Contains(profile, `value="opt_out"`) {
		t.Error("the withdrawal form is offered although the policy forbids it")
	}

	rec = a.post("/app/escrow", url.Values{fieldAction: {"opt_out"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("forbidden opt-out: status %d, want 400", rec.Code)
	}
	if user, _ := a.Store.UserByLogin("ada"); len(user.EscrowDEK) == 0 {
		t.Error("a posted opt-out went through despite the policy")
	}
}

// The recovery of §5.4 from end to end: the administrator gets the account
// back with the master password, the user's sessions are cut, their data key
// survives, and they are told it happened.
func TestRecoveryThroughThePanel(t *testing.T) {
	a := newApp(t, nil)
	a.setupAdmin("root", "root@example.org", testPassword)
	a.enableEscrow()
	user := a.coveredUser("ada", "ada@example.org")

	// A live session of the account about to be recovered, kept out of the
	// cookie jar the way another browser would be.
	adaSession := a.signIn("ada", testPassword)
	sess := a.session()
	before := append([]byte(nil), sess.DEK()...)

	a.signIn("root", testPassword)
	const temp = "temporary handover"
	rec := a.post("/admin/", url.Values{
		fieldAction:         {"recover_user"},
		fieldUserID:         {user.ID},
		fieldMasterPassword: {masterPassword},
		fieldTempPassword:   {temp},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("recover: status %d (body: %s)", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Recovered ada") {
		t.Errorf("the panel does not confirm the recovery:\n%s", body)
	}
	// This instance has no relay, and the notice may not simply be dropped.
	if !strings.Contains(body, "could not be sent") {
		t.Error("the administrator was not told the mandatory notice did not go out")
	}
	if _, ok := a.Sessions.Get(adaSession); ok {
		t.Error("the recovered account's session outlived the credentials it was opened with")
	}

	a.signIn("ada", temp)
	after := a.session()
	if !crypto.Equal(before, after.DEK()) {
		t.Fatal("recovery replaced the DEK; the data it was meant to save is gone")
	}
	if !after.MustChangePassword() {
		t.Error("a password the administrator chose was not marked for changing")
	}
	if entries := a.Store.Audit(store.AuditFilter{Action: store.ActionEscrowRecover}); len(entries) != 1 {
		t.Errorf("recorded %d recoveries in the audit log, want 1", len(entries))
	}
}

// The master password is the one secret on the instance that opens somebody
// else's data, so guessing at it is throttled (§5.4, §24.3).
func TestRecoveryIsThrottled(t *testing.T) {
	a := newApp(t, nil)
	a.RecoveryLimit = ratelimit.New(ratelimit.Options{Free: 1})
	a.setupAdmin("root", "root@example.org", testPassword)
	a.enableEscrow()
	user := a.coveredUser("ada", "ada@example.org")

	wrong := url.Values{
		fieldAction:         {"recover_user"},
		fieldUserID:         {user.ID},
		fieldMasterPassword: {"not the master password"},
		fieldTempPassword:   {"temporary handover"},
	}
	for i := 0; i < 2; i++ {
		rec := a.post("/admin/", wrong)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("attempt %d: status %d, want 400", i+1, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "not the escrow master password") {
			t.Errorf("attempt %d does not name the problem: %s", i+1, rec.Body.String())
		}
	}

	rec := a.post("/admin/", wrong)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("throttled status = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("no Retry-After on a throttled recovery")
	}
	// The throttle holds even for the right password: the point is to stop
	// the guessing, not to reward the guess that lands.
	right := url.Values{
		fieldAction:         {"recover_user"},
		fieldUserID:         {user.ID},
		fieldMasterPassword: {masterPassword},
		fieldTempPassword:   {"temporary handover"},
	}
	if rec := a.post("/admin/", right); rec.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429 while throttled", rec.Code)
	}
	if stored, _ := a.Store.User(user.ID); !stored.EscrowRecoveredAt.IsZero() {
		t.Error("a throttled attempt recovered the account anyway")
	}
}

// Resetting an account covered by escrow would destroy data that a recovery
// would keep, so it is refused and the refusal names the alternative (§5.5).
func TestResetOffersRecoveryInstead(t *testing.T) {
	a := newApp(t, nil)
	a.setupAdmin("root", "root@example.org", testPassword)
	a.enableEscrow()
	user := a.coveredUser("ada", "ada@example.org")

	rec := a.post("/admin/", url.Values{
		fieldAction:       {"reset_password"},
		fieldUserID:       {user.ID},
		fieldTempPassword: {"temporary handover"},
		fieldConfirmReset: {"1"},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "recover it with the master password instead") {
		t.Errorf("the refusal does not point at the recovery:\n%s", rec.Body.String())
	}
	if _, _, err := a.Store.Authenticate("ada", testPassword); err != nil {
		t.Errorf("the refused reset changed the account anyway: %v", err)
	}

	// Once the copy is withdrawn there is nothing left to recover, and the
	// destructive path is the only one there is.
	if err := a.Store.EscrowOptOut(store.Actor{ID: user.ID, Login: user.Login}, user.ID); err != nil {
		t.Fatalf("EscrowOptOut: %v", err)
	}
	rec = a.post("/admin/", url.Values{
		fieldAction:       {"reset_password"},
		fieldUserID:       {user.ID},
		fieldTempPassword: {"temporary handover"},
		fieldConfirmReset: {"1"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("reset of an uncovered account: status %d (body: %s)", rec.Code, rec.Body.String())
	}
	if _, _, err := a.Store.Authenticate("ada", "temporary handover"); err != nil {
		t.Errorf("the reset did not take effect: %v", err)
	}
}

// Pausing and resuming deposit leaves the copies alone, and resuming asks for
// the master password so that a scheme nobody can use is not switched back on.
func TestEscrowPauseAndResume(t *testing.T) {
	a := newApp(t, nil)
	a.setupAdmin("root", "root@example.org", testPassword)
	a.enableEscrow()
	user := a.coveredUser("ada", "ada@example.org")

	if rec := a.post("/admin/", url.Values{fieldAction: {"disable_escrow"}}); rec.Code != http.StatusOK {
		t.Fatalf("disable: status %d", rec.Code)
	}
	if stored, _ := a.Store.User(user.ID); len(stored.EscrowDEK) == 0 {
		t.Error("pausing deposit dropped a copy the user was told about")
	}

	rec := a.post("/admin/", url.Values{
		fieldAction:         {"resume_escrow"},
		fieldMasterPassword: {"not the master password"},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("resume with a wrong master password: status %d, want 400", rec.Code)
	}
	if a.Store.Settings().Escrow.Enabled {
		t.Fatal("a refused resume switched deposit back on")
	}

	rec = a.post("/admin/", url.Values{
		fieldAction:         {"resume_escrow"},
		fieldMasterPassword: {masterPassword},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("resume: status %d (body: %s)", rec.Code, rec.Body.String())
	}
	if !a.Store.Settings().Escrow.Active() {
		t.Error("escrow is not active after being resumed")
	}
}

// Changing the master password re-seals the private key only, so every
// deposited copy keeps working (§5.4).
func TestChangeMasterPasswordThroughThePanel(t *testing.T) {
	a := newApp(t, nil)
	a.setupAdmin("root", "root@example.org", testPassword)
	a.enableEscrow()
	user := a.coveredUser("ada", "ada@example.org")

	const next = "a different master password"
	rec := a.post("/admin/", url.Values{
		fieldAction:         {"change_master_password"},
		fieldMasterPassword: {masterPassword},
		fieldNewMaster:      {next},
		fieldConfirmMaster:  {"a mistyped master password"},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("mismatched confirmation: status %d, want 400", rec.Code)
	}

	rec = a.post("/admin/", url.Values{
		fieldAction:         {"change_master_password"},
		fieldMasterPassword: {masterPassword},
		fieldNewMaster:      {next},
		fieldConfirmMaster:  {next},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("change master password: status %d (body: %s)", rec.Code, rec.Body.String())
	}

	rec = a.post("/admin/", url.Values{
		fieldAction:         {"recover_user"},
		fieldUserID:         {user.ID},
		fieldMasterPassword: {next},
		fieldTempPassword:   {"temporary handover"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("recovery under the new master password: status %d (body: %s)", rec.Code, rec.Body.String())
	}
}

// A plain user has no way to reach the administrator's half of the scheme.
func TestEscrowPanelIsAdminOnly(t *testing.T) {
	a := newApp(t, nil)
	a.setupAdmin("root", "root@example.org", testPassword)
	a.enableEscrow()
	user := a.coveredUser("ada", "ada@example.org")

	a.signIn("ada", testPassword)
	rec := a.post("/admin/", url.Values{
		fieldAction:         {"recover_user"},
		fieldUserID:         {user.ID},
		fieldMasterPassword: {masterPassword},
		fieldTempPassword:   {"temporary handover"},
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if stored, _ := a.Store.User(user.ID); !stored.EscrowRecoveredAt.IsZero() {
		t.Error("a plain user recovered an account")
	}
}
