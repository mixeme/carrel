package web

import (
	"net/http"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"carrel.dev/carrel/server/internal/discover"
	"carrel.dev/carrel/server/internal/dupes"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// An attendee is an address; the address books only say who it is (Typecase
// task 77, the owner's decisions of 01–02.10.2026). The card draws the people
// of an event by their addresses, a known one by the mark of their book and a
// link (recordPeople). The organiser edits the list (s-agenda-3): the form
// posts the organiser hidden and each other a ticked box under one name, and
// a line typed in; the suggestions of that line are asked at
// /app/contacts/suggest (s-agenda-12, -13). An invited person sees the list
// and why it is not theirs to change (s-agenda-14). The books are the ticked
// ones whose lists the session has read: no server is asked, as for Related.

// attendeeName is the name the attendees' field posts under; the line typed
// in is attendeeName + ".add".
const attendeeName = "attendee"

// bookPerson is one address of a card in a ticked address book.
type bookPerson struct {
	name, email, href, book string
	colour                  view.Colour
}

// bookPeople are the addresses of the cards of the ticked address books the
// session has read, a row per address, in the catalog's order.
func (s *Server) bookPeople(sess *session.Session) []bookPerson {
	blob := s.catalogOf(sess)
	var out []bookPerson
	for _, col := range blob.Collections {
		conn, ok := blob.Connection(col.AccountID)
		if !ok || col.Gone || !col.Has(view.KindContact) || !isTicked(blob, view.KindContact, col.AccountID, col.Path) {
			continue
		}
		from := origin{conn: conn, col: col}
		for _, e := range indexedIn(sess, col) {
			if e.Kind != view.KindContact || e.UID == "" {
				continue
			}
			for _, mail := range e.Keys.Emails {
				out = append(out, bookPerson{name: e.Title, email: mail, book: collectionLabel(col), colour: catalogColour(col),
					href: s.route(from.recordPath(segmentOf(view.KindContact), e.UID))})
			}
		}
	}
	return out
}

// knownPeople are bookPeople by their address; an address on two cards is
// the first book's.
func (s *Server) knownPeople(sess *session.Session) map[string]knownPerson {
	out := map[string]knownPerson{}
	for _, p := range s.bookPeople(sess) {
		if _, seen := out[p.email]; !seen {
			out[p.email] = knownPerson{name: p.name, href: p.href, colour: p.colour}
		}
	}
	return out
}

// attendeeValues are the attendees of a record as the field holds them: the
// organiser first, then each other, «mailto:» and the address in lower case.
func attendeeValues(obj *model.Object) []string {
	var out []string
	add := func(a string) {
		if a = dupes.NormEmail(a); a != "" && !slices.Contains(out, "mailto:"+a) {
			out = append(out, "mailto:"+a)
		}
	}
	if obj == nil {
		return out
	}
	add(cal.Organiser(obj).Email)
	for _, a := range cal.Attendees(obj) {
		add(a.Email)
	}
	return out
}

// attendeeAddresses are the addresses of a field's values, without «mailto:»,
// in lower case, each once; what is not an address is left out.
func attendeeAddresses(vals []string) []string {
	var out []string
	for _, v := range vals {
		if a := dupes.NormEmail(v); a != "" && !slices.Contains(out, a) {
			out = append(out, a)
		}
	}
	return out
}

// invitedBy is who organises a record the person is only invited to: its
// organiser, when that is not one of the person's addresses on the
// connection; "" when the record is theirs or has no organiser.
func invitedBy(c *collectionCtx, obj *model.Object) (cal.Attendee, bool) {
	if obj == nil {
		return cal.Attendee{}, false
	}
	org := cal.Organiser(obj)
	a := dupes.NormEmail(org.Email)
	if a == "" || ownAddress(c.conn, a) {
		return cal.Attendee{}, false
	}
	return org, true
}

// myAddress is the person's own address on the connection, for the
// organiser of a record that gains its first attendees: the first the
// principal named, else the login when it is an address.
func myAddress(c *collectionCtx) string {
	for _, a := range c.conn.Addresses {
		if a = dupes.NormEmail(a); a != "" {
			return a
		}
	}
	return dupes.NormEmail(c.conn.Login)
}

// attendeeField is the field of an edit (s-agenda-3, -14, -15) for what the
// form holds: its values, the record's people for their names and answers,
// the books for the rest.
func (s *Server) attendeeField(c *collectionCtx, st editState) view.AttendeeField {
	known := s.knownPeople(c.sess)
	people := map[string]view.AttendeeRow{}
	if st.obj != nil {
		for _, p := range recordPeople(st.obj, known) {
			people[dupes.NormEmail(string(p.Email))] = p
		}
	}
	organiser := ""
	if st.obj != nil {
		organiser = dupes.NormEmail(cal.Organiser(st.obj).Email)
	}
	f := view.AttendeeField{Name: attendeeName, Rows: []view.AttendeeRow{}, Suggest: s.route("/app/contacts/suggest")}
	for _, a := range attendeeAddresses(st.values[attendeeName]) {
		row, ok := people[a]
		if !ok {
			row = view.AttendeeRow{Name: text(a), Email: text(a)}
			if p, found := known[a]; found {
				row.Name, row.Href, row.Colour = text(p.name), p.href, p.colour
			}
		}
		row.Value, row.Fixed = "mailto:"+a, a == organiser
		f.Rows = append(f.Rows, row)
	}
	if org, invited := invitedBy(c, st.obj); invited {
		who := org.Name
		if who == "" {
			who = dupes.NormEmail(org.Email)
		}
		f.ReadOnly, f.Hint = true, text("You are invited by "+who+": only the organiser changes the list.")
		return f
	}
	f.Hint = attendeeHint(c.conn.Scheduling)
	if c.col.ReadOnly {
		f.ReadOnly = true
		return f
	}
	f.Add = &view.FormRow{Name: attendeeName + ".add", Label: "Add attendee", Kind: view.InputText, Placeholder: "Name or address",
		Value: text(first(st.values[attendeeName+".add"])), Error: text(st.errors[attendeeName+".add"])}
	return f
}

// typedAttendee is the address the typed line names without scripts: an
// address as it is; else the one person of the ticked books whose name
// holds the words, if they have one address.
func (s *Server) typedAttendee(sess *session.Session, typed string) (string, string) {
	typed = strings.TrimSpace(typed)
	if a := dupes.NormEmail(typed); a != "" {
		if strings.ContainsAny(a, " \t;:,<>\"") {
			return "", "An attendee is an e-mail address."
		}
		return a, ""
	}
	var found []string
	needle := strings.ToLower(typed)
	for _, p := range s.bookPeople(sess) {
		if strings.Contains(strings.ToLower(p.name), needle) && !slices.Contains(found, p.email) {
			found = append(found, p.email)
		}
	}
	switch len(found) {
	case 0:
		return "", "Not in the ticked address books: type the address."
	case 1:
		return found[0], ""
	}
	return "", "More than one address in the ticked address books matches: type the address."
}

// attendeeOps are what a form asks of the attendees: the ones unticked go,
// the ones added come with their name and an answer not given yet (S-7: the
// others keep their values and parameters as they were). A record that gains
// its first attendees and has no organiser gets the person's own address.
// The organiser stays. A form that did not draw the field (no line typed
// in) only adds what it carries — a person a timeline creates an event for.
func (s *Server) attendeeOps(c *collectionCtx, obj *model.Object, have, want formValues, errs map[string]string) []model.PatchOp {
	if _, invited := invitedBy(c, obj); invited {
		return nil
	}
	held := attendeeAddresses(have[attendeeName])
	asked := attendeeAddresses(want[attendeeName])
	add, drawn := want[attendeeName+".add"]
	if typed := strings.TrimSpace(first(add)); drawn && typed != "" {
		a, why := s.typedAttendee(c.sess, typed)
		if why != "" {
			errs[attendeeName+".add"] = why
			return nil
		}
		if !slices.Contains(asked, a) {
			asked = append(asked, a)
		}
	}
	if !drawn {
		asked = append(slices.Clone(held), asked...)
		asked = attendeeAddresses(asked)
	}
	var props map[string][]model.RawValue
	organiser := ""
	if obj != nil {
		props = obj.Props()
		organiser = dupes.NormEmail(cal.Organiser(obj).Email)
	}
	if organiser != "" && !slices.Contains(asked, organiser) {
		asked = append([]string{organiser}, asked...)
	}
	a, b := slices.Clone(held), slices.Clone(asked)
	sort.Strings(a)
	sort.Strings(b)
	if slices.Equal(a, b) {
		return nil
	}
	cur := props["ATTENDEE"]
	known := s.knownPeople(c.sess)
	var texts []string
	var params []map[string]string
	kept := map[string]bool{}
	for _, v := range cur {
		// A line that is no mail address — a room by its «urn:uuid:», a
		// phone — is not in the field, and a second line of an address is
		// not either: neither is the form's to take away (S-7.3).
		addr := dupes.NormEmail(cal.CalAddress(v.Text))
		if addr != "" && !slices.Contains(asked, addr) {
			continue
		}
		if addr != "" {
			kept[addr] = true
		}
		texts, params = append(texts, v.Text), append(params, v.Params)
	}
	invited := len(kept) > 0
	for _, addr := range asked {
		if kept[addr] || addr == organiser {
			continue
		}
		p := map[string]string{"PARTSTAT": "NEEDS-ACTION", "RSVP": "TRUE"}
		if who := known[addr].name; who != "" {
			p["CN"] = who
		}
		texts, params = append(texts, "mailto:"+addr), append(params, p)
		invited = true
	}
	var ops []model.PatchOp
	switch {
	case len(texts) == 0 && len(cur) > 0:
		ops = append(ops, model.RemoveProp{Prop: "ATTENDEE"})
	case len(texts) > 0:
		ops = append(ops, model.SetValues{Prop: "ATTENDEE", Values: texts})
		for i, p := range params {
			for _, k := range sortedKeys(p) {
				ops = append(ops, model.SetParam{Prop: "ATTENDEE", Index: i, Param: k, Value: p[k]})
			}
		}
	}
	if organiser == "" && invited {
		if me := myAddress(c); me != "" {
			ops = append(ops, model.SetText{Prop: "ORGANIZER", Value: "mailto:" + me})
			snap := s.store.Snapshot()
			if u := snap.UserByID(c.sess.AccountID()); u != nil && strings.TrimSpace(u.DisplayName) != "" {
				ops = append(ops, model.SetParam{Prop: "ORGANIZER", Index: 0, Param: "CN", Value: strings.TrimSpace(u.DisplayName)})
			}
		}
	}
	return ops
}

// suggestMost is how many suggestions one answer gives.
const suggestMost = 8

// getContactsSuggest answers the line of the attendees' field (s-agenda-12,
// -13): the people of the ticked address books the session has read whose
// name or address holds what was typed, a row per address — those whose name
// begins with it first —, and an address no book holds offered as «Add».
func (s *Server) getContactsSuggest(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	s.writePage(w, r, http.StatusOK, s.suggestions(sess, r.URL.Query().Get("q")))
}

func (s *Server) suggestions(sess *session.Session, q string) view.SuggestList {
	out := view.SuggestList{Items: []view.SuggestItem{}}
	q = strings.TrimSpace(q)
	if q == "" {
		return out
	}
	needle := strings.ToLower(q)
	type hit struct {
		p      bookPerson
		at     int // where the name holds it, in runes; -1 — by the address
		prefix bool
	}
	var hits []hit
	addressed := false
	for _, p := range s.bookPeople(sess) {
		at := runeIndex(strings.ToLower(p.name), needle)
		switch {
		case at >= 0:
			hits = append(hits, hit{p: p, at: at, prefix: at == 0})
		case strings.Contains(p.email, needle):
			hits = append(hits, hit{p: p, at: -1})
		}
		if p.email == dupes.NormEmail(q) {
			addressed = true
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].prefix != hits[j].prefix {
			return hits[i].prefix
		}
		return hits[i].p.name < hits[j].p.name
	})
	if a := dupes.NormEmail(q); a != "" && !addressed {
		out.Items = append(out.Items, view.SuggestItem{Email: text(a), Value: "mailto:" + a, Add: true})
		out.Note = "Not in the ticked address books."
	}
	seen := map[string]bool{}
	for _, h := range hits {
		if len(out.Items) >= suggestMost || seen[h.p.email] {
			continue
		}
		seen[h.p.email] = true
		out.Items = append(out.Items, view.SuggestItem{Name: foundRuns(h.p.name, h.at, utf8.RuneCountInString(q)), Email: text(h.p.email),
			Book: text(h.p.book), Colour: h.p.colour, Value: "mailto:" + h.p.email})
	}
	return out
}

// runeIndex is where needle stands in s, in runes; -1 — nowhere.
func runeIndex(s, needle string) int {
	i := strings.Index(s, needle)
	if i < 0 {
		return -1
	}
	return utf8.RuneCountInString(s[:i])
}

// foundRuns is a name with what was found in it strong (s-agenda-12:
// «‹Ве›ра Лебедева», «Се‹ве›рин Ким»): n runes from at; at < 0 — found by
// the address, the name plain.
func foundRuns(name string, at, n int) view.Runs {
	rs := []rune(name)
	if at < 0 || at+n > len(rs) {
		return view.Runs{{Text: text(name)}}
	}
	var out view.Runs
	if at > 0 {
		out = append(out, view.Run{Text: text(string(rs[:at]))})
	}
	out = append(out, view.Run{Text: text(string(rs[at : at+n])), Mark: view.RunStrong})
	if at+n < len(rs) {
		out = append(out, view.Run{Text: text(string(rs[at+n:]))})
	}
	return out
}

// attendeeHint is the line under the attendees: what to type, and who sends
// the invitations — «the server», not its name (the owner, 04.11.2026;
// Typecase's note 168, s-agenda-3), in every answer it gave about them.
func attendeeHint(scheduling string) view.Text {
	switch scheduling {
	case discover.SchedulingYes:
		return "A name or an address. On save, the server sends the invitations, not Carrel."
	case discover.SchedulingNo:
		return "A name or an address. The server sends no invitations, and neither does Carrel."
	}
	return "A name or an address. The server did not say whether it sends invitations; Carrel does not."
}
