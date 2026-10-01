package session

import "time"

// Reading is what the session last read of one collection (S-22): when, and
// the newest change the server reported among the records it read. Nothing
// here is asked of a server; it is kept from reads that happened anyway, and
// unlike the list index it does not expire with CARREL_CACHE_COLLECTION_TTL.
type Reading struct {
	At      time.Time // when a list of the collection was last read
	Changed time.Time // newest getlastmodified among whole reads; zero when none said
	// Fetched, Size and Took are the last fetch of a feed (S-25.2): what
	// the answer weighed and how long it took to read. Unset for a DAV
	// collection, and for a feed not fetched in this session. A fast local
	// read can measure no time at all, so Fetched says there was one.
	Fetched bool
	Size    int64
	Took    time.Duration
}

func readingKey(accountID, path string) string { return accountID + "\x00" + path }

// NoteReading folds one read into the collection's reading: the later time and
// the newer change win, so reading one kind of a calendar does not hide what
// reading another kind saw. A fetch of a feed (Fetched) replaces the last
// one's measure; a read that measured nothing keeps it.
func (s *Session) NoteReading(accountID, path string, r Reading) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	if s.readings == nil {
		s.readings = make(map[string]Reading)
	}
	k := readingKey(accountID, path)
	old := s.readings[k]
	if !r.Fetched {
		r.Fetched, r.Size, r.Took = old.Fetched, old.Size, old.Took
	}
	if r.At.Before(old.At) {
		r.At = old.At
	}
	if r.Changed.Before(old.Changed) {
		r.Changed = old.Changed
	}
	s.readings[k] = r
}

// ReadingOf returns the reading of one collection; false when this session has
// read no list of it.
func (s *Session) ReadingOf(accountID, path string) (Reading, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.readings[readingKey(accountID, path)]
	return r, ok
}
