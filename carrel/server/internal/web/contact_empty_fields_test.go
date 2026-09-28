package web_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// review Р-10, "Show N empty fields" (s-contacts-3): the optional fields a
// card does not fill come in a field set of their own, its legend saying
// how many; the client folds it. Filled ones stay where they are, and a new
// contact shows them all. The nickname and the birthday stand in the head
// empty too (s-contacts-4); an address without any is its «Add address».
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
	if got := strings.Join(names, " "); got != "title add categories note" || empty.Legend != "Show 4 empty fields" {
		t.Errorf("empty set %q = %s", empty.Legend, got)
	}
	var head []string
	for _, r := range edit.Form.Sets[0].Rows {
		head = append(head, r.Name)
		if r.Name == "title" {
			t.Error("an empty field is in the main set too")
		}
	}
	if !strings.Contains(strings.Join(head, " "), "nickname org bday") {
		t.Errorf("head = %q, want the empty nickname and birthday in it", head)
	}
	for _, set := range edit.Form.Sets {
		if set.Legend == "Other" || set.Legend == "Addresses" {
			t.Errorf("rubric %q drawn with all its fields folded", set.Legend)
		}
	}

	fresh := decode[view.ContactEditPage](t, f.h.do(get(f.path("contacts")+"/new", f.sess)))
	for _, set := range fresh.Form.Sets {
		if strings.HasPrefix(string(set.Legend), "Show ") {
			t.Error("a new contact hides its fields")
		}
	}
}
