// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"io/fs"
	"net/http"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/web"
)

func TestListTemplatesExposeDetailLinks(t *testing.T) {
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
		if name != "contacts_page.html" && !strings.Contains(body, `data-detail-section=`) {
			t.Errorf("%s is missing data-detail-section", name)
		}
		if !strings.Contains(body, "detail-link") {
			t.Errorf("%s is missing detail-link on list rows", name)
		}
		if !strings.Contains(body, `hx-target="#app-details"`) {
			t.Errorf("%s does not target the details panel", name)
		}
		if !strings.Contains(body, `/panel"`) {
			t.Errorf("%s does not load a panel fragment", name)
		}
	}
}

func TestContactPanelIsAFragment(t *testing.T) {
	davSrv := startCardDAVBook(t)
	defer davSrv.Close()

	a := newApp(t, nil)
	a.Guard = dav.NewGuard(dav.GuardConfig{Allowlist: []string{"127.0.0.1"}})
	a.setupAdmin("root", "", testPassword)
	accID, colEnc := a.connectAddressBook(t, davSrv.URL)

	rec := a.get("/app/contacts/" + accID + "/" + colEnc + "/ada/panel")
	if rec.Code != http.StatusOK {
		t.Fatalf("panel status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "app-rail") {
		t.Fatalf("panel includes the page shell:\n%s", body)
	}
	wantHref := `href="/app/contacts/` + accID + `/` + colEnc + `/ada/edit"`
	for _, want := range []string{
		`class="detail-panel"`,
		"Ada Lovelace",
		`data-detail-close`,
		wantHref,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("panel body missing %q", want)
		}
	}
}

func TestContactCardStillRendersFullPage(t *testing.T) {
	davSrv := startCardDAVBook(t)
	defer davSrv.Close()

	a := newApp(t, nil)
	a.Guard = dav.NewGuard(dav.GuardConfig{Allowlist: []string{"127.0.0.1"}})
	a.setupAdmin("root", "", testPassword)
	accID, colEnc := a.connectAddressBook(t, davSrv.URL)

	rec := a.get("/app/contacts/" + accID + "/" + colEnc + "/ada")
	if rec.Code != http.StatusOK {
		t.Fatalf("person view status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "app-rail") {
		t.Fatal("direct contact link should still render the full shell")
	}
	if strings.Contains(body, `contact-form`) {
		t.Fatal("person view should not include the edit form")
	}
	if !strings.Contains(body, "Ada Lovelace") {
		t.Fatal("person view should show the contact name")
	}

	edit := a.get("/app/contacts/" + accID + "/" + colEnc + "/ada/edit")
	if edit.Code != http.StatusOK {
		t.Fatalf("edit status = %d", edit.Code)
	}
	if !strings.Contains(edit.Body.String(), `contact-form`) {
		t.Fatal("/edit URL should offer the contact edit form")
	}
}
