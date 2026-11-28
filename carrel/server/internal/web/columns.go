package web

import (
	"net/http"
	"net/url"
	"slices"
	"strings"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/view"
)

// The choice of columns of a list (from-typecase notes 23 and 25, the
// owner's decisions; components-21, components-42, S-9.6). The Columns menu
// is a form: its ticked items come as col, in their order, «Reset to
// defaults» sends col=default. The choice is this browser's (U-12): a cookie
// per section, read back on every list of the section — the combined view
// and each collection alike. Nothing of it goes to the account or its blob.

// listColumn is a column a list offers: its id (the value of col), its
// label, and whether the section shows it until the person chooses.
type listColumn struct {
	id, label string
	shown     bool
}

// listColumns are the columns of each section that has the choice, in the
// order of its menu frame: the order they stand in by default. The first is
// the row's name, always shown and always first. No section shows tags in
// its rows until asked to (Typecase Р-53, note 44): they are in the panel of
// the record and in the rail. contacts: components-21,
// -42; tasks: s-tasks-12; notes: s-notes-23; files: s-files-20; users:
// s-admin-21; audit: s-admin-23; timeline: s-contacts-25.
var listColumns = map[string][]listColumn{
	"contacts": {
		{"name", "Name", true}, {"phone", "Phone", true}, {"email", "Email", true}, {"org", "Organisation", true},
		{"birthday", "Birthday", false}, {"tags", "Tags", false}, {"changed", "Changed", false}, {"source", "Source", true},
	},
	// The line of an agenda (s-agenda-18): Time and Title stand always.
	"calendar": {
		{"time", "Time", true}, {"title", "Title", true}, {"where", "Where", true}, {"attendees", "Attendees and status", true},
		{"source", "Source", true}, {"tags", "Tags", false}, {"changed", "Changed", false},
	},
	"tasks": {
		{"title", "Title", true}, {"due", "Due", true}, {"complete", "Complete", true}, {"source", "Source", true},
		{"tags", "Tags", false}, {"priority", "Priority", false}, {"status", "Status", false}, {"changed", "Changed", false},
		// Where a task is (Р10 of scans/kinds-parity.md): the agenda has the
		// column, a task has LOCATION; no frame draws it — Typecase's task 170.
		{"where", "Where", false},
		// A task's people, as the agenda's column (after Р3), last and off
		// (s-tasks-12).
		{"attendees", "Attendees", false},
	},
	"notes": {
		{"title", "Title", true}, {"excerpt", "Excerpt", true}, {"date", "Date", true}, {"source", "Source", true},
		{"tags", "Tags", false}, {"changed", "Changed", false},
		// No column of people: a note's are not edited (Typecase's Р-64).
	},
	"files": {
		{"name", "Name", true}, {"size", "Size", true}, {"type", "Type", true}, {"changed", "Changed", true},
	},
	"users": {
		{"login", "Login", true}, {"role", "Role", true}, {"created", "Created", true}, {"last-login", "Last login", true},
		{"dav", "DAV", true}, {"sessions", "Sessions", true}, {"escrow", "Escrow", true}, {"address", "Address", false},
	},
	"audit": {
		{"when", "When", true}, {"action", "Action", true}, {"actor", "Actor", true}, {"target", "Target", true}, {"detail", "Detail", true},
	},
	"timeline": {
		{"title", "Title", true}, {"date", "Date", true}, {"kind", "Kind", true}, {"source", "Source", true},
	},
}

// recordSections are the sections of records, whose lists are a combined
// view and one collection each; only the combined view offers Source — one
// collection names its source in its head (note 25).
var recordSections = []string{"contacts", "calendar", "tasks", "notes"}

// fixedColumns is how many of a section's columns stand always, first and
// in their order: one — the row's name — unless said here.
var fixedColumns = map[string]int{"calendar": 2}

// fixedOf are the ids of the columns of a section that stand always.
func fixedOf(segment string) []string {
	n := fixedColumns[segment]
	if n == 0 {
		n = 1
	}
	var out []string
	for _, c := range listColumns[segment][:min(n, len(listColumns[segment]))] {
		out = append(out, c.id)
	}
	return out
}

// sectionKinds are the kinds whose orders (listSorts) a section's list is
// sorted by.
var sectionKinds = map[string]view.Kind{
	"contacts": view.KindContact, "tasks": view.KindTask, "notes": view.KindNote, "files": view.KindFile,
}

// offeredColumns are the columns a list of the section offers: one
// collection's list of records offers no Source.
func offeredColumns(segment string, single bool) []listColumn {
	cols := listColumns[segment]
	if !single || !slices.Contains(recordSections, segment) {
		return cols
	}
	var out []listColumn
	for _, c := range cols {
		if c.id != "source" {
			out = append(out, c)
		}
	}
	return out
}

func columnIDs(cols []listColumn, shownOnly bool) []string {
	var out []string
	for _, c := range cols {
		if c.shown || !shownOnly {
			out = append(out, c.id)
		}
	}
	return out
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
	seen := map[string]bool{}
	out := fixedOf(segment)
	for _, id := range out {
		seen[id] = true
	}
	fixed := len(out)
	for _, id := range ids {
		if known[id] && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	if len(out) == fixed && !knownAny(known, ids) {
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

// sortColumn is the column an order of the section sorts by, or "" when
// the value names no order of it.
func sortColumn(segment, value string) string {
	k, ok := sectionKinds[segment]
	if !ok || value == "" {
		return ""
	}
	for _, o := range listSorts[k] {
		if o.value == value {
			return o.column
		}
	}
	return ""
}

// columnsShown are the columns a list draws, in their order: the choice, or
// the section's default, of what the list offers — and the column of the
// order the address asks for, which choosing that order shows (s-tasks-13).
// Without a choice every offered column stands in its default place, so
// the one the order shows takes its own place among them.
func columnsShown(r *http.Request, segment string, query url.Values, single bool) (shown []string, chosen bool) {
	cols := offeredColumns(segment, single)
	offered := columnIDs(cols, false)
	forced := sortColumn(segment, strings.TrimSpace(query.Get("sort")))
	if choice := chosenColumns(r, segment); choice != nil {
		for _, id := range choice {
			if slices.Contains(offered, id) {
				shown = append(shown, id)
			}
		}
		if forced != "" && !slices.Contains(shown, forced) {
			shown = append(shown, forced)
		}
		return shown, true
	}
	for _, c := range cols {
		if c.shown || c.id == forced {
			shown = append(shown, c.id)
		}
	}
	return shown, false
}

// columnsMenu is the Columns of a list's bar (components-42): the form that
// posts to action, keeping the list's query, and the items — the columns
// shown, in their order, then the others in their default order; until a
// choice, every item in its default order. Nil for a section without the
// choice.
func (s *Server) columnsMenu(r *http.Request, csrf view.FormRow, segment, action string, query url.Values, single bool) *view.Columns {
	cols := offeredColumns(segment, single)
	if len(cols) == 0 {
		return nil
	}
	q := cloneValues(query)
	q.Del("unsorted")
	if len(q) > 0 {
		action += "?" + q.Encode()
	}
	form := s.plainForm(action, http.MethodPost, csrf)
	form.Actions = []view.Action{
		{ID: "columns.apply", Label: "Apply", Href: form.Action, Method: http.MethodPost, Primary: true, Enabled: true},
		{ID: "columns.default", Label: "Reset to defaults", Href: form.Action, Method: http.MethodPost, Enabled: true},
	}
	item := func(c listColumn, on bool) view.ColumnItem {
		it := view.ColumnItem{ID: c.id, Label: text(c.label), On: on}
		if slices.Contains(fixedOf(segment), c.id) {
			it.On, it.Fixed, it.Aside = true, true, "always"
		}
		return it
	}
	menu := &view.Columns{Label: "Columns", Icon: "i-cols", Form: form}
	shown, chosen := columnsShown(r, segment, query, single)
	if !chosen {
		for _, c := range cols {
			menu.Items = append(menu.Items, item(c, slices.Contains(shown, c.id)))
		}
		return menu
	}
	for _, id := range shown {
		for _, c := range cols {
			if c.id == id {
				menu.Items = append(menu.Items, item(c, true))
			}
		}
	}
	for _, c := range cols {
		if !slices.Contains(shown, c.id) {
			menu.Items = append(menu.Items, item(c, false))
		}
	}
	return menu
}

// saveColumns takes the form of the Columns menu and answers with the list
// it came from, back, its query kept (S-9.6): the ticked columns, in the
// order they came, are this browser's choice for the section; col=default
// forgets it. The first column is not sent — its box is off, it is always
// shown. A choice made in one collection's menu, which has no Source,
// leaves Source where the section's choice had it. A choice that takes away
// the column the list is sorted by puts the list in its default order and
// tells it so (unsorted=<the order>); the caller has checked who asks.
func (s *Server) saveColumns(w http.ResponseWriter, r *http.Request, segment, back string, single bool) {
	if err := readForm(r); err != nil {
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
	var shown []string
	// Reset to defaults is a button of the same form: its value comes with
	// the boxes ticked, whatever they are (the owner's phone, 28.11.2026).
	if slices.Contains(cols, "default") {
		cookie.MaxAge = -1
		shown = columnIDs(offeredColumns(segment, single), true)
	} else {
		ids := knownColumns(segment, append(fixedOf(segment), cols...))
		if single && slices.Contains(recordSections, segment) && !slices.Contains(ids, "source") {
			ids = keptSource(r, segment, ids)
		}
		cookie.Value = strings.Join(ids, ".")
		cookie.MaxAge = int(conf.ColumnsChoiceAge.Seconds())
		shown = ids
	}
	http.SetCookie(w, cookie)
	q := r.URL.Query()
	changed := q.Has("unsorted")
	q.Del("unsorted")
	if order := strings.TrimSpace(q.Get("sort")); order != "" {
		if col := sortColumn(segment, order); col != "" && !slices.Contains(shown, col) {
			q.Del("sort")
			if order != listSorts[sectionKinds[segment]][0].value {
				q.Set("unsorted", order)
			}
			changed = true
		}
	}
	switch {
	case !changed && r.URL.RawQuery != "":
		back += "?" + r.URL.RawQuery
	case len(q) > 0:
		back += "?" + q.Encode()
	}
	s.redirect(w, r, back)
}

// keptSource puts Source back into a choice made where it was not offered,
// at the place the section's choice — or its default — had it.
func keptSource(r *http.Request, segment string, ids []string) []string {
	before := chosenColumns(r, segment)
	if before == nil {
		before = columnIDs(listColumns[segment], true)
	}
	at := slices.Index(before, "source")
	if at < 0 {
		return ids
	}
	return slices.Insert(ids, min(at, len(ids)), "source")
}

// unsortedNotice is the word of a list whose order went back to its default
// because the column it was sorted by was taken away (saveColumns). The
// address loses the mark, so no link of the page carries it on; nil when the
// address has none, or the person chose an order since.
func unsortedNotice(r *http.Request, segment string) *view.Notice {
	q := r.URL.Query()
	if !q.Has("unsorted") {
		return nil
	}
	n := unsortedNoticeOf(q, segment)
	r.URL.RawQuery = q.Encode()
	return n
}

// unsortedNoticeOf is unsortedNotice over a query, which loses the mark.
func unsortedNoticeOf(q url.Values, segment string) *view.Notice {
	if !q.Has("unsorted") {
		return nil
	}
	was := q.Get("unsorted")
	q.Del("unsorted")
	k, ok := sectionKinds[segment]
	if !ok || q.Get("sort") != "" {
		return nil
	}
	col := sortColumn(segment, was)
	if col == "" {
		return nil
	}
	label := col
	for _, c := range listColumns[segment] {
		if c.id == col {
			label = c.label
		}
	}
	return &view.Notice{
		Role:  view.NoticeInfo,
		Title: "Back to the default order",
		Body: text(label + " is no longer shown, so the list is no longer sorted by it; it is in its default order, " +
			listSorts[k][0].label + "."),
	}
}

// The routes of the menu's form: the combined view and each collection of
// a section of records, a folder of files, a person's timeline, the users
// and the audit log.
func combinedColumnsHandler(segment string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		if _, ok := s.requireLogin(w, r); !ok {
			return
		}
		s.saveColumns(w, r, segment, "/app/"+segment, false)
	}
}

func collectionColumnsHandler(segment string) handler {
	return func(s *Server, w http.ResponseWriter, r *http.Request) {
		if _, ok := s.requireLogin(w, r); !ok {
			return
		}
		s.saveColumns(w, r, segment, "/app/"+segment+"/"+url.PathEscape(r.PathValue("account"))+"/"+url.PathEscape(r.PathValue("col")), true)
	}
}

func (s *Server) postFilesColumns(w http.ResponseWriter, r *http.Request) {
	c, ok := s.openFiles(w, r, "columns")
	if !ok {
		return
	}
	s.saveColumns(w, r, "files", c.href("", c.segs, true), false)
}

func (s *Server) postTimelineColumns(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireLogin(w, r); !ok {
		return
	}
	s.saveColumns(w, r, "timeline", "/app/contacts/"+url.PathEscape(r.PathValue("account"))+"/"+url.PathEscape(r.PathValue("col"))+
		"/"+url.PathEscape(r.PathValue("uid"))+"/timeline", false)
}

func (s *Server) postAdminColumns(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	s.saveColumns(w, r, "users", "/admin/", false)
}

func (s *Server) postAuditColumns(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireAdmin(w, r); !ok {
		return
	}
	s.saveColumns(w, r, "audit", "/admin/audit", false)
}
