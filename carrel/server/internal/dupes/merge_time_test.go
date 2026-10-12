package dupes

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
)

func parseDoc(t *testing.T, path, doc string) *model.Object {
	t.Helper()
	obj, err := cal.Parse(path, `"1"`, strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	return obj
}

func encoded(t *testing.T, obj *model.Object) string {
	t.Helper()
	var buf bytes.Buffer
	if err := cal.Encode(&buf, obj); err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(buf.String(), "\r\n", "\n")
}

// А6 of the audit of 11.10.2026 (the owner's decision of 12.10.2026: «the
// person decides what to take from where»): members whose time differs make
// a question, «When», answered like any other. Without an answer the kept
// record's time stays. The chosen time is taken whole — the start with its
// zone and the zone's definition, the end, the rule, the days left out and
// the occurrences rewritten by hand, which then belong to the kept record;
// the kept record's own rewritten occurrences go with the time it gives up.
func TestMergeTakesTheChosenTime(t *testing.T) {
	kept := parseDoc(t, "/c/k.ics", "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//t//EN\r\n"+
		"BEGIN:VEVENT\r\nUID:k\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:Standup\r\nDTSTART:20260810T070000Z\r\nDTEND:20260810T080000Z\r\n"+
		"RRULE:FREQ=WEEKLY;BYDAY=MO\r\nEXDATE:20260817T070000Z\r\nX-KEPT:mine\r\nEND:VEVENT\r\n"+
		"BEGIN:VEVENT\r\nUID:k\r\nDTSTAMP:20260801T000000Z\r\nRECURRENCE-ID:20260824T070000Z\r\nDTSTART:20260824T090000Z\r\nSUMMARY:Standup, late\r\nEND:VEVENT\r\n"+
		"END:VCALENDAR\r\n")
	other := parseDoc(t, "/c/o.ics", "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//t//EN\r\n"+
		"BEGIN:VTIMEZONE\r\nTZID:Office\r\nBEGIN:STANDARD\r\nDTSTART:19700101T000000\r\nTZOFFSETFROM:+0300\r\nTZOFFSETTO:+0300\r\nEND:STANDARD\r\nEND:VTIMEZONE\r\n"+
		"BEGIN:VEVENT\r\nUID:o\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:Standup\r\nDTSTART;TZID=Office:20260811T140000\r\nDURATION:PT30M\r\n"+
		"RRULE:FREQ=WEEKLY;BYDAY=TU\r\nLOCATION:Room 4\r\nEND:VEVENT\r\n"+
		"BEGIN:VEVENT\r\nUID:o\r\nDTSTAMP:20260801T000000Z\r\nRECURRENCE-ID;TZID=Office:20260818T140000\r\nDTSTART;TZID=Office:20260819T140000\r\nSUMMARY:Standup, on Wednesday\r\nEND:VEVENT\r\n"+
		"END:VCALENDAR\r\n")

	merged, plan, err := Merge(kept, []*model.Object{other}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var when *Question
	for i := range plan.Questions {
		if plan.Questions[i].Prop == When {
			when = &plan.Questions[i]
		}
	}
	if when == nil || len(when.Options) != 2 || when.Options[0].From != 0 || when.Options[1].From != 1 {
		t.Fatalf("no question about the time: %+v", plan.Questions)
	}
	if got := merged.PropText("DTSTART"); len(got) != 1 || got[0] != "20260810T070000Z" || plan.Time != 0 {
		t.Errorf("without an answer the time is not the kept record's: %v, plan.Time %d", got, plan.Time)
	}
	if plan.Instances[1] != 1 || plan.Dropped != 0 {
		t.Errorf("without an answer: instances %v, dropped %d", plan.Instances, plan.Dropped)
	}

	merged, plan, err = Merge(kept, []*model.Object{other}, map[string]int{When: 1})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Time != 1 || plan.Dropped != 1 || plan.Instances[1] != 0 {
		t.Errorf("plan = time %d, dropped %d, instances %v; want the other's time, the kept record's occurrence dropped, the other's carried", plan.Time, plan.Dropped, plan.Instances)
	}
	doc := encoded(t, merged)
	for _, want := range []string{
		"DTSTART;TZID=Office:20260811T140000", "DURATION:PT30M", "RRULE:FREQ=WEEKLY;BYDAY=TU",
		"BEGIN:VTIMEZONE", "TZID:Office",
		"RECURRENCE-ID;TZID=Office:20260818T140000", "SUMMARY:Standup, on Wednesday",
		"X-KEPT:mine", "LOCATION:Room 4",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("the merged record lacks %q:\n%s", want, doc)
		}
	}
	for _, gone := range []string{"DTEND:", "EXDATE:", "BYDAY=MO", "Standup, late", "UID:o"} {
		if strings.Contains(doc, gone) {
			t.Errorf("the merged record still holds %q:\n%s", gone, doc)
		}
	}
	if n := strings.Count(doc, "UID:k"); n != 2 || cal.UID(merged) != "k" {
		t.Errorf("the series and its occurrence are not the kept record's: %d × UID:k\n%s", n, doc)
	}
	// What was read back is one record with one rewritten occurrence.
	again := parseDoc(t, "/c/k.ics", strings.ReplaceAll(doc, "\n", "\r\n"))
	if over, err := cal.Overrides(again, time.UTC); err != nil || len(over) != 1 {
		t.Errorf("read back: %d rewritten occurrences, %v", len(over), err)
	}
}

// A task's time is its due date and its start; the same question.
func TestMergeTakesTheChosenDue(t *testing.T) {
	kept := ics(t, "VTODO", "UID:k", "DTSTAMP:20260801T000000Z", "SUMMARY:Report", "DUE;VALUE=DATE:20260810")
	other := ics(t, "VTODO", "UID:o", "DTSTAMP:20260801T000000Z", "SUMMARY:Report", "DUE:20260810T150000Z", "PRIORITY:1")
	if _, plan, err := Merge(kept, []*model.Object{other}, nil); err != nil || len(plan.Questions) != 1 || plan.Questions[0].Prop != When {
		t.Fatalf("questions = %+v, %v", plan.Questions, err)
	}
	merged, _, err := Merge(kept, []*model.Object{other}, map[string]int{When: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got := merged.Props()["DUE"]; len(got) != 1 || got[0].Text != "20260810T150000Z" || got[0].Params["VALUE"] != "" {
		t.Errorf("DUE = %+v, want the other's instant", got)
	}
	if merged.PropText("PRIORITY")[0] != "1" || cal.UID(merged) != "k" {
		t.Errorf("merged = %v", merged.Props())
	}
}

// The same time written the same way is no question.
func TestMergeAsksNothingAboutTheSameTime(t *testing.T) {
	kept := ics(t, "VEVENT", "UID:k", "DTSTAMP:20260801T000000Z", "SUMMARY:Standup", "DTSTART:20260810T070000Z")
	other := ics(t, "VEVENT", "UID:o", "DTSTAMP:20260802T000000Z", "SUMMARY:Standup", "DTSTART:20260810T070000Z", "LOCATION:Room 4")
	_, plan, err := Merge(kept, []*model.Object{other}, nil)
	if err != nil || len(plan.Questions) != 0 {
		t.Errorf("questions = %+v, %v", plan.Questions, err)
	}
}
