// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/store"
)

const (
	fieldAction        = "action"
	fieldRole          = "role"
	fieldInviteID      = "invite_id"
	fieldSMTPHost      = "smtp_host"
	fieldSMTPPort      = "smtp_port"
	fieldSMTPTLS       = "smtp_tls"
	fieldSMTPUser      = "smtp_username"
	fieldSMTPPass      = "smtp_password"
	fieldSMTPFrom      = "smtp_from"
	fieldSMTPFromName  = "smtp_from_name"
	fieldTestEmail     = "test_email"
	fieldTempPassword  = "temp_password"
	fieldSelfReg       = "self_registration"
	fieldInviteTTL     = "invite_ttl_hours"
	fieldSessionIdle   = "session_idle_hours"
	fieldSessionAbs    = "session_absolute_days"
	fieldAuditAction   = "audit_action"
	fieldAuditCategory = "audit_category"
	fieldUserFilter    = "user_filter"
)

// auditSecurityActions and auditFailureActions back the All / Security /
// Failures segment of 2.6.C6. Security is everything that changes who can
// sign in or what a session can do; Failures is the subset that is, on its
// own, a sign something went wrong rather than a person doing their job.
var auditSecurityActions = []string{
	store.ActionLogin, store.ActionLoginFailed, store.ActionLogout,
	store.ActionPasswordChange, store.ActionPasswordReset,
	store.ActionSessionsKilled, store.ActionSessionEnd, store.ActionSessionsEndOthers,
	store.ActionUserDisable, store.ActionUserEnable, store.ActionUserDelete,
	store.ActionEscrowEnable, store.ActionEscrowDisable, store.ActionEscrowOptIn,
	store.ActionEscrowOptOut, store.ActionEscrowRecover,
}

var auditFailureActions = []string{store.ActionLoginFailed}

func auditCategoryActions(category string) []string {
	switch category {
	case "security":
		return auditSecurityActions
	case "failures":
		return auditFailureActions
	default:
		return nil
	}
}

// Administration is split into one page per topic so the panel is not a
// single sheet of unrelated forms. /admin/ is users; the rest live under
// /admin/{section}.
const (
	adminSectionUsers    = "users"
	adminSectionInvites  = "invites"
	adminSectionSettings = "settings"
	adminSectionDAV      = "dav"
	adminSectionEscrow   = "escrow"
	adminSectionAudit    = "audit"
)

// adminView is what an administration page renders.
type adminView struct {
	// Section is which subsection this response belongs to.
	Section         string
	Settings        store.Settings
	Invites         []inviteRow
	Users           []userRow
	NewInviteLink   string
	InviteEmailSent string
	SMTPConfigured  bool
	SMTPDiag        string
	// UserCreated is the login of an account just made with a temporary password.
	UserCreated string
	// RegisterURL is the public sign-up address, shown when self-registration is on.
	RegisterURL string
	// Escrow is the instance-wide deposit status (§5.4).
	Escrow escrowStatus
	// EscrowCoverage is how many accounts the master password reaches.
	EscrowCoverage int
	// Recovered names the account the last recovery restored.
	Recovered string
	// MailWarning is set when a notice that may not be skipped could not be
	// sent, so the administrator knows to deliver it themselves.
	MailWarning string
	// AuditAction is the exact-action filter applied to the log viewer.
	AuditAction string
	// AuditCategory is the All / Security / Failures segment of 2.6.C6,
	// narrowing alongside AuditAction rather than replacing it.
	AuditCategory string
	// AuditLog is the newest entries matching AuditAction and AuditCategory.
	AuditLog []store.AuditEntry
	// UserFilter is the All / Administrators / Disabled segment on the users
	// list (2.6.C6).
	UserFilter string
	// AllUsers is every account regardless of UserFilter, for pickers — the
	// password-reset form must offer someone even while the table is
	// narrowed to a different filter.
	AllUsers []userRow
	// Durations for the settings form, rounded for display.
	InviteTTLHours      int64
	SessionIdleHours    int64
	SessionAbsoluteDays int64
	// DAVDiag holds discovery output from the admin validator.
	DAVDiag string
}

type userRow struct {
	*store.User
	Sessions int
	DAVCount int
}

type inviteRow struct {
	*store.Invite
	Status string
}

// AdminHome is the users page, the landing screen after an administrator
// signs in (§5.5). The other topics are AdminSection.
func (s *Server) AdminHome(w http.ResponseWriter, r *http.Request) {
	s.serveAdmin(w, r, adminSectionUsers)
}

// AdminSection is one administration subsection: invitations, settings,
// escrow or the audit log. An unknown name is a 404 rather than the users
// page, so a mistyped URL is not silently rewritten.
func (s *Server) AdminSection(w http.ResponseWriter, r *http.Request) {
	section := r.PathValue("section")
	switch section {
	case "users":
		section = adminSectionUsers
	case adminSectionInvites, adminSectionSettings, adminSectionDAV, adminSectionEscrow, adminSectionAudit:
	default:
		http.NotFound(w, r)
		return
	}
	s.serveAdmin(w, r, section)
}

func (s *Server) serveAdmin(w http.ResponseWriter, r *http.Request, section string) {
	if r.Method == http.MethodPost {
		s.adminSubmit(w, r, section)
		return
	}
	s.renderAdmin(w, r, adminView{
		Section:       section,
		AuditAction:   r.URL.Query().Get(fieldAuditAction),
		AuditCategory: r.URL.Query().Get(fieldAuditCategory),
		UserFilter:    r.URL.Query().Get(fieldUserFilter),
	})
}

func (s *Server) adminSubmit(w http.ResponseWriter, r *http.Request, from string) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	sess := SessionFrom(r)
	actor := store.Actor{ID: sess.UserID, Login: sess.Login, IP: ClientIP(r)}

	var data adminView
	var err error
	switch r.PostFormValue(fieldAction) {
	case "create_invite_link":
		data, err = s.adminCreateInviteLink(w, r, actor)
	case "create_invite_email":
		data, err = s.adminCreateInviteEmail(w, r, actor)
	case "revoke_invite":
		data, err = s.adminRevokeInvite(r, actor)
	case "extend_invite":
		data, err = s.adminExtendInvite(r, actor)
	case "resend_invite":
		data, err = s.adminResendInvite(w, r, actor)
	case "create_user":
		data, err = s.adminCreateUser(r, actor)
	case "save_self_registration":
		data, err = s.adminSaveSelfRegistration(r, actor)
	case "save_smtp":
		data, err = s.adminSaveSMTP(r, actor)
	case "test_smtp":
		data, err = s.adminTestSMTP(r, actor)
	case "test_dav":
		data, err = s.adminTestDAV(r, actor)
	case "enable_escrow":
		data, err = s.adminEnableEscrow(r, actor)
	case "resume_escrow":
		data, err = s.adminResumeEscrow(r, actor)
	case "disable_escrow":
		data, err = s.adminDisableEscrow(r, actor)
	case "change_master_password":
		data, err = s.adminChangeMaster(r, actor)
	case "recover_user":
		data, err = s.adminRecoverUser(r, actor)
	case "reset_password":
		data, err = s.adminResetPassword(r, actor)
	case "disable_user":
		data, err = s.adminDisableUser(r, actor)
	case "enable_user":
		data, err = s.adminEnableUser(r, actor)
	case "delete_user":
		data, err = s.adminDeleteUser(r, actor)
	case "change_role":
		data, err = s.adminChangeRole(r, actor)
	case "kill_sessions":
		data, err = s.adminKillSessions(r, actor)
	case "save_settings":
		data, err = s.adminSaveSettings(r, actor)
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}

	if data.Section == "" {
		data.Section = adminSectionForAction(r.PostFormValue(fieldAction))
	}
	if data.Section == "" {
		data.Section = from
	}

	v := s.adminFrame(r)
	if err != nil {
		status := http.StatusBadRequest
		var throttled throttleError
		if errors.As(err, &throttled) {
			status = http.StatusTooManyRequests
			retryAfter(w, throttled.wait)
		}
		v.Error = err.Error()
		v.Data = s.buildAdminView(r, data)
		s.RenderStatus(w, status, adminTemplate(data.Section), v)
		return
	}

	switch {
	case data.NewInviteLink != "":
		v.Notice = "Invitation link created. Copy it below and share it with the new user."
	case data.InviteEmailSent != "":
		v.Notice = "Invitation email queued for " + data.InviteEmailSent + "."
	case data.Recovered != "":
		v.Notice = "Recovered " + data.Recovered +
			". Hand over the temporary password; they must change it at their next sign-in, and their data is intact."
	case data.UserCreated != "":
		v.Notice = "Created " + data.UserCreated +
			". Hand over the temporary password; they must change it at their next sign-in."
	}
	v.Data = s.buildAdminView(r, data)
	s.Render(w, adminTemplate(data.Section), v)
}

// AdminAuditExport streams the whole audit log as CSV (2.6.C5), honouring the
// same action filter as the viewer. It writes straight to the response as it
// reads, so the export never buffers the log a second time, and it is not
// capped at the 200-row viewer limit — an export is for the record, not for
// a screen.
func (s *Server) AdminAuditExport(w http.ResponseWriter, r *http.Request) {
	sess := SessionFrom(r)
	action := r.URL.Query().Get(fieldAuditAction)
	category := r.URL.Query().Get(fieldAuditCategory)
	entries := s.Store.Audit(store.AuditFilter{Action: action, Categories: auditCategoryActions(category)})

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="audit-log.csv"`)
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"when", "action", "actor", "target", "ip", "detail"})
	for _, e := range entries {
		_ = cw.Write([]string{
			e.At.UTC().Format(time.RFC3339),
			e.Action,
			e.ActorLogin,
			e.TargetLogin,
			e.IP,
			e.Detail,
		})
	}
	cw.Flush()

	if sess != nil {
		_ = s.Store.Log(store.AuditEntry{
			Action:     store.ActionAuditExport,
			ActorID:    sess.UserID,
			ActorLogin: sess.Login,
			IP:         ClientIP(r),
			Detail:     fmt.Sprintf("%d entries", len(entries)),
		})
	}
}

func (s *Server) buildAdminView(r *http.Request, partial adminView) adminView {
	now := time.Now()
	out := partial
	if out.Section == "" {
		out.Section = adminSectionUsers
	}
	out.Settings = s.Store.Settings()
	out.RegisterURL = s.publicURL(r, "/register")
	for _, u := range s.Store.Users() {
		row := userRow{
			User:     u,
			Sessions: s.Sessions.Count(u.ID),
			DAVCount: u.DAVAccountCount,
		}
		out.AllUsers = append(out.AllUsers, row)
		if matchesUserFilter(u, out.UserFilter) {
			out.Users = append(out.Users, row)
		}
	}
	out.Escrow = escrowStatusOf(out.Settings, nil)
	out.EscrowCoverage = s.Store.EscrowCoverage()
	for _, inv := range s.Store.Invites() {
		out.Invites = append(out.Invites, inviteRow{
			Invite: inv,
			Status: string(inv.Status(now)),
		})
	}
	out.AuditLog = s.Store.Audit(store.AuditFilter{
		Action:     out.AuditAction,
		Categories: auditCategoryActions(out.AuditCategory),
		Limit:      200,
	})
	out.InviteTTLHours = out.Settings.InviteTTLSeconds / 3600
	out.SessionIdleHours = out.Settings.SessionIdleSeconds / 3600
	out.SessionAbsoluteDays = out.Settings.SessionAbsoluteSeconds / 86400
	out.SMTPConfigured = out.Settings.SMTP.Configured()
	return out
}

func (s *Server) renderAdmin(w http.ResponseWriter, r *http.Request, partial adminView) {
	v := s.adminFrame(r)
	v.Data = s.buildAdminView(r, partial)
	s.Render(w, adminTemplate(partial.Section), v)
}

func (s *Server) adminFrame(r *http.Request) View {
	v := s.View(r, "Administration")
	v.InAdmin = true
	v.ShellLayout = "admin"
	s.firstLoginEscrowNotice(r, &v)
	return v
}

func adminTemplate(section string) string {
	switch section {
	case adminSectionInvites:
		return "admin_invites.html"
	case adminSectionSettings:
		return "admin_settings.html"
	case adminSectionDAV:
		return "admin_dav.html"
	case adminSectionEscrow:
		return "admin_escrow.html"
	case adminSectionAudit:
		return "admin_audit.html"
	default:
		return "admin.html"
	}
}

func adminSectionForAction(action string) string {
	switch action {
	case "create_invite_link", "create_invite_email", "revoke_invite", "extend_invite", "resend_invite",
		"create_user", "save_self_registration":
		return adminSectionInvites
	case "save_smtp", "test_smtp", "save_settings":
		return adminSectionSettings
	case "test_dav":
		return adminSectionDAV
	case "enable_escrow", "resume_escrow", "disable_escrow",
		"change_master_password", "recover_user":
		return adminSectionEscrow
	case "disable_user", "enable_user", "delete_user",
		"change_role", "kill_sessions", "reset_password":
		return adminSectionUsers
	default:
		return ""
	}
}

func (s *Server) adminCreateInviteLink(w http.ResponseWriter, r *http.Request, actor store.Actor) (adminView, error) {
	role := store.Role(r.PostFormValue(fieldRole))
	if !role.Valid() {
		role = store.RoleUser
	}

	_, token, err := s.Store.CreateInvite(actor, role, store.InviteDeliveryLink, "", 0)
	if err != nil {
		return adminView{}, err
	}

	link := s.publicURL(r, "/invite/"+token)
	return adminView{NewInviteLink: link}, nil
}

func (s *Server) adminCreateInviteEmail(w http.ResponseWriter, r *http.Request, actor store.Actor) (adminView, error) {
	if !s.Store.Settings().SMTP.Configured() {
		return adminView{}, fmt.Errorf("configure SMTP before sending invitations by email")
	}
	if s.Mail == nil {
		return adminView{}, fmt.Errorf("mail is not available")
	}

	email := store.NormalizeEmail(r.PostFormValue(fieldEmail))
	role := store.Role(r.PostFormValue(fieldRole))
	if !role.Valid() {
		role = store.RoleUser
	}
	if err := store.ValidateEmail(email); err != nil {
		return adminView{}, fmt.Errorf("%s", capitalize(err.Error()))
	}

	inv, token, err := s.Store.CreateInvite(actor, role, store.InviteDeliveryEmail, email, 0)
	if err != nil {
		return adminView{}, err
	}

	link := s.publicURL(r, "/invite/"+token)
	s.Mail.QueueInvite(inv.ID, email, actor.Login, link, inv.ExpiresAt)
	return adminView{InviteEmailSent: email}, nil
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
		if errors.Is(err, store.ErrInviteNotByEmail) {
			return adminView{}, fmt.Errorf("only email invitations can be resent")
		}
		return adminView{}, err
	}
	inv, err := s.Store.Invite(id)
	if err != nil {
		return adminView{}, err
	}
	if s.Mail == nil {
		return adminView{}, fmt.Errorf("mail is not available")
	}
	link := s.publicURL(r, "/invite/"+token)
	s.Mail.QueueInvite(inv.ID, inv.Email, actor.Login, link, inv.ExpiresAt)
	return adminView{InviteEmailSent: inv.Email}, nil
}

func (s *Server) adminCreateUser(r *http.Request, actor store.Actor) (adminView, error) {
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
	return adminView{UserCreated: login}, nil
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

func (s *Server) adminDisableUser(r *http.Request, actor store.Actor) (adminView, error) {
	userID := r.PostFormValue(fieldUserID)
	if err := s.Store.SetDisabled(actor, userID, true); err != nil {
		return adminView{}, adminUserErr(err)
	}
	s.Sessions.DestroyUser(userID)
	return adminView{}, nil
}

func (s *Server) adminEnableUser(r *http.Request, actor store.Actor) (adminView, error) {
	userID := r.PostFormValue(fieldUserID)
	if err := s.Store.SetDisabled(actor, userID, false); err != nil {
		return adminView{}, adminUserErr(err)
	}
	return adminView{}, nil
}

func (s *Server) adminDeleteUser(r *http.Request, actor store.Actor) (adminView, error) {
	userID := r.PostFormValue(fieldUserID)
	// The store decides first: a refused deletion — the last administrator,
	// say — must not have signed anyone out on its way to being refused.
	if err := s.Store.DeleteUser(actor, userID); err != nil {
		return adminView{}, adminUserErr(err)
	}
	s.Sessions.DestroyUser(userID)
	return adminView{}, nil
}

func (s *Server) adminChangeRole(r *http.Request, actor store.Actor) (adminView, error) {
	userID := r.PostFormValue(fieldUserID)
	role := store.Role(r.PostFormValue(fieldRole))
	if !role.Valid() {
		return adminView{}, fmt.Errorf("choose a valid role")
	}
	if err := s.Store.SetRole(actor, userID, role); err != nil {
		return adminView{}, adminUserErr(err)
	}
	return adminView{}, nil
}

func (s *Server) adminKillSessions(r *http.Request, actor store.Actor) (adminView, error) {
	userID := r.PostFormValue(fieldUserID)
	user, err := s.Store.User(userID)
	if err != nil {
		return adminView{}, adminUserErr(err)
	}
	n := s.Sessions.DestroyUser(userID)
	if n == 0 {
		return adminView{}, fmt.Errorf("this account has no active sessions")
	}
	if err := s.Store.Log(store.AuditEntry{
		Action:      store.ActionSessionsKilled,
		ActorID:     actor.ID,
		ActorLogin:  actor.Login,
		TargetID:    user.ID,
		TargetLogin: user.Login,
		IP:          actor.IP,
		Detail:      fmt.Sprintf("%d session(s)", n),
	}); err != nil {
		s.logError("audit kill sessions", err)
	}
	return adminView{}, nil
}

func (s *Server) adminSaveSettings(r *http.Request, actor store.Actor) (adminView, error) {
	var inviteTTL, idle, absolute int64
	for _, pair := range []struct {
		name string
		dst  *int64
		unit time.Duration
	}{
		{fieldInviteTTL, &inviteTTL, time.Hour},
		{fieldSessionIdle, &idle, time.Hour},
		{fieldSessionAbs, &absolute, 24 * time.Hour},
	} {
		raw := r.PostFormValue(pair.name)
		if raw == "" {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(raw, "%d", &n); err != nil || n < 1 {
			return adminView{}, fmt.Errorf("%s must be a positive number", pair.name)
		}
		*pair.dst = int64(time.Duration(n) * pair.unit / time.Second)
	}

	if err := s.Store.UpdateSettings(actor, func(st *store.Settings) {
		if inviteTTL > 0 {
			st.InviteTTLSeconds = inviteTTL
		}
		if idle > 0 {
			st.SessionIdleSeconds = idle
		}
		if absolute > 0 {
			st.SessionAbsoluteSeconds = absolute
		}
	}); err != nil {
		return adminView{}, err
	}
	return adminView{}, nil
}

func (s *Server) adminSaveSelfRegistration(r *http.Request, actor store.Actor) (adminView, error) {
	on := r.PostFormValue(fieldSelfReg) == "1"
	if on && !s.Store.Settings().SMTP.Configured() {
		return adminView{}, fmt.Errorf("configure SMTP before allowing self-registration")
	}
	if err := s.Store.UpdateSettings(actor, func(st *store.Settings) {
		st.SelfRegistration = on
	}); err != nil {
		return adminView{}, err
	}
	return adminView{}, nil
}

// matchesUserFilter backs the All / Administrators / Disabled segment on the
// users list (2.6.C6).
func matchesUserFilter(u *store.User, filter string) bool {
	switch filter {
	case "admin":
		return u.Role == store.RoleAdmin
	case "disabled":
		return u.Disabled
	default:
		return true
	}
}

func adminUserErr(err error) error {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return fmt.Errorf("account not found")
	case errors.Is(err, store.ErrLastAdmin):
		return fmt.Errorf("cannot change the last administrator")
	default:
		return err
	}
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
		v := s.settingsFrame(r, "Account", settingsSectionAccount)
		v.Error = capitalize(err.Error()) + "."
		v.Data = settingsView{Section: settingsSectionAccount, appView: s.buildAppView(r)}
		s.RenderStatus(w, http.StatusBadRequest, "settings_account.html", v)
		return
	}

	link := s.publicURL(r, "/confirm-email/"+token)
	expires := time.Now().Add(store.DefaultEmailChangeTTL)
	if s.Mail != nil {
		s.Mail.QueueEmailChange(newEmail, sess.Login, link, expires)
	}

	v := s.settingsFrame(r, "Account", settingsSectionAccount)
	v.Notice = "A confirmation link was sent to " + newEmail + "."
	v.Data = settingsView{Section: settingsSectionAccount, appView: s.buildAppView(r)}
	s.Render(w, "settings_account.html", v)
}
