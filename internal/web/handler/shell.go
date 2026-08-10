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

// AppHome is the signed-in landing page. Stage 1 has nothing to show there
// yet: the account list arrives with stage 2, the profile itself with step 10.
func (s *Server) AppHome(w http.ResponseWriter, r *http.Request) {
	s.Render(w, "app.html", s.View(r, "Carrel"))
}

// AdminHome is the administrator's landing page. The user list, settings and
// audit viewer are added in step 9.
func (s *Server) AdminHome(w http.ResponseWriter, r *http.Request) {
	s.Render(w, "admin.html", s.View(r, "Administration"))
}
