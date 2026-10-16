package web_test

import (
	"strings"
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
	var lossy, whole string
	for _, table := range page.Report.Rows {
		for _, row := range table.Lines {
			row := row.Cells
			switch {
			case len(row) < 3:
			case row[0] == "With a place":
				lossy = string(row[2])
			case row[0] == "Plain":
				whole = string(row[2])
			}
		}
	}
	if !strings.Contains(lossy, "did not keep") || !strings.Contains(lossy, "LOCATION") {
		t.Errorf("the row of the record that lost its place says %q", lossy)
	}
	if whole != "Created" {
		t.Errorf("the row of the record kept whole says %q", whole)
	}
	if !strings.Contains(string(page.Report.Note), "not kept whole") {
		t.Errorf("the note of the report = %q", page.Report.Note)
	}
}
