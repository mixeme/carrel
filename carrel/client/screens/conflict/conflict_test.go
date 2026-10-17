package conflict

import (
	"testing"

	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// A card is on at the side the server chose beforehand, and at none where it
// chose nothing (ChoiceUnset: Carrel cannot tell whose the change is, the
// owner's decision of 16.10.2026); then the form is not sent without its
// side (Required, Typecase's 198).
func TestACardIsOnWhereChosen(t *testing.T) {
	for _, c := range []struct {
		choice           view.ConflictChoice
		mineOn, theirsOn bool
	}{
		{view.ChoiceMine, true, false},
		{view.ChoiceTheirs, false, true},
		{view.ChoiceUnset, false, false},
	} {
		got := card(view.ConflictRow{Property: "NICKNAME", Name: "choice.0", Choice: c.choice}, false).Options
		if got[0].On != c.mineOn || got[1].On != c.theirsOn {
			t.Errorf("choice %q: mine on %v, server on %v", c.choice, got[0].On, got[1].On)
		}
		if need := c.choice == view.ChoiceUnset; got[0].Required != need || got[1].Required != need {
			t.Errorf("choice %q: required %v, %v", c.choice, got[0].Required, got[1].Required)
		}
	}
}

// The words of a card's sides (s-conflict-1, -6, -7): «Your form» where
// whose change it is cannot be told; a record deleted on the server is a
// card by its name, «Your edit · write it back» against «Server · leave it
// deleted».
func TestACardNamesItsSides(t *testing.T) {
	source := func(c w.Check) string { return c.Parts[0].(w.Choice).Source }
	for _, c := range []struct {
		row             view.ConflictRow
		deleted         bool
		mine, theirs    string
		mono, plainName bool
	}{
		{view.ConflictRow{Property: "EMAIL;work", Diff: view.DiffBoth, Choice: view.ChoiceMine}, false, "Your edit", "Server", true, false},
		{view.ConflictRow{Property: "NOTE", Diff: view.DiffUnknown}, false, "Your form", "Server", true, false},
		{view.ConflictRow{Property: "Анна Ковалёва"}, true, "Your edit · write it back", "Server · leave it deleted", false, true},
	} {
		got := card(c.row, c.deleted)
		if m, th := source(got.Options[0]), source(got.Options[1]); m != c.mine || th != c.theirs {
			t.Errorf("%s: %q / %q, want %q / %q", c.row.Property, m, th, c.mine, c.theirs)
		}
		if (got.Title.Lead != nil) != c.mono || (got.Title.Text == string(c.row.Property)) != c.plainName {
			t.Errorf("%s: title %+v", c.row.Property, got.Title)
		}
	}
}
