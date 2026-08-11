// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestAboutPublic(t *testing.T) {
	a := newApp(t, nil)
	a.Version = "1.2.3"
	a.Commit = "deadbeef"

	rec := a.get("/about")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /about = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"About Carrel",
		"1.2.3",
		"deadbeef",
		SourceURL,
		"AGPL-3.0-or-later",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("GET /about body missing %q", want)
		}
	}
}

func TestAboutFooterOnSignedInPages(t *testing.T) {
	a := newApp(t, nil)
	a.setupAdmin("root", "root@example.org", testPassword)

	rec := a.get("/app/")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /app/ = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `href="/about"`) {
		t.Error("signed-in page has no footer link to /about")
	}
	if !strings.Contains(rec.Body.String(), `data-refresh`) {
		t.Error("signed-in page has no refresh control")
	}
}

func TestManifest(t *testing.T) {
	a := newApp(t, nil)

	rec := a.get("/manifest.webmanifest")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET manifest = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/manifest+json") {
		t.Errorf("Content-Type = %q, want application/manifest+json", ct)
	}

	var manifest map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &manifest); err != nil {
		t.Fatalf("manifest JSON: %v", err)
	}
	if got := manifest["start_url"]; got != "/app/" {
		t.Errorf("start_url = %v, want /app/", got)
	}
	icons, ok := manifest["icons"].([]any)
	if !ok || len(icons) == 0 {
		t.Fatal("manifest has no icons")
	}
	icon, ok := icons[0].(map[string]any)
	if !ok || icon["src"] != "/static/icon.svg" {
		t.Errorf("icon src = %v, want /static/icon.svg", icon["src"])
	}
}

func TestAboutNoSessionRequired(t *testing.T) {
	a := newApp(t, nil)
	rec := a.get("/about")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /about = %d, want 200", rec.Code)
	}
}
