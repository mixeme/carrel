package publish

import (
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/view"
)

// webcal:// is HTTPS; another Carrel's secret link is read at its download;
// anything that is not an address of the web is refused.
func TestFeedURL(t *testing.T) {
	token := strings.Repeat("a", 43)
	for _, c := range []struct{ in, want string }{
		{"webcal://feeds.example/ru.ics", "https://feeds.example/ru.ics"},
		{"https://feeds.example/ru.ics?key=1", "https://feeds.example/ru.ics?key=1"},
		{"https://carrel.example/p/" + token, "https://carrel.example/p/" + token + "/download"},
		{"https://carrel.example/p/" + token + "/", "https://carrel.example/p/" + token + "/download"},
		{"file:///etc/passwd", ""},
		{"not an address", ""},
	} {
		got, err := FeedURL(c.in)
		if got != c.want || (c.want == "") != (err != nil) {
			t.Errorf("%s: %q %v, want %q", c.in, got, err, c.want)
		}
	}
}

// A feed lands where its records belong; its name is its own; an answer
// that is neither format is said to be so.
func TestParseFeed(t *testing.T) {
	doc := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nX-WR-CALNAME:Лига\r\n" +
		"BEGIN:VEVENT\r\nUID:e\r\nDTSTAMP:20260801T000000Z\r\nDTSTART:20260901T100000Z\r\nSUMMARY:Match\r\nEND:VEVENT\r\n" +
		"BEGIN:VJOURNAL\r\nUID:n\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:Rules\r\nEND:VJOURNAL\r\nEND:VCALENDAR\r\n"
	p, err := Parse([]byte(doc), time.Now())
	if err != nil || p.Name != "Лига" || len(p.Kinds()) != 2 || p.Kinds()[0] != view.KindEvent || p.Kinds()[1] != view.KindNote {
		t.Fatalf("parsed = %+v %v", p, err)
	}
	if p.Records[view.KindEvent][0].Path != "feed/e" {
		t.Errorf("path = %q", p.Records[view.KindEvent][0].Path)
	}
	book, err := Parse([]byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:c\r\nFN:Ann\r\nEND:VCARD\r\n"), time.Now())
	if err != nil || len(book.Records[view.KindContact]) != 1 {
		t.Errorf("book = %+v %v", book, err)
	}
	if _, err := Parse([]byte("<html>"), time.Now()); err != ErrNotAFeed {
		t.Errorf("html: %v", err)
	}
}
