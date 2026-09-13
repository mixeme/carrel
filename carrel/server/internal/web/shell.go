package web

import (
	"net/http"
	"time"

	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

func (s *Server) buildShell(sess *session.Session, title string) view.Shell {
	sh := view.Shell{
		Title: text(title),
		Top: view.TopBar{
			Search: view.SearchBox{
				Action:  s.route("/app/search"),
				History: nil,
				Jump:    nil,
				Whole:   nil,
			},
			NewMenu: nil,
			User:    nil,
			Refresh: view.Refresh{},
		},
		Rail: view.Rail{
			About: s.aboutAction(),
		},
	}
	if sess != nil {
		now := time.Now()
		if until := sess.UnlockUntil(now); until != nil {
			sh.Top.Unlocked = ptrWhen(s.timeFor(sess).formatWhen(*until, false, now))
		}
		if history := sess.History(); len(history) > 0 {
			tf := s.timeFor(sess)
			for _, e := range history {
				sh.Top.Search.History = append(sh.Top.Search.History, view.SearchHistoryRow{
					Query:   text(e.Query),
					Outcome: text(historyOutcome(e)),
					When:    tf.formatWhen(e.At, true, now),
					Href:    s.route(searchHref(e.Query)),
				})
			}
		}
		sh.Top.User = []view.MenuItem{
			{Action: view.Action{ID: "account.settings", Label: "Account", Href: s.route("/app/settings/account"), Method: "GET", Enabled: true}},
			{Action: view.Action{ID: "logout", Label: "Sign out", Href: s.route("/logout"), Method: "POST", Enabled: true}},
		}
		if s.users.ConsumeEscrowNotice(sess.AccountID()) {
			sh.Notices = []view.Notice{{
				Role:  view.NoticeAlert,
				Title: text("Key escrow applies to your account"),
				Body:  text("An administrator who holds the escrow master password can decrypt the DAV credentials you save here, and through them your contacts and calendars. This is not backup."),
			}}
		}
	}
	return sh
}

func (s *Server) aboutAction() view.Action {
	return view.Action{
		ID: "about", Label: text("Carrel"), Href: s.route("/about"), Method: "GET", Enabled: true,
	}
}

func (s *Server) tz() *time.Location {
	loc, err := time.LoadLocation(s.cfg.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

func ptrWhen(w view.When) *view.When { return &w }

func (s *Server) plainForm(action, method string, rows ...view.FormRow) view.Form {
	return view.Form{
		Action: s.route(action),
		Method: method,
		Sets: []view.FieldSet{{
			Rows: rows,
		}},
	}
}

func csrfField(sess *session.Session) view.FormRow {
	if sess == nil {
		return view.FormRow{Name: "_csrf", Kind: view.InputText, Value: ""}
	}
	return view.FormRow{Name: "_csrf", Kind: view.InputText, Value: text(sess.CSRFToken())}
}

func csrfFormRow(token string) view.FormRow {
	return view.FormRow{Name: "_csrf", Kind: view.InputText, Value: text(token)}
}

func (s *Server) anonPlainForm(w http.ResponseWriter, r *http.Request, action, method string, rows ...view.FormRow) view.Form {
	token := s.issueAnonymousCSRF(w, r)
	all := append([]view.FormRow{csrfFormRow(token)}, rows...)
	return s.plainForm(action, method, all...)
}
