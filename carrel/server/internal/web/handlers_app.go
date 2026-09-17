package web

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

func (s *Server) getAppRoot(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLogin(w, r); !ok {
		return
	}
	s.redirect(w, r, "/app/contacts")
}

func (s *Server) stubShellPage(w http.ResponseWriter, r *http.Request, title string, body any) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	_ = sess
	s.writePage(w, r, http.StatusOK, body)
}

// sheetStub answers the sheet-shaped screens that are not wired yet: publish,
// mail, the mail attachment and the file picker. An empty sheet as a fragment,
// 404 otherwise. It used to be getQuickNote and carried the notes route too;
// the quick note goes through the common sheet now (U-6.16), and what is left
// here is only the stub.
func (s *Server) sheetStub(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLogin(w, r); !ok {
		return
	}
	if WantsFragment(r) {
		_ = s.render.Fragment(w, r, http.StatusOK, TargetSheet, view.QuickNoteSheet{}, nil)
		return
	}
	s.notFound(w, r)
}

func (s *Server) getDuplicates(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	page := view.DuplicatesPage{Shell: s.buildShell(sess, "Duplicates")}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) postDuplicatesDecide(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	group := view.DuplicateGroup{}
	extra := []OOB{{Target: TargetNotices, Body: []view.Notice{}}}
	if WantsFragment(r) {
		_ = s.render.Fragment(w, r, http.StatusOK, TargetMain, group, extra)
		return
	}
	_ = sess
	s.notFound(w, r)
}

func (s *Server) getDuplicatesMerge(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	page := view.DuplicatesMergePage{Shell: s.buildShell(sess, "Merge duplicates")}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) postDuplicatesMerge(w http.ResponseWriter, r *http.Request) {
	s.getDuplicatesMerge(w, r)
}

func (s *Server) getPublish(w http.ResponseWriter, r *http.Request) {
	s.sheetStub(w, r)
}

func (s *Server) postPublish(w http.ResponseWriter, r *http.Request) {
	s.getPublish(w, r)
}

func (s *Server) getMailCompose(w http.ResponseWriter, r *http.Request) {
	s.sheetStub(w, r)
}

func (s *Server) postMailCompose(w http.ResponseWriter, r *http.Request) {
	s.getMailCompose(w, r)
}

func (s *Server) getMailAttach(w http.ResponseWriter, r *http.Request) {
	s.sheetStub(w, r)
}

func (s *Server) postMailAttach(w http.ResponseWriter, r *http.Request) {
	s.getMailAttach(w, r)
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
		Shell: s.buildShell(sess, "Change password"),
		Form: s.plainForm("/app/password", "POST", csrfField(sess),
			view.FormRow{Name: "current", Label: "Current password", Kind: view.InputPassword, Required: true, Error: currentErr},
			view.FormRow{Name: "password", Label: "New password", Kind: view.InputPassword, Required: true, Error: passErr, Hint: text("At least 8 characters.")},
			view.FormRow{Name: "password2", Label: "Repeat new password", Kind: view.InputPassword, Required: true, Error: pass2Err},
		),
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

func (s *Server) getContactTimeline(w http.ResponseWriter, r *http.Request) {
	s.ownedRecordStub(w, r)
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

func (s *Server) getFilesPicker(w http.ResponseWriter, r *http.Request) {
	s.sheetStub(w, r)
}

func (s *Server) postFilesPicker(w http.ResponseWriter, r *http.Request) {
	s.getFilesPicker(w, r)
}

func (s *Server) getFilesPublished(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	page := view.PublishedLinksPage{Shell: s.buildShell(sess, "Published links")}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) getSettingsAccount(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	page := view.AccountPage{Shell: s.buildShell(sess, "Account")}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) postSettingsAccount(w http.ResponseWriter, r *http.Request) {
	s.getSettingsAccount(w, r)
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

func (s *Server) postSettingsConnectionsTest(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLogin(w, r); !ok {
		return
	}
	trace := view.DiagnosticTrace{OK: false}
	// DAV failures become screen state, not 500 (S-9.5).
	if r.FormValue("simulate") == "large" {
		_ = dav.ErrResponseTooLarge
		trace.Steps = []view.TraceStep{{
			Name:   text("response"),
			State:  view.CheckBad,
			Detail: text("response too large"),
		}}
	}
	if WantsFragment(r) {
		_ = s.render.Fragment(w, r, http.StatusOK, TargetMain, trace, nil)
		return
	}
	s.notFound(w, r)
}

func (s *Server) getSettingsDevices(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	page := view.DevicesPage{Shell: s.buildShell(sess, "Devices")}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) postSettingsDevices(w http.ResponseWriter, r *http.Request) {
	s.getSettingsDevices(w, r)
}

func (s *Server) getSettingsDevicesProfile(w http.ResponseWriter, r *http.Request) {
	s.notFound(w, r)
}

func (s *Server) getSettingsDavloom(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	page := view.DavloomPage{Shell: s.buildShell(sess, "DAV proxy")}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) postSettingsDavloom(w http.ResponseWriter, r *http.Request) {
	s.getSettingsDavloom(w, r)
}

func (s *Server) getSettingsBackup(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	page := view.BackupPage{Shell: s.buildShell(sess, "Backup")}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) postSettingsBackup(w http.ResponseWriter, r *http.Request) {
	s.getSettingsBackup(w, r)
}

func (s *Server) unlockSheet(sess *session.Session, errText view.Text) view.UnlockSheet {
	sheet := view.UnlockSheet{
		Purpose: text("Confirm your password to reveal stored credentials, download a device profile, or change key escrow."),
		Warning: text("Everything revealed in this window is written to the audit log."),
		Form: s.plainForm("/app/settings/unlock", "POST",
			csrfField(sess),
			view.FormRow{Name: "password", Label: "Password", Kind: view.InputPassword, Required: true, Error: errText},
		),
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
	if WantsFragment(r) {
		_ = s.render.Fragment(w, r, http.StatusOK, TargetSheet, s.unlockSheet(sess, ""), nil)
		return
	}
	s.notFound(w, r)
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
	if !WantsFragment(r) {
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
		_ = s.render.Fragment(w, r, http.StatusOK, TargetSheet, s.unlockSheet(sess, text("Password is wrong.")), nil)
		return
	}
	sess.SetUnlockUntil(time.Now().Add(s.cfg.UnlockTTL))
	top := s.buildShell(sess, "").Top
	extra := []OOB{{Target: TargetTopbar, Body: top}}
	_ = s.render.Fragment(w, r, http.StatusOK, TargetSheet, s.unlockSheet(sess, ""), extra)
}
