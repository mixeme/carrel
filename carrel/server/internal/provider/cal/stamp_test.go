package cal

import (
	"strings"
	"testing"
	"time"
)

// A write is a revision (RFC 5545 §3.8.7.2, §3.8.7.3): an edit sets DTSTAMP and
// LAST-MODIFIED to the time of the write, in UTC, and touches nothing else; a
// new record gets both too. A client that shows the time of change shows
// Carrel's edit, not the one before it.
func TestWritesStampTheRevision(t *testing.T) {
	p, srv := davProvider(t)
	p.now = func() time.Time { return time.Date(2026, 9, 15, 10, 30, 0, 0, time.FixedZone("MSK", 3*3600)) }
	const want = "20260915T073000Z"

	path := work + "n1.ics"
	etag := srv.Seed(path, "BEGIN:VCALENDAR\nVERSION:2.0\nPRODID:-//T//EN\nBEGIN:VJOURNAL\nUID:n1\n"+
		"DTSTAMP:20260101T000000Z\nLAST-MODIFIED:20260101T000000Z\nSUMMARY:Note\nX-KEEP:yes\nEND:VJOURNAL\nEND:VCALENDAR\n")
	seeded, _, _ := srv.Object(path)
	obj, err := Parse(path, etag, strings.NewReader(seeded))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Update(t.Context(), obj); err != nil {
		t.Fatal(err)
	}
	got, _, _ := srv.Object(path)
	for _, line := range []string{"DTSTAMP:" + want, "LAST-MODIFIED:" + want, "SUMMARY:Note", "X-KEEP:yes", "UID:n1"} {
		if !strings.Contains(got, line) {
			t.Errorf("edited note lacks %q:\n%s", line, got)
		}
	}
	if strings.Contains(got, "20260101T000000Z") {
		t.Errorf("the time of change from before the edit stayed:\n%s", got)
	}

	fresh, err := NewObject("VTODO", "t1", time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Create(t.Context(), work+"t1.ics", fresh); err != nil {
		t.Fatal(err)
	}
	created, _, _ := srv.Object(work + "t1.ics")
	if !strings.Contains(created, "DTSTAMP:"+want) || !strings.Contains(created, "LAST-MODIFIED:"+want) {
		t.Errorf("new task:\n%s", created)
	}
}
