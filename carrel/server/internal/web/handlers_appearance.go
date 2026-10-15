package web

import (
	"net/http"
	"net/url"
	"strings"

	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// Appearance holds two halves of C-10 that do not mix (appearance-1;
// from-typecase note 50, the answer to the rest of task 89). The first day
// of the week and 12 or 24 hours live in the account, because time is
// assembled on the server and another browser must see the same. Theme,
// density and the lists' habits live in this browser: the one form sends
// them with the rest, and the server keeps them in a cookie of this browser
// — nothing is written to the account for a switch of the theme. Every page
// carries them (Shell.Look, put in as the page is drawn) and is printed by
// them.

// lookCookie is the cookie that keeps this browser's Look.
const lookCookie = "carrel_look"

// defaultLook is the Look of a browser that chose nothing: the frame's
// (appearance-1).
var defaultLook = view.Look{Theme: "auto", Panel: true, Source: true}

// lookOf is the Look the request's browser keeps.
func lookOf(r *http.Request) view.Look {
	look := defaultLook
	if r == nil {
		return look
	}
	c, err := r.Cookie(lookCookie)
	if err != nil {
		return look
	}
	q, err := url.ParseQuery(c.Value)
	if err != nil {
		return look
	}
	switch q.Get("theme") {
	case "light", "dark", "auto":
		look.Theme = q.Get("theme")
	}
	look.Compact = q.Get("density") == "compact"
	look.Panel = q.Get("panel") != "0"
	look.Source = q.Get("source") != "0"
	look.Wide = q.Get("wide") == "1"
	return look
}

// setLook keeps look in this browser.
func (s *Server) setLook(w http.ResponseWriter, r *http.Request, look view.Look) {
	q := url.Values{"theme": {look.Theme}, "density": {"comfortable"}, "panel": {"0"}, "source": {"0"}, "wide": {"0"}}
	if look.Compact {
		q.Set("density", "compact")
	}
	if look.Panel {
		q.Set("panel", "1")
	}
	if look.Source {
		q.Set("source", "1")
	}
	if look.Wide {
		q.Set("wide", "1")
	}
	http.SetCookie(w, &http.Cookie{Name: lookCookie, Value: q.Encode(), Path: s.cookiePath(), MaxAge: 400 * 24 * 3600,
		HttpOnly: true, Secure: s.secureCookies(r), SameSite: http.SameSiteLaxMode})
}

func (s *Server) getSettingsAppearance(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	s.writePage(w, r, http.StatusOK, s.appearancePage(sess, lookOf(r), "", ""))
}

func (s *Server) postSettingsAppearance(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	if r.URL.Query().Get("op") == "reset" {
		// «Reset remembered view settings»: the columns of every section and
		// this browser's look are forgotten.
		for _, c := range r.Cookies() {
			if strings.HasPrefix(c.Name, "carrel_cols_") || c.Name == lookCookie {
				http.SetCookie(w, &http.Cookie{Name: c.Name, Value: "", Path: s.cookiePath(), MaxAge: -1,
					HttpOnly: true, Secure: s.secureCookies(r), SameSite: http.SameSiteLaxMode})
			}
		}
		s.redirect(w, r, "/app/settings/appearance")
		return
	}
	look := view.Look{Theme: r.FormValue("theme"), Compact: r.FormValue("density") == "compact",
		Panel: r.FormValue("panel") != "", Source: r.FormValue("source") != "", Wide: r.FormValue("wide") != ""}
	if look.Theme != "light" && look.Theme != "dark" {
		look.Theme = "auto"
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
		s.writePage(w, r, http.StatusOK, s.appearancePage(sess, look, weekErr, clockErr))
		return
	}
	if err := s.users.SetTimePrefs(sess.AccountID(), week, clock == "12"); err != nil {
		http.Error(w, "could not save the settings", http.StatusInternalServerError)
		return
	}
	s.setLook(w, r, look)
	s.redirect(w, r, "/app/settings/appearance")
}

func (s *Server) appearancePage(sess *session.Session, look view.Look, weekErr, clockErr view.Text) view.AppearancePage {
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
	tick := func(name, label string, on bool) view.FormRow {
		row := view.FormRow{Name: name, Label: text(label), Kind: view.InputCheck}
		if on {
			row.Value = "on"
		}
		return row
	}
	density := "comfortable"
	if look.Compact {
		density = "compact"
	}
	form := view.Form{Action: s.route("/app/settings/appearance"), Method: "POST", Sets: []view.FieldSet{
		{Rows: []view.FormRow{csrfField(sess)}},
		{Legend: "Theme", Rows: []view.FormRow{{Name: "theme", Label: "Theme", Kind: view.InputRadio, Value: text(look.Theme),
			Hint:    "“Auto” follows the system. Kept in this browser — another device does not see it.",
			Options: []view.Option{option("auto", "Auto", look.Theme), option("light", "Light", look.Theme), option("dark", "Dark", look.Theme)}}}},
		{Legend: "Density", Rows: []view.FormRow{{Name: "density", Label: "Density", Kind: view.InputRadio, Value: text(density),
			Hint:    "List row height: 38 or 32 px. On a phone always 52. This browser.",
			Options: []view.Option{option("comfortable", "Comfortable", density), option("compact", "Compact", density)}}}},
		{Legend: "Lists", Rows: []view.FormRow{
			tick("panel", "Keep the details panel open", look.Panel),
			tick("source", "Show the source column", look.Source),
			tick("wide", "Open notes in full width", look.Wide),
		}},
		{Legend: "Dates and times", Rows: []view.FormRow{
			{Name: "weekStart", Label: "Week starts on", Kind: view.InputSelect, Value: text(week), Error: weekErr,
				Options: []view.Option{option("monday", "Monday", week), option("sunday", "Sunday", week), option("saturday", "Saturday", week)}},
			{Name: "clock", Label: "Time", Kind: view.InputSelect, Value: text(clock), Error: clockErr,
				Options: []view.Option{option("24", "24-hour", clock), option("12", "12-hour", clock)}},
		}},
	}}
	form.Actions = []view.Action{s.discard("appearance.discard", "/app/settings/appearance"),
		{ID: "appearance.save", Label: "Save", Method: "POST", Primary: true, Enabled: true}}
	return view.AppearancePage{
		Shell: s.withRail(s.buildShell(sess, "Appearance"), s.settingsRail(sess, "/app/settings/appearance")),
		Form:  form,
		Reset: view.Action{ID: "appearance.reset", Label: "Reset remembered view settings", Method: "POST", Enabled: true,
			Href: s.postQuery("/app/settings/appearance", url.Values{"op": {"reset"}})},
		Instance: view.Form{Sets: []view.FieldSet{{Rows: []view.FormRow{
			{Name: "timezone", Label: "Time zone", Kind: view.InputText, Value: text(tf.tz.String()), Hint: "This instance", ReadOnly: true},
		}}}},
		Notes: []view.Text{
			"Column sets are remembered per section, in this browser — the button below resets them.",
			"Week start and 12/24-hour clock live in the account: time is assembled on the server, so another browser sees the same.",
			"Time zone is one per instance, set by the administrator.",
		},
	}
}
