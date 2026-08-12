// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import "net/http"

// Index decides where a visitor to the root belongs: the setup screen while
// the volume has no administrator, their own page once signed in, and the
// login form otherwise.
func (s *Server) Index(w http.ResponseWriter, r *http.Request) {
	switch {
	case s.Store.NeedsBootstrap():
		http.Redirect(w, r, s.Path("/setup"), http.StatusSeeOther)
	case SessionFrom(r) != nil:
		http.Redirect(w, r, s.homeFor(SessionFrom(r)), http.StatusSeeOther)
	default:
		http.Redirect(w, r, s.Path("/login"), http.StatusSeeOther)
	}
}
