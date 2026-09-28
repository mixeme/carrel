package web_test

import (
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

var (
	frameSegBar = regexp.MustCompile(`(?s)<span class="tc-seg[^"]*">(.*?)</span>`)
	frameCount  = regexp.MustCompile(`^(.*\S) (\d+)$`)
)

// frameSegments are the options of the nth segment bar a frame draws, as
// written: «Open 12» where the frame gives a count, «All» where it gives
// none.
func frameSegments(t *testing.T, id string, nth int) []string {
	t.Helper()
	bars := frameSegBar.FindAllStringSubmatch(mockupFrame(t, id), -1)
	if nth >= len(bars) {
		t.Fatalf("%s draws %d segment bars, not %d", id, len(bars), nth+1)
	}
	var out []string
	for _, b := range frameButton.FindAllStringSubmatch(bars[nth][1], -1) {
		out = append(out, frameText(b[1]))
	}
	return out
}

// sameSegments: the server's segments are the frame's — the same names in
// the same order, a count after a space where the frame writes one and none
// where it writes none (the number is the server's own). The one shown now
// is On and stays a choice (C-6.2): enabled, no reason.
func sameSegments(t *testing.T, what string, got []view.Action, frame string, nth int) {
	t.Helper()
	want := frameSegments(t, frame, nth)
	if len(got) != len(want) {
		t.Fatalf("%s segments = %q, want (%s) %q", what, labels(got), frame, want)
	}
	on := 0
	for i, a := range got {
		label := string(a.Label)
		if m := frameCount.FindStringSubmatch(want[i]); m != nil {
			if g := frameCount.FindStringSubmatch(label); g == nil || g[1] != m[1] {
				t.Errorf("%s segment %d = %q, want %q and its count, as %s writes «%s»", what, i, label, m[1], frame, want[i])
			}
		} else if label != want[i] {
			t.Errorf("%s segment %d = %q, want %q (%s draws no count)", what, i, label, want[i], frame)
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
