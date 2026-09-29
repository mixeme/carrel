// The registry imports what is not a screen's package: a package two levels
// below.
package screens

import (
	"carrel.dev/carrel/client/screens/contacts/rows"
)

var All = []any{rows.Row}
