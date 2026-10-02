package web_test

import (
	"html"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

var (
	// A bar of segments: tc-seg itself, not tc-segfold around it (s-contacts-7).
	frameSegBar    = regexp.MustCompile(`(?s)<span class="tc-seg(?: [^"]*)?"[^>]*>(.*?)</span>`)
	frameSegChoice = regexp.MustCompile(`(?s)<button(?: [^>]*)?>(.*?)</button>|<a [^>]*href="([^"]*)"[^>]*>(.*?)</a>`)
	frameCount     = regexp.MustCompile(`^(.*\S) (\d+)$`)
	segmentCount   = regexp.MustCompile(`^\d+$`)
)

// frameSegment is one option as a frame draws it: its words as written —
// «Open 12» where the frame gives a count, «All» where it gives none — and
// its address where the option is a link (s-contacts-7), else empty.
type frameSegment struct{ text, href string }

// frameSegments are the options of the nth segment bar a frame draws.
func frameSegments(t *testing.T, id string, nth int) []frameSegment {
	t.Helper()
	bars := frameSegBar.FindAllStringSubmatch(mockupFrame(t, id), -1)
	if nth >= len(bars) {
		t.Fatalf("%s draws %d segment bars, not %d", id, len(bars), nth+1)
	}
	var out []frameSegment
	for _, b := range frameSegChoice.FindAllStringSubmatch(bars[nth][1], -1) {
		if b[1] != "" || b[3] == "" {
			out = append(out, frameSegment{text: frameText(b[1])})
		} else {
			out = append(out, frameSegment{text: frameText(b[3]), href: html.UnescapeString(b[2])})
		}
	}
	return out
}

// segmentWords is a segment the way a frame writes it: the label, and its
// count after a space where it has one (C-6.2).
func segmentWords(a view.Action) string {
	if a.Tally == "" {
		return string(a.Label)
	}
	return string(a.Label) + " " + string(a.Tally)
}

// sameSegments: the server's segments are the frame's — the same names in
// the same order, a count where the frame writes one and none where it
// writes none (the number is the server's own), apart from the label as
// SegFold draws them (C-6.2). Where the frame's option is a link, the
// server's goes to the same place of the screen: the last segment of the
// path and the query. The one shown now is On and stays a choice (C-6.2):
// enabled, no reason.
func sameSegments(t *testing.T, what string, got []view.Action, frame string, nth int) {
	t.Helper()
	want := frameSegments(t, frame, nth)
	if len(got) != len(want) {
		t.Fatalf("%s segments = %q, want (%s) %+v", what, labels(got), frame, want)
	}
	on := 0
	for i, a := range got {
		if m := frameCount.FindStringSubmatch(want[i].text); m != nil {
			if string(a.Label) != m[1] || !segmentCount.MatchString(string(a.Tally)) {
				t.Errorf("%s segment %d = %q, count %q; want %q and its count apart, as %s writes «%s»", what, i, a.Label, a.Tally, m[1], frame, want[i].text)
			}
		} else if string(a.Label) != want[i].text || a.Tally != "" {
			t.Errorf("%s segment %d = %q, count %q; want %q (%s draws no count)", what, i, a.Label, a.Tally, want[i].text, frame)
		}
		if want[i].href != "" && !sameScreenPlace(a.Href, want[i].href) {
			t.Errorf("%s segment %d goes to %q, %s to %q", what, i, a.Href, frame, want[i].href)
		}
		if a.On {
			on++
			if !a.Enabled || a.Reason != "" {
				t.Errorf("%s: the segment shown now %+v is off or gives a reason", what, a)
			}
		}
	}
	if on != 1 {
		t.Errorf("%s: %d segments are on, want one: %+v", what, on, got)
	}
}

// screenPlace is where an address goes within its screen: the last segment
// of its path and its query — the account and collection of a frame are
// not the set's.
func screenPlace(href string) string {
	path, query, _ := strings.Cut(href, "?")
	return path[strings.LastIndex(path, "/")+1:] + "?" + query
}

// sameScreenPlace says whether the server's address goes where the frame's
// does within the screen. A frame drawn on a section's combined view links
// its order to the section's root («/app/contacts?sort=az», s-contacts-1,
// Typecase ac658b0) while the set's list is one collection: there only the
// query is the screen's place.
func sameScreenPlace(server, frame string) bool {
	if screenPlace(server) == screenPlace(frame) {
		return true
	}
	path, query, _ := strings.Cut(frame, "?")
	_, have, _ := strings.Cut(server, "?")
	return strings.Count(strings.Trim(path, "/"), "/") == 1 && query == have
}

// segmentsOf picks the segments by the prefix of their ID.
func segmentsOf(all []view.Action, prefix string) []view.Action {
	out := []view.Action{}
	for _, a := range all {
		if strings.HasPrefix(a.ID, prefix) {
			out = append(out, a)
		}
	}
	return out
}
