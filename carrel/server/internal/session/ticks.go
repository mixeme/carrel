package session

import "time"

// tick is a task marked done: when, and the version the mark wrote.
type tick struct {
	at   time.Time
	etag string
}

// NoteTicked keeps that a task of a collection was marked done now (Р-42 of
// Typecase; the owner's word of 09.10.2026): a list answered from what the
// session read before it still holds the task open, and shows it there
// crossed, with its tick full, until the collection is read again. etag is
// the version the mark wrote: the full tick takes the mark off that one, not
// the one read before it.
func (s *Session) NoteTicked(accountID, path, uid, etag string, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	if s.ticks == nil {
		s.ticks = make(map[string]tick)
	}
	s.ticks[readingKey(accountID, path)+"\x00"+uid] = tick{at: at, etag: etag}
}

// ForgetTicked drops the mark of a task taken off again: its row is open
// once more.
func (s *Session) ForgetTicked(accountID, path, uid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.ticks, readingKey(accountID, path)+"\x00"+uid)
}

// TickedSinceRead says whether a task was marked done after the last read of
// its collection's list (ReadingOf): what the session read of it is older
// than the tick. etag is the version the mark wrote.
func (s *Session) TickedSinceRead(accountID, path, uid string) (etag string, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.ticks[readingKey(accountID, path)+"\x00"+uid]
	if !ok || !t.at.After(s.readings[readingKey(accountID, path)].At) {
		return "", false
	}
	return t.etag, true
}
