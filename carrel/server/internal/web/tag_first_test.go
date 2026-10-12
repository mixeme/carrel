package web_test

import (
	"testing"

	"carrel.dev/carrel/server/view"
)

// Typecase's note 44 (43ca489): the cell «Tags» of a row shows the first tag
// and a count of the rest, so the tag the list is filtered by stands first
// in the row's tags — the row says why it is in the list. The others keep
// the record's order; without a filter nothing moves.
func TestTheFilteredTagStandsFirstInARow(t *testing.T) {
	f := contactsFixture(t, false,
		"BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nCATEGORIES:math,poetry,engines\nEND:VCARD\n")
	list := f.path("contacts")
	rowTags := func(href string) []view.Tag {
		t.Helper()
		page := decode[view.ContactsListPage](t, f.h.do(get(href, f.sess)))
		if len(page.Groups) != 1 || len(page.Groups[0].Rows) != 1 {
			t.Fatalf("%s: groups = %+v", href, page.Groups)
		}
		return page.Groups[0].Rows[0].Tags
	}
	if got := tagLine(rowTags(list)); got != "math, poetry, engines" {
		t.Errorf("without a filter the row's tags = %s, want the card's order", got)
	}
	if got := tagLine(rowTags(list + "?tag=POETRY")); got != "poetry*, math, engines" {
		t.Errorf("filtered by poetry the row's tags = %s, want poetry first and the rest in the card's order", got)
	}
}
