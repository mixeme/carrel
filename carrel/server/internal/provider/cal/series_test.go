package cal

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/model"
)

// А1 (the owner's decision of 11.10.2026; s-agenda-22): the series of
// retimeSeries cut to two Mondays — 12 and 19 January, the day left out
// counted — leaves the meeting moved on 26 January without its day, and the
// day left out is still a day of the series. Cut to one, the day left out has
// no day either and goes without a word; the moved meeting goes when it is
// asked to, or becomes an event of its own. Three Mondays still have it.
func TestLeftWithoutDay(t *testing.T) {
	msk, _ := time.LoadLocation("Europe/Moscow")
	cut := func(count string) *model.Object {
		t.Helper()
		obj := parseICS(t, retimeSeries)
		patch, err := model.NewPatch(model.SetText{Prop: "RRULE", Value: "FREQ=WEEKLY;BYDAY=MO;COUNT=" + count})
		if err != nil {
			t.Fatal(err)
		}
		if err := patch.Apply(obj); err != nil {
			t.Fatal(err)
		}
		return obj
	}
	if left, err := LeftWithoutDay(parseICS(t, retimeSeries), msk); err != nil || len(left) != 0 {
		t.Fatalf("the series as it is: %+v, %v", left, err)
	}
	two := cut("2")
	left, err := LeftWithoutDay(two, msk)
	if err != nil || len(left) != 1 {
		t.Fatalf("left = %+v, %v", left, err)
	}
	day := time.Date(2026, 1, 26, 10, 0, 0, 0, msk)
	if !left[0].At.Equal(day) || !left[0].Moved || !left[0].Retitled || left[0].Summary != "Duty moved" || left[0].AllDay {
		t.Errorf("left[0] = %+v", left[0])
	}
	if l, _ := LeftWithoutDay(cut("3"), msk); len(l) != 0 {
		t.Errorf("three Mondays: %+v", l)
	}

	apart, ok, err := Apart(two, day, "duty-26", msk)
	if err != nil || !ok {
		t.Fatalf("apart: %v %v", ok, err)
	}
	var buf bytes.Buffer
	if err := Encode(&buf, apart); err != nil {
		t.Fatal(err)
	}
	doc := unfold(buf.String())
	wantLines(t, doc, "UID:duty-26", "DTSTART;TZID=Europe/Moscow:20260126T120000", "SUMMARY:Duty moved")
	if strings.Contains(doc, "RECURRENCE-ID") || strings.Contains(doc, "RRULE") {
		t.Errorf("the event apart is still of the series:\n%s", doc)
	}

	dropped, err := DropLeft(cut("1"), []time.Time{day}, msk)
	if err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if err := Encode(&buf, dropped); err != nil {
		t.Fatal(err)
	}
	doc = unfold(buf.String())
	if strings.Contains(doc, "RECURRENCE-ID") || strings.Contains(doc, "EXDATE") || !strings.Contains(doc, "COUNT=1") {
		t.Errorf("after DropLeft:\n%s", doc)
	}
	// Two Mondays keep their day left out on 19 January.
	three, err := DropLeft(two, []time.Time{day}, msk)
	if err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	_ = Encode(&buf, three)
	wantLines(t, unfold(buf.String()), "EXDATE;TZID=Europe/Moscow:20260119T100000")
}
