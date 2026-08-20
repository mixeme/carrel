// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"net/http"
	"strings"
)

// collectionPicker turns a destination chosen in the ⋯ menu of a merged view
// into that collection's own import or export URL (2.6.D2). A merged view has
// no collection of its own to act on until one is picked; the target screen
// re-checks ownership and read-only status itself, the same as it does for
// any other visitor, so this redirect carries no authority of its own.
func (s *Server) collectionPicker(w http.ResponseWriter, r *http.Request, section, action string) {
	home := s.Path("/app/" + section)
	accountID, colEnc, ok := strings.Cut(r.URL.Query().Get("dest"), "|")
	if !ok || accountID == "" || colEnc == "" {
		http.Redirect(w, r, home, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, s.Path("/app/"+section+"/"+accountID+"/"+colEnc+"/"+action), http.StatusSeeOther)
}

func (s *Server) ContactsImportPicker(w http.ResponseWriter, r *http.Request) {
	s.collectionPicker(w, r, "contacts", "import")
}

func (s *Server) ContactsExportPicker(w http.ResponseWriter, r *http.Request) {
	s.collectionPicker(w, r, "contacts", "export")
}

func (s *Server) CalendarImportPicker(w http.ResponseWriter, r *http.Request) {
	s.collectionPicker(w, r, "calendar", "import")
}

func (s *Server) CalendarExportPicker(w http.ResponseWriter, r *http.Request) {
	s.collectionPicker(w, r, "calendar", "export")
}

func (s *Server) NotesImportPicker(w http.ResponseWriter, r *http.Request) {
	s.collectionPicker(w, r, "notes", "import")
}

func (s *Server) NotesExportPicker(w http.ResponseWriter, r *http.Request) {
	s.collectionPicker(w, r, "notes", "export")
}
