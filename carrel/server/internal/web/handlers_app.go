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

func (s *Server) getSearch(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	page := view.SearchPage{Shell: s.buildShell(sess, "Search")}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) getSearchProgressSSE(w http.ResponseWriter, r *http.Request) {
	s.notFound(w, r)
}

func (s *Server) getSearchProgressPoll(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	bar := view.PollBar{}
	if WantsFragment(r) {
		_ = s.render.Fragment(w, r, http.StatusOK, TargetPoll, bar, nil)
		return
	}
	_ = sess
	s.notFound(w, r)
}

func (s *Server) postSearchProgressRetry(w http.ResponseWriter, r *http.Request) {
	s.getSearchProgressPoll(w, r)
}

func (s *Server) postSearchProgressCancel(w http.ResponseWriter, r *http.Request) {
	s.getSearchProgressPoll(w, r)
}

func (s *Server) getQuickNote(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	sheet := view.QuickNoteSheet{}
	if WantsFragment(r) {
		_ = s.render.Fragment(w, r, http.StatusOK, TargetSheet, sheet, nil)
		return
	}
	_ = sess
	s.notFound(w, r)
}

func (s *Server) postQuickNote(w http.ResponseWriter, r *http.Request) {
	s.getQuickNote(w, r)
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

func (s *Server) postSources(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	rail := s.buildShell(sess, "").Rail
	extra := []OOB{
		{Target: TargetRail, Body: rail},
		{Target: TargetPoll, Body: view.PollBar{}},
	}
	if WantsFragment(r) {
		_ = s.render.Fragment(w, r, http.StatusOK, TargetRail, rail, extra[1:])
		return
	}
	s.notFound(w, r)
}

func (s *Server) getPublish(w http.ResponseWriter, r *http.Request) {
	s.getQuickNote(w, r) // sheet pattern
}

func (s *Server) postPublish(w http.ResponseWriter, r *http.Request) {
	s.getPublish(w, r)
}

func (s *Server) getMailCompose(w http.ResponseWriter, r *http.Request) {
	s.getQuickNote(w, r)
}

func (s *Server) postMailCompose(w http.ResponseWriter, r *http.Request) {
	s.getMailCompose(w, r)
}

func (s *Server) getMailAttach(w http.ResponseWriter, r *http.Request) {
	s.getQuickNote(w, r)
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

func (s *Server) getKindRoot(w http.ResponseWriter, r *http.Request, kind string) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	page := s.emptyListPage(sess, kind)
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) getKindList(w http.ResponseWriter, r *http.Request, kind string) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	account := r.PathValue("account")
	col := r.PathValue("col")
	if !s.checkOwnership(w, r, sess, account, col, "") {
		return
	}
	page := s.emptyListPage(sess, kind)
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) emptyListPage(sess *session.Session, kind string) any {
	sh := s.buildShell(sess, kindTitle(kind))
	switch kind {
	case "contacts":
		return view.ContactsListPage{Shell: sh}
	case "calendar":
		return view.AgendaPage{Shell: sh}
	case "tasks":
		return view.TasksPage{Shell: sh}
	case "notes":
		return view.NotesPage{Shell: sh}
	default:
		return view.ContactsListPage{Shell: sh}
	}
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

func (s *Server) getKindRecord(w http.ResponseWriter, r *http.Request, kind string) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	if !s.checkOwnership(w, r, sess, r.PathValue("account"), r.PathValue("col"), r.PathValue("uid")) {
		return
	}
	s.notFound(w, r)
}

func (s *Server) getKindEdit(w http.ResponseWriter, r *http.Request, kind string) {
	s.getKindRecord(w, r, kind)
}

func (s *Server) postKindEdit(w http.ResponseWriter, r *http.Request, kind string) {
	s.getKindRecord(w, r, kind)
}

func (s *Server) getKindConflict(w http.ResponseWriter, r *http.Request, kind string) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	if !s.checkOwnership(w, r, sess, r.PathValue("account"), r.PathValue("col"), r.PathValue("uid")) {
		return
	}
	page := view.ConflictPage{Shell: s.buildShell(sess, "Conflict")}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) postKindConflict(w http.ResponseWriter, r *http.Request, kind string) {
	s.getKindConflict(w, r, kind)
}

func (s *Server) getKindMove(w http.ResponseWriter, r *http.Request, kind string) {
	s.getQuickNote(w, r)
}

func (s *Server) postKindMove(w http.ResponseWriter, r *http.Request, kind string) {
	s.getKindMove(w, r, kind)
}

func (s *Server) postKindDelete(w http.ResponseWriter, r *http.Request, kind string) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	if !s.checkOwnership(w, r, sess, r.PathValue("account"), r.PathValue("col"), r.PathValue("uid")) {
		return
	}
	s.redirect(w, r, "/app/"+kind+"/"+r.PathValue("account")+"/"+r.PathValue("col"))
}

func (s *Server) getKindImport(w http.ResponseWriter, r *http.Request, kind string) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	if !s.checkOwnership(w, r, sess, r.PathValue("account"), r.PathValue("col"), "") {
		return
	}
	page := view.ImportPage{Shell: s.buildShell(sess, "Import")}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) postKindImport(w http.ResponseWriter, r *http.Request, kind string) {
	s.getKindImport(w, r, kind)
}

func (s *Server) getKindExport(w http.ResponseWriter, r *http.Request, kind string) {
	s.notFound(w, r)
}

func (s *Server) getKindNew(w http.ResponseWriter, r *http.Request, kind string) {
	s.getQuickNote(w, r)
}

func (s *Server) postKindNew(w http.ResponseWriter, r *http.Request, kind string) {
	s.getKindNew(w, r, kind)
}

func (s *Server) getContactTimeline(w http.ResponseWriter, r *http.Request) {
	s.getKindRecord(w, r, "contacts")
}

func (s *Server) getPhotoPreview(w http.ResponseWriter, r *http.Request) {
	s.getKindRecord(w, r, "contacts")
}

func (s *Server) postPhotoPreview(w http.ResponseWriter, r *http.Request) {
	s.getPhotoPreview(w, r)
}

func (s *Server) getContactPhoto(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	if !s.checkOwnership(w, r, sess, r.PathValue("account"), r.PathValue("col"), r.PathValue("uid")) {
		return
	}
	s.notFound(w, r)
}

func (s *Server) getFilesRoot(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	page := view.FilesRootPage{Shell: s.buildShell(sess, "Files")}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) getFiles(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	path := r.PathValue("path")
	if !s.rejectPathTraversal(w, r, path) {
		return
	}
	if !s.checkOwnership(w, r, sess, r.PathValue("account"), r.PathValue("col"), "") {
		return
	}
	page := view.FilesPage{Shell: s.buildShell(sess, "Files")}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) getFilesDownload(w http.ResponseWriter, r *http.Request) {
	s.notFound(w, r)
}

func (s *Server) postFilesUpload(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	q := view.TransferQueue{}
	if WantsFragment(r) {
		_ = s.render.Fragment(w, r, http.StatusOK, TargetQueue, q, nil)
		return
	}
	_ = sess
	s.notFound(w, r)
}

func (s *Server) postFilesMkcol(w http.ResponseWriter, r *http.Request) {
	s.getFiles(w, r)
}

func (s *Server) postFilesOp(w http.ResponseWriter, r *http.Request) {
	s.getQuickNote(w, r)
}

func (s *Server) getFilesPicker(w http.ResponseWriter, r *http.Request) {
	s.getQuickNote(w, r)
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

func (s *Server) getAttach(w http.ResponseWriter, r *http.Request) {
	s.notFound(w, r)
}

func (s *Server) getCollectionNew(w http.ResponseWriter, r *http.Request) {
	s.getQuickNote(w, r)
}

func (s *Server) postCollectionNew(w http.ResponseWriter, r *http.Request) {
	s.getCollectionNew(w, r)
}

func (s *Server) getCollectionRename(w http.ResponseWriter, r *http.Request) {
	s.getQuickNote(w, r)
}

func (s *Server) postCollectionRename(w http.ResponseWriter, r *http.Request) {
	s.getCollectionRename(w, r)
}

func (s *Server) getCollectionMove(w http.ResponseWriter, r *http.Request) {
	s.getQuickNote(w, r)
}

func (s *Server) postCollectionMove(w http.ResponseWriter, r *http.Request) {
	s.getCollectionMove(w, r)
}

func (s *Server) getCollectionDelete(w http.ResponseWriter, r *http.Request) {
	s.getQuickNote(w, r)
}

func (s *Server) postCollectionDelete(w http.ResponseWriter, r *http.Request) {
	s.getCollectionDelete(w, r)
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

func (s *Server) getSettingsConnections(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	page := view.ConnectionsPage{
		Shell:    s.buildShell(sess, "Connections"),
		Accounts: nil,
		Add: s.plainForm("/app/settings/connections", "POST",
			csrfField(sess),
			view.FormRow{Name: "server", Label: "Server URL", Kind: view.InputURL, Value: text(r.FormValue("server"))},
			view.FormRow{Name: "login", Label: "Login", Kind: view.InputText},
			view.FormRow{Name: "password", Label: "Password", Kind: view.InputPassword},
			view.FormRow{Name: "certFingerprint", Label: "Certificate fingerprint (SHA-256)", Kind: view.InputText, Value: text(r.FormValue("certFingerprint"))},
		),
	}
	for _, warn := range connectionWarnings(r.FormValue("server"), r.FormValue("certFingerprint")) {
		page.Shell.Notices = append(page.Shell.Notices, warn)
	}
	s.writePage(w, r, http.StatusOK, page)
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

func (s *Server) postSettingsConnections(w http.ResponseWriter, r *http.Request) {
	s.getSettingsConnections(w, r)
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

func (s *Server) getSettingsAttachments(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	page := view.AttachmentsPage{Shell: s.buildShell(sess, "Attachments")}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) postSettingsAttachments(w http.ResponseWriter, r *http.Request) {
	s.getSettingsAttachments(w, r)
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

func (s *Server) getSettingsAppearance(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	page := view.AppearancePage{Shell: s.buildShell(sess, "Appearance")}
	s.writePage(w, r, http.StatusOK, page)
}

func (s *Server) postSettingsAppearance(w http.ResponseWriter, r *http.Request) {
	s.getSettingsAppearance(w, r)
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
		sheet.Until = ptrWhen(formatWhen(*until, s.tz(), false, time.Now()))
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
