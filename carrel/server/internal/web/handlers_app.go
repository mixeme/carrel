package web

import (
	"errors"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// getRoot is the bare address of the instance: into the application, as
// /app/ leads — the sign-in, the setup of an empty instance, or the
// contacts (C-5).
func (s *Server) getRoot(w http.ResponseWriter, r *http.Request) {
	s.redirect(w, r, "/app/")
}

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
		r.FormValue("current"), r.FormValue("password"), r.FormValue("repeat"))
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
		Shell: s.settingsShell(sess, "Change password", "/app/settings/account", "/app/password"),
		// The caret is in New password, not in the field before it
		// (account-3).
		Form: focused(withPrimary(s.plainForm("/app/password", "POST", csrfField(sess),
			view.FormRow{Name: "current", Label: "Current password", Kind: view.InputPassword, Required: true, Error: currentErr},
			view.FormRow{Name: "password", Label: "New password", Kind: view.InputPassword, Required: true, Error: passErr},
			view.FormRow{Name: "repeat", Label: "Repeat new password", Kind: view.InputPassword, Required: true, Error: pass2Err},
		), "password.save", "Change password"), "password"),
		// What the change does is said under the fields, where the fear of
		// losing the connections comes up (account-3).
		Note: "Your data stays encrypted under the same key; only the wrapping changes. Saved connections keep working.",
		Back: &view.Action{ID: "password.back", Label: "Back to settings", Href: s.route("/app/settings/account"), Method: "GET", Enabled: true},
	}
	// After a temporary password there is nowhere to go before the change:
	// the red line on top, and no way back (account-3).
	if mustChange {
		// Nothing further before the change (account-17, note 101): no
		// settings' rail, no search, New or refresh; of the account's menu
		// only Sign out.
		page.Back = nil
		page.Rail = view.Rail{}
		page.Top.Search, page.Top.New, page.Top.Refresh = view.SearchBox{}, view.Action{}, view.Refresh{}
		var out []view.MenuItem
		for _, it := range page.Top.User {
			if it.ID == "user.logout" {
				out = append(out, it)
			}
		}
		page.Top.User = out
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
	// Plain http to this very machine crosses no network: nothing goes in
	// the clear (a server beside Carrel, a stand).
	if strings.HasPrefix(lower, "http://") && !loopbackURL(server) {
		out = append(out, view.Notice{
			Role:  view.NoticeAlert,
			Title: text("Unencrypted connection"),
			Body:  text("Credentials will be sent in clear text. Continue only if you accept this risk."),
		})
	}
	// An https address says nothing over the dialog (s-profile-2, -16):
	// the fingerprint's field says when one is needed.
	return out
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
		_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, s.unlockSheet(sess, "", back, r.FormValue("for"), r.URL.Query()), nil)
		return
	}
	// Without scripts a sheet is a page of its own (U-8): "Show password" is
	// a plain link here, and a 404 behind it left no way to the password.
	sheet := s.unlockSheet(sess, "", back, r.FormValue("for"), r.URL.Query())
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

// loopbackURL reports whether an address names this machine: localhost or
// a loopback IP.
func loopbackURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if ip, err := netip.ParseAddr(host); err == nil {
		return ip.IsLoopback()
	}
	return host == "localhost" || strings.HasSuffix(host, ".localhost")
}
