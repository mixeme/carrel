package cal

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

var importStamp = time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

// mixedFile is what another program exports: an envelope of its own, one time
// zone, a series with an instance that overrides it, a task, a note, and a
// free/busy block that is no record.
const mixedFile = "BEGIN:VCALENDAR\r\n" +
	"VERSION:2.0\r\n" +
	"PRODID:-//Other//Other//EN\r\n" +
	"BEGIN:VTIMEZONE\r\nTZID:Europe/Moscow\r\n" +
	"BEGIN:STANDARD\r\nDTSTART:19700101T000000\r\nTZOFFSETFROM:+0300\r\nTZOFFSETTO:+0300\r\nEND:STANDARD\r\n" +
	"END:VTIMEZONE\r\n" +
	"BEGIN:VTIMEZONE\r\nTZID:Europe/Berlin\r\n" +
	"BEGIN:STANDARD\r\nDTSTART:19700101T000000\r\nTZOFFSETFROM:+0100\r\nTZOFFSETTO:+0100\r\nEND:STANDARD\r\n" +
	"END:VTIMEZONE\r\n" +
	"BEGIN:VEVENT\r\nUID:series\r\nDTSTAMP:20260101T000000Z\r\nDTSTART;TZID=Europe/Moscow:20260105T100000\r\n" +
	"RRULE:FREQ=WEEKLY\r\nSUMMARY:Ковка\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nUID:series\r\nRECURRENCE-ID;TZID=Europe/Moscow:20260112T100000\r\nDTSTAMP:20260101T000000Z\r\n" +
	"DTSTART;TZID=Europe/Moscow:20260112T110000\r\nSUMMARY:Ковка, позже\r\nEND:VEVENT\r\n" +
	"BEGIN:VTODO\r\nUID:task-1\r\nDTSTAMP:20260101T000000Z\r\nSUMMARY:Купить уголь\r\nEND:VTODO\r\n" +
	"BEGIN:VFREEBUSY\r\nUID:fb-1\r\nDTSTAMP:20260101T000000Z\r\nEND:VFREEBUSY\r\n" +
	"END:VCALENDAR\r\n"

// S-19: one record per master component, the instances that override it and
// the zones it names travelling with it; what is no record is named, not
// dropped.
func TestSplitKeepsSeriesInstanceAndZone(t *testing.T) {
	objs, strays, err := Split([]byte(mixedFile), importStamp)
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 2 {
		t.Fatalf("records = %d, want 2", len(objs))
	}
	if len(strays) != 1 || strays[0].Component != "VFREEBUSY" || strays[0].UID != "fb-1" {
		t.Fatalf("strays = %+v", strays)
	}

	series := string(objs[0].CalendarDoc())
	if UID(objs[0]) != "series" || objs[0].MainComponent() != "VEVENT" {
		t.Fatalf("first record = %s %s", objs[0].MainComponent(), UID(objs[0]))
	}
	if n := strings.Count(series, "BEGIN:VEVENT"); n != 2 {
		t.Errorf("the instance did not travel with its series: %d VEVENT in\n%s", n, series)
	}
	if !strings.Contains(series, "TZID:Europe/Moscow") {
		t.Errorf("the zone did not travel:\n%s", series)
	}
	if strings.Contains(series, "Europe/Berlin") {
		t.Errorf("a zone the record does not name travelled:\n%s", series)
	}
	if !strings.Contains(series, "PRODID:-//Other//Other//EN") {
		t.Errorf("the envelope of the file was not kept:\n%s", series)
	}
	if strings.Contains(series, "VTODO") || strings.Contains(series, "VFREEBUSY") {
		t.Errorf("another record travelled with this one:\n%s", series)
	}
	if UID(objs[1]) != "task-1" || objs[1].MainComponent() != "VTODO" {
		t.Errorf("second record = %s %s", objs[1].MainComponent(), UID(objs[1]))
	}
	// What Split made is what a server is offered: it has to encode.
	if err := Encode(&bytes.Buffer{}, objs[0]); err != nil {
		t.Errorf("the record does not encode: %v", err)
	}
}

// A file may hold several calendar objects one after another, and a component
// without UID or DTSTAMP — both required — cannot be written until it has
// them.
func TestSplitFillsWhatTheFileLeftOut(t *testing.T) {
	doc := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//A//A//EN\r\n" +
		"BEGIN:VJOURNAL\r\nSUMMARY:No identity\r\nEND:VJOURNAL\r\nEND:VCALENDAR\r\n" +
		"BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//B//B//EN\r\n" +
		"BEGIN:VJOURNAL\r\nUID:second\r\nDTSTAMP:20260101T000000Z\r\nSUMMARY:Second\r\nEND:VJOURNAL\r\nEND:VCALENDAR\r\n"
	objs, strays, err := Split([]byte(doc), importStamp)
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 2 || len(strays) != 0 {
		t.Fatalf("records = %d, strays = %+v", len(objs), strays)
	}
	if UID(objs[0]) == "" || len(UID(objs[0])) != 32 {
		t.Errorf("no UID was given: %q", UID(objs[0]))
	}
	if got := Text(objs[0], "DTSTAMP"); got != "20260915T120000Z" {
		t.Errorf("DTSTAMP = %q", got)
	}
	if err := Encode(&bytes.Buffer{}, objs[0]); err != nil {
		t.Errorf("the record does not encode: %v", err)
	}
}

// An occurrence whose series is not in the file is not a record of its own:
// it is named, so nobody has to guess where half a series went.
func TestSplitNamesAnOrphanOccurrence(t *testing.T) {
	doc := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//A//A//EN\r\n" +
		"BEGIN:VEVENT\r\nUID:gone\r\nRECURRENCE-ID:20260112T100000Z\r\nDTSTAMP:20260101T000000Z\r\n" +
		"SUMMARY:Alone\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	objs, strays, err := Split([]byte(doc), importStamp)
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 0 || len(strays) != 1 || strays[0].Summary != "Alone" {
		t.Fatalf("records = %d, strays = %+v", len(objs), strays)
	}
}

func TestSplitRefusesWhatIsNotACalendar(t *testing.T) {
	if _, _, err := Split([]byte("BEGIN:VCARD\r\nFN:Ada\r\nEND:VCARD\r\n"), importStamp); err == nil {
		t.Fatal("a vCard was taken for a calendar")
	}
}

// S-19: an import that meets the UID in the collection it writes to gives the
// record a new one — in every component, or the instance loses its series.
func TestSetUIDReachesTheInstanceToo(t *testing.T) {
	objs, _, err := Split([]byte(mixedFile), importStamp)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := SetUID(objs[0], "other")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(fresh.CalendarDoc())
	if UID(fresh) != "other" || strings.Count(doc, "UID:other") != 2 || strings.Contains(doc, "UID:series") {
		t.Fatalf("UID = %q in\n%s", UID(fresh), doc)
	}
	if !strings.Contains(doc, "RECURRENCE-ID") {
		t.Errorf("the instance was lost:\n%s", doc)
	}
}

// Components is what an export copies: the parts of a document without its
// envelope, as they came.
func TestComponentsAreTheDocumentWithoutItsEnvelope(t *testing.T) {
	got := Components([]byte(mixedFile))
	if len(got) != 6 {
		t.Fatalf("components = %d, want 6", len(got))
	}
	if got[0].Name != "VTIMEZONE" || got[0].TZID != "Europe/Moscow" {
		t.Errorf("first = %+v", got[0])
	}
	if !bytes.HasPrefix(got[2].Body, []byte("BEGIN:VEVENT\r\n")) || !bytes.HasSuffix(got[2].Body, []byte("END:VEVENT\r\n")) {
		t.Errorf("third = %q", got[2].Body)
	}
	for _, c := range got {
		if bytes.Contains(c.Body, []byte("VCALENDAR")) || bytes.Contains(c.Body, []byte("PRODID")) {
			t.Errorf("the envelope came along: %q", c.Body)
		}
	}
}
