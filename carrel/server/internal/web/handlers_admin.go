package web

import (
	"encoding/csv"
	"errors"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

type adminUsersFlash struct {
	errText string
}

func (s *Server) getAdminUsers(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	page := s.buildAdminUsersPage(r, sess, adminUsersFlash{})
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) postAdminUsers(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
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
		// The toggle of s-admin-12 names the role it gives: pressing it
		// twice does not turn it back.
		role := store.RoleUser
		if r.FormValue("role") == string(store.RoleAdmin) {
			role = store.RoleAdmin
		}
		err = s.users.SetUserRole(actorID, target, role)
	case "sessions":
		err = s.users.EndUserSessions(actorID, target)
	default:
		flash.errText = "Unknown action."
	}
	if err != nil {
		flash.errText = string(formError(err))
	} else if flash.errText == "" {
		s.redirect(w, r, "/admin/")
		return
	}
	page := s.buildAdminUsersPage(r, sess, flash)
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) buildAdminUsersPage(r *http.Request, sess *session.Session, flash adminUsersFlash) view.AdminUsersPage {
	snap := s.store.Snapshot()
	page := view.AdminUsersPage{
		Shell: s.withRail(s.buildShell(sess, "Users"), s.adminRail(sess, "/admin/")),
		Add: []view.Action{
			// s-admin-2: «Add user» leads to the three ways of s-admin-1;
			// «Reset a password» opens the sheet of s-admin-13.
			{ID: "admin.invite", Label: "Add user", Icon: "i-plus", Href: s.route("/admin/invites"), Method: "GET", Primary: true, Enabled: true},
			{ID: "admin.reset", Label: "Reset a password", Href: s.route("/admin/reset"), Method: "GET", Enabled: true},
		},
	}
	if flash.errText != "" {
		page.Shell.Notices = append(page.Shell.Notices, view.Notice{Role: view.NoticeAlert, Body: text(flash.errText)})
	}
	tf := s.timeFor(sess)
	// What the list holds, above it (Ад5; s-admin-2), and the filter by
	// login and the segments All · Administrators · Disabled (Ад6): a GET
	// of the list, as a list of records sifts.
	q := r.URL.Query()
	sift := strings.ToLower(strings.TrimSpace(q.Get("filter")))
	show := q.Get("show")
	accounts, admins, covered := 0, 0, 0
	for _, u := range snap.Users {
		if len(u.LoginHash) == 0 && u.Disabled {
			continue
		}
		accounts++
		if u.Role == store.RoleAdmin {
			admins++
		}
		if store.UserDeposited(&u) {
			covered++
		}
	}
	page.Subtitle = text(plural(accounts, "account") + " · " + plural(admins, "administrator") + " · " + strconv.Itoa(covered) + " covered by escrow")
	for _, u := range snap.Users {
		if len(u.LoginHash) == 0 && u.Disabled {
			continue
		}
		switch {
		case sift != "" && !strings.Contains(strings.ToLower(u.Login), sift):
			continue
		case show == "admins" && u.Role != store.RoleAdmin:
			continue
		case show == "disabled" && !u.Disabled:
			continue
		}
		// The frame writes the role by its word and both days absolute
		// (s-admin-2, C-10).
		row := view.AdminUserRow{
			Login:    text(u.Login),
			Role:     roleLabels[u.Role],
			Created:  view.When(tf.numDate(u.CreatedAt)),
			Accounts: u.ConnCount,
			Sessions: s.sessions.LiveCount(u.ID),
			Disabled: u.Disabled,
			Escrowed: store.UserDeposited(&u),
			Menu:     s.adminUserMenu(snap, &u),
		}
		if u.LastLogin != nil {
			row.LastLogin = tf.numStamp(*u.LastLogin)
		}
		// Address is the confirmed address; one waiting for its link is
		// not the account's yet (s-admin-22).
		if u.EmailConfirmed {
			row.Address = text(u.Email)
		}
		page.Rows = append(page.Rows, row)
	}
	// The bar of s-admin-2: Columns (s-admin-21). The list has no query of
	// its own; a POST's action is not one to come back to.
	total := len(page.Rows)
	page.Bar = view.ListBar{Loaded: total, Total: &total,
		Columns: s.columnsMenu(r, csrfField(sess), "users", "/admin/columns", nil, false)}
	page.Bar.Filter, page.Bar.FilterForm = text(q.Get("filter")), s.listFilter("/admin/", url.Values{"show": {show}}, q.Get("filter"))
	if show != "" && page.Bar.FilterForm != nil {
		page.Bar.FilterForm.Sets[0].Rows = append([]view.FormRow{{Name: "show", Kind: view.InputHidden, Value: text(show)}}, page.Bar.FilterForm.Sets[0].Rows...)
	}
	for _, sg := range []struct{ id, label, show string }{{"admin.users.all", "All", ""}, {"admin.users.admins", "Administrators", "admins"}, {"admin.users.disabled", "Disabled", "disabled"}} {
		v := url.Values{}
		if sg.show != "" {
			v.Set("show", sg.show)
		}
		if f := q.Get("filter"); f != "" {
			v.Set("filter", f)
		}
		href := "/admin/"
		if len(v) > 0 {
			href += "?" + v.Encode()
		}
		page.Bar.Segments = append(page.Bar.Segments, view.Action{ID: sg.id, Label: text(sg.label), Href: s.route(href), Method: "GET", Enabled: true, On: sg.show == show})
	}
	return page
}

// roleLabels are the roles by the words of the list of s-admin-8.
var roleLabels = map[store.Role]view.Text{store.RoleUser: "User", store.RoleAdmin: "Administrator"}

// adminUserMenu is the «⋯» of a person's row (s-admin-12): nothing on the
// row itself — the actions are equal, none is the main one. There are two
// roles, so the role is one item that turns it over: Make administrator,
// Make user. What cannot be done to this person is there, off, with the
// reason (C-6.2). A password is reset from the head of the list
// (s-admin-13), not here.
func (s *Server) adminUserMenu(snap store.State, u *store.User) []view.MenuItem {
	lastAdmin := u.Role == store.RoleAdmin && store.AdminCount(&snap) <= 1 && !u.Disabled
	op := func(action string, more ...string) string {
		q := url.Values{"action": {action}, "target": {u.ID}}
		for i := 0; i+1 < len(more); i += 2 {
			q.Set(more[i], more[i+1])
		}
		return s.postQuery("/admin/", q)
	}
	off := func(a view.Action, when bool, reason view.Text) view.Action {
		if when {
			a.Enabled, a.Reason = false, reason
		}
		return a
	}
	role := view.Action{ID: "admin.user.role", Label: "Make administrator", Href: op("role", "role", string(store.RoleAdmin)), Method: "POST", Enabled: true}
	if u.Role == store.RoleAdmin {
		role = off(view.Action{ID: "admin.user.role", Label: "Make user", Href: op("role", "role", string(store.RoleUser)), Method: "POST", Enabled: true},
			lastAdmin, "Someone has to stay an administrator.")
	}
	sessions := off(view.Action{ID: "admin.user.sessions", Label: "Kill sessions", Href: op("sessions"), Method: "POST", Enabled: true},
		s.sessions.LiveCount(u.ID) == 0, "No live sessions.")
	toggle := off(view.Action{ID: "admin.user.disable", Label: "Disable", Href: op("disable"), Method: "POST", Enabled: true},
		lastAdmin, "Cannot disable the last administrator.")
	if u.Disabled {
		toggle = view.Action{ID: "admin.user.enable", Label: "Enable", Href: op("enable"), Method: "POST", Enabled: true}
	}
	del := off(view.Action{ID: "admin.user.delete", Label: "Delete", Icon: "i-trash", Href: op("delete"), Method: "POST", Danger: true, Enabled: true},
		lastAdmin, "Cannot delete the last administrator.")
	// The account is deleted once its login is typed in a dialogue of its
	// own (s-admin-2); that dialogue has no frame yet (Typecase's task 195),
	// and the item alone sends no login — the store refuses it every time
	// (Ад7). Off, with the reason, until the dialogue is drawn.
	del = off(del, del.Enabled, "Deleting an account needs its own dialogue, which is not here yet.")
	return []view.MenuItem{{Action: role}, {Action: sessions}, {Action: toggle}, {Separator: true, Action: del}}
}

// resetWarning is what a reset destroys, said before the button (S-10.6):
// the wipe of S-10.4, whole.
const resetWarning = "The passwords this person saved for DAV servers are sealed with their own password, and a reset destroys them: " +
	"every account they connected has to be connected again, and their published links, devices and external backup triggers stop. " +
	"Their records on the servers stay as they are."

// resetHint is the hint under the temporary password (s-admin-13): who hands
// it over, and the way that destroys nothing.
const resetHint = "Hand it over yourself; they choose their own at the next sign-in. " +
	"If their key is deposited, recover the account in Key escrow instead: nothing is destroyed then."

// getAdminReset is the sheet «Reset a password» opens from the head of the
// people's list (s-admin-13); ?target= chooses the person.
func (s *Server) getAdminReset(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	s.writeResetSheet(w, r, s.resetSheet(sess, r.URL.Query().Get("target"), s.users.SuggestPassword(), nil))
}

// postAdminReset resets the chosen person's password to the temporary one
// typed, and shows it this once; a refusal stands at its field and changes
// nothing.
func (s *Server) postAdminReset(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	target, temp := r.FormValue("target"), r.FormValue("password")
	snap := s.store.Snapshot()
	u := snap.UserByID(target)
	if why := resetRefusal(sess, u); why != "" {
		s.writeResetSheet(w, r, s.resetSheet(sess, target, temp, map[string]view.Text{"target": why}))
		return
	}
	// The login typed is the confirmation (owner's decision 28.09.2026): a
	// reset destroys what the person had, and a slip of the list must not.
	if strings.TrimSpace(r.FormValue("confirm")) != u.Login {
		s.writeResetSheet(w, r, s.resetSheet(sess, target, temp,
			map[string]view.Text{"confirm": text("Type " + u.Login + " to confirm whose password is reset.")}))
		return
	}
	if err := s.users.AdminResetPassword(r.Context(), sess.AccountID(), target, temp); err != nil {
		field := "target"
		if errors.Is(err, user.ErrPasswordShort) {
			field = "password"
		}
		s.writeResetSheet(w, r, s.resetSheet(sess, target, temp, map[string]view.Text{field: formError(err)}))
		return
	}
	// Shown this once (s-admin-14): the answer to the POST, not a page to
	// come back to — the message, the password with Copy, and Done; the
	// form and the warning have done their work.
	s.writeResetSheet(w, r, view.PasswordResetSheet{
		Title: "Reset a password",
		Issued: &view.IssuedPassword{
			Login:    text(u.Login),
			Password: text(temp),
			Copy:     view.Action{ID: "admin.reset.copy", Label: "Copy", Icon: "i-dup", Method: "GET", Primary: true, Enabled: true},
			Message:  join("The password of ", strongRun(u.Login), " is reset."),
			Note:     "Hand it over yourself; they choose their own at the next sign-in.",
			Done:     view.Action{ID: "admin.reset.done", Label: "Done", Href: s.route("/admin/"), Method: "GET", Primary: true, Enabled: true},
		},
		Actions: []view.Action{},
	})
}

// resetRefusal is why a person is not reset here: nobody, oneself — whose
// password is changed in their own account — or a disabled account.
func resetRefusal(sess *session.Session, u *store.User) view.Text {
	switch {
	case u == nil || (len(u.LoginHash) == 0 && u.Disabled):
		return "Choose whose password to reset."
	case u.ID == sess.AccountID():
		return "Your own password is changed in your account settings."
	case u.Disabled:
		return "Enable the account first: a disabled account gets no reset."
	}
	return ""
}

// resetSheet is s-admin-13: Whose — the people of the list by login and
// role, those who cannot be reset here off with the reason — the warning,
// the temporary password, Reset password and Cancel.
func (s *Server) resetSheet(sess *session.Session, target, temp string, errs map[string]view.Text) view.PasswordResetSheet {
	snap := s.store.Snapshot()
	var people []view.Option
	for i := range snap.Users {
		u := &snap.Users[i]
		if len(u.LoginHash) == 0 && u.Disabled {
			continue
		}
		o := view.Option{Value: u.ID, Label: text(u.Login + " · " + string(roleLabels[u.Role])), Selected: u.ID == target}
		if why := resetRefusal(sess, u); why != "" {
			o.Disabled, o.Reason, o.Selected = true, why, false
		}
		people = append(people, o)
	}
	form := s.plainForm("/admin/reset", "POST",
		csrfField(sess),
		view.FormRow{Name: "target", Label: "Whose", Kind: view.InputSelect, Options: people, Required: true, Error: errs["target"]},
		view.FormRow{Name: "password", Label: "Temporary password", Kind: view.InputText, Required: true, Value: text(temp), Mono: true,
			Hint: resetHint, Error: errs["password"]},
		view.FormRow{Name: "confirm", Label: "Type the login to confirm", Kind: view.InputText, Required: true,
			Error: errs["confirm"], Placeholder: text(chosenLogin(people))},
	)
	// The caret is in the login typed to confirm, the one field left to
	// fill (s-admin-13).
	form = focused(form, "confirm")
	return view.PasswordResetSheet{
		Title:   "Reset a password",
		Form:    form,
		Warning: resetWarning,
		Actions: []view.Action{
			{ID: "admin.reset.run", Label: "Reset password", Href: form.Action, Method: "POST", Danger: true, Enabled: true},
			{ID: "admin.reset.cancel", Label: "Cancel", Href: s.route("/admin/"), Method: "GET", Enabled: true},
		},
	}
}

func (s *Server) writeResetSheet(w http.ResponseWriter, r *http.Request, sheet view.PasswordResetSheet) {
	w.Header().Set("Cache-Control", "no-store")
	if render.WantsFragment(r) && render.FragmentTarget(r) == render.TargetSheet {
		_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, sheet, nil)
		return
	}
	sheet.Under = s.underOf(r, "", "/admin/") // over Users, s-admin-13 (Лс29)
	s.writePage(w, r, http.StatusOK, sheet)
}

// adminInvitesFlash is what one answer of «Add users» says besides the
// page: an error on the field it belongs to, what was typed, and the
// temporary password just set — shown this once.
type adminInvitesFlash struct {
	errText    string // not a field's: a notice over the page
	inviteErr  view.Text
	userErr    view.Text
	userField  string // the temporary form's row the error stands on
	selfRegErr view.Text
	typed      url.Values
	issued     *view.IssuedPassword
}

func (s *Server) getAdminInvites(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	page := s.buildAdminInvitesPage(sess, r.URL.Query().Get("invite"), adminInvitesFlash{})
	s.writePage(w, r, http.StatusOK, page)
}

// postAdminInvites is every way of s-admin-1: an invitation by link or by
// letter, the temporary password, self-registration, and what a pending
// invitation's row does.
func (s *Server) postAdminInvites(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	actorID := sess.AccountID()
	flash := adminInvitesFlash{typed: url.Values{}}
	for _, k := range []string{"role", "login", "email"} {
		flash.typed.Set(k, r.FormValue(k))
	}
	var err error
	switch action := r.FormValue("action"); action {
	case "link", "send":
		// The role is chosen first; a letter needs its address, a link
		// none — any typed beside it is not sent.
		email := ""
		if action == "send" {
			// The button is off without a relay; the server does not take
			// it either — an invitation by letter that cannot go is a link.
			if !s.mailConfigured() {
				flash.inviteErr = "SMTP is not configured: no letter can go."
				break
			}
			if email = strings.TrimSpace(r.FormValue("email")); email == "" {
				flash.inviteErr = text(user.ErrBadAddress.Error())
				break
			}
		}
		var secret string
		if secret, err = s.users.Invite(actorID, email, formRole(r)); err != nil {
			flash.inviteErr = formError(err)
			break
		}
		if email != "" {
			_ = s.sendInviteMail(r, "", email, secret)
		}
		// The link stands in the card that made it (s-admin-1).
		s.redirect(w, r, "/admin/invites?"+url.Values{"invite": {s.inviteIDBySecret(secret)}}.Encode())
		return
	case "user":
		password := r.FormValue("password")
		login := normalizeLogin(r.FormValue("login"))
		err = s.users.CreateUser(r.Context(), actorID, login, r.FormValue("email"), password, formRole(r))
		if err != nil {
			flash.userErr = formError(err)
			switch {
			case errors.Is(err, user.ErrPasswordShort):
				flash.userField = "password"
			case errors.Is(err, user.ErrBadAddress):
				flash.userField = "email"
			default:
				flash.userField = "login"
			}
			break
		}
		// Shown this once (s-admin-17): the answer to the POST, not a page
		// to come back to. The form is empty again, with a new password to
		// offer.
		flash.typed = url.Values{}
		flash.issued = &view.IssuedPassword{
			Login:    text(login),
			Password: text(password),
			Copy:     view.Action{ID: "admin.user.password.copy", Label: "Copy", Icon: "i-dup", Method: "GET", Primary: true, Enabled: true},
			Message:  join("The user ", strongRun(login), " is created. Hand this over; until they change it, you can sign in as them."),
			Done:     view.Action{ID: "admin.user.password.done", Label: "Done", Href: s.route("/admin/invites"), Method: "GET", Enabled: true},
		}
	case "selfreg":
		enabled := r.FormValue("selfReg") == "on" || r.FormValue("selfReg") == "true"
		if err = s.users.SetSelfRegEnabled(actorID, enabled); err != nil {
			flash.selfRegErr = formError(err)
		}
	case "resend":
		var secret, email string
		if secret, email, err = s.users.ResendInvite(actorID, r.FormValue("invite")); err == nil {
			login := ""
			snap := s.store.Snapshot()
			for i := range snap.Invites {
				if snap.Invites[i].ID == r.FormValue("invite") {
					login = snap.Invites[i].Login
				}
			}
			_ = s.sendInviteMail(r, login, email, secret)
		}
	case "revoke":
		err = s.users.RevokeInvite(actorID, r.FormValue("invite"))
	case "extend":
		err = s.users.ExtendInvite(actorID, r.FormValue("invite"))
	default:
		flash.errText = "Unknown action."
	}
	if err != nil && flash.inviteErr == "" && flash.userErr == "" && flash.selfRegErr == "" {
		flash.errText = string(formError(err))
	}
	if flash.errText == "" && flash.inviteErr == "" && flash.userErr == "" && flash.selfRegErr == "" && flash.issued == nil {
		s.redirect(w, r, "/admin/invites")
		return
	}
	page := s.buildAdminInvitesPage(sess, "", flash)
	s.writePage(w, r, http.StatusOK, page)
}

// formRole is the role a form chose (s-admin-8); anything else is a user.
func formRole(r *http.Request) store.Role {
	if r.FormValue("role") == string(store.RoleAdmin) {
		return store.RoleAdmin
	}
	return store.RoleUser
}

// inviteIDBySecret names the invitation a secret was just issued for.
func (s *Server) inviteIDBySecret(secret string) string {
	snap := s.store.Snapshot()
	for i := range snap.Invites {
		if snap.Invites[i].Secret == secret {
			return snap.Invites[i].ID
		}
	}
	return ""
}

// roleRow is the Role field of s-admin-1: the list of s-admin-8, User
// unless another was chosen.
func roleRow(chosen string) view.FormRow {
	var opts []view.Option
	for _, role := range []store.Role{store.RoleUser, store.RoleAdmin} {
		opts = append(opts, view.Option{Value: string(role), Label: roleLabels[role],
			Selected: string(role) == chosen || (chosen != string(store.RoleAdmin) && role == store.RoleUser)})
	}
	return view.FormRow{Name: "role", Label: "Role", Kind: view.InputSelect, Options: opts, Required: true}
}

func (s *Server) buildAdminInvitesPage(sess *session.Session, made string, flash adminInvitesFlash) view.AdminInvitesPage {
	snap := s.store.Snapshot()
	now := s.now()
	mail := s.mailConfigured()
	typed := flash.typed
	if typed == nil {
		typed = url.Values{}
	}
	page := view.AdminInvitesPage{
		Shell:    s.withRail(s.buildShell(sess, "Add users"), s.adminRail(sess, "/admin/invites")),
		Subtitle: "Three ways, for three situations. There is no global mode: any of them works.",
	}
	if flash.errText != "" {
		page.Shell.Notices = append(page.Shell.Notices, view.Notice{Role: view.NoticeAlert, Body: text(flash.errText)})
	}

	// Invitation: the person chooses their own password — and login.
	send := view.Action{ID: "admin.invite.send", Label: "Send invitation", Method: "POST", Enabled: true,
		Href: s.postQuery("/admin/invites", url.Values{"action": {"send"}})}
	if !mail {
		send.Enabled, send.Reason = false, "SMTP is not configured: no letter can go."
	}
	invite := s.plainForm("/admin/invites?action=link", "POST",
		csrfField(sess),
		roleRow(typed.Get("role")),
		view.FormRow{Name: "email", Label: "Or send to", Kind: view.InputEmail, Value: text(typed.Get("email")),
			Placeholder: "anna@example.org", Error: flash.inviteErr},
	)
	invite.Actions = []view.Action{
		{ID: "admin.invite.create", Label: "Create link", Method: "POST", Primary: true, Enabled: true,
			Href: s.postQuery("/admin/invites", url.Values{"action": {"link"}})},
		send,
	}
	page.Invite = view.AddUserCard{Title: "Invitation", Aside: "chooses their own password", Form: invite}
	if made != "" {
		for i := range snap.Invites {
			inv := &snap.Invites[i]
			if inv.ID == made && inviteLive(inv, now) && inv.Secret != "" {
				page.Invite.Link = text(s.route("/invite/" + inv.Secret))
				page.Invite.Copy = &view.Action{ID: "admin.invite.copy", Label: "Copy", Href: string(page.Invite.Link), Method: "GET", Enabled: true}
			}
		}
	}

	// Temporary password: handed over in person, and said so.
	temp := view.FormRow{Name: "password", Label: "Temporary password", Kind: view.InputText, Required: true, Mono: true,
		Value: text(s.users.SuggestPassword())}
	rows := []view.FormRow{
		csrfField(sess),
		{Name: "action", Kind: view.InputHidden, Value: "user"},
		roleRow(typed.Get("role")),
		{Name: "login", Label: "Login", Kind: view.InputText, Required: true, Value: text(typed.Get("login"))},
		{Name: "email", Label: "Email", LabelAside: "(optional)", Kind: view.InputEmail, Value: text(typed.Get("email")), Placeholder: "—"},
		temp,
	}
	for i := range rows {
		if flash.userErr != "" && rows[i].Name == flash.userField {
			rows[i].Error = flash.userErr
		}
	}
	page.Temporary = view.AddUserCard{
		Title: "Temporary password",
		Badge: &view.Badge{Label: "hand to hand", Role: view.BadgeAlert},
		Warning: &view.Notice{Role: view.NoticeAlert,
			Body: "Until the user changes this password, you can sign in as them."},
		Form:   withPrimary(s.plainForm("/admin/invites", "POST", rows...), "admin.user.create", "Create user"),
		Issued: flash.issued,
	}

	// Self-registration: a public form, only while mail works.
	save := view.Action{ID: "admin.selfreg.save", Label: "Save", Href: s.route("/admin/invites"), Method: "POST", Enabled: true}
	if !snap.Settings.SelfRegEnabled && !mail {
		save.Enabled, save.Reason = false, "Mail is not set up: nobody who registers could confirm the address."
	}
	selfReg := s.plainForm("/admin/invites", "POST",
		csrfField(sess),
		view.FormRow{Name: "action", Kind: view.InputHidden, Value: "selfreg"},
		view.FormRow{Name: "selfReg", Label: "Publish the registration form", Kind: view.InputCheck,
			Value: text(boolStr(snap.Settings.SelfRegEnabled)), Error: flash.selfRegErr},
	)
	selfReg.Actions = []view.Action{save}
	page.SelfReg = view.AddUserCard{
		Title: "Self-registration",
		Lede:  "A visitor creates an account and confirms the address by email. Cannot be turned on until mail works.",
		Form:  selfReg,
	}
	if snap.Settings.SelfRegEnabled {
		page.SelfReg.Badge = &view.Badge{Label: "open", Role: view.BadgeNeutral}
		page.SelfReg.Link = text(s.route("/register"))
		page.SelfReg.Copy = &view.Action{ID: "admin.selfreg.copy", Label: "Copy", Href: string(page.SelfReg.Link), Method: "GET", Enabled: true}
	}

	tf := s.timeFor(sess)
	for i := range snap.Invites {
		inv := &snap.Invites[i]
		if !inviteLive(inv, now) {
			continue
		}
		row := view.InviteRow{
			// The first column is the address: the person chooses the
			// login, and a link has no address yet (s-admin-1).
			Address: "—",
			// Delivery and Status are columns of s-admin-1: how it was
			// handed over, and pending — the list holds live ones only.
			Delivery: "link",
			Status:   "pending",
			Sent:     inviteSentStatus(inv, mail),
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
		// for one that went by letter — a new secret, the old link dies —
		// and Copy link.
		if inv.Email != "" {
			row.Address, row.Delivery = text(inv.Email), "email"
			resend := view.Action{ID: "admin.invite.resend", Label: "Resend", Method: "POST", Enabled: true,
				Href: s.postQuery("/admin/invites", url.Values{"action": {"resend"}, "invite": {inv.ID}})}
			if !mail {
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
// "failed — " and the relay's reply, "SMTP not configured"); «—» for an
// invitation without an address — its link is handed over (s-admin-1).
func inviteSentStatus(inv *store.Invite, configured bool) view.Text {
	switch {
	case inv.Email == "":
		return "—"
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
		// The moment in numbers, as the frame writes it (Ад3; s-admin-7);
		// an action on oneself has no target but a dash (Ад26).
		whom := e.Whom
		if whom == e.Who {
			whom = "—"
		}
		page.Rows = append(page.Rows, view.AuditRow{
			When:   tf.numStamp(e.When),
			Who:    text(e.Who),
			What:   text(e.What),
			Whom:   text(whom),
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
	page.Bar.Actions = []view.Action{{ID: "audit.export", Label: "Export CSV", Method: "GET", Enabled: true,
		Href: s.postQuery("/admin/audit", csv)}}
	// Columns (s-admin-23), in the bar of s-admin-7 too; Export CSV hands
	// over every column whatever is shown.
	page.Bar.Columns = s.columnsMenu(r, csrfField(sess), "audit", "/admin/audit/columns", r.URL.Query(), false)
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

// chosenLogin is the login of the person a reset sheet has chosen: what the
// field that confirms it shows before anything is typed (Ад9; s-admin-13).
func chosenLogin(people []view.Option) string {
	for _, o := range people {
		if o.Selected {
			login, _, _ := strings.Cut(string(o.Label), " · ") // «anna · User · …»
			return login
		}
	}
	return ""
}
