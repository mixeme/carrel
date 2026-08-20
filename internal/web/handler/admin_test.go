// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/store"
)

func TestAdminUserListAndDisable(t *testing.T) {
	a := newApp(t, nil)
	bootstrap(t, a)
	loginAdmin(t, a)

	admin, err := a.Store.UserByLogin("root")
	if err != nil {
		t.Fatalf("UserByLogin: %v", err)
	}
	if _, err := a.Store.CreateUserWithPassword(
		store.Actor{ID: admin.ID, Login: admin.Login}, "ada", "ada@example.org", store.RoleUser, testPassword,
	); err != nil {
		t.Fatalf("CreateUserWithPassword: %v", err)
	}

	rec := a.get("/admin/")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin/ = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, ">ada<") {
		t.Errorf("user list missing ada: %s", body)
	}
	if !strings.Contains(body, "DAV") {
		t.Error("user list missing DAV column")
	}

	a.cookies = map[string]string{}
	a.get("/login")
	wantRedirect(t, a.post("/login", url.Values{fieldLogin: {"ada"}, fieldPassword: {testPassword}}), "/app/password")
	rec = a.get("/app/")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("forced password change redirect = %d, want 303", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/app/password" {
		t.Fatalf("redirect = %q, want /app/password", loc)
	}

	a.cookies = map[string]string{}
	loginAdmin(t, a)

	rec = a.post("/admin/", url.Values{
		"action":  {"disable_user"},
		"user_id": {mustUserID(t, a, "ada")},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("disable user = %d: %s", rec.Code, rec.Body.String())
	}

	a.cookies = map[string]string{}
	a.get("/login")
	rec = a.post("/login", url.Values{fieldLogin: {"ada"}, fieldPassword: {testPassword}})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("disabled user login = %d, want 401", rec.Code)
	}
}

func TestProfilePasswordChange(t *testing.T) {
	a := newApp(t, nil)
	bootstrap(t, a)

	admin, err := a.Store.UserByLogin("root")
	if err != nil {
		t.Fatalf("UserByLogin: %v", err)
	}
	if _, err := a.Store.CreateUserWithPassword(
		store.Actor{ID: admin.ID, Login: admin.Login}, "ada", "", store.RoleUser, testPassword,
	); err != nil {
		t.Fatalf("CreateUserWithPassword: %v", err)
	}

	a.get("/login")
	a.post("/login", url.Values{fieldLogin: {"ada"}, fieldPassword: {testPassword}})

	newPass := "brand new horse battery"
	rec := a.post("/app/password", url.Values{
		fieldCurrentPassword: {testPassword},
		fieldNewPassword:     {newPass},
		fieldConfirm:         {newPass},
	})
	wantRedirect(t, rec, "/app/")

	if sess := a.session(); sess == nil || sess.MustChangePassword() {
		t.Error("password change did not clear the forced-change flag")
	}

	a.post("/logout", url.Values{})
	a.get("/login")
	wantRedirect(t, a.post("/login", url.Values{fieldLogin: {"ada"}, fieldPassword: {newPass}}), "/app/")
}

func TestAdminDeleteUser(t *testing.T) {
	a := newApp(t, nil)
	bootstrap(t, a)
	loginAdmin(t, a)

	admin, err := a.Store.UserByLogin("root")
	if err != nil {
		t.Fatalf("UserByLogin: %v", err)
	}
	if _, err := a.Store.CreateUserWithPassword(
		store.Actor{ID: admin.ID, Login: admin.Login}, "bob", "", store.RoleUser, testPassword,
	); err != nil {
		t.Fatalf("CreateUserWithPassword: %v", err)
	}
	bobID := mustUserID(t, a, "bob")

	rec := a.post("/admin/", url.Values{
		"action":  {"delete_user"},
		"user_id": {bobID},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("delete user = %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := a.Store.UserByLogin("bob"); err == nil {
		t.Error("bob still exists after deletion")
	}
}

func TestAdminAuditLogFilter(t *testing.T) {
	a := newApp(t, nil)
	bootstrap(t, a)
	loginAdmin(t, a)

	rec := a.get("/admin/audit?audit_action=login")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin/audit = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "login") {
		t.Error("audit filter page missing login action")
	}
}

func TestAdminSectionsAreSeparatePages(t *testing.T) {
	a := newApp(t, nil)
	bootstrap(t, a)
	loginAdmin(t, a)

	users := a.get("/admin/").Body.String()
	if !strings.Contains(users, ">root<") {
		t.Error("users page missing the administrator")
	}
	if strings.Contains(users, `value="enable_escrow"`) {
		t.Error("users page still carries the escrow form")
	}
	if strings.Contains(users, `value="save_smtp"`) {
		t.Error("users page still carries the mail form")
	}
	if strings.Contains(users, `id="audit-action"`) {
		t.Error("users page still carries the audit filter")
	}
	if !strings.Contains(users, `href="/admin/invites"`) {
		t.Error("users page missing the subsection navigation")
	}

	if strings.Contains(users, `value="create_user"`) {
		t.Error("users page still carries the create-user form")
	}

	invites := a.get("/admin/invites")
	if invites.Code != http.StatusOK {
		t.Fatalf("GET /admin/invites = %d", invites.Code)
	}
	invitesBody := invites.Body.String()
	if !strings.Contains(invitesBody, `value="create_invite_link"`) {
		t.Error("add-users page missing the invitation form")
	}
	if !strings.Contains(invitesBody, `value="create_user"`) {
		t.Error("add-users page missing the temporary-password form")
	}
	if !strings.Contains(invitesBody, `value="save_self_registration"`) {
		t.Error("add-users page missing the self-registration form")
	}

	settings := a.get("/admin/settings")
	if settings.Code != http.StatusOK {
		t.Fatalf("GET /admin/settings = %d", settings.Code)
	}
	body := settings.Body.String()
	if !strings.Contains(body, `value="save_settings"`) {
		t.Error("settings page missing global settings")
	}
	if !strings.Contains(body, `value="save_smtp"`) {
		t.Error("settings page missing mail settings")
	}
	if strings.Contains(body, `name="creation_mode"`) {
		t.Error("settings page still carries user creation mode")
	}
	if strings.Contains(body, `name="self_registration"`) {
		t.Error("settings page still carries the self-registration checkbox")
	}
	if strings.Contains(body, `value="test_dav"`) {
		t.Error("settings page still carries the DAV validator")
	}

	dav := a.get("/admin/dav")
	if dav.Code != http.StatusOK {
		t.Fatalf("GET /admin/dav = %d", dav.Code)
	}
	if !strings.Contains(dav.Body.String(), `value="test_dav"`) {
		t.Error("DAV validator page missing the discovery form")
	}
	if !strings.Contains(dav.Body.String(), `value="exercise_dav"`) {
		t.Error("DAV validator page missing the full test form")
	}

	escrow := a.get("/admin/escrow")
	if escrow.Code != http.StatusOK {
		t.Fatalf("GET /admin/escrow = %d", escrow.Code)
	}
	if !strings.Contains(escrow.Body.String(), `value="enable_escrow"`) {
		t.Error("escrow page missing the enable form")
	}

	audit := a.get("/admin/audit")
	if audit.Code != http.StatusOK {
		t.Fatalf("GET /admin/audit = %d", audit.Code)
	}
	if !strings.Contains(audit.Body.String(), `id="audit-action"`) {
		t.Error("audit page missing the filter")
	}

	if rec := a.get("/admin/unknown"); rec.Code != http.StatusNotFound {
		t.Errorf("GET /admin/unknown = %d, want 404", rec.Code)
	}
}

func TestCreateUserWithTemporaryPassword(t *testing.T) {
	a := newApp(t, nil)
	bootstrap(t, a)
	loginAdmin(t, a)

	rec := a.post("/admin/invites", url.Values{
		"action":        {"create_user"},
		"login":         {"ada"},
		"email":         {"ada@example.org"},
		"role":          {"user"},
		"temp_password": {testPassword},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("create user = %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Created ada") {
		t.Errorf("missing created notice: %s", rec.Body.String())
	}

	u, err := a.Store.UserByLogin("ada")
	if err != nil {
		t.Fatalf("UserByLogin: %v", err)
	}
	if !u.MustChangePassword {
		t.Error("temporary password must force a change")
	}
}

func TestSelfRegistrationRequiresSMTP(t *testing.T) {
	a := newApp(t, nil)
	bootstrap(t, a)
	loginAdmin(t, a)

	rec := a.post("/admin/invites", url.Values{
		"action":            {"save_self_registration"},
		"self_registration": {"1"},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("enable without SMTP = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func mustUserID(t *testing.T, a *app, login string) string {
	t.Helper()
	u, err := a.Store.UserByLogin(login)
	if err != nil {
		t.Fatalf("UserByLogin(%q): %v", login, err)
	}
	return u.ID
}
