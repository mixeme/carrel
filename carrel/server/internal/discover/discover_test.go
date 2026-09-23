package discover

import (
	"testing"

	"carrel.dev/carrel/server/view"
)

func TestDiffAddedRemoved(t *testing.T) {
	stored := []Collection{
		{Path: "/cal/personal", DisplayName: "Personal"},
		{Path: "/cal/work", DisplayName: "Work"},
	}
	found := []Collection{
		{Path: "/cal/personal", DisplayName: "Personal"},
		{Path: "/cal/team", DisplayName: "Team"},
	}
	d := CompareCollections(found, stored)
	if len(d.Added) != 1 || d.Added[0].Path != "/cal/team" {
		t.Fatalf("added = %#v", d.Added)
	}
	if len(d.Removed) != 1 || d.Removed[0] != "/cal/work" {
		t.Fatalf("removed = %#v", d.Removed)
	}
}

func TestCalendarKinds(t *testing.T) {
	got := map[view.Kind]bool{}
	for _, k := range CalendarKinds([]string{"VEVENT", "VTODO"}) {
		got[k] = true
	}
	if len(got) != 2 || !got[view.KindEvent] || !got[view.KindTask] {
		t.Fatalf("kinds = %v, want event and task", got)
	}
	// A calendar that names no components serves all three (S-6).
	if len(CalendarKinds(nil)) != 3 {
		t.Fatalf("empty component set = %v, want all three", CalendarKinds(nil))
	}
}
