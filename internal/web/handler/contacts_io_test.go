// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/config"
	"gitea.mixdep.ru/mix/carrel/internal/dav"
)

func TestContactsImportExportAndPrintChrome(t *testing.T) {
	davSrv := startCardDAVBook(t)
	defer davSrv.Close()

	a := newApp(t, nil)
	a.Guard = dav.NewGuard(dav.GuardConfig{Allowlist: []string{"127.0.0.1"}})
	a.Import = config.Import{MaxBytes: 1 << 20, MaxCards: 100}
	a.setupAdmin("root", "", testPassword)
	accID, colEnc := a.connectAddressBook(t, davSrv.URL)

	list := a.get("/app/contacts/" + accID + "/" + colEnc)
	if list.Code != http.StatusOK {
		t.Fatalf("list = %d", list.Code)
	}
	body := list.Body.String()
	if !strings.Contains(body, "Import") || !strings.Contains(body, "Export") || !strings.Contains(body, "data-print") {
		t.Fatalf("list missing import/export/print controls:\n%s", body)
	}
	if !strings.Contains(body, "print-footer") || !strings.Contains(body, "source-label") {
		t.Fatalf("list missing print metadata:\n%s", body)
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField(CSRFField, a.token())
	_ = mw.WriteField("action", "preview_import")
	part, err := mw.CreateFormFile("file", "people.vcf")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(part, "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:ada\r\nFN:Ada Clone\r\nEND:VCARD\r\n"+
		"BEGIN:VCARD\r\nVERSION:3.0\r\nUID:new-person\r\nFN:Новый\r\nTEL:+7-900\r\nEND:VCARD\r\n")
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	preview := a.postRaw("/app/contacts/"+accID+"/"+colEnc+"/import", mw.FormDataContentType(), &buf)
	if preview.Code != http.StatusOK {
		t.Fatalf("preview = %d %s", preview.Code, preview.Body.String())
	}
	prev := preview.Body.String()
	if !strings.Contains(prev, "new UID will be assigned") {
		t.Fatalf("expected collision note:\n%s", prev)
	}
	if !strings.Contains(prev, "Новый") {
		t.Fatalf("cyrillic name missing:\n%s", prev)
	}

	confirm := a.post("/app/contacts/"+accID+"/"+colEnc+"/import", url.Values{
		"action": {"confirm_import"},
	})
	if confirm.Code != http.StatusOK {
		t.Fatalf("confirm = %d %s", confirm.Code, confirm.Body.String())
	}
	if !strings.Contains(confirm.Body.String(), "Imported 2") {
		t.Fatalf("report = %s", confirm.Body.String())
	}

	exp := a.get("/app/contacts/" + accID + "/" + colEnc + "/export")
	if exp.Code != http.StatusOK {
		t.Fatalf("export = %d", exp.Code)
	}
	cd := exp.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") || !strings.Contains(cd, ".vcf") {
		t.Fatalf("Content-Disposition = %q", cd)
	}
	exported := exp.Body.String()
	if !strings.Contains(exported, "BEGIN:VCARD") || !strings.Contains(exported, "Ada Lovelace") {
		t.Fatalf("export body = %s", exported)
	}
	if !strings.Contains(exported, "Новый") && !strings.Contains(exported, "Ada Clone") {
		t.Fatalf("imported cards missing from export:\n%s", exported)
	}
}

func (a *app) postRaw(path, contentType string, body *bytes.Buffer) *httptest.ResponseRecorder {
	a.t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set(CSRFHeader, a.token())
	return a.do(req)
}
