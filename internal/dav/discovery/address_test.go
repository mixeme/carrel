// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package discovery

import (
	"errors"
	"testing"
)

func TestAddressFromName(t *testing.T) {
	if got := AddressFromName("Поездки"); got != "poezdki" {
		t.Fatalf("AddressFromName = %q", got)
	}
	if got := AddressFromName(""); got != "collection" {
		t.Fatalf("empty = %q", got)
	}
}

func TestValidateAddress(t *testing.T) {
	for _, bad := range []string{"../x", "a/b", ".hidden", ""} {
		if err := ValidateAddress(bad); err == nil {
			t.Fatalf("ValidateAddress(%q) wanted error", bad)
		}
	}
	if err := ValidateAddress("poezdki"); err != nil {
		t.Fatal(err)
	}
}

func TestUniqueAddress(t *testing.T) {
	existing := []Collection{{Path: "/calendars/mix/trips/"}}
	if got := UniqueAddress("/calendars/mix/", "trips", existing); got != "trips-2" {
		t.Fatalf("got %q", got)
	}
}

func TestCollectionHref(t *testing.T) {
	got := CollectionHref("/calendars/mix/", "poezdki")
	want := "/calendars/mix/poezdki/"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestColorFromAddress(t *testing.T) {
	a := ColorFromAddress("book-1")
	b := ColorFromAddress("book-1")
	if a != b || a == "" {
		t.Fatalf("color = %q", a)
	}
}

func TestFindCollection(t *testing.T) {
	cols := []Collection{{Path: "/a/b/", DisplayName: "B"}}
	col, ok := FindCollection(cols, "/a/b/")
	if !ok || col.DisplayName != "B" {
		t.Fatal("not found")
	}
	_, ok = FindCollection(cols, "/nope/")
	if ok {
		t.Fatal("wanted miss")
	}
}

func TestFormatRequestDiag(t *testing.T) {
	err := errors.New("plain")
	if FormatRequestDiag(err) != "plain" {
		t.Fatal("plain message")
	}
}
