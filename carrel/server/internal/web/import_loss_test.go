package web_test

import (
	"testing"

	"carrel.dev/carrel/server/internal/davtest"
	"github.com/emersion/go-webdav/caldav"
)

// Б35 of the audit of 11.10.2026: an import did not read back what it
// wrote. A server that keeps less than it was sent answered 201, and the row
// said «Created» over a record that had lost a part of itself. Every other
// write is read back (S-7.1); now this one is, and the row and the note of
// the report say what the server did not keep.
func TestImportSaysWhatTheServerDidNotKeep(t *testing.T) {
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	work := davtest.CalendarHome + "work/"
	srv.AddCalendar(caldav.Calendar{Path: work, Name: "Work"})
	srv.Drop = []string{"LOCATION"}
	f := connectFixture(t, srv, work, nil)

	file := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Other//Other//EN\r\n" +
		"BEGIN:VEVENT\r\nUID:with-place\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20260107T100000Z\r\nDTEND:20260107T110000Z\r\nSUMMARY:With a place\r\nLOCATION:Room 4\r\nEND:VEVENT\r\n" +
		"BEGIN:VEVENT\r\nUID:plain\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20260108T100000Z\r\nDTEND:20260108T110000Z\r\nSUMMARY:Plain\r\nEND:VEVENT\r\n" +
		"END:VCALENDAR\r\n"
	page := runImportOf(t, f, "calendar", "week.ics", file)
	if page.Report.Created != 2 {
		t.Fatalf("report = %+v", page.Report)
	}
	// «Not kept by the server» (s-io-9; Typecase's note 111 p. 8): the
	// record that lost its place and what it lost, the one kept whole not.
	if page.Report.Lost != 1 || len(page.Report.Losts) != 1 || string(page.Report.Losts[0]) != "With a place · LOCATION" {
		t.Errorf("not kept = %d %q", page.Report.Lost, page.Report.Losts)
	}
}
