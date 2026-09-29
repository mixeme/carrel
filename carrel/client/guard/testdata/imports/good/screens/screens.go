// The registry lists the screens: it imports the package of each, one level
// below it.
package screens

import (
	"carrel.dev/carrel/client/screens/contacts"
	"carrel.dev/carrel/server/view"
)

var All = []any{func(p *view.ContactsListPage) any { return contacts.List(p) }}
