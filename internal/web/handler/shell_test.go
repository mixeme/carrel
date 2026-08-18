// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/web"
)

func TestListTemplatesUseListRows(t *testing.T) {
	templateFS, err := fs.Sub(web.TemplateFS, "template")
	if err != nil {
		t.Fatalf("template FS: %v", err)
	}
	for _, name := range []string{"contacts.html", "contacts_page.html", "agenda.html", "tasks.html", "notes.html"} {
		b, err := fs.ReadFile(templateFS, name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		body := string(b)
		if !strings.Contains(body, `class="list-bar"`) {
			t.Errorf("%s is missing the collection colour bar", name)
		}
		if !strings.Contains(body, `class="list-row`) {
			t.Errorf("%s is missing list-row markup", name)
		}
	}
}

func TestEmbeddedBaseHasShell(t *testing.T) {
	templateFS, err := fs.Sub(web.TemplateFS, "template")
	if err != nil {
		t.Fatalf("template FS: %v", err)
	}
	b, err := fs.ReadFile(templateFS, "base.html")
	if err != nil {
		t.Fatalf("read base.html: %v", err)
	}
	if !strings.Contains(string(b), "app-rail") {
		t.Fatal("embedded base.html is missing the application shell")
	}
}

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
	if !strings.Contains(rec.Body.String(), `value="refresh_cache"`) {
		t.Error("signed-in page has no refresh control")
	}
	if !strings.Contains(rec.Body.String(), `class="app-rail`) {
		body := rec.Body.String()
		snippet := body
		if len(snippet) > 600 {
			snippet = snippet[:600]
		}
		t.Fatalf("signed-in app page has no section rail; snippet:\n%s", snippet)
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

func TestShellNavigationMarksCurrentSection(t *testing.T) {
	a := newApp(t, nil)
	a.setupAdmin("root", "root@example.org", testPassword)

	cases := []struct {
		path string
		want string
	}{
		{"/app/contacts", "Contacts"},
		{"/app/calendar", "Calendar"},
		{"/app/tasks", "Tasks"},
		{"/app/notes", "Notes"},
		{"/app/files", "Files"},
		{"/app/search", "Search"},
		{"/app/duplicates", "Duplicates"},
	}

	for _, tc := range cases {
		rec := a.get(tc.path)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d, want 200", tc.path, rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, `class="app-rail`) {
			t.Fatalf("GET %s body missing section rail", tc.path)
		}
		marker := `aria-current="page"` + ">" + tc.want + "<"
		if !strings.Contains(body, marker) {
			t.Errorf("GET %s: want %q marked current in the rail", tc.path, tc.want)
		}
	}
}
