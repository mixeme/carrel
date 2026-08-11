// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"errors"
	"net/http"

	"gitea.mixdep.ru/mix/carrel/internal/crypto"
	"gitea.mixdep.ru/mix/carrel/internal/session"
	"gitea.mixdep.ru/mix/carrel/internal/store"
)

// Form fields shared by the credential screens.
const (
	fieldLogin    = "login"
	fieldPassword = "password"
	fieldConfirm  = "confirm"
	fieldEmail    = "email"
	fieldNext     = "next"
)

// badCredentials is the only thing a failed login is ever told. An unknown
// login, a wrong password and a disabled account must be indistinguishable
// from outside; which one it was goes to the audit log (§5.1, §24.3).
const badCredentials = "Incorrect login or password."

// loginForm is what the login template renders. The password is never echoed
// back.
type loginForm struct {
	Login string
	Next  string
}

// Login serves the sign-in form and takes it.
func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	// A volume with no administrator has nobody to sign in as.
	if s.Store.NeedsBootstrap() {
		http.Redirect(w, r, s.Path("/setup"), http.StatusSeeOther)
		return
	}
	if sess := SessionFrom(r); sess != nil {
		http.Redirect(w, r, s.homeFor(sess), http.StatusSeeOther)
		return
	}
	if r.Method == http.MethodPost {
		s.loginSubmit(w, r)
		return
	}

	v := s.View(r, "Sign in")
	v.Data = loginForm{Next: SafeRedirect(r.URL.Query().Get(fieldNext), "")}
	s.Render(w, "login.html", v)
}

func (s *Server) loginSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.loginError(w, r, http.StatusBadRequest, "", "", "Could not read the form.")
		return
	}
	login := store.NormalizeLogin(r.PostFormValue(fieldLogin))
	password := r.PostFormValue(fieldPassword)
	next := SafeRedirect(r.PostFormValue(fieldNext), "")

	// Both keys are counted: the address stops one attacker walking the user
	// list, the login stops a spread-out attack on one account (§24.5).
	keys := s.loginKeys(r, login)
	if ok, wait := s.LoginLimit.AllowAll(keys...); !ok {
		retryAfter(w, wait)
		s.loginError(w, r, http.StatusTooManyRequests, login, next,
			"Too many attempts. Wait a moment and try again.")
		return
	}

	user, dek, err := s.Store.Authenticate(login, password)
	if err != nil {
		s.LoginLimit.FailAll(keys...)
		s.auditFailedLogin(r, login, err)
		s.loginError(w, r, http.StatusUnauthorized, login, next, badCredentials)
		return
	}

	// The counters were this person's own typos; a success clears them.
	s.LoginLimit.Reset(keys...)
	s.startSession(w, r, user, dek, next)
}

// loginKeys are the rate-limiter keys for one attempt.
func (s *Server) loginKeys(r *http.Request, login string) []string {
	keys := []string{"login:ip:" + ClientIP(r)}
	if login != "" {
		keys = append(keys, "login:user:"+login)
	}
	return keys
}

// auditFailedLogin records the attempt with the reason the response withholds.
func (s *Server) auditFailedLogin(r *http.Request, login string, cause error) {
	detail := "bad credentials"
	switch {
	case errors.Is(cause, store.ErrUserDisabled):
		detail = "account disabled"
	case !errors.Is(cause, store.ErrAuth):
		// A derivation or decryption failure is a fault, not a wrong password.
		detail = "authentication error"
		s.logError("authenticate", cause)
	}
	if err := s.Store.LogLoginFailure(login, ClientIP(r), detail); err != nil {
		s.logError("audit failed login", err)
	}
}

func (s *Server) loginError(w http.ResponseWriter, r *http.Request, status int, login, next, msg string) {
	v := s.View(r, "Sign in")
	v.Error = msg
	v.Data = loginForm{Login: login, Next: next}
	s.RenderStatus(w, status, "login.html", v)
}

// startSession puts a user who has just proved their password into a fresh
// session and sends them on. The DEK passes into the keyring, which owns it
// from here and wipes it when the session ends (§4).
func (s *Server) startSession(w http.ResponseWriter, r *http.Request, user *store.User, dek crypto.Key, next string) {
	sess, err := s.Sessions.Create(session.User{
		ID:                 user.ID,
		Login:              user.Login,
		Admin:              user.IsAdmin(),
		MustChangePassword: user.MustChangePassword,
		EscrowNotice:       len(user.EscrowDEK) > 0 && !user.EscrowNoticeSeen,
	}, dek)
	if err != nil {
		dek.Zero()
		s.logError("create session", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := s.Store.RecordLogin(store.Actor{ID: user.ID, Login: user.Login, IP: ClientIP(r)}); err != nil {
		// The login itself stands; only the bookkeeping failed.
		s.logError("record login", err)
	}

	s.SetSessionCookie(w, r, sess)
	s.redirect(w, r, SafeRedirect(next, s.homeFor(sess)))
}

// Logout ends the session and wipes its key material.
func (s *Server) Logout(w http.ResponseWriter, r *http.Request) {
	if sess := SessionFrom(r); sess != nil {
		s.Sessions.Destroy(sess.ID)
		if err := s.Store.Log(store.AuditEntry{
			Action:      store.ActionLogout,
			ActorID:     sess.UserID,
			ActorLogin:  sess.Login,
			TargetID:    sess.UserID,
			TargetLogin: sess.Login,
			IP:          ClientIP(r),
		}); err != nil {
			s.logError("audit logout", err)
		}
	}
	s.ClearSessionCookie(w, r)
	s.redirect(w, r, s.Path("/login"))
}

// Forgot answers the "forgot password" link. There is nothing behind it to
// click: without the password there is no KEK, so no one — administrator
// included — can decrypt the account's data (§5.3). The page says so plainly
// rather than offering a reset that would silently destroy the data.
func (s *Server) Forgot(w http.ResponseWriter, r *http.Request) {
	s.Render(w, "forgot.html", s.View(r, "Password recovery"))
}

// homeFor is where a signed-in user belongs: administrators land on the panel,
// everyone else on their own page. Both are section roots, so the trailing
// slash is the canonical form and the browser is not sent through a redirect.
func (s *Server) homeFor(sess *session.Session) string {
	if sess != nil && sess.MustChangePassword() {
		return s.Path("/app/password")
	}
	if sess != nil && sess.Admin {
		return s.Path("/admin/")
	}
	return s.Path("/app/")
}

// redirect sends the browser to target, using the htmx header when the request
// came from a fragment so the whole page navigates.
func (s *Server) redirect(w http.ResponseWriter, r *http.Request, target string) {
	if IsHTMX(r) {
		w.Header().Set("HX-Redirect", target)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}
