package web

import (
	"testing"

	"carrel.dev/carrel/server/internal/discover"
)

// The owner, 04.11.2026 (Typecase's note 168): the invitations are sent by
// «the server», not by its name, in every answer the server gave about
// them; the frames draw the one that sends (s-agenda-3), the other two
// follow its words.
func TestTheAttendeeHintSaysTheServer(t *testing.T) {
	for _, tc := range []struct {
		sch  string
		want string
	}{
		{discover.SchedulingYes, "A name or an address. On save, the server sends the invitations, not Carrel."},
		{discover.SchedulingNo, "A name or an address. The server sends no invitations, and neither does Carrel."},
		{discover.SchedulingUnknown, "A name or an address. The server did not say whether it sends invitations; Carrel does not."},
	} {
		if got := attendeeHint(tc.sch); string(got) != tc.want {
			t.Errorf("%q: %q, want %q", tc.sch, got, tc.want)
		}
	}
}
