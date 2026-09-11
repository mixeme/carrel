package web

import (
	"net/http"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/session"
)

func (s *Server) requireLogin(w http.ResponseWriter, r *http.Request) (*session.Session, bool) {
	sess, ok := s.sessionFrom(r)
	if !ok {
		if WantsFragment(r) {
			s.notFound(w, r)
			return nil, false
		}
		s.redirect(w, r, "/login")
		return nil, false
	}
	sess.Touch(time.Now())
	return sess, true
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) (*session.Session, bool) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return nil, false
	}
	if !s.users.IsAdmin(sess.AccountID()) {
		s.notFound(w, r)
		return nil, false
	}
	return sess, true
}

// checkOwnership returns false and writes 404 when the viewer does not own the
// resource. Foreign and missing are indistinguishable (S-9.5).
func (s *Server) checkOwnership(w http.ResponseWriter, r *http.Request, sess *session.Session, account, col, uid string) bool {
	if account != "" && !s.users.OwnsAccount(sess.AccountID(), account) {
		s.notFound(w, r)
		return false
	}
	if col != "" && !s.users.OwnsCollection(sess.AccountID(), account, col) {
		s.notFound(w, r)
		return false
	}
	if uid != "" && !s.users.OwnsRecord(sess.AccountID(), account, col, uid) {
		s.notFound(w, r)
		return false
	}
	return true
}

func (s *Server) rejectPathTraversal(w http.ResponseWriter, r *http.Request, path string) bool {
	if path == "" {
		return true
	}
	for _, seg := range strings.Split(path, "/") {
		if seg == ".." {
			s.notFound(w, r)
			return false
		}
	}
	return true
}

// tokenNotFound responds uniformly for invalid, revoked, or expired secrets
// (S-9.5, S-10.7) with constant minimum delay.
func (s *Server) tokenNotFound(w http.ResponseWriter, r *http.Request, start time.Time) {
	constantTimeDelay(start, 200*time.Millisecond)
	s.notFound(w, r)
}
