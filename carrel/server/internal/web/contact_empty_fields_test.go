package web_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// forms-tail.md, "Show N empty fields" (s-contacts-3): the optional fields a
// card does not fill come in a field set of their own, its legend saying
// how many; the client folds it. Filled ones stay where they are, and a new
// contact shows them all.
func TestContactEmptyFieldsInTheirOwnSet(t *testing.T) {
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nORG:Analytical\nEND:VCARD\n")
	edit := decode[view.ContactEditPage](t, f.h.do(get(f.path("contacts")+"/ada/edit", f.sess)))
	var empty *view.FieldSet
	for i := range edit.Form.Sets {
		if strings.HasPrefix(string(edit.Form.Sets[i].Legend), "Show ") {
			empty = &edit.Form.Sets[i]
		}
	}
	if empty == nil {
		t.Fatalf("sets = %+v, want one of the empty fields", edit.Form.Sets)
	}
	var names []string
	for _, r := range empty.Rows {
		names = append(names, r.Name)
	}
	if got := strings.Join(names, " "); got != "nickname title bday categories note" || empty.Legend != "Show 5 empty fields" {
		t.Errorf("empty set %q = %s", empty.Legend, got)
	}
	for _, r := range edit.Form.Sets[0].Rows {
		if r.Name == "nickname" {
			t.Error("an empty field is in the main set too")
		}
	}
	rowNamed(t, edit.Form, "org")

	fresh := decode[view.ContactEditPage](t, f.h.do(get(f.path("contacts")+"/new", f.sess)))
	for _, set := range fresh.Form.Sets {
		if strings.HasPrefix(string(set.Legend), "Show ") {
			t.Error("a new contact hides its fields")
		}
	}
}
