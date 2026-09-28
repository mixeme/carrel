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
	"strings"
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
	Card    []string
	Scanned bool
	Records []rewriteRecord
	Unread  []rewriteUnread
}

type rewritePair struct {
	From string // as the card held it
	To   string
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
	Scheduling string   // the connection's: discover.Scheduling…
	Others     bool     // organised by neither the person on the card nor the one who saves
	Past       bool     // every occurrence behind
	Names      []string // the old addresses it names, lower-cased
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
// removed and one added — that pair. Otherwise each removed address goes to
// the default address of the card as saved. Case does not make an address
// another one.
func replacedAddresses(was []string, after *model.Object) []rewritePair {
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
	to := defaultEmail(after)
	var out []rewritePair
	for _, a := range removed {
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
func (s *Server) offerRewrite(c *collectionCtx, uid string, was []string, after *model.Object) (string, string) {
	pairs := replacedAddresses(was, after)
	if len(pairs) == 0 {
		return "", ""
	}
	token, err := user.NewPlanID()
	if err != nil {
		return "", ""
	}
	saveRewritePlan(c.sess, &rewritePlan{Token: token, Account: c.conn.ID, Col: c.col.Path, UID: uid, Name: contactName(after),
		Pairs: pairs, Card: append(append([]string(nil), was...), cardEmails(after)...)})
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
	if WantsFragment(r) && FragmentTarget(r) == TargetSheet {
		_ = s.render.Fragment(w, r, http.StatusOK, TargetSheet, sheet, nil)
		return
	}
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
				})
			}
		}
	}
	p.Scanned = true
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

// rewriteSheet draws the plan as the frame does (s-contacts-15). The server
// does not know the person's gender: where the frame says «her» and «she»
// the sheet says «their» and «the person» (to-typecase 25).
func (s *Server) rewriteSheet(c *collectionCtx, p *rewritePlan) view.RewriteSheet {
	sheet := view.RewriteSheet{
		Title:   "Rewrite the address in their records?",
		Subject: text(titleOr(p.Name, "Contact") + " · saved"),
		Pairs:   []view.RewritePair{},
		Effects: []view.Field{},
		Kept:    []view.Field{},
	}
	for _, pr := range p.Pairs {
		n := 0
		for _, r := range p.Records {
			if slices.Contains(r.Names, strings.ToLower(pr.From)) {
				n++
			}
		}
		note := "no events or tasks name the old one"
		switch {
		case n == 1:
			note = "1 event or task names the old one; it drops out of the person’s timeline"
		case n > 1:
			note = fmt.Sprintf("%d events and tasks name the old one; they drop out of the person’s timeline", n)
		}
		sheet.Pairs = append(sheet.Pairs, view.RewritePair{From: text(pr.From), To: text(pr.To), Note: text(note)})
	}
	var scheduling, others int
	past := false
	var unknown []string
	unknownN := map[string]int{}
	for _, r := range p.Records {
		if r.ReadOnly {
			continue
		}
		switch r.Scheduling {
		case discover.SchedulingYes:
			scheduling++
		case discover.SchedulingUnknown:
			if unknownN[r.Server] == 0 {
				unknown = append(unknown, r.Server)
			}
			unknownN[r.Server]++
		}
		if r.Others {
			others++
		}
		past = past || r.Past
	}
	if scheduling > 0 {
		sheet.Effects = append(sheet.Effects, view.Field{Label: text(plural(scheduling, "record") + " on servers that schedule"),
			Value: "the server may send a cancellation to the old address, an invitation to the new one and an update to the other attendees"})
	}
	for _, server := range unknown {
		sheet.Effects = append(sheet.Effects, view.Field{Label: text(plural(unknownN[server], "record") + " on " + server),
			Value: "the server did not say whether it schedules"})
	}
	if others > 0 {
		sheet.Effects = append(sheet.Effects, view.Field{Label: text(plural(others, "meeting") + " organised by others"),
			Value: "the organiser’s server may refuse the change, and its next update may undo it"})
	}
	if past {
		sheet.Effects = append(sheet.Effects, view.Field{Label: "Past meetings",
			Value: "will show the new address, as if the person had been invited at it"})
	}
	for _, g := range p.kept() {
		note := "read-only; they keep the old address"
		if g.n == 1 {
			note = "read-only; it keeps the old address"
		}
		sheet.Kept = append(sheet.Kept, view.Field{Label: text(plural(g.n, "record") + " in " + g.where), Value: text(note)})
	}
	for _, u := range p.Unread {
		sheet.Kept = append(sheet.Kept, view.Field{Label: text(u.Label), Value: text("could not be read: " + lowerClause(u.Why) + " What it holds keeps the old address.")})
	}
	action := s.rewriteAddress(c, p.UID, p.Token, "")
	sheet.Form = s.plainForm(action, "POST", csrfField(c.sess))
	confirm := view.Action{ID: "contact.rewrite.confirm", Label: text("Rewrite " + plural(p.writable(), "record")),
		Href: s.route(action), Method: "POST", Enabled: true}
	if p.writable() == 0 {
		confirm.Enabled, confirm.Href = false, ""
		confirm.Reason = "Nothing here can be rewritten: every record that names the old address is read-only or could not be read."
	}
	sheet.Actions = []view.Action{confirm,
		{ID: "contact.rewrite.leave", Label: "Leave as is", Href: s.route(s.rewriteAddress(c, p.UID, p.Token, "leave")), Method: "POST", Enabled: true}}
	return sheet
}

// postContactRewrite answers the sheet: «Leave as is» drops the plan;
// «Rewrite N records» writes it and says on the card what came of it
// (s-contacts-16).
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
	out := s.applyRewrite(r.Context(), c.sess, p)
	s.users.Journal(c.sess.AccountID(), "contact.rewrite", fmt.Sprintf("%d of %d", out.written, p.writable()))
	card, err := c.find(r.Context(), p.UID)
	if err != nil {
		card = nil
	}
	s.writePage(w, r, http.StatusOK, s.recordPage(c, p.UID, card, out.notice(p)))
}

// rewriteOutcome is what answering the sheet did.
type rewriteOutcome struct {
	written int
	refused []string // sentences, one per record not written
	lost    []string // detail lines: written, but the server did not keep all
}

func (o rewriteOutcome) notice(p *rewritePlan) view.Notice {
	n := view.Notice{Role: view.NoticeInfo, Title: text(fmt.Sprintf("Rewritten in %d of %d records.", o.written, p.writable()))}
	parts := append([]string(nil), o.refused...)
	for _, g := range p.kept() {
		if g.n == 1 {
			parts = append(parts, "1 record in "+g.where+" is read-only and keeps the old address.")
		} else {
			parts = append(parts, fmt.Sprintf("%d records in %s are read-only and keep the old address.", g.n, g.where))
		}
	}
	for _, u := range p.Unread {
		parts = append(parts, u.Label+" could not be read, and what it holds keeps the old address.")
	}
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
func (s *Server) applyRewrite(ctx context.Context, sess *session.Session, p *rewritePlan) rewriteOutcome {
	var out rewriteOutcome
	blob := s.catalogOf(sess)
	pairs := p.old()
	sources := map[string]*collectionSource{}
	for _, rec := range p.Records {
		if rec.ReadOnly {
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
			if loss := model.CompareAfterPut(src.conn.ID, sent, got, sess.Losses()); loss != nil {
				out.lost = append(out.lost, name+" · "+rec.Where+": "+strings.Join(loss.Properties, ", "))
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
