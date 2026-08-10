// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"gitea.mixdep.ru/mix/carrel/internal/mail"
	"gitea.mixdep.ru/mix/carrel/internal/ratelimit"
	"gitea.mixdep.ru/mix/carrel/internal/session"
	"gitea.mixdep.ru/mix/carrel/internal/store"
)

// Cookie names. The session cookie carries the identifier only; everything
// about the session, the DEK above all, stays in the server's memory (§4).
const (
	SessionCookie = "carrel_session"
	CSRFCookie    = "carrel_csrf"
)

// Server is what every handler needs: where the app is mounted, whom to
// believe about the client address, where sessions live, and what is on the
// volume.
type Server struct {
	// BasePath is the prefix the service is mounted under, "" for the root.
	// It never ends in a slash.
	BasePath  string
	Trust     *ProxyTrust
	Sessions  *session.Manager
	Store     *store.Store
	Templates *Templates
	// LoginLimit throttles the login form by address and by account (§24.3).
	LoginLimit *ratelimit.Limiter
	// InviteLimit throttles public invite and email-confirmation links (§24.3).
	InviteLimit *ratelimit.Limiter
	Mail        *mail.Queue
	Logger      *slog.Logger
}

// Path turns an app-relative path into an absolute one under BasePath.
func (s *Server) Path(p string) string {
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if s.BasePath == "" {
		return p
	}
	return s.BasePath + p
}

// cookiePath scopes cookies to the mount point, so two services on one host do
// not overwrite each other's sessions.
func (s *Server) cookiePath() string {
	if s.BasePath == "" {
		return "/"
	}
	return s.BasePath + "/"
}

// SetSessionCookie hands the browser its session identifier. Secure is set
// only when the request demonstrably arrived over TLS: sending it from a
// plain-HTTP instance would make the cookie unusable (§24.5).
func (s *Server) SetSessionCookie(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    sess.ID,
		Path:     s.cookiePath(),
		HttpOnly: true,
		Secure:   s.Trust.IsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie removes the session cookie.
func (s *Server) ClearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookie,
		Value:    "",
		Path:     s.cookiePath(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.Trust.IsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

type contextKey int

const (
	sessionKey contextKey = iota
	csrfKey
	clientIPKey
)

// LoadSession attaches the caller's session to the request context when the
// cookie names a live one. It does not require a session: public pages run
// through the same chain.
func (s *Server) LoadSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), clientIPKey, s.Trust.ClientIP(r))

		if c, err := r.Cookie(SessionCookie); err == nil {
			if sess, ok := s.Sessions.Get(c.Value); ok {
				ctx = context.WithValue(ctx, sessionKey, sess)
			} else {
				// The cookie names a session that is gone: clear it so the
				// browser stops sending it.
				s.ClearSessionCookie(w, r)
			}
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// SessionFrom returns the caller's session, or nil when they are not logged in.
func SessionFrom(r *http.Request) *session.Session {
	sess, _ := r.Context().Value(sessionKey).(*session.Session)
	return sess
}

// ClientIP returns the address LoadSession attributed the request to.
func ClientIP(r *http.Request) string {
	ip, _ := r.Context().Value(clientIPKey).(string)
	return ip
}
