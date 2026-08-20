// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"encoding/json"
	"io/fs"
	"net/http"
)

// SourceURL is where public links to the running service's source code point.
// Per spec §22, this is the GitHub mirror, not the internal Gitea host.
const SourceURL = "https://github.com/mixeme/carrel"

// About serves the public «About this service» page (§22).
func (s *Server) About(w http.ResponseWriter, r *http.Request) {
	s.Render(w, "about.html", s.View(r, "About"))
}

// Manifest serves the web app manifest for install-as-app (§13).
func (s *Server) Manifest(w http.ResponseWriter, r *http.Request) {
	icon := s.Path("/static/icon.svg")
	manifest := map[string]any{
		"name":             "Carrel",
		"short_name":       "Carrel",
		"description":      "Self-hosted web client for CalDAV, CardDAV and WebDAV",
		"start_url":        s.Path("/app/"),
		"scope":            s.Path("/"),
		"display":          "standalone",
		"background_color": "#F5F1E8",
		"theme_color":      "#4A6B52",
		"icons": []map[string]string{
			{
				"src":     icon,
				"sizes":   "any",
				"type":    "image/svg+xml",
				"purpose": "any maskable",
			},
		},
	}
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(manifest)
}

// ServiceWorker serves the PWA shell worker (§13, wave 3.6). It is not under
// /static/ so its scope covers the mount prefix; updates must not be cached
// for a year by the browser.
func (s *Server) ServiceWorker(w http.ResponseWriter, r *http.Request, staticFS fs.FS) {
	if staticFS == nil {
		http.NotFound(w, r)
		return
	}
	b, err := fs.ReadFile(staticFS, "sw.js")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(b)
}
