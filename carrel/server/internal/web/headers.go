package web

import (
	"net/http"
	"strings"
)

const referrerPolicyNoReferrer = "no-referrer"

// contentSecurityPolicy is the policy of every page (S-29.1, C-11): nothing
// runs or is styled inline, nothing is fetched from another origin but the
// QR code's data: image (S-23), nothing frames a page of Carrel. A download
// replaces it with a sandbox of its own (downloadHeaders).
const contentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; " +
	"font-src 'self'; connect-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'"

// secureHeaders are the headers of every answer of the web listener, set
// before any handler and before the mux's own 404: a client renders what it
// gets on this origin, so the policy is promised whoever writes the body
// — the JSON renderer included (audit №3, Ф3-1). A handler that knows
// better overrides: a file keeps its sandbox and its caching, a photo its
// ETag, a page with a secret in its path answers no-referrer. Everything
// else is personal and is not kept by a cache on the way.
func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// pathHasURLSecret reports whether the request path carries a secret in the
// URL (C-5).
func pathHasURLSecret(path string) bool {
	switch {
	case strings.HasPrefix(path, "/invite/"):
		return len(path) > len("/invite/")
	case strings.HasPrefix(path, "/confirm-email/"):
		return len(path) > len("/confirm-email/")
	case strings.HasPrefix(path, "/forgot/"):
		return len(path) > len("/forgot/")
	case strings.HasPrefix(path, "/p/"):
		return len(path) > len("/p/")
	default:
		return false
	}
}

func setSecretPathHeaders(w http.ResponseWriter, r *http.Request) {
	if pathHasURLSecret(r.URL.Path) {
		w.Header().Set("Referrer-Policy", referrerPolicyNoReferrer)
	}
}
