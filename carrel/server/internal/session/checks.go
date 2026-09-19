package session

import "carrel.dev/carrel/server/view"

// NoteCheck keeps the trace of the last check of one connection (C-6
// ConnectionCard.Trace). It belongs to the session, like a search result:
// nothing about a check is written to the blob or the journal.
func (s *Session) NoteCheck(accountID string, trace *view.DiagnosticTrace) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	if s.checks == nil {
		s.checks = make(map[string]*view.DiagnosticTrace)
	}
	s.checks[accountID] = trace
}

// CheckOf is the trace of the last check of the connection in this session;
// nil when none ran.
func (s *Session) CheckOf(accountID string) *view.DiagnosticTrace {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.checks[accountID]
}
