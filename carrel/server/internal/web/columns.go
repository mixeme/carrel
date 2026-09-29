package web

import (
	"net/http"
	"net/url"
	"strings"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/view"
)

// The choice of columns of a list (from-typecase note 23, the owner's
// decision — whole; components-21, components-42, S-9.6). The Columns menu is
// a form: its ticked items come as col, in their order, «Reset to defaults»
// sends col=default. The choice is this browser's (U-12): a cookie per
// section, read back on every list of the section — the combined view and
// each collection alike. Nothing of it goes to the account or its blob.

// listColumn is a column a list offers: its id (the value of col), its
// label, and whether the section shows it until the person chooses.
type listColumn struct {
	id, label string
	shown     bool
}

// listColumns are the columns of each section that has the choice, in the
// order of components-21: the order they stand in by default. The first is
// the record's name, always shown and always first. Only the contacts have
// the menu drawn (components-21, -42, s-contacts-23, -24); the other lists
// wait for their frames (to-typecase task 45).
var listColumns = map[string][]listColumn{
	"contacts": {
		{"name", "Name", true}, {"phone", "Phone", true}, {"email", "Email", true}, {"org", "Organisation", true},
		{"birthday", "Birthday", false}, {"tags", "Tags", false}, {"changed", "Changed", false}, {"source", "Source", true},
	},
}

// columnsCookie is the cookie that keeps a section's choice.
func columnsCookie(segment string) string { return "carrel_cols_" + segment }

// chosenColumns is the choice this browser keeps for the section: the ids
// it knows, each once, the first column first. Nil — no choice: the
// cookie is not there, or names nothing the section has.
func chosenColumns(r *http.Request, segment string) []string {
	c, err := r.Cookie(columnsCookie(segment))
	if err != nil {
		return nil
	}
	return knownColumns(segment, strings.Split(c.Value, "."))
}

// knownColumns keeps of ids those the section has, each once, and puts the
// first column in front. Nil when none is known.
func knownColumns(segment string, ids []string) []string {
	cols := listColumns[segment]
	if len(cols) == 0 {
		return nil
	}
	known := map[string]bool{}
	for _, c := range cols {
		known[c.id] = true
	}
	seen := map[string]bool{cols[0].id: true}
	out := []string{cols[0].id}
	for _, id := range ids {
		if known[id] && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	if len(out) == 1 && !knownAny(known, ids) {
		return nil
	}
	return out
}

func knownAny(known map[string]bool, ids []string) bool {
	for _, id := range ids {
		if known[id] {
			return true
		}
	}
	return false
}

// columnsMenu is the Columns of a list's bar (components-42): the form that
// posts to the list's address plus /columns, keeping the list's query, and
// the items — the columns shown, in their order, then the others in the
// order of components-21. Nil for a section without the choice.
func (s *Server) columnsMenu(r *http.Request, csrf view.FormRow, segment, listPath string) *view.Columns {
	cols := listColumns[segment]
	if len(cols) == 0 {
		return nil
	}
	action := listPath + "/columns"
	if q := r.URL.RawQuery; q != "" {
		action += "?" + q
	}
	form := s.plainForm(action, http.MethodPost, csrf)
	form.Actions = []view.Action{
		{ID: "columns.apply", Label: "Apply", Href: form.Action, Method: http.MethodPost, Enabled: true},
		{ID: "columns.default", Label: "Reset to defaults", Href: form.Action, Method: http.MethodPost, Enabled: true},
	}
	item := func(c listColumn, on bool) view.ColumnItem {
		it := view.ColumnItem{ID: c.id, Label: text(c.label), On: on}
		if c.id == cols[0].id {
			it.On, it.Fixed, it.Aside = true, true, "always"
		}
		return it
	}
	menu := &view.Columns{Label: "Columns", Form: form}
	chosen := chosenColumns(r, segment)
	if chosen == nil {
		for _, c := range cols {
			menu.Items = append(menu.Items, item(c, c.shown))
		}
		return menu
	}
	on := map[string]bool{}
	for _, id := range chosen {
		on[id] = true
		for _, c := range cols {
			if c.id == id {
				menu.Items = append(menu.Items, item(c, true))
			}
		}
	}
	for _, c := range cols {
		if !on[c.id] {
			menu.Items = append(menu.Items, item(c, false))
		}
	}
	return menu
}

// postColumns takes the form of the Columns menu and answers with the list
// it came from, its query kept (S-9.6): the ticked columns, in the order they
// came, are this browser's choice for the section; col=default forgets it.
// The first column is not sent — its box is off, it is always shown.
func (s *Server) postColumns(w http.ResponseWriter, r *http.Request, segment, listPath string) {
	if _, ok := s.requireLogin(w, r); !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	cookie := &http.Cookie{
		Name:     columnsCookie(segment),
		Path:     s.cookiePath(),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.secureCookies(r),
	}
	cols := r.PostForm["col"]
	if len(cols) == 1 && cols[0] == "default" {
		cookie.MaxAge = -1
	} else {
		cookie.Value = strings.Join(knownColumns(segment, append([]string{listColumns[segment][0].id}, cols...)), ".")
		cookie.MaxAge = int(conf.ColumnsChoiceAge.Seconds())
	}
	http.SetCookie(w, cookie)
	back := listPath
	if q := r.URL.RawQuery; q != "" {
		back += "?" + q
	}
	s.redirect(w, r, back)
}

// The routes of the menu's form: the combined view and each collection of
// a section with the choice.
func combinedColumnsHandler(segment string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		s.postColumns(w, r, segment, "/app/"+segment)
	}
}

func collectionColumnsHandler(segment string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		s.postColumns(w, r, segment, "/app/"+segment+"/"+url.PathEscape(r.PathValue("account"))+"/"+url.PathEscape(r.PathValue("col")))
	}
}
