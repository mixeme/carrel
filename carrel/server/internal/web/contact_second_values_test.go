package web_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// A property the card shows once — the name, the organisation and the title
// in its head — may stand twice on the server. The first is where it always
// was; the others are under «Kept as the server sent them», by the
// property's name (the owner's decision of 02.11.2026, variant Б): nothing
// of the record is out of sight, and the form stays as it is.
func TestTheSecondValuesOfTheHeadAreKept(t *testing.T) {
	card := strings.Replace(hardCard, "FN:Ada Lovelace\n", "FN:Ada Lovelace\nFN:Augusta Ada King\n", 1)
	f := contactsFixture(t, false, card)
	page := decode[view.ContactPage](t, f.h.do(get(f.path("contacts")+"/hard", f.sess)))

	if page.Name != "Ada Lovelace" || page.Subtitle != "Analytical, Engines · Mathematician" {
		t.Errorf("the head = %q / %q, want the first values", page.Name, page.Subtitle)
	}
	kept := map[string]bool{}
	for _, f := range page.Foreign {
		kept[string(f.Label)+" · "+string(f.Value)] = true
	}
	for _, want := range []string{"FN · Augusta Ada King", "ORG · Royal Society", "TITLE · Countess", "X-CUSTOM · keep"} {
		if !kept[want] {
			t.Errorf("not kept: %q in %+v", want, page.Foreign)
		}
	}
	for _, f := range page.Fields {
		if f.Value == "Countess" || f.Value == "Royal Society" {
			t.Errorf("a second value among the fields: %+v", f)
		}
	}
	// On paper the head's fields come first, each once; the rest is kept
	// there too (print-3).
	var lead []string
	for _, f := range page.Print.Lead {
		lead = append(lead, string(f.Label)+" · "+string(f.Value))
	}
	if strings.Join(lead, "; ") != "Organisation · Analytical, Engines; Title · Mathematician" {
		t.Errorf("on paper first = %q", lead)
	}
}
