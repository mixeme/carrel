// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"net/http"

	"gitea.mixdep.ru/mix/carrel/internal/provider/files"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

func (s *Server) filesAttachmentsShortcut(sess *session.Session) (url, hint string) {
	if target, ok := s.attachmentTarget(sess); ok {
		for _, row := range s.fileCollections(sess) {
			if row.AccountID != target.AccountID {
				continue
			}
			if normalizeCollectionPath(row.Path) != normalizeCollectionPath(target.Root) {
				continue
			}
			base := s.Path("/app/files/" + row.AccountID + "/" + row.ColEnc)
			if target.Rel == "" {
				return base, row.Label()
			}
			return folderURL(base, target.Rel), row.Label() + " / " + files.Base(target.Rel)
		}
	}
	return s.Path("/app/settings/attachments"), ""
}

// FilesPublished is a placeholder until publication links are listed in wave 4.3.
func (s *Server) FilesPublished(w http.ResponseWriter, r *http.Request) {
	v := s.View(r, "Files")
	url, hint := s.filesAttachmentsShortcut(SessionFrom(r))
	v.Data = filesView{
		Sources:         s.fileCollections(SessionFrom(r)),
		PublishedActive: true,
		AttachmentsURL:  url,
		AttachmentsHint: hint,
	}
	s.Render(w, "files_published.html", v)
}
