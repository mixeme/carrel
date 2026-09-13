package session

import "carrel.dev/carrel/server/internal/conf"

// Transfer is one row of the transfer queue (S-17.3): an upload, a move or a
// copy the person started. The queue lives in the session, not in the profile,
// and the session does not know what a row moves. A waiting row asks what to
// do with a taken name and stays until it is answered; Discard lets go of
// what a row holds — an upload waiting for its answer keeps its file on disk.
// A kept row does not change: an answer takes it out and keeps a new one.
type Transfer interface {
	ID() string
	Waiting() bool
	Discard()
}

// KeepTransfer puts a row at the end of the queue. Past TransferQueueLen the
// oldest row that asks nothing goes, then the oldest question; whatever goes
// is discarded. An ended session discards the row at once.
func (s *Session) KeepTransfer(t Transfer) {
	s.mu.Lock()
	if s.ended {
		s.mu.Unlock()
		t.Discard()
		return
	}
	s.transfers = append(s.transfers, t)
	var gone []Transfer
	for len(s.transfers) > conf.TransferQueueLen {
		victim := 0
		for i, q := range s.transfers {
			if !q.Waiting() {
				victim = i
				break
			}
		}
		gone = append(gone, s.transfers[victim])
		s.transfers = append(s.transfers[:victim], s.transfers[victim+1:]...)
	}
	s.mu.Unlock()
	for _, g := range gone {
		g.Discard()
	}
}

// TakeTransfer removes one row of this session and hands it over, so two
// answers to one question do not both act. A row of another session is not
// found here.
func (s *Session) TakeTransfer(id string) (Transfer, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, t := range s.transfers {
		if t.ID() == id {
			s.transfers = append(s.transfers[:i], s.transfers[i+1:]...)
			return t, true
		}
	}
	return nil, false
}

// ReadTransfers returns the queue in the order the rows came. Waiting rows
// stay; the others are handed over once — a finished row is shown by the
// answer that reads it, not on every page after.
func (s *Session) ReadTransfers() []Transfer {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := append([]Transfer(nil), s.transfers...)
	kept := s.transfers[:0]
	for _, t := range s.transfers {
		if t.Waiting() {
			kept = append(kept, t)
		}
	}
	clear(s.transfers[len(kept):])
	s.transfers = kept
	return out
}
