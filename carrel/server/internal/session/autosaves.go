package session

// autosaved is what an edit's auto-saves wrote (Typecase's note 229): the
// version the form was opened at, and the version the last write of this
// session made of it.
type autosaved struct {
	from, to string
}

// autosavesKept is how many records' auto-saves a session remembers: one
// edit at a time in a window, a few windows.
const autosavesKept = 32

// NoteAutosaved keeps that an edit opened at version from (empty for a new
// record, whose form carries none) was written by this session as version
// to. The page does not change while it auto-saves: its form still says
// from, and the next write of the same form — an auto-save or its Save —
// edits to, not from (AutosavedFrom).
func (s *Session) NoteAutosaved(accountID, path, uid, from, to string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	if s.autosaves == nil {
		s.autosaves = make(map[string]autosaved)
	}
	key := readingKey(accountID, path) + "\x00" + uid
	if _, ok := s.autosaves[key]; !ok && len(s.autosaves) >= autosavesKept {
		for k := range s.autosaves {
			delete(s.autosaves, k)
			break
		}
	}
	s.autosaves[key] = autosaved{from: from, to: to}
}

// AutosavedFrom is the version a form opened at from edits now: the one this
// session's auto-save last wrote of it, or from itself when none did. Only
// this session's writes are followed: a version written elsewhere since
// is still a conflict, as the server's precondition says.
func (s *Session) AutosavedFrom(accountID, path, uid, from string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a, ok := s.autosaves[readingKey(accountID, path)+"\x00"+uid]; ok && a.from == from {
		return a.to
	}
	return from
}
