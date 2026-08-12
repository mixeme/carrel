// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package files

import (
	"errors"
	"testing"
)

func TestResolveStaysInsideCollection(t *testing.T) {
	const root = "/dav/files/"
	cases := []struct {
		rel  string
		want string
	}{
		{"", "/dav/files/"},
		{"a.txt", "/dav/files/a.txt"},
		{"/a.txt", "/dav/files/a.txt"},
		{"sub/a.txt", "/dav/files/sub/a.txt"},
		{"./sub//a.txt", "/dav/files/sub/a.txt"},
		{"sub/./deep/a.txt", "/dav/files/sub/deep/a.txt"},
		// A name that only looks like a traversal is a name.
		{"..hidden", "/dav/files/..hidden"},
		{"a..b/c", "/dav/files/a..b/c"},
	}
	for _, tc := range cases {
		got, err := Resolve(root, tc.rel)
		if err != nil {
			t.Fatalf("Resolve(%q): %v", tc.rel, err)
		}
		if got != tc.want {
			t.Fatalf("Resolve(%q) = %q, want %q", tc.rel, got, tc.want)
		}
	}
}

// §24.4 asks for `../` to be refused rather than normalised away. A path that
// tried to leave is refused on its own terms, so the refusal shows up in a log
// and a test rather than turning silently into a different path.
func TestResolveRefusesTraversal(t *testing.T) {
	const root = "/dav/files/"
	for _, rel := range []string{
		"../secrets",
		"..",
		"sub/../../etc/passwd",
		"sub/..",
		"./../x",
		"a/b/../../../c",
	} {
		if _, err := Resolve(root, rel); !errors.Is(err, ErrOutsideCollection) {
			t.Fatalf("Resolve(%q) error = %v, want ErrOutsideCollection", rel, err)
		}
	}
}

func TestResolveRefusesBackslashAndControlBytes(t *testing.T) {
	for _, rel := range []string{`sub\..\..\x`, "a\x00b", "line\nbreak", "tab\there"} {
		if _, err := Resolve("/dav/files/", rel); err == nil {
			t.Fatalf("Resolve(%q) succeeded, want a refusal", rel)
		}
	}
}

func TestCleanNameRefusesPaths(t *testing.T) {
	for _, name := range []string{"", ".", "..", "a/b", `a\b`, "x\x01y"} {
		if _, err := CleanName(name); err == nil {
			t.Fatalf("CleanName(%q) succeeded, want a refusal", name)
		}
	}
	got, err := CleanName("  Отчёт 2026.pdf  ")
	if err != nil {
		t.Fatalf("CleanName: %v", err)
	}
	if got != "Отчёт 2026.pdf" {
		t.Fatalf("CleanName = %q", got)
	}
}

func TestRelativeIsTheInverseOfResolve(t *testing.T) {
	const root = "/dav/files/"
	rel, err := Relative(root, "/dav/files/sub/a.txt")
	if err != nil {
		t.Fatalf("Relative: %v", err)
	}
	if rel != "sub/a.txt" {
		t.Fatalf("Relative = %q", rel)
	}
	if rel, err := Relative(root, "/dav/files/"); err != nil || rel != "" {
		t.Fatalf("Relative(root) = %q, %v", rel, err)
	}
	// The check that keeps an ATTACH URI from naming somebody else's tree.
	if _, err := Relative(root, "/dav/calendars/user/x.ics"); !errors.Is(err, ErrOutsideCollection) {
		t.Fatalf("Relative outside root error = %v, want ErrOutsideCollection", err)
	}
}

func TestParentAndBase(t *testing.T) {
	if parent, ok := Parent("a/b/c.txt"); !ok || parent != "a/b" {
		t.Fatalf("Parent = %q, %v", parent, ok)
	}
	if parent, ok := Parent("c.txt"); !ok || parent != "" {
		t.Fatalf("Parent of top-level = %q, %v", parent, ok)
	}
	if _, ok := Parent(""); ok {
		t.Fatal("the root of a collection should have no parent")
	}
	if got := Base("a/b/c.txt"); got != "c.txt" {
		t.Fatalf("Base = %q", got)
	}
}
