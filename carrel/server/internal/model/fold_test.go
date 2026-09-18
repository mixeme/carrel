package model

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// docs/servers.md, SV-31: neither library folds, so Carrel does. Every line of
// a written record is at most 75 octets, a fold is CRLF and one space, and
// unfolding gives back exactly what went in.
func TestFoldLines(t *testing.T) {
	long := strings.Repeat("A", 300)
	doc := "BEGIN:VCARD\r\nVERSION:4.0\r\nFN:Ada\r\nPHOTO:data:image/png;base64," + long + "\r\nEND:VCARD\r\n"

	got := string(FoldLines([]byte(doc)))
	for _, line := range strings.Split(strings.TrimSuffix(got, "\r\n"), "\r\n") {
		if len(line) > 75 {
			t.Errorf("a line of %d octets: %q", len(line), line)
		}
	}
	if unfold(got) != doc {
		t.Errorf("unfolding gave back something else:\n%q\nwant\n%q", unfold(got), doc)
	}
	// The short lines are untouched, and the long one is folded with a space.
	if !strings.HasPrefix(got, "BEGIN:VCARD\r\nVERSION:4.0\r\nFN:Ada\r\nPHOTO:data:image/png;base64,") {
		t.Errorf("the short lines changed:\n%q", got[:120])
	}
	if !strings.Contains(got, "\r\n A") {
		t.Errorf("no continuation line:\n%q", got)
	}
}

// A document already short enough comes back as it was, and folding twice
// changes nothing: the second pass sees only short lines.
func TestFoldLinesLeavesShortLinesAlone(t *testing.T) {
	doc := "BEGIN:VEVENT\r\nUID:ada\r\nSUMMARY:" + strings.Repeat("x", 66) + "\r\nEND:VEVENT\r\n"
	if got := string(FoldLines([]byte(doc))); got != doc {
		t.Fatalf("a document of short lines changed:\n%q", got)
	}
	long := "SUMMARY:" + strings.Repeat("y", 200) + "\r\n"
	once := FoldLines([]byte(long))
	if twice := FoldLines(once); string(twice) != string(once) {
		t.Errorf("folding twice changed it:\n%q\nthen\n%q", once, twice)
	}
}

// RFC 6350 §3.2: a fold does not fall inside a character. A line of Cyrillic
// is two octets per letter, so a naive cut at 75 would split one.
func TestFoldLinesNeverSplitsARune(t *testing.T) {
	doc := "SUMMARY:" + strings.Repeat("ё", 120) + "\r\n"
	got := string(FoldLines([]byte(doc)))
	for _, line := range strings.Split(strings.TrimSuffix(got, "\r\n"), "\r\n") {
		if !utf8.ValidString(line) {
			t.Fatalf("a line is not valid UTF-8: %q", line)
		}
		if len(line) > 75 {
			t.Errorf("a line of %d octets", len(line))
		}
	}
	if unfold(got) != doc {
		t.Errorf("unfolding gave back something else:\n%q", unfold(got))
	}
}

// A line without a break at the end keeps not having one: what came in is what
// goes out, folded.
func TestFoldLinesKeepsTheLastLineAsItCame(t *testing.T) {
	doc := "NOTE:" + strings.Repeat("z", 100)
	got := string(FoldLines([]byte(doc)))
	if strings.HasSuffix(got, "\r\n") {
		t.Errorf("a break appeared at the end: %q", got[len(got)-10:])
	}
	if unfold(got) != doc {
		t.Errorf("unfolding gave back something else: %q", unfold(got))
	}
}

// unfold is what a parser does: a CRLF followed by one space or tab is not
// there (RFC 5545 §3.1).
func unfold(s string) string {
	s = strings.ReplaceAll(s, "\r\n ", "")
	return strings.ReplaceAll(s, "\r\n\t", "")
}
