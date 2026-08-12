// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/dav"
)

func TestConnectDAVRequiresAuth(t *testing.T) {
	a := newApp(t, nil)
	rec := a.get("/app/")
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if !strings.HasPrefix(rec.Header().Get("Location"), "/login") {
		t.Fatalf("Location = %q, want /login", rec.Header().Get("Location"))
	}
}

func TestConnectDAVMockServer(t *testing.T) {
	srv := startMockDAV(t)
	defer srv.Close()

	a := newApp(t, nil)
	a.Guard = dav.NewGuard(dav.GuardConfig{Allowlist: []string{"127.0.0.1"}})
	a.setupAdmin("root", "", testPassword)

	resp := a.post("/app/", url.Values{
		fieldAction:   {"connect_dav"},
		fieldDAVLabel: {"Work"},
		fieldDAVURL:   {srv.URL + "/"},
		fieldDAVUser:  {"mix"},
		fieldDAVPass:  {"secret"},
	})
	if resp.Code != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.Code, body)
	}
	if !strings.Contains(resp.Body.String(), "Work") {
		t.Fatal("response should list connected account")
	}

	sess := a.session()
	count, err := a.Store.DAVAccountCount(sess.UserID, sess.DEK())
	if err != nil || count != 1 {
		t.Fatalf("DAVAccountCount = %d, err = %v", count, err)
	}
}

func TestRefreshCacheRedirects(t *testing.T) {
	a := newApp(t, nil)
	bootstrap(t, a)
	loginAdmin(t, a)

	req := httptest.NewRequest(http.MethodPost, "/app/", strings.NewReader(url.Values{
		fieldAction: {"refresh_cache"},
		CSRFField:   {a.token()},
	}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", "http://example.com/admin/")
	req.AddCookie(&http.Cookie{Name: SessionCookie, Value: a.cookies[SessionCookie]})

	resp := a.do(req)
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", resp.Code)
	}
	if loc := resp.Header().Get("Location"); loc != "http://example.com/admin/" {
		t.Fatalf("Location = %q", loc)
	}
}

func startMockDAV(t *testing.T) *httptest.Server {
	t.Helper()
	const (
		principal = "/principals/user/"
		calHome   = "/calendars/user/"
		abHome    = "/addressbooks/user/"
	)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := normalizeMockPath(r.URL.Path)
		switch {
		case r.Method == http.MethodHead && strings.HasPrefix(path, "/.well-known/"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == "PROPFIND" && path == "/":
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusMultiStatus)
			io.WriteString(w, `<?xml version="1.0"?><d:multistatus xmlns:d="DAV:"><d:response><d:href>/</d:href><d:propstat><d:prop><d:current-user-principal><d:href>`+principal+`</d:href></d:current-user-principal></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response></d:multistatus>`)
		case r.Method == "PROPFIND" && path == normalizeMockPath(principal):
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusMultiStatus)
			io.WriteString(w, `<?xml version="1.0"?><d:multistatus xmlns:d="DAV:" xmlns:cal="urn:ietf:params:xml:ns:caldav" xmlns:card="urn:ietf:params:xml:ns:carddav"><d:response><d:href>`+principal+`</d:href><d:propstat><d:prop><cal:calendar-home-set><d:href>`+calHome+`</d:href></cal:calendar-home-set><card:addressbook-home-set><d:href>`+abHome+`</d:href></card:addressbook-home-set></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response></d:multistatus>`)
		case r.Method == "PROPFIND" && path == normalizeMockPath(calHome):
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusMultiStatus)
			io.WriteString(w, `<?xml version="1.0"?><d:multistatus xmlns:d="DAV:" xmlns:cal="urn:ietf:params:xml:ns:caldav" xmlns:cs="http://calendarserver.org/ns/"><d:response><d:href>`+calHome+`default/</d:href><d:propstat><d:prop><d:displayname>Cal</d:displayname><d:resourcetype><cal:calendar/></d:resourcetype><cs:getctag>1</cs:getctag></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response></d:multistatus>`)
		case r.Method == "PROPFIND" && path == normalizeMockPath(abHome):
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusMultiStatus)
			io.WriteString(w, `<?xml version="1.0"?><d:multistatus xmlns:d="DAV:" xmlns:card="urn:ietf:params:xml:ns:carddav" xmlns:cs="http://calendarserver.org/ns/"><d:response><d:href>`+abHome+`default/</d:href><d:propstat><d:prop><d:displayname>AB</d:displayname><d:resourcetype><card:addressbook/></d:resourcetype><cs:getctag>1</cs:getctag></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response></d:multistatus>`)
		default:
			http.NotFound(w, r)
		}
	}))
}

func normalizeMockPath(p string) string {
	if p == "" {
		return "/"
	}
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}
