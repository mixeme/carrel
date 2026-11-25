package web

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"
)

const anonCSRFCookieName = "carrel_csrf"

func newAnonymousCSRFToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func (s *Server) setAnonymousCSRFCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.csrfCookie(),
		Value:    token,
		Path:     s.cookiePath(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.secureCookies(r),
	})
}

// issueAnonymousCSRF sets carrel_csrf and returns the same value for _csrf.
func (s *Server) issueAnonymousCSRF(w http.ResponseWriter, r *http.Request) string {
	token, err := newAnonymousCSRFToken()
	if err != nil {
		// Unreachable in practice; fall back to an empty form field.
		return ""
	}
	s.setAnonymousCSRFCookie(w, r, token)
	return token
}

func needsAnonymousCSRF(r *http.Request) bool {
	p := r.URL.Path
	switch {
	case p == "/login", p == "/setup", p == "/register", p == "/forgot":
		return true
	case strings.HasPrefix(p, "/invite/"):
		return true
	case strings.HasPrefix(p, "/forgot/"):
		return true
	default:
		return false
	}
}

func (s *Server) checkAnonymousCSRF(r *http.Request, token string) bool {
	if token == "" {
		return false
	}
	c, err := r.Cookie(s.csrfCookie())
	if err != nil {
		return false
	}
	return tokensEqual(c.Value, token)
}
