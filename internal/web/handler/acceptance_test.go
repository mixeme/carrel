// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/crypto"
	"gitea.mixdep.ru/mix/carrel/internal/store"
)

// The stage-1 acceptance run of §21, walked in one piece over the real
// middleware chain: an empty volume, the first administrator, an invitation
// handed over by link alone, the account it creates, and that account signing
// in on its own afterwards.
func TestStageOneAcceptanceFlow(t *testing.T) {
	a := newApp(t, nil)

	// An empty volume offers nothing but the setup screen.
	wantRedirect(t, a.get("/"), "/setup")
	if !a.Store.NeedsBootstrap() {
		t.Fatal("a fresh volume did not report itself as needing bootstrap")
	}

	bootstrap(t, a)
	loginAdmin(t, a)

	// SMTP is untouched throughout, so the link is the only way the invitation
	// can reach anyone (§21).
	if a.Store.Settings().SMTP.Configured() {
		t.Fatal("SMTP is configured on a fresh volume")
	}
	token := a.createInvite(string(store.RoleUser))

	// Until the invited user accepts, there is nothing to authenticate them
	// with and nothing that could decrypt their data (§21).
	if _, err := a.Store.UserByLogin("ada"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("an account existed before the invitation was accepted: %v", err)
	}
	if _, _, err := a.Store.Authenticate("ada", testPassword); err == nil {
		t.Error("the invited login authenticated before anyone set a password")
	}

	// The administrator hands the link over and steps out of the way.
	a.post("/logout", url.Values{})
	a.cookies = map[string]string{}

	if rec := a.get("/invite/" + token); rec.Code != http.StatusOK {
		t.Fatalf("GET the invite link = %d, want 200", rec.Code)
	}
	rec := a.post("/invite/"+token, url.Values{
		fieldLogin:    {"ada"},
		fieldEmail:    {"ada@example.org"},
		fieldPassword: {testPassword},
		fieldConfirm:  {testPassword},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("accepting the invitation = %d, want 303 (body: %s)", rec.Code, rec.Body.String())
	}

	user, err := a.Store.UserByLogin("ada")
	if err != nil {
		t.Fatalf("the accepted invitation created no account: %v", err)
	}
	if !user.Activated() || len(user.WrappedDEK) == 0 {
		t.Error("the new account has no credentials of its own")
	}
	if user.Role != store.RoleUser {
		t.Errorf("role = %q, want %q", user.Role, store.RoleUser)
	}

	// The token is spent: a second bearer of the same link gets nowhere, and
	// only its digest was ever stored (§21).
	a.post("/logout", url.Values{})
	a.cookies = map[string]string{}
	if rec := a.get("/invite/" + token); rec.Code != http.StatusGone {
		t.Errorf("a used invite link answered %d, want 410", rec.Code)
	}
	for _, inv := range a.Store.Invites() {
		if inv.Login != "ada" {
			continue
		}
		if !crypto.Equal(inv.TokenHash, crypto.HashToken(token)) {
			t.Error("the stored digest does not match the token that was issued")
		}
		if strings.Contains(string(inv.TokenHash), token) {
			t.Error("the raw token was stored alongside its digest")
		}
	}

	// The invited user signs in with the password they chose themselves.
	a.get("/login")
	wantRedirect(t, a.post("/login", url.Values{
		fieldLogin:    {"ada"},
		fieldPassword: {testPassword},
	}), "/app/")
	if sess := a.session(); sess == nil || sess.DEK() == nil {
		t.Fatal("the invited user's session carries no data key")
	}
	if sess := a.session(); sess.MustChangePassword() {
		t.Error("a self-chosen password was marked as needing a change")
	}
	if rec := a.get("/app/settings/connections"); rec.Code != http.StatusOK {
		t.Errorf("GET /app/settings/connections = %d, want 200", rec.Code)
	}
	if rec := a.get("/admin/"); rec.Code != http.StatusForbidden {
		t.Errorf("the invited user reached the panel: status %d", rec.Code)
	}
}

// With escrow off, an administrator holds nothing that opens another account:
// the data key never leaves the owner's session (§21).
func TestAdminCannotReadAnotherAccountsData(t *testing.T) {
	a := newApp(t, nil)
	bootstrap(t, a)
	loginAdmin(t, a)
	token := a.createInvite(string(store.RoleUser))
	// Kept past the administrator's logout, which wipes the session copy.
	adminDEK := a.session().DEK().Clone()

	a.post("/logout", url.Values{})
	a.cookies = map[string]string{}
	a.get("/invite/" + token)
	a.post("/invite/"+token, url.Values{
		fieldLogin:    {"ada"},
		fieldPassword: {testPassword},
		fieldConfirm:  {testPassword},
	})

	// Stand in for the DAV credentials of stage 2: a blob sealed under the
	// user's own key, which is the only thing that opens it.
	_, userDEK, err := a.Store.Authenticate("ada", testPassword)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	secret := []byte("dav password for ada")
	sealed, err := crypto.Seal(userDEK, secret, nil)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}

	if crypto.Equal(adminDEK, userDEK) {
		t.Fatal("the two accounts share a data key")
	}
	if _, err := crypto.Open(adminDEK, sealed, nil); !errors.Is(err, crypto.ErrDecrypt) {
		t.Errorf("the administrator's key opened another account's data: %v", err)
	}

	// Nothing on the panel exposes the key material either.
	a.post("/logout", url.Values{})
	a.cookies = map[string]string{}
	loginAdmin(t, a)
	body := a.get("/admin/").Body.String()
	for _, forbidden := range []string{string(secret), testPassword, token} {
		if strings.Contains(body, forbidden) {
			t.Errorf("the panel disclosed %q", forbidden)
		}
	}
	user, err := a.Store.UserByLogin("ada")
	if err != nil {
		t.Fatalf("UserByLogin: %v", err)
	}
	// Escrow is off, so there is no second copy of the key anywhere (§21).
	if len(user.EscrowDEK) != 0 {
		t.Error("a recoverable copy of the data key exists with escrow off")
	}
}

// Disabling an account ends its sessions there and then, not at the next
// login attempt (§21).
func TestDisableEndsActiveSessionsAtOnce(t *testing.T) {
	admin := newApp(t, nil)
	bootstrap(t, admin)
	loginAdmin(t, admin)
	token := admin.createInvite(string(store.RoleUser))

	// A second browser for the invited user, so both sessions are live at the
	// same time.
	user := &app{Server: admin.Server, t: t, handler: admin.handler, cookies: map[string]string{}}
	user.get("/invite/" + token)
	user.post("/invite/"+token, url.Values{
		fieldLogin:    {"ada"},
		fieldPassword: {testPassword},
		fieldConfirm:  {testPassword},
	})
	if rec := user.get("/app/settings/connections"); rec.Code != http.StatusOK {
		t.Fatalf("the invited user is not signed in: status %d", rec.Code)
	}
	userSession := user.session()
	if userSession == nil {
		t.Fatal("no session for the invited user")
	}

	record, err := admin.Store.UserByLogin("ada")
	if err != nil {
		t.Fatalf("UserByLogin: %v", err)
	}
	rec := admin.post("/admin/", url.Values{
		fieldAction: {"disable_user"},
		fieldUserID: {record.ID},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("disable = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	if _, ok := admin.Sessions.Get(userSession.ID); ok {
		t.Error("the disabled account's session survived")
	}
	if userSession.DEK() != nil {
		t.Error("the disabled account's data key was left in memory")
	}
	if rec := user.get("/app/"); rec.Code != http.StatusSeeOther {
		t.Errorf("the disabled user still reached their page: status %d", rec.Code)
	}

	// And they cannot sign back in.
	user.cookies = map[string]string{}
	user.get("/login")
	if rec := user.post("/login", url.Values{
		fieldLogin:    {"ada"},
		fieldPassword: {testPassword},
	}); rec.Code != http.StatusUnauthorized {
		t.Errorf("a disabled account signed in: status %d", rec.Code)
	}
}

// The instance cannot be left without an administrator (§21).
func TestLastAdminSurvivesThePanel(t *testing.T) {
	a := newApp(t, nil)
	bootstrap(t, a)
	loginAdmin(t, a)

	admin, err := a.Store.UserByLogin("root")
	if err != nil {
		t.Fatalf("UserByLogin: %v", err)
	}

	for _, tc := range []struct {
		name string
		form url.Values
	}{
		{"delete", url.Values{fieldAction: {"delete_user"}, fieldUserID: {admin.ID}}},
		{"demote", url.Values{fieldAction: {"change_role"}, fieldUserID: {admin.ID}, fieldRole: {string(store.RoleUser)}}},
		{"disable", url.Values{fieldAction: {"disable_user"}, fieldUserID: {admin.ID}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := a.post("/admin/", tc.form)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), "last administrator") {
				t.Errorf("the page does not explain the refusal: %s", rec.Body.String())
			}
			still, err := a.Store.UserByLogin("root")
			if err != nil || !still.IsAdmin() || still.Disabled {
				t.Errorf("the last administrator was changed anyway: %+v (%v)", still, err)
			}
		})
	}
}

// A destructive reset is spelled out before it is confirmed, and the escrow
// alternative is named (§5.5, §21).
func TestResetPasswordIsAnnouncedAsDestructive(t *testing.T) {
	a := newApp(t, nil)
	bootstrap(t, a)
	loginAdmin(t, a)

	body := a.get("/admin/").Body.String()
	idx := strings.Index(body, `value="reset_password"`)
	if idx < 0 {
		t.Fatal("the panel has no password reset form")
	}
	// The warning has to be above the form, where it is read before the button
	// is pressed.
	warning := body[:idx]
	for _, want := range []string{"destructive", "unreadable", "escrow"} {
		if !strings.Contains(strings.ToLower(warning), want) {
			t.Errorf("the reset warning does not mention %q", want)
		}
	}
}

// createInvite creates an invitation from the panel and returns its token,
// taken from the link the administrator is shown.
func (a *app) createInvite(role string) string {
	a.t.Helper()
	rec := a.post("/admin/invites", url.Values{
		fieldAction: {"create_invite_link"},
		fieldRole:   {role},
	})
	if rec.Code != http.StatusOK {
		a.t.Fatalf("create invite = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	field := strings.Index(body, `id="invite-link"`)
	if field < 0 {
		a.t.Fatalf("the panel shows no invitation link:\n%s", body)
	}
	rest := body[field:]
	start := strings.Index(rest, `value="`)
	if start < 0 {
		a.t.Fatal("the invitation link field carries no value")
	}
	rest = rest[start+len(`value="`):]
	link := rest[:strings.Index(rest, `"`)]

	const prefix = "/invite/"
	at := strings.Index(link, prefix)
	if at < 0 {
		a.t.Fatalf("link %q is not an invitation link", link)
	}
	return link[at+len(prefix):]
}
