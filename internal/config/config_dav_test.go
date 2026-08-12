// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"testing"
	"time"
)

func TestDAVDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CARREL_DATA_DIR", dir)
	clearEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DAV.ConnectTimeout.Duration() != DefaultDAVConnectTimeout {
		t.Fatalf("connect timeout = %v", cfg.DAV.ConnectTimeout.Duration())
	}
	if cfg.DAV.MaxRedirects != DefaultDAVMaxRedirects {
		t.Fatalf("max redirects = %d", cfg.DAV.MaxRedirects)
	}
	if cfg.Cache.CollectionTTL() != DefaultCacheCollectionTTL {
		t.Fatalf("cache ttl = %v", cfg.Cache.CollectionTTL())
	}
}

func TestDAVEnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CARREL_DATA_DIR", dir)
	t.Setenv("CARREL_DAV_SSRF_ALLOWLIST", "baikal.local, 10.0.0.0/8")
	t.Setenv("CARREL_DAV_CONNECT_TIMEOUT", "5")
	t.Setenv("CARREL_CACHE_COLLECTION_TTL", "120")
	clearEnvExcept(t, "CARREL_DATA_DIR", "CARREL_DAV_SSRF_ALLOWLIST", "CARREL_DAV_CONNECT_TIMEOUT", "CARREL_CACHE_COLLECTION_TTL")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.DAV.SSRFAllowlist) != 2 {
		t.Fatalf("allowlist = %v", cfg.DAV.SSRFAllowlist)
	}
	if cfg.DAV.ConnectTimeout.Duration() != 5*time.Second {
		t.Fatalf("connect timeout = %v", cfg.DAV.ConnectTimeout.Duration())
	}
	if cfg.Cache.CollectionTTL() != 120*time.Second {
		t.Fatalf("cache ttl = %v", cfg.Cache.CollectionTTL())
	}
}
