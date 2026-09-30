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
	want := []string{"Phone · mobile", "Phone · work", "Email · work", "Birthday", "Tags", "Note"}
	if !equalStrings(labels, want) {
		t.Errorf("fields = %v,\nwant      %v", labels, want)
	}
}

// The first thing offered on a record is the thing most often done with it:
// Edit on view, the deletion last and marked (s-contacts-6); moving,
// copying, exporting and publishing a secret link are under «⋯»
// (s-contacts-10) — not what a person came to the card for.
func TestTheActionsOfACardStandInTheOrderOfTheFrame(t *testing.T) {
	page := annaPage(t)
	want := []string{"record.edit", "record.print", "record.delete"}
	if got := ids(page.Actions); !equalStrings(got, want) {
		t.Errorf("actions = %v,\nwant    %v", got, want)
	}
	if last := page.Actions[len(page.Actions)-1]; !last.Danger {
		t.Errorf("the deletion is not marked: %+v", last)
	}
	want = []string{"record.move", "record.copy", "record.export", "publish"}
	if got := ids(menuActions(page.Menu)); !equalStrings(got, want) {
		t.Errorf("menu = %v,\nwant %v", got, want)
	}
}

// A place is «Where», as the frames of an event and a task write it
// (s-agenda-2): «Location» is the name of the property, and the card is
// read by a person, not by a parser.
func TestAPlaceIsWhere(t *testing.T) {
	f := calendarFixture(t, nil, "e1.ics",
		ics("BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260801T000000Z\nSUMMARY:Standup\nLOCATION:Room 4\nDTSTART;VALUE=DATE:20260810\nEND:VEVENT"))
	page := decode[view.EventPage](t, f.h.do(get(f.path("calendar")+"/e1", f.sess)))
	for _, fl := range page.Fields {
		if fl.Label == "Location" {
			t.Fatalf("the card says «Location»: %+v", page.Fields)
		}
		if fl.Label == "Where" && fl.Value == "Room 4" {
			return
		}
	}
	t.Errorf("no place among %+v", page.Fields)
}

// The form of a contact asks in the order of the frame (s-contacts-3): who
// the person is, then where they work and when they were born, then how to
// reach them, and last what is written about them. The nickname stands in
// the head empty too (s-contacts-4).
func TestTheContactFormAsksInTheOrderOfTheFrame(t *testing.T) {
	f := contactsFixture(t, false, annaCard)
	page := decode[view.ContactEditPage](t, f.h.do(get(f.path("contacts")+"/anna/edit", f.sess)))
	var names []string
	for _, set := range page.Form.Sets {
		for _, r := range set.Rows {
			switch r.Name {
			case "_csrf", "etag", "readat":
			default:
				names = append(names, r.Name)
			}
		}
	}
	want := []string{"fn", "n.given", "n.family", "nickname", "org", "title", "bday"}
	if len(names) < len(want) || !equalStrings(names[:len(want)], want) {
		t.Errorf("the form asks %v,\nand should open with %v", names, want)
	}
}
