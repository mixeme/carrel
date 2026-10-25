package session

import "time"

// NoteTicked keeps that a task of a collection was marked done now (Р-42 of
// Typecase; the owner's word of 09.10.2026): a list answered from what the
// session read before it still holds the task open, and shows it there
// crossed, with its tick full, until the collection is read again.
func (s *Session) NoteTicked(accountID, path, uid string, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	if s.ticks == nil {
		s.ticks = make(map[string]time.Time)
	}
	s.ticks[readingKey(accountID, path)+"\x00"+uid] = at
}

// TickedSinceRead says whether a task was marked done after the last read of
// its collection's list (ReadingOf): what the session read of it is older
// than the tick.
func (s *Session) TickedSinceRead(accountID, path, uid string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	at, ok := s.ticks[readingKey(accountID, path)+"\x00"+uid]
	if !ok {
		return false
	}
	return at.After(s.readings[readingKey(accountID, path)].At)
}
