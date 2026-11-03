package web

import (
	"errors"
	"testing"

	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/card"
	"carrel.dev/carrel/server/view"
)

// The notice of records the server returned and Carrel could not read (the
// owner's words of 02.11.2026): how many are not shown and why, that the
// server still holds them unchanged; each by its path and the parser's
// error in the details. A card and a calendar record say it alike.
func TestUnreadableWordsAreDirect(t *testing.T) {
	bad := errors.New("vcard: missing END:VCARD")
	cards := unreadableCardsNotice(&card.PartialRead{Records: []card.Unreadable{{Path: "/dav/people/broken.vcf", Err: bad}}})
	events := appendUnreadable(nil, unreadableOf([]cal.Unreadable{
		{Path: "/dav/work/a.ics", Err: errors.New("ical: no END:VEVENT")},
		{Path: "/dav/work/b.ics", Err: errors.New("ical: no END:VEVENT")},
	}))
	for _, c := range []struct {
		n           view.Notice
		title, body string
		first       string
		lines       int
	}{
		{cards, "1 record not shown: invalid vCard.", "Unchanged on the server.", "/dav/people/broken.vcf · vcard: missing END:VCARD", 1},
		{events[0], "2 records not shown: invalid iCalendar.", "Unchanged on the server.", "/dav/work/a.ics · ical: no END:VEVENT", 2},
	} {
		if string(c.n.Title) != c.title || string(c.n.Body) != c.body || c.n.Role != view.NoticeAlert {
			t.Errorf("%q / %q (%s),\nwant %q / %q", c.n.Title, c.n.Body, c.n.Role, c.title, c.body)
		}
		if d := c.n.Details; d == nil || d.Summary != "Not shown" || len(d.Lines) != c.lines || string(d.Lines[0]) != c.first {
			t.Errorf("details = %+v,\nwant first %q", d, c.first)
		}
	}
	// The export they stop, in the owner's words of 03.11.2026.
	two := []cal.Unreadable{{Path: "/dav/people/a1b2.vcf", Err: bad}, {Path: "/dav/people/c3d4.vcf", Err: bad}}
	for _, c := range []struct {
		n           view.Notice
		title, body string
	}{
		{exportStopped(true, two), "Export stopped: 2 records are invalid vCard.", "No file was made. The records are unchanged on the server."},
		{exportStopped(false, two[:1]), "Export stopped: 1 record is invalid iCalendar.", "No file was made. The record is unchanged on the server."},
	} {
		if string(c.n.Title) != c.title || string(c.n.Body) != c.body || c.n.Role != view.NoticeAlert {
			t.Errorf("%q / %q (%s),\nwant %q / %q", c.n.Title, c.n.Body, c.n.Role, c.title, c.body)
		}
		if d := c.n.Details; d == nil || d.Summary != "Not exported" || string(d.Lines[0]) != "/dav/people/a1b2.vcf · vcard: missing END:VCARD" {
			t.Errorf("details = %+v", d)
		}
	}
}
