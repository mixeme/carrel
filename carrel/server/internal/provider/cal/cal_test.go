package cal

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/view"
)

func TestParseEvent(t *testing.T) {
	data, err := os.ReadFile("testdata/event.ics")
	if err != nil {
		t.Fatal(err)
	}
	obj, err := Parse("/cal/e.ics", "e1", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if !obj.IsCalendar() || obj.MainComponent() != "VEVENT" {
		t.Fatalf("kind=%q", obj.MainComponent())
	}
	if Kind(obj) != view.KindEvent {
		t.Fatal("wrong kind")
	}
	if got := obj.PropText("SUMMARY"); len(got) != 1 || got[0] != "Team sync" {
		t.Fatalf("summary=%v", got)
	}
}

func TestTimezoneSurvivesEncode(t *testing.T) {
	data, _ := os.ReadFile("testdata/event.ics")
	obj, err := Parse("/cal/e.ics", "e1", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	patch, _ := model.NewPatch(model.SetText{Prop: "SUMMARY", Value: "Updated"})
	if err := patch.Apply(obj); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Encode(&buf, obj); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "VTIMEZONE") {
		t.Fatal("lost VTIMEZONE")
	}
	if !strings.Contains(got, "BEGIN:VALARM") {
		t.Fatal("lost VALARM")
	}
	if !strings.Contains(got, "Updated") {
		t.Fatal("summary not updated")
	}
}

func TestRecurrenceIDSiblingSurvivesEncode(t *testing.T) {
	data, err := os.ReadFile("testdata/series.ics")
	if err != nil {
		t.Fatal(err)
	}
	obj, err := Parse("/cal/s.ics", "e1", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if obj.MainComponent() != "VEVENT" || obj.PropText("SUMMARY")[0] != "Series" {
		t.Fatalf("main=%q summary=%v", obj.MainComponent(), obj.PropText("SUMMARY"))
	}
	patch, _ := model.NewPatch(model.SetText{Prop: "RRULE", Value: "FREQ=WEEKLY;COUNT=8"})
	if err := patch.Apply(obj); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := Encode(&buf, obj); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "RECURRENCE-ID") {
		t.Fatal("lost RECURRENCE-ID sibling")
	}
	if !strings.Contains(got, "Exception") {
		t.Fatal("lost exception summary")
	}
	if strings.Count(got, "BEGIN:VEVENT") != 2 {
		t.Fatalf("VEVENT count in\n%s", got)
	}
}

func TestParseTaskKeepsUnknownInRaw(t *testing.T) {
	data, _ := os.ReadFile("testdata/task.ics")
	obj, err := Parse("/cal/t.ics", "e1", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if Kind(obj) != view.KindTask {
		t.Fatal("not task")
	}
	if got := obj.PropText("X-CUSTOM"); len(got) != 1 || got[0] != "keep-me" {
		t.Fatalf("X-CUSTOM=%v", got)
	}
	var buf bytes.Buffer
	_ = Encode(&buf, obj)
	if !strings.Contains(buf.String(), "X-CUSTOM:keep-me") {
		t.Fatal("unknown property lost")
	}
}

func TestExpandWeekly(t *testing.T) {
	data, _ := os.ReadFile("testdata/event.ics")
	obj, _ := Parse("/cal/e.ics", "e1", bytes.NewReader(data))
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	times, err := Expand(obj, from, to, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if len(times) != 3 {
		t.Fatalf("got %d instances: %v", len(times), times)
	}
}

func TestRawRRULE(t *testing.T) {
	data, _ := os.ReadFile("testdata/event.ics")
	obj, _ := Parse("/cal/e.ics", "e1", bytes.NewReader(data))
	if !strings.Contains(RawRRULE(obj), "WEEKLY") {
		t.Fatalf("rrule=%q", RawRRULE(obj))
	}
}

func TestExpandExdateAndRdate(t *testing.T) {
	ics := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\n" +
		"UID:ex\r\nDTSTAMP:20260101T120000Z\r\nDTSTART:20260115T100000Z\r\n" +
		"RRULE:FREQ=WEEKLY;COUNT=3\r\nEXDATE:20260122T100000Z\r\n" +
		"RDATE:20260120T100000Z\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	obj, err := Parse("/cal/ex.ics", "e", strings.NewReader(ics))
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	times, err := Expand(obj, from, to, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	want := []time.Time{
		time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 20, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 29, 10, 0, 0, 0, time.UTC),
	}
	if len(times) != len(want) {
		t.Fatalf("got %v", times)
	}
	for i := range want {
		if !times[i].Equal(want[i]) {
			t.Fatalf("times = %v", times)
		}
	}
}

func TestExpandRdateWithoutRRule(t *testing.T) {
	ics := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\n" +
		"UID:rd\r\nDTSTAMP:20260101T120000Z\r\nDTSTART:20260110T100000Z\r\n" +
		"RDATE:20260120T100000Z\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	obj, err := Parse("/cal/rd.ics", "e", strings.NewReader(ics))
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	times, err := Expand(obj, from, to, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, tm := range times {
		if tm.Equal(time.Date(2026, 1, 20, 10, 0, 0, 0, time.UTC)) {
			found = true
		}
	}
	if !found {
		t.Fatalf("RDATE missing: %v", times)
	}
}

func TestRenderMarkdownEscapesHTML(t *testing.T) {
	blocks := RenderMarkdown("<script>x</script>\n\nHello **world**.")
	if len(blocks) == 0 {
		t.Fatal("no blocks")
	}
	found := false
	for _, b := range blocks {
		if strings.Contains(string(b.Text), "<script>") {
			t.Fatalf("raw html in block: %q", b.Text)
		}
		if strings.Contains(string(b.Text), "&lt;script&gt;") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected escaped script")
	}
}

func TestRenderMarkdownInlineKinds(t *testing.T) {
	blocks := RenderMarkdown("Hello **world** and *x* and [a](https://example.com).")
	if len(blocks) != 1 || blocks[0].Kind != view.BlockPara {
		t.Fatalf("blocks = %+v", blocks)
	}
	var kinds []view.BlockKind
	var href string
	for _, c := range blocks[0].Children {
		kinds = append(kinds, c.Kind)
		if c.Kind == view.BlockLink {
			href = c.Href
		}
		if c.Kind == view.BlockStrong && string(c.Text) != "world" {
			t.Fatalf("strong = %+v", c)
		}
		if c.Kind == view.BlockEmph && string(c.Text) != "x" {
			t.Fatalf("emph = %+v", c)
		}
	}
	if !containsKind(kinds, view.BlockStrong) || !containsKind(kinds, view.BlockEmph) || !containsKind(kinds, view.BlockLink) {
		t.Fatalf("kinds = %v", kinds)
	}
	if href != "https://example.com" {
		t.Fatalf("href = %q", href)
	}
}

func TestRenderMarkdownCodeFence(t *testing.T) {
	blocks := RenderMarkdown("```\nfmt.Println(1)\n```")
	if len(blocks) != 1 || blocks[0].Kind != view.BlockCode {
		t.Fatalf("blocks = %+v", blocks)
	}
	if !strings.Contains(string(blocks[0].Text), "fmt.Println") {
		t.Fatalf("code = %q", blocks[0].Text)
	}
}

func TestNoteDescriptionIsMarkdown(t *testing.T) {
	data, err := os.ReadFile("testdata/note.ics")
	if err != nil {
		t.Fatal(err)
	}
	obj, err := Parse("/cal/n.ics", "e", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	body := Description(obj)
	if !strings.Contains(body, "**world**") || strings.Contains(body, "<p>") {
		t.Fatalf("description = %q", body)
	}
	blocks := RenderMarkdown(body)
	if len(blocks) == 0 || blocks[0].Kind != view.BlockHeading {
		t.Fatalf("blocks = %+v", blocks)
	}
}

func containsKind(kinds []view.BlockKind, want view.BlockKind) bool {
	for _, k := range kinds {
		if k == want {
			return true
		}
	}
	return false
}

func TestUpdateRejectsEmptyETag(t *testing.T) {
	p := &Provider{}
	if _, err := p.Update(t.Context(), model.NewCalendarObject("/c/x.ics", "", "VEVENT", nil, nil)); err != ErrEmptyETag {
		t.Fatalf("got %v", err)
	}
}
