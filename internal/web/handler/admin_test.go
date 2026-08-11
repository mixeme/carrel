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

func TestAdminDeleteRequiresLoginConfirm(t *testing.T) {
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
		"action":        {"delete_user"},
		"user_id":       {bobID},
		"confirm_login": {"wrong"},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("wrong confirm = %d, want 400", rec.Code)
	}

	rec = a.post("/admin/", url.Values{
		"action":        {"delete_user"},
		"user_id":       {bobID},
		"confirm_login": {"bob"},
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

	rec := a.get("/admin/?audit_action=login")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /admin/ = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "login") {
		t.Error("audit filter page missing login action")
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
