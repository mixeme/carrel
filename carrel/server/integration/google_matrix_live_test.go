//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"carrel.dev/carrel/server/view"
)

// The matrix «every field × a hard record» in the calendar this Google
// account already has (a calendar cannot be made here, MKCALENDAR answers
// 404). No record with attendees is seeded: Google would send the
// invitations from the owner's account (personalAccount). Every record is
// deleted by name on cleanup.
func TestGoogleCalendarMatrix(t *testing.T) {
	endpoint, user, pass := googleCalConfig(t)
	ctx := context.Background()
	client := liveClient(t, endpoint, user, pass)
	app := newLiveApp(t)
	connectLive(t, app, endpoint, user, pass)
	cal := ""
	for _, c := range liveCatalog(t, app).Collections {
		if c.Has(view.KindEvent) && !c.ReadOnly {
			cal = c.Path
			break
		}
	}
	if cal == "" {
		t.Skip("no calendar this account can write in the catalog")
	}
	suffix := fmt.Sprintf("carrel-google-%d-%d", os.Getpid(), time.Now().Unix()%100000)
	liveMatrix(ctx, t, client, endpoint, user, pass, map[view.Kind]string{view.KindEvent: cal, view.KindTask: cal}, suffix)
}
