package conflict

import (
	"testing"

	"carrel.dev/carrel/server/view"
)

// A card is on at the side the server chose beforehand, and at none where it
// chose nothing (ChoiceUnset: Carrel cannot tell whose the change is, the
// owner's decision of 16.10.2026).
func TestACardIsOnWhereChosen(t *testing.T) {
	for _, c := range []struct {
		choice           view.ConflictChoice
		mineOn, theirsOn bool
	}{
		{view.ChoiceMine, true, false},
		{view.ChoiceTheirs, false, true},
		{view.ChoiceUnset, false, false},
	} {
		got := card(view.ConflictRow{Property: "NICKNAME", Name: "choice.0", Choice: c.choice}).Options
		if got[0].On != c.mineOn || got[1].On != c.theirsOn {
			t.Errorf("choice %q: mine on %v, server on %v", c.choice, got[0].On, got[1].On)
		}
	}
}
