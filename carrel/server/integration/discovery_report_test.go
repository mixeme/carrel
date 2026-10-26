//go:build integration

package integration

import (
	"context"
	"os"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/discover"
)

// A report, not a check. For every server the credentials name it says what
// discovery finds: the principal, the homes, and every collection with its
// kinds, its rights and the size it takes. It writes nothing anywhere.
//
// This is how a server joins the live set: run the report, read what the
// server actually is — a whole account, or one calendar somebody handed over —
// and then write only the tests it can take. Kept out of the live set itself
// (a report has nothing to fail), it runs on purpose:
//
//	CARREL_LIVE_REPORT=1 go test -tags=integration -run TestLiveDiscoveryReport ./integration/ -v
func TestLiveDiscoveryReport(t *testing.T) {
	if os.Getenv("CARREL_LIVE_REPORT") == "" {
		t.Skip("set CARREL_LIVE_REPORT=1 for the report of what every live server holds")
	}
	liveCredEnv()
	for _, s := range liveServers() {
		t.Run(s.name, func(t *testing.T) {
			if s.url == "" || s.user == "" || s.pass == "" {
				t.Skip("no credentials in myenv/carrel/credentials.txt")
			}
			reportServer(t, s)
		})
	}
}

type liveServer struct{ name, url, user, pass string }

// liveServers is every server the credentials file can name. A server it does
// not name skips itself.
func liveServers() []liveServer {
	env := os.Getenv
	out := []liveServer{
		{"baikal", env("CARREL_BAIKAL_URL"), env("CARREL_BAIKAL_USER"), env("CARREL_BAIKAL_PASS")},
		{"sftpgo", env("CARREL_SFTPGO_URL"), env("CARREL_SFTPGO_USER"), env("CARREL_SFTPGO_PASS")},
		{"opencloud", env("CARREL_OPENCLOUD_URL"), env("CARREL_OPENCLOUD_USER"), env("CARREL_OPENCLOUD_PASS")},
		{"yandex-caldav", env("CARREL_YANDEX_CALDAV_URL"), env("CARREL_YANDEX_CALDAV_USER"), env("CARREL_YANDEX_CALDAV_PASS")},
		{"yandex-carddav", env("CARREL_YANDEX_CARDDAV_URL"), env("CARREL_YANDEX_CARDDAV_USER"), env("CARREL_YANDEX_CARDDAV_PASS")},
		{"mailru", env("CARREL_MAILRU_URL"), env("CARREL_MAILRU_USER"), env("CARREL_MAILRU_PASS")},
		{"sogo-calendar", env("CARREL_SOGO_CAL_URL"), env("CARREL_SOGO_USER"), env("CARREL_SOGO_PASS")},
		{"sogo-contacts", env("CARREL_SOGO_CARD_URL"), env("CARREL_SOGO_USER"), env("CARREL_SOGO_PASS")},
		{"google-carddav", env("CARREL_GOOGLE_CARDDAV_URL"), env("CARREL_GOOGLE_CARDDAV_USER"), env("CARREL_GOOGLE_CARDDAV_PASS")},
		{"google-caldav", env("CARREL_GOOGLE_CALDAV_URL"), env("CARREL_GOOGLE_CALDAV_USER"), env("CARREL_GOOGLE_CALDAV_PASS")},
		{"nextcloud", env("CARREL_NEXTCLOUD_URL"), env("CARREL_NEXTCLOUD_USER"), env("CARREL_NEXTCLOUD_PASS")},
		{"icloud-carddav", env("CARREL_ICLOUD_CARDDAV_URL"), env("CARREL_ICLOUD_CARDDAV_USER"), env("CARREL_ICLOUD_CARDDAV_PASS")},
		{"icloud-caldav", env("CARREL_ICLOUD_CALDAV_URL"), env("CARREL_ICLOUD_CALDAV_USER"), env("CARREL_ICLOUD_CALDAV_PASS")},
	}
	if u := env("CARREL_RADICALE_URL"); u != "" {
		out = append(out, liveServer{"radicale", u, radicaleUser, radicalePass})
	}
	return out
}

func reportServer(t *testing.T, s liveServer) {
	t.Helper()
	res, trace, err := discover.Run(context.Background(), testGuard(t), s.url, s.user, s.pass, "")
	if trace != nil {
		for _, st := range trace.Steps {
			t.Logf("step %-24s %-5s %s", st.Name, st.State, st.Detail)
		}
	}
	if err != nil {
		t.Logf("discovery refused: %v", err)
		return
	}
	t.Logf("base      %s", res.BaseURL)
	t.Logf("principal %s", res.Principal)
	t.Logf("homes     calendar=%q addressbook=%q fileOnly=%v", res.CalendarHome, res.AddressBookHome, res.FileOnly)
	t.Logf("schedules %q addresses=%q", res.Scheduling, res.Addresses)
	if len(res.Collections) == 0 {
		t.Log("collections: none")
		return
	}
	for _, c := range res.Collections {
		kinds := make([]string, len(c.Kinds))
		for i, k := range c.Kinds {
			kinds[i] = string(k)
		}
		t.Logf("  %-58s kinds=%-22s readOnly=%-5v comps=%v max=%d name=%q",
			c.Path, strings.Join(kinds, ","), c.ReadOnly, c.Components, c.MaxResourceSize, c.DisplayName)
	}
}
