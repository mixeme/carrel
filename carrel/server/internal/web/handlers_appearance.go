package web

import (
	"net/http"

	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// Appearance holds the account half of C-10: the first day of the week and
// 12 or 24 hours live in the account, because time is assembled on the server
// and another browser must see the same. Theme, density and columns live in
// the browser; the server knows nothing of them, so Browser stays empty.

func (s *Server) getSettingsAppearance(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	s.writePage(w, r, http.StatusOK, s.appearancePage(sess, "", ""))
}

func (s *Server) postSettingsAppearance(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	week, clock := r.FormValue("weekStart"), r.FormValue("clock")
	var weekErr, clockErr view.Text
	if _, known := user.WeekStarts[week]; !known {
		weekErr = text("Choose a day from the list.")
	}
	if clock != "12" && clock != "24" {
		clockErr = text("Choose 12-hour or 24-hour.")
	}
	if weekErr != "" || clockErr != "" {
		s.writePage(w, r, http.StatusOK, s.appearancePage(sess, weekErr, clockErr))
		return
	}
	if err := s.users.SetTimePrefs(sess.AccountID(), week, clock == "12"); err != nil {
		http.Error(w, "could not save the settings", http.StatusInternalServerError)
		return
	}
	s.redirect(w, r, "/app/settings/appearance")
}

func (s *Server) appearancePage(sess *session.Session, weekErr, clockErr view.Text) view.AppearancePage {
	tf := s.timeFor(sess)
	week, clock := "monday", "24"
	for name, day := range user.WeekStarts {
		if day == tf.week {
			week = name
		}
	}
	if tf.h12 {
		clock = "12"
	}
	option := func(value, label, selected string) view.Option {
		return view.Option{Value: value, Label: text(label), Selected: value == selected}
	}
	account := s.plainForm("/app/settings/appearance", "POST",
		csrfField(sess),
		view.FormRow{Name: "weekStart", Label: "Week starts on", Kind: view.InputSelect, Value: text(week), Error: weekErr,
			Options: []view.Option{option("monday", "Monday", week), option("sunday", "Sunday", week), option("saturday", "Saturday", week)}},
		view.FormRow{Name: "clock", Label: "Time", Kind: view.InputSelect, Value: text(clock), Error: clockErr,
			Options: []view.Option{option("24", "24-hour", clock), option("12", "12-hour", clock)}},
	)
	account.Actions = []view.Action{{ID: "appearance.save", Label: "Save", Method: "POST", Enabled: true}}
	return view.AppearancePage{
		Shell:   s.withRail(s.buildShell(sess, "Appearance"), s.settingsRail(sess, "/app/settings/appearance")),
		Account: account,
		Instance: view.Form{Sets: []view.FieldSet{{Rows: []view.FormRow{
			{Name: "timezone", Label: "Time zone", Kind: view.InputText, Value: text(tf.tz.String()), Hint: "This instance", ReadOnly: true},
		}}}},
		Notes: []view.Text{
			"Week start and 12/24-hour clock live in the account: time is assembled on the server, so another browser sees the same.",
			"Time zone is one per instance, set by the administrator.",
		},
	}
}
