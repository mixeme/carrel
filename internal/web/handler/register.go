// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"errors"
	"net/http"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/store"
)

// registerForm is rendered on the public sign-up screen.
type registerForm struct {
	Login string
	Email string
}

// Register is the public self-registration form (§5.2). It is a 404 when the
// administrator has not opened it, so a closed instance does not advertise
// a form that will refuse every submission.
func (s *Server) Register(w http.ResponseWriter, r *http.Request) {
	if s.Store.NeedsBootstrap() {
		http.Redirect(w, r, s.Path("/setup"), http.StatusSeeOther)
		return
	}
	if sess := SessionFrom(r); sess != nil {
		http.Redirect(w, r, s.homeFor(sess), http.StatusSeeOther)
		return
	}
	if !s.registrationOpen() {
		http.NotFound(w, r)
		return
	}
	if r.Method == http.MethodPost {
		s.registerSubmit(w, r)
		return
	}
	v := s.View(r, "Create an account")
	v.Data = registerForm{}
	s.Render(w, "register.html", v)
}

func (s *Server) registerSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	form := registerForm{
		Login: store.NormalizeLogin(r.PostFormValue(fieldLogin)),
		Email: store.NormalizeEmail(r.PostFormValue(fieldEmail)),
	}
	password := r.PostFormValue(fieldPassword)
	if msg := validateRegistration(form.Login, form.Email, password, r.PostFormValue(fieldConfirm)); msg != "" {
		s.registerError(w, r, form, msg)
		return
	}
	if s.Mail == nil {
		s.registerError(w, r, form, "Mail is not available. Ask the administrator.")
		return
	}

	_, token, err := s.Store.Register(form.Login, form.Email, password, ClientIP(r))
	if err != nil {
		if errors.Is(err, store.ErrRegistrationClosed) {
			http.NotFound(w, r)
			return
		}
		if errors.Is(err, store.ErrLoginTaken) {
			s.registerError(w, r, form, "That login is already in use.")
			return
		}
		s.logError("register", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	expires := time.Now().Add(store.DefaultEmailChangeTTL)
	link := s.publicURL(r, "/confirm-email/"+token)
	s.Mail.QueueRegistration(form.Email, form.Login, link, expires)

	v := s.View(r, "Check your email")
	v.Notice = "A confirmation link has been sent to " + form.Email +
		". Open it to finish creating your account."
	s.Render(w, "register_sent.html", v)
}

func validateRegistration(login, email, password, confirm string) string {
	if err := store.ValidateLogin(login); err != nil {
		return capitalize(err.Error()) + "."
	}
	if email == "" {
		return "An email address is required."
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

func (s *Server) registerError(w http.ResponseWriter, r *http.Request, form registerForm, msg string) {
	v := s.View(r, "Create an account")
	v.Error = msg
	v.Data = form
	s.RenderStatus(w, http.StatusBadRequest, "register.html", v)
}
