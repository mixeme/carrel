// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package discovery

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/dav"
)

func TestDecodeMockCalendar(t *testing.T) {
	ms, err := dav.ParseMultiStatus(strings.NewReader(calendarMS("/calendars/user/default/")))
	if err != nil {
		t.Fatal(err)
	}
	var resType dav.ResourceType
	if err := ms.Responses[0].DecodeProp(&resType); err != nil {
		t.Fatalf("DecodeProp resourcetype: %v", err)
	}
	if len(resType.Raw) == 0 {
		t.Fatal("resourcetype has no child elements")
	}
	col, ok, err := decodeCollection(ms.Responses[0], KindCalendar)
	if err != nil {
		t.Fatalf("decodeCollection: %v", err)
	}
	if !ok {
		t.Fatalf("decodeCollection did not recognize calendar collection; raw=%+v", resType.Raw)
	}
	if col.DisplayName != "Calendar" {
		t.Fatalf("displayname = %q", col.DisplayName)
	}
}

func TestDiscoverMockServer(t *testing.T) {
	const (
		principal = "/principals/user/"
		calHome   = "/calendars/user/"
		abHome    = "/addressbooks/user/"
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := normalizeMockPath(r.URL.Path)
		switch {
		case r.Method == http.MethodHead && strings.HasPrefix(path, "/.well-known/"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == "PROPFIND" && path == "/":
			writeMS(w, principalMS(principal))
		case r.Method == "PROPFIND" && path == normalizeMockPath(principal):
			writeMS(w, homeSetsMS(calHome, abHome))
		case r.Method == "PROPFIND" && path == normalizeMockPath(calHome):
			writeMS(w, calendarMS(calHome+"default/"))
		case r.Method == "PROPFIND" && path == normalizeMockPath(abHome):
			writeMS(w, addressbookMS(abHome+"default/"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	g := dav.NewGuard(dav.GuardConfig{Allowlist: []string{"127.0.0.1"}})
	result, trace, err := Discover(context.Background(), g, Credentials{
		BaseURL:  srv.URL + "/",
		Username: "mix",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("Discover: %v\ntrace: %+v", err, trace)
	}
	if len(result.Collections) < 2 {
		t.Fatalf("collections = %d, want at least 2", len(result.Collections))
	}
}

// The arrangement §6 calls the main path: Baikal under `/dav.php/`, with the
// site root serving an ordinary web page.
//
// Asking the server root for the principal reaches that page, which answers 200
// with HTML instead of 207 with a multistatus, and discovery fails on a URL that
// is entirely correct. This is what a live Baikal actually does, and no fake
// server caught it until one was pointed at.
func TestDiscoverAtABasePathIgnoresTheSiteRoot(t *testing.T) {
	const (
		base      = "/dav.php/"
		principal = "/dav.php/principals/mix/"
		calHome   = "/dav.php/calendars/mix/"
		abHome    = "/dav.php/addressbooks/mix/"
	)
	rootHits := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := normalizeMockPath(r.URL.Path)
		switch {
		case r.Method == http.MethodHead && strings.HasPrefix(path, "/.well-known/"):
			w.WriteHeader(http.StatusNotFound)
		// The site root is a web page. It answers every method with 200 and no
		// XML at all, exactly as a web server in front of Baikal does.
		case path == "/":
			rootHits++
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "<html><body>nothing to see here</body></html>")
		case r.Method == "PROPFIND" && path == base && r.Header.Get("Depth") == "1":
			writeMS(w, rootListingMS(base, "principals/", "calendars/", "addressbooks/", "shared/"))
		case r.Method == "PROPFIND" && path == base:
			writeMS(w, principalMS(principal))
		case r.Method == "PROPFIND" && path == normalizeMockPath(principal):
			writeMS(w, homeSetsMS(calHome, abHome))
		case r.Method == "PROPFIND" && path == normalizeMockPath(calHome):
			writeMS(w, calendarMS(calHome+"default/"))
		case r.Method == "PROPFIND" && path == normalizeMockPath(abHome):
			writeMS(w, addressbookMS(abHome+"default/"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	g := dav.NewGuard(dav.GuardConfig{Allowlist: []string{"127.0.0.1"}})
	result, trace, err := Discover(context.Background(), g, Credentials{
		BaseURL: srv.URL + base, Username: "mix", Password: "secret",
	})
	if err != nil {
		t.Fatalf("Discover against a base path: %v\ntrace: %+v", err, trace)
	}
	if result.Principal != principal {
		t.Fatalf("principal = %q, want %q", result.Principal, principal)
	}
	var kinds []string
	for _, col := range result.Collections {
		kinds = append(kinds, string(col.Kind))
	}
	if len(result.Collections) != 3 {
		t.Fatalf("collections = %v, want a calendar, an address book and one file collection", kinds)
	}
	// The root may be probed, but it must not be what the answer depends on.
	if rootHits > 0 {
		t.Logf("the site root was reached %d time(s); acceptable as a fallback, not as the first choice", rootHits)
	}
}

// A URL entered without its trailing slash has to work too: people copy
// `https://host/dav.php` out of a wiki page far more often than with the slash.
func TestDiscoverAcceptsABasePathWithoutATrailingSlash(t *testing.T) {
	const principal = "/dav.php/principals/mix/"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := normalizeMockPath(r.URL.Path)
		switch {
		case r.Method == http.MethodHead:
			w.WriteHeader(http.StatusNotFound)
		case path == "/":
			w.WriteHeader(http.StatusOK)
		case r.Method == "PROPFIND" && path == "/dav.php/" && r.Header.Get("Depth") == "1":
			writeMS(w, rootListingMS("/dav.php/", "files/"))
		case r.Method == "PROPFIND" && path == "/dav.php/":
			writeMS(w, principalMS(principal))
		default:
			writeMS(w, `<?xml version="1.0"?><d:multistatus xmlns:d="DAV:"><d:response>`+
				`<d:href>`+path+`</d:href><d:propstat><d:prop/>`+
				`<d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response></d:multistatus>`)
		}
	}))
	defer srv.Close()

	g := dav.NewGuard(dav.GuardConfig{Allowlist: []string{"127.0.0.1"}})
	result, trace, err := Discover(context.Background(), g, Credentials{
		BaseURL: srv.URL + "/dav.php", Username: "mix", Password: "secret",
	})
	if err != nil {
		t.Fatalf("Discover without a trailing slash: %v\ntrace: %+v", err, trace)
	}
	if result.Principal != principal {
		t.Fatalf("principal = %q", result.Principal)
	}
}

// §6: a plain collection under the root is a file collection, and the containers
// the DAV homes live in are not. Baikal answers the root with `calendars/`,
// `addressbooks/` and `principals/`, none of them marked as anything at that
// depth, so telling them apart is the whole of the work here.
func TestDiscoverFindsFileCollectionsAndSkipsTheServiceTrees(t *testing.T) {
	const (
		principal = "/dav/principals/user/"
		calHome   = "/dav/calendars/user/"
		abHome    = "/dav/addressbooks/user/"
		root      = "/dav/"
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := normalizeMockPath(r.URL.Path)
		switch {
		case r.Method == http.MethodHead && strings.HasPrefix(path, "/.well-known/"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == "PROPFIND" && path == root && r.Header.Get("Depth") == "1":
			writeMS(w, rootListingMS(root, "principals/", "calendars/", "addressbooks/", "files/", "shared/"))
		// The principal is looked for at the server root rather than at the base
		// path, which is what the chain of §6 does.
		case r.Method == "PROPFIND" && path == "/":
			writeMS(w, principalMS(principal))
		case r.Method == "PROPFIND" && path == normalizeMockPath(principal):
			writeMS(w, homeSetsMS(calHome, abHome))
		case r.Method == "PROPFIND" && path == normalizeMockPath(calHome):
			writeMS(w, calendarMS(calHome+"default/"))
		case r.Method == "PROPFIND" && path == normalizeMockPath(abHome):
			writeMS(w, addressbookMS(abHome+"default/"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	g := dav.NewGuard(dav.GuardConfig{Allowlist: []string{"127.0.0.1"}})
	result, trace, err := Discover(context.Background(), g, Credentials{
		BaseURL:  srv.URL + root,
		Username: "mix",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("Discover: %v\ntrace: %+v", err, trace)
	}
	var filePaths []string
	for _, col := range result.Collections {
		if col.Kind == KindFiles {
			filePaths = append(filePaths, col.Path)
		}
	}
	want := []string{root + "files/", root + "shared/"}
	if strings.Join(filePaths, ",") != strings.Join(want, ",") {
		t.Fatalf("file collections = %v, want %v", filePaths, want)
	}
}

// A WebDAV server with no calendars and no address books connects, because a
// separate account for files is exactly the arrangement §23.10 expects.
func TestDiscoverAcceptsAFilesOnlyServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := normalizeMockPath(r.URL.Path)
		switch {
		case r.Method == http.MethodHead:
			w.WriteHeader(http.StatusNotFound)
		case r.Method == "PROPFIND" && path == "/" && r.Header.Get("Depth") == "1":
			writeMS(w, rootListingMS("/", "documents/"))
		case r.Method == "PROPFIND" && path == "/":
			writeMS(w, principalMS("/principals/mix/"))
		default:
			// No home-set for either protocol: this server has neither.
			writeMS(w, `<?xml version="1.0"?><d:multistatus xmlns:d="DAV:"><d:response>`+
				`<d:href>`+path+`</d:href><d:propstat><d:prop/>`+
				`<d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response></d:multistatus>`)
		}
	}))
	defer srv.Close()

	g := dav.NewGuard(dav.GuardConfig{Allowlist: []string{"127.0.0.1"}})
	result, trace, err := Discover(context.Background(), g, Credentials{
		BaseURL: srv.URL + "/", Username: "mix", Password: "secret",
	})
	if err != nil {
		t.Fatalf("Discover: %v\ntrace: %+v", err, trace)
	}
	if len(result.Collections) != 1 || result.Collections[0].Kind != KindFiles {
		t.Fatalf("collections = %+v, want one file collection", result.Collections)
	}
}

// A plain WebDAV server — SFTPGo, Apache mod_dav, the Go net/webdav package —
// answers PROPFIND and does not advertise current-user-principal. That property
// is CalDAV/CardDAV. Treating its absence as a failed connection is what made
// a dedicated files account unconnectable: well-known 404s, then the entered
// URL, then "no current-user-principal at /".
func TestDiscoverAcceptsAPlainWebDAVServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := normalizeMockPath(r.URL.Path)
		switch {
		case r.Method == http.MethodHead:
			w.WriteHeader(http.StatusNotFound)
		case r.Method == "PROPFIND" && path == "/":
			writeMS(w, filesOnlyRootMS("/"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	g := dav.NewGuard(dav.GuardConfig{Allowlist: []string{"127.0.0.1"}})
	result, trace, err := Discover(context.Background(), g, Credentials{
		BaseURL: srv.URL + "/", Username: "mix", Password: "secret",
	})
	if err != nil {
		t.Fatalf("Discover: %v\ntrace: %+v", err, trace)
	}
	if result.Principal != "" {
		t.Fatalf("principal = %q, want empty on a files-only server", result.Principal)
	}
	if len(result.Collections) != 1 || result.Collections[0].Kind != KindFiles {
		t.Fatalf("collections = %+v, want the entered URL as one file collection", result.Collections)
	}
	if got := normalizePath(result.Collections[0].Path); got != "/" {
		t.Fatalf("file collection path = %q, want /", result.Collections[0].Path)
	}
}

// The Baikal arrangement in reverse: DAV under a path, an ordinary web page at
// the site root. The root probe must not turn a missing principal on the DAV
// path into a hard failure about HTML.
func TestDiscoverAcceptsPlainWebDAVAtABasePath(t *testing.T) {
	const base = "/webdav/"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := normalizeMockPath(r.URL.Path)
		switch {
		case r.Method == http.MethodHead:
			w.WriteHeader(http.StatusNotFound)
		case path == "/":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "<html><body>nothing to see here</body></html>")
		case r.Method == "PROPFIND" && path == base:
			writeMS(w, filesOnlyRootMS(base))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	g := dav.NewGuard(dav.GuardConfig{Allowlist: []string{"127.0.0.1"}})
	result, trace, err := Discover(context.Background(), g, Credentials{
		BaseURL: srv.URL + base, Username: "mix", Password: "secret",
	})
	if err != nil {
		t.Fatalf("Discover: %v\ntrace: %+v", err, trace)
	}
	if len(result.Collections) != 1 || result.Collections[0].Kind != KindFiles {
		t.Fatalf("collections = %+v, want the entered URL as one file collection", result.Collections)
	}
	if got := normalizePath(result.Collections[0].Path); got != base {
		t.Fatalf("file collection path = %q, want %q", result.Collections[0].Path, base)
	}
}

// A 401 is an authentication failure, not a files-only server.
func TestDiscoverDoesNotTreatAuthFailureAsFiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	g := dav.NewGuard(dav.GuardConfig{Allowlist: []string{"127.0.0.1"}})
	_, _, err := Discover(context.Background(), g, Credentials{
		BaseURL: srv.URL + "/", Username: "mix", Password: "wrong",
	})
	if err == nil {
		t.Fatal("Discover succeeded on 401, want an authentication failure")
	}
	if strings.Contains(err.Error(), "no current-user-principal") {
		t.Fatalf("treated 401 as a missing principal: %v", err)
	}
}

// rootListingMS is a Depth:1 answer for the root: the root itself and the
// members named, every one of them a bare collection.
func rootListingMS(root string, members ...string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	b.WriteString(`<d:multistatus xmlns:d="DAV:">`)
	write := func(path, name string) {
		b.WriteString(`<d:response><d:href>` + path + `</d:href><d:propstat><d:prop>` +
			`<d:resourcetype><d:collection/></d:resourcetype>` +
			`<d:displayname>` + name + `</d:displayname>` +
			`<d:current-user-privilege-set><d:privilege><d:write/></d:privilege></d:current-user-privilege-set>` +
			`</d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`)
	}
	write(root, "root")
	for _, member := range members {
		write(root+member, strings.TrimSuffix(member, "/"))
	}
	b.WriteString(`</d:multistatus>`)
	return b.String()
}

func writeMS(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusMultiStatus)
	io.WriteString(w, body)
}

func filesOnlyRootMS(path string) string {
	return `<?xml version="1.0" encoding="utf-8"?>
<d:multistatus xmlns:d="DAV:">
  <d:response>
    <d:href>` + path + `</d:href>
    <d:propstat>
      <d:prop>
        <d:resourcetype><d:collection/></d:resourcetype>
        <d:displayname>Files</d:displayname>
        <d:current-user-privilege-set><d:privilege><d:write/></d:privilege></d:current-user-privilege-set>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>`
}

func principalMS(principal string) string {
	return `<?xml version="1.0" encoding="utf-8"?>
<d:multistatus xmlns:d="DAV:">
  <d:response>
    <d:href>/</d:href>
    <d:propstat>
      <d:prop>
        <d:current-user-principal><d:href>` + principal + `</d:href></d:current-user-principal>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>`
}

func homeSetsMS(calHome, abHome string) string {
	return `<?xml version="1.0" encoding="utf-8"?>
<d:multistatus xmlns:d="DAV:" xmlns:cal="urn:ietf:params:xml:ns:caldav" xmlns:card="urn:ietf:params:xml:ns:carddav">
  <d:response>
    <d:href>/principals/user/</d:href>
    <d:propstat>
      <d:prop>
        <cal:calendar-home-set><d:href>` + calHome + `</d:href></cal:calendar-home-set>
        <card:addressbook-home-set><d:href>` + abHome + `</d:href></card:addressbook-home-set>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>`
}

func calendarMS(path string) string {
	return `<?xml version="1.0" encoding="utf-8"?>
<d:multistatus xmlns:d="DAV:" xmlns:cal="urn:ietf:params:xml:ns:caldav" xmlns:cs="http://calendarserver.org/ns/">
  <d:response>
    <d:href>` + path + `</d:href>
    <d:propstat>
      <d:prop>
        <d:displayname>Calendar</d:displayname>
        <d:resourcetype><cal:calendar/></d:resourcetype>
        <cs:getctag>cal-1</cs:getctag>
        <d:current-user-privilege-set><d:privilege><d:read/></d:privilege></d:current-user-privilege-set>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>`
}

func addressbookMS(path string) string {
	return `<?xml version="1.0" encoding="utf-8"?>
<d:multistatus xmlns:d="DAV:" xmlns:card="urn:ietf:params:xml:ns:carddav" xmlns:cs="http://calendarserver.org/ns/">
  <d:response>
    <d:href>` + path + `</d:href>
    <d:propstat>
      <d:prop>
        <d:displayname>Contacts</d:displayname>
        <d:resourcetype><card:addressbook/></d:resourcetype>
        <cs:getctag>ab-1</cs:getctag>
        <d:current-user-privilege-set><d:privilege><d:read/></d:privilege></d:current-user-privilege-set>
      </d:prop>
      <d:status>HTTP/1.1 200 OK</d:status>
    </d:propstat>
  </d:response>
</d:multistatus>`
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
