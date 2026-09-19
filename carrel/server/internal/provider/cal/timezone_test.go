package cal

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-ical"
)

func encodedTZ(t *testing.T, name string, year int) string {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Props.SetText(ical.PropProductID, ProdID)
	cal.Children = append(cal.Children, Timezone(loc, year))
	var buf bytes.Buffer
	if err := ical.NewEncoder(&buf).Encode(cal); err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(buf.String(), "\r\n", "\n")
}

// A zone with summer time has two observances with their yearly rules; a
// zone without has one; both carry the TZID a record names.
func TestTimezoneFromTheZoneDatabase(t *testing.T) {
	london := encodedTZ(t, "Europe/London", 2026)
	for _, want := range []string{
		"TZID:Europe/London",
		"BEGIN:DAYLIGHT\nDTSTART:20260329T010000\nRRULE:FREQ=YEARLY;BYMONTH=3;BYDAY=-1SU\nTZNAME:BST\nTZOFFSETFROM:+0000\nTZOFFSETTO:+0100\nEND:DAYLIGHT",
		"BEGIN:STANDARD\nDTSTART:20261025T020000\nRRULE:FREQ=YEARLY;BYMONTH=10;BYDAY=-1SU\nTZNAME:GMT\nTZOFFSETFROM:+0100\nTZOFFSETTO:+0000\nEND:STANDARD",
	} {
		if !strings.Contains(london, want) {
			t.Errorf("Europe/London lacks\n%s\nin\n%s", want, london)
		}
	}
	moscow := encodedTZ(t, "Europe/Moscow", 2026)
	if !strings.Contains(moscow, "BEGIN:STANDARD\nDTSTART:19700101T000000\nTZNAME:MSK\nTZOFFSETFROM:+0300\nTZOFFSETTO:+0300\nEND:STANDARD") ||
		strings.Contains(moscow, "DAYLIGHT") {
		t.Errorf("Europe/Moscow:\n%s", moscow)
	}
	if got := utcOffset(-(4*3600 + 30*60)); got != "-0430" {
		t.Errorf("offset = %s", got)
	}
}
