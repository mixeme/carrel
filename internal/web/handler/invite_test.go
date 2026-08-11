// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestInviteLifecycle(t *testing.T) {
	a := newApp(t, nil)

	a.setupAdmin("root", "root@example.org", testPassword)
	a.get("/admin/")

	form := url.Values{
		"action": {"create_invite_link"},
		"role":   {"user"},
	}
	rec := a.post("/admin/", form)
	if rec.Code != http.StatusOK {
		t.Fatalf("create invite status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "/invite/") {
		t.Fatalf("admin page missing invite link:\n%s", body)
	}

	const marker = `value="http://`
	idx := strings.Index(body, marker)
	if idx < 0 {
		t.Fatal("no invite link in response")
	}
	rest := body[idx+len(`value="`):]
	end := strings.Index(rest, `"`)
	link := rest[:end]
	token := strings.TrimPrefix(link, "http://example.com/invite/")

	accept := url.Values{
		fieldLogin:    {"ada"},
		fieldEmail:    {"ada@example.org"},
		fieldPassword: {testPassword},
		fieldConfirm:  {testPassword},
	}
	a.get("/invite/" + token)
	rec = a.post("/invite/"+token, accept)
	if rec.Code != http.StatusSeeOther && rec.Code != http.StatusNoContent {
		t.Fatalf("accept invite status = %d, want redirect", rec.Code)
	}

	a.post("/logout", url.Values{})
	a.get("/login")
	rec = a.post("/login", url.Values{
		fieldLogin:    {"ada"},
		fieldPassword: {testPassword},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("login as invited user status = %d", rec.Code)
	}
	rec = a.get("/app/")
	if rec.Code != http.StatusOK {
		t.Fatalf("app home status = %d", rec.Code)
	}
}

func (a *app) setupAdmin(login, email, password string) {
	a.get("/setup")
	rec := a.post("/setup", url.Values{
		fieldLogin:    {login},
		fieldEmail:    {email},
		fieldPassword: {password},
		fieldConfirm:  {password},
	})
	if rec.Code != http.StatusSeeOther && rec.Code != http.StatusNoContent {
		a.t.Fatalf("setup status = %d, want redirect", rec.Code)
	}
}
