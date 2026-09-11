package web

import (
	"errors"
	"net/http"
	"time"

	"carrel.dev/carrel/server/internal/conf"
)

type handler func(*Server, http.ResponseWriter, *http.Request)

func (s *Server) handle(pattern string, h handler) {
	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		s.serve(w, r, h)
	})
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request, h handler) {
	setSecretPathHeaders(w, r)
	now := time.Now()

	// /healthz and /about carry no bucket at all (architecture.md): probes
	// and the licence obligation never see 429, cookie or not.
	if c, err := r.Cookie(cookieName); err == nil && c.Value != "" && !unmetered(r.URL.Path) {
		if !s.limit.allowSession(c.Value, now) {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
	}

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		r.Body = http.MaxBytesReader(w, r.Body, conf.MaxFormBodyBytes)

		sess, hasSess := s.sessionFrom(r)
		if needsCSRF(r) && hasSess {
			if token := r.Header.Get(csrfHeaderName); token != "" {
				if !sess.CheckCSRF(token) {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
			} else if token, err := csrfTokenFromBody(r); err != nil {
				if tooLarge(err) {
					http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
					return
				}
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			} else if !sess.CheckCSRF(token) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		} else if needsCSRF(r) && !hasSess && needsAnonymousCSRF(r) {
			if token := r.Header.Get(csrfHeaderName); token != "" {
				if !checkAnonymousCSRF(r, token) {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
			} else if token, err := csrfTokenFromBody(r); err != nil {
				if tooLarge(err) {
					http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
					return
				}
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			} else if !checkAnonymousCSRF(r, token) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		} else if err := r.ParseForm(); err != nil {
			if tooLarge(err) {
				http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
	}

	h(s, w, r)
}

func unmetered(path string) bool {
	return path == "/healthz" || path == "/about"
}

func tooLarge(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}

func needsCSRF(r *http.Request) bool {
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func (s *Server) writePage(w http.ResponseWriter, r *http.Request, status int, body any) {
	if WantsFragment(r) {
		_ = s.render.Fragment(w, r, status, FragmentTarget(r), body, nil)
		return
	}
	_ = s.render.Page(w, r, status, body)
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

func (s *Server) redirect(w http.ResponseWriter, r *http.Request, url string) {
	http.Redirect(w, r, s.route(url), http.StatusSeeOther)
}
