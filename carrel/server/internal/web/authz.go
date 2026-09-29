package web

import (
	"net/http"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

func (s *Server) requireLogin(w http.ResponseWriter, r *http.Request) (*session.Session, bool) {
	sess, ok := s.sessionFrom(r)
	if !ok {
		if render.WantsFragment(r) {
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
	key, live := sess.DataKey()
	if !live {
		s.notFound(w, r)
		return false
	}
	if account != "" && !s.users.OwnsAccount(sess.AccountID(), key, account) {
		s.notFound(w, r)
		return false
	}
	if col != "" && !s.users.OwnsCollection(sess.AccountID(), key, account, col) {
		s.notFound(w, r)
		return false
	}
	if uid != "" && !s.users.OwnsRecord(sess.AccountID(), key, account, col, uid) {
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

// tokenNotFound responds uniformly for invalid, used, revoked, or expired
// secrets (S-9.5, S-10.7, C-11) with constant minimum delay: one page,
// «This link does not work» (rest-1), 404 — «not found» is true of a link
// that never was, and it is the only thing the answer may say. Nothing in
// it depends on the route or the secret: the words, the door and the
// headers are the same for /invite/, /forgot/, /confirm-email/ and /p/
// (their no-referrer is set by the path's family, not by the secret), and
// for the backup trigger's /api/backup/ as well.
func (s *Server) tokenNotFound(w http.ResponseWriter, r *http.Request, start time.Time) {
	constantTimeDelay(start, 200*time.Millisecond)
	s.writePage(w, r, http.StatusNotFound, view.DeadLinkPage{
		Plain: view.Plain{
			Title: "This link does not work",
			Lede:  "It may have expired, been used already, or been withdrawn. Ask whoever sent it for a new one.",
		},
		SignIn: view.Action{ID: "deadlink.login", Label: "Sign in", Href: s.route("/login"), Method: "GET", Enabled: true},
	})
}
