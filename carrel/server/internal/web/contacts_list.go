package web

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/card"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// What a list of contacts carries beside its rows, as the frames draw it
// (s-contacts-1, s-contacts-9, s-io-1, states-4; the client's scan of the
// contacts section, group Б): the line under the title, Print, the filter
// and Compact rows that work without scripts, the row open in the panel and
// the panel itself, and what a list with nothing to show says (U-6.0).

// openKey carries ?open= past the builders of a list: the panel's row is
// not a query the list's other links keep (C-6.4).
type openKey struct{}

// withoutOpen takes ?open= out of the request, to the context.
func withoutOpen(r *http.Request) *http.Request {
	q := r.URL.Query()
	open := q.Get("open")
	if open == "" {
		return r
	}
	q.Del("open")
	r = r.WithContext(context.WithValue(r.Context(), openKey{}, open))
	u := *r.URL
	u.RawQuery = q.Encode()
	r.URL = &u
	return r
}

// compactCookie is this browser's «Compact rows» of a section (S-9.6).
func compactCookie(segment string) string { return "carrel_compact_" + segment }

// compactChoice takes «Compact rows» (?compact=1 or 0): the choice is this
// browser's, kept by a cookie of the section, and the list comes back
// without the mark. It reports whether it answered.
func (s *Server) compactChoice(w http.ResponseWriter, r *http.Request, segment, back string) bool {
	q := r.URL.Query()
	v, ok := q["compact"]
	if !ok {
		return false
	}
	cookie := &http.Cookie{Name: compactCookie(segment), Path: s.cookiePath(), HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: s.secureCookies(r), MaxAge: -1}
	if len(v) > 0 && v[0] == "1" {
		cookie.Value, cookie.MaxAge = "1", int(conf.ColumnsChoiceAge.Seconds())
	}
	http.SetCookie(w, cookie)
	q.Del("compact")
	if len(q) > 0 {
		back += "?" + q.Encode()
	}
	s.redirect(w, r, back)
	return true
}

// compactAction is «Compact rows» of the bar: on when this browser chose it,
// and a link to the other choice.
func (s *Server) compactAction(r *http.Request, segment, base string) *view.Action {
	on := false
	if c, err := r.Cookie(compactCookie(segment)); err == nil && c.Value == "1" {
		on = true
	}
	q := r.URL.Query()
	q.Set("compact", map[bool]string{true: "0", false: "1"}[on])
	return &view.Action{ID: "list.compact", Label: "Compact rows", Href: s.route(base + "?" + q.Encode()), Method: "GET", Enabled: true, On: on}
}

// printAction is Print of a list's head (the owner's decision 4 of the
// client's plan, 28.09.2026): the same address with print=1, the same view
// model, drawn on paper by the client (U-10).
func (s *Server) printAction(base string) view.Action {
	return view.Action{ID: "list.print", Label: "Print", Icon: "i-print", Href: s.route(base + "?print=1"), Method: "GET", Enabled: true}
}

// listFilter is the filter of a list without scripts: a GET to the list
// with the filter and the fields of the query the list keeps (task, sort,
// tag). With scripts the client sifts what it loaded itself.
func (s *Server) listFilter(base string, q url.Values, filter string) *view.Form {
	var rows []view.FormRow
	for _, name := range []string{"task", "sort", "tag"} {
		if v := q.Get(name); v != "" {
			rows = append(rows, view.FormRow{Name: name, Kind: view.InputHidden, Value: text(v)})
		}
	}
	rows = append(rows, view.FormRow{Name: "filter", Label: "Filter", Kind: view.InputText, Value: text(filter)})
	f := s.plainForm(base, "GET", rows...)
	return &f
}

// filterContacts keeps the cards whose name, phone, address or organisation
// holds the filter, as the client sifts them with scripts.
func filterContacts(items []listed, filter string) []listed {
	needle := strings.ToLower(strings.TrimSpace(filter))
	if needle == "" {
		return items
	}
	var out []listed
	for _, it := range items {
		hay := []string{contactName(it.pick("FN")), firstValue(it.obj, "TEL"), firstValue(it.obj, "EMAIL"), structured(firstValue(it.obj, "ORG"))}
		for _, o := range it.also {
			hay = append(hay, firstValue(o.obj, "TEL"), firstValue(o.obj, "EMAIL"), structured(firstValue(o.obj, "ORG")))
		}
		if strings.Contains(strings.ToLower(strings.Join(hay, "\n")), needle) {
			out = append(out, it)
		}
	}
	return out
}

// openInPanel answers ?open={the row's address}: the row is on, and the card
// stands in the panel, read (s-contacts-1). Close is the list without it.
func (s *Server) openInPanel(r *http.Request, sess *session.Session, base string, items []listed, groups []view.ContactGroup) *view.Details {
	open, _ := r.Context().Value(openKey{}).(string)
	if open == "" {
		return nil
	}
	for gi := range groups {
		for ri := range groups[gi].Rows {
			row := &groups[gi].Rows[ri]
			if row.Href != open {
				continue
			}
			for _, it := range items {
				if card.UID(it.obj) != row.UID || s.route(it.from.recordPath("contacts", row.UID)) != open {
					continue
				}
				c, ok := s.collectionOf(sess, recordKinds["contacts"], it.from.conn.ID, it.from.col.Path)
				if !ok {
					return nil
				}
				row.On = true
				d := s.recordDetails(c, row.UID, it.obj)
				d.Close = s.route(base)
				if r.URL.RawQuery != "" {
					d.Close += "?" + r.URL.RawQuery
				}
				return &d
			}
		}
	}
	return nil
}

// countGroups puts each group's count by its rubric (s-contacts-1): the
// client does not count (C-6.1).
func countGroups(groups []view.ContactGroup) {
	for i := range groups {
		groups[i].Count = len(groups[i].Rows)
	}
}

// combinedSubtitle is the line under a combined book's title: how many of
// the books are ticked, on how many accounts, and — once the poll is over —
// how many cards (s-contacts-1).
func combinedSubtitle(rail view.Rail, spec kindSpec, done bool) string {
	live, on := 0, 0
	accounts := map[string]bool{}
	var total *int
	for i, sec := range rail.Sections {
		for _, row := range sec.Rows {
			if i == 0 && sec.AccountID == "" && sec.Account == "" {
				total = row.Count // the combined row
				continue
			}
			if row.State == view.CollectionGone {
				continue
			}
			live++
			if row.Tick == view.TickOn {
				on++
				accounts[string(sec.Account)] = true
			}
		}
	}
	parts := []string{fmt.Sprintf("%d of %s", on, plural(live, collectionNouns[spec.kind])), plural(len(accounts), "account")}
	if done && total != nil {
		parts = append(parts, plural(*total, recordNouns[spec.kind]))
	}
	return strings.Join(parts, " · ")
}

// bookSubtitle is the line under one book's title: its connection and how
// many cards (s-io-1), or, for a book the server gave no write to, that
// (states-4).
func bookSubtitle(c *collectionCtx, cards int, read bool) string {
	switch {
	case c.col.ReadOnly:
		return connLabel(c.conn) + " · the server granted no write access"
	case read:
		return connLabel(c.conn) + " · " + plural(cards, recordNouns[c.spec.kind])
	}
	return connLabel(c.conn)
}

// bookEmpty is what an empty book says: that it is empty, and the way to
// fill it (U-6.0) — none in a book that cannot be written.
func bookEmpty(c *collectionCtx, actions []view.Action) *view.Notice {
	n := &view.Notice{Role: view.NoticeInfo, Title: "This book is empty", Actions: []view.Action{}}
	if c.col.ReadOnly {
		n.Body = "The server granted no write access: nothing can be added here."
		return n
	}
	n.Body = "Add a contact, or import a file of them."
	for _, a := range actions {
		if a.ID == "record.new" || a.ID == "collection.import" {
			a.Arrow = nil
			n.Actions = append(n.Actions, a)
		}
	}
	return n
}

// nothingTicked is what a combined book says with no book ticked: why it is
// empty, and a button that ticks every one (U-6.0, S-14).
func (s *Server) nothingTicked(spec kindSpec) *view.Notice {
	root := "/app/" + spec.segment
	return &view.Notice{Role: view.NoticeInfo, Title: text("No " + collectionNouns[spec.kind] + "s are ticked"),
		Body: text("Tick the " + collectionNouns[spec.kind] + "s to show in the rail, or all of them at once."),
		Actions: []view.Action{{ID: "rail.tick.all", Label: text("Tick every " + collectionNouns[spec.kind]), Method: "POST", Enabled: true,
			Href: s.postQuery("/app/sources/"+spec.segment, url.Values{"toggle": {s.route(root)}, "back": {root}})}}}
}

// whereTo is New's arrow in a combined view (s-contacts-9): the ticked
// collections of the view, each a way to a new record there; the one the
// last record was made in is on and «last used», and the button writes
// there; a read-only one is shown closed with its reason («ro»).
func (s *Server) whereTo(blob sealed.Blob, spec kindSpec, button *view.Action) {
	last, remembered := blob.LastFor(spec.kind)
	var items []view.MenuItem
	for _, col := range blob.Collections {
		conn, ok := blob.Connection(col.AccountID)
		if !ok || conn.Feed || col.Gone || !col.Has(spec.kind) || !isTicked(blob, spec.kind, col.AccountID, col.Path) {
			continue
		}
		it := view.MenuItem{Action: view.Action{ID: "record.new", Label: text(collectionLabel(col)), Method: "GET",
			Href: s.route(origin{conn: conn, col: col}.listPath(spec.segment) + "/new"), Enabled: !col.ReadOnly},
			Colour: catalogColour(col), ReadOnly: col.ReadOnly}
		if col.ReadOnly {
			it.Reason = "Read-only"
		}
		if remembered && last.AccountID == col.AccountID && last.Path == col.Path && !col.ReadOnly {
			it.On, it.Aside = true, "last used"
			button.Href = it.Href
		}
		items = append(items, it)
	}
	button.Arrow = items
}

// compared says whether the record on its server is the version the list of
// this session read (s-contacts-6, «Read 2 min ago · not changed on
// server»): the ETag just read against the one the index holds. Nothing is
// said when the list was not read.
func (s *Server) compared(c *collectionCtx, uid string, obj *model.Object) view.Text {
	for _, e := range c.index() {
		if e.UID != uid {
			continue
		}
		if e.ETag == obj.ETag {
			return "not changed on server"
		}
		return "changed on server since"
	}
	return ""
}
