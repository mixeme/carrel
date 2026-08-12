// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/model"
	"gitea.mixdep.ru/mix/carrel/internal/provider/calendar"
	"gitea.mixdep.ru/mix/carrel/internal/provider/files"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

// markdownImportSource is one folder of Markdown files offered as an import
// source, so a directory of notes already on the person's own WebDAV does not
// have to be downloaded and uploaded again to get into a collection (§23.9).
type markdownImportSource struct {
	AccountID string
	ColEnc    string
	Label     string
	Rel       string
}

// webdavMarkdownSources lists the file collections that could hold notes. It
// does not go looking through them: a listing per collection on every visit to
// the import screen would be a fan-out nobody asked for. The person names the
// folder and Carrel reads that one.
func (s *Server) webdavMarkdownSources(sess *session.Session) []markdownImportSource {
	rows := s.fileCollections(sess)
	out := make([]markdownImportSource, 0, len(rows))
	for _, row := range rows {
		out = append(out, markdownImportSource{
			AccountID: row.AccountID, ColEnc: row.ColEnc,
			Label: row.Label() + " · " + row.AccountLabel,
		})
	}
	return out
}

// maxWebDAVImportFiles caps one folder read, on the same reasoning as the entry
// ceiling of the file browser.
const maxWebDAVImportFiles = 500

// previewWebDAVImport reads a folder of `.md` files from a file collection and
// previews them exactly as an uploaded zip is previewed.
//
// The files are read one at a time and each is capped at the import ceiling, so
// a folder somebody filled with a gigabyte of text costs the ceiling and not the
// gigabyte.
func (s *Server) previewWebDAVImport(w http.ResponseWriter, r *http.Request, sess *session.Session, p *calendar.Provider, col discovery.Collection, view notesImportView, collection string) {
	sourceKey := strings.TrimSpace(r.PostFormValue("webdav_source"))
	folder, err := files.CleanRelative(r.PostFormValue("webdav_folder"))
	if err != nil {
		s.renderNotesError(w, r, fmt.Errorf("that folder name cannot be used"), view.AccountID, view.ColEnc)
		return
	}
	var source markdownImportSource
	for _, candidate := range s.webdavMarkdownSources(sess) {
		if candidate.AccountID+"|"+candidate.ColEnc == sourceKey {
			source = candidate
			break
		}
	}
	if source.AccountID == "" {
		s.renderNotesError(w, r, fmt.Errorf("choose a file collection to read from"), view.AccountID, view.ColEnc)
		return
	}
	root, err := DecodeCollectionPath(source.ColEnc)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	fp, acc, err := s.filesProvider(sess, source.AccountID)
	if err != nil {
		s.renderNotesError(w, r, err, view.AccountID, view.ColEnc)
		return
	}
	if _, err := findFileCollection(acc, root); err != nil {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	listing, err := fp.List(ctx, root, folder)
	if err != nil {
		s.renderNotesError(w, r, err, view.AccountID, view.ColEnc)
		return
	}

	existing, err := s.existingUIDs(ctx, p, collection)
	if err != nil {
		s.renderNotesError(w, r, err, view.AccountID, view.ColEnc)
		return
	}
	maxBytes := s.importMaxBytes()
	loc := s.timezone()
	draft := session.ImportDraft{Key: view.DraftKey, AccountID: view.AccountID, Collection: collection}
	view.HasPreview = true
	read := 0
	for _, entry := range listing.Entries {
		if entry.Dir || !model.IsMarkdownName(entry.Name) {
			continue
		}
		if read >= maxWebDAVImportFiles {
			view.TruncatedNote = fmt.Sprintf("Only the first %d files in that folder were read.", maxWebDAVImportFiles)
			break
		}
		read++
		row, card := s.readWebDAVNote(ctx, fp, root, entry, maxBytes, loc, existing)
		if row.UIDCollision {
			view.CollisionCount++
		}
		if row.ParseError != "" {
			view.ErrorCount++
		} else if len(card.Body) > 0 {
			view.OKCount++
		}
		draft.Cards = append(draft.Cards, card)
		view.Notes = append(view.Notes, row)
	}
	if read == 0 && view.TruncatedNote == "" {
		view.TruncatedNote = "No Markdown files in that folder."
	}
	sess.PutImport(draft)
	s.renderNotesImport(w, r, view)
}

func (s *Server) readWebDAVNote(ctx context.Context, fp *files.Provider, root string, entry files.Entry, maxBytes int64, loc *time.Location, existing map[string]bool) (notesImportRow, session.ImportCard) {
	row := notesImportRow{Source: entry.Rel}
	card := session.ImportCard{Source: entry.Rel}
	download, err := fp.Open(ctx, root, entry.Rel, nil)
	if err != nil {
		row.ParseError = userFacingDAVError(err)
		card.ParseError = row.ParseError
		return row, card
	}
	defer download.Body.Close()
	body, err := io.ReadAll(io.LimitReader(download.Body, maxBytes))
	if err != nil {
		row.ParseError = "could not read the file"
		card.ParseError = row.ParseError
		return row, card
	}
	note, err := model.ParseMarkdown(entry.Name, body, entry.ModTime)
	if err != nil {
		row.ParseError = err.Error()
		card.ParseError = row.ParseError
		return row, card
	}
	row.Title, row.Tags, row.OriginalUID = note.Title, note.SortedTags(), note.UID
	if note.HasDate {
		row.Date = note.Date.In(loc).Format("2006-01-02")
	}
	row.UIDCollision = note.UID != "" && existing[note.UID]
	raw, err := buildJournalBody(note, loc)
	if err != nil {
		row.ParseError = err.Error()
		card.ParseError = row.ParseError
		return row, card
	}
	card.Body, card.OriginalUID = raw, note.UID
	card.DisplayName, card.UIDCollision = displayOr(note.Title, entry.Rel), row.UIDCollision
	return row, card
}
