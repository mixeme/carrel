package session

import (
	"testing"
	"time"
)

// S-22: reads of one collection fold together — the latest read and the newest
// change win, whatever order the kinds were read in — and an ended session
// keeps nothing.
func TestReadingFoldsReads(t *testing.T) {
	s := &Session{}
	t0 := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	if _, ok := s.ReadingOf("a", "/cal/"); ok {
		t.Fatal("a reading before any read")
	}
	s.NoteReading("a", "/cal/", Reading{At: t0, Changed: t0.Add(-time.Hour)})
	s.NoteReading("a", "/cal/", Reading{At: t0.Add(time.Minute)}) // an agenda: no change
	s.NoteReading("a", "/cal/", Reading{At: t0.Add(-time.Minute), Changed: t0.Add(-2 * time.Hour)})
	r, ok := s.ReadingOf("a", "/cal/")
	if !ok || !r.At.Equal(t0.Add(time.Minute)) || !r.Changed.Equal(t0.Add(-time.Hour)) {
		t.Errorf("reading = %+v, %v", r, ok)
	}
	if _, ok := s.ReadingOf("b", "/cal/"); ok {
		t.Error("another connection's collection shares the reading")
	}
	s.ended = true
	s.readings = nil
	s.NoteReading("a", "/cal/", Reading{At: t0})
	if _, ok := s.ReadingOf("a", "/cal/"); ok {
		t.Error("an ended session kept a reading")
	}
}
