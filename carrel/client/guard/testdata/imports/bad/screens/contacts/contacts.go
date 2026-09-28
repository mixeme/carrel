// A screen that goes around the library: every import below but the first is
// a breach the guard must name.
package contacts

import (
	_ "embed"
	"fmt"
	"html/template"
	"strings"

	"carrel.dev/carrel/client/zone"
	"carrel.dev/carrel/server/view"
)

//go:embed contacts.css
var sheet string

var page = template.Must(template.New("list").Parse(""))

func List(p *view.ContactsListPage) string {
	_, _ = zone.Of(p, "main")
	return fmt.Sprint(strings.ToUpper(string(p.Title)), sheet, page)
}
