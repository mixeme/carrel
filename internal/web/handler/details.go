// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"context"
	"net/http"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/session"
)

// ContactPanel returns a read-only contact card for the details column.
func (s *Server) ContactPanel(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	colEnc := r.PathValue("col")
	uid := r.PathValue("uid")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil || uid == "" {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	card, err := s.loadContactCard(r.Context(), sess, accountID, collection, colEnc, uid)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	v := s.View(r, card.Contact.DisplayName())
	v.Data = card
	s.RenderFragment(w, "contact_panel.html", v)
}

// EventPanel returns a read-only event card for the details column.
func (s *Server) EventPanel(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	colEnc := r.PathValue("col")
	uid := r.PathValue("uid")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil || uid == "" {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	card, err := s.loadEventCard(r.Context(), sess, accountID, collection, colEnc, uid)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	v := s.View(r, card.Event.DisplayTitle())
	v.Data = card
	s.RenderFragment(w, "event_panel.html", v)
}

// TaskPanel returns a read-only task card for the details column.
func (s *Server) TaskPanel(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	colEnc := r.PathValue("col")
	uid := r.PathValue("uid")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil || uid == "" {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	card, err := s.loadTaskCard(r.Context(), sess, accountID, collection, colEnc, uid)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	v := s.View(r, card.Task.DisplayTitle())
	v.Data = card
	s.RenderFragment(w, "task_panel.html", v)
}

// NotePanel returns a read-only note card for the details column.
func (s *Server) NotePanel(w http.ResponseWriter, r *http.Request) {
	accountID := r.PathValue("account")
	colEnc := r.PathValue("col")
	uid := r.PathValue("uid")
	collection, err := DecodeCollectionPath(colEnc)
	if err != nil || uid == "" {
		http.NotFound(w, r)
		return
	}
	sess := SessionFrom(r)
	card, err := s.loadNoteCard(r.Context(), sess, accountID, collection, colEnc, uid)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	v := s.View(r, card.Note.DisplayTitle())
	v.Data = card
	s.RenderFragment(w, "note_panel.html", v)
}

func (s *Server) loadTaskCard(ctx context.Context, sess *session.Session, accountID, collection, colEnc, uid string) (taskCardView, error) {
	p, acc, err := s.calendarProvider(sess, accountID)
	if err != nil {
		return taskCardView{}, err
	}
	col, err := findCalendar(acc, collection)
	if err != nil {
		return taskCardView{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	obj, err := p.Get(ctx, normalizeCollectionPath(col.Path), calendarObjectPath(col.Path, uid))
	if err != nil {
		return taskCardView{}, err
	}
	task, err := obj.Todo(s.timezone())
	if err != nil {
		return taskCardView{}, err
	}
	return taskCardView{
		Sources: s.taskSources(sess), AccountID: accountID, ColEnc: colEnc,
		Collection: col, AccountLabel: accountLabel(*acc), UID: task.UID,
		ETag: obj.ETag, Task: task, Form: formFromTask(task, s.timezone()),
		Related:  s.resolveRelated(ctx, p, accountID, colEnc, normalizeCollectionPath(col.Path), task.Related),
		ReadOnly: col.ReadOnly,
		Source:   s.objectSource(sess, accountID, normalizeCollectionPath(col.Path), obj.Path, obj.ETag),
	}, nil
}

func (s *Server) loadNoteCard(ctx context.Context, sess *session.Session, accountID, collection, colEnc, uid string) (noteCardView, error) {
	p, acc, err := s.calendarProvider(sess, accountID)
	if err != nil {
		return noteCardView{}, err
	}
	col, err := findCalendar(acc, collection)
	if err != nil {
		return noteCardView{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	obj, err := p.Get(ctx, normalizeCollectionPath(col.Path), calendarObjectPath(col.Path, uid))
	if err != nil {
		return noteCardView{}, err
	}
	note, err := obj.Note(s.timezone())
	if err != nil {
		return noteCardView{}, err
	}
	card := noteCardView{
		Sources: s.noteSourcesOrNil(sess), AccountID: accountID, ColEnc: colEnc,
		Collection: col, AccountLabel: accountLabel(*acc), UID: note.UID,
		ETag: obj.ETag, Note: note, Form: formFromNote(note, s.timezone()),
		Related:     s.resolveRelated(ctx, p, accountID, colEnc, normalizeCollectionPath(col.Path), note.Related),
		Attachments: s.attachmentRows(sess, sectionNotes, accountID, colEnc, note.UID, note.Attachments),
		Section:     sectionNotes.Path,
		ReadOnly:    col.ReadOnly,
		Source:      s.objectSource(sess, accountID, col.Path, obj.Path, obj.ETag),
	}
	_, card.CanAttach = s.attachmentTarget(sess)
	return card, nil
}
