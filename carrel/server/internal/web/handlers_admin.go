package web

import (
	"net/http"
	"net/url"
	"time"

	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

type adminUsersFlash struct {
	errText   string
	wipeLink  string
	escrowAsk bool
}

func (s *Server) getAdminUsers(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	page := s.buildAdminUsersPage(sess, adminUsersFlash{})
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) postAdminUsers(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	actorID := sess.AccountID()
	target := r.FormValue("target")
	action := r.FormValue("action")
	var flash adminUsersFlash
	var err error
	switch action {
	case "disable":
		err = s.users.DisableUser(actorID, target)
	case "delete":
		err = s.users.DeleteUser(actorID, target, r.FormValue("login"))
	case "role":
		role := store.RoleUser
		if r.FormValue("role") == string(store.RoleAdmin) {
			role = store.RoleAdmin
		}
		err = s.users.SetUserRole(actorID, target, role)
	case "sessions":
		err = s.users.EndUserSessions(actorID, target)
	case "reset":
		token, _, escrowAsk, handLink, resetErr := s.users.AdminResetPassword(ctx, actorID, target)
		if resetErr != nil {
			err = resetErr
			break
		}
		if escrowAsk {
			flash.escrowAsk = true
			break
		}
		if handLink {
			flash.wipeLink = s.route("/forgot/" + token)
			break
		}
		login := ""
		snap := s.store.Snapshot()
		if u := snap.UserByID(target); u != nil {
			login = u.Login
		}
		if token != "" && !s.sendForgotWipeMail(ctx, login, token) {
			flash.errText = "A recovery link was prepared but outbound mail is not available on this instance yet."
		}
	case "selfreg":
		enabled := r.FormValue("selfReg") == "on" || r.FormValue("selfReg") == "true"
		err = s.users.SetSelfRegEnabled(actorID, enabled)
	default:
		flash.errText = "Unknown action."
	}
	if err != nil {
		flash.errText = string(formError(err))
	} else if flash.errText == "" && !flash.escrowAsk && flash.wipeLink == "" {
		s.redirect(w, r, "/admin/")
		return
	}
	page := s.buildAdminUsersPage(sess, flash)
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) buildAdminUsersPage(sess *session.Session, flash adminUsersFlash) view.AdminUsersPage {
	snap := s.store.Snapshot()
	now := time.Now()
	page := view.AdminUsersPage{
		Shell: s.buildShell(sess, "Users"),
		Add: []view.Action{
			{ID: "admin.invite", Label: "Invite", Href: s.route("/admin/invites"), Method: "GET", Enabled: true},
		},
		SelfReg: s.plainForm("/admin/", "POST",
			csrfField(sess),
			view.FormRow{Name: "action", Kind: view.InputText, Value: text("selfreg")},
			view.FormRow{Name: "selfReg", Label: "Open registration", Kind: view.InputCheck,
				Value: text(boolStr(snap.Settings.SelfRegEnabled))},
		),
	}
	if flash.errText != "" {
		page.SelfReg.Sets[0].Rows[len(page.SelfReg.Sets[0].Rows)-1].Error = text(flash.errText)
	}
	if flash.wipeLink != "" {
		page.Shell.Notices = append(page.Shell.Notices, view.Notice{
			Role:  view.NoticeAlert,
			Title: text("One-time recovery link"),
			Body:  text("Pass this link to the account owner in person. It destroys encrypted connections and cannot be undone."),
			Actions: []view.Action{{
				ID: "admin.reset.link", Label: text(flash.wipeLink), Href: flash.wipeLink, Method: "GET", Enabled: true,
			}},
		})
	}
	if flash.escrowAsk {
		page.Shell.Notices = append(page.Shell.Notices, view.Notice{
			Role:  view.NoticeAlert,
			Title: text("Key escrow covers this account"),
			Body:  text("Password reset would destroy encrypted data. Use key escrow recovery instead."),
			Actions: []view.Action{{
				ID: "admin.escrow", Label: "Key escrow", Href: s.route("/admin/escrow"), Method: "GET", Enabled: true,
			}},
		})
	}
	tf := s.timeFor(sess)
	for _, u := range snap.Users {
		if len(u.LoginHash) == 0 && u.Disabled {
			continue
		}
		row := view.AdminUserRow{
			Login:    text(u.Login),
			Role:     text(string(u.Role)),
			Created:  tf.formatWhen(u.CreatedAt, false, now),
			Accounts: u.ConnCount,
			Sessions: s.sessions.LiveCount(u.ID),
			Disabled: u.Disabled,
			Escrowed: store.UserDeposited(&u),
			Menu:     s.adminUserMenu(sess, snap, &u),
		}
		if u.LastLogin != nil {
			row.LastLogin = tf.formatWhen(*u.LastLogin, true, now)
		}
		page.Rows = append(page.Rows, row)
	}
	return page
}

func (s *Server) adminUserMenu(sess *session.Session, snap store.State, u *store.User) []view.MenuItem {
	lastAdmin := u.Role == store.RoleAdmin && store.AdminCount(&snap) <= 1 && !u.Disabled
	menu := []view.MenuItem{
		{Action: view.Action{
			ID: "admin.user.sessions", Label: "End sessions",
			Href:   s.postQuery("/admin/", url.Values{"action": {"sessions"}, "target": {u.ID}}),
			Method: "POST", Enabled: s.sessions.LiveCount(u.ID) > 0,
		}},
	}
	if !u.Disabled {
		menu = append(menu, view.MenuItem{Action: view.Action{
			ID: "admin.user.disable", Label: "Disable",
			Href:   s.postQuery("/admin/", url.Values{"action": {"disable"}, "target": {u.ID}}),
			Method: "POST", Enabled: !lastAdmin, Reason: text("Cannot disable the last administrator."),
		}})
	}
	if u.Role == store.RoleUser {
		menu = append(menu, view.MenuItem{Action: view.Action{
			ID: "admin.user.role", Label: "Make administrator",
			Href:   s.postQuery("/admin/", url.Values{"action": {"role"}, "target": {u.ID}, "role": {"admin"}}),
			Method: "POST", Enabled: true,
		}})
	} else if !lastAdmin {
		menu = append(menu, view.MenuItem{Action: view.Action{
			ID: "admin.user.role", Label: "Make user",
			Href:   s.postQuery("/admin/", url.Values{"action": {"role"}, "target": {u.ID}, "role": {"user"}}),
			Method: "POST", Enabled: true,
		}})
	}
	menu = append(menu,
		view.MenuItem{Separator: true, Action: view.Action{
			ID: "admin.user.reset", Label: "Reset password",
			Href:   s.postQuery("/admin/", url.Values{"action": {"reset"}, "target": {u.ID}}),
			Method: "POST", Enabled: !u.Disabled, Danger: true,
			Reason: text("Destroys encrypted connections. Issues the same wipe link as /forgot."),
		}},
		view.MenuItem{Action: view.Action{
			ID: "admin.user.delete", Label: "Delete",
			Href:   s.postQuery("/admin/", url.Values{"action": {"delete"}, "target": {u.ID}}),
			Method: "POST", Enabled: !lastAdmin, Danger: true,
			Reason: text("Type the login to confirm. Cannot delete the last administrator."),
		}},
	)
	_ = sess
	return menu
}

type adminInvitesFlash struct {
	errText string
}

func (s *Server) getAdminInvites(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	page := s.buildAdminInvitesPage(sess, adminInvitesFlash{})
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) postAdminInvites(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	actorID := sess.AccountID()
	action := r.FormValue("action")
	var flash adminInvitesFlash
	var err error
	switch action {
	case "create", "resend":
		login := r.FormValue("login")
		email := r.FormValue("email")
		var secret string
		secret, err = s.users.CreateInvite(actorID, login, email)
		if err == nil && secret != "" {
			_ = s.sendInviteMail(ctx, login, email, secret)
		}
	case "revoke":
		err = s.users.RevokeInvite(actorID, r.FormValue("invite"))
	case "extend":
		err = s.users.ExtendInvite(actorID, r.FormValue("invite"))
	default:
		if action == "" && r.FormValue("login") != "" {
			var secret string
			secret, err = s.users.CreateInvite(actorID, r.FormValue("login"), r.FormValue("email"))
			if err == nil && secret != "" {
				_ = s.sendInviteMail(ctx, r.FormValue("login"), r.FormValue("email"), secret)
			}
		} else {
			flash.errText = "Unknown action."
		}
	}
	if err != nil {
		flash.errText = string(formError(err))
	}
	if flash.errText == "" {
		s.redirect(w, r, "/admin/invites")
		return
	}
	page := s.buildAdminInvitesPage(sess, flash)
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) buildAdminInvitesPage(sess *session.Session, flash adminInvitesFlash) view.AdminInvitesPage {
	snap := s.store.Snapshot()
	now := time.Now()
	page := view.AdminInvitesPage{
		Shell: s.buildShell(sess, "Invites"),
		Add: s.plainForm("/admin/invites", "POST",
			csrfField(sess),
			view.FormRow{Name: "action", Kind: view.InputText, Value: text("create")},
			view.FormRow{Name: "login", Label: "Login", Kind: view.InputText, Required: true},
			view.FormRow{Name: "email", Label: "Email", Kind: view.InputEmail},
		),
	}
	if flash.errText != "" {
		page.Add.Sets[0].Rows[len(page.Add.Sets[0].Rows)-1].Error = text(flash.errText)
	}
	tf := s.timeFor(sess)
	for i := range snap.Invites {
		inv := &snap.Invites[i]
		if !inviteLive(inv, now) {
			continue
		}
		row := view.InviteRow{
			Login:   text(inv.Login),
			Address: text(inv.Email),
			Sent:    inviteSentStatus(s.cfg.MailEnabled),
			Expires: tf.formatWhen(inv.ExpiresAt, false, now),
			Actions: []view.Action{
				{ID: "admin.invite.extend", Label: "Extend",
					Href:   s.postQuery("/admin/invites", url.Values{"action": {"extend"}, "invite": {inv.ID}}),
					Method: "POST", Enabled: true},
				{ID: "admin.invite.revoke", Label: "Revoke",
					Href:   s.postQuery("/admin/invites", url.Values{"action": {"revoke"}, "invite": {inv.ID}}),
					Method: "POST", Enabled: true, Danger: true},
				{ID: "admin.invite.resend", Label: "Send again",
					Href:   s.postQuery("/admin/invites", url.Values{"action": {"resend"}, "login": {inv.Login}, "email": {inv.Email}}),
					Method: "POST", Enabled: true},
			},
		}
		if inv.Secret != "" {
			row.Link = text(s.route("/invite/" + inv.Secret))
		}
		page.Rows = append(page.Rows, row)
	}
	return page
}

func inviteLive(inv *store.Invite, now time.Time) bool {
	return !inv.Revoked && !inv.Used && now.Before(inv.ExpiresAt)
}

func inviteSentStatus(mailEnabled bool) view.Text {
	if !mailEnabled {
		return text("SMTP not configured")
	}
	return text("SMTP not configured")
}

func (s *Server) getAdminSettings(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	page := view.AdminSettingsPage{Shell: s.buildShell(sess, "Instance settings")}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) postAdminSettings(w http.ResponseWriter, r *http.Request) {
	s.getAdminSettings(w, r)
}

func (s *Server) postAdminMailTest(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	result := view.MailTestResult{OK: false, Reply: text("SMTP is not configured.")}
	if WantsFragment(r) {
		_ = s.render.Fragment(w, r, http.StatusOK, TargetMain, result, nil)
		return
	}
	s.notFound(w, r)
}

func (s *Server) getAdminAudit(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	snap := s.store.Snapshot()
	now := time.Now()
	page := view.AdminAuditPage{Shell: s.buildShell(sess, "Audit log")}
	tf := s.timeFor(sess)
	for i := len(snap.Audit) - 1; i >= 0; i-- {
		e := snap.Audit[i]
		page.Rows = append(page.Rows, view.AuditRow{
			When:   tf.formatWhen(e.When, false, now),
			Who:    text(e.Who),
			What:   text(e.What),
			Whom:   text(e.Whom),
			Detail: text(e.Detail),
		})
	}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) getAdminDav(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	page := view.AdminDavPage{Shell: s.buildShell(sess, "DAV allowlist")}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) postAdminDav(w http.ResponseWriter, r *http.Request) {
	s.getAdminDav(w, r)
}

func (s *Server) getAdminInstall(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	page := view.AdminInstallPage{Shell: s.buildShell(sess, "Install")}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) postAdminInstall(w http.ResponseWriter, r *http.Request) {
	s.getAdminInstall(w, r)
}
