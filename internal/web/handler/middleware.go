// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"log/slog"
	"net/http"
	"strings"
)

// Middleware wraps a handler.
type Middleware func(http.Handler) http.Handler

// Chain applies middleware so that the first listed runs first.
func Chain(h http.Handler, mw ...Middleware) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}

// contentSecurityPolicy is the main defence against injected markup (§24.5).
// There is no 'unsafe-inline' for scripts: htmx works through attributes, so
// nothing inline is needed, and allowing it would give up most of what CSP is
// for. Everything is served from this origin; data: is open for images only,
// which is what inline icons need.
const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self'; " +
	"worker-src 'self'; " +
	"style-src 'self'; " +
	"img-src 'self' data:; " +
	"font-src 'self'; " +
	"connect-src 'self'; " +
	"form-action 'self'; " +
	"frame-ancestors 'none'; " +
	"base-uri 'none'; " +
	"object-src 'none'"

// hstsValue is only ever sent when the request demonstrably arrived over TLS.
// Sending it from a plain-HTTP instance would lock users out of their own
// service for a year.
const hstsValue = "max-age=31536000; includeSubDomains"

// SecurityHeaders sets the response headers of §24.5 on every route.
func SecurityHeaders(trust *ProxyTrust) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("Content-Security-Policy", contentSecurityPolicy)
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("X-Frame-Options", "DENY")
			if h.Get("Referrer-Policy") == "" {
				h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			}
			if trust.IsHTTPS(r) {
				h.Set("Strict-Transport-Security", hstsValue)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// NoReferrer suppresses the Referer header for pages whose URL holds a secret.
// An invite link must not leak through a click on anything the page shows
// (§24.3).
func NoReferrer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// DefaultMaxBody is the request body limit applied everywhere. Endpoints that
// legitimately take more — photo upload, archive restore — raise it for
// themselves (§24.4).
const DefaultMaxBody = 1 << 20 // 1 MiB

// MaxBody caps the request body. A caller reading past the limit gets an
// error rather than an unbounded allocation.
func MaxBody(n int64) Middleware {
	return MaxBodyFunc(func(*http.Request) int64 { return n })
}

// MaxBodyFunc caps the request body at a ceiling chosen per request.
//
// The choice has to be made here, before anything reads the body, and not in the
// handler that wants a larger one. The CSRF check of §24.5 reads a multipart form
// to find the token a plain HTML form carries as a field, so a limit imposed
// after it would truncate the upload it was meant to authorise — and the person
// would be told their token was invalid, which is both untrue and unactionable.
func MaxBodyFunc(limit func(*http.Request) int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := limit(r)
			if n <= 0 {
				n = DefaultMaxBody
			}
			r.Body = http.MaxBytesReader(w, r.Body, n)
			next.ServeHTTP(w, r)
		})
	}
}

// Recover turns a panic into a 500 and a log line. The client learns nothing
// about what broke; the stack stays on the server.
func Recover(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if v := recover(); v != nil {
					if logger != nil {
						logger.Error("panic in handler",
							"method", r.Method,
							"path", r.URL.Path,
							"panic", v,
						)
					}
					http.Error(w, "internal server error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// Health answers the liveness probe. It says nothing about the version, the
// number of users or the state of anything downstream (§24.3).
func Health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

// IsHTMX reports whether the request came from htmx, which decides between a
// fragment and a full page.
func IsHTMX(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("HX-Request"), "true")
}
