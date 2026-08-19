// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"io/fs"
	"net/http"
	"strings"
)

// Handler builds the whole service: the middleware chain of §24.5 wrapped
// around the routes below.
func (s *Server) Handler(staticFS fs.FS) http.Handler {
	return Chain(s.routes(staticFS),
		Recover(s.Logger),
		SecurityHeaders(s.Trust),
		MaxBodyFunc(s.bodyLimit),
		s.LoadSession,
	)
}

// bodyLimit is the ceiling for one request: the default of §24.4 everywhere, and
// the configured ceiling on the handful of paths that take a file.
//
// The list is by path rather than by handler because the decision comes before
// routing. Each of these handlers then applies its own ceiling again, so the one
// that matters is the smaller of the two and neither is load-bearing alone.
func (s *Server) bodyLimit(r *http.Request) int64 {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		return DefaultMaxBody
	}
	path := r.URL.Path
	switch {
	// The file browser and the two attach forms of §23.10.
	case strings.HasPrefix(path, s.Path("/app/files/")), strings.HasSuffix(path, "/attach"):
		return s.filesMaxUpload()
	// Import of .vcf, .ics and .md (§23.7, §23.9).
	case strings.HasSuffix(path, "/import"):
		return s.importMaxBytes()
	// A contact card carries a photo upload (§11).
	case strings.HasPrefix(path, s.Path("/app/contacts/")):
		if s.Photo.MaxUploadBytes > 0 {
			return s.Photo.MaxUploadBytes
		}
		return photoUploadMaxBody
	}
	return DefaultMaxBody
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

	register := http.HandlerFunc(s.Register)
	pages.Handle(s.Path("/register"), Chain(register, s.RateLimit(s.InviteLimit, "register")))

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
	app.HandleFunc("GET "+s.Path("/app/settings/connections"), s.SettingsConnections)
	app.HandleFunc("POST "+s.Path("/app/settings/connections"), s.SettingsConnections)
	app.HandleFunc("GET "+s.Path("/app/settings/account"), s.SettingsAccount)
	app.HandleFunc("GET "+s.Path("/app/settings/attachments"), s.SettingsAttachments)
	app.HandleFunc("POST "+s.Path("/app/settings/attachments"), s.SettingsAttachments)
	app.HandleFunc("GET "+s.Path("/app/settings/appearance"), s.SettingsAppearance)
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
	app.HandleFunc("GET "+s.Path("/app/contacts/{account}/{col}/{uid}/panel"), s.ContactPanel)
	app.HandleFunc("GET "+s.Path("/app/contacts/{account}/{col}/{uid}/edit"), s.ContactEdit)
	app.HandleFunc("POST "+s.Path("/app/contacts/{account}/{col}/{uid}/edit"), s.ContactEdit)
	app.HandleFunc("GET "+s.Path("/app/contacts/{account}/{col}/{uid}"), s.ContactPerson)
	app.HandleFunc("POST "+s.Path("/app/contacts/{account}/{col}/{uid}/conflict"), s.ConflictResolve)
	app.HandleFunc("GET "+s.Path("/app/contacts/{account}/{col}/{uid}/photo-preview"), s.ContactPhotoPreview)
	app.HandleFunc("GET "+s.Path("/app/calendar"), s.CalendarHome)
	app.HandleFunc("GET "+s.Path("/app/calendar/{account}/{col}"), s.CalendarAgenda)
	app.HandleFunc("GET "+s.Path("/app/calendar/{account}/{col}/new"), s.EventNew)
	app.HandleFunc("POST "+s.Path("/app/calendar/{account}/{col}/new"), s.EventNew)
	app.HandleFunc("GET "+s.Path("/app/calendar/{account}/{col}/import"), s.CalendarImport)
	app.HandleFunc("POST "+s.Path("/app/calendar/{account}/{col}/import"), s.CalendarImport)
	app.HandleFunc("GET "+s.Path("/app/calendar/{account}/{col}/export"), s.CalendarExport)
	app.HandleFunc("GET "+s.Path("/app/calendar/{account}/{col}/{uid}/panel"), s.EventPanel)
	app.HandleFunc("GET "+s.Path("/app/calendar/{account}/{col}/{uid}"), s.EventCard)
	app.HandleFunc("POST "+s.Path("/app/calendar/{account}/{col}/{uid}"), s.EventCard)
	app.HandleFunc("POST "+s.Path("/app/calendar/{account}/{col}/{uid}/conflict"), s.CalendarConflictResolve)
	app.HandleFunc("POST "+s.Path("/app/calendar/{account}/{col}/{uid}/attach"), s.EventAttachment)
	// Legacy timeline URL redirects to the contact screen (§1.8).
	app.HandleFunc("GET "+s.Path("/app/contacts/{account}/{col}/{uid}/timeline"), s.ContactTimeline)

	app.HandleFunc("GET "+s.Path("/app/tasks"), s.TasksHome)
	app.HandleFunc("GET "+s.Path("/app/tasks/{account}/{col}"), s.TasksList)
	app.HandleFunc("GET "+s.Path("/app/tasks/{account}/{col}/new"), s.TaskNew)
	app.HandleFunc("POST "+s.Path("/app/tasks/{account}/{col}/new"), s.TaskNew)
	app.HandleFunc("GET "+s.Path("/app/tasks/{account}/{col}/{uid}/panel"), s.TaskPanel)
	app.HandleFunc("GET "+s.Path("/app/tasks/{account}/{col}/{uid}"), s.TaskCard)
	app.HandleFunc("POST "+s.Path("/app/tasks/{account}/{col}/{uid}"), s.TaskCard)
	app.HandleFunc("POST "+s.Path("/app/tasks/{account}/{col}/{uid}/conflict"), s.TaskConflictResolve)
	app.HandleFunc("POST "+s.Path("/app/tasks/{account}/{col}/{uid}/attach"), s.TaskAttachment)

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
	app.HandleFunc("GET "+s.Path("/app/notes/{account}/{col}/related-search"), s.NoteRelatedSearch)
	app.HandleFunc("GET "+s.Path("/app/notes/{account}/{col}/{uid}/panel"), s.NotePanel)
	app.HandleFunc("GET "+s.Path("/app/notes/{account}/{col}/{uid}"), s.NoteCard)
	app.HandleFunc("POST "+s.Path("/app/notes/{account}/{col}/{uid}"), s.NoteCard)
	app.HandleFunc("GET "+s.Path("/app/notes/{account}/{col}/{uid}/export"), s.NoteExport)
	app.HandleFunc("POST "+s.Path("/app/notes/{account}/{col}/{uid}/conflict"), s.NoteConflictResolve)
	app.HandleFunc("POST "+s.Path("/app/notes/{account}/{col}/{uid}/attach"), s.NoteAttachment)

	// The file section of §6 and §7.
	app.HandleFunc("GET "+s.Path("/app/files"), s.FilesHome)
	app.HandleFunc("GET "+s.Path("/app/files/picker"), s.FilesFolderPicker)
	app.HandleFunc("GET "+s.Path("/app/files/picker/children"), s.FilesFolderChildren)
	app.HandleFunc("GET "+s.Path("/app/files/{account}/{col}"), s.FilesBrowse)
	app.HandleFunc("POST "+s.Path("/app/files/{account}/{col}"), s.FilesBrowse)

	// Legacy unified URL; merged views live in each section now (§1.7).
	app.HandleFunc("GET "+s.Path("/app/unified"), s.SectionHome)
	app.HandleFunc("POST "+s.Path("/app/unified/sources"), s.FindSources)
	app.HandleFunc("POST "+s.Path("/app/calendar/sources"), s.FindSources)
	app.HandleFunc("POST "+s.Path("/app/contacts/sources"), s.FindSources)
	app.HandleFunc("POST "+s.Path("/app/tasks/sources"), s.FindSources)
	app.HandleFunc("POST "+s.Path("/app/notes/sources"), s.FindSources)
	app.HandleFunc("GET "+s.Path("/app/search"), s.Search)
	app.HandleFunc("POST "+s.Path("/app/search/sources"), s.FindSources)
	// The duplicates screen of §15. It follows the same poll as the screens
	// above, because detection works on records that have been loaded.
	app.HandleFunc("GET "+s.Path("/app/duplicates"), s.Duplicates)
	app.HandleFunc("POST "+s.Path("/app/duplicates/sources"), s.FindSources)
	app.HandleFunc("POST "+s.Path("/app/duplicates/decide"), s.DuplicateDecide)
	app.HandleFunc("POST "+s.Path("/app/duplicates/merge"), s.DuplicateMerge)
	app.HandleFunc("GET "+s.Path("/app/find/{task}/results"), s.FindResults)
	app.HandleFunc("GET "+s.Path("/app/find/{task}/stream"), s.FindStream)
	app.HandleFunc("POST "+s.Path("/app/find/{task}/retry"), s.FindRetry)
	app.HandleFunc("POST "+s.Path("/app/find/{task}/cancel"), s.FindCancel)
	pages.Handle(s.Path("/app/"), Chain(app, s.RequireAuth, s.RequirePasswordChange))

	// Contact photos: authenticated, separate prefix (§11).
	photos := http.NewServeMux()
	photos.HandleFunc("GET "+s.Path("/c/{account}/{col}/{uid}/photo"), s.ContactPhoto)
	pages.Handle(s.Path("/c/"), Chain(photos, s.RequireAuth, s.RequirePasswordChange))

	// File downloads, on their own prefix for the same reason photos are: these
	// answer with somebody's file rather than a page, and a browser asked to save
	// one should not be handed a URL that looks like a screen (§7).
	downloads := http.NewServeMux()
	downloads.HandleFunc("GET "+s.Path("/d/{account}/{col}"), s.FileDownload)
	downloads.HandleFunc("HEAD "+s.Path("/d/{account}/{col}"), s.FileDownload)
	pages.Handle(s.Path("/d/"), Chain(downloads, s.RequireAuth, s.RequirePasswordChange))

	// Opening an attachment (§23.10). The URI is never taken from the request:
	// the object is read and the attachment at that index is resolved against the
	// collections this user has, so this is a proxy for their own files and not
	// a fetcher of arbitrary addresses (§24.2).
	attachments := http.NewServeMux()
	attachments.HandleFunc("GET "+s.Path("/a/{section}/{account}/{col}/{uid}/{index}"), s.AttachmentOpen)
	attachments.HandleFunc("HEAD "+s.Path("/a/{section}/{account}/{col}/{uid}/{index}"), s.AttachmentOpen)
	pages.Handle(s.Path("/a/"), Chain(attachments, s.RequireAuth, s.RequirePasswordChange))

	// The administrator's section.
	admin := http.NewServeMux()
	admin.HandleFunc("GET "+s.Path("/admin/{$}"), s.AdminHome)
	admin.HandleFunc("POST "+s.Path("/admin/{$}"), s.AdminHome)
	admin.HandleFunc("GET "+s.Path("/admin/{section}"), s.AdminSection)
	admin.HandleFunc("POST "+s.Path("/admin/{section}"), s.AdminSection)
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
