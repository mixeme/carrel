package cal

import (
	"testing"
)

// Typecase's notes 111 p. 5, 113; s-io-7: a record whose time cannot be read
// is refused alone, with the line it stands on; the file's other records
// are read all the same.
func TestSplitRefusesARecordWithATimeItCannotRead(t *testing.T) {
	doc := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//T//EN\r\n" +
		"BEGIN:VTODO\r\nUID:ok\r\nDTSTAMP:20260101T000000Z\r\nSUMMARY:Купить\r\nDUE;VALUE=DATE:20260901\r\nEND:VTODO\r\n" +
		"BEGIN:VTODO\r\nUID:bad\r\nDTSTAMP:20260101T000000Z\r\nSUMMARY:Сдать отчёт\r\nDUE;VALUE=DATE:20261301\r\nEND:VTODO\r\n" +
		"BEGIN:VEVENT\r\nUID:rule\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20260105T100000Z\r\nSUMMARY:Планёрка\r\nRRULE:FREQ=SOMETIMES\r\nEND:VEVENT\r\n" +
		"END:VCALENDAR\r\n"
	objs, strays, err := Split([]byte(doc), importStamp)
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 1 {
		t.Fatalf("records = %d", len(objs))
	}
	if len(strays) != 2 || !strays[0].Broken || strays[0].Summary != "Сдать отчёт" || strays[0].Reason != "line 14: cannot read DUE «20261301»" {
		t.Errorf("strays = %+v", strays)
	}
	if len(strays) == 2 && strays[1].Reason != "line 21: cannot read RRULE «FREQ=SOMETIMES»" {
		t.Errorf("the rule = %q", strays[1].Reason)
	}
}
