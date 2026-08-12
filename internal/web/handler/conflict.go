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
	"gitea.mixdep.ru/mix/carrel/internal/provider/contacts"
	"gitea.mixdep.ru/mix/carrel/internal/session"
)

type conflictView struct {
	AccountID    string
	ColEnc       string
	UID          string
	Key          string
	Lines        []model.DiffLine
	LocalName    string
	RemoteName   string
	RemoteGone   bool
	CollectionName string
}

func (s *Server) showConflict(w http.ResponseWriter, r *http.Request, sess *session.Session, accountID, collection, colEnc, uid string, err error) {
	var conflict *contacts.ConflictError
	if !errors.As(err, &conflict) {
		s.renderContactError(w, r, err, accountID, colEnc)
		return
	}

	key := conflictKey(accountID, collection, uid)
	draft := session.ConflictDraft{
		Key:        key,
		AccountID:  accountID,
		Collection: collection,
		Path:       conflict.Path,
		UID:        uid,
	}
	if conflict.Local != nil {
		body, marshalErr := conflict.Local.Marshal()
		if marshalErr == nil {
			draft.Body = body
		}
		if uid == "" {
			uid = conflict.Local.UID()
			draft.UID = uid
		}
	}
	sess.PutConflict(draft)

	lines, _ := model.Diff(conflict.Local, conflict.Remote)
	localName, remoteName := "Your edit", "Server version"
	if conflict.Local != nil {
		if c, err := conflict.Local.Contact(); err == nil {
			localName = c.DisplayName()
		}
	}
	if conflict.Remote != nil {
		if c, err := conflict.Remote.Contact(); err == nil {
			remoteName = c.DisplayName()
		}
	}

	v := s.View(r, "Conflict")
	v.Error = "This contact changed on the server since you opened it."
	v.Data = conflictView{
		AccountID:      accountID,
		ColEnc:         colEnc,
		UID:            uid,
		Key:            key,
		Lines:          lines,
		LocalName:      localName,
		RemoteName:     remoteName,
		RemoteGone:     conflict.Remote == nil,
		CollectionName: collection,
	}
	s.RenderStatus(w, http.StatusConflict, "conflict.html", v)
}

// ConflictResolve handles the three choices on the conflict screen (§9).
func (s *Server) ConflictResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.resolveConflict(w, r)
}

func (s *Server) resolveConflict(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sess := SessionFrom(r)
	key := strings.TrimSpace(r.PostFormValue("conflict_key"))
	choice := strings.TrimSpace(r.PostFormValue("choice"))
	accountID := r.PathValue("account")
	colEnc := r.PathValue("col")
	uid := r.PathValue("uid")
	if accountID == "" {
		accountID = r.PostFormValue("account_id")
	}
	if colEnc == "" {
		colEnc = r.PostFormValue("col")
	}
	if uid == "" {
		uid = r.PostFormValue("uid")
	}

	draft, ok := sess.PeekConflict(key)
	if !ok {
		http.Redirect(w, r, s.Path("/app/contacts/"+accountID+"/"+colEnc), http.StatusSeeOther)
		return
	}

	switch choice {
	case "keep_server", "cancel":
		sess.ClearConflict(key)
		target := s.Path("/app/contacts/" + accountID + "/" + colEnc)
		if choice == "keep_server" && uid != "" {
			target = s.Path("/app/contacts/" + accountID + "/" + colEnc + "/" + urlPathEscape(uid))
		}
		http.Redirect(w, r, target, http.StatusSeeOther)
		return
	case "apply_mine":
		// continue below
	default:
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if len(draft.Body) == 0 {
		sess.ClearConflict(key)
		http.Error(w, "nothing to apply", http.StatusBadRequest)
		return
	}

	p, _, err := s.contactsProvider(sess, draft.AccountID)
	if err != nil {
		s.renderContactError(w, r, err, draft.AccountID, colEnc)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	local, err := model.ParseVCard(draft.Path, "", draft.Body)
	if err != nil {
		s.renderContactError(w, r, err, draft.AccountID, colEnc)
		return
	}

	// Re-read the server version so the write is conditional on what is there now.
	remote, err := p.Get(ctx, draft.Collection, draft.Path)
	if err != nil {
		// Object may have been deleted; try create.
		local.ETag = ""
		local.Path = draft.Path
		if _, err := p.Create(ctx, draft.Collection, local); err != nil {
			if contacts.IsConflict(err) {
				s.showConflict(w, r, sess, draft.AccountID, draft.Collection, colEnc, draft.UID, err)
				return
			}
			s.renderContactError(w, r, err, draft.AccountID, colEnc)
			return
		}
		sess.ClearConflict(key)
		http.Redirect(w, r, s.Path("/app/contacts/"+draft.AccountID+"/"+colEnc+"/"+urlPathEscape(draft.UID)), http.StatusSeeOther)
		return
	}

	local.ETag = remote.ETag
	local.Path = remote.Path
	if _, err := p.Update(ctx, draft.Collection, local); err != nil {
		if contacts.IsConflict(err) {
			s.showConflict(w, r, sess, draft.AccountID, draft.Collection, colEnc, draft.UID, err)
			return
		}
		s.renderContactError(w, r, err, draft.AccountID, colEnc)
		return
	}
	sess.ClearConflict(key)
	http.Redirect(w, r, s.Path("/app/contacts/"+draft.AccountID+"/"+colEnc+"/"+urlPathEscape(draft.UID)), http.StatusSeeOther)
}
