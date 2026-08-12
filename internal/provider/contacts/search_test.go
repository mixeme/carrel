// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package contacts

import (
	"context"
	"strings"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

// matchesCardQuery applies an addressbook-query filter to one card. CardDAV
// lets the client ask for "any of", and a server that joined the filters the
// other way would answer almost nothing, so the fake honours the test attribute.
func matchesCardQuery(body string, query *dav.AddressBookQuery) bool {
	if query.Filter == nil || len(query.Filter.PropFilters) == 0 {
		return true
	}
	anyOf := query.Filter.Test != "allof"
	for _, filter := range query.Filter.PropFilters {
		if filter.TextMatch == nil {
			continue
		}
		hit := cardPropContains(body, filter.Name, filter.TextMatch.Text)
		if anyOf && hit {
			return true
		}
		if !anyOf && !hit {
			return false
		}
	}
	return !anyOf
}

func cardPropContains(body, property, text string) bool {
	want := strings.ToLower(text)
	for _, line := range strings.Split(body, "\r\n") {
		name := line
		if i := strings.IndexAny(line, ":;"); i >= 0 {
			name = line[:i]
		}
		// Group prefixes are part of a vCard property name: item1.EMAIL.
		if i := strings.LastIndex(name, "."); i >= 0 {
			name = name[i+1:]
		}
		if !strings.EqualFold(name, property) {
			continue
		}
		if strings.Contains(strings.ToLower(line), want) {
			return true
		}
	}
	return false
}

func searchServer() *fakeServer {
	srv := newFakeServer()
	srv.add("alice.vcf", `"a1"`, "FN:Alice Smith", "EMAIL:alice@example.org", "ORG:Acme")
	srv.add("bob.vcf", `"b1"`, "FN:Bob Jones", "EMAIL:bob@acme.example", "NOTE:met at the fair")
	srv.add("carol.vcf", `"c1"`, "FN:Carol Brown", "TEL:+7 900 000 0000")
	return srv
}

func TestContactSearchMatchesAnyOfTheProperties(t *testing.T) {
	srv := searchServer()
	p := newProvider(t, srv, Options{})

	// "acme" is in one card's ORG and another's EMAIL: both are answers (§16).
	result, err := p.Search(context.Background(), collection, "acme")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(result.Objects) != 2 {
		t.Fatalf("objects = %+v, want both cards", result.Objects)
	}
	names := map[string]bool{}
	for _, obj := range result.Objects {
		names[obj.UID()] = true
	}
	if !names["alice"] || !names["bob"] {
		t.Errorf("results = %v", names)
	}
	// Unlike CalDAV, one report covers every property (§16).
	if len(srv.queries) != 1 {
		t.Fatalf("%d queries, want 1", len(srv.queries))
	}
	query := srv.queries[0]
	if query.Filter.Test != "anyof" {
		t.Errorf("filter test = %q, want anyof", query.Filter.Test)
	}
	if len(query.Filter.PropFilters) != len(dav.SearchedVCardProps) {
		t.Errorf("%d prop filters, want %d", len(query.Filter.PropFilters), len(dav.SearchedVCardProps))
	}
	if got := query.Filter.PropFilters[0].TextMatch.Collation; got != dav.CalDAVCollation {
		t.Errorf("collation = %q, want the case-insensitive one", got)
	}
}

func TestContactSearchSkipsTheCollectionItself(t *testing.T) {
	srv := searchServer()
	p := newProvider(t, srv, Options{})

	result, err := p.Search(context.Background(), collection, "Carol")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(result.Objects) != 1 || result.Objects[0].UID() != "carol" {
		t.Fatalf("objects = %+v, want the one card and not the collection", result.Objects)
	}
}

// A search fills the body cache, so listing the same card afterwards is
// answered inside the process (§12).
func TestContactSearchCachesWhatItRead(t *testing.T) {
	srv := searchServer()
	cache := session.NewCache(session.CacheConfig{}, nil)
	p := newProvider(t, srv, Options{Cache: cache})

	result, err := p.Search(context.Background(), collection, "Alice")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(result.Objects) != 1 {
		t.Fatalf("objects = %+v", result.Objects)
	}
	card := result.Objects[0]
	before := srv.reports
	got, err := p.Multiget(context.Background(), collection, []string{card.Path}, map[string]string{card.Path: card.ETag})
	if err != nil {
		t.Fatalf("Multiget: %v", err)
	}
	if len(got.Objects) != 1 || got.Objects[0].UID() != "alice" {
		t.Fatalf("multiget = %+v", got.Objects)
	}
	if srv.reports != before {
		t.Errorf("reading a searched card again sent %d reports", srv.reports-before)
	}
}

func TestContactSearchRejectsEmptyInput(t *testing.T) {
	p := newProvider(t, searchServer(), Options{})
	if _, err := p.Search(context.Background(), "", "alice"); err == nil {
		t.Error("Search accepted an empty collection")
	}
	if _, err := p.Search(context.Background(), collection, " "); err == nil {
		t.Error("Search accepted empty text")
	}
}

func TestContactSearchReportsAServerThatRefuses(t *testing.T) {
	srv := searchServer()
	srv.refuseReport = true
	p := newProvider(t, srv, Options{})
	if _, err := p.Search(context.Background(), collection, "alice"); err == nil {
		t.Fatal("Search hid a server that refused the report")
	}
}
