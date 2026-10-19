package web

import (
	"net/http"
	"net/url"
	"strings"

	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// agentLabel names a browser the way its owner will recognise it in the
// list of sessions: "Firefox · Windows". Nothing else of the User-Agent is
// kept.
func agentLabel(ua string) string {
	var browser, system string
	for _, b := range []struct{ token, name string }{
		{"Edg/", "Edge"}, {"OPR/", "Opera"}, {"YaBrowser/", "Yandex Browser"}, {"Firefox/", "Firefox"},
		{"Chrome/", "Chrome"}, {"Safari/", "Safari"}, {"curl/", "curl"},
	} {
		if strings.Contains(ua, b.token) {
			browser = b.name
			break
		}
	}
	for _, o := range []struct{ token, name string }{
		{"Android", "Android"}, {"iPhone", "iPhone"}, {"iPad", "iPad"}, {"Windows", "Windows"},
		{"Mac OS X", "macOS"}, {"Linux", "Linux"},
	} {
		if strings.Contains(ua, o.token) {
			system = o.name
			break
		}
	}
	switch {
	case browser != "" && system != "":
		return browser + " · " + system
	case browser != "":
		return browser
	case system != "":
		return system
	}
	return "Unknown browser"
}

// sessionRows are the account's live sessions (C-6 SessionRow), this one
// marked and without End: signing out does that.
func (s *Server) sessionRows(sess *session.Session) []view.SessionRow {
	tf, now := s.timeFor(sess), s.now()
	var rows []view.SessionRow
	for _, in := range s.sessions.SessionsOf(sess.AccountID()) {
		row := view.SessionRow{
			Agent: text(in.Agent), Address: text(in.Address),
			Started: tf.numStamp(in.Started), LastSeen: tf.formatWhen(in.LastSeen, true, now),
			Current: in.PublicID == sess.PublicID(),
		}
		if row.Current {
			row.End = view.Action{ID: "account.session.end", Label: "End", Method: "POST", Reason: "This is the session you are in: sign out to end it."}
		} else {
			row.End = view.Action{ID: "account.session.end", Label: "End", Method: "POST", Enabled: true,
				Href: s.postQuery("/app/settings/account", url.Values{"op": {"end"}, "s": {in.PublicID}})}
		}
		rows = append(rows, row)
	}
	return rows
}

// deleteAction is the danger zone (owner's decision 19.09.2026): the
// password's sheet, which asks the password as its confirmation and
// deletes (account-15, Typecase's note 86). What it deletes is the line
// beside it, AccountPage.DangerNote (account-1).
func (s *Server) deleteAction(sess *session.Session) view.Action {
	return view.Action{ID: "account.delete", Label: "Delete account", Danger: true, Enabled: true, Method: "GET",
		Href: s.unlockHref(unlockDelete, "/app/settings/account")}
}

// postAccountSessions ends one session or every other; true when the op was
// one of them.
func (s *Server) postAccountSessions(w http.ResponseWriter, r *http.Request, sess *session.Session) bool {
	switch r.FormValue("op") {
	case "end":
		if pub := r.FormValue("s"); pub != "" && pub != sess.PublicID() {
			if !s.sessions.EndPublic(sess.AccountID(), pub) {
				s.notFound(w, r)
				return true
			}
		}
	case "end-others":
		s.sessions.EndAccountExcept(sess.AccountID(), sess.ID())
	default:
		return false
	}
	s.redirect(w, r, "/app/settings/account")
	return true
}
