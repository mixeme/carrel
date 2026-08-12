// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package calendar

import (
	"context"
	"errors"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

func todoBody(uid, summary, extra string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test//EN\r\nBEGIN:VTODO\r\n" +
		"UID:" + uid + "\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:" + summary + "\r\n" +
		extra + "END:VTODO\r\nEND:VCALENDAR\r\n"
}

func journalBody(uid, summary, extra string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Test//EN\r\nBEGIN:VJOURNAL\r\n" +
		"UID:" + uid + "\r\nDTSTAMP:20260801T000000Z\r\nDTSTART:20260801T090000Z\r\nSUMMARY:" + summary + "\r\n" +
		extra + "END:VJOURNAL\r\nEND:VCALENDAR\r\n"
}

// mixedCalendar is one collection holding all three component kinds, which is
// what a server that keeps tasks and notes beside events looks like (§10).
func mixedCalendar() *fakeCalendar {
	server := newFakeCalendar()
	server.add("meeting.ics", `"e1"`, eventBody("meeting", "Budget meeting", "20260812T100000Z", "20260812T110000Z", ""))
	server.add("chase.ics", `"t1"`, todoBody("chase", "Chase invoice", "DESCRIPTION:the budget one\r\nSTATUS:NEEDS-ACTION\r\n"))
	server.add("thought.ics", `"j1"`, journalBody("thought", "A thought", "DESCRIPTION:nothing to do with money\r\n"))
	return server
}

func TestQueryComponentReturnsOnlyThatComponent(t *testing.T) {
	server := mixedCalendar()
	p := testProvider(t, server)

	set, err := p.QueryComponent(context.Background(), testCollection, dav.CompTodo, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("QueryComponent: %v", err)
	}
	if len(set.Objects) != 1 {
		t.Fatalf("objects = %d, want the one task", len(set.Objects))
	}
	if uid := set.Objects[0].UID(); uid != "chase" {
		t.Errorf("UID = %q, want the task", uid)
	}
	if set.FromCache {
		t.Error("a first query reads as cached")
	}
	// A task list has no time range to filter on, so none is sent (§10).
	if len(server.queries) != 1 {
		t.Fatalf("%d queries, want 1", len(server.queries))
	}
	inner := server.queries[0].Filter.CompFilter.CompFilter
	if inner.Name != dav.CompTodo {
		t.Errorf("comp-filter = %q, want VTODO", inner.Name)
	}
	if inner.TimeRange != nil {
		t.Errorf("a task query carries a time range: %+v", inner.TimeRange)
	}
}

// A server is free to answer a filter it does not implement with everything it
// has. Putting an event in a task list because of that would be worse than a
// slow request, so the provider filters what came back as well.
func TestQueryComponentDropsWhatTheServerShouldNotHaveSent(t *testing.T) {
	server := mixedCalendar()
	server.ignoreFilter = true
	p := testProvider(t, server)

	set, err := p.QueryComponent(context.Background(), testCollection, dav.CompJournal, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("QueryComponent: %v", err)
	}
	if len(set.Objects) != 1 || set.Objects[0].UID() != "thought" {
		t.Fatalf("objects = %+v, want only the note", set.Objects)
	}
}

func TestQueryComponentSecondReadComesFromTheCache(t *testing.T) {
	server := mixedCalendar()
	clock := time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)
	cache := session.NewCache(session.CacheConfig{CollectionTTL: time.Minute}, func() time.Time { return clock })
	p, err := New(server, Options{AccountID: "acc", Cache: cache, Location: time.UTC})
	if err != nil {
		t.Fatal(err)
	}

	first, err := p.QueryComponent(context.Background(), testCollection, dav.CompTodo, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("first QueryComponent: %v", err)
	}
	second, err := p.QueryComponent(context.Background(), testCollection, dav.CompTodo, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("second QueryComponent: %v", err)
	}
	if !second.FromCache {
		t.Error("the second read does not report itself as cached, which §16 shows in the progress panel")
	}
	if len(server.queries) != 1 {
		t.Errorf("%d queries reached the server, want the second read to stay in the process", len(server.queries))
	}
	if len(second.Objects) != len(first.Objects) || second.Objects[0].UID() != first.Objects[0].UID() {
		t.Errorf("the cached read differs: %+v", second.Objects)
	}

	// A collection that has moved on invalidates the set rather than serving
	// it, once the metadata is looked at again (§12).
	server.ctag = "ctag-2"
	server.add("later.ics", `"t2"`, todoBody("later", "Something else", ""))
	clock = clock.Add(2 * time.Minute)
	third, err := p.QueryComponent(context.Background(), testCollection, dav.CompTodo, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("third QueryComponent: %v", err)
	}
	if third.FromCache || len(third.Objects) != 2 {
		t.Errorf("a changed collection was served from cache: cached %v, objects %d", third.FromCache, len(third.Objects))
	}
}

func TestSearchLooksAtEveryComponentAndSaysEachAnswerOnce(t *testing.T) {
	server := mixedCalendar()
	p := testProvider(t, server)

	set, err := p.Search(context.Background(), testCollection, "budget")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	uids := map[string]int{}
	for _, obj := range set.Objects {
		uids[obj.UID()]++
	}
	// The event matches on SUMMARY, the task on DESCRIPTION, and the note not
	// at all. A match on two properties is still one row (§16).
	if len(set.Objects) != 2 || uids["meeting"] != 1 || uids["chase"] != 1 {
		t.Fatalf("results = %v", uids)
	}
	// Three components over two properties, because CalDAV has no "any of"
	// at the property level.
	want := len(SearchProps) * 3
	if len(server.queries) != want {
		t.Errorf("%d queries, want %d", len(server.queries), want)
	}
	if got := server.queries[0].Filter.CompFilter.CompFilter.PropFilters[0].TextMatch.Collation; got != dav.CalDAVCollation {
		t.Errorf("collation = %q, want the case-insensitive one", got)
	}
}

func TestSearchIsCaseInsensitive(t *testing.T) {
	server := mixedCalendar()
	p := testProvider(t, server)

	set, err := p.Search(context.Background(), testCollection, "BUDGET", dav.CompEvent)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(set.Objects) != 1 || set.Objects[0].UID() != "meeting" {
		t.Fatalf("results = %+v", set.Objects)
	}
}

// §16: a search that is abandoned stops where it is and hands back what it
// found, so a cancelled screen still shows the partial list.
func TestSearchStopsWhenTheContextEnds(t *testing.T) {
	server := mixedCalendar()
	p := testProvider(t, server)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	set, err := p.Search(ctx, testCollection, "budget")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Search error = %v, want the cancellation", err)
	}
	if set == nil {
		t.Fatal("a cancelled search returned no result to render")
	}
	if len(server.queries) != 0 {
		t.Errorf("%d queries left after the search was cancelled", len(server.queries))
	}
}

func TestSearchReportsAServerThatCannotAnswer(t *testing.T) {
	server := mixedCalendar()
	server.refuseQuery = true
	p := testProvider(t, server)

	if _, err := p.Search(context.Background(), testCollection, "budget"); err == nil {
		t.Fatal("Search hid a server that refused every report")
	}
}

func TestQueryComponentRejectsEmptyInput(t *testing.T) {
	p := testProvider(t, mixedCalendar())
	if _, err := p.QueryComponent(context.Background(), "", dav.CompTodo, time.Time{}, time.Time{}); err == nil {
		t.Error("QueryComponent accepted an empty collection")
	}
	if _, err := p.QueryComponent(context.Background(), testCollection, " ", time.Time{}, time.Time{}); err == nil {
		t.Error("QueryComponent accepted an empty component")
	}
	if _, err := p.Search(context.Background(), testCollection, "  "); err == nil {
		t.Error("Search accepted empty text")
	}
}
