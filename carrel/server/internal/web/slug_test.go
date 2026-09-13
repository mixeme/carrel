package web

import "testing"

func TestCollectionSlug(t *testing.T) {
	for name, want := range map[string]string{
		"Поездки":          "poezdki",
		"Дни рождения":     "dni-rozhdeniya",
		"Щука и Ёж":        "shchuka-i-ezh",
		"Їжак, Ґанок, Єва": "izhak-ganok-yeva",
		"Café Crème":       "cafe-creme",
		"Work  - 2026!":    "work-2026",
		"  Trips  ":        "trips",
		"../..":            "col",
		"日本":               "col",
		"":                 "col",
	} {
		if got := collectionSlug(name); got != want {
			t.Errorf("collectionSlug(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestUniqueSlug(t *testing.T) {
	taken := map[string]bool{"work": true, "work-2": true}
	if got := uniqueSlug("work", taken); got != "work-3" {
		t.Errorf("uniqueSlug = %q, want work-3", got)
	}
	if got := uniqueSlug("trips", taken); got != "trips" {
		t.Errorf("uniqueSlug = %q, want trips", got)
	}
}

func TestAddressProblem(t *testing.T) {
	for seg, bad := range map[string]bool{
		"":             false,
		"team":         false,
		"Team_2.0~x":   false,
		"..":           true,
		"a..b":         true,
		".hidden":      true,
		"a/b":          true,
		"с кириллицей": true,
		"space here":   true,
	} {
		if got := addressProblem(seg) != ""; got != bad {
			t.Errorf("addressProblem(%q) refused = %v, want %v", seg, got, bad)
		}
	}
}
