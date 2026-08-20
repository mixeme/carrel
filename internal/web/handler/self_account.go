// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/session"
	"gitea.mixdep.ru/mix/carrel/internal/store"
)

const (
	fieldSessionID    = "session_id"
	fieldConfirmLogin = "confirm_login"
	fieldDisplayName  = "display_name"
)

// settingsAccountSubmit is the self-service counterpart of adminKillSessions
// and adminDeleteUser (2.6.C3, 2.6.C4): a person could already see and end
// someone else's sessions from the admin panel, and an administrator could
// already delete a user, but nobody could do either to their own account.
func (s *Server) settingsAccountSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sess := SessionFrom(r)
	if sess == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	actor := storeActorFrom(r, sess)

	switch r.PostFormValue(fieldAction) {
	case "delete_account":
		s.deleteOwnAccount(w, r, sess, actor)
	case "save_display_name":
		s.saveDisplayName(w, r, sess)
	case "confirm_email":
		s.resendEmailConfirmation(w, r, sess, actor)
	case "end_session":
		err := s.endOwnSession(r, sess, actor)
		notice := ""
		if err == nil {
			notice = "Session ended."
		}
		s.renderAccountAction(w, r, err, notice)
	case "end_other_sessions":
		n, err := s.endOtherSessions(r, sess, actor)
		notice := ""
		if err == nil {
			notice = fmt.Sprintf("Ended %d other session(s).", n)
		}
		s.renderAccountAction(w, r, err, notice)
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
	}
}

func (s *Server) renderAccountAction(w http.ResponseWriter, r *http.Request, err error, notice string) {
	v := s.settingsFrame(r, "Account", settingsSectionAccount)
	status := http.StatusOK
	if err != nil {
		v.Error = err.Error()
		status = http.StatusBadRequest
	} else if notice != "" {
		v.Notice = notice
	}
	v.Data = settingsView{Section: settingsSectionAccount, appView: s.buildAppView(r)}
	s.RenderStatus(w, status, "settings_account.html", v)
}

// endOwnSession ends one of the caller's own sessions, other than the one
// serving this request — ending the live session would wipe its DEK before
// buildAppView needed it to render the response, and "sign out" already
// covers ending the current one on purpose.
func (s *Server) endOwnSession(r *http.Request, sess *session.Session, actor store.Actor) error {
	id := strings.TrimSpace(r.PostFormValue(fieldSessionID))
	if id == "" {
		return fmt.Errorf("choose a session to end")
	}
	if id == sess.ID {
		return fmt.Errorf("that is your current session — use Sign out instead")
	}
	owned := false
	for _, info := range s.Sessions.Sessions(sess.UserID) {
		if info.ID == id {
			owned = true
			break
		}
	}
	if !owned {
		return fmt.Errorf("session not found")
	}
	s.Sessions.Destroy(id)
	_ = s.Store.Log(store.AuditEntry{
		Action:      store.ActionSessionEnd,
		ActorID:     actor.ID,
		ActorLogin:  actor.Login,
		TargetID:    sess.UserID,
		TargetLogin: sess.Login,
		IP:          actor.IP,
	})
	return nil
}

// endOtherSessions ends every session but this one.
func (s *Server) endOtherSessions(r *http.Request, sess *session.Session, actor store.Actor) (int, error) {
	n := 0
	for _, info := range s.Sessions.Sessions(sess.UserID) {
		if info.ID == sess.ID {
			continue
		}
		s.Sessions.Destroy(info.ID)
		n++
	}
	if n == 0 {
		return 0, fmt.Errorf("no other sessions to end")
	}
	_ = s.Store.Log(store.AuditEntry{
		Action:      store.ActionSessionsEndOthers,
		ActorID:     actor.ID,
		ActorLogin:  actor.Login,
		TargetID:    sess.UserID,
		TargetLogin: sess.Login,
		IP:          actor.IP,
		Detail:      fmt.Sprintf("%d session(s)", n),
	})
	return n, nil
}

// deleteOwnAccount removes the signed-in user's own account (2.6.C4). §5.5
// already lets an administrator delete someone; this is the same operation
// with the caller as both actor and target, gated on typing the login back —
// there is no password re-check because the account holder proving they can
// type their own login, while already holding a live session, is the same
// bar §10.1 sets for deleting a collection.
func (s *Server) deleteOwnAccount(w http.ResponseWriter, r *http.Request, sess *session.Session, actor store.Actor) {
	if strings.TrimSpace(r.PostFormValue(fieldConfirmLogin)) != sess.Login {
		v := s.settingsFrame(r, "Account", settingsSectionAccount)
		v.Error = "Type your login exactly to confirm."
		v.Data = settingsView{Section: settingsSectionAccount, appView: s.buildAppView(r)}
		s.RenderStatus(w, http.StatusBadRequest, "settings_account.html", v)
		return
	}
	if err := s.Store.DeleteUser(actor, sess.UserID); err != nil {
		v := s.settingsFrame(r, "Account", settingsSectionAccount)
		if errors.Is(err, store.ErrLastAdmin) {
			v.Error = "You are the last administrator: promote someone else before deleting this account."
		} else {
			v.Error = err.Error()
		}
		v.Data = settingsView{Section: settingsSectionAccount, appView: s.buildAppView(r)}
		s.RenderStatus(w, http.StatusBadRequest, "settings_account.html", v)
		return
	}
	// The store decides first, the same order adminDeleteUser uses: a refused
	// deletion must not have ended any sessions on its way to being refused.
	s.Sessions.DestroyUser(sess.UserID)
	s.ClearSessionCookie(w, r)
	s.redirect(w, r, s.Path("/login"))
}

func (s *Server) saveDisplayName(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	name := r.PostFormValue(fieldDisplayName)
	if err := s.Store.SetDisplayName(sess.UserID, name); err != nil {
		s.renderAccountAction(w, r, err, "")
		return
	}
	s.renderAccountAction(w, r, nil, "Display name saved.")
}

func (s *Server) resendEmailConfirmation(w http.ResponseWriter, r *http.Request, sess *session.Session, actor store.Actor) {
	user, err := s.Store.User(sess.UserID)
	if err != nil {
		s.renderAccountAction(w, r, err, "")
		return
	}
	if user.Email == "" {
		s.renderAccountAction(w, r, fmt.Errorf("no address on file"), "")
		return
	}
	if user.EmailConfirmed {
		s.renderAccountAction(w, r, fmt.Errorf("address already confirmed"), "")
		return
	}
	token, err := s.Store.RequestEmailChange(actor, sess.UserID, user.Email, 0)
	if err != nil {
		s.renderAccountAction(w, r, err, "")
		return
	}
	link := s.publicURL(r, "/confirm-email/"+token)
	expires := time.Now().Add(store.DefaultEmailChangeTTL)
	if s.Mail != nil {
		s.Mail.QueueEmailChange(user.Email, sess.Login, link, expires)
	}
	s.renderAccountAction(w, r, nil, "A confirmation link was sent to "+user.Email+".")
}
