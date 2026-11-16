//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// S-30.2 on Mail.ru (owner's decision 20.09.2026). The account gives the
// address of one calendar and that address is the collection (SV-22): there
// is no home to make a second in, so the run edits an event in the calendar
// that is there — the fields, the rule of a series, the zone — and leaves
// nothing behind but what it cleans up.
//
// Until now Mail.ru had no test of its own at all, only a line in the
// discovery report.

func mailruConfig(t *testing.T) (endpoint, user, pass string) {
	liveCredEnv()
	endpoint = os.Getenv("CARREL_MAILRU_URL")
	user = os.Getenv("CARREL_MAILRU_USER")
	pass = os.Getenv("CARREL_MAILRU_PASS")
	if endpoint == "" || user == "" || pass == "" {
		t.Skip("CARREL_MAILRU_URL, CARREL_MAILRU_USER, CARREL_MAILRU_PASS required")
	}
	return endpoint, user, pass
}

func TestMailruEventFields(t *testing.T) {
	endpoint, user, pass := mailruConfig(t)
	ctx := context.Background()
	client := liveClient(t, endpoint, user, pass)
	app := newLiveApp(t)
	connID := connectLive(t, app, endpoint, user, pass)

	var cal sealed.Collection
	for _, c := range liveCatalog(t, app).Collections {
		if c.Has(view.KindEvent) {
			cal = c
			break
		}
	}
	if cal.Path == "" {
		t.Skip("no calendar in the catalog")
	}
	suffix := fmt.Sprintf("carrel-mailru-%d-%d", os.Getpid(), time.Now().Unix()%100000)
	liveEventFields(ctx, t, app, client, connID, cal.Path, suffix)
	t.Run("matrix", func(t *testing.T) {
		liveMatrix(ctx, t, client, endpoint, user, pass, map[view.Kind]string{view.KindEvent: cal.Path, view.KindTask: cal.Path}, suffix)
	})
}
