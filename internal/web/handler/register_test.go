// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/mail"
	"gitea.mixdep.ru/mix/carrel/internal/store"
)

func TestRegisterIsNotFoundWhenClosed(t *testing.T) {
	a := newApp(t, nil)
	bootstrap(t, a)

	if rec := a.get("/register"); rec.Code != http.StatusNotFound {
		t.Fatalf("GET /register while closed = %d, want 404", rec.Code)
	}
	login := a.get("/login")
	if strings.Contains(login.Body.String(), "/register") {
		t.Error("login page advertised registration while it was closed")
	}
}

func TestRegisterLifecycle(t *testing.T) {
	a := newApp(t, nil)
	bootstrap(t, a)
	openRegistration(t, a)

	rec := a.get("/register")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /register = %d, want 200", rec.Code)
	}

	login := a.get("/login")
	if !strings.Contains(login.Body.String(), `href="/register"`) {
		t.Error("login page missing the registration link")
	}

	rec = a.post("/register", url.Values{
		fieldLogin:    {"ada"},
		fieldEmail:    {"ada@example.org"},
		fieldPassword: {testPassword},
		fieldConfirm:  {testPassword},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /register = %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "confirmation link") {
		t.Errorf("missing confirmation notice: %s", rec.Body.String())
	}

	u, err := a.Store.UserByLogin("ada")
	if err != nil {
		t.Fatalf("UserByLogin: %v", err)
	}
	if !u.Unconfirmed {
		t.Fatal("registered account was usable before confirmation")
	}

	a.get("/login")
	rec = a.post("/login", url.Values{fieldLogin: {"ada"}, fieldPassword: {testPassword}})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("unconfirmed login = %d, want 401", rec.Code)
	}

	_, token, err := a.Store.Register("ada", "ada@example.org", testPassword, "")
	if err != nil {
		t.Fatalf("Register resend: %v", err)
	}
	rec = a.get("/confirm-email/" + token)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /confirm-email = %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Your account is ready") {
		t.Errorf("confirmation page: %s", rec.Body.String())
	}

	a.get("/login")
	wantRedirect(t, a.post("/login", url.Values{fieldLogin: {"ada"}, fieldPassword: {testPassword}}), "/app/")
}

func openRegistration(t *testing.T, a *app) {
	t.Helper()
	admin, err := a.Store.UserByLogin("root")
	if err != nil {
		t.Fatalf("UserByLogin: %v", err)
	}
	if err := a.Store.UpdateSettings(store.Actor{ID: admin.ID, Login: admin.Login}, func(st *store.Settings) {
		st.SMTP.Host = "localhost"
		st.SMTP.Port = 25
		st.SMTP.TLS = store.TLSNone
		st.SMTP.FromAddress = "carrel@example.org"
		st.SelfRegistration = true
	}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	a.Mail = &mail.Queue{Store: a.Store}
}
