//go:build integration

// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package discovery

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/dav"
)

func TestDiscoverBaikal(t *testing.T) {
	url := strings.TrimSpace(os.Getenv("CARREL_TEST_DAV_URL"))
	user := strings.TrimSpace(os.Getenv("CARREL_TEST_DAV_USER"))
	pass := os.Getenv("CARREL_TEST_DAV_PASSWORD")
	if url == "" || user == "" || pass == "" {
		t.Skip("CARREL_TEST_DAV_URL, CARREL_TEST_DAV_USER, CARREL_TEST_DAV_PASSWORD not set")
	}

	allowInsecure := os.Getenv("CARREL_TEST_DAV_ALLOW_INSECURE") == "1"
	cfg := dav.GuardConfig{
		ConnectTimeout:   15 * time.Second,
		RequestTimeout:   60 * time.Second,
		MaxResponseBytes: 10 << 20,
		MaxRedirects:     5,
	}
	if allowInsecure {
		cfg.InsecureSkipVerify = true
	}

	g := dav.NewGuard(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	result, trace, err := Discover(ctx, g, Credentials{BaseURL: url, Username: user, Password: pass})
	if err != nil {
		t.Fatalf("Discover: %v\ntrace steps: %d", err, len(trace.Steps))
	}

	var calendars, books int
	for _, col := range result.Collections {
		switch col.Kind {
		case KindCalendar:
			calendars++
		case KindAddressBook:
			books++
		}
	}
	if calendars == 0 || books == 0 {
		t.Fatalf("want calendar and addressbook collections, got calendars=%d books=%d (total %d)",
			calendars, books, len(result.Collections))
	}
}

func TestDiscoverWebDAV(t *testing.T) {
	url := strings.TrimSpace(os.Getenv("CARREL_TEST_WEBDAV_URL"))
	user := strings.TrimSpace(os.Getenv("CARREL_TEST_WEBDAV_USER"))
	pass := os.Getenv("CARREL_TEST_WEBDAV_PASSWORD")
	if url == "" || user == "" || pass == "" {
		t.Skip("CARREL_TEST_WEBDAV_URL, CARREL_TEST_WEBDAV_USER, CARREL_TEST_WEBDAV_PASSWORD not set")
	}

	cfg := dav.GuardConfig{
		ConnectTimeout:     15 * time.Second,
		RequestTimeout:     60 * time.Second,
		MaxResponseBytes:   10 << 20,
		MaxRedirects:       5,
		InsecureSkipVerify: os.Getenv("CARREL_TEST_WEBDAV_ALLOW_INSECURE") == "1",
	}

	g := dav.NewGuard(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	result, trace, err := Discover(ctx, g, Credentials{BaseURL: url, Username: user, Password: pass})
	if err != nil {
		t.Fatalf("Discover: %v\ntrace steps: %d", err, len(trace.Steps))
	}
	var files int
	for _, col := range result.Collections {
		if col.Kind == KindFiles {
			files++
		}
	}
	if files == 0 {
		t.Fatalf("want a file collection, got %+v", result.Collections)
	}
}
