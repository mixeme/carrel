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

// from-typecase note 9, s-notes-1 and s-notes-2: a task item of a note is a
// task with its tick, not an item whose text begins with «[ ] ». Tight and
// loose lists alike; an item that is no task stays an item.
func TestRenderMarkdownTaskItems(t *testing.T) {
	for _, src := range []string{
		"- [ ] check the changelog\n- [x] ask for screenshots\n- plain",
		"- [ ] check the changelog\n\n- [x] ask for screenshots\n\n- plain",
	} {
		blocks := RenderMarkdown(src)
		if len(blocks) != 1 || blocks[0].Kind != view.BlockList || len(blocks[0].Children) != 3 {
			t.Fatalf("%q: blocks = %+v", src, blocks)
		}
		items := blocks[0].Children
		if items[0].Kind != view.BlockTask || items[0].Checked || string(items[0].Text) != "check the changelog" {
			t.Errorf("%q: first = %+v", src, items[0])
		}
		if items[1].Kind != view.BlockTask || !items[1].Checked || string(items[1].Text) != "ask for screenshots" {
			t.Errorf("%q: second = %+v", src, items[1])
		}
		if items[2].Kind != view.BlockItem || items[2].Checked || string(items[2].Text) != "plain" {
			t.Errorf("%q: third = %+v", src, items[2])
		}
	}
}

// s-notes-2 promises tables among what a note's Markdown shows: a GFM table
// is a table of rows, the first the header, its cells with their inline
// marks and their alignment; raw HTML in a cell stays escaped text.
func TestRenderMarkdownTables(t *testing.T) {
	blocks := RenderMarkdown("| Item | Cost |\n|:--|--:|\n| **Rent** | 900 |\n| [Lamp](https://example.com) <b>x</b> | |")
	if len(blocks) != 1 || blocks[0].Kind != view.BlockTable || len(blocks[0].Children) != 3 {
		t.Fatalf("blocks = %+v", blocks)
	}
	rows := blocks[0].Children
	for i, r := range rows {
		if r.Kind != view.BlockRow || r.Header != (i == 0) || len(r.Children) != 2 {
			t.Fatalf("row %d = %+v", i, r)
		}
		for j, c := range r.Children {
			want := []view.CellAlign{view.AlignLeft, view.AlignRight}[j]
			if c.Kind != view.BlockCell || c.Align != want {
				t.Errorf("row %d cell %d = %+v, want a cell aligned %q", i, j, c, want)
			}
		}
	}
	if head := rows[0].Children; string(head[0].Text) != "Item" || string(head[1].Text) != "Cost" {
		t.Errorf("header = %+v", head)
	}
	if rent := rows[1].Children[0]; len(rent.Children) != 1 || rent.Children[0].Kind != view.BlockStrong || string(rent.Children[0].Text) != "Rent" {
		t.Errorf("a strong cell = %+v", rent)
	}
	if cost := rows[1].Children[1]; string(cost.Text) != "900" || len(cost.Children) != 0 {
		t.Errorf("a plain cell = %+v", cost)
	}
	lamp := rows[2].Children[0]
	if len(lamp.Children) < 2 || lamp.Children[0].Kind != view.BlockLink || lamp.Children[0].Href != "https://example.com" {
		t.Fatalf("a cell with a link = %+v", lamp)
	}
	for _, k := range lamp.Children {
		if strings.Contains(string(k.Text), "<b>") {
			t.Errorf("raw html in a cell: %+v", k)
		}
	}
	if empty := rows[2].Children[1]; empty.Text != "" || len(empty.Children) != 0 {
		t.Errorf("an empty cell = %+v", empty)
	}
	// Not a table without its delimiter row: a paragraph with the pipes.
	if b := RenderMarkdown("a | b"); len(b) != 1 || b[0].Kind != view.BlockPara || string(b[0].Text) != "a | b" {
		t.Errorf("a line with pipes = %+v", b)
	}
}

// A backslash escape is Markdown's way to write a mark as itself: «\*»
// shows «*», and «\|» in a table cell a pipe, in every kind of block. Code,
// fenced or inline, keeps its backslashes as written; raw HTML stays
// escaped text.
func TestRenderMarkdownBackslashEscapes(t *testing.T) {
	var texts []string
	var collect func(bs []view.Block)
	collect = func(bs []view.Block) {
		for _, b := range bs {
			if b.Text != "" {
				texts = append(texts, string(b.Kind)+":"+string(b.Text))
			}
			collect(b.Children)
		}
	}
	for src, want := range map[string]string{
		"a \\* b":                        "para:a * b",
		"# a \\# b":                      "heading:a # b",
		"- a \\_ b":                      "item:a _ b",
		"- [ ] a \\[ b":                  "task:a [ b",
		"> a \\> b":                      "para:a > b",
		"**a \\* b**":                    "strong:a * b",
		"*a \\_ b*":                      "emph:a _ b",
		"[a \\] b](https://example.com)": "link:a ] b",
		"| h |\n|---|\n| a \\| b |":      "cell:a | b",
		"`a \\* b`":                      "code:a \\* b",
		"```\na \\* b\n```":              "code:a \\* b\n",
		"x <b a=\"\\*\">":                "para:&lt;b a=&#34;\\*&#34;&gt;",
	} {
		texts = nil
		collect(RenderMarkdown(src))
		found := false
		for _, s := range texts {
			if s == want {
				found = true
			}
		}
		if !found {
			t.Errorf("%q: texts %q, want %q among them", src, texts, want)
		}
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
