package web

import (
	"errors"
	"net/http"
	"net/url"
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
		s.writePage(w, r, http.StatusTooManyRequests, s.passwordPage(sess, mustChange, errTooManyAttempts))
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

// tooManyAttempts is what a form that takes a password says past the
// ceiling of attempts: the form again, the words at its field — not a bare
// «too many requests» (Ад39).
const tooManyAttempts = "Too many attempts just now. Wait a few minutes and try again."

var errTooManyAttempts = errors.New(tooManyAttempts)

func (s *Server) mustChangePassword(accountID string) bool {
	snap := s.store.Snapshot()
	u := snap.UserByID(accountID)
	return u != nil && u.MustChangePassword
}

func (s *Server) passwordPage(sess *session.Session, mustChange bool, err error) view.PasswordPage {
	var currentErr, passErr, pass2Err view.Text
	switch {
	case errors.Is(err, errTooManyAttempts):
		currentErr = text(tooManyAttempts)
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
		// The caret is in New password, not in the field before it
		// (account-3).
		Form: focused(withPrimary(s.plainForm("/app/password", "POST", csrfField(sess),
			view.FormRow{Name: "current", Label: "Current password", Kind: view.InputPassword, Required: true, Error: currentErr},
			view.FormRow{Name: "password", Label: "New password", Kind: view.InputPassword, Required: true, Error: passErr},
			view.FormRow{Name: "password2", Label: "Repeat new password", Kind: view.InputPassword, Required: true, Error: pass2Err},
		), "password.save", "Change password"), "password"),
		// What the change does is said under the fields, where the fear of
		// losing the connections comes up (account-3).
		Note: "Your data stays encrypted under the same key; only the wrapping changes. Saved connections keep working.",
		Back: &view.Action{ID: "password.back", Label: "Back to settings", Href: s.route("/app/settings/account"), Method: "GET", Enabled: true},
	}
	// After a temporary password there is nowhere to go before the change:
	// the red line on top, and no way back (account-3).
	if mustChange {
		page.Back = nil
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

// unlockSheet is the password sheet; back is the page it was opened over,
// where its form goes back to when it is a page of its own (no scripts);
// forID the connection whose password it was opened for, "" — none.
func (s *Server) unlockSheet(sess *session.Session, errText view.Text, back, forID string) view.UnlockSheet {
	cancelTo := back
	if cancelTo == "" {
		cancelTo = "/app/settings/connections"
	}
	sheet := view.UnlockSheet{
		Purpose: text("Confirm your password to reveal stored credentials, download a device profile, issue a davloom device password, change key escrow, or delete your account."),
		// One line under the field, what the window opens and for how long
		// (s-profile-3, Нс19).
		Warning: text("Unlocks credentials everywhere for " + durationWords(s.cfg.UnlockTTL) + ": passwords, device profiles, key escrow — anything handed out or handed over."),
		// The person's own password, not the server's (s-plan-3, s-profile-3);
		// Show and Cancel (Нс20).
		Form: focused(withPrimary(s.plainForm("/app/settings/unlock", "POST",
			csrfField(sess),
			view.FormRow{Name: "back", Kind: view.InputHidden, Value: text(back)},
			view.FormRow{Name: "for", Kind: view.InputHidden, Value: text(forID)},
			view.FormRow{Name: "password", Label: "Your Carrel password", Kind: view.InputPassword, Required: true, Error: errText},
		), "unlock.submit", "Show",
			view.Action{ID: "unlock.cancel", Label: "Cancel", Href: s.route(cancelTo), Method: "GET", Enabled: true}), "password"),
	}
	// Opened from a connection's Show password, the sheet says whose (s-profile-3, Нс18).
	if f, ok := s.catalogOf(sess).Subscription(forID); ok && forID != "" {
		// A feed's password (Нс25): the sheet names the feed.
		sheet.Purpose, sheet.Subject = "Show password", text(collectionLabel(f.FeedCollection()))
	} else if conn, ok := s.catalogOf(sess).Connection(forID); ok && forID != "" {
		sheet.Purpose, sheet.Subject = "Show password", text(connLabel(conn))
	}
	if until := sess.UnlockUntil(s.now()); until != nil {
		sheet.Until = ptrWhen(s.timeFor(sess).formatWhen(*until, false, s.now()))
	}
	return sheet
}

func (s *Server) getSettingsUnlock(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	// The page the sheet is opened over: ?back=, else where the link was
	// followed from.
	back, ok := s.returnPath(r, r.URL.Query().Get("back"))
	if !ok {
		back, _ = s.returnPath(r, r.Referer())
	}
	if render.WantsFragment(r) {
		_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, s.unlockSheet(sess, "", back, r.FormValue("for")), nil)
		return
	}
	// Without scripts a sheet is a page of its own (U-8): "Show password" is
	// a plain link here, and a 404 behind it left no way to the password.
	sheet := s.unlockSheet(sess, "", back, r.FormValue("for"))
	// Over the page it was opened from (Лс29: s-plan-3 over «Set up a new
	// device», s-profile-3 over Connections).
	sheet.Under = s.underOf(r, back, "/app/settings/connections")
	s.writePage(w, r, http.StatusOK, sheet)
}

// returnPath reads a page of this instance to come back to, from an address
// (a Referer) or a path (a form's back): a path under /app/ or /admin/ of
// this origin and base path, with its query. Anything else — another host,
// //host, a page outside the shell — is not followed.
func (s *Server) returnPath(r *http.Request, raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || raw == "" || u.Opaque != "" || u.User != nil {
		return "", false
	}
	if u.Scheme != "" || u.Host != "" {
		if (u.Scheme != "http" && u.Scheme != "https") || u.Host != r.Host {
			return "", false
		}
	}
	p, ok := strings.CutPrefix(u.EscapedPath(), s.basePath+"/")
	if !ok || strings.HasPrefix(p, "/") || strings.Contains(p, "\\") || !(strings.HasPrefix(p, "app/") || strings.HasPrefix(p, "admin/")) {
		return "", false
	}
	if u.RawQuery != "" {
		p += "?" + u.RawQuery
	}
	return "/" + p, true
}

// postSettingsLock is Lock now (s-profile-4, s-plan-1…5): the unlock window
// of this session ends at once, before its time — every secret is asked for
// again through the sheet —, and the sign-in stays: the session, its key
// and its data are as they were (S-10.7). It releases nothing, so nothing
// goes to the journal. The answer is the page it was pressed on, read from
// the Referer of this very origin; anything else goes to Connections.
func (s *Server) postSettingsLock(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	sess.SetUnlockUntil(time.Time{})
	back, ok := s.returnPath(r, r.Referer())
	if !ok {
		back = "/app/settings/connections"
	}
	s.redirect(w, r, back)
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
	back, backOK := s.returnPath(r, r.FormValue("back"))
	start := time.Now()
	if !s.limit.allowLogin(sess.Login(), s.clientIP(r), start) {
		s.writePage(w, r, http.StatusTooManyRequests, s.unlockSheet(sess, text(tooManyAttempts), back, r.FormValue("for")))
		return
	}
	if !s.users.Unlock(r.Context(), sess.AccountID(), r.FormValue("password")) {
		constantTimeDelay(start, 300*time.Millisecond)
		// Without scripts the sheet is a page of its own (U-8): it comes
		// back as one, the error at its field.
		s.writePage(w, r, http.StatusOK, s.unlockSheet(sess, text("Password is wrong."), back, r.FormValue("for")))
		return
	}
	sess.SetUnlockUntil(s.now().Add(s.cfg.UnlockTTL))
	if !render.WantsFragment(r) {
		// A plain form goes back to the page the sheet was opened over; it
		// draws the open window itself.
		if !backOK {
			back = "/app/settings/connections"
		}
		s.redirect(w, r, back)
		return
	}
	top := s.buildShell(sess, "").Top
	extra := []render.OOB{{Target: render.TargetTopbar, Body: top}}
	_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, s.unlockSheet(sess, "", back, r.FormValue("for")), extra)
}
