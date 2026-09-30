// A bind that goes around its rule: it imports a screen, another package of
// the client and a package that writes text, and it has words of its own.
package bind

import (
	"fmt"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/client/screens/login"
	"carrel.dev/carrel/client/zone"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

var _ = login.Screen
var _ = zone.Of
var _ = bind.Words

func Back(a view.Action) w.Button {
	return w.Button{Label: fmt.Sprint("Back to ", a.Label), Title: "Sign in"}
}

func Initial(a view.Action) rune {
	if a.Label == "" {
		return 'x'
	}
	return ' '
}
