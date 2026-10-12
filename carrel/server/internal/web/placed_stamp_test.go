package web_test

import (
	"net/http"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/davtest"
	"github.com/emersion/go-webdav/caldav"
)

// Б34 of the audit of 11.10.2026: a record moved to another server, copied
// or imported was written with «changed now» — LAST-MODIFIED of an event,
// REV of a card —, though nobody changed it: every list sorted by Changed
// put it first, and the day it was really changed was gone. A record put
// where it was not made keeps its time of change.
func TestARecordPutElsewhereKeepsWhenItWasChanged(t *testing.T) {
	t.Run("a card moved to another account", func(t *testing.T) {
		f := newMoveFixture(t)
		f.srv.Seed(peopleBook+"ada.vcf", strings.Replace(movedCard, "FN:Ada\n", "FN:Ada\nREV:20250101T000000Z\n", 1))
		resp := f.h.do(postConfirm(t, f, sheetFor(f, peopleBook, "ada", f.otherID, workBook, false)))
		if resp.Code != http.StatusSeeOther {
			t.Fatalf("move: %d %s", resp.Code, resp.Body.String())
		}
		copied, _, ok := f.other.Object(workBook + "ada.vcf")
		if !ok {
			t.Fatalf("no copy on the other account: %v", f.other.Paths(workBook))
		}
		mustContain(t, copied, "REV:20250101T000000Z")
	})
	t.Run("an event imported", func(t *testing.T) {
		srv := davtest.New(t)
		srv.Login, srv.Password = "alice", "secret"
		work := davtest.CalendarHome + "work/"
		srv.AddCalendar(caldav.Calendar{Path: work, Name: "Work"})
		f := connectFixture(t, srv, work, nil)
		file := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Other//Other//EN\r\n" +
			"BEGIN:VEVENT\r\nUID:old\r\nDTSTAMP:20250101T000000Z\r\nLAST-MODIFIED:20250102T030405Z\r\n" +
			"DTSTART:20260107T100000Z\r\nDTEND:20260107T110000Z\r\nSUMMARY:Old\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
		if page := runImportOf(t, f, "calendar", "old.ics", file); page.Report.Created != 1 {
			t.Fatalf("report = %+v", page.Report)
		}
		mustContain(t, storedWith(t, f, work, "UID:old"), "LAST-MODIFIED:20250102T030405Z")
	})
}
