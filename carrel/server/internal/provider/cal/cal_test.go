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
	if !strings.Contains(buf.String(), "VTIMEZONE") {
		t.Fatal("lost VTIMEZONE")
	}
	if !strings.Contains(buf.String(), "Updated") {
		t.Fatal("summary not updated")
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

func TestUpdateRejectsEmptyETag(t *testing.T) {
	p := &Provider{}
	if _, err := p.Update(t.Context(), model.NewCalendarObject("/c/x.ics", "", "VEVENT", nil, nil)); err != ErrEmptyETag {
		t.Fatalf("got %v", err)
	}
}
