package web

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/emersion/go-ical"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/discover"
	"carrel.dev/carrel/server/internal/fanout"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// Events and tasks name a person by address — ORGANIZER, ATTENDEE, SENT-BY,
// DELEGATED-FROM, DELEGATED-TO — and the timeline finds them by the addresses
// of the card (S-15.4). Replace an address on the card and every record that
// named the old one drops out of the person's timeline. Unlike RELATED-TO
// (relink.go) this is not mended silently: an attendee changed is an act in
// somebody's schedule, and a server that schedules may send letters for it.
// So the save asks (owner's decision, 24.09.2026), and says beforehand what
// rewriting can set off (s-contacts-15):
//
//   - which address becomes which: one removed and one added is that pair;
//     otherwise each removed address goes to the card's default one. Only
//     removals ask nothing;
//   - how many records sit on servers that schedule, and on which servers it
//     is not known;
//   - how many meetings someone else organised — the organiser's server may
//     refuse the change and its next update may undo it;
//   - that past meetings will show the new address;
//   - what cannot change: records in read-only collections.
//
// The records are looked for where relink looks: the ticked collections.
// The scan is kept in the session as a plan, each record with the version it
// was read at; the answer writes that version or names why it did not.

// rewritePlan is one question and, once scanned, what answering it writes.
type rewritePlan struct {
	Token   string
	Account string // the card
	Col     string
	UID     string
	Name    string
	Pairs   []rewritePair
	// Card is every address the card had before the save and has after
	// it: a meeting organised at one of them is the person's own.
	Card []string
	// Now is the card's addresses as saved: what each old one may be
	// rewritten to (s-contacts-29).
	Now     []string
	Scanned bool
	Records []rewriteRecord
	Unread  []rewriteUnread
}

type rewritePair struct {
	From string // as the card held it
	To   string // the one offered; "" — «Do not rewrite»
}

// rewriteRecord is one event or task that names an old address.
type rewriteRecord struct {
	Account    string
	Col        string
	Path       string
	ETag       string
	Title      string
	Where      string // the collection's label
	Server     string // the connection's label
	ReadOnly   bool
	Scheduling string    // the connection's: discover.Scheduling…
	Others     bool      // organised by neither the person on the card nor the one who saves
	Past       bool      // every occurrence behind
	Names      []string  // the old addresses it names, lower-cased
	Href       string    // the record's page
	Start      time.Time // when it is, for the lists of the groups
	Colour     string    // the collection's mark
}

// rewriteUnread is a ticked collection that could not be read.
type rewriteUnread struct {
	Label string
	Why   string
}

func (p *rewritePlan) old() map[string]string {
	out := make(map[string]string, len(p.Pairs))
	for _, pr := range p.Pairs {
		out[strings.ToLower(pr.From)] = pr.To
	}
	return out
}

// rewriteAnswer is what the sheet's form chose: the address each old one
// goes to — "" leaves it — and the groups ticked (s-contacts-29).
type rewriteAnswer struct {
	to     map[string]string // old address, lower-cased → new one or ""
	groups map[string]bool
}

// defaultAnswer is the sheet as offered: each pair's address, every group
// ticked.
func (p *rewritePlan) defaultAnswer() rewriteAnswer {
	a := rewriteAnswer{to: p.old(), groups: map[string]bool{}}
	for _, g := range p.groups() {
		a.groups[g.value] = true
	}
	return a
}

// rewriteGroupOf is one group of risk (s-contacts-29) and its records.
type rewriteGroupOf struct {
	value, label, note string
	records            []int // indexes into the plan's records
}

// groups are the groups of risk of the records that can be written, in the
// sheet's order: on servers that schedule, on each server that did not say
// whether it does, organised by others, past.
func (p *rewritePlan) groups() []rewriteGroupOf {
	var scheduling, others, past rewriteGroupOf
	scheduling = rewriteGroupOf{value: "scheduling", label: "On servers that schedule",
		note: "The server may send a cancellation to the old address, an invitation to the new one and an update to the other attendees."}
	others = rewriteGroupOf{value: "others", label: "Meetings organised by others",
		note: "The organiser’s server may refuse the change, and its next update may undo it."}
	past = rewriteGroupOf{value: "past", label: "Past meetings",
		note: "They will show the new address, as if the person had been invited at it."}
	var unknown []rewriteGroupOf
	at := map[string]int{}
	for i, r := range p.Records {
		if r.ReadOnly {
			continue
		}
		switch r.Scheduling {
		case discover.SchedulingYes:
			scheduling.records = append(scheduling.records, i)
		case discover.SchedulingUnknown:
			j, ok := at[r.Server]
			if !ok {
				j = len(unknown)
				at[r.Server] = j
				unknown = append(unknown, rewriteGroupOf{value: "unknown." + strconv.Itoa(j+1), label: "On " + r.Server,
					note: "The server did not say whether it schedules."})
			}
			unknown[j].records = append(unknown[j].records, i)
		}
		if r.Others {
			others.records = append(others.records, i)
		}
		if r.Past {
			past.records = append(past.records, i)
		}
	}
	var out []rewriteGroupOf
	for _, g := range append(append([]rewriteGroupOf{scheduling}, unknown...), others, past) {
		if len(g.records) > 0 {
			out = append(out, g)
		}
	}
	return out
}

// inGroups are the groups of a record by its index.
func inGroups(groups []rewriteGroupOf, i int) []string {
	var out []string
	for _, g := range groups {
		if slices.Contains(g.records, i) {
			out = append(out, g.value)
		}
	}
	return out
}

// written tells whether the answer rewrites the record: it names an old
// address the answer sends somewhere, and every group it stands in is
// ticked — a record in two groups is rewritten only when both are
// (s-contacts-29).
func (p *rewritePlan) written(a rewriteAnswer, groups []rewriteGroupOf, i int) bool {
	r := p.Records[i]
	if r.ReadOnly || !slices.ContainsFunc(r.Names, func(n string) bool { return a.to[n] != "" }) {
		return false
	}
	for _, g := range inGroups(groups, i) {
		if !a.groups[g] {
			return false
		}
	}
	return true
}

// count is how many records the answer rewrites.
func (p *rewritePlan) count(a rewriteAnswer) int {
	groups := p.groups()
	n := 0
	for i := range p.Records {
		if p.written(a, groups, i) {
			n++
		}
	}
	return n
}

func (p *rewritePlan) writable() int {
	n := 0
	for _, r := range p.Records {
		if !r.ReadOnly {
			n++
		}
	}
	return n
}

// keptGroup is the read-only records of one collection.
type keptGroup struct {
	where string
	n     int
}

// kept are the read-only records by collection, in the order the scan met
// them.
func (p *rewritePlan) kept() []keptGroup {
	var out []keptGroup
	at := map[string]int{}
	for _, r := range p.Records {
		if !r.ReadOnly {
			continue
		}
		key := r.Account + " " + r.Col
		i, ok := at[key]
		if !ok {
			i = len(out)
			at[key] = i
			out = append(out, keptGroup{where: r.Where})
		}
		out[i].n++
	}
	return out
}

func loadRewritePlan(sess *session.Session, token string) (*rewritePlan, bool) {
	data := sess.Draft(session.DraftRewrite)
	if data == nil || token == "" {
		return nil, false
	}
	var p rewritePlan
	if gob.NewDecoder(bytes.NewReader(data)).Decode(&p) != nil || p.Token != token {
		return nil, false
	}
	return &p, true
}

func saveRewritePlan(sess *session.Session, p *rewritePlan) {
	var buf bytes.Buffer
	if gob.NewEncoder(&buf).Encode(p) == nil {
		sess.SetDraft(session.DraftRewrite, buf.Bytes())
	}
}

func dropRewritePlan(sess *session.Session, token string) {
	if _, ok := loadRewritePlan(sess, token); ok {
		sess.SetDraft(session.DraftRewrite, nil)
	}
}

// replacedAddresses compares the addresses a card had before a save with the
// card as saved (decision of 24.09.2026). Removed only — nothing. One
// removed and one added — that pair. Otherwise each removed address is
// offered the one typed in its line of the form — typed are the form's rows
// of addresses, in the card's order, the old ones first —, else nothing:
// «Do not rewrite» (the owner's decision of 12.10.2026, А4; until then each
// went to the card's default address). Case does not make an address
// another one.
func replacedAddresses(was, typed []string, after *model.Object) []rewritePair {
	now := cardEmails(after)
	in := func(list []string, a string) bool {
		return slices.ContainsFunc(list, func(b string) bool { return strings.EqualFold(a, b) })
	}
	var removed, added []string
	for _, a := range was {
		if !in(now, a) && !in(removed, a) {
			removed = append(removed, a)
		}
	}
	for _, a := range now {
		if !in(was, a) && !in(added, a) {
			added = append(added, a)
		}
	}
	if len(removed) == 0 || len(added) == 0 {
		return nil // removals only: nothing was replaced
	}
	if len(removed) == 1 && len(added) == 1 {
		return []rewritePair{{From: removed[0], To: added[0]}}
	}
	var out []rewritePair
	for _, a := range removed {
		to := ""
		if i := slices.IndexFunc(was, func(b string) bool { return strings.EqualFold(a, b) }); i < len(typed) {
			if t := strings.TrimSpace(typed[i]); t != "" && in(added, t) {
				to = t
			}
		}
		out = append(out, rewritePair{From: a, To: to})
	}
	slices.SortFunc(out, func(a, b rewritePair) int { return strings.Compare(strings.ToLower(a.From), strings.ToLower(b.From)) })
	return out
}

func cardEmails(obj *model.Object) []string {
	if obj == nil {
		return nil
	}
	var out []string
	for _, v := range obj.Props()["EMAIL"] {
		if a := strings.TrimSpace(v.Text); a != "" {
			out = append(out, a)
		}
	}
	return out
}

// defaultEmail is the card's preferred address — PREF=1 (vCard 4), else
// TYPE=pref (vCard 3) — else its first.
func defaultEmail(obj *model.Object) string {
	vals := obj.Props()["EMAIL"]
	for _, v := range vals {
		if strings.TrimSpace(v.Params["PREF"]) == "1" && strings.TrimSpace(v.Text) != "" {
			return strings.TrimSpace(v.Text)
		}
	}
	for _, v := range vals {
		for _, t := range model.ParamValues("TYPE", v.Params["TYPE"]) {
			if strings.EqualFold(strings.TrimSpace(t), "pref") && strings.TrimSpace(v.Text) != "" {
				return strings.TrimSpace(v.Text)
			}
		}
	}
	if all := cardEmails(obj); len(all) > 0 {
		return all[0]
	}
	return ""
}

// offerRewrite keeps a new plan for a saved card, was being its addresses
// before the edit. It answers where the save goes next — the sheet, or ""
// when no address was replaced — and the plan's token.
func (s *Server) offerRewrite(c *collectionCtx, uid string, was, typed []string, after *model.Object) (string, string) {
	pairs := replacedAddresses(was, typed, after)
	if len(pairs) == 0 {
		return "", ""
	}
	token, err := user.NewPlanID()
	if err != nil {
		return "", ""
	}
	saveRewritePlan(c.sess, &rewritePlan{Token: token, Account: c.conn.ID, Col: c.col.Path, UID: uid, Name: contactName(after),
		Pairs: pairs, Card: append(append([]string(nil), was...), cardEmails(after)...), Now: cardEmails(after)})
	return s.rewriteAddress(c, uid, token, ""), token
}

func (s *Server) rewriteAddress(c *collectionCtx, uid, token, op string) string {
	q := url.Values{"plan": {token}}
	if op != "" {
		q.Set("op", op)
	}
	return c.recordPath(uid) + "/rewrite?" + q.Encode()
}

// rewritePlanOf is the plan the address names, for this card; nil sends the
// person to the card.
func (s *Server) rewritePlanOf(w http.ResponseWriter, r *http.Request) (*collectionCtx, *rewritePlan, bool) {
	c, ok := s.openCollection(w, r, "contacts", true)
	if !ok {
		return nil, nil, false
	}
	uid := r.PathValue("uid")
	p, ok := loadRewritePlan(c.sess, r.URL.Query().Get("plan"))
	if !ok || p.Account != c.conn.ID || p.Col != c.col.Path || p.UID != uid {
		s.redirect(w, r, c.recordPath(uid))
		return nil, nil, false
	}
	return c, p, true
}

// getContactRewrite is the sheet (s-contacts-15). The first GET scans; the
// plan keeps what it found, so the sheet drawn again says the same.
func (s *Server) getContactRewrite(w http.ResponseWriter, r *http.Request) {
	c, p, ok := s.rewritePlanOf(w, r)
	if !ok {
		return
	}
	if !p.Scanned {
		s.scanRewrite(r.Context(), c.sess, p)
		saveRewritePlan(c.sess, p)
	}
	if len(p.Records) == 0 && len(p.Unread) == 0 {
		dropRewritePlan(c.sess, p.Token)
		s.redirect(w, r, c.recordPath(p.UID))
		return
	}
	sheet := s.rewriteSheet(c, p)
	if render.WantsFragment(r) && render.FragmentTarget(r) == render.TargetSheet {
		_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, sheet, nil)
		return
	}
	// The card's edit page under the sheet, the card as it was just saved
	// (s-contacts-15).
	sheet.Under = s.underOf(r, "", c.recordPath(p.UID)+"/edit")
	s.writePage(w, r, http.StatusOK, sheet)
}

// scanRewrite reads the events and tasks of the ticked collections whole and
// keeps each that names an old address, with how it stands (S-15.4). A
// collection that does not answer is named, not a failure of the sheet
// (S-21). Notes are not read: they name nobody by address.
func (s *Server) scanRewrite(ctx context.Context, sess *session.Session, p *rewritePlan) {
	old := p.old()
	mine := map[string]bool{}
	for _, pr := range p.Pairs {
		mine[strings.ToLower(pr.From)] = true
		mine[strings.ToLower(pr.To)] = true
	}
	for _, a := range p.Card {
		mine[strings.ToLower(a)] = true
	}
	now := s.now()
	seen := map[string]bool{}
	for _, c := range s.tickedCollections(sess, s.catalogOf(sess)) {
		where := collectionLabel(c.col)
		kinds := slices.DeleteFunc(slices.Clone(c.kinds), func(k view.Kind) bool { return k != view.KindEvent && k != view.KindTask })
		if len(kinds) == 0 {
			continue
		}
		if c.broken != nil {
			p.Unread = append(p.Unread, rewriteUnread{Label: where + " · " + connLabel(c.conn), Why: refusalReason(c.broken)})
			continue
		}
		for _, k := range kinds {
			items, err := c.records(ctx, k, fanout.Query{})
			if err != nil && !errors.Is(err, fanout.ErrIncomplete) {
				// Said once for the collection, whichever kind asked.
				p.Unread = append(p.Unread, rewriteUnread{Label: where + " · " + connLabel(c.conn), Why: refusalReason(err)})
				break
			}
			for _, it := range items {
				obj := it.Object
				if obj == nil || seen[c.conn.ID+"\x00"+obj.Path] {
					continue
				}
				names := namesOf(obj, old)
				if len(names) == 0 {
					continue
				}
				seen[c.conn.ID+"\x00"+obj.Path] = true
				organiser := strings.ToLower(cal.Organiser(obj).Email)
				p.Records = append(p.Records, rewriteRecord{
					Account: c.conn.ID, Col: c.col.Path, Path: obj.Path, ETag: obj.ETag,
					Title:      titleOr(cal.Text(obj, ical.PropSummary), "Untitled"),
					Where:      where,
					Server:     connLabel(c.conn),
					ReadOnly:   c.col.ReadOnly || c.feed != nil || c.cal == nil,
					Scheduling: c.conn.Scheduling,
					Others:     organiser != "" && !mine[organiser] && !ownAddress(c.conn, organiser),
					Past:       cal.Ended(obj, now),
					Names:      names,
					Href:       origin{conn: c.conn, col: c.col}.recordPath(segmentOf(k), cal.UID(obj)),
					Start:      recordStart(obj),
					Colour:     string(catalogColour(c.col)),
				})
			}
		}
	}
	p.Scanned = true
}

// recordStart is when an event or a task is: its start, else its due date.
func recordStart(obj *model.Object) time.Time {
	if tm, err := cal.TimingOf(obj, time.UTC); err == nil && !tm.Start.IsZero() {
		return tm.Start
	}
	if due, err := cal.DueOf(obj, time.UTC); err == nil {
		return due.Start
	}
	return time.Time{}
}

// namesOf are the old addresses a record names anywhere a person is named.
func namesOf(obj *model.Object, old map[string]string) []string {
	var out []string
	add := func(a string) {
		a = strings.ToLower(strings.TrimSpace(a))
		if _, ok := old[a]; ok && !slices.Contains(out, a) {
			out = append(out, a)
		}
	}
	for _, person := range cal.People(obj) {
		add(person.Email)
		add(person.SentBy)
		for _, d := range person.DelegatedFrom {
			add(d)
		}
		for _, d := range person.DelegatedTo {
			add(d)
		}
	}
	return out
}

// ownAddress is an address of the person on the connection: one its principal
// named, or the login when the login is an address.
func ownAddress(conn sealed.Connection, a string) bool {
	if slices.Contains(conn.Addresses, a) {
		return true
	}
	return strings.Contains(conn.Login, "@") && strings.EqualFold(conn.Login, a)
}

// rewriteSheet draws the plan as the frames do (s-contacts-15, -29). The
// server does not know the person's gender: where the frame says «her» and
// «she» the sheet says «their» and «the person» (to-typecase 25). Each old
// address has its choice (А4), each group of risk its tick (А5), and the
// button counts what the form as offered rewrites; the script counts again
// at every change (Tally, from-typecase note 65).
func (s *Server) rewriteSheet(c *collectionCtx, p *rewritePlan) view.RewriteSheet {
	sheet := view.RewriteSheet{
		Title:   "Rewrite the address in their records?",
		Subject: text(titleOr(p.Name, "Contact") + " · saved"),
		Pairs:   []view.RewritePair{},
		Groups:  []view.RewriteGroup{},
		Tally:   []string{},
		// Only the ticked collections were read, as relink reads them
		// (owner's decision, s-contacts-19).
		Note:     "Collections that are not ticked were not looked in.",
		Count:    "Rewrite {n} records",
		CountOne: "Rewrite 1 record",
	}
	if len(p.Pairs) > 1 {
		sheet.Title = "Rewrite the addresses in their records?"
	}
	field := map[string]string{} // old address, lower-cased → its choice's name
	for k, pr := range p.Pairs {
		name := "to." + strconv.Itoa(k+1)
		field[strings.ToLower(pr.From)] = name
		n, writable := 0, 0
		for _, r := range p.Records {
			if slices.Contains(r.Names, strings.ToLower(pr.From)) {
				n++
				if !r.ReadOnly {
					writable++
				}
			}
		}
		// How many name it, and that they keep it when it is not to be
		// rewritten (s-contacts-15, -19); with nothing to write, no choice
		// either (s-contacts-20).
		note := plural(n, "record") + " name it"
		if n == 1 {
			note = "1 record names it"
		}
		var choice view.FormRow
		if writable > 0 {
			switch {
			case pr.To == "" && n == 1:
				note += "; it keeps it"
			case pr.To == "":
				note += "; they keep it"
			}
			choice = view.FormRow{Name: name, Label: text("Rewrite " + pr.From + " to"), Kind: view.InputSelect, Value: text(pr.To)}
			for _, a := range p.Now {
				choice.Options = append(choice.Options, view.Option{Value: a, Label: text(a), Selected: strings.EqualFold(a, pr.To)})
			}
			choice.Options = append(choice.Options, view.Option{Value: "", Label: "Do not rewrite", Selected: pr.To == "", Separator: true})
		}
		sheet.Pairs = append(sheet.Pairs, view.RewritePair{From: text(pr.From), To: text(pr.To), Note: text(note), Choice: choice})
	}
	groups := p.groups()
	answer := p.defaultAnswer()
	for _, g := range groups {
		rg := view.RewriteGroup{Name: "group", Value: g.value, Label: text(g.label), On: answer.groups[g.value],
			Count: text(plural(len(g.records), "record")), Note: text(g.note)}
		// The group's records by their day (Typecase's note 53) — the past
		// ones the latest first, as s-contacts-15 lists them —, then by
		// title: the order the collections were read in is no order.
		byDay := slices.Clone(g.records)
		slices.SortStableFunc(byDay, func(a, b int) int {
			if c := p.Records[a].Start.Compare(p.Records[b].Start); c != 0 {
				if g.value == "past" {
					return -c
				}
				return c
			}
			return strings.Compare(p.Records[a].Title, p.Records[b].Title)
		})
		for _, i := range byDay {
			r := p.Records[i]
			row := view.Field{Label: text(r.Title), Colour: view.Colour(r.Colour), Href: s.route(r.Href)}
			if !r.Start.IsZero() {
				row.Value = text(c.tf.shortDay(r.Start))
			}
			rg.Records = append(rg.Records, row)
		}
		sheet.Groups = append(sheet.Groups, rg)
	}
	// A record in two groups is rewritten only when both are ticked: the
	// sheet names one such (s-contacts-29) — only when a group of it is not
	// ticked and it stays as it is; with every group ticked, as the sheet
	// opens, there is nothing to say (Typecase's note 74, 185 p. 3).
	for i, r := range p.Records {
		in := inGroups(groups, i)
		unticked := slices.ContainsFunc(in, func(v string) bool { return !answer.groups[v] })
		if len(in) > 1 && !r.ReadOnly && unticked {
			var labels []string
			for _, v := range in {
				for _, g := range groups {
					if g.value == v {
						labels = append(labels, "«"+g.label+"»")
					}
				}
			}
			sheet.Both = text("A record that stands in two groups is rewritten only when both are ticked: «" + r.Title + "» stands in both " +
				strings.Join(labels, " and ") + ".")
			break
		}
	}
	for i, r := range p.Records {
		if r.ReadOnly {
			continue
		}
		var names []string
		for _, n := range r.Names {
			if f := field[n]; f != "" {
				names = append(names, f)
			}
		}
		cond := strings.Join(names, "|")
		for _, g := range inGroups(groups, i) {
			cond += " group=" + g
		}
		sheet.Tally = append(sheet.Tally, cond)
	}
	sheet.KeptNote = text(strings.Join(p.keptSentences(), " "))
	action := s.rewriteAddress(c, p.UID, p.Token, "")
	sheet.Form = s.plainForm(action, "POST", csrfField(c.sess), view.FormRow{Name: "groups", Kind: view.InputHidden, Value: "1"})
	n := p.count(answer)
	// The sheet's button: it sends the choices and the ticks of its form
	// (Лс32; s-contacts-19, -20, -29).
	confirm := view.Action{ID: "contact.rewrite.confirm", Label: text("Rewrite " + plural(n, "record")),
		Href: s.route(action), Method: "POST", Primary: true, Enabled: true, Submits: true}
	switch {
	case p.writable() == 0:
		confirm.Enabled, confirm.Href = false, ""
		confirm.Reason = "Nothing here can be rewritten." // the line above says what keeps it (s-contacts-20)
	case n == 0:
		confirm.Enabled, confirm.Href = false, ""
		confirm.Reason = "Nothing is chosen to be rewritten: choose an address, or tick a group."
	}
	sheet.Actions = []view.Action{confirm,
		{ID: "contact.rewrite.leave", Label: "Leave as is", Href: s.route(s.rewriteAddress(c, p.UID, p.Token, "leave")), Method: "POST", Enabled: true}}
	return sheet
}

// rewriteAnswerOf is the answer the sheet's form sent: each old address's
// choice — one of the card's addresses or «Do not rewrite» —, and the groups
// ticked. A form without them (one that posts the plan alone) answers as the
// sheet offered.
func (p *rewritePlan) rewriteAnswerOf(r *http.Request) rewriteAnswer {
	a := p.defaultAnswer()
	for k, pr := range p.Pairs {
		name := "to." + strconv.Itoa(k+1)
		if _, sent := r.Form[name]; !sent {
			continue
		}
		v := strings.TrimSpace(r.FormValue(name))
		if v != "" && !slices.ContainsFunc(p.Now, func(b string) bool { return strings.EqualFold(b, v) }) {
			continue // not an address of the card: the offer stands
		}
		a.to[strings.ToLower(pr.From)] = v
	}
	if r.FormValue("groups") != "" {
		ticked := r.Form["group"]
		for g := range a.groups {
			a.groups[g] = slices.Contains(ticked, g)
		}
	}
	return a
}

// postContactRewrite answers the sheet: «Leave as is» drops the plan;
// «Rewrite N records» writes what the form chose and says on the card what
// came of it (s-contacts-16).
func (s *Server) postContactRewrite(w http.ResponseWriter, r *http.Request) {
	c, p, ok := s.rewritePlanOf(w, r)
	if !ok {
		return
	}
	dropRewritePlan(c.sess, p.Token)
	if r.URL.Query().Get("op") == "leave" || !p.Scanned {
		s.redirect(w, r, c.recordPath(p.UID))
		return
	}
	r.FormValue("groups") // parses the form
	answer := p.rewriteAnswerOf(r)
	out := s.applyRewrite(r.Context(), c.sess, p, answer)
	s.users.Journal(c.sess.AccountID(), "contact.rewrite", fmt.Sprintf("%d of %d", out.written, p.count(answer)))
	card, err := c.find(r.Context(), p.UID)
	if err != nil {
		s.writePage(w, r, http.StatusOK, s.recordPage(c, p.UID, nil, out.notice(p, answer)))
		return
	}
	// The card as it is opened, its other copies with it (s-contacts-16).
	s.writePage(w, r, http.StatusOK, s.withLinked(r.Context(), c, p.UID, card, s.recordPage(c, p.UID, card, out.notice(p, answer))))
}

// rewriteOutcome is what answering the sheet did.
type rewriteOutcome struct {
	written int
	refused []string // sentences, one per record not written
	lost    []string // detail lines: written, but the server did not keep all
}

// keptSentences say what keeps the old address whatever is chosen — the
// read-only records and the collections that could not be read —, a
// sentence each: the sheet's line over its note (s-contacts-15) and the
// outcome's message say them alike.
func (p *rewritePlan) keptSentences() []string {
	var out []string
	for _, g := range p.kept() {
		if g.n == 1 {
			out = append(out, "1 record in "+g.where+" is read-only and keeps the old address.")
		} else {
			out = append(out, fmt.Sprintf("%d records in %s are read-only and keep the old address.", g.n, g.where))
		}
	}
	for _, u := range p.Unread {
		out = append(out, u.Label+" could not be read, and what it holds keeps the old address.")
	}
	return out
}

func (o rewriteOutcome) notice(p *rewritePlan, a rewriteAnswer) view.Notice {
	// Every record chosen and written counts them alone (s-contacts-21); a
	// shortfall says of how many.
	chosen := p.count(a)
	n := view.Notice{Role: view.NoticeInfo, Title: text("Rewritten in " + plural(o.written, "record") + ".")}
	if o.written != chosen {
		n.Title = text(fmt.Sprintf("Rewritten in %d of %s.", o.written, plural(chosen, "record")))
	}
	parts := append(append([]string(nil), o.refused...), p.keptSentences()...)
	n.Body = text(strings.Join(parts, " "))
	if len(o.refused) > 0 || len(o.lost) > 0 || len(p.Unread) > 0 {
		n.Role = view.NoticeAlert
	}
	if len(o.lost) > 0 {
		n.Details = &view.Diagnostic{Summary: "Not kept by the server"}
		for _, l := range o.lost {
			n.Details.Lines = append(n.Details.Lines, text(l))
		}
	}
	return n
}

// applyRewrite writes every record of the plan that can be written: read
// again, still the version the sheet was drawn from, rewritten in every
// component, put with If-Match (S-8) and read back (S-7.1).
func (s *Server) applyRewrite(ctx context.Context, sess *session.Session, p *rewritePlan, a rewriteAnswer) rewriteOutcome {
	var out rewriteOutcome
	blob := s.catalogOf(sess)
	// Only the addresses the answer sends somewhere are rewritten; the
	// records it leaves — a group not ticked, no address chosen — are not
	// read again (А4, А5).
	pairs := map[string]string{}
	for from, to := range a.to {
		if to != "" {
			pairs[from] = to
		}
	}
	groups := p.groups()
	sources := map[string]*collectionSource{}
	for i, rec := range p.Records {
		if !p.written(a, groups, i) {
			continue
		}
		name := "«" + rec.Title + "»"
		id := rec.Account + ":" + rec.Col
		src := sources[id]
		if src == nil {
			conn, hasConn := blob.Connection(rec.Account)
			col, hasCol := blob.Collection(rec.Account, rec.Col)
			if !hasConn || !hasCol || col.Gone {
				out.refused = append(out.refused, name+" was not written: its collection is no longer in the catalog.")
				continue
			}
			src = s.openSource(sess, id, conn, col)
			sources[id] = src
		}
		if src.broken != nil || src.cal == nil {
			out.refused = append(out.refused, name+" was not written: "+clause(src.broken, "its server could not be reached"))
			continue
		}
		obj, err := src.cal.Get(ctx, rec.Path)
		if err != nil {
			out.refused = append(out.refused, name+" was not written: it could not be read again — "+clause(err, ""))
			continue
		}
		if t := titleOr(cal.Text(obj, ical.PropSummary), "Untitled"); t != rec.Title {
			name = "«" + t + "»"
		}
		if strings.Trim(obj.ETag, `"`) != strings.Trim(rec.ETag, `"`) {
			out.refused = append(out.refused, name+" was not written: it changed after Carrel read it.")
			continue
		}
		sent, n, err := cal.RewriteAddresses(obj, pairs)
		if err != nil {
			out.refused = append(out.refused, name+" was not written: "+clause(err, ""))
			continue
		}
		if n == 0 {
			continue
		}
		sent.ETag = obj.ETag
		_, err = src.cal.Update(ctx, sent)
		switch {
		case dav.IsStatus(err, http.StatusPreconditionFailed):
			out.refused = append(out.refused, name+" was not written: it changed after Carrel read it.")
			continue
		case rec.Others && (dav.IsStatus(err, http.StatusForbidden) || dav.IsStatus(err, http.StatusConflict)):
			out.refused = append(out.refused, name+" was refused: its organiser’s server does not take changes to attendees.")
			continue
		case err != nil:
			out.refused = append(out.refused, name+" was refused: "+clause(err, ""))
			continue
		}
		sess.Cache().InvalidatePath(src.conn.ID, rec.Path)
		dropWholeReads(sess, src.conn.ID, src.col.Path)
		out.written++
		if got, err := src.cal.Get(ctx, rec.Path); err == nil {
			if loss := writeLoss(src.conn.ID, sent, got, sess.Losses(), false); loss != nil {
				out.lost = append(out.lost, name+" · "+rec.Where+": "+lossSaid(loss))
			}
		}
	}
	return out
}

// clause is why, as the end of a sentence: «the server answered 403
// Forbidden.»
func clause(err error, fallback string) string {
	why := fallback
	if err != nil {
		why = refusalReason(err)
	}
	return lowerClause(why)
}

// lowerClause is a sentence made the end of another one.
func lowerClause(why string) string {
	why = strings.TrimRight(strings.TrimSpace(why), ".")
	if r, n := utf8.DecodeRuneInString(why); n > 0 {
		why = string(unicode.ToLower(r)) + why[n:]
	}
	return why + "."
}
