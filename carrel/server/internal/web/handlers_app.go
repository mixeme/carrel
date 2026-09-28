package web

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

func (s *Server) getAppRoot(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLogin(w, r); !ok {
		return
	}
	s.redirect(w, r, "/app/contacts")
}

func (s *Server) getPassword(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	s.writePage(w, r, http.StatusOK, s.passwordPage(sess, s.mustChangePassword(sess.AccountID()), nil))
}

func (s *Server) postPassword(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	mustChange := s.mustChangePassword(sess.AccountID())
	start := time.Now()
	if !s.limit.allowLogin(sess.Login(), s.clientIP(r), start) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	err := s.users.ChangePassword(r.Context(), sess.AccountID(), sess.ID(),
		r.FormValue("current"), r.FormValue("password"), r.FormValue("password2"))
	if err != nil {
		if errors.Is(err, user.ErrWrongPassword) {
			constantTimeDelay(start, 300*time.Millisecond)
		}
		s.writePage(w, r, http.StatusOK, s.passwordPage(sess, mustChange, err))
		return
	}
	if mustChange {
		s.redirect(w, r, "/app/contacts")
		return
	}
	s.redirect(w, r, "/app/settings/account")
}

func (s *Server) mustChangePassword(accountID string) bool {
	snap := s.store.Snapshot()
	u := snap.UserByID(accountID)
	return u != nil && u.MustChangePassword
}

func (s *Server) passwordPage(sess *session.Session, mustChange bool, err error) view.PasswordPage {
	var currentErr, passErr, pass2Err view.Text
	switch {
	case errors.Is(err, user.ErrWrongPassword):
		currentErr = text("Password is wrong.")
	case errors.Is(err, user.ErrPasswordShort):
		passErr = formError(err)
	case errors.Is(err, user.ErrPasswordMismatch):
		pass2Err = formError(err)
	}
	page := view.PasswordPage{
		// The password lives under the account, and the screen stands in
		// the settings: it carries their rail with the account marked.
		Shell: s.withRail(s.buildShell(sess, "Change password"), s.settingsRail(sess, "/app/settings/account")),
		Form: withSubmit(s.plainForm("/app/password", "POST", csrfField(sess),
			view.FormRow{Name: "current", Label: "Current password", Kind: view.InputPassword, Required: true, Error: currentErr},
			view.FormRow{Name: "password", Label: "New password", Kind: view.InputPassword, Required: true, Error: passErr, Hint: text("At least 8 characters.")},
			view.FormRow{Name: "password2", Label: "Repeat new password", Kind: view.InputPassword, Required: true, Error: pass2Err},
		), "password.save", "Change password"),
	}
	if mustChange {
		page.Warning = view.Notice{
			Role: view.NoticeAlert,
			Body: text("You are signed in with a temporary password. Choose a new one before continuing."),
		}
	}
	return page
}

func kindTitle(kind string) string {
	switch kind {
	case "contacts":
		return "Contacts"
	case "calendar":
		return "Calendar"
	case "tasks":
		return "Tasks"
	case "notes":
		return "Notes"
	default:
		return kind
	}
}

// ownedRecordStub answers record routes whose functions come in later steps:
// ownership first, then 404.
func (s *Server) ownedRecordStub(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	if !s.checkOwnership(w, r, sess, r.PathValue("account"), r.PathValue("col"), r.PathValue("uid")) {
		return
	}
	s.notFound(w, r)
}

func connectionWarnings(server, certFingerprint string) []view.Notice {
	if server == "" {
		return nil
	}
	lower := strings.ToLower(server)
	var out []view.Notice
	if strings.HasPrefix(lower, "http://") {
		out = append(out, view.Notice{
			Role:  view.NoticeAlert,
			Title: text("Unencrypted connection"),
			Body:  text("Credentials will be sent in clear text. Continue only if you accept this risk."),
		})
	}
	if strings.HasPrefix(lower, "https://") {
		pin := dav.NormalizeFingerprint(certFingerprint)
		if pin == "" {
			out = append(out, view.Notice{
				Role:  view.NoticeAlert,
				Title: text("Certificate verification"),
				Body:  text("If the server uses a self-signed or otherwise untrusted certificate, run Test and enter the SHA-256 fingerprint shown there. The fingerprint is stored with the connection password and must be accepted again if the certificate changes."),
			})
		}
	}
	return out
}

func (s *Server) unlockSheet(sess *session.Session, errText view.Text) view.UnlockSheet {
	sheet := view.UnlockSheet{
		Purpose: text("Confirm your password to reveal stored credentials, download a device profile, issue a davloom device password, change key escrow, or delete your account."),
		Warning: text("Everything revealed in this window is written to the audit log."),
		Form: withSubmit(s.plainForm("/app/settings/unlock", "POST",
			csrfField(sess),
			view.FormRow{Name: "password", Label: "Password", Kind: view.InputPassword, Required: true, Error: errText},
		), "unlock.submit", "Show"),
	}
	if until := sess.UnlockUntil(time.Now()); until != nil {
		sheet.Until = ptrWhen(s.timeFor(sess).formatWhen(*until, false, time.Now()))
	}
	return sheet
}

func (s *Server) getSettingsUnlock(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	if render.WantsFragment(r) {
		_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, s.unlockSheet(sess, ""), nil)
		return
	}
	// Without scripts a sheet is a page of its own (U-8): "Show password" is
	// a plain link here, and a 404 behind it left no way to the password.
	s.writePage(w, r, http.StatusOK, s.unlockSheet(sess, ""))
}

// postSettingsUnlock is the one confirmation sheet behind every secret
// release (S-10.7): the person's own password, checked against their login
// hash, opens the window for CARREL_UNLOCK_TTL in this session only. Wrong
// answers share the login bucket, so a stolen cookie gets five guesses a
// minute and every guess costs an Argon2.
func (s *Server) postSettingsUnlock(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	if !render.WantsFragment(r) {
		s.notFound(w, r)
		return
	}
	start := time.Now()
	if !s.limit.allowLogin(sess.Login(), s.clientIP(r), start) {
		http.Error(w, "too many requests", http.StatusTooManyRequests)
		return
	}
	if !s.users.Unlock(r.Context(), sess.AccountID(), r.FormValue("password")) {
		constantTimeDelay(start, 300*time.Millisecond)
		_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, s.unlockSheet(sess, text("Password is wrong.")), nil)
		return
	}
	sess.SetUnlockUntil(time.Now().Add(s.cfg.UnlockTTL))
	top := s.buildShell(sess, "").Top
	extra := []render.OOB{{Target: render.TargetTopbar, Body: top}}
	_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, s.unlockSheet(sess, ""), extra)
}
