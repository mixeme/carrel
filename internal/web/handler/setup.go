// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"errors"
	"net/http"

	"gitea.mixdep.ru/mix/carrel/internal/store"
)

// setupForm is what the setup template renders back on a rejected submission.
// The password is never echoed.
type setupForm struct {
	Login string
	Email string
}

// Setup creates the first administrator on an empty volume (§5.1). It is the
// screen a fresh `docker run` lands on, and it stops working the moment an
// administrator exists — otherwise it would be a public way to mint one.
func (s *Server) Setup(w http.ResponseWriter, r *http.Request) {
	if !s.Store.NeedsBootstrap() {
		http.Redirect(w, r, s.Path("/login"), http.StatusSeeOther)
		return
	}
	if r.Method == http.MethodPost {
		s.setupSubmit(w, r)
		return
	}
	s.Render(w, "setup.html", s.View(r, "Set up Carrel"))
}

func (s *Server) setupSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.setupError(w, r, http.StatusBadRequest, setupForm{}, "Could not read the form.")
		return
	}
	form := setupForm{
		Login: store.NormalizeLogin(r.PostFormValue(fieldLogin)),
		Email: store.NormalizeEmail(r.PostFormValue(fieldEmail)),
	}
	password := r.PostFormValue(fieldPassword)

	// The form is checked here so that the store's remaining failures are all
	// faults of the instance, not of what was typed.
	if msg := validateAccountForm(form.Login, form.Email, password, r.PostFormValue(fieldConfirm)); msg != "" {
		s.setupError(w, r, http.StatusBadRequest, form, msg)
		return
	}

	user, err := s.Store.CreateFirstAdmin(form.Login, form.Email, password, ClientIP(r))
	switch {
	case errors.Is(err, store.ErrNotBootstrap):
		// Someone else got there first between the form and the submission.
		http.Redirect(w, r, s.Path("/login"), http.StatusSeeOther)
		return
	case errors.Is(err, store.ErrLoginTaken):
		s.setupError(w, r, http.StatusBadRequest, form, "That login is already in use.")
		return
	case err != nil:
		s.logError("create first admin", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Sign the new administrator in straight away. The DEK comes from a normal
	// authentication rather than out of the create call: the store hands key
	// material to whoever proves the password, and nowhere else (§4).
	authed, dek, err := s.Store.Authenticate(user.Login, password)
	if err != nil {
		s.logError("sign in first admin", err)
		http.Redirect(w, r, s.Path("/login"), http.StatusSeeOther)
		return
	}
	s.startSession(w, r, authed, dek, "")
}

func (s *Server) setupError(w http.ResponseWriter, r *http.Request, status int, form setupForm, msg string) {
	v := s.View(r, "Set up Carrel")
	v.Error = msg
	v.Data = form
	s.RenderStatus(w, status, "setup.html", v)
}

// validateAccountForm returns the message to show the person filling in a new
// account, or "" when the values are acceptable. It is shared by setup and, in
// stage 6, by invite acceptance.
func validateAccountForm(login, email, password, confirm string) string {
	if err := store.ValidateLogin(login); err != nil {
		return capitalize(err.Error()) + "."
	}
	if err := store.ValidateEmail(email); err != nil {
		return capitalize(err.Error()) + "."
	}
	if err := store.ValidatePassword(password); err != nil {
		return capitalize(err.Error()) + "."
	}
	if password != confirm {
		return "The two passwords do not match."
	}
	return ""
}

// validateInvitePassword checks only the password fields on invite acceptance.
// Login and email are fixed by the invitation itself.
func validateInvitePassword(password, confirm string) string {
	if err := store.ValidatePassword(password); err != nil {
		return capitalize(err.Error()) + "."
	}
	if password != confirm {
		return "The two passwords do not match."
	}
	return ""
}

// capitalize upper-cases the first letter so a validation message reads as a
// sentence in the UI.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] -= 'a' - 'A'
	}
	return string(r)
}
