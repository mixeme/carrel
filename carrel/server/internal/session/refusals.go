package session

// A server that refused a request once refuses it again: iCloud answers every
// calendar-query with a text-match on UID 412 (docs/servers.md, SV-54). What a
// server refused in a collection is kept for the session, so the round-about
// way is taken straight away the next time instead of after one more refusal.
// Nothing here is asked of a server; it dies with the session, and a new one
// asks once again — a server may have learnt better in between.

func refusalKey(accountID, path, what string) string {
	return accountID + "\x00" + path + "\x00" + what
}

// NoteRefusal keeps that the server of a connection refused what in a
// collection.
func (s *Session) NoteRefusal(accountID, path, what string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	if s.refusals == nil {
		s.refusals = make(map[string]bool)
	}
	s.refusals[refusalKey(accountID, path, what)] = true
}

// Refused reports whether this session saw the server refuse what in the
// collection.
func (s *Session) Refused(accountID, path, what string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.refusals[refusalKey(accountID, path, what)]
}
