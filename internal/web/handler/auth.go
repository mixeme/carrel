// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/ratelimit"
)

// RequireAuth guards everything under /app. An anonymous caller is sent to the
// login form; an htmx fragment gets HX-Redirect instead, because a redirect
// inside a fragment would swap the login page into a corner of the app.
func (s *Server) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if SessionFrom(r) == nil {
			s.redirectToLogin(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAdmin guards everything under /admin. A logged-in non-administrator
// is refused outright rather than redirected: they are not going to become an
// administrator by signing in again.
func (s *Server) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := SessionFrom(r)
		if sess == nil {
			s.redirectToLogin(w, r)
			return
		}
		if !sess.Admin {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) redirectToLogin(w http.ResponseWriter, r *http.Request) {
	target := s.Path("/login")
	if next := safeNext(r); next != "" {
		target += "?next=" + url.QueryEscape(next)
	}
	if IsHTMX(r) {
		w.Header().Set("HX-Redirect", target)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}

// safeNext returns the path to come back to after login, or "" when it cannot
// be trusted. Only a same-origin path is ever echoed back: a value starting
// with "//" or carrying a scheme would turn the login form into an open
// redirect.
func safeNext(r *http.Request) string {
	if r.Method != http.MethodGet {
		return ""
	}
	p := r.URL.RequestURI()
	if !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") || strings.Contains(p, "\\") {
		return ""
	}
	return p
}

// SafeRedirect returns dst when it is a same-origin path, and fallback
// otherwise. Handlers use it for the post-login "next" parameter.
func SafeRedirect(dst, fallback string) string {
	if dst == "" || !strings.HasPrefix(dst, "/") || strings.HasPrefix(dst, "//") || strings.Contains(dst, "\\") {
		return fallback
	}
	if u, err := url.Parse(dst); err != nil || u.Scheme != "" || u.Host != "" {
		return fallback
	}
	return dst
}

// RateLimit throttles an endpoint by client address. It is meant for the
// public routes of §24.3 — invite links above all — and is counted separately
// from the login limiter.
func (s *Server) RateLimit(l *ratelimit.Limiter, scope string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := scope + ":" + ClientIP(r)
			if ok, wait := l.Allow(key); !ok {
				retryAfter(w, wait)
				http.Error(w, "too many requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func retryAfter(w http.ResponseWriter, wait time.Duration) {
	seconds := int(wait.Round(time.Second) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
}
