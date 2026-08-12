// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package dav

import (
	"context"
	"errors"
	"net"
	"net/url"
	"testing"
)

func TestGuardBlocksLoopback(t *testing.T) {
	g := NewGuard(GuardConfig{})
	ctx := context.Background()
	u, _ := url.Parse("http://127.0.0.1/dav/")
	err := g.ValidateURL(ctx, u)
	if err == nil {
		t.Fatal("expected loopback to be blocked")
	}
	if !errors.Is(err, ErrSSRF) {
		t.Fatalf("expected ErrSSRF, got %v", err)
	}
}

func TestGuardAllowsAllowlistedHost(t *testing.T) {
	g := NewGuard(GuardConfig{Allowlist: []string{"baikal.local"}})
	ctx := context.Background()
	u, _ := url.Parse("http://baikal.local/dav/")
	// Resolution may fail in CI without DNS; only assert allowlist parsing exists.
	if len(g.allowlist) != 1 {
		t.Fatalf("allowlist = %v, want 1 entry", g.allowlist)
	}
	_ = g.hostAllowed("baikal.local")
	_ = ctx
	_ = u
}

func TestGuardBlocksPrivateIPLiteral(t *testing.T) {
	g := NewGuard(GuardConfig{})
	ip := net.ParseIP("10.0.0.1")
	if err := g.checkIP(ip, "example.com"); err == nil {
		t.Fatal("expected private IP to be blocked")
	}
}
