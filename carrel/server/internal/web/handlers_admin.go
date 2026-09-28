package web

import (
	"encoding/csv"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"strings"
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
	case "enable":
		err = s.users.EnableUser(actorID, target)
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
		if token != "" && !s.sendForgotWipeMail(r, login, token) {
			// The letter did not go: the administrator hands the link over,
			// as for an account without an address.
			flash.wipeLink = s.route("/forgot/" + token)
			flash.errText = "The letter with the recovery link could not be sent. Hand the link over in person."
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
	now := s.now()
	page := view.AdminUsersPage{
		Shell: s.withRail(s.buildShell(sess, "Users"), s.adminRail(sess, "/admin/")),
		Add: []view.Action{
			{ID: "admin.invite", Label: "Invite", Href: s.route("/admin/invites"), Method: "GET", Enabled: true},
		},
		SelfReg: withSubmit(s.plainForm("/admin/", "POST",
			csrfField(sess),
			view.FormRow{Name: "action", Kind: view.InputHidden, Value: text("selfreg")},
			view.FormRow{Name: "selfReg", Label: "Open registration", Kind: view.InputCheck,
				Value: text(boolStr(snap.Settings.SelfRegEnabled))},
		), "admin.selfreg.save", "Save"),
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
		// The frame writes the role by its word and both days absolute
		// (s-admin-2, C-10).
		row := view.AdminUserRow{
			Login:    text(u.Login),
			Role:     roleLabels[u.Role],
			Created:  view.When(tf.date(u.CreatedAt)),
			Accounts: u.ConnCount,
			Sessions: s.sessions.LiveCount(u.ID),
			Disabled: u.Disabled,
			Escrowed: store.UserDeposited(&u),
			Menu:     s.adminUserMenu(snap, &u),
		}
		if u.LastLogin != nil {
			row.LastLogin = tf.formatWhen(*u.LastLogin, false, now)
		}
		page.Rows = append(page.Rows, row)
	}
	return page
}

// roleLabels are the roles by the words of the list of s-admin-8.
var roleLabels = map[store.Role]view.Text{store.RoleUser: "User", store.RoleAdmin: "Administrator"}

// adminUserMenu is the «⋯» of a person's row (s-admin-12): nothing on the
// row itself — the actions are equal, none is the main one. What cannot be
// done to this person is there, off, with the reason (C-6.2). Reset
// password stays last above the line until the head's «Reset a password»
// has its person picker drawn (to-typecase task 16).
func (s *Server) adminUserMenu(snap store.State, u *store.User) []view.MenuItem {
	lastAdmin := u.Role == store.RoleAdmin && store.AdminCount(&snap) <= 1 && !u.Disabled
	op := func(action string) string {
		return s.postQuery("/admin/", url.Values{"action": {action}, "target": {u.ID}})
	}
	off := func(a view.Action, when bool, reason view.Text) view.Action {
		if when {
			a.Enabled, a.Reason = false, reason
		}
		return a
	}
	role := off(view.Action{ID: "admin.user.role", Label: "Set role", Method: "GET", Enabled: true,
		Href: s.postQuery("/admin/role", url.Values{"target": {u.ID}})},
		lastAdmin, "The last administrator cannot become a user.")
	sessions := off(view.Action{ID: "admin.user.sessions", Label: "Kill sessions", Href: op("sessions"), Method: "POST", Enabled: true},
		s.sessions.LiveCount(u.ID) == 0, "No live sessions.")
	toggle := off(view.Action{ID: "admin.user.disable", Label: "Disable", Href: op("disable"), Method: "POST", Enabled: true},
		lastAdmin, "Cannot disable the last administrator.")
	if u.Disabled {
		toggle = view.Action{ID: "admin.user.enable", Label: "Enable", Href: op("enable"), Method: "POST", Enabled: true}
	}
	reset := off(view.Action{ID: "admin.user.reset", Label: "Reset password", Href: op("reset"), Method: "POST", Danger: true, Enabled: true},
		u.Disabled, "Enable the account first: a disabled account gets no reset.")
	del := off(view.Action{ID: "admin.user.delete", Label: "Delete", Icon: "i-trash", Href: op("delete"), Method: "POST", Danger: true, Enabled: true},
		lastAdmin, "Cannot delete the last administrator.")
	return []view.MenuItem{{Action: role}, {Action: sessions}, {Action: toggle}, {Action: reset}, {Separator: true, Action: del}}
}

// getAdminUserRole is the sheet «Set role» opens: the roles of s-admin-8,
// the person's own chosen. The last administrator has none to choose.
func (s *Server) getAdminUserRole(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	snap := s.store.Snapshot()
	u := snap.UserByID(r.URL.Query().Get("target"))
	if u == nil || (len(u.LoginHash) == 0 && u.Disabled) {
		s.notFound(w, r)
		return
	}
	var opts []view.Option
	for _, role := range []store.Role{store.RoleUser, store.RoleAdmin} {
		opts = append(opts, view.Option{Value: string(role), Label: roleLabels[role], Selected: u.Role == role})
	}
	form := withSubmit(s.plainForm("/admin/?"+url.Values{"action": {"role"}, "target": {u.ID}}.Encode(), "POST",
		csrfField(sess),
		view.FormRow{Name: "role", Label: "Role", Kind: view.InputSelect, Options: opts, Required: true},
	), "admin.user.role.save", "Save",
		view.Action{ID: "admin.user.role.cancel", Label: "Cancel", Href: s.route("/admin/"), Method: "GET", Enabled: true})
	sheet := view.UserRoleSheet{Login: text(u.Login), Form: form}
	w.Header().Set("Cache-Control", "no-store")
	if WantsFragment(r) && FragmentTarget(r) == TargetSheet {
		_ = s.render.Fragment(w, r, http.StatusOK, TargetSheet, sheet, nil)
		return
	}
	s.writePage(w, r, http.StatusOK, sheet)
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
			_ = s.sendInviteMail(r, login, email, secret)
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
				_ = s.sendInviteMail(r, r.FormValue("login"), r.FormValue("email"), secret)
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
	now := s.now()
	page := view.AdminInvitesPage{
		Shell: s.withRail(s.buildShell(sess, "Invites"), s.adminRail(sess, "/admin/invites")),
		Add: withSubmit(s.plainForm("/admin/invites", "POST",
			csrfField(sess),
			view.FormRow{Name: "action", Kind: view.InputHidden, Value: text("create")},
			view.FormRow{Name: "login", Label: "Login", Kind: view.InputText, Required: true},
			view.FormRow{Name: "email", Label: "Email", Kind: view.InputEmail},
		), "admin.invite.create", "Create link"),
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
			// Delivery and Status are columns of s-admin-1: how it was
			// handed over, and pending — the list holds live ones only.
			Delivery: "link",
			Status:   "pending",
			Sent:     inviteSentStatus(inv, s.mailConfigured()),
			Expires:  tf.formatWhen(inv.ExpiresAt, false, now),
			// The rare rest is under «⋯» (s-admin-11): Extend when the
			// person did not make it in time, Revoke under the line.
			Menu: []view.MenuItem{
				{Action: view.Action{ID: "admin.invite.extend", Label: "Extend",
					Href:   s.postQuery("/admin/invites", url.Values{"action": {"extend"}, "invite": {inv.ID}}),
					Method: "POST", Enabled: true}},
				{Separator: true, Action: view.Action{ID: "admin.invite.revoke", Label: "Revoke",
					Href:   s.postQuery("/admin/invites", url.Values{"action": {"revoke"}, "invite": {inv.ID}}),
					Method: "POST", Enabled: true, Danger: true}},
			},
		}
		// On the row what an invitation is opened for (s-admin-1): Resend
		// for one that went by letter, and Copy link.
		if inv.Email != "" {
			row.Delivery = "email"
			resend := view.Action{ID: "admin.invite.resend", Label: "Resend", Method: "POST", Enabled: true,
				Href: s.postQuery("/admin/invites", url.Values{"action": {"resend"}, "login": {inv.Login}, "email": {inv.Email}})}
			if !s.mailConfigured() {
				resend.Enabled, resend.Reason = false, "SMTP is not configured: no letter can go."
			}
			row.Actions = append(row.Actions, resend)
		}
		copyLink := view.Action{ID: "admin.invite.copy", Label: "Copy link", Method: "GET", Enabled: true}
		if inv.Secret != "" {
			row.Link = text(s.route("/invite/" + inv.Secret))
			copyLink.Href = string(row.Link)
		} else {
			copyLink.Enabled, copyLink.Reason = false, "This invitation was made before links were kept. Resend it or make a new one."
		}
		row.Actions = append(row.Actions, copyLink)
		page.Rows = append(page.Rows, row)
	}
	return page
}

func inviteLive(inv *store.Invite, now time.Time) bool {
	return !inv.Revoked && !inv.Used && now.Before(inv.ExpiresAt)
}

// inviteSentStatus is how the invitation's letter went (C-6: "sent",
// "failed — " and the relay's reply, "SMTP not configured"); nothing for an
// invitation without an address — its link is handed over.
func inviteSentStatus(inv *store.Invite, configured bool) view.Text {
	switch {
	case inv.Email == "":
		return ""
	case inv.Mailed == "sent":
		return text("sent")
	case inv.Mailed == "failed" && inv.MailReply != "":
		// A failure is named by the relay's reply, not by the word alone
		// (s-admin-1): «failed — 550 mailbox unavailable».
		return text("failed — " + inv.MailReply)
	case inv.Mailed == "failed":
		return text("failed")
	case !configured:
		return text("SMTP not configured")
	}
	return ""
}

// auditSecurity are the lines about who may get in and what may read
// without a session: sign-ins, passwords, escrow, devices, links, wipes
// (s-admin-7 · Security).
var auditSecurity = []string{
	"login.", "password", "escrow", "device", "publish", "davloom", "session", "setup", "wipe", "invite",
	"connection.password", "backup.trigger", "dav.exercise", "user.",
}

func auditIsSecurity(what string) bool {
	for _, p := range auditSecurity {
		if strings.HasPrefix(what, p) {
			return true
		}
	}
	return false
}

// auditIsFailure is a line that says something did not happen.
func auditIsFailure(what string) bool {
	return strings.HasSuffix(what, ".fail") || strings.HasSuffix(what, ".failed") || strings.HasSuffix(what, ".refused")
}

func (s *Server) getAdminAudit(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	snap := s.store.Snapshot()
	now := s.now()
	show := r.URL.Query().Get("show")
	if show != "security" && show != "failures" {
		show = "all"
	}
	// The action is the code the journal writes (s-admin-10): login.ok,
	// login.fail, … — the same the Action column shows.
	action := r.URL.Query().Get("action")
	inShow := func(what string) bool {
		switch show {
		case "security":
			return auditIsSecurity(what)
		case "failures":
			return auditIsFailure(what)
		}
		return true
	}
	keep := func(what string) bool {
		return inShow(what) && (action == "" || what == action)
	}
	if r.URL.Query().Get("format") == "csv" {
		s.writeAuditCSV(w, snap.Audit, keep)
		return
	}
	page := view.AdminAuditPage{Shell: s.buildShell(sess, "Audit log")}
	page.Rail = s.adminRail(sess, "/admin/audit")
	tf := s.timeFor(sess)
	for i := len(snap.Audit) - 1; i >= 0; i-- {
		e := snap.Audit[i]
		if !keep(e.What) {
			continue
		}
		page.Rows = append(page.Rows, view.AuditRow{
			When:   tf.formatWhen(e.When, false, now),
			Who:    text(e.Who),
			What:   text(e.What),
			Whom:   text(e.Whom),
			Detail: text(e.Detail),
		})
	}
	page.Bar.Loaded = len(page.Rows)
	total := len(snap.Audit)
	page.Bar.Total = &total
	page.Filter = s.auditActionFilter(sess, snap.Audit, show, action, inShow)
	// s-admin-7 writes the segments without counts.
	for _, seg := range []struct{ id, label, value string }{
		{"audit.all", "All", "all"},
		{"audit.security", "Security", "security"},
		{"audit.failures", "Failures", "failures"},
	} {
		q := url.Values{}
		if seg.value != "all" {
			q.Set("show", seg.value)
		}
		if action != "" {
			q.Set("action", action)
		}
		page.Bar.Segments = append(page.Bar.Segments, view.Action{
			ID: seg.id, Label: text(seg.label), Method: "GET",
			Enabled: true, On: seg.value == show, Href: s.postQuery("/admin/audit", q),
		})
	}
	csv := url.Values{"format": {"csv"}}
	if show != "all" {
		csv.Set("show", show)
	}
	if action != "" {
		csv.Set("action", action)
	}
	page.Bar.Actions = []view.Action{{ID: "audit.export", Label: "Export CSV", Icon: "i-down", Method: "GET", Enabled: true,
		Href: s.postQuery("/admin/audit", csv)}}
	s.writePage(w, r, http.StatusOK, page)
}

// auditActionFilter is the choice of an action (s-admin-7, s-admin-10): All
// actions, a line, then the actions of the lines the chosen segment shows, in
// the order of their names. A chosen action the segment does not show stays
// in the list, so the choice is never lost from sight.
func (s *Server) auditActionFilter(sess *session.Session, entries []store.AuditEntry, show, action string, inShow func(string) bool) view.Form {
	seen := map[string]bool{}
	var codes []string
	for _, e := range entries {
		if inShow(e.What) && !seen[e.What] {
			seen[e.What] = true
			codes = append(codes, e.What)
		}
	}
	if action != "" && !seen[action] {
		codes = append(codes, action)
	}
	slices.Sort(codes)
	row := view.FormRow{Name: "action", Label: "Action", Kind: view.InputSelect, Value: text(action),
		Options: []view.Option{{Value: "", Label: "All actions", Selected: action == ""}}}
	for i, code := range codes {
		row.Options = append(row.Options, view.Option{Value: code, Label: text(code), Selected: code == action, Separator: i == 0})
	}
	rows := []view.FormRow{row}
	if show != "all" {
		rows = append(rows, view.FormRow{Name: "show", Kind: view.InputHidden, Value: text(show)})
	}
	return view.Form{Action: s.route("/admin/audit"), Method: "GET", Sets: []view.FieldSet{{Rows: rows}}}
}

// writeAuditCSV hands over what the screen shows, oldest first, as the
// journal holds it: the times in RFC 3339 and no zone of the reader's, so
// two exports can be compared.
func (s *Server) writeAuditCSV(w http.ResponseWriter, entries []store.AuditEntry, keep func(string) bool) {
	h := w.Header()
	h.Set("Content-Type", "text/csv; charset=utf-8")
	if v := mime.FormatMediaType("attachment", map[string]string{"filename": "carrel-audit.csv"}); v != "" {
		h.Set("Content-Disposition", v)
	} else {
		h.Set("Content-Disposition", "attachment")
	}
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "sandbox; default-src 'none'")
	h.Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusOK)
	out := csv.NewWriter(w)
	_ = out.Write([]string{"when", "who", "what", "whom", "detail"})
	for _, e := range entries {
		if !keep(e.What) {
			continue
		}
		if out.Write([]string{e.When.UTC().Format(time.RFC3339), e.Who, e.What, e.Whom, e.Detail}) != nil {
			return
		}
	}
	out.Flush()
}
