package note

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// A related record this session has not read is its UID with the server's
// reason, not «8f1c-4a02-b7d3 ·» with nothing after the dot (the phone,
// 26.11.2026).
func TestARelatedRecordNotReadSaysWhy(t *testing.T) {
	p := &view.NotePage{Doc: view.NoteDoc{Title: "Заметка со встречи 10.08"}, Meta: view.NoteMeta{Related: []view.TimelineRow{
		{Title: "8f1c-4a02-b7d3", Why: "Not in the lists loaded for this collection"},
		{Title: "Синк-ап по релизу", Kind: "event", When: "10 Aug", Why: "Related to · in another collection"},
	}}}
	html := Screen(p).Render()
	for _, want := range []string{"8f1c-4a02-b7d3", "· Not in the lists loaded for this collection", "· event · 10 Aug"} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q in %s", want, html)
		}
	}
	if strings.Contains(html, "· <") {
		t.Errorf("a «· » before nothing: %s", html)
	}
}
