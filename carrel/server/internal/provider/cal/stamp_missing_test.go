package cal

import (
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav/caldav"
)

// Yandex keeps a task without its DTSTAMP (docs/servers.md, SV-69): every
// task of it was «not shown: invalid iCalendar» (acceptance on our test
// servers, 18.11.2026). DTSTAMP is the record's stamp, not its content
// (S-7.1): a record without one is read, its stamp taken from LAST-MODIFIED,
// else CREATED, else the time the answer gives.
func TestARecordWithoutItsStampIsRead(t *testing.T) {
	doc := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Yandex//EN\r\n" +
		"BEGIN:VTODO\r\nUID:aa10-77bc-0e31\r\nSUMMARY:Продлить страховку\r\nLAST-MODIFIED:20260801T090000Z\r\nEND:VTODO\r\n" +
		"END:VCALENDAR\r\n"
	data, err := ical.NewDecoder(strings.NewReader(doc)).Decode()
	if err != nil {
		t.Fatal(err)
	}
	modified := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	objs, bad := fromReports([]caldav.CalendarObject{{Path: "/c/aa10-77bc-0e31.ics", ETag: `"e1"`, ModTime: modified, Data: data}}, ical.CompToDo)
	if len(bad) != 0 || len(objs) != 1 {
		t.Fatalf("read %d, unreadable %+v", len(objs), bad)
	}
	if got := objs[0].Props()[ical.PropDateTimeStamp]; len(got) != 1 || got[0].Text != "20260801T090000Z" {
		t.Errorf("DTSTAMP = %+v, want LAST-MODIFIED's", got)
	}
	if got := objs[0].Props()[ical.PropSummary]; len(got) != 1 || got[0].Text != "Продлить страховку" {
		t.Errorf("SUMMARY = %+v", got)
	}

	bare := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Yandex//EN\r\n" +
		"BEGIN:VTODO\r\nUID:aa11\r\nSUMMARY:Собрать чемодан\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"
	data, err = ical.NewDecoder(strings.NewReader(bare)).Decode()
	if err != nil {
		t.Fatal(err)
	}
	objs, bad = fromReports([]caldav.CalendarObject{{Path: "/c/aa11.ics", ModTime: modified, Data: data}}, ical.CompToDo)
	if len(bad) != 0 || len(objs) != 1 {
		t.Fatalf("bare: read %d, unreadable %+v", len(objs), bad)
	}
	if got := objs[0].Props()[ical.PropDateTimeStamp]; len(got) != 1 || got[0].Text != "20260802T100000Z" {
		t.Errorf("bare DTSTAMP = %+v, want the answer's time", got)
	}
}
