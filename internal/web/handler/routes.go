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
	pages.HandleFunc("GET "+s.Path("/about"), s.About)
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
	app.HandleFunc("POST "+s.Path("/app/{$}"), s.AppHome)
	app.HandleFunc("GET "+s.Path("/app/password"), s.ChangePassword)
	app.HandleFunc("POST "+s.Path("/app/password"), s.ChangePassword)
	app.HandleFunc("POST "+s.Path("/app/email"), s.RequestEmailChange)
	app.HandleFunc("POST "+s.Path("/app/escrow"), s.ProfileEscrow)
	app.HandleFunc("GET "+s.Path("/app/contacts"), s.ContactsHome)
	app.HandleFunc("GET "+s.Path("/app/contacts/{account}/{col}"), s.ContactsList)
	app.HandleFunc("GET "+s.Path("/app/contacts/{account}/{col}/page"), s.ContactsPage)
	app.HandleFunc("GET "+s.Path("/app/contacts/{account}/{col}/new"), s.ContactNew)
	app.HandleFunc("POST "+s.Path("/app/contacts/{account}/{col}/new"), s.ContactNew)
	app.HandleFunc("GET "+s.Path("/app/contacts/{account}/{col}/import"), s.ContactsImport)
	app.HandleFunc("POST "+s.Path("/app/contacts/{account}/{col}/import"), s.ContactsImport)
	app.HandleFunc("GET "+s.Path("/app/contacts/{account}/{col}/export"), s.ContactsExport)
	app.HandleFunc("GET "+s.Path("/app/contacts/{account}/{col}/{uid}"), s.ContactCard)
	app.HandleFunc("POST "+s.Path("/app/contacts/{account}/{col}/{uid}"), s.ContactCard)
	app.HandleFunc("POST "+s.Path("/app/contacts/{account}/{col}/{uid}/conflict"), s.ConflictResolve)
	app.HandleFunc("GET "+s.Path("/app/contacts/{account}/{col}/{uid}/photo-preview"), s.ContactPhotoPreview)
	app.HandleFunc("GET "+s.Path("/app/calendar"), s.CalendarHome)
	app.HandleFunc("GET "+s.Path("/app/calendar/{account}/{col}"), s.CalendarAgenda)
	app.HandleFunc("GET "+s.Path("/app/calendar/{account}/{col}/new"), s.EventNew)
	app.HandleFunc("POST "+s.Path("/app/calendar/{account}/{col}/new"), s.EventNew)
	app.HandleFunc("GET "+s.Path("/app/calendar/{account}/{col}/import"), s.CalendarImport)
	app.HandleFunc("POST "+s.Path("/app/calendar/{account}/{col}/import"), s.CalendarImport)
	app.HandleFunc("GET "+s.Path("/app/calendar/{account}/{col}/export"), s.CalendarExport)
	app.HandleFunc("GET "+s.Path("/app/calendar/{account}/{col}/{uid}"), s.EventCard)
	app.HandleFunc("POST "+s.Path("/app/calendar/{account}/{col}/{uid}"), s.EventCard)
	app.HandleFunc("POST "+s.Path("/app/calendar/{account}/{col}/{uid}/conflict"), s.CalendarConflictResolve)
	app.HandleFunc("GET "+s.Path("/app/contacts/{account}/{col}/{uid}/timeline"), s.ContactTimeline)

	app.HandleFunc("GET "+s.Path("/app/tasks"), s.TasksHome)
	app.HandleFunc("GET "+s.Path("/app/tasks/{account}/{col}"), s.TasksList)
	app.HandleFunc("GET "+s.Path("/app/tasks/{account}/{col}/new"), s.TaskNew)
	app.HandleFunc("POST "+s.Path("/app/tasks/{account}/{col}/new"), s.TaskNew)
	app.HandleFunc("GET "+s.Path("/app/tasks/{account}/{col}/{uid}"), s.TaskCard)
	app.HandleFunc("POST "+s.Path("/app/tasks/{account}/{col}/{uid}"), s.TaskCard)
	app.HandleFunc("POST "+s.Path("/app/tasks/{account}/{col}/{uid}/conflict"), s.TaskConflictResolve)

	// Notes. The quick form comes before the collection routes so /new and
	// /quick cannot be read as an account identifier.
	app.HandleFunc("GET "+s.Path("/app/notes"), s.NotesHome)
	app.HandleFunc("GET "+s.Path("/app/notes/quick"), s.NoteQuick)
	app.HandleFunc("POST "+s.Path("/app/notes/quick"), s.NoteQuick)
	app.HandleFunc("GET "+s.Path("/app/notes/{account}/{col}"), s.NotesList)
	app.HandleFunc("GET "+s.Path("/app/notes/{account}/{col}/new"), s.NoteNew)
	app.HandleFunc("POST "+s.Path("/app/notes/{account}/{col}/new"), s.NoteNew)
	app.HandleFunc("GET "+s.Path("/app/notes/{account}/{col}/import"), s.NotesImport)
	app.HandleFunc("POST "+s.Path("/app/notes/{account}/{col}/import"), s.NotesImport)
	app.HandleFunc("GET "+s.Path("/app/notes/{account}/{col}/export"), s.NotesExport)
	app.HandleFunc("GET "+s.Path("/app/notes/{account}/{col}/{uid}"), s.NoteCard)
	app.HandleFunc("POST "+s.Path("/app/notes/{account}/{col}/{uid}"), s.NoteCard)
	app.HandleFunc("GET "+s.Path("/app/notes/{account}/{col}/{uid}/export"), s.NoteExport)
	app.HandleFunc("POST "+s.Path("/app/notes/{account}/{col}/{uid}/conflict"), s.NoteConflictResolve)

	// The unified view, the search and the progress endpoints they share (§14,
	// §16).
	app.HandleFunc("GET "+s.Path("/app/unified"), s.Unified)
	app.HandleFunc("POST "+s.Path("/app/unified/sources"), s.FindSources)
	app.HandleFunc("GET "+s.Path("/app/search"), s.Search)
	app.HandleFunc("POST "+s.Path("/app/search/sources"), s.FindSources)
	app.HandleFunc("GET "+s.Path("/app/find/{task}/results"), s.FindResults)
	app.HandleFunc("GET "+s.Path("/app/find/{task}/stream"), s.FindStream)
	app.HandleFunc("POST "+s.Path("/app/find/{task}/retry"), s.FindRetry)
	app.HandleFunc("POST "+s.Path("/app/find/{task}/cancel"), s.FindCancel)
	pages.Handle(s.Path("/app/"), Chain(app, s.RequireAuth, s.RequirePasswordChange))

	// Contact photos: authenticated, separate prefix (§11).
	photos := http.NewServeMux()
	photos.HandleFunc("GET "+s.Path("/c/{account}/{col}/{uid}/photo"), s.ContactPhoto)
	pages.Handle(s.Path("/c/"), Chain(photos, s.RequireAuth, s.RequirePasswordChange))

	// The administrator's section.
	admin := http.NewServeMux()
	admin.HandleFunc("GET "+s.Path("/admin/{$}"), s.AdminHome)
	admin.HandleFunc("POST "+s.Path("/admin/{$}"), s.AdminHome)
	pages.Handle(s.Path("/admin/"), Chain(admin, s.RequireAdmin, s.RequirePasswordChange))

	mux := http.NewServeMux()
	// The probe and the assets need no session and no token; issuing a CSRF
	// cookie on every health check would be pure noise.
	mux.HandleFunc("GET "+s.Path("/healthz"), Health)
	mux.HandleFunc("GET "+s.Path("/manifest.webmanifest"), s.Manifest)
	if staticFS != nil {
		mux.Handle("GET "+s.Path("/static/"),
			http.StripPrefix(s.Path("/static/"), http.FileServer(http.FS(staticFS))))
	}
	mux.Handle(s.Path("/"), Chain(pages, s.CSRF))
	return mux
}
