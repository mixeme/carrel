// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import "time"

// Log appends one entry on its own. Most changes carry their audit record in
// the same commit; this is for events with nothing to change — a failed login,
// a logout, a test message (§5.5).
func (s *Store) Log(entry AuditEntry) error {
	return s.update(func(state *State) error {
		appendAudit(state, s.now(), entry)
		return nil
	})
}

// LogLoginFailure records a rejected login attempt. The login is kept, the
// password never is.
func (s *Store) LogLoginFailure(login, ip, detail string) error {
	return s.Log(AuditEntry{
		Action:      ActionLoginFailed,
		TargetLogin: NormalizeLogin(login),
		IP:          ip,
		Detail:      detail,
	})
}

// Audit returns log entries matching the filter, newest first.
func (s *Store) Audit(f AuditFilter) []AuditEntry {
	var out []AuditEntry
	s.read(func(state *State) {
		for i := len(state.Audit) - 1; i >= 0; i-- {
			e := state.Audit[i]
			if !matches(e, f) {
				continue
			}
			out = append(out, e)
			if f.Limit > 0 && len(out) >= f.Limit {
				return
			}
		}
	})
	return out
}

func matches(e AuditEntry, f AuditFilter) bool {
	switch {
	case f.Action != "" && e.Action != f.Action:
		return false
	case f.ActorID != "" && e.ActorID != f.ActorID:
		return false
	case f.TargetID != "" && e.TargetID != f.TargetID:
		return false
	case !f.Since.IsZero() && e.At.Before(f.Since):
		return false
	case len(f.Categories) > 0 && !containsAction(f.Categories, e.Action):
		return false
	}
	return true
}

func containsAction(actions []string, action string) bool {
	for _, a := range actions {
		if a == action {
			return true
		}
	}
	return false
}

// appendAudit stamps and appends an entry inside an update, so the record and
// the change it describes reach the disk together. Entries are never edited;
// once the log is full the oldest are dropped, which bounds the state file
// without rewriting history.
func appendAudit(state *State, now time.Time, e AuditEntry) {
	state.AuditSeq++
	e.Seq = state.AuditSeq
	if e.At.IsZero() {
		e.At = now
	}
	state.Audit = append(state.Audit, e)
	if over := len(state.Audit) - MaxAuditEntries; over > 0 {
		state.Audit = append(state.Audit[:0], state.Audit[over:]...)
	}
}
