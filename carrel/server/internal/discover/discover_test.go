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
	k := calendarKinds([]string{"VEVENT", "VTODO"})
	if len(k) != 2 {
		t.Fatalf("kinds = %v", k)
	}
}

func TestFileOnlyResultMarksFileKind(t *testing.T) {
	c := Collection{Path: "/docs", Kinds: []view.Kind{view.KindFile}}
	if len(c.Kinds) != 1 || c.Kinds[0] != view.KindFile {
		t.Fatalf("kinds = %v", c.Kinds)
	}
}
