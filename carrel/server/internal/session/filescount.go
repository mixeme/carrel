package session

// How many files a storage holds at every depth, as this session's walk of
// its tree found it (the owner's decision of 13.11.2026; web/files_count.go).
// A count is a few bytes and is not under the budgets of the cache (S-4.2):
// once found it stays with the session, and the walk decides when to find it
// again. It dies with the session.

func filesCountKey(accountID, path string) string { return accountID + "\x00" + path }

// SetFilesCount keeps what the walk found of a storage; nil forgets it.
func (s *Session) SetFilesCount(accountID, path string, value []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	if value == nil {
		delete(s.filesCounts, filesCountKey(accountID, path))
		return
	}
	if s.filesCounts == nil {
		s.filesCounts = make(map[string]string)
	}
	s.filesCounts[filesCountKey(accountID, path)] = string(value)
}

// FilesCount is what the walk last found of a storage.
func (s *Session) FilesCount(accountID, path string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.filesCounts[filesCountKey(accountID, path)]
	return []byte(v), ok
}
