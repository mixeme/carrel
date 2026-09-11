package web

import (
	"net/http"
	"strings"
)

const referrerPolicyNoReferrer = "no-referrer"

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
