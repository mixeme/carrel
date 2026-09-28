// A screen as the guard allows it: the view models, the widgets, strconv and
// slices — and one import more that a row of the registry of screen
// specifics excuses.
package contacts

import (
	"slices"
	"strconv"

	// специфика экрана: U-5.7 #1
	"time"

	"carrel.dev/carrel/server/view"
	"carrel.dev/typecase/widget"
)

func List(p *view.ContactsListPage) widget.Node {
	n := len(p.Groups)
	_ = slices.Clone(p.Groups)
	_ = time.Second
	return widget.Text(strconv.Itoa(n))
}
