// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package dav

import (
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testGuard() *Guard { return NewGuard(GuardConfig{Allowlist: []string{"127.0.0.1"}}) }

// TestPropFindBodyNamesRequestedProperties is a regression test. The requested
// properties used to be serialised as the name of the Go field holding them, so
// the server was handed a body naming no property it knew — and answered with
// whatever it felt like instead of what was asked for.
func TestPropFindBodyNamesRequestedProperties(t *testing.T) {
	body, err := xml.Marshal(NewPropFind(ResourceTypeName, GetETagName, GetCTagName))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(body)
	for _, want := range []string{
		`<propfind xmlns="DAV:">`,
		`<prop xmlns="DAV:">`,
		`<resourcetype xmlns="DAV:">`,
		`<getetag xmlns="DAV:">`,
		`<getctag xmlns="http://calendarserver.org/ns/">`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("body has no %s\n%s", want, got)
		}
	}
	if strings.Contains(got, "<Raw>") || strings.Contains(got, "<Name>") {
		t.Errorf("body still carries Go field names:\n%s", got)
	}
}

func TestAddressBookMultigetBody(t *testing.T) {
	body, err := xml.Marshal(NewAddressBookMultiget([]string{"/ab/1.vcf", "/ab/2.vcf"}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(body)
	for _, want := range []string{
		`<addressbook-multiget xmlns="urn:ietf:params:xml:ns:carddav">`,
		`<getetag xmlns="DAV:">`,
		`<address-data xmlns="urn:ietf:params:xml:ns:carddav">`,
		`<href xmlns="DAV:">/ab/1.vcf</href>`,
		`<href xmlns="DAV:">/ab/2.vcf</href>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("body has no %s\n%s", want, got)
		}
	}
}

const multigetResponse = `<?xml version="1.0" encoding="utf-8"?>
<multistatus xmlns="DAV:" xmlns:card="urn:ietf:params:xml:ns:carddav">
  <response>
    <href>/ab/1.vcf</href>
    <propstat>
      <prop>
        <getetag>"v1"</getetag>
        <card:address-data>BEGIN:VCARD&#13;
VERSION:3.0&#13;
UID:one&#13;
FN:Ada Lovelace&#13;
END:VCARD&#13;
</card:address-data>
      </prop>
      <status>HTTP/1.1 200 OK</status>
    </propstat>
  </response>
</multistatus>`

func TestClientReport(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "REPORT" {
			t.Errorf("method = %s, want REPORT", r.Method)
		}
		if r.Header.Get("Depth") != "0" {
			t.Errorf("depth = %q, want 0", r.Header.Get("Depth"))
		}
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/xml") {
			t.Errorf("content type = %q", ct)
		}
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		w.WriteHeader(http.StatusMultiStatus)
		io.WriteString(w, multigetResponse)
	}))
	defer srv.Close()

	client, err := NewClient(testGuard(), srv.URL, "mix", "secret")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	report := NewAddressBookMultiget([]string{"/ab/1.vcf"})
	ms, err := client.Report(context.Background(), "/ab/", DepthZero, report)
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	if !strings.Contains(gotBody, "<href xmlns=\"DAV:\">/ab/1.vcf</href>") {
		t.Errorf("request body did not carry the href:\n%s", gotBody)
	}
	if len(ms.Responses) != 1 {
		t.Fatalf("responses = %d, want 1", len(ms.Responses))
	}

	var data AddressData
	if err := ms.Responses[0].DecodeProp(&data); err != nil {
		t.Fatalf("decode address-data: %v", err)
	}
	if !strings.Contains(data.Data, "FN:Ada Lovelace") {
		t.Errorf("address data = %q", data.Data)
	}
	var etag GetETag
	if err := ms.Responses[0].DecodeProp(&etag); err != nil {
		t.Fatalf("decode getetag: %v", err)
	}
	if etag.ETag != `"v1"` {
		t.Errorf("etag = %q", etag.ETag)
	}
}

func TestPutOptsSendsMediaTypeAndPrecondition(t *testing.T) {
	for _, tc := range []struct {
		name            string
		opts            PutOptions
		wantIfMatch     string
		wantIfNoneMatch string
	}{
		{
			name:        "update names the version it replaces",
			opts:        PutOptions{ContentType: MediaTypeVCard, IfMatch: `"v1"`},
			wantIfMatch: `"v1"`,
		},
		{
			name:            "create refuses to replace anything",
			opts:            PutOptions{ContentType: MediaTypeVCard, IfNoneMatch: true},
			wantIfNoneMatch: "*",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("If-Match"); got != tc.wantIfMatch {
					t.Errorf("If-Match = %q, want %q", got, tc.wantIfMatch)
				}
				if got := r.Header.Get("If-None-Match"); got != tc.wantIfNoneMatch {
					t.Errorf("If-None-Match = %q, want %q", got, tc.wantIfNoneMatch)
				}
				if got := r.Header.Get("Content-Type"); got != MediaTypeVCard {
					t.Errorf("Content-Type = %q, want %q", got, MediaTypeVCard)
				}
				w.Header().Set("ETag", `"v2"`)
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			client, err := NewClient(testGuard(), srv.URL, "mix", "secret")
			if err != nil {
				t.Fatalf("NewClient: %v", err)
			}
			etag, err := client.PutOpts(context.Background(), "/ab/1.vcf", strings.NewReader("BEGIN:VCARD\r\nEND:VCARD\r\n"), tc.opts)
			if err != nil {
				t.Fatalf("PutOpts: %v", err)
			}
			if etag != `"v2"` {
				t.Errorf("etag = %q", etag)
			}
		})
	}
}

func TestPreconditionFailedIsRecognised(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "etag mismatch", http.StatusPreconditionFailed)
	}))
	defer srv.Close()

	client, err := NewClient(testGuard(), srv.URL, "", "")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_, err = client.PutOpts(context.Background(), "/ab/1.vcf", strings.NewReader("x"), PutOptions{IfMatch: `"old"`})
	if err == nil {
		t.Fatal("PutOpts succeeded on a refused precondition")
	}
	if !IsPreconditionFailed(err) {
		t.Errorf("IsPreconditionFailed(%v) = false", err)
	}
	if got := StatusCode(err); got != http.StatusPreconditionFailed {
		t.Errorf("StatusCode = %d, want 412", got)
	}
	if IsPreconditionFailed(io.EOF) {
		t.Error("a plain error was taken for a refused precondition")
	}
}
