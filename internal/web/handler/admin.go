// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/store"
)

const (
	fieldRole        = "role"
	fieldInviteID    = "invite_id"
	fieldSMTPHost    = "smtp_host"
	fieldSMTPPort    = "smtp_port"
	fieldSMTPTLS     = "smtp_tls"
	fieldSMTPUser    = "smtp_username"
	fieldSMTPPass    = "smtp_password"
	fieldSMTPFrom    = "smtp_from"
	fieldSMTPFromName = "smtp_from_name"
	fieldTestEmail   = "test_email"
	fieldTempPassword = "temp_password"
)

// adminView is what the administration page renders.
type adminView struct {
	Settings      store.Settings
	Invites       []inviteRow
	Users         []*store.User
	NewInviteLink string
	SMTPDiag      string
	CreationMode  store.CreationMode
}

type inviteRow struct {
	*store.Invite
	Status string
}

// AdminHome is the administrator's workspace: invitations, user creation and
// mail settings until the full panel arrives in step 9.
func (s *Server) AdminHome(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		s.adminSubmit(w, r)
		return
	}
	s.renderAdmin(w, r, adminView{})
}

func (s *Server) adminSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	action := r.PostFormValue("action")
	sess := SessionFrom(r)
	actor := store.Actor{ID: sess.UserID, Login: sess.Login, IP: ClientIP(r)}

	var data adminView
	var err error
	switch action {
	case "create_invite":
		data, err = s.adminCreateInvite(w, r, actor)
	case "revoke_invite":
		data, err = s.adminRevokeInvite(r, actor)
	case "extend_invite":
		data, err = s.adminExtendInvite(r, actor)
	case "resend_invite":
		data, err = s.adminResendInvite(w, r, actor)
	case "create_user":
		data, err = s.adminCreateUser(r, actor)
	case "save_smtp":
		data, err = s.adminSaveSMTP(r, actor)
	case "test_smtp":
		data, err = s.adminTestSMTP(r, actor)
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}

	if err != nil {
		v := s.View(r, "Administration")
		v.Error = err.Error()
		v.Data = s.buildAdminView(r, data)
		s.RenderStatus(w, http.StatusBadRequest, "admin.html", v)
		return
	}
	v := s.View(r, "Administration")
	v.Data = s.buildAdminView(r, data)
	if data.NewInviteLink != "" {
		v.Notice = "Invitation created. Copy the link below and share it with the new user."
	}
	if data.SMTPDiag != "" {
		v.Data = s.buildAdminView(r, data)
	}
	s.Render(w, "admin.html", v)
}

func (s *Server) buildAdminView(r *http.Request, partial adminView) adminView {
	now := time.Now()
	out := partial
	out.Settings = s.Store.Settings()
	out.CreationMode = out.Settings.CreationMode
	out.Users = s.Store.Users()
	for _, inv := range s.Store.Invites() {
		out.Invites = append(out.Invites, inviteRow{
			Invite: inv,
			Status: string(inv.Status(now)),
		})
	}
	return out
}

func (s *Server) renderAdmin(w http.ResponseWriter, r *http.Request, partial adminView) {
	v := s.View(r, "Administration")
	v.Data = s.buildAdminView(r, partial)
	s.Render(w, "admin.html", v)
}

func (s *Server) adminCreateInvite(w http.ResponseWriter, r *http.Request, actor store.Actor) (adminView, error) {
	login := store.NormalizeLogin(r.PostFormValue(fieldLogin))
	email := store.NormalizeEmail(r.PostFormValue(fieldEmail))
	role := store.Role(r.PostFormValue(fieldRole))
	if !role.Valid() {
		role = store.RoleUser
	}
	if err := store.ValidateLogin(login); err != nil {
		return adminView{}, fmt.Errorf("%s", capitalize(err.Error()))
	}
	if err := store.ValidateEmail(email); err != nil {
		return adminView{}, fmt.Errorf("%s", capitalize(err.Error()))
	}

	inv, token, err := s.Store.CreateInvite(actor, login, email, role, 0)
	if err != nil {
		if errors.Is(err, store.ErrLoginTaken) {
			return adminView{}, fmt.Errorf("that login is already in use")
		}
		return adminView{}, err
	}

	link := s.publicURL(r, "/invite/"+token)
	if s.Mail != nil {
		s.Mail.QueueInvite(inv.ID, email, actor.Login, link, inv.ExpiresAt)
	}
	return adminView{NewInviteLink: link}, nil
}

func (s *Server) adminRevokeInvite(r *http.Request, actor store.Actor) (adminView, error) {
	id := r.PostFormValue(fieldInviteID)
	if err := s.Store.RevokeInvite(actor, id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return adminView{}, fmt.Errorf("invitation not found")
		}
		if errors.Is(err, store.ErrInviteInvalid) {
			return adminView{}, fmt.Errorf("invitation is no longer pending")
		}
		return adminView{}, err
	}
	return adminView{}, nil
}

func (s *Server) adminExtendInvite(r *http.Request, actor store.Actor) (adminView, error) {
	id := r.PostFormValue(fieldInviteID)
	if err := s.Store.ExtendInvite(actor, id, 0); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return adminView{}, fmt.Errorf("invitation not found")
		}
		return adminView{}, err
	}
	return adminView{}, nil
}

func (s *Server) adminResendInvite(w http.ResponseWriter, r *http.Request, actor store.Actor) (adminView, error) {
	id := r.PostFormValue(fieldInviteID)
	token, err := s.Store.ResendInvite(actor, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return adminView{}, fmt.Errorf("invitation not found")
		}
		if errors.Is(err, store.ErrInviteInvalid) {
			return adminView{}, fmt.Errorf("invitation is no longer pending")
		}
		return adminView{}, err
	}
	inv, err := s.Store.Invite(id)
	if err != nil {
		return adminView{}, err
	}
	link := s.publicURL(r, "/invite/"+token)
	if s.Mail != nil {
		s.Mail.QueueInvite(inv.ID, inv.Email, actor.Login, link, inv.ExpiresAt)
	}
	return adminView{NewInviteLink: link}, nil
}

func (s *Server) adminCreateUser(r *http.Request, actor store.Actor) (adminView, error) {
	if s.Store.Settings().CreationMode != store.CreationAdminPassword {
		return adminView{}, fmt.Errorf("user creation with a temporary password is not enabled")
	}
	login := store.NormalizeLogin(r.PostFormValue(fieldLogin))
	email := store.NormalizeEmail(r.PostFormValue(fieldEmail))
	role := store.Role(r.PostFormValue(fieldRole))
	if !role.Valid() {
		role = store.RoleUser
	}
	temp := r.PostFormValue(fieldTempPassword)
	if _, err := s.Store.CreateUserWithPassword(actor, login, email, role, temp); err != nil {
		if errors.Is(err, store.ErrLoginTaken) {
			return adminView{}, fmt.Errorf("that login is already in use")
		}
		return adminView{}, err
	}
	return adminView{}, nil
}

func (s *Server) adminSaveSMTP(r *http.Request, actor store.Actor) (adminView, error) {
	host := r.PostFormValue(fieldSMTPHost)
	portStr := r.PostFormValue(fieldSMTPPort)
	tlsMode := store.TLSMode(r.PostFormValue(fieldSMTPTLS))
	user := r.PostFormValue(fieldSMTPUser)
	pass := r.PostFormValue(fieldSMTPPass)
	from := store.NormalizeEmail(r.PostFormValue(fieldSMTPFrom))
	fromName := r.PostFormValue(fieldSMTPFromName)

	var port int
	if portStr != "" {
		if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil || port < 1 || port > 65535 {
			return adminView{}, fmt.Errorf("SMTP port must be between 1 and 65535")
		}
	}
	if host != "" && tlsMode == "" {
		tlsMode = store.TLSStartTLS
	}

	if err := s.Store.UpdateSettings(actor, func(st *store.Settings) {
		st.SMTP.Host = host
		st.SMTP.Port = port
		st.SMTP.TLS = tlsMode
		st.SMTP.Username = user
		st.SMTP.FromAddress = from
		st.SMTP.FromName = fromName
	}); err != nil {
		return adminView{}, err
	}
	if pass != "" {
		if err := s.Store.SetSMTPPassword(actor, pass); err != nil {
			return adminView{}, err
		}
	}
	return adminView{}, nil
}

func (s *Server) adminTestSMTP(r *http.Request, actor store.Actor) (adminView, error) {
	to := store.NormalizeEmail(r.PostFormValue(fieldTestEmail))
	if to == "" {
		return adminView{}, fmt.Errorf("enter an address to send the test message to")
	}
	if s.Mail == nil {
		return adminView{}, fmt.Errorf("mail is not configured")
	}
	res := s.Mail.SendTest(to)
	detail := "failed"
	if res.OK {
		detail = "ok"
	}
	_ = s.Store.Log(store.AuditEntry{
		Action:     store.ActionSMTPTest,
		ActorID:    actor.ID,
		ActorLogin: actor.Login,
		IP:         actor.IP,
		Detail:     detail,
	})
	return adminView{SMTPDiag: res.Diagnostic}, nil
}

// RequestEmailChange starts the confirmation flow from the profile (§5.3).
func (s *Server) RequestEmailChange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sess := SessionFrom(r)
	if sess == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	newEmail := store.NormalizeEmail(r.PostFormValue(fieldEmail))
	actor := store.Actor{ID: sess.UserID, Login: sess.Login, IP: ClientIP(r)}

	token, err := s.Store.RequestEmailChange(actor, sess.UserID, newEmail, 0)
	if err != nil {
		v := s.View(r, "Carrel")
		v.Error = capitalize(err.Error()) + "."
		s.RenderStatus(w, http.StatusBadRequest, "app.html", v)
		return
	}

	link := s.publicURL(r, "/confirm-email/"+token)
	expires := time.Now().Add(store.DefaultEmailChangeTTL)
	if s.Mail != nil {
		s.Mail.QueueEmailChange(newEmail, sess.Login, link, expires)
	}

	v := s.View(r, "Carrel")
	v.Notice = "A confirmation link was sent to " + newEmail + "."
	s.Render(w, "app.html", v)
}
