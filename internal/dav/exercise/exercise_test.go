// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package exercise

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
)

func TestRunExercisesCalendarAddressBookAndFiles(t *testing.T) {
	const (
		principal = "/principals/user/"
		calHome   = "/calendars/user/"
		abHome    = "/addressbooks/user/"
		calCol    = calHome + "default/"
		abCol     = abHome + "default/"
		filesRoot = "/files/"
	)

	store := map[string]*mockObject{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := normalizePath(r.URL.Path)
		switch {
		case r.Method == http.MethodHead && strings.HasPrefix(path, "/.well-known/"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == "PROPFIND" && path == "/":
			writeMS(w, principalMS(principal))
		case r.Method == "PROPFIND" && path == normalizePath(principal):
			writeMS(w, homeSetsMS(calHome, abHome))
		case r.Method == "PROPFIND" && path == normalizePath(calHome):
			writeMS(w, calendarMS(calCol))
		case r.Method == "PROPFIND" && path == normalizePath(abHome):
			writeMS(w, addressbookMS(abCol))
		case r.Method == "PROPFIND" && path == normalizePath(filesRoot):
			writeMS(w, filesMS(filesRoot))
		case r.Method == "PUT":
			body, _ := io.ReadAll(r.Body)
			if r.Header.Get("If-None-Match") == "*" && store[path] != nil {
				http.Error(w, "exists", http.StatusPreconditionFailed)
				return
			}
			if match := r.Header.Get("If-Match"); match != "" && match != `"stale"` {
				if obj := store[path]; obj == nil || obj.etag != match {
					http.Error(w, "conflict", http.StatusPreconditionFailed)
					return
				}
			}
			if match := r.Header.Get("If-Match"); match == `"stale"` {
				http.Error(w, "conflict", http.StatusPreconditionFailed)
				return
			}
			etag := `"` + fmt.Sprintf("%d", len(store)+1) + `"`
			store[path] = &mockObject{etag: etag, body: string(body)}
			w.Header().Set("ETag", etag)
			w.WriteHeader(http.StatusCreated)
		case r.Method == "GET":
			obj := store[path]
			if obj == nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("ETag", obj.etag)
			_, _ = io.WriteString(w, obj.body)
		case r.Method == "DELETE":
			if store[path] == nil {
				http.NotFound(w, r)
				return
			}
			delete(store, path)
			w.WriteHeader(http.StatusNoContent)
		case r.Method == "REPORT":
			reportBody, _ := io.ReadAll(r.Body)
			raw := string(reportBody)
			switch {
			case strings.Contains(raw, "calendar-query"):
				writeMS(w, calendarQueryMS(store, calCol))
			case strings.Contains(raw, "calendar-multiget"):
				writeMS(w, calendarMultigetMS(store, raw))
			case strings.Contains(raw, "addressbook-query"):
				writeMS(w, addressbookQueryMS(store, abCol, raw))
			case strings.Contains(raw, "addressbook-multiget"):
				writeMS(w, addressbookMultigetMS(store, raw))
			default:
				http.Error(w, "unknown report", http.StatusBadRequest)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	g := dav.NewGuard(dav.GuardConfig{Allowlist: []string{"127.0.0.1"}})
	result, discTrace, err := discovery.Discover(context.Background(), g, discovery.Credentials{
		BaseURL:  srv.URL + "/",
		Username: "mix",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("Discover: %v\ntrace: %+v", err, discTrace)
	}
	result.Collections = append(result.Collections, discovery.Collection{
		Path:     filesRoot,
		Kind:     discovery.KindFiles,
		ReadOnly: false,
	})

	client, err := dav.NewClient(g, result.BaseURL, "mix", "secret")
	if err != nil {
		t.Fatal(err)
	}

	trace := &discovery.Trace{}
	if err := Run(context.Background(), client, result, trace); err != nil {
		t.Fatalf("Run: %v\ntrace: %+v", err, trace)
	}
	for _, step := range trace.Steps {
		if step.Detail != "ok" && step.Detail != "412 as expected" {
			t.Fatalf("step %s = %q", step.Name, step.Detail)
		}
	}
	if len(store) != 0 {
		t.Fatalf("objects left behind: %d", len(store))
	}
}

type mockObject struct {
	etag string
	body string
}

func normalizePath(p string) string {
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if len(p) > 1 && !strings.HasSuffix(p, "/") && strings.Count(p, ".") == 0 {
		// keep file paths without trailing slash
	}
	return p
}

func writeMS(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = io.WriteString(w, body)
}

func principalMS(principal string) string {
	return fmt.Sprintf(`<multistatus xmlns="DAV:"><response><href>/</href><propstat><prop><current-user-principal><href>%s</href></current-user-principal></prop><status>HTTP/1.1 200 OK</status></propstat></response></multistatus>`, principal)
}

func homeSetsMS(calHome, abHome string) string {
	return fmt.Sprintf(`<multistatus xmlns="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav" xmlns:card="urn:ietf:params:xml:ns:carddav"><response><href>/principals/user/</href><propstat><prop><c:calendar-home-set><href>%s</href></c:calendar-home-set><card:addressbook-home-set><href>%s</href></card:addressbook-home-set></prop><status>HTTP/1.1 200 OK</status></propstat></response></multistatus>`, calHome, abHome)
}

func calendarMS(col string) string {
	return fmt.Sprintf(`<multistatus xmlns="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav"><response><href>%s</href><propstat><prop><displayname>Calendar</displayname><resourcetype><collection/><c:calendar/></resourcetype><c:supported-calendar-component-set><c:comp name="VEVENT"/></c:supported-calendar-component-set></prop><status>HTTP/1.1 200 OK</status></propstat></response></multistatus>`, col)
}

func addressbookMS(col string) string {
	return fmt.Sprintf(`<multistatus xmlns="DAV:" xmlns:card="urn:ietf:params:xml:ns:carddav"><response><href>%s</href><propstat><prop><displayname>Contacts</displayname><resourcetype><collection/><card:addressbook/></resourcetype></prop><status>HTTP/1.1 200 OK</status></propstat></response></multistatus>`, col)
}

func filesMS(root string) string {
	return fmt.Sprintf(`<multistatus xmlns="DAV:"><response><href>%s</href><propstat><prop><resourcetype><collection/></resourcetype></prop><status>HTTP/1.1 200 OK</status></propstat></response></multistatus>`, root)
}

func calendarQueryMS(store map[string]*mockObject, col string) string {
	var b strings.Builder
	b.WriteString(`<multistatus xmlns="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">`)
	for path, obj := range store {
		if !strings.HasPrefix(path, col) || !strings.HasSuffix(path, ".ics") {
			continue
		}
		fmt.Fprintf(&b, `<response><href>%s</href><propstat><prop><getetag>%s</getetag><c:calendar-data>%s</c:calendar-data></prop><status>HTTP/1.1 200 OK</status></propstat></response>`, xmlEscape(path), obj.etag, xmlEscape(obj.body))
	}
	b.WriteString(`</multistatus>`)
	return b.String()
}

func calendarMultigetMS(store map[string]*mockObject, raw string) string {
	return multigetMS(store, raw, "calendar-data")
}

func addressbookQueryMS(store map[string]*mockObject, col, raw string) string {
	var b strings.Builder
	b.WriteString(`<multistatus xmlns="DAV:" xmlns:card="urn:ietf:params:xml:ns:carddav">`)
	for path, obj := range store {
		if !strings.HasPrefix(path, col) || !strings.HasSuffix(path, ".vcf") {
			continue
		}
		if !strings.Contains(obj.body, marker) {
			continue
		}
		fmt.Fprintf(&b, `<response><href>%s</href><propstat><prop><getetag>%s</getetag><card:address-data>%s</card:address-data></prop><status>HTTP/1.1 200 OK</status></propstat></response>`, xmlEscape(path), obj.etag, xmlEscape(obj.body))
	}
	b.WriteString(`</multistatus>`)
	return b.String()
}

func addressbookMultigetMS(store map[string]*mockObject, raw string) string {
	return multigetMS(store, raw, "address-data")
}

func multigetMS(store map[string]*mockObject, raw, dataElem string) string {
	var b strings.Builder
	b.WriteString(`<multistatus xmlns="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav" xmlns:card="urn:ietf:params:xml:ns:carddav">`)
	for path, obj := range store {
		if !strings.Contains(raw, xmlEscape(path)) && !strings.Contains(raw, path) {
			continue
		}
		ns := "c"
		if dataElem == "address-data" {
			ns = "card"
		}
		fmt.Fprintf(&b, `<response><href>%s</href><propstat><prop><getetag>%s</getetag><`+ns+`:`+dataElem+`>%s</`+ns+`:`+dataElem+`></prop><status>HTTP/1.1 200 OK</status></propstat></response>`, xmlEscape(path), obj.etag, xmlEscape(obj.body))
	}
	b.WriteString(`</multistatus>`)
	return b.String()
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
