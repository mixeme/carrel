// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package desktop

import (
	"errors"
	"testing"
)

func TestNormalizeRemoteURL(t *testing.T) {
	tests := []struct {
		in   string
		want string
		err  error
	}{
		{in: "https://carrel.example", want: "https://carrel.example"},
		{in: "https://carrel.example/", want: "https://carrel.example"},
		{in: "carrel.example", want: "https://carrel.example"},
		{in: " http://127.0.0.1:8080 ", want: "http://127.0.0.1:8080"},
		{in: "https://host/carrel", want: "https://host/carrel"},
		{in: "", err: ErrRemoteURL},
		{in: "ftp://x", err: errors.New("scheme")},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := NormalizeRemoteURL(tc.in)
			if tc.err != nil {
				if err == nil {
					t.Fatalf("expected error containing %v", tc.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResolveRemoteURL(t *testing.T) {
	cfg := &Config{Mode: ModeRemote, RemoteURL: "https://a.example"}
	got, err := ResolveRemoteURL(cfg, "")
	if err != nil || got != "https://a.example" {
		t.Fatalf("ResolveRemoteURL() = %q, %v", got, err)
	}
	got, err = ResolveRemoteURL(cfg, "http://b.example")
	if err != nil || got != "http://b.example" {
		t.Fatalf("override = %q, %v", got, err)
	}
	_, err = ResolveRemoteURL(&Config{Mode: ModeLocal}, "")
	if err == nil {
		t.Fatal("expected error for local mode")
	}
}
