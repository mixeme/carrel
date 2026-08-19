// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package desktop

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// NormalizeRemoteURL trims and validates a Carrel instance base URL for Remote
// mode. The path suffix, if any, is kept so reverse-proxy base paths work.
func NormalizeRemoteURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ErrRemoteURL
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("desktop: remote url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("desktop: remote url scheme %q", u.Scheme)
	}
	if u.Host == "" {
		return "", ErrRemoteURL
	}
	u.Fragment = ""
	u.RawFragment = ""
	out := u.String()
	return strings.TrimRight(out, "/"), nil
}

// jsString quotes s for safe embedding in JavaScript source.
func jsString(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(b)
}
