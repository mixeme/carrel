// A bind as the guards allow it: the view models, the widgets, strconv and
// slices; no word of its own — a space is none.
package bind

import (
	"slices"
	"strconv"

	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

func Count(rows []view.FormRow) w.Text {
	return w.Text(strconv.Itoa(len(slices.Clone(rows))) + " ")
}

func Posts(a view.Action) bool { return a.Method == "POST" || a.Method == "GET" }
