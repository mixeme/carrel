// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"errors"
	"net/http"

	"gitea.mixdep.ru/mix/carrel/internal/store"
)

const (
	fieldCurrentPassword = "current_password"
	fieldNewPassword     = "new_password"
)

// passwordForm is what the change-password screen renders.
type passwordForm struct {
	Forced bool
}

// ChangePassword serves the password change form and processes it. A user on a
// temporary password is held here until they replace it (§5.2).
func (s *Server) ChangePassword(w http.ResponseWriter, r *http.Request) {
	sess := SessionFrom(r)
	if sess == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method == http.MethodPost {
		s.changePasswordSubmit(w, r)
		return
	}

	v := s.View(r, "Change password")
	v.Data = passwordForm{Forced: sess.MustChangePassword()}
	s.Render(w, "password.html", v)
}

func (s *Server) changePasswordSubmit(w http.ResponseWriter, r *http.Request) {
	sess := SessionFrom(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	current := r.PostFormValue(fieldCurrentPassword)
	next := r.PostFormValue(fieldNewPassword)
	confirm := r.PostFormValue(fieldConfirm)
	if next != confirm {
		v := s.View(r, "Change password")
		v.Error = "The new passwords do not match."
		v.Data = passwordForm{Forced: sess.MustChangePassword()}
		s.RenderStatus(w, http.StatusBadRequest, "password.html", v)
		return
	}

	if err := s.Store.ChangePassword(sess.UserID, current, next); err != nil {
		v := s.View(r, "Change password")
		if errors.Is(err, store.ErrAuth) {
			v.Error = "The current password is incorrect."
		} else {
			v.Error = capitalize(err.Error()) + "."
		}
		v.Data = passwordForm{Forced: sess.MustChangePassword()}
		s.RenderStatus(w, http.StatusBadRequest, "password.html", v)
		return
	}

	forced := sess.MustChangePassword()
	s.Sessions.SetMustChangePassword(sess.UserID, false)
	if forced {
		s.redirect(w, r, s.homeFor(sess))
		return
	}

	v := s.View(r, "Change password")
	v.Notice = "Your password was changed."
	v.Data = passwordForm{Forced: false}
	s.Render(w, "password.html", v)
}

// RequirePasswordChange sends users who still carry a temporary password to the
// change screen. Logout and the change form itself stay reachable (§5.2).
func (s *Server) RequirePasswordChange(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := SessionFrom(r)
		if sess == nil || !sess.MustChangePassword() {
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == s.Path("/app/password") {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == s.Path("/logout") {
			next.ServeHTTP(w, r)
			return
		}
		s.redirect(w, r, s.Path("/app/password"))
	})
}
