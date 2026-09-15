package card

import (
	"strings"
	"testing"
	"time"
)

// A write is a revision (RFC 6350 §6.7.4): an edit sets REV to the time of the
// write, in UTC, and touches nothing else; a new contact gets one too.
func TestWritesStampTheRevision(t *testing.T) {
	p, srv := davProvider(t)
	p.now = func() time.Time { return time.Date(2026, 9, 15, 10, 30, 0, 0, time.FixedZone("MSK", 3*3600)) }
	const want = "REV:20260915T073000Z"

	path := book + "ada.vcf"
	etag := srv.Seed(path, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada\nREV:20260101T000000Z\nX-KEEP:yes\nEND:VCARD\n")
	seeded, _, _ := srv.Object(path)
	obj, err := Parse(path, etag, strings.NewReader(seeded))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Update(t.Context(), obj); err != nil {
		t.Fatal(err)
	}
	got, _, _ := srv.Object(path)
	for _, line := range []string{want, "FN:Ada", "X-KEEP:yes", "UID:ada", "VERSION:4.0"} {
		if !strings.Contains(got, line) {
			t.Errorf("edited card lacks %q:\n%s", line, got)
		}
	}
	if strings.Contains(got, "20260101T000000Z") {
		t.Errorf("the revision from before the edit stayed:\n%s", got)
	}

	if _, err := p.Create(t.Context(), book+"bob.vcf", NewObject("bob")); err != nil {
		t.Fatal(err)
	}
	if created, _, _ := srv.Object(book + "bob.vcf"); !strings.Contains(created, want) {
		t.Errorf("new contact:\n%s", created)
	}
}
