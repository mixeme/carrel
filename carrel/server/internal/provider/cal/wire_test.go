package cal

import (
	"bytes"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/model"
)

// A record whose line go-ical cannot take apart is an error of that record:
// go-ical panics on these, and the caller was a fan-out goroutine (S-21).
func TestMalformedRecordIsAnErrorNotAPanic(t *testing.T) {
	for _, line := range []string{`X-FOO;A=b`, `X-FOO;A="x"y:v`} {
		doc := strings.ReplaceAll("BEGIN:VCALENDAR\nVERSION:2.0\nBEGIN:VEVENT\nUID:u1\n"+line+"\nEND:VEVENT\nEND:VCALENDAR\n", "\n", "\r\n")
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s: panic %v", line, r)
				}
			}()
			if _, err := Parse("/c/x.ics", "1", strings.NewReader(doc)); err == nil {
				t.Errorf("%s: read as a record", line)
			}
		}()
	}
}

// An edit of the summary leaves every other property of the event as it
// came, all values of a list parameter included: DELEGATED-TO kept only its
// first address, and X- parameters their first value (S-7).
func TestEventEditKeepsEveryParameterValue(t *testing.T) {
	doc := strings.ReplaceAll(`BEGIN:VCALENDAR
VERSION:2.0
PRODID:x
BEGIN:VEVENT
UID:u1
DTSTAMP:20260101T000000Z
DTSTART:20260101T100000Z
SUMMARY:old
ATTENDEE;CN="Doe, J";DELEGATED-TO="mailto:a@x","mailto:b@x":mailto:c@x
X-FOO;X-P=a,b:v
END:VEVENT
END:VCALENDAR
`, "\n", "\r\n")
	obj, err := Parse("/c/x.ics", "1", strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	patch, err := model.NewPatch(model.SetText{Prop: "SUMMARY", Value: "new"})
	if err != nil {
		t.Fatal(err)
	}
	if err := patch.Apply(obj); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Encode(&buf, obj); err != nil {
		t.Fatal(err)
	}
	back, err := Parse("/c/x.ics", "1", &buf)
	if err != nil {
		t.Fatal(err)
	}
	att := back.Props()["ATTENDEE"]
	if len(att) != 1 || att[0].Params["DELEGATED-TO"] != "mailto:a@x,mailto:b@x" || att[0].Params["CN"] != "Doe, J" {
		t.Errorf("ATTENDEE = %+v\n%s", att, buf.String())
	}
	if foo := back.Props()["X-FOO"]; len(foo) != 1 || foo[0].Params["X-P"] != "a,b" {
		t.Errorf("X-FOO = %+v", foo)
	}
	if got := back.PropText("SUMMARY"); len(got) != 1 || got[0] != "new" {
		t.Errorf("SUMMARY = %v", got)
	}
}
