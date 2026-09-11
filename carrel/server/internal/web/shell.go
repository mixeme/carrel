package web

import (
	"time"

	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

func (s *Server) buildShell(sess *session.Session, title string) view.Shell {
	sh := view.Shell{
		Title: text(title),
		Top: view.TopBar{
			Search: view.SearchBox{
				Action:  "/app/search",
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
			sh.Top.Unlocked = ptrWhen(formatWhen(*until, s.tz(), false, now))
		}
		sh.Top.User = []view.MenuItem{
			{Action: view.Action{ID: "account.settings", Label: "Account", Href: "/app/settings/account", Method: "GET", Enabled: true}},
			{Action: view.Action{ID: "logout", Label: "Sign out", Href: "/logout", Method: "POST", Enabled: true}},
		}
	}
	return sh
}

func (s *Server) aboutAction() view.Action {
	return view.Action{
		ID: "about", Label: text("Carrel"), Href: "/about", Method: "GET", Enabled: true,
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

func plainForm(action, method string, rows ...view.FormRow) view.Form {
	return view.Form{
		Action: action,
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
