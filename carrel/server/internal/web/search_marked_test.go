package web_test

import (
	"testing"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/view"
)

// Пк13 (s-find-1: «**Ковал**ёва Анна», «Звонок с **Ковал**ёвой»): a result's
// title in pieces, what matched the query strong, whatever its case; a
// result found by another field has no pieces.
func TestASearchMarksWhatMatched(t *testing.T) {
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work"})
	srv.Seed(searchBook+"ada.vcf", adaCard())
	srv.Seed(searchWork+"call.ics", ics("BEGIN:VEVENT\nUID:call\nDTSTAMP:20260101T000000Z\nDTSTART:20260910T150000Z\nSUMMARY:Call with ADA\nEND:VEVENT"))
	f := connectFixture(t, srv, searchBook, nil)
	tickAll(t, f.h, f.sess)
	page := search(t, f.h, f.sess, "q=ada")
	waitPoll(t, f.h, f.sess, page.Poll.TaskID, nil)
	page = search(t, f.h, f.sess, "q=ada")
	want := map[string]view.Runs{
		"Ada Lovelace":  {{Text: "Ada", Mark: view.RunStrong}, {Text: " Lovelace"}},
		"Call with ADA": {{Text: "Call with "}, {Text: "ADA", Mark: view.RunStrong}},
	}
	for _, row := range page.Results {
		w, ok := want[string(row.Title)]
		if !ok {
			continue
		}
		delete(want, string(row.Title))
		if len(row.Marked) != len(w) || row.Marked[0] != w[0] || row.Marked[1] != w[1] {
			t.Errorf("%q marked %+v, want %+v", row.Title, row.Marked, w)
		}
	}
	for title := range want {
		t.Errorf("no result %q in %+v", title, page.Results)
	}
}
