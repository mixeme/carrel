// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/model"
	"gitea.mixdep.ru/mix/carrel/internal/provider/calendar"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

// icalSection ties the three calendar screens to their route prefix and to what
// they call a record, so one conflict screen serves events, tasks and notes
// (§9). Nothing about resolving a 412 differs between them.
type icalSection struct {
	Path   string
	Entity string
	Title  string
}

var (
	sectionCalendar = icalSection{Path: "calendar", Entity: "event", Title: "Calendar conflict"}
	sectionTasks    = icalSection{Path: "tasks", Entity: "task", Title: "Task conflict"}
	sectionNotes    = icalSection{Path: "notes", Entity: "note", Title: "Note conflict"}
)

func (s *Server) showCalendarConflict(w http.ResponseWriter, r *http.Request, sess *session.Session, accountID, collection, colEnc, uid string, err error) {
	s.showICalConflict(w, r, sess, sectionCalendar, accountID, collection, colEnc, uid, err)
}

func (s *Server) showICalConflict(w http.ResponseWriter, r *http.Request, sess *session.Session, section icalSection, accountID, collection, colEnc, uid string, err error) {
	var conflict *calendar.ConflictError
	if !errors.As(err, &conflict) {
		s.renderSectionError(w, r, section, err, accountID, colEnc)
		return
	}
	key := conflictKey(accountID, collection, uid)
	draft := session.ConflictDraft{
		Key: key, AccountID: accountID, Collection: collection,
		Path: conflict.Path, UID: uid,
	}
	if conflict.Local != nil {
		draft.Body, _ = conflict.Local.Marshal()
		if draft.UID == "" {
			draft.UID = conflict.Local.UID()
			uid = draft.UID
		}
	}
	sess.PutConflict(draft)
	lines, _ := model.Diff(conflict.Local, conflict.Remote)
	localName, remoteName := "Your edit", "Server version"
	if name := s.icalTitle(conflict.Local); name != "" {
		localName = name
	}
	if name := s.icalTitle(conflict.Remote); name != "" {
		remoteName = name
	}
	v := s.View(r, section.Title)
	v.Error = "This " + section.Entity + " changed on the server since you opened it."
	v.Data = conflictView{
		AccountID: accountID, ColEnc: colEnc, UID: uid, Key: key, Lines: lines,
		LocalName: localName, RemoteName: remoteName, RemoteGone: conflict.Remote == nil,
		CollectionName: collection, BasePath: section.Path, Entity: section.Entity,
	}
	s.RenderStatus(w, http.StatusConflict, "conflict.html", v)
}

// icalTitle names an object whatever component it holds.
func (s *Server) icalTitle(obj *model.Object) string {
	if obj == nil {
		return ""
	}
	loc := s.timezone()
	switch obj.Component() {
	case "VTODO":
		if task, err := obj.Todo(loc); err == nil {
			return task.DisplayTitle()
		}
	case "VJOURNAL":
		if note, err := obj.Note(loc); err == nil {
			return note.DisplayTitle()
		}
	default:
		if ev, err := obj.Event(loc); err == nil {
			return ev.DisplayTitle()
		}
	}
	return ""
}

// CalendarConflictResolve applies one choice from the event conflict screen.
func (s *Server) CalendarConflictResolve(w http.ResponseWriter, r *http.Request) {
	s.resolveICalConflict(w, r, sectionCalendar)
}

// TaskConflictResolve applies one choice from the task conflict screen.
func (s *Server) TaskConflictResolve(w http.ResponseWriter, r *http.Request) {
	s.resolveICalConflict(w, r, sectionTasks)
}

// NoteConflictResolve applies one choice from the note conflict screen.
func (s *Server) NoteConflictResolve(w http.ResponseWriter, r *http.Request) {
	s.resolveICalConflict(w, r, sectionNotes)
}

func (s *Server) resolveICalConflict(w http.ResponseWriter, r *http.Request, section icalSection) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sess := SessionFrom(r)
	key, choice := strings.TrimSpace(r.PostFormValue("conflict_key")), strings.TrimSpace(r.PostFormValue("choice"))
	accountID, colEnc, uid := r.PathValue("account"), r.PathValue("col"), r.PathValue("uid")
	listURL := s.Path("/app/" + section.Path + "/" + accountID + "/" + colEnc)
	draft, ok := sess.PeekConflict(key)
	if !ok {
		http.Redirect(w, r, listURL, http.StatusSeeOther)
		return
	}
	if choice == "keep_server" || choice == "cancel" {
		sess.ClearConflict(key)
		target := listURL
		if choice == "keep_server" && uid != "" {
			target += "/" + urlPathEscape(uid)
		}
		http.Redirect(w, r, target, http.StatusSeeOther)
		return
	}
	if choice != "apply_mine" || len(draft.Body) == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	p, _, err := s.calendarProvider(sess, draft.AccountID)
	if err != nil {
		s.renderSectionError(w, r, section, err, draft.AccountID, colEnc)
		return
	}
	local, err := model.ParseICal(draft.Path, "", draft.Body)
	if err != nil {
		s.renderSectionError(w, r, section, err, draft.AccountID, colEnc)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	remote, err := p.Get(ctx, draft.Collection, draft.Path)
	if err != nil {
		local.ETag, local.Path = "", draft.Path
		_, err = p.Create(ctx, draft.Collection, local)
	} else {
		local.ETag, local.Path = remote.ETag, remote.Path
		_, err = p.Update(ctx, draft.Collection, local)
	}
	if err != nil {
		if calendar.IsConflict(err) {
			s.showICalConflict(w, r, sess, section, draft.AccountID, draft.Collection, colEnc, draft.UID, err)
			return
		}
		s.renderSectionError(w, r, section, err, draft.AccountID, colEnc)
		return
	}
	sess.ClearConflict(key)
	http.Redirect(w, r, listURL+"/"+urlPathEscape(draft.UID), http.StatusSeeOther)
}

// renderSectionError reports a DAV failure on the screen the section belongs to.
func (s *Server) renderSectionError(w http.ResponseWriter, r *http.Request, section icalSection, err error, accountID, colEnc string) {
	switch section.Path {
	case sectionTasks.Path:
		s.renderTasksError(w, r, err, accountID, colEnc)
	case sectionNotes.Path:
		s.renderNotesError(w, r, err, accountID, colEnc)
	default:
		s.renderCalendarError(w, r, err, accountID, colEnc)
	}
}
