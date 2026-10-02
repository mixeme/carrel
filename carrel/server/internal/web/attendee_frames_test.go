package web_test

import (
	"net/url"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// frameAttendee is one attendee of a card as the frame draws it: a known
// person — the mark of their book and a link — or an unknown one by the
// address, mono; and the status after it.
type frameAttendee struct {
	known  bool
	status string
}

// frameAttendees reads the attendees of a card's frame (s-agenda-1, -2): the
// count of the rubric and the rows in columns.
func frameAttendees(t *testing.T, id string) (count string, rows []frameAttendee) {
	t.Helper()
	frame := mockupFrame(t, id)
	head := regexp.MustCompile(`<div class="tc-rubric">Attendees <span class="tc-mono[^"]*">(\d+)</span></div>\s*<ul class="tc-sec__list tc-sec__list--cols">(.*?)</ul>`).FindStringSubmatch(frame)
	if head == nil {
		t.Fatalf("%s draws no attendees in columns: read the frame again", id)
	}
	for _, li := range regexp.MustCompile(`<li>(.*?)</li>`).FindAllStringSubmatch(head[2], -1) {
		known := regexp.MustCompile(`^<span class="tc-mark[^"]*" data-tc-colour="#[0-9A-Fa-f]{6}"></span> <a [^>]*>[^<]+</a> `).MatchString(li[1])
		unknown := regexp.MustCompile(`^<span class="tc-mono">[^<@]+@[^<]+</span> `).MatchString(li[1])
		status := regexp.MustCompile(`<span class="tc-dim">· ([^<]+)</span>$`).FindStringSubmatch(li[1])
		if known == unknown || status == nil {
			t.Fatalf("%s: an attendee the reading does not know: %s", id, li[1])
		}
		rows = append(rows, frameAttendee{known: known, status: status[1]})
	}
	return head[1], rows
}

// An attendee is an address; the address books only say who it is (the
// owner's decisions of 01–02.10.2026, task 77). The card and the panel draw
// them in columns, the organiser first: a person found by the address in a
// ticked address book is the mark of their book and a link to the card, an
// unknown one the address (s-agenda-1, -2). The books are the ticked ones
// whose lists the session has read: no server is asked for it, as for the
// records of Related.
func TestAttendeesAreTheFrames(t *testing.T) {
	for _, id := range []string{"s-agenda-1", "s-agenda-2"} {
		count, rows := frameAttendees(t, id)
		if count != "4" || len(rows) != 4 || rows[0].status != "organiser" || !rows[1].known || rows[3].known {
			t.Fatalf("%s: %s attendees %+v — read the frame again", id, count, rows)
		}
	}
	app := newMockupApp(t)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	rec := app.listPath(t, "calendar", "lichnyj") + "/8f1c-4a02-b7d3"
	people := func() []view.AttendeeRow {
		t.Helper()
		return decode[view.EventPage](t, app.h.do(get(rec, app.sess))).Attendees
	}
	for _, p := range people() {
		if p.Href != "" || p.Colour != "" {
			t.Errorf("%s is known before any book was read: %+v", p.Email, p)
		}
	}
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	got := people()
	byMail := map[string]view.AttendeeRow{}
	for _, p := range got {
		byMail[string(p.Email)] = p
	}
	if len(got) != 4 || got[0].Status != "organiser" {
		t.Fatalf("attendees = %+v, want four, the organiser first", got)
	}
	anna := byMail["a.kovaleva@example.org"]
	if anna.Href != app.listPath(t, "contacts", "kontakty")+"/4f2b-9c31-a0e7" || !strings.EqualFold(string(anna.Colour), "#4A6B52") || anna.Name != "Анна Ковалёва" {
		t.Errorf("Анна = %+v, want her card and the colour of Контакты", anna)
	}
	for _, mail := range []string{"mix@example.org", "v@example.org"} {
		if p, ok := byMail[mail]; !ok || p.Href != "" || p.Colour != "" {
			t.Errorf("%s = %+v, want an address in no ticked book", mail, p)
		}
	}
	// The panel of the agenda is the card's.
	agenda := decode[view.AgendaPage](t, app.h.do(get(app.listPath(t, "calendar", "lichnyj")+"?from=2026-08-10&open="+url.QueryEscape(rec), app.sess)))
	if agenda.Details == nil || len(agenda.Details.Attendees) != 4 || agenda.Details.Attendees[1].Href == "" && agenda.Details.Attendees[2].Href == "" {
		t.Errorf("the panel's attendees = %+v", agenda.Details)
	}
}
