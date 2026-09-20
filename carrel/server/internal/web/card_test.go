package web_test

import (
	"testing"

	"carrel.dev/carrel/server/view"
)

const annaCard = "BEGIN:VCARD\nVERSION:3.0\nUID:anna\nN:Ковалёва;Анна;;;\nFN:Анна Ковалёва\n" +
	"ORG:Издательство «Слово»\nTITLE:Выпускающий редактор\n" +
	"TEL;TYPE=CELL:+7 912 445-10-88\nTEL;TYPE=WORK:+7 495 120-45-00\n" +
	"EMAIL;TYPE=WORK:a.kovaleva@example.org\nBDAY:1987-03-14\n" +
	"CATEGORIES:работа,издательство\nNOTE:Пишет по вторникам.\nEND:VCARD\n"

func annaPage(t *testing.T) view.ContactPage {
	t.Helper()
	f := contactsFixture(t, false, annaCard)
	return decode[view.ContactPage](t, f.h.do(get(f.path("contacts")+"/anna", f.sess)))
}

// The head of a card is the person: their name, and under it where they
// work and as what (s-contacts-6). Organisation and title do not stand in
// the list of fields — the client must not have to guess which two rows to
// lift out of it.
func TestTheCardHeadCarriesTheWork(t *testing.T) {
	page := annaPage(t)
	if got := string(page.Subtitle); got != "Издательство «Слово» · Выпускающий редактор" {
		t.Errorf("subtitle = %q", got)
	}
	for _, f := range page.Fields {
		if f.Label == "Organisation" || f.Label == "Title" {
			t.Errorf("%q is in the head and must not be a field too", f.Label)
		}
	}
}

// The order of the frame: how to reach the person first, then who they are,
// then what is written about them.
func TestTheFieldsOfACardStandInTheOrderOfTheFrame(t *testing.T) {
	page := annaPage(t)
	var labels []string
	for _, f := range page.Fields {
		labels = append(labels, string(f.Label))
	}
	want := []string{"Phone · mobile", "Phone · work", "Email · work", "Birthday", "Categories", "Note"}
	if !equalStrings(labels, want) {
		t.Errorf("fields = %v,\nwant      %v", labels, want)
	}
}

// The first thing offered on a record is the thing most often done with it:
// Edit, then the ways of sending and moving it, and the deletion last and
// marked (s-contacts-6). Publishing a secret link is not what a person came
// to the card for.
func TestTheActionsOfACardStandInTheOrderOfTheFrame(t *testing.T) {
	page := annaPage(t)
	var ids []string
	for _, a := range page.Actions {
		ids = append(ids, a.ID)
	}
	want := []string{"record.edit", "record.move", "record.copy", "publish", "record.delete"}
	if !equalStrings(ids, want) {
		t.Errorf("actions = %v,\nwant    %v", ids, want)
	}
	if last := page.Actions[len(page.Actions)-1]; !last.Danger {
		t.Errorf("the deletion is not marked: %+v", last)
	}
}
