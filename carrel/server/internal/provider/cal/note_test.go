package cal

import (
	"bytes"
	"strings"
	"testing"
)

// noteRecord is a note as another client left it: a title, a date in a zone,
// categories, properties Carrel has no field for, and a body with a line
// break and an escaped comma.
const noteRecord = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Other//Other//EN\r\n" +
	"BEGIN:VJOURNAL\r\nUID:note-1\r\nDTSTAMP:20260101T000000Z\r\n" +
	"DTSTART;TZID=Europe/Moscow:20260105T100000\r\nSUMMARY:Ковка\r\n" +
	"CATEGORIES:work,travel\r\nSTATUS:FINAL\r\nX-JTX-BOARD:2\r\n" +
	"DESCRIPTION:Первая строка\\nВторая\\, с запятой\r\n" +
	"END:VJOURNAL\r\nEND:VCALENDAR\r\n"

// S-19: the header block names four fields, everything else keeps its own
// name, and the body is the description unescaped. What has no field of its
// own is in the file all the same.
func TestNoteMarkdownLosesNothing(t *testing.T) {
	obj, err := Parse("/n/note.ics", "e1", strings.NewReader(noteRecord))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := NoteMarkdown(&buf, obj); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	for _, want := range []string{
		"---\ntitle: Ковка\n",
		"date;TZID=Europe/Moscow: 20260105T100000\n",
		"categories: work,travel\n",
		"uid: note-1\n",
		"status: FINAL\n",
		"x-jtx-board: 2\n",
		"\n---\n\nПервая строка\nВторая, с запятой\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q in\n%s", want, got)
		}
	}
	if strings.Count(got, "uid") != 1 {
		t.Errorf("uid written twice:\n%s", got)
	}
}

// The file reads back as the record it was written from: an export followed
// by an import of the same file changes nothing but the stamps a write puts
// on it.
func TestNoteMarkdownReadsBack(t *testing.T) {
	obj, err := Parse("/n/note.ics", "e1", strings.NewReader(noteRecord))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := NoteMarkdown(&buf, obj); err != nil {
		t.Fatal(err)
	}
	back, err := ParseNote("Ковка.md", buf.Bytes(), importStamp)
	if err != nil {
		t.Fatal(err)
	}
	if back.MainComponent() != "VJOURNAL" {
		t.Fatalf("component = %q", back.MainComponent())
	}
	for _, prop := range []string{"UID", "SUMMARY", "CATEGORIES", "STATUS", "X-JTX-BOARD", "DESCRIPTION", "DTSTART"} {
		if a, b := obj.PropText(prop), back.PropText(prop); len(a) != len(b) || (len(a) == 1 && a[0] != b[0]) {
			t.Errorf("%s: %q became %q", prop, a, b)
		}
	}
	if got := back.Props()["DTSTART"][0].Params["TZID"]; got != "Europe/Moscow" {
		t.Errorf("the zone was lost: %q", got)
	}
	if err := Encode(&bytes.Buffer{}, back); err != nil {
		t.Errorf("the note does not encode: %v", err)
	}
}

// A file written by hand has no block at all: it is a note whose body is the
// whole file, and the list takes the title from its first line.
func TestParseNoteWithoutAHeaderBlock(t *testing.T) {
	back, err := ParseNote("Уголь.md", []byte("# Уголь\n\nЗаказать мешок.\n"), importStamp)
	if err != nil {
		t.Fatal(err)
	}
	if got := Description(back); got != "# Уголь\n\nЗаказать мешок." {
		t.Errorf("body = %q", got)
	}
	if got := Text(back, "SUMMARY"); got != "" {
		t.Errorf("a title was invented: %q", got)
	}
	if len(UID(back)) != 32 || Text(back, "DTSTAMP") != "20260915T120000Z" {
		t.Errorf("uid = %q dtstamp = %q", UID(back), Text(back, "DTSTAMP"))
	}
}

// An empty file is a note all the same, and the only name it has is its own.
func TestParseNoteOfAnEmptyFile(t *testing.T) {
	back, err := ParseNote("Пустая.md", nil, importStamp)
	if err != nil {
		t.Fatal(err)
	}
	if got := Text(back, "SUMMARY"); got != "Пустая" {
		t.Errorf("title = %q", got)
	}
	if err := Encode(&bytes.Buffer{}, back); err != nil {
		t.Errorf("the note does not encode: %v", err)
	}
}

// A date written by hand in the header block — as Markdown tools write it —
// is the note's date (s-io-8: «title, date and tags from front matter»); a
// date as the wire writes it stays as it is, and words that are no date are
// kept as written.
func TestParseNoteReadsADateWrittenByHand(t *testing.T) {
	for _, c := range []struct{ date, want, param string }{
		{"2026-03-11", "20260311", "DATE"},
		{"2026-03-11 10:15", "20260311T101500", ""},
		{"2026-03-11T10:15:30Z", "20260311T101530Z", ""},
		{"20260311T101500Z", "20260311T101500Z", ""},
		{"in March", "in March", ""},
	} {
		back, err := ParseNote("n.md", []byte("---\ntitle: Встреча\ndate: "+c.date+"\n---\n\nТекст\n"), importStamp)
		if err != nil {
			t.Fatal(err)
		}
		v := back.Props()["DTSTART"]
		if len(v) != 1 || v[0].Text != c.want || v[0].Params["VALUE"] != c.param {
			t.Errorf("%q: DTSTART = %+v, want %q VALUE=%q", c.date, v, c.want, c.param)
		}
	}
}
