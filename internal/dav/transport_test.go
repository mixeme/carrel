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

func TestClientPropFind(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PROPFIND" {
			t.Fatalf("method = %s, want PROPFIND", r.Method)
		}
		if r.Header.Get("Depth") != "0" {
			t.Fatalf("depth = %q, want 0", r.Header.Get("Depth"))
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusMultiStatus)
		io.WriteString(w, sampleMultiStatus)
	}))
	defer srv.Close()

	g := NewGuard(GuardConfig{Allowlist: []string{"127.0.0.1"}})
	client, err := NewClient(g, srv.URL, "mix", "secret")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ms, err := client.PropFind(context.Background(), "/", DepthZero, []xml.Name{CurrentUserPrincipalName})
	if err != nil {
		t.Fatalf("PropFind: %v", err)
	}
	if len(ms.Responses) != 1 {
		t.Fatalf("responses = %d", len(ms.Responses))
	}
}

func TestClientGetStreams(t *testing.T) {
	body := strings.Repeat("x", 32)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, body)
	}))
	defer srv.Close()

	g := NewGuard(GuardConfig{Allowlist: []string{"127.0.0.1"}})
	client, err := NewClient(g, srv.URL, "", "")
	if err != nil {
		t.Fatal(err)
	}
	rc, _, err := client.Get(context.Background(), "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Fatalf("body = %q, want %q", got, body)
	}
}
