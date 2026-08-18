// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-ical"

	"gitea.mixdep.ru/mix/carrel/internal/account"
	"gitea.mixdep.ru/mix/carrel/internal/dav"
	"gitea.mixdep.ru/mix/carrel/internal/dav/discovery"
	"gitea.mixdep.ru/mix/carrel/internal/model"
	"gitea.mixdep.ru/mix/carrel/internal/provider/calendar"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

type notesView struct {
	Sources      []sourceRow
	AccountID    string
	ColEnc       string
	Collection   discovery.Collection
	AccountLabel string
	Tag          string
	Tags         []string
	Rows         []noteRow
	ReadOnly     bool
	Empty        bool
	NoLists      bool
	PrintDate    string
}

type noteRow struct {
	UID        string
	Title      string
	Excerpt    string
	DateLabel  string
	Date       string
	Tags       []string
	HasRelated bool
}

// NotesHome redirects to the collection notes were last written to, or shows
// the empty state.
func (s *Server) NotesHome(w http.ResponseWriter, r *http.Request) {
	sess := SessionFrom(r)
	rows, err := s.noteSources(sess)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if len(rows) == 0 {
		v := s.View(r, "Notes")
		v.Data = notesView{NoLists: true}
		s.Render(w, "notes.html", v)
		return
	}
	target := rows[0]
	if preferred, ok := s.defaultCollection(sess, account.ViewNotes, rows); ok {
		target = preferred
	}
	http.Redirect(w, r, s.Path("/app/notes/"+target.AccountID+"/"+target.ColEnc), http.StatusSeeOther)
}

// NotesList renders the VJOURNALs of one collection, newest first.
func (s *Server) NotesList(w http.ResponseWriter, r *http.Request) {
	accountID, colEnc := r.PathValue("account"), r.PathValue("col")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	view, err := s.buildNotes(r.Context(), sess, accountID, collection, colEnc, r.URL.Query().Get("tag"))
	if err != nil {
		s.renderNotesError(w, r, err, accountID, colEnc)
		return
	}
	v := s.View(r, "Notes")
	v.Notice = strings.TrimSpace(r.URL.Query().Get("notice"))
	v.Data = view
	s.Render(w, "notes.html", v)
}

func (s *Server) buildNotes(ctx context.Context, sess *session.Session, accountID, collection, colEnc, tag string) (notesView, error) {
	p, acc, err := s.calendarProvider(sess, accountID)
	if err != nil {
		return notesView{}, err
	}
	col, err := findCalendar(acc, collection)
	if err != nil {
		return notesView{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	set, err := p.QueryComponent(ctx, normalizeCollectionPath(col.Path), dav.CompJournal, time.Time{}, time.Time{})
	if err != nil {
		return notesView{}, err
	}
	loc := s.timezone()
	notes := make([]model.Note, 0, len(set.Objects))
	for _, obj := range set.Objects {
		note, noteErr := obj.Note(loc)
		if noteErr != nil {
			continue
		}
		notes = append(notes, note)
	}
	// §23.9 sorts notes by date; newest first is the order a journal is read in.
	sort.SliceStable(notes, func(i, j int) bool {
		if notes[i].Date.Equal(notes[j].Date) {
			return strings.ToLower(notes[i].DisplayTitle()) < strings.ToLower(notes[j].DisplayTitle())
		}
		return notes[i].Date.After(notes[j].Date)
	})

	view := notesView{
		AccountID: accountID, ColEnc: colEnc, Collection: col,
		AccountLabel: accountLabel(*acc), ReadOnly: col.ReadOnly,
		Tag: strings.TrimSpace(tag), PrintDate: time.Now().UTC().Format("2006-01-02 15:04 UTC"),
	}
	if rows, listErr := s.noteSources(sess); listErr == nil {
		view.Sources = rows
	}
	tags := make(map[string]bool)
	for _, note := range notes {
		for _, t := range note.Categories {
			tags[t] = true
		}
		if view.Tag != "" && !hasTag(note.Categories, view.Tag) {
			continue
		}
		row := noteRow{
			UID: note.UID, Title: note.DisplayTitle(), Excerpt: note.Excerpt(180),
			Tags: note.Categories, HasRelated: len(note.Related) > 0,
		}
		if !note.Date.IsZero() {
			local := note.Date.In(loc)
			row.Date = local.Format("2006-01-02")
			row.DateLabel = local.Format("2 Jan 2006")
			if !note.DateOnly {
				row.DateLabel += " " + local.Format("15:04")
			}
		}
		view.Rows = append(view.Rows, row)
	}
	for t := range tags {
		view.Tags = append(view.Tags, t)
	}
	sort.Strings(view.Tags)
	view.Empty = len(view.Rows) == 0
	return view, nil
}

func hasTag(tags []string, want string) bool {
	for _, tag := range tags {
		if strings.EqualFold(tag, want) {
			return true
		}
	}
	return false
}

func (s *Server) noteSources(sess *session.Session) ([]sourceRow, error) {
	return s.collectionsOfKind(sess, discovery.KindCalendar, account.ViewNotes, dav.CompJournal)
}

func (s *Server) noteSourcesOrNil(sess *session.Session) []sourceRow {
	rows, err := s.noteSources(sess)
	if err != nil {
		return nil
	}
	return rows
}

type noteCardView struct {
	Sources      []sourceRow
	AccountID    string
	ColEnc       string
	Collection   discovery.Collection
	AccountLabel string
	UID          string
	ETag         string
	Note         model.Note
	Form         noteForm
	Related      []relatedRow
	// Attachments are the pictures and files of §23.10 — the one thing a note
	// could not carry before this stage.
	Attachments []attachmentRow
	// CanAttach says the folder of §23.10 has been chosen. Without it the card
	// says where to choose one rather than offering a form that cannot work.
	CanAttach bool
	// Section is the URL segment the shared attachments block posts to.
	Section   string
	ReadOnly  bool
	IsNew     bool
	PrintDate string
}

// relatedRow is one RELATED-TO target resolved to something clickable, or left
// as a bare UID when the other object is not in this collection.
type relatedRow struct {
	UID     string
	Kind    string
	Title   string
	URL     string
	RelType string
}

type noteForm struct {
	Summary     string
	Description string
	Date        string
	Time        string
	Categories  string
	Related     string
}

// NoteQuick is the two-second path of §23.9: one field, reachable from every
// screen through the navigation bar, filing into the collection last used.
func (s *Server) NoteQuick(w http.ResponseWriter, r *http.Request) {
	sess := SessionFrom(r)
	rows, err := s.noteSources(sess)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	writable := make([]sourceRow, 0, len(rows))
	for _, row := range rows {
		if !row.ReadOnly {
			writable = append(writable, row)
		}
	}
	target, hasTarget := s.defaultCollection(sess, account.ViewNotes, rows)
	back := s.quickNoteBack(r)
	fullNoteURL := ""
	if hasTarget {
		fullNoteURL = s.Path("/app/notes/" + target.AccountID + "/" + target.ColEnc + "/new")
	}
	if r.Method == http.MethodGet {
		v := s.View(r, "New note")
		v.Data = quickNoteView{
			Sources: writable, Target: target, HasTarget: hasTarget, Back: back,
			Overlay: IsHTMX(r), FullNoteURL: fullNoteURL,
		}
		if IsHTMX(r) {
			s.RenderFragment(w, "note_quick.html", v)
			return
		}
		s.Render(w, "note_quick.html", v)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if !hasTarget {
		http.Error(w, "no writable notes collection is connected", http.StatusBadRequest)
		return
	}
	// A chosen collection overrides the default, and becomes the new default.
	if key := strings.TrimSpace(r.PostFormValue("source")); key != "" {
		for _, row := range writable {
			if row.Key() == key {
				target = row
				break
			}
		}
	}
	body := strings.TrimSpace(r.PostFormValue("body"))
	if body == "" {
		v := s.View(r, "New note")
		v.Error = "A note needs some text."
		v.Data = quickNoteView{
			Sources: writable, Target: target, HasTarget: hasTarget, Back: back,
			Overlay: IsHTMX(r), FullNoteURL: fullNoteURL,
		}
		s.renderQuickNote(w, r, http.StatusBadRequest, v)
		return
	}
	uid, err := s.createNote(r.Context(), sess, target, noteForm{Description: body})
	if err != nil {
		v := s.View(r, "New note")
		v.Error = userFacingDAVError(err)
		v.Data = quickNoteView{
			Sources: writable, Target: target, HasTarget: hasTarget, Back: back,
			Overlay: IsHTMX(r), FullNoteURL: fullNoteURL,
		}
		s.renderQuickNote(w, r, http.StatusBadGateway, v)
		return
	}
	s.rememberDefault(sess, account.ViewNotes, target.AccountID, target.Path)
	s.redirectNotice(w, r, s.Path("/app/notes/"+target.AccountID+"/"+target.ColEnc+"/"+urlPathEscape(uid)), "Note saved.")
}

type quickNoteView struct {
	Sources     []sourceRow
	Target      sourceRow
	HasTarget   bool
	Back        string
	Overlay     bool
	FullNoteURL string
}

func (s *Server) quickNoteBack(r *http.Request) string {
	if r.Method == http.MethodPost {
		return SafeRedirect(r.FormValue("back"), s.Path("/app/notes"))
	}
	return SafeRedirect(r.URL.Query().Get("back"), s.Path("/app/notes"))
}

func (s *Server) renderQuickNote(w http.ResponseWriter, r *http.Request, status int, v View) {
	if IsHTMX(r) {
		if status != http.StatusOK {
			w.WriteHeader(status)
		}
		s.RenderFragment(w, "note_quick.html", v)
		return
	}
	s.RenderStatus(w, status, "note_quick.html", v)
}

// createNote writes one note and returns its UID.
func (s *Server) createNote(ctx context.Context, sess *session.Session, target sourceRow, form noteForm) (string, error) {
	p, acc, err := s.calendarProvider(sess, target.AccountID)
	if err != nil {
		return "", err
	}
	col, err := findCalendar(acc, target.Path)
	if err != nil {
		return "", err
	}
	if col.ReadOnly {
		return "", fmt.Errorf("this notes collection is read-only")
	}
	collection := normalizeCollectionPath(col.Path)
	uid, err := model.NewUID()
	if err != nil {
		return "", err
	}
	obj, err := model.NewJournal(uid)
	if err != nil {
		return "", err
	}
	patch, err := form.toPatch(s.timezone())
	if err != nil {
		return "", err
	}
	if err := obj.Apply(patch); err != nil {
		return "", err
	}
	obj.Path = calendarObjectPath(collection, uid)
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if _, err := p.Create(ctx, collection, obj); err != nil {
		return "", err
	}
	return uid, nil
}

// NoteNew shows and takes the full note form.
func (s *Server) NoteNew(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.noteSave(w, r, true)
		return
	}
	accountID, colEnc := r.PathValue("account"), r.PathValue("col")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	_, acc, err := s.calendarProvider(sess, accountID)
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
	now := time.Now().In(s.timezone())
	form := noteForm{Date: now.Format("2006-01-02")}
	// A note created from an event carries the link and the date of the
	// meeting it is the minutes of (§23.9).
	form.Related = strings.TrimSpace(r.URL.Query().Get("related"))
	if date := strings.TrimSpace(r.URL.Query().Get("date")); date != "" {
		if _, parseErr := time.Parse("2006-01-02", date); parseErr == nil {
			form.Date = date
		}
	}
	form.Summary = strings.TrimSpace(r.URL.Query().Get("summary"))
	v := s.View(r, "New note")
	v.Data = noteCardView{
		Sources: s.noteSourcesOrNil(sess), AccountID: accountID, ColEnc: colEnc,
		Collection: col, AccountLabel: accountLabel(*acc), IsNew: true, Form: form,
	}
	s.Render(w, "note.html", v)
}

// NoteCard shows one note and takes its edits.
func (s *Server) NoteCard(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.noteSave(w, r, false)
		return
	}
	accountID, colEnc, uid := r.PathValue("account"), r.PathValue("col"), r.PathValue("uid")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil || uid == "" {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	card, err := s.loadNoteCard(r.Context(), sess, accountID, collection, colEnc, uid)
	if err != nil {
		s.renderNotesError(w, r, err, accountID, colEnc)
		return
	}
	card.PrintDate = time.Now().UTC().Format("2006-01-02 15:04 UTC")
	v := s.View(r, card.Note.DisplayTitle())
	v.Notice = strings.TrimSpace(r.URL.Query().Get("notice"))
	v.Data = card
	s.Render(w, "note.html", v)
}

// resolveRelated looks each RELATED-TO up in the same collection. A link whose
// target is elsewhere or gone is shown as the bare UID rather than dropped: the
// property is still on the object and hiding it would misrepresent it (§23.9).
func (s *Server) resolveRelated(ctx context.Context, p *calendar.Provider, accountID, colEnc, collection string, relations []model.Relation) []relatedRow {
	out := make([]relatedRow, 0, len(relations))
	for _, rel := range relations {
		row := relatedRow{UID: rel.UID, RelType: rel.RelType, Title: rel.UID, Kind: "unknown"}
		obj, err := p.Get(ctx, collection, calendarObjectPath(collection, rel.UID))
		if err == nil {
			switch obj.Component() {
			case "VEVENT":
				row.Kind, row.URL = "event", s.Path("/app/calendar/"+accountID+"/"+colEnc+"/"+urlPathEscape(rel.UID))
			case "VTODO":
				row.Kind, row.URL = "task", s.Path("/app/tasks/"+accountID+"/"+colEnc+"/"+urlPathEscape(rel.UID))
			case "VJOURNAL":
				row.Kind, row.URL = "note", s.Path("/app/notes/"+accountID+"/"+colEnc+"/"+urlPathEscape(rel.UID))
			}
			if title := s.icalTitle(obj); title != "" {
				row.Title = title
			}
		}
		out = append(out, row)
	}
	return out
}

func (s *Server) noteSave(w http.ResponseWriter, r *http.Request, isNew bool) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	accountID, colEnc, uid := r.PathValue("account"), r.PathValue("col"), r.PathValue("uid")
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
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	if r.PostFormValue(fieldAction) == "delete" {
		err = p.Delete(ctx, collection, calendarObjectPath(collection, uid), strings.TrimSpace(r.PostFormValue("etag")))
		if err != nil {
			s.handleNoteWriteError(w, r, sess, err, accountID, collection, colEnc, uid)
			return
		}
		http.Redirect(w, r, s.Path("/app/notes/"+accountID+"/"+colEnc), http.StatusSeeOther)
		return
	}

	form := parseNoteForm(r)
	var obj *model.Object
	if isNew {
		uid, err = model.NewUID()
		if err == nil {
			obj, err = model.NewJournal(uid)
		}
	} else {
		obj, err = p.Get(ctx, collection, calendarObjectPath(collection, uid))
		if err == nil && strings.TrimSpace(r.PostFormValue("etag")) != "" {
			obj.ETag = strings.TrimSpace(r.PostFormValue("etag"))
		}
	}
	if err != nil {
		s.renderNotesError(w, r, err, accountID, colEnc)
		return
	}
	patch, err := form.toPatch(s.timezone())
	if err == nil {
		err = obj.Apply(patch)
	}
	if err != nil {
		note, _ := obj.Note(s.timezone())
		v := s.View(r, map[bool]string{true: "New note", false: "Note"}[isNew])
		v.Error = capitalize(err.Error())
		v.Data = noteCardView{
			Sources: s.noteSourcesOrNil(sess), AccountID: accountID, ColEnc: colEnc,
			Collection: col, AccountLabel: accountLabel(*acc), UID: note.UID,
			ETag: obj.ETag, Note: note, Form: form, IsNew: isNew,
		}
		s.RenderStatus(w, http.StatusBadRequest, "note.html", v)
		return
	}
	var result *calendar.WriteResult
	if isNew {
		obj.Path = calendarObjectPath(collection, uid)
		result, err = p.Create(ctx, collection, obj)
	} else {
		result, err = p.Update(ctx, collection, obj)
	}
	if err != nil {
		s.handleNoteWriteError(w, r, sess, err, accountID, collection, colEnc, uid)
		return
	}
	s.rememberDefault(sess, account.ViewNotes, accountID, collection)
	notice := "Note saved."
	if result != nil && result.ReportLoss && !result.Loss.Empty() {
		notice = "Note saved, but " + result.Loss.String() + "."
	}
	s.redirectNotice(w, r, s.Path("/app/notes/"+accountID+"/"+colEnc+"/"+urlPathEscape(uid)), notice)
}

func (s *Server) handleNoteWriteError(w http.ResponseWriter, r *http.Request, sess *session.Session, err error, accountID, collection, colEnc, uid string) {
	if calendar.IsConflict(err) {
		s.showICalConflict(w, r, sess, sectionNotes, accountID, collection, colEnc, uid, err)
		return
	}
	s.renderNotesError(w, r, err, accountID, colEnc)
}

func parseNoteForm(r *http.Request) noteForm {
	return noteForm{
		Summary:     strings.TrimSpace(r.PostFormValue("summary")),
		Description: strings.TrimSpace(r.PostFormValue("description")),
		Date:        strings.TrimSpace(r.PostFormValue("date")),
		Time:        strings.TrimSpace(r.PostFormValue("time")),
		Categories:  strings.TrimSpace(r.PostFormValue("categories")),
		Related:     strings.TrimSpace(r.PostFormValue("related")),
	}
}

func (f noteForm) toPatch(loc *time.Location) (*model.Patch, error) {
	if loc == nil {
		loc = time.Local
	}
	if strings.TrimSpace(f.Summary) == "" && strings.TrimSpace(f.Description) == "" {
		return nil, fmt.Errorf("a note needs a title or some text")
	}
	p := &model.Patch{}
	setTextOrRemove(p, ical.PropSummary, f.Summary)
	setTextOrRemove(p, ical.PropDescription, f.Description)
	setTextOrRemove(p, ical.PropCategories, cleanCommaList(f.Categories))

	date := f.Date
	if date == "" {
		date = time.Now().In(loc).Format("2006-01-02")
	}
	if f.Time == "" {
		day, err := time.ParseInLocation("2006-01-02", date, loc)
		if err != nil {
			return nil, fmt.Errorf("date must use YYYY-MM-DD")
		}
		p.Set(ical.PropDateTimeStart, dateValue(day))
	} else {
		moment, err := time.ParseInLocation("2006-01-02 15:04", date+" "+f.Time, loc)
		if err != nil {
			return nil, fmt.Errorf("date and time are invalid")
		}
		p.Set(ical.PropDateTimeStart, model.Value{
			Text:   moment.Format("20060102T150405"),
			Params: map[string][]string{"TZID": {loc.String()}},
		})
	}
	if values := model.RelationValues(model.ParseRelations(f.Related)); len(values) > 0 {
		p.Set(ical.PropRelatedTo, values...)
	} else {
		p.Remove(ical.PropRelatedTo)
	}
	p.SetText(ical.PropDateTimeStamp, time.Now().UTC().Format("20060102T150405Z"))
	p.SetText(ical.PropLastModified, time.Now().UTC().Format("20060102T150405Z"))
	return p, nil
}

func formFromNote(note model.Note, loc *time.Location) noteForm {
	f := noteForm{
		Summary: note.Summary, Description: note.Description,
		Categories: strings.Join(note.Categories, ", "),
		Related:    strings.Join(model.RelationUIDs(note.Related), ", "),
	}
	if !note.Date.IsZero() {
		local := note.Date.In(loc)
		f.Date = local.Format("2006-01-02")
		if !note.DateOnly {
			f.Time = local.Format("15:04")
		}
	}
	return f
}

func (s *Server) renderNotesError(w http.ResponseWriter, r *http.Request, err error, accountID, colEnc string) {
	v := s.View(r, "Notes")
	v.Error = userFacingDAVError(err)
	v.Data = notesView{Sources: s.noteSourcesOrNil(SessionFrom(r)), AccountID: accountID, ColEnc: colEnc}
	s.RenderStatus(w, http.StatusBadRequest, "notes.html", v)
}
