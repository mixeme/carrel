package web_test

import (
	"fmt"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// A book bigger than one answer may be (CARREL_DAV_MAX_RESPONSE_BYTES, 10 MiB)
// is read in parts, not refused. Three hundred cards with a 40 KB photo each
// is an ordinary book from a phone, and it used to show as "People is
// unavailable" — a server that answered, called silent.
func TestABigBookIsReadInParts(t *testing.T) {
	photo := strings.Repeat("A", 40_000)
	var cards []string
	for i := 0; i < 300; i++ {
		cards = append(cards, fmt.Sprintf("BEGIN:VCARD\nVERSION:3.0\nUID:u%03d\nFN:Person %03d\nPHOTO;ENCODING=b;TYPE=JPEG:%s\nEND:VCARD\n", i, i, photo))
	}
	f := contactsFixture(t, false, cards...)
	page := decode[view.ContactsListPage](t, f.h.do(get(f.path("contacts"), f.sess)))
	n := 0
	for _, g := range page.Groups {
		n += len(g.Rows)
	}
	if n != 300 || len(page.Notices) > 0 {
		t.Errorf("rows %d, notices %+v", n, page.Notices)
	}
}

// The same for a calendar read whole: a list of tasks has no window.
func TestABigTaskListIsReadInParts(t *testing.T) {
	text := strings.Repeat("x", 40_000)
	var files []string
	for i := 0; i < 300; i++ {
		files = append(files, fmt.Sprintf("t%03d.ics", i),
			ics(fmt.Sprintf("BEGIN:VTODO\nUID:t%03d\nDTSTAMP:20260101T000000Z\nSUMMARY:Task %03d\nDESCRIPTION:%s\nEND:VTODO", i, i, text)))
	}
	f := calendarFixture(t, nil, files...)
	page := decode[view.TasksPage](t, f.h.do(get(f.path("tasks")+"?seg=all", f.sess)))
	n := 0
	for _, g := range page.Groups {
		n += len(g.Rows)
	}
	if n != 300 || len(page.Notices) > 0 {
		t.Errorf("rows %d, notices %+v", n, page.Notices)
	}
}
