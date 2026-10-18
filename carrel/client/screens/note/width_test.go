package note

import (
	"testing"

	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// Typecase's note 79 (task 204): a note stands at the reading measure unless
// the browser chose the full width (Look.Wide), and the bar's switch says
// which is in force.
func TestANoteIsReadAtItsMeasureByDefault(t *testing.T) {
	for _, c := range []struct {
		wide bool
		view w.DocView
	}{
		{false, w.DocViewMeasure},
		{true, w.DocViewFull},
	} {
		look := view.Look{Wide: c.wide}
		if got := layoutOf(look); got != c.view {
			t.Errorf("wide %v: layout %v, want %v", c.wide, got, c.view)
		}
		seg := widthSeg(look, true)
		if seg.Options[0].On != c.wide || seg.Options[1].On == c.wide {
			t.Errorf("wide %v: the switch says %+v", c.wide, seg.Options)
		}
	}
}
