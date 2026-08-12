// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"

	"gitea.mixdep.ru/mix/carrel/internal/session"
)

// baseTemplate holds the page frame every screen is rendered into.
const baseTemplate = "base.html"

// Templates is the parsed template set. Each page is parsed together with the
// frame, so a page can only render through it, and the whole set is built once
// at startup: a broken template fails the process rather than one request.
type Templates struct {
	pages map[string]*template.Template
}

// LoadTemplates parses every page in fsys against the base frame.
func LoadTemplates(fsys fs.FS) (*Templates, error) {
	names, err := fs.Glob(fsys, "*.html")
	if err != nil {
		return nil, fmt.Errorf("handler: list templates: %w", err)
	}
	t := &Templates{pages: make(map[string]*template.Template, len(names))}
	for _, name := range names {
		if name == baseTemplate {
			continue
		}
		page, err := template.New(name).ParseFS(fsys, baseTemplate, name)
		if err != nil {
			return nil, fmt.Errorf("handler: parse %s: %w", name, err)
		}
		t.pages[name] = page
	}
	if len(t.pages) == 0 {
		return nil, fmt.Errorf("handler: no page templates found")
	}
	return t, nil
}

// View is what every template receives. Handlers add whatever else the page
// needs through Data.
type View struct {
	Title string
	// Base is the mount prefix for links and assets, "" at the root.
	Base string
	// CSRF is the token the page's forms must submit (§24.5).
	CSRF string
	// Session is the caller's session, nil on the public pages.
	Session *session.Session
	// Version and Commit come from build ldflags and appear on /about (§22).
	Version string
	Commit  string
	// SourceURL is the public mirror where users can fetch the source (§22).
	SourceURL string
	// ShowFiles decides whether the navigation carries a Files entry. §6 makes
	// the section conditional on a plain collection having been discovered and
	// forbids a setting for it, so this is asked of the accounts rather than of
	// a preference.
	ShowFiles bool
	// Error and Notice are the two message slots the frame renders.
	Error  string
	Notice string
	Data   any
}

// Admin reports whether the page is being shown to an administrator, which is
// what decides the navigation entries.
func (v View) Admin() bool { return v.Session != nil && v.Session.Admin }

// View builds the common part of a page's data for the current request.
func (s *Server) View(r *http.Request, title string) View {
	sess := SessionFrom(r)
	return View{
		Title:     title,
		Base:      s.BasePath,
		CSRF:      CSRFToken(r),
		Session:   sess,
		Version:   s.Version,
		Commit:    s.Commit,
		SourceURL: SourceURL,
		ShowFiles: s.hasFileCollections(sess),
	}
}

// Render writes a page with status 200.
func (s *Server) Render(w http.ResponseWriter, name string, v View) {
	s.RenderStatus(w, http.StatusOK, name, v)
}

// RenderStatus writes a page with an explicit status — a rejected form comes
// back as 400 or 401 with the form itself in the body.
//
// The page is rendered into a buffer first: a template that fails halfway
// through would otherwise leave a truncated body behind a 200.
func (s *Server) RenderStatus(w http.ResponseWriter, status int, name string, v View) {
	page, ok := s.Templates.pages[name]
	if !ok {
		s.logError("unknown template", fmt.Errorf("no template %q", name))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := page.ExecuteTemplate(&buf, "base", v); err != nil {
		s.logError("render template", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	// Every page here is either a form carrying a token or a view of one
	// account's data. None of it belongs in a shared or back-button cache.
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

// RenderFragment writes a template's body block without the page frame, for
// htmx partial swaps.
func (s *Server) RenderFragment(w http.ResponseWriter, name string, v View) {
	page, ok := s.Templates.pages[name]
	if !ok {
		s.logError("unknown template", fmt.Errorf("no template %q", name))
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := page.ExecuteTemplate(&buf, "body", v); err != nil {
		s.logError("render fragment", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = buf.WriteTo(w)
}

// Fragment renders a template's body block into memory. The progress stream of
// §16 needs the bytes rather than a writer, because one connection carries a
// succession of them and a failure to render must not truncate the stream.
func (s *Server) Fragment(name string, v View) ([]byte, error) {
	page, ok := s.Templates.pages[name]
	if !ok {
		return nil, fmt.Errorf("handler: no template %q", name)
	}
	var buf bytes.Buffer
	if err := page.ExecuteTemplate(&buf, "body", v); err != nil {
		return nil, fmt.Errorf("handler: render %s: %w", name, err)
	}
	return buf.Bytes(), nil
}

func (s *Server) logError(msg string, err error) {
	if s.Logger != nil {
		s.Logger.Error(msg, "error", err)
	}
}
