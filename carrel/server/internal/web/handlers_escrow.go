package web

import (
	"net/http"
	"net/url"
	"time"

	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

type adminEscrowFlash struct {
	formErr    string
	masterErr  string
	recoverErr string
	result     *view.EscrowRecoveryResult
}

func (s *Server) getSettingsEscrow(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	page := s.buildEscrowPage(sess, "")
	s.writePage(w, r, http.StatusOK, page)
}

// postSettingsEscrow deposits or withdraws. The deposit sheet (account-2)
// carries the password: it opens the same unlock window as every release
// of a secret (S-10.7), under the same buckets, and deposits at once.
// Without a password the window has to be open already. Withdrawal asks no
// password — it releases nothing — and comes from its own sheet.
func (s *Server) postSettingsEscrow(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	action := r.FormValue("action")
	switch action {
	case "deposit":
		if password := r.FormValue("password"); password != "" {
			start := time.Now()
			if !s.limit.allowLogin(sess.Login(), s.clientIP(r), start) {
				http.Error(w, "too many requests", http.StatusTooManyRequests)
				return
			}
			if !s.users.Unlock(r.Context(), sess.AccountID(), password) {
				constantTimeDelay(start, 300*time.Millisecond)
				s.depositRefused(w, r, sess, "Password is wrong.")
				return
			}
			sess.SetUnlockUntil(time.Now().Add(s.cfg.UnlockTTL))
		}
		if sess.UnlockUntil(time.Now()) == nil {
			s.depositRefused(w, r, sess, "Confirm your password first.")
			return
		}
		key, ok := sess.DataKey()
		if !ok {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if err := s.users.DepositEscrow(sess.AccountID(), key); err != nil {
			s.depositRefused(w, r, sess, string(formError(err)))
			return
		}
	case "withdraw":
		if err := s.users.WithdrawEscrow(sess.AccountID()); err != nil {
			page := s.buildEscrowPage(sess, string(formError(err)))
			s.writePage(w, r, http.StatusOK, page)
			return
		}
	}
	s.redirect(w, r, "/app/settings/escrow")
}

// depositRefused answers the deposit sheet with its error: the sheet again
// when it was asked as one, the escrow page with it otherwise.
func (s *Server) depositRefused(w http.ResponseWriter, r *http.Request, sess *session.Session, errText string) {
	if WantsFragment(r) {
		_ = s.render.Fragment(w, r, http.StatusOK, TargetSheet, s.depositSheet(sess, text(errText)), nil)
		return
	}
	s.writePage(w, r, http.StatusOK, s.buildEscrowPage(sess, errText))
}

// getEscrowDeposit is «Deposit my key» over Account (account-2): what
// happens, and who is pressing.
func (s *Server) getEscrowDeposit(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	snap := s.store.Snapshot()
	if u := snap.UserByID(sess.AccountID()); u == nil || !store.EscrowActive(&snap) || store.UserDeposited(u) {
		s.notFound(w, r)
		return
	}
	sheet := s.depositSheet(sess, "")
	if WantsFragment(r) {
		_ = s.render.Fragment(w, r, http.StatusOK, TargetSheet, sheet, nil)
		return
	}
	s.writePage(w, r, http.StatusOK, sheet)
}

// getEscrowWithdraw is the sheet before the deposited copy is destroyed
// (account-7): nothing is taken away until its button.
func (s *Server) getEscrowWithdraw(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	snap := s.store.Snapshot()
	if u := snap.UserByID(sess.AccountID()); u == nil || !store.UserDeposited(u) {
		s.notFound(w, r)
		return
	}
	sheet := view.EscrowWithdrawSheet{
		Warning: escrowWithdrawNote,
		Form: s.plainForm("/app/settings/escrow", "POST", csrfField(sess),
			view.FormRow{Name: "action", Kind: view.InputHidden, Value: "withdraw"}),
		Actions: []view.Action{
			{ID: "escrow.withdraw", Label: "Withdraw my deposited key", Method: "POST", Danger: true, Enabled: true,
				Href: s.postQuery("/app/settings/escrow", url.Values{"action": {"withdraw"}})},
			{ID: "escrow.withdraw.cancel", Label: "Cancel", Href: s.route("/app/settings/account"), Method: "GET", Enabled: true},
		},
	}
	if WantsFragment(r) {
		_ = s.render.Fragment(w, r, http.StatusOK, TargetSheet, sheet, nil)
		return
	}
	s.writePage(w, r, http.StatusOK, sheet)
}

// escrowWithdrawNote is the line beside the withdraw button (account-4) and
// the warning of its sheet.
const escrowWithdrawNote view.Text = "After withdrawal nobody can recover this account, including you."

// depositSheet is the unlock sheet with the deposit's purpose and button
// (account-2): the password it asks opens the one window of S-10.7.
func (s *Server) depositSheet(sess *session.Session, errText view.Text) view.UnlockSheet {
	sheet := view.UnlockSheet{
		Purpose: "Deposit my key",
		Warning: "An administrator who holds the escrow master password will be able to decrypt the DAV credentials you save here, and through them your contacts, calendars, tasks, notes and files. Ending sessions does not undo this. You can withdraw the copy later from Account.",
		Form: withSubmit(s.plainForm("/app/settings/escrow", "POST",
			csrfField(sess),
			view.FormRow{Name: "action", Kind: view.InputHidden, Value: "deposit"},
			view.FormRow{Name: "password", Label: "Your Carrel password", Kind: view.InputPassword, Required: true, Error: errText,
				Hint: "Asked because the key is being handed over, not just looked at. Same unlock as showing a saved password."},
		), "escrow.deposit", "Deposit my key",
			view.Action{ID: "escrow.deposit.cancel", Label: "Cancel", Href: s.route("/app/settings/account"), Method: "GET", Enabled: true}),
	}
	if until := sess.UnlockUntil(time.Now()); until != nil {
		sheet.Until = ptrWhen(s.timeFor(sess).formatWhen(*until, false, time.Now()))
	}
	return sheet
}

// escrowStatus is Key escrow as the person sees it: on the account page in
// the words of account-1 and account-7, on its own page in those of
// account-4. The deposit and the last recovery are two values, not one.
func (s *Server) escrowStatus(sess *session.Session, u *store.User, onAccount bool) view.EscrowStatus {
	snap := s.store.Snapshot()
	active := store.EscrowActive(&snap)
	status := view.EscrowStatus{Active: active, Note: "Key escrow is disabled on this instance.", Actions: []view.Action{}}
	if u != nil && store.UserDeposited(u) {
		tf := s.timeFor(sess)
		status.Covered = true
		status.By = "administrator with master password"
		if u.EscrowDepositedAt != nil {
			status.Deposited = view.When(tf.dayKey(*u.EscrowDepositedAt))
			status.Badge = &view.Badge{Label: text("deposited " + string(status.Deposited)), Role: view.BadgeAlert}
		}
		status.LastRecovery = "never"
		if u.EscrowLastRecovery != nil {
			status.LastRecovery = view.When(tf.dayKey(*u.EscrowLastRecovery))
		}
		status.Note = "A copy of your data key is deposited on this server. An administrator who holds the escrow master password can decrypt the DAV credentials you save here, and through them your contacts and calendars."
		if onAccount {
			status.Note = "A copy of your data key is deposited on this server. If you lose your password, an administrator who holds the escrow master password can recover the account — and, with it, can read the DAV credentials you save here."
		}
		status.ActionNote = escrowWithdrawNote
		status.Actions = []view.Action{{
			ID: "escrow.withdraw", Label: "Withdraw my deposited key", Danger: true,
			Href: s.route("/app/settings/escrow/withdraw"), Method: "GET", Enabled: true,
		}}
		return status
	}
	if !active {
		return status
	}
	status.Note = "An administrator who holds the escrow master password can decrypt the DAV credentials you save here, and through them your contacts and calendars. This is not backup."
	if onAccount {
		status.Note = "No copy of your data key is deposited. Nobody but you can decrypt your data — and if you lose your password, it cannot be recovered by anyone. Depositing asks for your Carrel password first: it is the same unlock that shows a saved password, and for the same reason — the key is being handed over, not just looked at."
	}
	status.Actions = []view.Action{{
		ID: "escrow.deposit", Label: "Deposit my key",
		Href: s.route("/app/settings/escrow/deposit"), Method: "GET", Enabled: true,
	}}
	return status
}

func (s *Server) buildEscrowPage(sess *session.Session, errText string) view.EscrowPage {
	snap := s.store.Snapshot()
	u := snap.UserByID(sess.AccountID())
	page := view.EscrowPage{
		Shell:  s.withRail(s.buildShell(sess, "Key escrow"), s.settingsRail(sess, "/app/settings/escrow")),
		Status: s.escrowStatus(sess, u, false),
	}
	if u != nil && store.UserDeposited(u) {
		page.Subtitle = "A copy of your key on this server"
	}
	if store.EscrowActive(&snap) && u != nil && !store.UserDeposited(u) {
		sheet := s.depositSheet(sess, text(errText))
		page.Confirm = &sheet
	}
	return page
}

func (s *Server) getAdminEscrow(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	page := s.buildAdminEscrowPage(sess, adminEscrowFlash{})
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) postAdminEscrow(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireAdmin(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	action := r.FormValue("action")
	var flash adminEscrowFlash
	switch action {
	case "enable":
		err := s.users.EnableEscrow(ctx, sess.AccountID(),
			r.FormValue("master"), r.FormValue("master2"))
		if err != nil {
			flash.formErr = string(formError(err))
		}
	case "policy":
		cover := r.FormValue("coverNew") == "on" || r.FormValue("coverNew") == "true"
		if err := s.users.SetEscrowCoverNew(sess.AccountID(), cover); err != nil {
			flash.formErr = string(formError(err))
		}
	case "master":
		err := s.users.ChangeEscrowMaster(ctx, sess.AccountID(),
			r.FormValue("current"), r.FormValue("master"), r.FormValue("master2"))
		if err != nil {
			flash.masterErr = string(formError(err))
		}
	case "recover":
		login := r.FormValue("login")
		if !s.limit.allowLogin(login, s.clientIP(r), time.Now()) {
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		temp, err := s.users.RecoverEscrow(ctx, sess.AccountID(), login, r.FormValue("master"))
		if err != nil {
			flash.recoverErr = string(formError(err))
		} else {
			mailed := s.sendEscrowRecoveryMail(r, login, sess.Login())
			flash.result = &view.EscrowRecoveryResult{
				Login:    text(login),
				Password: text(temp),
				Copy: view.Action{
					ID: "admin.escrow.copy", Label: "Copy", Method: "GET", Enabled: true,
				},
				Mailed: s.escrowRecoveryMailedStatus(login, mailed),
			}
			page := s.buildAdminEscrowPage(sess, flash)
			s.writePage(w, r, http.StatusOK, page)
			return
		}
	default:
		flash.formErr = "Unknown action."
	}
	if flash.formErr != "" || flash.masterErr != "" || flash.recoverErr != "" {
		page := s.buildAdminEscrowPage(sess, flash)
		s.writePage(w, r, http.StatusOK, page)
		return
	}
	s.redirect(w, r, "/admin/escrow")
}

func (s *Server) escrowRecoveryMailedStatus(login string, sent bool) view.Text {
	switch {
	case sent:
		return text("sent")
	case !s.mailConfigured():
		return text("SMTP not configured")
	case s.users.ConfirmedAddress(login) == "":
		return text("no confirmed address")
	}
	return text("failed")
}

func (s *Server) buildAdminEscrowPage(sess *session.Session, flash adminEscrowFlash) view.AdminEscrowPage {
	snap := s.store.Snapshot()
	enabled := store.EscrowActive(&snap)
	page := view.AdminEscrowPage{
		Shell:   s.withRail(s.buildShell(sess, "Key escrow"), s.adminRail(sess, "/admin/escrow")),
		Enabled: enabled,
		Warning: view.Notice{
			Role:  view.NoticeAlert,
			Title: text("Administrative access to saved credentials"),
			Body:  text("Key escrow is not a key backup. It is a way in: an administrator holding the master password can decrypt a covered user's saved DAV credentials, and through them their contacts and calendars. Every covered user is told so in their profile, and every recovery is logged and mailed to them."),
		},
		Result: flash.result,
	}
	if enabled {
		page.Form = withSubmit(s.plainForm("/admin/escrow", "POST",
			csrfField(sess),
			view.FormRow{Name: "action", Kind: view.InputHidden, Value: text("policy")},
			view.FormRow{Name: "coverNew", Label: "New accounts are covered", Kind: view.InputCheck,
				Value: text(boolStr(snap.Settings.EscrowCoverNewUsers))},
		), "escrow.policy.save", "Save")
		if flash.formErr != "" {
			page.Form.Sets[0].Rows[len(page.Form.Sets[0].Rows)-1].Error = text(flash.formErr)
		}
		page.Master = withSubmit(s.plainForm("/admin/escrow", "POST",
			csrfField(sess),
			view.FormRow{Name: "action", Kind: view.InputHidden, Value: text("master")},
			view.FormRow{Name: "current", Label: "Current", Kind: view.InputPassword, Required: true},
			view.FormRow{Name: "master", Label: "New", Kind: view.InputPassword, Required: true,
				Hint: text("Only the recovery private key is re-encrypted. Deposited copies are untouched.")},
			view.FormRow{Name: "master2", Label: "Repeat", Kind: view.InputPassword, Required: true},
		), "escrow.master.change", "Change")
		if flash.masterErr != "" {
			page.Master.Sets[0].Rows[len(page.Master.Sets[0].Rows)-1].Error = text(flash.masterErr)
		}
		page.Recover = withSubmit(s.plainForm("/admin/escrow", "POST",
			csrfField(sess),
			view.FormRow{Name: "action", Kind: view.InputHidden, Value: text("recover")},
			view.FormRow{Name: "login", Label: "Account", Kind: view.InputText, Required: true},
			view.FormRow{Name: "master", Label: "Master password", Kind: view.InputPassword, Required: true},
		), "escrow.recover", "Recover")
		if flash.recoverErr != "" {
			page.Recover.Sets[0].Rows[len(page.Recover.Sets[0].Rows)-1].Error = text(flash.recoverErr)
		}
	} else {
		page.Form = withSubmit(s.plainForm("/admin/escrow", "POST",
			csrfField(sess),
			view.FormRow{Name: "action", Kind: view.InputHidden, Value: text("enable")},
			view.FormRow{Name: "master", Label: "Master password", Kind: view.InputPassword, Required: true,
				Hint: text("If you lose this password, deposited keys cannot be opened. This is not backup.")},
			view.FormRow{Name: "master2", Label: "Repeat", Kind: view.InputPassword, Required: true},
		), "escrow.enable", "Turn on key escrow")
		if flash.formErr != "" {
			page.Form.Sets[0].Rows[len(page.Form.Sets[0].Rows)-1].Error = text(flash.formErr)
		}
	}
	tf := s.timeFor(sess)
	for _, u := range snap.Users {
		if u.Disabled || !store.UserDeposited(&u) {
			continue
		}
		row := view.AdminUserRow{
			Login:    text(u.Login),
			Escrowed: true,
		}
		if u.EscrowDepositedAt != nil {
			row.Created = tf.formatWhen(*u.EscrowDepositedAt, false, time.Now())
		}
		if u.EscrowLastRecovery != nil {
			row.LastLogin = tf.formatWhen(*u.EscrowLastRecovery, false, time.Now())
		}
		page.Covered = append(page.Covered, row)
	}
	return page
}

func boolStr(v bool) string {
	if v {
		return "true"
	}
	return ""
}
