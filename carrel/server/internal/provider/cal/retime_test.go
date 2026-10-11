package cal

import (
	"bytes"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/model"
)

// A series of Mondays at ten in Moscow with a day left out and a day moved
// to noon: what liveEventFields seeds every server with.
const retimeSeries = `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//T//EN
BEGIN:VEVENT
UID:duty
DTSTAMP:20260101T000000Z
DTSTART;TZID=Europe/Moscow:20260112T100000
DTEND;TZID=Europe/Moscow:20260112T110000
SUMMARY:Duty
RRULE:FREQ=WEEKLY;BYDAY=MO;COUNT=6
EXDATE;TZID=Europe/Moscow:20260119T100000
END:VEVENT
BEGIN:VEVENT
UID:duty
DTSTAMP:20260101T000000Z
RECURRENCE-ID;TZID=Europe/Moscow:20260126T100000
DTSTART;TZID=Europe/Moscow:20260126T120000
DTEND;TZID=Europe/Moscow:20260126T130000
SUMMARY:Duty moved
END:VEVENT
END:VCALENDAR
`

// retimedDoc gives the series in doc another start and returns the document
// RetimeExceptions makes of it, and how many exceptions it moved.
func retimedDoc(t *testing.T, doc string, ops ...model.PatchOp) (string, int) {
	t.Helper()
	obj := parseICS(t, doc)
	was := obj.Props()["DTSTART"][0]
	patch, err := model.NewPatch(ops...)
	if err != nil {
		t.Fatal(err)
	}
	if err := patch.Apply(obj); err != nil {
		t.Fatal(err)
	}
	out, n, err := RetimeExceptions(obj, was)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Encode(&buf, out); err != nil {
		t.Fatal(err)
	}
	return unfold(buf.String()), n
}

func wantLines(t *testing.T, doc string, lines ...string) {
	t.Helper()
	for _, l := range lines {
		if !strings.Contains(doc, l+"\r\n") && !strings.Contains(doc, l+"\n") {
			t.Errorf("no line %q in\n%s", l, doc)
		}
	}
}

// The owner's decision of 11.10.2026: a series given another time or another
// zone takes its exceptions with it. A day left out stays left out and a day
// moved stays moved — the person changed when the meeting is, not what they
// had done to two of its days. Before, the series went to ten in
// Yekaterinburg and the exceptions stayed at ten in Moscow, where no
// occurrence was any more: the day came back and the moved one hung loose.
func TestRetimeExceptionsFollowTheSeriesToAnotherZone(t *testing.T) {
	doc, n := retimedDoc(t, retimeSeries,
		model.SetText{Prop: "DTSTART", Value: "20260112T100000"},
		model.SetParam{Prop: "DTSTART", Index: 0, Param: "TZID", Value: "Asia/Yekaterinburg"})
	if n != 2 {
		t.Errorf("moved %d exceptions, want the day left out and the day moved", n)
	}
	wantLines(t, doc,
		"EXDATE;TZID=Asia/Yekaterinburg:20260119T100000",
		"RECURRENCE-ID;TZID=Asia/Yekaterinburg:20260126T100000",
		// The time the day was moved to is the person's own: it stays.
		"DTSTART;TZID=Europe/Moscow:20260126T120000")
}

func TestRetimeExceptionsFollowTheSeriesToAnotherHour(t *testing.T) {
	doc, n := retimedDoc(t, retimeSeries,
		model.SetText{Prop: "DTSTART", Value: "20260112T113000"},
		model.SetParam{Prop: "DTSTART", Index: 0, Param: "TZID", Value: "Europe/Moscow"})
	if n != 2 {
		t.Errorf("moved %d exceptions, want 2", n)
	}
	wantLines(t, doc,
		"EXDATE;TZID=Europe/Moscow:20260119T113000",
		"RECURRENCE-ID;TZID=Europe/Moscow:20260126T113000",
		"DTSTART;TZID=Europe/Moscow:20260126T120000")
}

// Mail.ru keeps the exceptions in UTC beside a series in a zone. They name
// the same occurrences, and go with the series all the same — written the
// way the series' start now is.
func TestRetimeExceptionsWrittenInUTC(t *testing.T) {
	doc := strings.NewReplacer(
		"EXDATE;TZID=Europe/Moscow:20260119T100000", "EXDATE:20260119T070000Z",
		"RECURRENCE-ID;TZID=Europe/Moscow:20260126T100000", "RECURRENCE-ID:20260126T070000Z").Replace(retimeSeries)
	got, n := retimedDoc(t, doc,
		model.SetText{Prop: "DTSTART", Value: "20260112T100000"},
		model.SetParam{Prop: "DTSTART", Index: 0, Param: "TZID", Value: "Asia/Yekaterinburg"})
	if n != 2 {
		t.Errorf("moved %d exceptions, want 2", n)
	}
	wantLines(t, got,
		"EXDATE;TZID=Asia/Yekaterinburg:20260119T100000",
		"RECURRENCE-ID;TZID=Asia/Yekaterinburg:20260126T100000")
}

// Whole days, and several days left out on one line.
func TestRetimeExceptionsOfWholeDays(t *testing.T) {
	doc := `BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//T//EN
BEGIN:VEVENT
UID:days
DTSTAMP:20260101T000000Z
DTSTART;VALUE=DATE:20260112
SUMMARY:Days
RRULE:FREQ=WEEKLY;COUNT=6
EXDATE;VALUE=DATE:20260119,20260202
END:VEVENT
END:VCALENDAR
`
	got, n := retimedDoc(t, doc,
		model.SetText{Prop: "DTSTART", Value: "20260113"},
		model.SetParam{Prop: "DTSTART", Index: 0, Param: "VALUE", Value: "DATE"})
	if n != 2 {
		t.Errorf("moved %d exceptions, want the two days of the line", n)
	}
	wantLines(t, got, "EXDATE;VALUE=DATE:20260120,20260203")
}

// A start that did not change moves nothing, and a record without
// exceptions is handed back as it is.
func TestRetimeExceptionsLeavesAloneWhatDidNotMove(t *testing.T) {
	obj := parseICS(t, retimeSeries)
	out, n, err := RetimeExceptions(obj, obj.Props()["DTSTART"][0])
	if err != nil || n != 0 || out != obj {
		t.Errorf("a start that did not change: %d moved, the same object %v, %v", n, out == obj, err)
	}
	if _, n := retimedDoc(t, rewriteSeries, model.SetText{Prop: "SUMMARY", Value: "Other"}); n != 0 {
		t.Errorf("a change of the title moved %d exceptions", n)
	}
}

// What a server did to the exceptions is read by the instant they name, not
// by how it wrote them: UTC for a zone is the same occurrence, a moved day
// that is gone is not.
func TestMissingExceptions(t *testing.T) {
	sent := parseICS(t, retimeSeries)
	utc := parseICS(t, strings.NewReplacer(
		"EXDATE;TZID=Europe/Moscow:20260119T100000", "EXDATE:20260119T070000Z",
		"RECURRENCE-ID;TZID=Europe/Moscow:20260126T100000", "RECURRENCE-ID:20260126T070000Z").Replace(retimeSeries))
	if days, moved := MissingExceptions(sent, utc); days != 0 || moved != 0 {
		t.Errorf("the same exceptions written in UTC: %d days and %d moved are missing", days, moved)
	}
	bare := parseICS(t, retimeSeries[:strings.Index(retimeSeries, "EXDATE")]+"END:VEVENT\nEND:VCALENDAR\n")
	if days, moved := MissingExceptions(sent, bare); days != 1 || moved != 1 {
		t.Errorf("a server that kept the series alone: %d days and %d moved are missing, want 1 and 1", days, moved)
	}
	loose := parseICS(t, strings.Replace(retimeSeries, "RECURRENCE-ID;TZID=Europe/Moscow:20260126T100000\n", "", 1))
	if _, moved := MissingExceptions(sent, loose); moved != 1 {
		t.Errorf("a moved day kept without what ties it to the series: %d missing, want 1", moved)
	}
}

// The report of what a server did not keep names the exceptions by what
// became of them, not by how they are written.
func TestExceptionLoss(t *testing.T) {
	sent := parseICS(t, retimeSeries)
	utc := parseICS(t, strings.NewReplacer(
		"EXDATE;TZID=Europe/Moscow:20260119T100000", "EXDATE:20260119T070000Z",
		"RECURRENCE-ID;TZID=Europe/Moscow:20260126T100000", "RECURRENCE-ID:20260126T070000Z").Replace(retimeSeries))
	if got := ExceptionLoss([]string{"DTSTART", "EXDATE"}, sent, utc); strings.Join(got, ",") != "DTSTART" {
		t.Errorf("the days kept in UTC are reported lost: %v", got)
	}
	bare := parseICS(t, retimeSeries[:strings.Index(retimeSeries, "EXDATE")]+"END:VEVENT\nEND:VCALENDAR\n")
	if got := ExceptionLoss([]string{"EXDATE"}, sent, bare); strings.Join(got, ",") != "EXDATE,RECURRENCE-ID" {
		t.Errorf("a server that kept the series alone lost %v, want EXDATE and RECURRENCE-ID", got)
	}
	if got := ExceptionLoss(nil, sent, sent); len(got) != 0 {
		t.Errorf("nothing lost is reported as %v", got)
	}
}
