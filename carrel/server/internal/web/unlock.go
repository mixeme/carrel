package web

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// What the password's sheet is opened for (for=, Typecase's note 86): the
// title is what is asked for, the main button the act itself. Anything else
// names a connection or a feed whose password is shown.
const (
	unlockProfile = "profile" // Generate profile (s-plan-3)
	unlockDavloom = "davloom" // Add device: a device's password (s-davloom-5)
	unlockDelete  = "delete"  // Delete account (account-15)
	unlockTrigger = "trigger" // a backup job's external trigger (193 p. 3); job= names it
)

// unlockHref opens the password's sheet for what, over the page back.
func (s *Server) unlockHref(what, back string, more ...string) string {
	q := url.Values{"for": {what}, "back": {back}}
	for i := 0; i+1 < len(more); i += 2 {
		q.Set(more[i], more[i+1])
	}
	return s.postQuery("/app/settings/unlock", q)
}

// unlockSheet is the one sheet of the person's password before anything is
// handed out (Typecase's note 86): the title — what for —, on its right
// whose, the field, the one line of what it opens and for how long, and the
// act itself as its main button; deleting the account says what goes,
// before the field and in alarm, and asks the password as its
// confirmation, without the line. back is the page it was opened over, where
// Cancel goes and where its form comes back to as a page of its own (no
// scripts); what names what it was opened for.
func (s *Server) unlockSheet(sess *session.Session, errText view.Text, back, what string, extra url.Values) view.UnlockSheet {
	cancelTo := back
	if cancelTo == "" {
		cancelTo = "/app/settings/connections"
	}
	hidden := []view.FormRow{
		csrfField(sess),
		{Name: "back", Kind: view.InputHidden, Value: text(back)},
		{Name: "for", Kind: view.InputHidden, Value: text(what)},
	}
	if job := extra.Get("job"); job != "" {
		hidden = append(hidden, view.FormRow{Name: "job", Kind: view.InputHidden, Value: text(job)})
	}
	sheet := view.UnlockSheet{
		Purpose: "Confirm your password",
		Warning: text("Unlocks stored credentials for " + durationWords(s.cfg.UnlockTTL) + "."),
	}
	act := view.Action{ID: "unlock.submit", Label: "Confirm", Method: "POST", Primary: true, Enabled: true}
	blob := s.catalogOf(sess)
	switch what {
	case unlockProfile:
		sheet.Purpose, act.Label = "Generate profile", "Generate"
	case unlockDavloom:
		sheet.Purpose, act.Label = "Add device", "Create password"
		if held, ok := heldDevice(sess); ok {
			sheet.Subject = text(strings.TrimSpace(held.Get("name")))
		}
	case unlockDelete:
		// The password is the confirmation here: what goes is said before
		// it, and nothing about a window (account-15).
		sheet.Purpose, act.Label, act.Primary, act.Danger = "Delete account", "Delete account", false, true
		sheet.Alert, sheet.Warning = accountDeleteNote, ""
	case unlockTrigger:
		// The job's external trigger, the job named (193 p. 3).
		sheet.Purpose, act.Label = "External trigger", "Turn on"
		if job, ok := blob.BackupJob(extra.Get("job")); ok {
			sheet.Subject = text(job.Name)
		}
	default:
		if f, ok := blob.Subscription(what); ok && what != "" {
			// A feed's password (Нс25): the sheet names the feed.
			sheet.Purpose, sheet.Subject, act.Label = "Show password", text(collectionLabel(f.FeedCollection())), "Show"
		} else if conn, ok := blob.Connection(what); ok && what != "" {
			// Whose, so that two cards' Show password are not confused
			// (s-profile-3, Нс18).
			sheet.Purpose, sheet.Subject, act.Label = "Show password", text(connLabel(conn)), "Show"
		}
	}
	sheet.Form = focused(s.plainForm("/app/settings/unlock", "POST", append(hidden,
		view.FormRow{Name: "password", Label: "Your Carrel password", Kind: view.InputPassword, Required: true, Error: errText})...), "password")
	sheet.Form.Actions = []view.Action{act,
		{ID: "unlock.cancel", Label: "Cancel", Href: s.route(cancelTo), Method: "GET", Enabled: true}}
	return sheet
}

// postSettingsUnlock is the password's sheet sent (S-10.7, Typecase's note
// 86): the person's own password, checked against their login hash, opens
// the window for CARREL_UNLOCK_TTL in this session, and the act the sheet
// was opened for is done at once — the profile's link issued, the device's
// password created from Add device as typed, the job's trigger turned on,
// the account deleted (the password its confirmation, asked even inside the
// window). A wrong password is the sheet again, its error at the field.
func (s *Server) postSettingsUnlock(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	back, backOK := s.returnPath(r, r.FormValue("back"))
	what := r.FormValue("for")
	extra := url.Values{"job": {r.FormValue("job")}}
	start := time.Now()
	if !s.limit.allowLogin(sess.Login(), s.clientIP(r), start) {
		s.writePage(w, r, http.StatusTooManyRequests, s.unlockSheet(sess, text(tooManyAttempts), back, what, extra))
		return
	}
	if !s.users.Unlock(r.Context(), sess.AccountID(), r.FormValue("password")) {
		constantTimeDelay(start, 300*time.Millisecond)
		// Without scripts the sheet is a page of its own (U-8): it comes
		// back as one, the error at its field.
		s.writePage(w, r, http.StatusOK, s.unlockSheet(sess, text("Password is wrong."), back, what, extra))
		return
	}
	if what == unlockDelete {
		if err := s.users.DeleteUser(sess.AccountID(), sess.AccountID(), sess.Login()); err != nil {
			s.writePage(w, r, http.StatusOK, s.unlockSheet(sess, formError(err), back, what, extra))
			return
		}
		s.clearSessionCookie(w, r)
		s.redirect(w, r, "/login")
		return
	}
	sess.SetUnlockUntil(s.now().Add(s.cfg.UnlockTTL))
	if !backOK {
		back = "/app/settings/connections"
	}
	switch what {
	case unlockProfile:
		token, _, err := s.users.IssueDeviceProfile(sess.AccountID(), profileAccountIDs(s.catalogOf(sess)))
		if err == nil {
			// The card is the page's while the link lives (Нс40).
			sess.SetDraft(session.DraftProfileLink, []byte(token))
		}
	case unlockDavloom:
		if held, ok := heldDevice(sess); ok {
			key, _ := sess.DataKey()
			name := strings.TrimSpace(held.Get("name"))
			id, password, err := s.users.IssueDevice(r.Context(), sess.AccountID(), key, name, deviceScopeOf(held))
			if err != nil {
				// What was typed comes back, the refusal with it.
				s.redirect(w, r, "/app/settings/davloom?add=1&held=1")
				return
			}
			sess.SetDraft(session.DraftDavloomAdd, nil)
			sess.SetDraft(session.DraftIssuedDevice, []byte(name+"\x00"+password+"\x00"+id))
			back = "/app/settings/davloom"
		}
	case unlockTrigger:
		if job, ok := s.catalogOf(sess).BackupJob(r.FormValue("job")); ok {
			key, _ := sess.DataKey()
			if secret, err := s.users.EnableBackupTrigger(sess.AccountID(), key, job.ID); err == nil {
				sess.SetDraft(session.DraftIssuedTrigger, []byte(job.ID+"\x00"+secret))
			}
		}
	}
	if !render.WantsFragment(r) || what == unlockDavloom || what == unlockProfile || what == unlockTrigger {
		// A plain form goes back to the page the sheet was opened over; it
		// draws the open window itself, and what was just issued.
		s.redirect(w, r, back)
		return
	}
	top := s.buildShell(sess, "").Top
	extraOOB := []render.OOB{{Target: render.TargetTopbar, Body: top}}
	_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, s.unlockSheet(sess, "", back, what, extra), extraOOB)
}

// holdDevice keeps Add device as typed while the password's sheet is asked
// (s-davloom-5).
func holdDevice(sess *session.Session, form url.Values) {
	held := url.Values{}
	for k, v := range form {
		if k != csrfFormField && k != "op" && k != "password" {
			held[k] = v
		}
	}
	raw, _ := json.Marshal(held)
	sess.SetDraft(session.DraftDavloomAdd, raw)
}

// heldDevice is Add device as it was typed before the password's sheet.
func heldDevice(sess *session.Session) (url.Values, bool) {
	raw := sess.Draft(session.DraftDavloomAdd)
	var held url.Values
	if raw == nil || json.Unmarshal(raw, &held) != nil {
		return nil, false
	}
	return held, true
}

// issuedTrigger is the secret of a job's trigger just turned on through the
// password's sheet, for that job's page, once.
func issuedTrigger(sess *session.Session, jobID string) (string, bool) {
	raw := sess.Draft(session.DraftIssuedTrigger)
	id, secret, ok := strings.Cut(string(raw), "\x00")
	if !ok || id != jobID {
		return "", false
	}
	sess.SetDraft(session.DraftIssuedTrigger, nil)
	return secret, true
}
