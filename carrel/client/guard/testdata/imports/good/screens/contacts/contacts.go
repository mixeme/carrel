// A screen as the guard allows it: the view models, the widgets, bind,
// strconv and slices — and one import more that a row of the registry of
// screen specifics excuses.
package contacts

import (
	"slices"
	"strconv"

	// специфика экрана: U-5.7 #1
	"time"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	"carrel.dev/typecase/widget"
)

func List(p *view.ContactsListPage) widget.Node {
	n := len(p.Groups)
	_ = slices.Clone(p.Groups)
	_ = time.Second
	_ = bind.Words(p.Lede)
	return widget.Text(strconv.Itoa(n))
}
