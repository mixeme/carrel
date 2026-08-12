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
