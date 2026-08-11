// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"errors"
	"net/http"

	"gitea.mixdep.ru/mix/carrel/internal/store"
)

// inviteForm is rendered on the accept-invite screen.
type inviteForm struct {
	Token       string
	Login       string
	Email       string
	EmailLocked bool
}

// AcceptInvite lets a bearer choose login, email and password and create their
// account (§5.2).
func (s *Server) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		s.inviteInvalid(w, r)
		return
	}

	if r.Method == http.MethodPost {
		s.acceptInviteSubmit(w, r, token)
		return
	}

	inv, err := s.Store.LookupInvite(token)
	if err != nil {
		s.inviteInvalid(w, r)
		return
	}
	v := s.View(r, "Accept invitation")
	v.Data = inviteForm{
		Token:       token,
		Email:       inv.Email,
		EmailLocked: inv.Delivery == store.InviteDeliveryEmail,
	}
	s.Render(w, "invite.html", v)
}

func (s *Server) acceptInviteSubmit(w http.ResponseWriter, r *http.Request, token string) {
	if err := r.ParseForm(); err != nil {
		s.inviteInvalid(w, r)
		return
	}

	inv, err := s.Store.LookupInvite(token)
	if err != nil {
		s.inviteInvalid(w, r)
		return
	}

	form := inviteForm{
		Token:       token,
		Login:       store.NormalizeLogin(r.PostFormValue(fieldLogin)),
		Email:       store.NormalizeEmail(r.PostFormValue(fieldEmail)),
		EmailLocked: inv.Delivery == store.InviteDeliveryEmail,
	}
	if form.EmailLocked {
		form.Email = inv.Email
	}

	password := r.PostFormValue(fieldPassword)
	if msg := validateInviteAccept(form.Login, form.Email, password, r.PostFormValue(fieldConfirm), form.EmailLocked); msg != "" {
		v := s.View(r, "Accept invitation")
		v.Error = msg
		v.Data = form
		s.RenderStatus(w, http.StatusBadRequest, "invite.html", v)
		return
	}

	user, err := s.Store.AcceptInvite(token, form.Login, form.Email, password, ClientIP(r))
	if err != nil {
		if errors.Is(err, store.ErrInviteInvalid) {
			s.inviteInvalid(w, r)
			return
		}
		if errors.Is(err, store.ErrLoginTaken) {
			v := s.View(r, "Accept invitation")
			v.Error = "That login is already in use."
			v.Data = form
			s.RenderStatus(w, http.StatusBadRequest, "invite.html", v)
			return
		}
		s.logError("accept invite", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	authed, dek, err := s.Store.Authenticate(user.Login, password)
	if err != nil {
		s.logError("sign in after invite", err)
		http.Redirect(w, r, s.Path("/login"), http.StatusSeeOther)
		return
	}
	s.startSession(w, r, authed, dek, "")
}

func validateInviteAccept(login, email, password, confirm string, emailLocked bool) string {
	if err := store.ValidateLogin(login); err != nil {
		return capitalize(err.Error()) + "."
	}
	if !emailLocked {
		if err := store.ValidateEmail(email); err != nil {
			return capitalize(err.Error()) + "."
		}
	}
	if err := store.ValidatePassword(password); err != nil {
		return capitalize(err.Error()) + "."
	}
	if password != confirm {
		return "The two passwords do not match."
	}
	return ""
}

func (s *Server) inviteInvalid(w http.ResponseWriter, r *http.Request) {
	v := s.View(r, "Invitation")
	s.RenderStatus(w, http.StatusGone, "invite_invalid.html", v)
}

// ConfirmEmail applies a pending address after the bearer follows the link
// from their inbox (§5.3).
func (s *Server) ConfirmEmail(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	if token == "" {
		s.emailConfirmInvalid(w, r)
		return
	}

	user, err := s.Store.ConfirmEmailChange(token)
	if err != nil {
		s.emailConfirmInvalid(w, r)
		return
	}
	v := s.View(r, "Email confirmed")
	v.Notice = "The address for " + user.Login + " is now " + user.Email + "."
	s.Render(w, "email_confirmed.html", v)
}

func (s *Server) emailConfirmInvalid(w http.ResponseWriter, r *http.Request) {
	v := s.View(r, "Email confirmation")
	v.Error = "This confirmation link is not valid. It may have expired or already been used."
	s.RenderStatus(w, http.StatusGone, "email_confirmed.html", v)
}

// publicURL builds an absolute URL for links in outbound mail.
func (s *Server) publicURL(r *http.Request, path string) string {
	scheme := "http"
	if s.Trust.IsHTTPS(r) {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
		host = "localhost"
	}
	return scheme + "://" + host + s.Path(path)
}
