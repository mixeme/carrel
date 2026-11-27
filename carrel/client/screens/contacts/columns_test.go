package contacts

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Columns a person chose draw the row of chosen columns (s-contacts-23):
// their cells in their order, not the frame's own row of phone, email and
// organisation (the owner, 27.11.2026: Columns did not change the table).
func TestChosenColumnsDrawTheirRow(t *testing.T) {
	cols := &view.Columns{Items: []view.ColumnItem{
		{ID: "name", On: true}, {ID: "email", On: true}, {ID: "birthday", On: true}, {ID: "tags", On: true},
		{ID: "source", On: true}, {ID: "phone"}, {ID: "org"}, {ID: "changed"},
	}}
	shown := chosen(cols)
	r := view.ContactRow{Name: "Анна Ковалёва", Email: "a.kovaleva@example.org", Birthday: "14.03.1987", Phone: "+7 912 445-10-88",
		Tags: []view.Tag{{Label: "работа"}, {Label: "издательство"}}, Stamp: "nextcloud · Контакты"}
	html := columnRow(r, shown).Render()
	var order []string
	for _, c := range []string{"tc-col--name", "tc-col--email", "tc-col--birthday", "tc-col--tags", "tc-col--source"} {
		if i := strings.Index(html, c); i < 0 {
			t.Errorf("no %s in %s", c, html)
		} else {
			order = append(order, c)
		}
	}
	if strings.Contains(html, "tc-col--phone") || !strings.Contains(html, "14.03.1987") || !strings.Contains(html, "tc-row--cols") {
		t.Errorf("row = %s", html)
	}
	if strings.Index(html, "tc-col--email") > strings.Index(html, "tc-col--birthday") {
		t.Errorf("the cells are not in the chosen order: %s", html)
	}
	// The frame's own set is the frame's own row.
	def := &view.Columns{Items: []view.ColumnItem{{ID: "name", On: true}, {ID: "phone", On: true}, {ID: "email", On: true}, {ID: "org", On: true}, {ID: "source", On: true}}}
	if chosen(def) != nil {
		t.Error("the default set is drawn as chosen columns")
	}
}
