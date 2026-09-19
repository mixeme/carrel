package web

import (
	"net/http"
	"net/url"
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
			NewMenu: s.newMenu(sess),
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
		return view.FormRow{Name: "_csrf", Kind: view.InputHidden, Value: ""}
	}
	return view.FormRow{Name: "_csrf", Kind: view.InputHidden, Value: text(sess.CSRFToken())}
}

func csrfFormRow(token string) view.FormRow {
	return view.FormRow{Name: "_csrf", Kind: view.InputHidden, Value: text(token)}
}

func (s *Server) anonPlainForm(w http.ResponseWriter, r *http.Request, action, method string, rows ...view.FormRow) view.Form {
	token := s.issueAnonymousCSRF(w, r)
	all := append([]view.FormRow{csrfFormRow(token)}, rows...)
	return s.plainForm(action, method, all...)
}

// newMenu is the create menu of the top bar: one item per kind, each opening
// the quick sheet from the shell (U-6.16). The sheet itself decides where the
// record goes — the collection the last one was created in, or a question when
// there is none — so the menu does not need to know about collections.
func (s *Server) newMenu(sess *session.Session) []view.MenuItem {
	if sess == nil {
		return nil
	}
	kinds := []struct {
		segment string
		label   string
	}{
		{"contacts", "Contact"},
		{"calendar", "Event"},
		{"tasks", "Task"},
		{"notes", "Note"},
	}
	out := make([]view.MenuItem, 0, len(kinds))
	for _, k := range kinds {
		out = append(out, view.MenuItem{Action: view.Action{
			ID:      "new." + k.segment,
			Label:   text(k.label),
			Href:    s.route("/app/" + k.segment + "/quick"),
			Method:  "GET",
			Enabled: true,
		}})
	}
	// A file and a letter create nothing in a collection of records, so they
	// stand last, past the line (U-6.4). The upload asks where on its sheet;
	// without mail on the instance the letter is not offered.
	out = append(out, view.MenuItem{Separator: true, Action: view.Action{
		ID: "new.upload", Label: "Upload file", Icon: "i-up", Href: s.postQuery("/app/files/picker", url.Values{"for": {"upload"}}), Method: "GET", Enabled: true,
	}})
	if s.mailOn() {
		out = append(out, view.MenuItem{Action: view.Action{
			ID: "new.letter", Label: "Letter", Icon: "i-mail", Href: s.route("/app/mail/compose"), Method: "GET", Enabled: true,
		}})
	}
	return out
}

// withSubmit gives a form the button that sends it, and any others after it.
// The words on a button are the server's (C-3.1): a client that has to guess
// "Submit" is guessing the product.
func withSubmit(f view.Form, id, label string, more ...view.Action) view.Form {
	f.Actions = append([]view.Action{{ID: id, Label: text(label), Href: f.Action, Method: f.Method, Enabled: true}}, more...)
	return f
}
