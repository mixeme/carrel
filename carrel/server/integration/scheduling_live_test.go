//go:build integration

package integration

import (
	"context"
	"testing"

	"carrel.dev/carrel/server/internal/discover"
)

// S-6, S-15.4: discovery asks the principal whether it schedules. The plan
// of 24.09.2026 names what each server says: Nextcloud, Baikal, SoGo, Yandex
// and Google announce a schedule-outbox-URL; Mail.ru hands out a calendar's
// own address and has no principal to ask, so it stays unknown; Radicale
// does not implement RFC 6638. A server whose answer differs from this
// list fails: the rewrite sheet would promise letters that never come, or
// keep quiet about letters that do.
func TestLiveSchedulingIsKnown(t *testing.T) {
	liveCredEnv()
	want := map[string]string{
		"nextcloud":     discover.SchedulingYes,
		"baikal":        discover.SchedulingYes,
		"sogo-calendar": discover.SchedulingYes,
		"yandex-caldav": discover.SchedulingYes,
		"google-caldav": discover.SchedulingYes,
		"mailru":        discover.SchedulingUnknown,
		"radicale":      discover.SchedulingNo,
	}
	for _, s := range liveServers() {
		expected, named := want[s.name]
		if !named {
			continue
		}
		t.Run(s.name, func(t *testing.T) {
			if s.url == "" || s.user == "" || s.pass == "" {
				t.Skip("no credentials in dev/credentials.txt")
			}
			res, _, err := discover.Run(context.Background(), testGuard(t), s.url, s.user, s.pass, "")
			if err != nil {
				t.Skipf("%s did not answer discovery: %v", s.url, err)
			}
			if res.Scheduling != expected {
				t.Errorf("scheduling = %q, want %q (addresses %q)", res.Scheduling, expected, res.Addresses)
			}
			if expected == discover.SchedulingYes && len(res.Addresses) == 0 {
				t.Logf("%s schedules but named no mailto: address of the person", s.name)
			}
		})
	}
}

// Xandikos does not implement RFC 6638: it answers the principal without an
// outbox.
func TestXandikosDoesNotSchedule(t *testing.T) {
	endpoint, user, pass := xandikosConfig(t)
	res, _, err := discover.Run(context.Background(), testGuard(t), endpoint, user, pass, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Scheduling != discover.SchedulingNo {
		t.Errorf("scheduling = %q, want %q", res.Scheduling, discover.SchedulingNo)
	}
}
