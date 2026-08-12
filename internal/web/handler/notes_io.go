// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/account"
	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/model"
	"gitea.mixdep.ru/mix/carrel/internal/provider/calendar"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

// NoteExport downloads one note as a Markdown file (§23.9).
func (s *Server) NoteExport(w http.ResponseWriter, r *http.Request) {
	accountID, colEnc, uid := r.PathValue("account"), r.PathValue("col"), r.PathValue("uid")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil || uid == "" {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	p, acc, err := s.calendarProvider(sess, accountID)
	if err != nil {
		http.Error(w, userFacingDAVError(err), http.StatusBadRequest)
		return
	}
	col, err := findCalendar(acc, collection)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	collection = normalizeCollectionPath(col.Path)
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	obj, err := p.Get(ctx, collection, calendarObjectPath(collection, uid))
	if err != nil {
		http.Error(w, userFacingDAVError(err), http.StatusBadGateway)
		return
	}
	note, err := obj.Note(s.timezone())
	if err != nil {
		http.Error(w, "that object is not a note", http.StatusBadRequest)
		return
	}
	src := model.MarkdownSource{Account: accountLabel(*acc), Collection: collectionLabel(col)}
	body := model.RenderMarkdown(note, src)
	filename := model.MarkdownFilename(note, nil)
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", contentDisposition(filename))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(body)
}

// NotesExport streams the notes of a collection as a zip of Markdown files.
//
// The archive is written straight to the response as it is built: §23.9 asks for
// no temporary files on the server, and a journal of a few thousand notes has no
// business being assembled in memory first.
func (s *Server) NotesExport(w http.ResponseWriter, r *http.Request) {
	accountID, colEnc := r.PathValue("account"), r.PathValue("col")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	p, acc, err := s.calendarProvider(sess, accountID)
	if err != nil {
		http.Error(w, userFacingDAVError(err), http.StatusBadRequest)
		return
	}
	col, err := findCalendar(acc, collection)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	collection = normalizeCollectionPath(col.Path)
	loc := s.timezone()
	from, to, err := optionalDateRange(r, loc)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	set, err := p.QueryComponent(ctx, collection, dav.CompJournal, time.Time{}, time.Time{})
	if err != nil {
		http.Error(w, userFacingDAVError(err), http.StatusBadGateway)
		return
	}
	src := model.MarkdownSource{Account: accountLabel(*acc), Collection: collectionLabel(col)}
	notes := make([]model.Note, 0, len(set.Objects))
	for _, obj := range set.Objects {
		note, noteErr := obj.Note(loc)
		if noteErr != nil {
			continue
		}
		if !withinRange(note.Date, from, to) {
			continue
		}
		notes = append(notes, note)
	}
	sort.SliceStable(notes, func(i, j int) bool { return notes[i].Date.Before(notes[j].Date) })

	// Headers go out before the first entry: once the archive starts flowing
	// there is no way back to a status code.
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", contentDisposition(notesArchiveName(col, from, to)))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	zw := zip.NewWriter(w)
	taken := make(map[string]bool, len(notes))
	for _, note := range notes {
		name := model.MarkdownFilename(note, taken)
		entry, err := zw.CreateHeader(&zip.FileHeader{
			Name: name, Method: zip.Deflate, Modified: note.Date,
		})
		if err != nil {
			s.logError("export notes", err)
			break
		}
		if _, err := entry.Write(model.RenderMarkdown(note, src)); err != nil {
			s.logError("export notes", err)
			break
		}
	}
	if err := zw.Close(); err != nil {
		s.logError("export notes", err)
	}
}

func notesArchiveName(col discovery.Collection, from, to time.Time) string {
	stem := "notes"
	if name := strings.TrimSpace(col.DisplayName); name != "" {
		stem = name
	}
	parts := []string{sanitizeFilename(stem)}
	if !from.IsZero() {
		parts = append(parts, from.Format("2006-01-02"))
	}
	if !to.IsZero() {
		parts = append(parts, to.Format("2006-01-02"))
	}
	return strings.Join(parts, "-") + ".zip"
}

func collectionLabel(col discovery.Collection) string {
	if name := strings.TrimSpace(col.DisplayName); name != "" {
		return name
	}
	return col.Path
}

func contentDisposition(filename string) string {
	safe := sanitizeFilename(filename)
	if safe == "" {
		safe = "download"
	}
	return fmt.Sprintf("attachment; filename=%q", safe)
}

// optionalDateRange reads from/to query parameters, both optional.
func optionalDateRange(r *http.Request, loc *time.Location) (time.Time, time.Time, error) {
	var from, to time.Time
	if v := strings.TrimSpace(r.URL.Query().Get("from")); v != "" {
		parsed, err := time.ParseInLocation("2006-01-02", v, loc)
		if err != nil {
			return from, to, fmt.Errorf("from must be a YYYY-MM-DD date")
		}
		from = parsed
	}
	if v := strings.TrimSpace(r.URL.Query().Get("to")); v != "" {
		parsed, err := time.ParseInLocation("2006-01-02", v, loc)
		if err != nil {
			return from, to, fmt.Errorf("to must be a YYYY-MM-DD date")
		}
		to = parsed.AddDate(0, 0, 1)
	}
	if !from.IsZero() && !to.IsZero() && to.Before(from) {
		return from, to, fmt.Errorf("to must not precede from")
	}
	return from, to, nil
}

func withinRange(at, from, to time.Time) bool {
	if at.IsZero() {
		return from.IsZero() && to.IsZero()
	}
	if !from.IsZero() && at.Before(from) {
		return false
	}
	if !to.IsZero() && !at.Before(to) {
		return false
	}
	return true
}

type notesImportView struct {
	Sources        []sourceRow
	AccountID      string
	ColEnc         string
	Collection     discovery.Collection
	AccountLabel   string
	DraftKey       string
	Notes          []notesImportRow
	OKCount        int
	ErrorCount     int
	CollisionCount int
	TruncatedNote  string
	HasPreview     bool
	// WebDAVSources are the file collections a folder of Markdown can be read
	// from instead of uploading one (§23.9 block B8).
	WebDAVSources []markdownImportSource
}

type notesImportRow struct {
	Source, Title, Date, OriginalUID, ParseError string
	Tags                                         []string
	UIDCollision                                 bool
}

type notesImportReportView struct {
	Sources      []sourceRow
	AccountID    string
	ColEnc       string
	Collection   discovery.Collection
	AccountLabel string
	Created      int
	Failed       []string
	Collisions   []string
}

// NotesImport previews and confirms a Markdown import (§23.9).
//
// The target collection is chosen explicitly and never defaulted: this is the
// one operation in the notes screen that is massive and not undoable by ordinary
// means, and §23.9 makes that the exception to "do not ask every time".
func (s *Server) NotesImport(w http.ResponseWriter, r *http.Request) {
	accountID, colEnc := r.PathValue("account"), r.PathValue("col")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	p, acc, err := s.calendarProvider(sess, accountID)
	if err != nil {
		s.renderNotesError(w, r, err, accountID, colEnc)
		return
	}
	col, err := findCalendar(acc, collection)
	if err != nil {
		s.renderNotesError(w, r, err, accountID, colEnc)
		return
	}
	if col.ReadOnly {
		http.Error(w, "this notes collection is read-only", http.StatusForbidden)
		return
	}
	collection = normalizeCollectionPath(col.Path)
	key := "notes-" + importDraftKey(accountID, collection)
	base := notesImportView{
		Sources: s.noteSourcesOrNil(sess), AccountID: accountID, ColEnc: colEnc,
		Collection: col, AccountLabel: accountLabel(*acc), DraftKey: key,
		WebDAVSources: s.webdavMarkdownSources(sess),
	}
	if r.Method == http.MethodGet {
		s.renderNotesImport(w, r, base)
		return
	}
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/") {
		s.previewNotesImport(w, r, sess, p, col, base, collection)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	switch r.FormValue("action") {
	case "cancel_import":
		sess.ClearImport(key)
		http.Redirect(w, r, s.Path("/app/notes/"+accountID+"/"+colEnc), http.StatusSeeOther)
	case "confirm_import":
		s.confirmNotesImport(w, r, sess, p, col, base, collection)
	case "webdav_import":
		// The same preview, read from a folder on the person's own WebDAV rather
		// than from an upload (§23.9 block B8). Downloading a directory of notes
		// only to upload it again is work the server can spare them.
		s.previewWebDAVImport(w, r, sess, p, col, base, collection)
	default:
		http.Error(w, "bad request", http.StatusBadRequest)
	}
}

func (s *Server) renderNotesImport(w http.ResponseWriter, r *http.Request, data notesImportView) {
	v := s.View(r, "Import notes")
	v.Data = data
	s.Render(w, "notes_import.html", v)
}

func (s *Server) previewNotesImport(w http.ResponseWriter, r *http.Request, sess *session.Session, p *calendar.Provider, col discovery.Collection, view notesImportView, collection string) {
	maxBytes := s.importMaxBytes()
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	if err := r.ParseMultipartForm(maxBytes); err != nil {
		http.Error(w, "upload too large or invalid", http.StatusBadRequest)
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "choose a .md file or a .zip of them", http.StatusBadRequest)
		return
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, maxBytes))
	if err != nil {
		http.Error(w, "could not read upload", http.StatusBadRequest)
		return
	}
	filename := ""
	if hdr != nil {
		filename = path.Base(hdr.Filename)
	}
	maxNotes := s.Import.MaxCards
	if maxNotes <= 0 {
		maxNotes = 5000
	}
	parsed, truncErr := model.ReadMarkdownImportPayload(filename, body, maxNotes)
	if truncErr != nil {
		view.TruncatedNote = truncErr.Error()
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	existing, err := s.existingUIDs(ctx, p, collection)
	if err != nil {
		s.renderNotesError(w, r, err, view.AccountID, view.ColEnc)
		return
	}
	loc := s.timezone()
	draft := session.ImportDraft{Key: view.DraftKey, AccountID: view.AccountID, Collection: collection}
	view.HasPreview = true
	for _, item := range parsed {
		row := notesImportRow{Source: item.Source, ParseError: item.Error}
		card := session.ImportCard{Source: item.Source, ParseError: item.Error}
		if item.Error == "" {
			note := item.Note
			row.Title, row.Tags, row.OriginalUID = note.Title, note.SortedTags(), note.UID
			if note.HasDate {
				row.Date = note.Date.In(loc).Format("2006-01-02")
			}
			row.UIDCollision = note.UID != "" && existing[note.UID]
			if row.UIDCollision {
				view.CollisionCount++
			}
			raw, buildErr := buildJournalBody(note, loc)
			if buildErr != nil {
				row.ParseError, card.ParseError = buildErr.Error(), buildErr.Error()
			} else {
				card.Body, card.OriginalUID = raw, note.UID
				card.DisplayName, card.UIDCollision = displayOr(note.Title, item.Source), row.UIDCollision
				view.OKCount++
			}
		}
		if row.ParseError != "" {
			view.ErrorCount++
		}
		draft.Cards = append(draft.Cards, card)
		view.Notes = append(view.Notes, row)
	}
	sess.PutImport(draft)
	s.renderNotesImport(w, r, view)
}

// buildJournalBody turns a parsed Markdown file into a VJOURNAL body. It goes
// through NewJournal and Apply like every other write, so the import path cannot
// invent a serialisation of its own (§8).
func buildJournalBody(note model.MarkdownNote, loc *time.Location) ([]byte, error) {
	uid := strings.TrimSpace(note.UID)
	if uid == "" {
		fresh, err := model.NewUID()
		if err != nil {
			return nil, err
		}
		uid = fresh
	}
	obj, err := model.NewJournal(uid)
	if err != nil {
		return nil, err
	}
	if err := obj.Apply(note.Patch(loc)); err != nil {
		return nil, err
	}
	return obj.Marshal()
}

func (s *Server) confirmNotesImport(w http.ResponseWriter, r *http.Request, sess *session.Session, p *calendar.Provider, col discovery.Collection, view notesImportView, collection string) {
	draft, ok := sess.TakeImport(view.DraftKey)
	if !ok {
		http.Error(w, "no import in progress", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	existing, err := s.existingUIDs(ctx, p, collection)
	if err != nil {
		s.renderNotesError(w, r, err, view.AccountID, view.ColEnc)
		return
	}
	report := notesImportReportView{
		Sources: view.Sources, AccountID: view.AccountID, ColEnc: view.ColEnc,
		Collection: col, AccountLabel: view.AccountLabel,
	}
	for _, item := range draft.Cards {
		if item.ParseError != "" || len(item.Body) == 0 {
			continue
		}
		obj, err := model.ParseICal("import", "", item.Body)
		if err != nil {
			report.Failed = append(report.Failed, item.Source+": "+err.Error())
			continue
		}
		uid := strings.TrimSpace(obj.UID())
		// Import always creates. A UID that is already here gets a new one and
		// a line in the report, because overwriting it would make a second run
		// destroy whatever was edited after the first (§23.9).
		if uid == "" || existing[uid] {
			fresh, uidErr := model.NewUID()
			if uidErr != nil {
				report.Failed = append(report.Failed, item.Source+": "+uidErr.Error())
				continue
			}
			if uid != "" {
				report.Collisions = append(report.Collisions, fmt.Sprintf("%s (%s → %s)", item.DisplayName, uid, fresh))
			}
			if err := obj.AssignUID(fresh); err != nil {
				report.Failed = append(report.Failed, item.Source+": "+err.Error())
				continue
			}
			uid = fresh
		}
		obj.Path = calendarObjectPath(collection, uid)
		if _, err := p.Create(ctx, collection, obj); err != nil {
			report.Failed = append(report.Failed, item.DisplayName+": "+userFacingDAVError(err))
			continue
		}
		existing[uid] = true
		report.Created++
	}
	s.rememberDefault(sess, account.ViewNotes, view.AccountID, collection)
	v := s.View(r, "Notes import report")
	v.Notice = fmt.Sprintf("Imported %d note(s).", report.Created)
	v.Data = report
	s.Render(w, "notes_import_report.html", v)
}

// existingUIDs is the set of object identities already in a collection, taken
// from the path map rather than by reading every body.
func (s *Server) existingUIDs(ctx context.Context, p *calendar.Provider, collection string) (map[string]bool, error) {
	listing, err := p.List(ctx, collection)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(listing.ETags))
	for _, objectPath := range listing.Paths() {
		out[uidFromCalendarPath(objectPath)] = true
	}
	return out, nil
}

func displayOr(primary, fallback string) string {
	if s := strings.TrimSpace(primary); s != "" {
		return s
	}
	return fallback
}
