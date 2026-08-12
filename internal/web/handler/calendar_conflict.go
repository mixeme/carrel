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

func (s *Server) showCalendarConflict(w http.ResponseWriter, r *http.Request, sess *session.Session, accountID, collection, colEnc, uid string, err error) {
	var conflict *calendar.ConflictError
	if !errors.As(err, &conflict) {
		s.renderCalendarError(w, r, err, accountID, colEnc)
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
	if conflict.Local != nil {
		if ev, parseErr := conflict.Local.Event(s.timezone()); parseErr == nil {
			localName = ev.DisplayTitle()
		}
	}
	if conflict.Remote != nil {
		if ev, parseErr := conflict.Remote.Event(s.timezone()); parseErr == nil {
			remoteName = ev.DisplayTitle()
		}
	}
	v := s.View(r, "Calendar conflict")
	v.Error = "This event changed on the server since you opened it."
	v.Data = conflictView{
		AccountID: accountID, ColEnc: colEnc, UID: uid, Key: key, Lines: lines,
		LocalName: localName, RemoteName: remoteName, RemoteGone: conflict.Remote == nil,
		CollectionName: collection, BasePath: "calendar", Entity: "event",
	}
	s.RenderStatus(w, http.StatusConflict, "conflict.html", v)
}

// CalendarConflictResolve applies one choice from the event conflict screen.
func (s *Server) CalendarConflictResolve(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sess := SessionFrom(r)
	key, choice := strings.TrimSpace(r.PostFormValue("conflict_key")), strings.TrimSpace(r.PostFormValue("choice"))
	accountID, colEnc, uid := r.PathValue("account"), r.PathValue("col"), r.PathValue("uid")
	draft, ok := sess.PeekConflict(key)
	if !ok {
		http.Redirect(w, r, s.Path("/app/calendar/"+accountID+"/"+colEnc), http.StatusSeeOther)
		return
	}
	if choice == "keep_server" || choice == "cancel" {
		sess.ClearConflict(key)
		target := s.Path("/app/calendar/" + accountID + "/" + colEnc)
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
		s.renderCalendarError(w, r, err, draft.AccountID, colEnc)
		return
	}
	local, err := model.ParseICal(draft.Path, "", draft.Body)
	if err != nil {
		s.renderCalendarError(w, r, err, draft.AccountID, colEnc)
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
			s.showCalendarConflict(w, r, sess, draft.AccountID, draft.Collection, colEnc, draft.UID, err)
			return
		}
		s.renderCalendarError(w, r, err, draft.AccountID, colEnc)
		return
	}
	sess.ClearConflict(key)
	http.Redirect(w, r, s.Path("/app/calendar/"+draft.AccountID+"/"+colEnc+"/"+urlPathEscape(draft.UID)), http.StatusSeeOther)
}
