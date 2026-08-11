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

// appView is what the signed-in landing page renders. The account list arrives
// with stage 2 and the rest of the profile with step 10; what is here now is
// the escrow transparency §5.4 requires before anyone saves a credential.
type appView struct {
	Escrow escrowStatus
}

// buildAppView reads the caller's own record. A user who cannot be read is not
// an error worth a page of its own: the escrow section simply reports nothing.
func (s *Server) buildAppView(r *http.Request) appView {
	sess := SessionFrom(r)
	if sess == nil {
		return appView{}
	}
	user, err := s.Store.User(sess.UserID)
	if err != nil {
		s.logError("load profile", err)
		return appView{}
	}
	return appView{Escrow: escrowStatusOf(s.Store.Settings(), user)}
}

// AppHome is the signed-in landing page.
func (s *Server) AppHome(w http.ResponseWriter, r *http.Request) {
	v := s.View(r, "Carrel")
	s.firstLoginEscrowNotice(r, &v)
	v.Data = s.buildAppView(r)
	s.Render(w, "app.html", v)
}
