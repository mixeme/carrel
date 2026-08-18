// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQuickNoteSheetIsAFragment(t *testing.T) {
	box := startCalBox(t)
	a, accID, colEnc := calendarApp(t, box)

	req := httptest.NewRequest(http.MethodGet, "/app/notes/quick?back=/app/calendar", nil)
	req.Header.Set("HX-Request", "true")
	rec := a.do(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("quick sheet = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "app-rail") {
		t.Fatalf("quick note sheet includes the page shell:\n%s", body)
	}
	for _, want := range []string{
		`class="app-sheet"`,
		`data-quick-note-form`,
		`data-sheet-close`,
		"Save note",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("quick note sheet missing %q", want)
		}
	}
	_ = accID
	_ = colEnc
}

func TestQuickNotePageStillRendersFullShell(t *testing.T) {
	box := startCalBox(t)
	a, _, _ := calendarApp(t, box)

	rec := a.get("/app/notes/quick")
	if rec.Code != http.StatusOK {
		t.Fatalf("quick page = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "app-rail") {
		t.Fatal("direct /app/notes/quick should still render the full shell")
	}
	if !strings.Contains(body, `data-quick-note-form`) {
		t.Fatal("quick note page should still carry the form")
	}
}

func TestShellHasQuickNoteAndCreateMenu(t *testing.T) {
	a := newApp(t, nil)
	a.setupAdmin("root", "root@example.org", testPassword)

	rec := a.get("/app/calendar")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /app/calendar = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`data-quick-note-open`,
		`data-create-menu-toggle`,
		`hx-target="#app-sheet"`,
		`id="app-overlay"`,
		`Contact`,
		`Event`,
		`Task`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("shell missing %q", want)
		}
	}
}
