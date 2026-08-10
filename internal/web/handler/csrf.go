// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"

	"gitea.mixdep.ru/mix/carrel/internal/crypto"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

// Where a token may be submitted from. The header is what htmx sends; the
// field is what a plain form posts (§24.5).
const (
	CSRFHeader = "X-CSRF-Token"
	CSRFField  = "csrf_token"
)

// CSRF rejects mutating requests that do not carry the expected token.
//
// For a logged-in caller the token is the session's own, so it cannot be
// obtained without already holding the session cookie. The setup, login and
// invite forms are posted by people who have no session yet; those fall back
// to a double-submit cookie, which is weaker but still forces the attacker to
// read a response from this origin.
func (s *Server) CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expected := ""
		if sess := SessionFrom(r); sess != nil {
			expected = sess.CSRF
		} else {
			var err error
			if expected, err = s.anonymousToken(w, r); err != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
		}
		r = r.WithContext(context.WithValue(r.Context(), csrfKey, expected))

		if safeMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}
		if expected == "" || !crypto.Equal([]byte(expected), []byte(submittedToken(r))) {
			http.Error(w, "invalid or missing CSRF token", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CSRFToken returns the token a template must place in its form.
func CSRFToken(r *http.Request) string {
	token, _ := r.Context().Value(csrfKey).(string)
	return token
}

// anonymousToken reads the double-submit cookie, issuing one if the browser
// has none yet.
func (s *Server) anonymousToken(w http.ResponseWriter, r *http.Request) (string, error) {
	if c, err := r.Cookie(CSRFCookie); err == nil && c.Value != "" {
		return c.Value, nil
	}
	raw, err := crypto.Random(session.IDLen)
	if err != nil {
		return "", err
	}
	defer crypto.Zero(raw)
	token := base64.RawURLEncoding.EncodeToString(raw)

	// HttpOnly: the token reaches the page through the rendered form, never
	// through script reading the cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookie,
		Value:    token,
		Path:     s.cookiePath(),
		HttpOnly: true,
		Secure:   s.Trust.IsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
	return token, nil
}

func safeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	}
	return false
}

// submittedToken pulls the token out of the request. The form body is parsed
// only for url-encoded posts; anything else must use the header, which keeps
// this middleware from consuming a body it does not understand.
func submittedToken(r *http.Request) string {
	if v := r.Header.Get(CSRFHeader); v != "" {
		return v
	}
	ct := r.Header.Get("Content-Type")
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	if !strings.EqualFold(strings.TrimSpace(ct), "application/x-www-form-urlencoded") {
		return ""
	}
	if err := r.ParseForm(); err != nil {
		return ""
	}
	return r.PostFormValue(CSRFField)
}
