// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sidecar

import "testing"

func TestArchiveNameFor(t *testing.T) {
	if got := ArchiveNameFor("0.10.0", "linux", "amd64"); got != "carrel_0.10.0_linux_amd64.tar.gz" {
		t.Fatalf("got %q", got)
	}
	if got := ArchiveNameFor("v1.0.0", "windows", "amd64"); got != "carrel_1.0.0_windows_amd64.zip" {
		t.Fatalf("got %q", got)
	}
}

func TestReleaseBaseURL(t *testing.T) {
	got := ReleaseBaseURL(DefaultReleaseRepo, "0.10.0")
	want := "https://github.com/mixeme/carrel/releases/download/v0.10.0"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
