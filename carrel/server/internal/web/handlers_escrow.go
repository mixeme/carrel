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

func (s *Server) postSettingsEscrow(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	action := r.FormValue("action")
	switch action {
	case "deposit":
		if sess.UnlockUntil(time.Now()) == nil {
			page := s.buildEscrowPage(sess, "Confirm your password first.")
			s.writePage(w, r, http.StatusOK, page)
			return
		}
		key, ok := sess.DataKey()
		if !ok {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if err := s.users.DepositEscrow(sess.AccountID(), key); err != nil {
			page := s.buildEscrowPage(sess, string(formError(err)))
			s.writePage(w, r, http.StatusOK, page)
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

func (s *Server) buildEscrowPage(sess *session.Session, errText string) view.EscrowPage {
	snap := s.store.Snapshot()
	u := snap.UserByID(sess.AccountID())
	active := store.EscrowActive(&snap)
	status := view.EscrowStatus{
		Active: active,
		Note:   text("Key escrow is disabled on this instance."),
	}
	if active {
		status.Note = text("An administrator who holds the escrow master password can decrypt the DAV credentials you save here, and through them your contacts and calendars. This is not backup.")
	}
	if u != nil && store.UserDeposited(u) {
		status.Covered = true
		status.By = text("administrator with master password")
		if u.EscrowDepositedAt != nil {
			status.LastUsed = formatWhen(*u.EscrowDepositedAt, s.tz(), false, time.Now())
		}
		status.Actions = []view.Action{{
			ID: "escrow.withdraw", Label: "Withdraw my deposited key",
			Href:   s.postQuery("/app/settings/escrow", url.Values{"action": {"withdraw"}}),
			Method: "POST", Enabled: true,
		}}
	} else if active {
		status.Actions = []view.Action{{
			ID: "escrow.deposit", Label: "Deposit my key",
			Href:   s.postQuery("/app/settings/escrow", url.Values{"action": {"deposit"}}),
			Method: "POST", Enabled: sess.UnlockUntil(time.Now()) != nil,
		}}
	}
	page := view.EscrowPage{
		Shell:  s.buildShell(sess, "Key escrow"),
		Status: status,
	}
	if active && u != nil && !store.UserDeposited(u) {
		sheet := s.unlockSheet(sess, text(errText))
		sheet.Purpose = text("Confirm your password to deposit your data key.")
		sheet.Warning = text("An administrator who holds the escrow master password will be able to decrypt the DAV credentials you save here, and through them your contacts and calendars. Ending sessions does not undo this.")
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
			mailed := s.sendEscrowRecoveryMail(ctx, login, temp)
			flash.result = &view.EscrowRecoveryResult{
				Login:    text(login),
				Password: text(temp),
				Copy: view.Action{
					ID: "admin.escrow.copy", Label: "Copy", Method: "GET", Enabled: true,
				},
				Mailed: escrowRecoveryMailedStatus(mailed),
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

func escrowRecoveryMailedStatus(sent bool) view.Text {
	if sent {
		return text("sent")
	}
	return text("SMTP not configured")
}

func (s *Server) buildAdminEscrowPage(sess *session.Session, flash adminEscrowFlash) view.AdminEscrowPage {
	snap := s.store.Snapshot()
	enabled := store.EscrowActive(&snap)
	page := view.AdminEscrowPage{
		Shell:   s.buildShell(sess, "Key escrow"),
		Enabled: enabled,
		Warning: view.Notice{
			Role:  view.NoticeAlert,
			Title: text("Administrative access to saved credentials"),
			Body:  text("Key escrow is not a key backup. It is a way in: an administrator holding the master password can decrypt a covered user's saved DAV credentials, and through them their contacts and calendars. Every covered user is told so in their profile, and every recovery is logged and mailed to them."),
		},
		Result: flash.result,
	}
	if enabled {
		page.Form = s.plainForm("/admin/escrow", "POST",
			csrfField(sess),
			view.FormRow{Name: "action", Kind: view.InputText, Value: text("policy")},
			view.FormRow{Name: "coverNew", Label: "New accounts are covered", Kind: view.InputCheck,
				Value: text(boolStr(snap.Settings.EscrowCoverNewUsers))},
		)
		if flash.formErr != "" {
			page.Form.Sets[0].Rows[len(page.Form.Sets[0].Rows)-1].Error = text(flash.formErr)
		}
		page.Master = s.plainForm("/admin/escrow", "POST",
			csrfField(sess),
			view.FormRow{Name: "action", Kind: view.InputText, Value: text("master")},
			view.FormRow{Name: "current", Label: "Current", Kind: view.InputPassword, Required: true},
			view.FormRow{Name: "master", Label: "New", Kind: view.InputPassword, Required: true,
				Hint: text("Only the recovery private key is re-encrypted. Deposited copies are untouched.")},
			view.FormRow{Name: "master2", Label: "Repeat", Kind: view.InputPassword, Required: true},
		)
		if flash.masterErr != "" {
			page.Master.Sets[0].Rows[len(page.Master.Sets[0].Rows)-1].Error = text(flash.masterErr)
		}
		page.Recover = s.plainForm("/admin/escrow", "POST",
			csrfField(sess),
			view.FormRow{Name: "action", Kind: view.InputText, Value: text("recover")},
			view.FormRow{Name: "login", Label: "Account", Kind: view.InputText, Required: true},
			view.FormRow{Name: "master", Label: "Master password", Kind: view.InputPassword, Required: true},
		)
		if flash.recoverErr != "" {
			page.Recover.Sets[0].Rows[len(page.Recover.Sets[0].Rows)-1].Error = text(flash.recoverErr)
		}
	} else {
		page.Form = s.plainForm("/admin/escrow", "POST",
			csrfField(sess),
			view.FormRow{Name: "action", Kind: view.InputText, Value: text("enable")},
			view.FormRow{Name: "master", Label: "Master password", Kind: view.InputPassword, Required: true,
				Hint: text("If you lose this password, deposited keys cannot be opened. This is not backup.")},
			view.FormRow{Name: "master2", Label: "Repeat", Kind: view.InputPassword, Required: true},
		)
		if flash.formErr != "" {
			page.Form.Sets[0].Rows[len(page.Form.Sets[0].Rows)-1].Error = text(flash.formErr)
		}
	}
	for _, u := range snap.Users {
		if u.Disabled || !store.UserDeposited(&u) {
			continue
		}
		row := view.AdminUserRow{
			Login:    text(u.Login),
			Escrowed: true,
		}
		if u.EscrowDepositedAt != nil {
			row.Created = formatWhen(*u.EscrowDepositedAt, s.tz(), false, time.Now())
		}
		if u.EscrowLastRecovery != nil {
			row.LastLogin = formatWhen(*u.EscrowLastRecovery, s.tz(), false, time.Now())
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
