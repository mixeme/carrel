// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package desktop

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestIsAuthSurface(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/login", true},
		{"/login/", true},
		{"/carrel/login", true},
		{"/setup", true},
		{"/register", true},
		{"/forgot", true},
		{"/app/calendar", false},
		{"/about", false},
		{"/", false},
		{"/__carrel_signout", false},
	}
	for _, tc := range tests {
		if got := isAuthSurface(tc.path); got != tc.want {
			t.Errorf("isAuthSurface(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestIsInAppPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/app", true},
		{"/app/", true},
		{"/app/calendar", true},
		{"/base/app/notes", true},
		{"/admin", true},
		{"/admin/users", true},
		{"/login", false},
		{"/about", false},
		{"/", false},
	}
	for _, tc := range tests {
		if got := isInAppPath(tc.path); got != tc.want {
			t.Errorf("isInAppPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestShellSignOutURL(t *testing.T) {
	got := shellSignOutURL("http://wails.localhost/")
	if got != "http://wails.localhost/__carrel_signout" {
		t.Fatalf("got %q", got)
	}
	got = shellSignOutURL("wails://wails/")
	if got != "wails://wails/__carrel_signout" {
		t.Fatalf("linux origin: %q", got)
	}
	got = shellSignOutURL("http://wails.localhost:34115/index.html")
	if got != "http://wails.localhost:34115/__carrel_signout" {
		t.Fatalf("dev origin: %q", got)
	}
}

func TestSignOutWatchScriptSkipsWails(t *testing.T) {
	s := signOutWatchScript("http://wails.localhost/__carrel_signout")
	if !strings.Contains(s, "wails") || !strings.Contains(s, inAppStorageKey) {
		t.Fatal("watch script missing markers")
	}
	if !strings.Contains(s, "/__carrel_signout") {
		t.Fatal("watch script missing sign-out URL")
	}
}

func TestAssetMiddlewareSignOut(t *testing.T) {
	app := &App{target: "http://127.0.0.1:9"}
	inner := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("inner handler should not run")
	})
	h := app.assetMiddleware(inner)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, signOutPath, nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Fatalf("Location = %q", loc)
	}
	if app.target != "" {
		t.Fatalf("target still %q", app.target)
	}
}
