// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"io/fs"
	"net/http"
)

// Handler builds the whole service: the middleware chain of §24.5 wrapped
// around the routes below.
func (s *Server) Handler(staticFS fs.FS) http.Handler {
	return Chain(s.routes(staticFS),
		Recover(s.Logger),
		SecurityHeaders(s.Trust),
		MaxBody(DefaultMaxBody),
		s.LoadSession,
	)
}

// routes wires the endpoints that exist so far. The probe and the static
// assets answer on their own; everything that renders a page or takes a form
// goes through the CSRF check, and the two guarded sections are mounted whole
// so a later step cannot add a route inside them and forget the guard.
func (s *Server) routes(staticFS fs.FS) http.Handler {
	pages := http.NewServeMux()
	pages.HandleFunc("GET "+s.Path("/{$}"), s.Index)
	pages.HandleFunc("GET "+s.Path("/setup"), s.Setup)
	pages.HandleFunc("POST "+s.Path("/setup"), s.Setup)
	pages.HandleFunc("GET "+s.Path("/login"), s.Login)
	pages.HandleFunc("POST "+s.Path("/login"), s.Login)
	pages.HandleFunc("POST "+s.Path("/logout"), s.Logout)
	pages.HandleFunc("GET "+s.Path("/forgot"), s.Forgot)

	// Public token pages: no Referer on the URL (§24.3) and a separate rate
	// limit from login.
	invite := http.NewServeMux()
	invite.HandleFunc("GET "+s.Path("/invite/{token}"), s.AcceptInvite)
	invite.HandleFunc("POST "+s.Path("/invite/{token}"), s.AcceptInvite)
	pages.Handle(s.Path("/invite/"), Chain(invite, NoReferrer, s.RateLimit(s.InviteLimit, "invite")))

	confirm := http.NewServeMux()
	confirm.HandleFunc("GET "+s.Path("/confirm-email/{token}"), s.ConfirmEmail)
	pages.Handle(s.Path("/confirm-email/"), Chain(confirm, NoReferrer, s.RateLimit(s.InviteLimit, "confirm")))

	// The user's own section. Anything added under /app is behind RequireAuth.
	app := http.NewServeMux()
	app.HandleFunc("GET "+s.Path("/app/{$}"), s.AppHome)
	app.HandleFunc("GET "+s.Path("/app/password"), s.ChangePassword)
	app.HandleFunc("POST "+s.Path("/app/password"), s.ChangePassword)
	app.HandleFunc("POST "+s.Path("/app/email"), s.RequestEmailChange)
	app.HandleFunc("POST "+s.Path("/app/escrow"), s.ProfileEscrow)
	pages.Handle(s.Path("/app/"), Chain(app, s.RequireAuth, s.RequirePasswordChange))

	// The administrator's section.
	admin := http.NewServeMux()
	admin.HandleFunc("GET "+s.Path("/admin/{$}"), s.AdminHome)
	admin.HandleFunc("POST "+s.Path("/admin/{$}"), s.AdminHome)
	pages.Handle(s.Path("/admin/"), Chain(admin, s.RequireAdmin, s.RequirePasswordChange))

	mux := http.NewServeMux()
	// The probe and the assets need no session and no token; issuing a CSRF
	// cookie on every health check would be pure noise.
	mux.HandleFunc("GET "+s.Path("/healthz"), Health)
	if staticFS != nil {
		mux.Handle("GET "+s.Path("/static/"),
			http.StripPrefix(s.Path("/static/"), http.FileServer(http.FS(staticFS))))
	}
	mux.Handle(s.Path("/"), Chain(pages, s.CSRF))
	return mux
}
