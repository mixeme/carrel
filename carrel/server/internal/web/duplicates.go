package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/dupes"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// S-16: duplicates are found on the fly among the records this session has
// already loaded — the index every list leaves in the cache — and never by
// asking a server. Three decisions: Link (the blob only, no server is
// written), Not duplicates (the group's signature in the blob) and Merge on
// the server, which writes the kept record with If-Match and only then
// deletes the others, each with its own.

// loadedRecord is one record of a loaded list with where it lives.
type loadedRecord struct {
	rec   dupes.Record
	entry indexEntry
	from  origin
}

// dupState is what the duplicates screen is built from: every loaded record,
// the groups found among them and the person's decisions.
type dupState struct {
	blob      sealed.Blob
	records   map[sealed.RecordRef]loadedRecord
	compared  int
	threshold int
	weights   dupes.Weights
	// read are the collections whose list this session read, by connection
	// and collection: compared, though they may hold nothing to compare
	read  map[dupes.CollectionRef]bool
	found []dupes.Group
	// apart is how many linked groups this read found come apart, their
	// other members gone (Б16): the screen says so.
	apart int
}

// dupThreshold is the person's number of points, else the instance's.
func (s *Server) dupThreshold(blob sealed.Blob) int {
	if blob.Duplicates.Threshold > 0 {
		return blob.Duplicates.Threshold
	}
	return s.cfg.DuplicatesThreshold
}

// dupWeights are the person's points per signal over the instance's.
func (s *Server) dupWeights(blob sealed.Blob) dupes.Weights {
	w := s.cfg.DuplicatesWeights
	for _, sig := range dupes.Signals {
		if n, ok := blob.Duplicates.Weights[string(sig)]; ok {
			dupes.SetWeight(&w, sig, n)
		}
	}
	return w
}

var signalLabels = map[dupes.Reason]string{
	dupes.SameUID: "Same UID", dupes.SameEmail: "Same email", dupes.SamePhone: "Same phone",
	dupes.SameName: "Same name", dupes.SameBirthday: "Same birthday",
	dupes.SameTitleWhen: "Same title and date (events, tasks, notes)",
}

// weightField is the field of a signal's weight in the menu Weights (s-dup-3,
// -7): «weight.phone»; the title with its date is «weight.title».
func weightField(sig dupes.Reason) string {
	if sig == dupes.SameTitleWhen {
		return "weight.title"
	}
	return "weight." + string(sig)
}

var dupKinds = []view.Kind{view.KindContact, view.KindEvent, view.KindTask, view.KindNote}

// loadedRecords reads the index of every catalog collection the session has
// loaded. present holds, per collection and kind, the UIDs of a list read
// whole: only such a list says a member is gone. The agenda reads a week, so
// an event missing from it may simply be in another week. A collection the
// catalog no longer holds, or holds as gone, has nothing in it. read are
// the collections whose list was read at all: compared, empty or not.
func (s *Server) loadedRecords(sess *session.Session, blob sealed.Blob) ([]loadedRecord, map[dupes.CollectionRef]map[string]bool, map[dupes.CollectionRef]bool) {
	var out []loadedRecord
	present := map[dupes.CollectionRef]map[string]bool{}
	read := map[dupes.CollectionRef]bool{} // by connection and collection: a list read, empty or not
	for _, col := range dupCollections(blob) {
		conn, ok := blob.Connection(col.AccountID)
		if !ok || col.Gone {
			continue
		}
		for _, k := range dupKinds {
			if !col.Has(k) {
				continue
			}
			spec := recordKinds[segmentOf(k)]
			data, ok := sess.Cache().PeekETagMap(conn.ID, col.Path+"\x00"+spec.component)
			if !ok {
				continue
			}
			var entries []indexEntry
			if json.Unmarshal(data, &entries) != nil {
				continue
			}
			read[dupes.CollectionRef{AccountID: conn.ID, Col: col.Path}] = true
			uids := map[string]bool{}
			for _, e := range entries {
				if e.Kind != k || e.UID == "" {
					continue
				}
				uids[e.UID] = true
				ref := sealed.RecordRef{AccountID: conn.ID, Col: col.Path, UID: e.UID}
				out = append(out, loadedRecord{
					rec:   dupes.Record{Ref: ref, Kind: k, Keys: e.Keys},
					entry: e,
					from:  origin{conn: conn, col: col},
				})
			}
			if k != view.KindEvent {
				present[dupes.CollectionRef{AccountID: conn.ID, Col: col.Path, Kind: k}] = uids
			}
		}
	}
	for _, g := range blob.Duplicates.Linked {
		for _, m := range g.Members {
			if col, ok := blob.Collection(m.AccountID, m.Col); !ok || col.Gone {
				present[dupes.CollectionRef{AccountID: m.AccountID, Col: m.Col, Kind: g.Kind}] = map[string]bool{}
			}
		}
	}
	return out, present, read
}

// uidHolders are the collections known to hold a record of kind k with
// this UID: a list the session has read, or a linked group's member. The
// move sheet closes them «already there» before the press (s-contacts-2);
// one not read may hold it too, and the press asks it (moveRecord).
func (s *Server) uidHolders(sess *session.Session, blob sealed.Blob, k view.Kind, uid string) map[dupes.CollectionRef]bool {
	out := map[dupes.CollectionRef]bool{}
	recs, _, _ := s.loadedRecords(sess, blob)
	for _, r := range recs {
		if r.rec.Kind == k && r.rec.Ref.UID == uid {
			out[dupes.CollectionRef{AccountID: r.rec.Ref.AccountID, Col: r.rec.Ref.Col}] = true
		}
	}
	for _, g := range blob.Duplicates.Linked {
		if g.Kind != k {
			continue
		}
		for _, m := range g.Members {
			if m.UID == uid {
				out[dupes.CollectionRef{AccountID: m.AccountID, Col: m.Col}] = true
			}
		}
	}
	return out
}

// readDuplicates finds the groups among what is loaded. A linked member the
// loaded lists say is gone leaves its group silently, and the blob is written
// without a journal line (S-16).
func (s *Server) readDuplicates(sess *session.Session, prune bool) dupState {
	blob := s.catalogOf(sess)
	loaded, present, read := s.loadedRecords(sess, blob)
	apart := 0
	if prune {
		if kept, changed := dupes.Prune(blob.Duplicates.Linked, present); changed {
			apart = len(blob.Duplicates.Linked) - len(kept)
			key, _ := sess.DataKey()
			err := s.users.EditDuplicates(sess.AccountID(), key, "", "", func(d *sealed.Duplicates) error {
				d.Linked, _ = dupes.Prune(d.Linked, present)
				return nil
			})
			if err == nil {
				blob.Duplicates.Linked = kept
			}
		}
	}
	st := dupState{blob: blob, records: map[sealed.RecordRef]loadedRecord{}, threshold: s.dupThreshold(blob), apart: apart, read: read}
	recs := make([]dupes.Record, 0, len(loaded))
	for _, l := range loaded {
		if _, seen := st.records[l.rec.Ref]; seen {
			continue
		}
		st.records[l.rec.Ref] = l
		recs = append(recs, l.rec)
	}
	st.compared = len(recs)
	var notDupes [][]sealed.RecordRef
	for _, ig := range blob.Duplicates.Ignored {
		notDupes = append(notDupes, ig.Members)
	}
	st.weights = s.dupWeights(blob)
	st.found = dupes.Find(recs, st.weights, st.threshold, notDupes)
	return st
}

// candidates are the found groups still to decide: not held by one linked
// group and not marked "not duplicates".
func (st dupState) candidates() []dupes.Group {
	var out []dupes.Group
	for _, g := range st.found {
		if !dupes.Covered(st.blob.Duplicates.Linked, g) && !st.blob.Duplicates.IsIgnored(g.ID) {
			out = append(out, g)
		}
	}
	return out
}

// duplicateCount is the number beside Duplicates in the rail: the groups
// still to decide among what is loaded; none while nothing was compared —
// no number is not a zero (s-dup-6).
func (s *Server) duplicateCount(sess *session.Session) *int {
	st := s.readDuplicates(sess, false)
	if st.compared == 0 {
		return nil
	}
	n := len(st.candidates())
	return &n
}

// dupGroup is one group of the screen, whatever its decision.
type dupGroup struct {
	id       string
	kind     view.Kind
	score    int
	reasons  []dupes.Reason
	refs     []sealed.RecordRef
	decision view.DuplicateDecision
	// title is what the decision was called when it was made (sealed)
	title string
}

// groups are the groups of the screen: to decide first, then linked, then
// those marked "not duplicates" — shown so the mark can be taken back where
// it was made.
func (st dupState) groups() []dupGroup {
	var todo, linked, ignored []dupGroup
	for _, g := range st.found {
		dg := dupGroup{id: g.ID, kind: g.Kind, score: g.Score, reasons: g.Reasons, refs: g.Refs()}
		switch {
		case dupes.Covered(st.blob.Duplicates.Linked, g):
		case st.blob.Duplicates.IsIgnored(g.ID):
			dg.decision = view.DecisionIgnored
			for _, ig := range st.blob.Duplicates.Ignored {
				if ig.ID == g.ID {
					dg.title = ig.Title
				}
			}
			ignored = append(ignored, dg)
		default:
			todo = append(todo, dg)
		}
	}
	for _, l := range st.blob.Duplicates.Linked {
		dg := dupGroup{id: l.ID, kind: l.Kind, refs: l.Members, decision: view.DecisionLinked, title: l.Title}
		// Its points are its loaded records' (Assess): a group linked
		// whole matches as one even when one of its copies alone would
		// not — «Синк-ап по релизу» half an hour later in «Общий».
		var recs []dupes.Record
		for _, ref := range l.Members {
			if r, ok := st.records[ref]; ok {
				recs = append(recs, r.rec)
			}
		}
		if len(recs) > 1 {
			dg.score, dg.reasons = dupes.Assess(recs, st.weights)
		}
		linked = append(linked, dg)
	}
	// Linked groups by kind, then by name (s-dup-1): the order of the blob
	// is the order they were linked in, which says nothing.
	slices.SortStableFunc(linked, func(a, b dupGroup) int {
		if ka, kb := slices.Index(dupKinds, a.kind), slices.Index(dupKinds, b.kind); ka != kb {
			return ka - kb
		}
		return strings.Compare(st.titleOf(a), st.titleOf(b))
	})
	// A «not duplicates» is the person's decision whether or not its
	// records are loaded: scored when they are (s-dup-1, «60 points · same
	// phone»), named and taken back from here when not (s-dup-6). Find keeps
	// its members apart, so it is never found as a group itself.
	for _, ig := range st.blob.Duplicates.Ignored {
		if slices.ContainsFunc(ignored, func(g dupGroup) bool { return g.id == ig.ID }) {
			continue
		}
		dg := dupGroup{id: ig.ID, kind: ig.Kind, refs: ig.Members, decision: view.DecisionIgnored, title: ig.Title}
		var recs []dupes.Record
		for _, ref := range ig.Members {
			if l, ok := st.records[ref]; ok {
				recs = append(recs, l.rec)
			}
		}
		if len(recs) == len(ig.Members) {
			dg.score, dg.reasons = dupes.Assess(recs, st.weights)
		}
		ignored = append(ignored, dg)
	}
	out := append(append(todo, linked...), ignored...)
	for i := range out {
		out[i].refs = st.inCatalogOrder(out[i].refs)
	}
	return out
}

// inCatalogOrder is the members of a group in the order of the rail
// (railOrder).
func (st dupState) inCatalogOrder(refs []sealed.RecordRef) []sealed.RecordRef {
	return railOrder(st.blob, refs)
}

// railOrder is records in the order the rail shows their collections: by
// connection, then as the catalog holds the collections of it, then by UID
// (Typecase's note 125 p. 1) — the rows of a group's card, the variants of
// its questions, «Where … is kept», «The same person in other books» and
// «Where the merged record stays» read the same way every time. A path is
// not seen in the rail, and an account's ID is random: neither orders.
func railOrder(blob sealed.Blob, refs []sealed.RecordRef) []sealed.RecordRef {
	rank := map[string]int{}
	for i, c := range blob.Connections {
		rank[c.ID] = i
	}
	for i, f := range blob.Subscriptions {
		rank[f.ID] = len(blob.Connections) + i
	}
	at := map[[2]string]int{}
	for i, c := range blob.Collections {
		at[[2]string{c.AccountID, c.Path}] = i
	}
	out := slices.Clone(refs)
	slices.SortStableFunc(out, func(a, b sealed.RecordRef) int {
		if ra, rb := rank[a.AccountID], rank[b.AccountID]; ra != rb {
			return ra - rb
		}
		if ca, cb := at[[2]string{a.AccountID, a.Col}], at[[2]string{b.AccountID, b.Col}]; ca != cb {
			return ca - cb
		}
		return strings.Compare(a.UID, b.UID)
	})
	return out
}

// loaded reports whether a record of g is among what this session read.
func (st dupState) loaded(g dupGroup) bool {
	for _, ref := range g.refs {
		if _, ok := st.records[ref]; ok {
			return true
		}
	}
	return false
}

// titleOf is the name of a group: its loaded records' names, each once,
// «Анна Ковалёва · Алексей Смирнов»; a decision none of whose records is
// loaded, the name it was made under.
func (st dupState) titleOf(g dupGroup) string {
	var names []string
	for _, ref := range g.refs {
		if l, ok := st.records[ref]; ok && l.entry.Title != "" && !slices.Contains(names, l.entry.Title) {
			names = append(names, l.entry.Title)
		}
	}
	if len(names) == 0 {
		return g.title
	}
	return strings.Join(names, " · ")
}

// decidedView is a decision none of whose records is loaded (s-dup-6,
// «Decided earlier»; from-typecase note 117 p. 10): what was decided, where
// its records lie and under which UIDs, and its undoing — a button of its
// card's form, as a group's.
func (s *Server) decidedView(sess *session.Session, st dupState, g dupGroup) view.DecidedCard {
	out := view.DecidedCard{ID: g.id, Kind: g.kind, Title: text(st.titleOf(g)), Decision: g.decision, Note: "Not loaded in this session"}
	var where, uids []string
	for _, m := range st.groupMembers(g.refs) {
		if !slices.Contains(where, string(m.stamp)) {
			where = append(where, string(m.stamp))
		}
		if !slices.Contains(uids, m.ref.UID) {
			uids = append(uids, m.ref.UID)
		}
	}
	out.Where, out.UIDs = text(strings.Join(where, ", ")), text("UID "+strings.Join(uids, ", "))
	if g.decision == view.DecisionLinked {
		out.Badge = view.Badge{Label: text(fmt.Sprintf("linked ×%d", len(g.refs))), Role: view.BadgeLinked}
		out.Actions = []view.Action{decideAction("duplicates.unlink", "Unlink", "unlink")}
	} else {
		out.Badge = view.Badge{Label: "not duplicates", Role: view.BadgeNeutral}
		out.Actions = []view.Action{decideAction("duplicates.again", "Offer again", "again")}
	}
	out.Form = &view.Form{Action: s.route("/app/duplicates/decide"), Method: "POST", Sets: []view.FieldSet{{Rows: []view.FormRow{
		csrfField(sess),
		{Name: "group", Kind: view.InputHidden, Value: text(g.id)},
	}}}}
	return out
}

func (st dupState) group(id string) (dupGroup, bool) {
	for _, g := range st.groups() {
		if g.id == id {
			return g, true
		}
	}
	return dupGroup{}, false
}

func reasonText(k view.Kind, r dupes.Reason) string {
	switch r {
	case dupes.SameUID:
		return "same UID"
	case dupes.SameEmail:
		return "same email"
	case dupes.SamePhone:
		return "same phone"
	case dupes.SameName:
		return "same name"
	case dupes.SameBirthday:
		return "same birthday"
	}
	switch k {
	case view.KindEvent:
		return "same title, same start"
	case view.KindTask:
		return "same title, same due date"
	default:
		return "same title, same date"
	}
}

// groupHint is what a group means, said under its rows (s-dup-1, Дб6): an
// event under different UIDs in several calendars, which the agenda shows
// as many times, while it waits for a decision; a note copied into another
// notebook. A group marked «not duplicates», and one of contacts or tasks,
// says nothing.
func groupHint(g dupGroup) view.Text {
	if g.decision == view.DecisionIgnored || slices.Contains(g.reasons, dupes.SameUID) {
		return ""
	}
	n := len(g.refs)
	two := n == 2
	pick := func(a, b string) string {
		if two {
			return a
		}
		return b
	}
	switch g.kind {
	case view.KindEvent:
		if g.decision == view.DecisionLinked {
			return ""
		}
		return text(fmt.Sprintf("The same event in %s calendars, different UIDs. Agenda shows it %s; here you decide whether to keep %s or write one and delete the %s.",
			countWord(n), pick("twice", strconv.Itoa(n)+" times"), pick("both", "them all"), pick("other", "others")))
	case view.KindNote:
		return text(fmt.Sprintf("Same title and the same date: a note copied into %s. Linked, the %s are shown as one row; merging writes one and deletes the %s.",
			pick("a second notebook", "other notebooks"), countWord(n), pick("other", "others")))
	}
	return ""
}

// member is one row of a group: the record as its list left it, or — for a
// linked member whose list this session has not read — its address alone.
func (s *Server) member(st dupState, kind view.Kind, ref sealed.RecordRef) view.DuplicateMember {
	conn, _ := st.blob.Connection(ref.AccountID)
	col, ok := st.blob.Collection(ref.AccountID, ref.Col)
	if !ok {
		col = sealed.Collection{AccountID: ref.AccountID, Path: ref.Col}
	}
	o := origin{conn: conn, col: col}
	m := view.DuplicateMember{
		Href: s.route(o.recordPath(segmentOf(kind), ref.UID)), Stamp: o.stamp(), Colour: catalogColour(col),
		Title: text(ref.UID), Detail: "Not loaded in this session", ReadOnly: col.ReadOnly,
	}
	if l, ok := st.records[ref]; ok {
		e := l.entry
		m.Title, m.Detail = text(e.Title), text(e.Detail)
		switch kind {
		case view.KindContact:
			av := avatarFor(ref.UID, e.Title)
			if e.Row.Photo {
				av.Href = s.photoHref(o, ref.UID, e.ETag, photoThumb)
			}
			m.Avatar, m.Phone, m.Email, m.Org = &av, text(e.Row.Phone), text(e.Row.Email), text(e.Row.Org)
		case view.KindEvent:
			m.Lead, m.Place, m.Repeats = text(e.Row.Lead), text(e.Row.Place), text(e.Row.Repeats)
		case view.KindTask:
			done := e.Done
			m.Done, m.Due, m.Percent = &done, text(e.Detail), e.Row.Progress
		case view.KindNote:
			m.Excerpt, m.Date = text(e.Row.Excerpt), text(e.Detail)
		}
	}
	return m
}

// mergeBlock is why a group cannot be merged, or "": every member is written
// or deleted, so every one must be in a live collection that takes writes.
func mergeBlock(blob sealed.Blob, refs []sealed.RecordRef) string {
	for _, ref := range refs {
		col, ok := blob.Collection(ref.AccountID, ref.Col)
		if _, hasConn := blob.Connection(ref.AccountID); !ok || !hasConn || col.Gone {
			return "A member is in a collection that is no longer in the catalog."
		}
		if col.ReadOnly {
			return collectionLabel(col) + " is read-only: its record could be neither rewritten nor deleted."
		}
	}
	return ""
}

// decideAction is a decision of a group's card: a button of its form, do=…
// (from-typecase note 117 p. 8, s-dup-1).
func decideAction(id, label, do string) view.Action {
	return view.Action{ID: id, Label: text(label), Method: "POST", Enabled: true, Submits: true, Name: "do", Value: do}
}

func (s *Server) groupView(sess *session.Session, st dupState, g dupGroup) view.DuplicateGroup {
	out := view.DuplicateGroup{ID: g.id, Kind: g.kind, Score: g.score, Decision: g.decision, Hint: groupHint(g)}
	for _, r := range g.reasons {
		out.Reasons = append(out.Reasons, text(reasonText(g.kind, r)))
	}
	for _, ref := range g.refs {
		out.Rows = append(out.Rows, s.member(st, g.kind, ref))
	}
	out.Title = out.Rows[0].Title
	switch g.decision {
	case view.DecisionLinked:
		out.Badge = view.Badge{Label: text(fmt.Sprintf("linked ×%d", len(g.refs))), Role: view.BadgeLinked}
	case view.DecisionIgnored:
		out.Badge = view.Badge{Label: "not duplicates", Role: view.BadgeNeutral}
	default:
		out.Badge = view.Badge{Label: text(fmt.Sprintf("candidate ×%d", len(g.refs))), Role: view.BadgeNeutral}
	}
	if g.score > 0 {
		why := make([]string, len(out.Reasons))
		for i, r := range out.Reasons {
			why[i] = string(r)
		}
		out.Points = text(fmt.Sprintf("%d points · %s", g.score, strings.Join(why, ", ")))
	}
	// Merge on the server carries the values chosen on the card to the
	// merge (Дб9), which asks nothing (from-typecase note 117 p. 3); a
	// group it cannot write is not merged at all, and says why (p. 9).
	merge := decideAction("duplicates.merge", "Merge on the server", "merge")
	merge.Danger = true
	if reason := mergeBlock(st.blob, g.refs); reason != "" {
		merge.Enabled, merge.Reason = false, text(reason)
	}
	// «Not duplicates» asks nothing: there is nothing to keep (s-dup-1).
	var cs []question
	if g.decision != view.DecisionIgnored {
		cs = st.questions(g)
	}
	out.Choices = mergeRowsOf(cs)
	if len(cs) > 0 {
		// The questions say what differs; what such a pair means stands
		// only on a card with nothing to ask (s-dup-1: «Дежурство», «Что
		// почитать про CRDT»).
		out.Hint = ""
	}
	switch g.decision {
	case view.DecisionLinked:
		out.Actions = []view.Action{decideAction("duplicates.unlink", "Unlink", "unlink"), merge}
		if len(cs) > 0 {
			choose := decideAction("duplicates.choose", "Keep these values", "choose")
			choose.Primary = true // s-dup-1
			out.Actions = append([]view.Action{choose}, out.Actions...)
		}
	case view.DecisionIgnored:
		out.Actions = []view.Action{decideAction("duplicates.again", "Offer again", "again")}
	default:
		// What a group still to decide is first offered is the frame's main
		// button (s-dup-1).
		link := decideAction("duplicates.link", "Link them", "link")
		link.Primary = true
		out.Actions = []view.Action{link, decideAction("duplicates.ignore", "Not duplicates", "ignore"), merge}
	}
	// Every card is a form (U-8; from-typecase note 117 p. 8): its group
	// and token hidden, its questions in it, every decision a button of it.
	out.Form = &view.Form{Action: s.route("/app/duplicates/decide"), Method: "POST", Sets: []view.FieldSet{{Rows: []view.FormRow{
		csrfField(sess),
		{Name: "group", Kind: view.InputHidden, Value: text(g.id)},
	}}}}
	return out
}

func kindFilter(r *http.Request) view.Kind {
	k := view.Kind(r.URL.Query().Get("kind"))
	for _, have := range dupKinds {
		if k == have {
			return k
		}
	}
	return ""
}

var dupSegmentLabels = map[view.Kind]string{
	view.KindContact: "Contacts", view.KindEvent: "Events", view.KindTask: "Tasks", view.KindNote: "Notes",
}

// dupSources is the rail of the duplicates screen (s-dup-1): the
// collections the comparison had to work with, how many records of each it
// holds, and which of them this session has actually read. Duplicates are
// found among what is loaded and never by asking a server (S-16), so a
// screen that says «no duplicates» must also say how little it looked at.
func dupSources(st dupState, only view.Kind) []view.RailSection {
	held := map[dupes.CollectionRef]int{}
	for _, l := range st.records {
		held[dupes.CollectionRef{AccountID: l.rec.Ref.AccountID, Col: l.rec.Ref.Col}]++
	}
	sec := view.RailSection{State: view.CollectionOK}
	loaded, live := 0, 0
	seen := map[dupes.CollectionRef]bool{}
	// By kind, as the screen's segments go — the books, the calendars, the
	// lists, the notebooks —, each collection once, under the first kind it
	// is ticked for (s-dup-1: «Работа» among the calendars).
	for _, k := range dupKinds {
		if only != "" && k != only {
			continue
		}
		for _, col := range dupCollections(st.blob) {
			ref := dupes.CollectionRef{AccountID: col.AccountID, Col: col.Path}
			// What is ticked, and what this session read anyway — a list
			// opened alone: an unticked collection no list read is never
			// polled, so naming it a source would promise a comparison
			// that cannot happen.
			if col.Gone || !col.Has(k) || seen[ref] || !isTicked(st.blob, k, col.AccountID, col.Path) && !st.read[ref] {
				continue
			}
			seen[ref] = true
			live++
			// No tick: a source is compared or not, and says how many of
			// its records were — «—» before (s-dup-6).
			row := view.SourceRow{
				Label: text(collectionLabel(col)), Colour: catalogColour(col),
				Tick: view.TickOn, ReadOnly: col.ReadOnly, State: view.CollectionOK, Feed: col.Feed,
			}
			if st.read[ref] {
				n := held[ref]
				loaded++
				row.Count = &n
			}
			sec.Rows = append(sec.Rows, row)
		}
	}
	if len(sec.Rows) == 0 {
		return nil
	}
	// The count apart, monospaced (s-dup-1; Дб2).
	sec.Account, sec.Tally = "Sources", text(fmt.Sprintf("%d / %d", loaded, live))
	return []view.RailSection{sec}
}

// dupCollections are the collections the duplicates compare: those of the
// catalog and the feeds subscribed to (Дб3; s-dup-1: «Праздники РФ», is-ext)
// — read, never written, so a group with one can be linked, not merged
// (mergeBlock: a feed is read-only).
func dupCollections(blob sealed.Blob) []sealed.Collection {
	cols := slices.Clone(blob.Collections)
	for _, f := range blob.Subscriptions {
		cols = append(cols, f.FeedCollection())
	}
	return cols
}

func (s *Server) duplicatesPage(sess *session.Session, st dupState, only view.Kind) view.DuplicatesPage {
	page := view.DuplicatesPage{Shell: s.buildShell(sess, "Duplicates")}
	page.Top.Refresh = s.duplicatesRefresh(sess, only)
	page.Rail = view.Rail{Nav: s.navRows(sess, st.blob, "duplicates"), About: s.aboutAction(), Davloom: s.davloomMark(sess)}
	page.Rail.Sections = dupSources(st, only)
	all := st.groups()
	counts := map[view.Kind]int{}
	for _, g := range all {
		if g.decision == view.DecisionNone {
			counts[g.kind]++
		}
		switch {
		case only != "" && g.kind != only:
		case g.decision != view.DecisionNone && !st.loaded(g):
			page.Decided = append(page.Decided, s.decidedView(sess, st, g))
		default:
			page.Groups = append(page.Groups, s.groupView(sess, st, g))
		}
	}
	segment := func(k view.Kind, label string, n int) view.Action {
		q := url.Values{}
		if k != "" {
			q.Set("kind", string(k))
		}
		a := view.Action{ID: "duplicates.kind." + string(k), Label: text(label), Tally: text(strconv.Itoa(n)), Href: s.postQuery("/app/duplicates", q), Method: "GET", Enabled: true, On: only == k}
		if k == "" {
			a.ID = "duplicates.kind.all"
		}
		return a
	}
	total := 0
	for _, n := range counts {
		total += n
	}
	page.Bar.Segments = []view.Action{segment("", "All", total)}
	for _, k := range dupKinds {
		page.Bar.Segments = append(page.Bar.Segments, segment(k, dupSegmentLabels[k], counts[k]))
	}
	if st.compared == 0 {
		// Nothing read, nothing compared (s-dup-6, Дб10): no segments and
		// no cards — the decisions are kept and come back with the lists.
		page.Bar.Segments, page.Groups = nil, nil
	}
	page.Bar.Loaded = st.compared
	// What was compared and how many groups wait (s-dup-1, -6; Дб1): the
	// groups of the kind shown.
	page.Subtitle = text(plural(st.compared, "record") + " compared")
	if wait := counts[only]; only == "" && total > 0 || only != "" && wait > 0 {
		if only == "" {
			wait = total
		}
		page.Subtitle += text(fmt.Sprintf(" · %d to look at", wait))
	}
	page.Threshold = view.ThresholdControl{Value: st.threshold, Default: s.cfg.DuplicatesThreshold}
	// The threshold is a form of its own (U-8, s-dup-1): the field goes by
	// Enter, as the frame draws no button beside it.
	page.Threshold.Form = view.Form{Action: s.route("/app/duplicates/decide"), Method: "POST", Sets: []view.FieldSet{{Rows: []view.FormRow{
		csrfField(sess),
		{Name: "do", Kind: view.InputHidden, Value: "threshold"},
		{Name: "value", Label: "Match threshold", Kind: view.InputNumber, Value: text(strconv.Itoa(st.threshold)), Mono: true},
	}}}}
	mine := s.dupWeights(st.blob)
	owned := false
	for _, sig := range dupes.Signals {
		_, own := st.blob.Duplicates.Weights[string(sig)]
		owned = owned || own
		page.Threshold.Weights = append(page.Threshold.Weights, view.WeightControl{
			Signal: string(sig), Name: weightField(sig), Label: text(signalLabels[sig]), Value: dupes.Weight(mine, sig),
			Default: dupes.Weight(s.cfg.DuplicatesWeights, sig), Own: own,
		})
	}
	// The menu of weights is a form (s-dup-3, -7; from-typecase note 117
	// p. 7): Save sends the fields as they stand, do=weights; Back to
	// defaults sends the same form, do=defaults — the instance's weight for
	// every signal —, off when every weight is the default already.
	reset := decideAction("duplicates.weights.defaults", "Back to defaults", "defaults")
	if !owned {
		reset.Enabled, reset.Reason = false, "Every weight is the default already."
	}
	save := decideAction("duplicates.weights.save", "Save", "weights")
	save.Primary = true
	page.Threshold.WeightForm = view.Form{
		Action: s.route("/app/duplicates/decide"), Method: "POST",
		Sets:    []view.FieldSet{{Rows: []view.FormRow{csrfField(sess)}}},
		Actions: []view.Action{save, reset},
	}
	if st.blob.Duplicates.Threshold > 0 {
		page.Threshold.Note = text(fmt.Sprintf("Your own number, kept with your decisions. The instance uses %d.", s.cfg.DuplicatesThreshold))
	} else {
		page.Threshold.Note = "The instance's number. Set your own to change what is offered."
	}
	if st.apart > 0 {
		// Groups whose other members are gone came apart as the screen was
		// read (Б16): said, not left to be noticed, in the owner's words of
		// 03.11.2026.
		title, body := strconv.Itoa(st.apart)+" linked groups removed.", "Their other records are no longer at their addresses (deleted or moved). The records left are unchanged on the server."
		if st.apart == 1 {
			title, body = "1 linked group removed.", "Its other records are no longer at their addresses (deleted or moved). The records left are unchanged on the server."
		}
		page.Notices = append(page.Notices, view.Notice{Role: view.NoticeInfo, Title: text(title), Body: text(body)})
	}
	page.Notices = append(page.Notices, view.Notice{
		Role: view.NoticeAccent,
		Body: "Linking changes nothing on any server: the records stay where they are and are shown as one. Merging on the server is a separate, destructive step you are asked to confirm.",
	})
	if st.compared == 0 {
		page.Notices = append(page.Notices, view.Notice{
			Role:  view.NoticeInfo,
			Title: "Nothing is loaded yet.",
			Body:  "Duplicates are found only among the records this session has already read, so no server was asked. Open a list or a combined view, then come back.",
		})
	}
	page.Print = view.Print{Title: page.Title, Range: printRange(plural(st.compared, "record")+" compared", fmt.Sprintf("%d points and above", st.threshold))}
	return page
}

func (s *Server) getDuplicates(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	s.writePage(w, r, http.StatusOK, s.duplicatesPage(sess, s.readDuplicates(sess, true), kindFilter(r)))
}

// postDuplicatesDecide takes one decision: do=link, unlink, ignore or again
// on group=, a link carrying the answers to the group's Choices as
// choice.<PROP>=<option>, and do=choose changing them on a linked group; do=threshold with value= (empty or 0: the instance's number
// again); or do=weights with weight.<signal>= for the signals sent (empty:
// the instance's weight again). The answer to a decision is the page of
// duplicates as it now stands, with what there is to say in its notices —
// a group is no zone of C-7.2 (the lead's decision П2, 02.10.2026); a form
// without scripts and nothing to say, the threshold and the weights go back
// to the screen. No weighting suits everyone, so the person sets their own;
// a number out of range is not saved.
func (s *Server) postDuplicatesDecide(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	key, live := sess.DataKey()
	if !live {
		s.notFound(w, r)
		return
	}
	do := r.FormValue("do")
	if do == "threshold" {
		raw := strings.TrimSpace(r.FormValue("value"))
		n, err := strconv.Atoi(raw)
		if raw == "" {
			n, err = 0, nil
		}
		if err != nil || n < 0 || n > conf.MaxDuplicatesPoints {
			// A number that is not one is said, not dropped (Б45 of the
			// audit of 11.10.2026): the screen showed the old threshold as
			// if it were the one just typed.
			// The owner's words of 03.11.2026: the field by its name, what
			// it takes, and what it stays.
			s.duplicatesRefused(w, r, sess, func(page *view.DuplicatesPage) view.Notice {
				return view.Notice{Role: view.NoticeAlert,
					Title: text(fmt.Sprintf("Match threshold not changed: «%s» is not a whole number from 0 to %d.", raw, conf.MaxDuplicatesPoints)),
					Body:  text(fmt.Sprintf("It stays %d.", page.Threshold.Value))}
			})
			return
		}
		_ = s.users.EditDuplicates(sess.AccountID(), key, "duplicates.threshold", strconv.Itoa(n), func(d *sealed.Duplicates) error {
			d.Threshold = n
			return nil
		})
		s.redirect(w, r, "/app/duplicates")
		return
	}
	if do == "defaults" {
		_ = s.users.EditDuplicates(sess.AccountID(), key, "duplicates.weights", "", func(d *sealed.Duplicates) error {
			d.Weights = nil
			return nil
		})
		s.redirect(w, r, "/app/duplicates")
		return
	}
	if do == "weights" {
		var refused []string
		sentN := 0
		if r.ParseForm() == nil {
			_ = s.users.EditDuplicates(sess.AccountID(), key, "duplicates.weights", "", func(d *sealed.Duplicates) error {
				for _, sig := range dupes.Signals {
					vals, sent := r.Form[weightField(sig)]
					if !sent || len(vals) == 0 {
						continue
					}
					raw := strings.TrimSpace(vals[0])
					sentN++
					if raw == "" {
						delete(d.Weights, string(sig))
						continue
					}
					n, err := strconv.Atoi(raw)
					if err != nil || n < 0 || n > conf.MaxDuplicatesPoints {
						refused = append(refused, signalLabels[sig]+" · «"+raw+"»")
						continue
					}
					if d.Weights == nil {
						d.Weights = map[string]int{}
					}
					d.Weights[string(sig)] = n
				}
				return nil
			})
		}
		if len(refused) > 0 {
			// As a delete in Files (the owner's words of 03.11.2026): how
			// many of how many, the rule in the line, what was saved; each
			// by its signal under «Not saved».
			s.duplicatesRefused(w, r, sess, func(*view.DuplicatesPage) view.Notice {
				lines := make([]view.Text, 0, len(refused))
				for _, l := range refused {
					lines = append(lines, text(l))
				}
				return view.Notice{Role: view.NoticeAlert,
					Title:   text(fmt.Sprintf("%d of %s not saved: a weight is a whole number from 0 to %d.", len(refused), plural(sentN, "weight"), conf.MaxDuplicatesPoints)),
					Body:    text(fmt.Sprintf("%d saved, %d unchanged.", sentN-len(refused), len(refused))),
					Details: &view.Diagnostic{Summary: "Not saved", Lines: lines}}
			})
			return
		}
		s.redirect(w, r, "/app/duplicates")
		return
	}
	st := s.readDuplicates(sess, true)
	g, found := st.group(r.FormValue("group"))
	var notice *view.Notice
	var err error
	switch {
	case !found:
		notice = &view.Notice{
			Role: view.NoticeAlert, Title: "This group is not among the loaded records",
			Body: "The lists it came from are no longer loaded, or a record in it changed. Nothing was decided. Open the lists again, then the duplicates.",
		}
	case do == "merge":
		// The card's answers go to the merge as the members they chose
		// (pick.<PROP>=the member's place in the group): the merge's own
		// questions are numbered otherwise (Дб9).
		// A value unticked goes as drop.<PROP>=the value: it is not
		// carried into the merged record.
		prefs, dropped := answersOf(r, st.questions(g))
		q := url.Values{"group": {g.id}}
		for prop, ref := range prefs {
			for place, m := range g.refs {
				if m == ref {
					q.Set("pick."+prop, strconv.Itoa(place))
				}
			}
		}
		for prop, keys := range dropped {
			for _, k := range keys {
				q.Add("drop."+prop, k)
			}
		}
		s.redirect(w, r, "/app/duplicates/merge?"+q.Encode())
		return
	case do == "link" && g.decision != view.DecisionLinked:
		prefs, dropped := answersOf(r, st.questions(g))
		err = s.users.EditDuplicates(sess.AccountID(), key, "duplicates.link", string(g.kind), func(d *sealed.Duplicates) error {
			d.Linked = dupes.Link(d.Linked, sealed.LinkedGroup{Kind: g.kind, Members: g.refs, Prefs: prefs, Dropped: keptDrops(dropped), Title: st.titleOf(g)})
			d.Ignored = d.Unignored(g.id)
			return nil
		})
	case do == "choose" && g.decision == view.DecisionLinked:
		prefs, dropped := answersOf(r, st.questions(g))
		err = s.users.EditDuplicates(sess.AccountID(), key, "duplicates.choose", string(g.kind), func(d *sealed.Duplicates) error {
			for i := range d.Linked {
				if d.Linked[i].ID != g.id {
					continue
				}
				if d.Linked[i].Prefs == nil {
					d.Linked[i].Prefs = map[string]sealed.RecordRef{}
				}
				for prop, ref := range prefs {
					d.Linked[i].Prefs[prop] = ref
				}
				// What was asked is answered anew: a value ticked again is
				// no longer dropped.
				for prop, keys := range dropped {
					if d.Linked[i].Dropped == nil {
						d.Linked[i].Dropped = map[string][]string{}
					}
					d.Linked[i].Dropped[prop] = keys
				}
				d.Linked[i].Dropped = keptDrops(d.Linked[i].Dropped)
			}
			return nil
		})
	case do == "unlink" && g.decision == view.DecisionLinked:
		err = s.users.EditDuplicates(sess.AccountID(), key, "duplicates.unlink", string(g.kind), func(d *sealed.Duplicates) error {
			var kept []sealed.LinkedGroup
			for _, l := range d.Linked {
				if l.ID != g.id {
					kept = append(kept, l)
				}
			}
			d.Linked = kept
			return nil
		})
	case do == "ignore" && g.decision == view.DecisionNone:
		err = s.users.EditDuplicates(sess.AccountID(), key, "duplicates.ignore", string(g.kind), func(d *sealed.Duplicates) error {
			d.Ignored = append(d.Unignored(g.id), sealed.IgnoredGroup{ID: g.id, Kind: g.kind, Members: g.refs, Title: st.titleOf(g)})
			return nil
		})
	case do == "again" && g.decision == view.DecisionIgnored:
		err = s.users.EditDuplicates(sess.AccountID(), key, "duplicates.again", string(g.kind), func(d *sealed.Duplicates) error {
			d.Ignored = d.Unignored(g.id)
			return nil
		})
	}
	if errors.Is(err, user.ErrNotFound) {
		s.notFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "could not save the decision", http.StatusInternalServerError)
		return
	}
	if notice == nil && !render.WantsFragment(r) {
		s.redirect(w, r, "/app/duplicates")
		return
	}
	page := s.duplicatesPage(sess, s.readDuplicates(sess, false), kindFilter(r))
	if notice != nil {
		page.Notices = append(page.Notices, *notice)
	}
	s.writePage(w, r, http.StatusOK, page)
}

// duplicatesRefused answers a setting of the duplicates screen that was not
// taken: the screen as it stands, and why — said of the screen, which holds
// what stays.
func (s *Server) duplicatesRefused(w http.ResponseWriter, r *http.Request, sess *session.Session, why func(*view.DuplicatesPage) view.Notice) {
	page := s.duplicatesPage(sess, s.readDuplicates(sess, false), kindFilter(r))
	page.Notices = append(page.Notices, why(&page))
	s.writePage(w, r, http.StatusOK, page)
}

// foldLinked shows the members of a linked group as one row (S-16): the
// first member present keeps its place and carries the others, which leave
// the list. Nothing is folded that the person did not link.
func (s *Server) foldLinked(blob sealed.Blob, k view.Kind, items []listed) []listed {
	if len(blob.Duplicates.Linked) == 0 {
		return items
	}
	uidOf := func(it listed) string { return listedUID(k, it) }
	keep := map[string]int{} // linked group → index in out
	out := make([]listed, 0, len(items))
	for _, it := range items {
		ref := sealed.RecordRef{AccountID: it.from.conn.ID, Col: it.from.col.Path, UID: uidOf(it)}
		g, ok := dupes.LinkedOf(blob.Duplicates.Linked, ref)
		if !ok || g.Kind != k {
			out = append(out, it)
			continue
		}
		if at, seen := keep[g.ID]; seen {
			out[at].also = append(out[at].also, it)
			for prop, want := range g.Prefs {
				if want == ref {
					out[at].prefer[prop] = it.obj
				}
			}
			continue
		}
		keep[g.ID] = len(out)
		it.prefer = map[string]*model.Object{}
		for prop, want := range g.Prefs {
			if want == ref {
				it.prefer[prop] = it.obj
			}
		}
		out = append(out, it)
	}
	// Which member of a group the folded row shows is a decision of S-16 —
	// the first of its members — and not a race. Until this stood here it
	// was whichever source answered first, so the same list gave a
	// different row, a different address and a different stamp on the next
	// run, and no fixture could hold it.
	for _, at := range keep {
		out[at] = shownFirst(blob.Duplicates.Linked, k, out[at])
	}
	return out
}

// shownFirst puts the group's members in the order the group holds them:
// the earliest becomes the row, the rest its others. A member the poll did
// not bring is simply absent — the order of those that came is still the
// group's.
func shownFirst(linked []sealed.LinkedGroup, k view.Kind, row listed) listed {
	if len(row.also) == 0 {
		return row
	}
	all := append([]listed{row}, row.also...)
	g, ok := dupes.LinkedOf(linked, sealed.RecordRef{
		AccountID: row.from.conn.ID, Col: row.from.col.Path, UID: listedUID(k, row),
	})
	if !ok {
		return row
	}
	rank := make(map[sealed.RecordRef]int, len(g.Members))
	for i, m := range g.Members {
		rank[m] = i
	}
	place := func(it listed) int {
		if i, ok := rank[(sealed.RecordRef{AccountID: it.from.conn.ID, Col: it.from.col.Path, UID: listedUID(k, it)})]; ok {
			return i
		}
		return len(g.Members)
	}
	sort.SliceStable(all, func(i, j int) bool { return place(all[i]) < place(all[j]) })
	first := all[0]
	first.also, first.prefer = all[1:], row.prefer
	return first
}

// linkedElsewhere gives the rows of one collection's list the members of
// their linked groups that live in other collections: the list reads only
// its own, and the link is still the person's decision (S-16) — the frames
// of one list draw «linked ×2» and both sources (s-tasks-15, s-notes-31;
// from-typecase note 36). A member whose collection the catalog no longer
// holds is not named.
func linkedElsewhere(blob sealed.Blob, k view.Kind, items []listed) []listed {
	if len(blob.Duplicates.Linked) == 0 {
		return items
	}
	for i, it := range items {
		ref := sealed.RecordRef{AccountID: it.from.conn.ID, Col: it.from.col.Path, UID: listedUID(k, it)}
		g, ok := dupes.LinkedOf(blob.Duplicates.Linked, ref)
		if !ok || g.Kind != k {
			continue
		}
		here := map[sealed.RecordRef]bool{ref: true}
		for _, o := range it.also {
			here[sealed.RecordRef{AccountID: o.from.conn.ID, Col: o.from.col.Path, UID: listedUID(k, o)}] = true
		}
		for _, m := range g.Members {
			if here[m] {
				continue
			}
			conn, okConn := blob.Connection(m.AccountID)
			col, okCol := blob.Collection(m.AccountID, m.Col)
			if okConn && okCol && !col.Gone {
				items[i].elsewhere = append(items[i].elsewhere, origin{conn: conn, col: col}.stamp())
			}
		}
	}
	return items
}

// linkedSources are the stamps of a folded row: its own, its others' and
// those of the members that live elsewhere (linkedElsewhere).
func linkedSources(it listed) []view.Stamp {
	if len(it.also)+len(it.elsewhere) == 0 {
		return nil
	}
	out := []view.Stamp{it.from.stamp()}
	for _, o := range it.also {
		out = append(out, o.from.stamp())
	}
	return append(out, it.elsewhere...)
}

func linkedBadge(it listed) []view.Badge {
	n := len(it.also) + len(it.elsewhere)
	if n == 0 {
		return nil
	}
	// «linked ×2», as the frames say it (s-contacts-1): the sources stand
	// beside the badge, so naming them twice says nothing.
	return []view.Badge{{Label: text(fmt.Sprintf("linked ×%d", n+1)), Role: view.BadgeLinked}}
}

func linkedState(it listed) view.RecordState {
	if len(it.also)+len(it.elsewhere) > 0 {
		return view.RecordLinked
	}
	return view.RecordOK
}

// questions are the questions of g among its loaded members, answered as
// its linked group was (S-16).
func (st dupState) questions(g dupGroup) []question {
	l := st.linked(g.id)
	return questionsOf(g.kind, st.groupMembers(g.refs), l.Prefs, l.Dropped)
}

// groupMembers are the members as the questions see them: where each lives
// and, when its list is loaded, its values.
func (st dupState) groupMembers(refs []sealed.RecordRef) []groupMember {
	out := make([]groupMember, 0, len(refs))
	for _, ref := range refs {
		conn, _ := st.blob.Connection(ref.AccountID)
		col, ok := st.blob.Collection(ref.AccountID, ref.Col)
		if !ok {
			col = sealed.Collection{AccountID: ref.AccountID, Path: ref.Col}
		}
		m := groupMember{ref: ref, col: collectionLabel(col), colour: catalogColour(col), stamp: origin{conn: conn, col: col}.stamp()}
		if l, ok := st.records[ref]; ok {
			m.values, m.loaded = l.entry.Values, true
		}
		out = append(out, m)
	}
	return out
}

// linked is the person's linked group of this ID; empty when there is none.
func (st dupState) linked(id string) sealed.LinkedGroup {
	for _, l := range st.blob.Duplicates.Linked {
		if l.ID == id {
			return l
		}
	}
	return sealed.LinkedGroup{}
}

// keptDrops are the dropped values worth keeping: a property nothing of
// which is dropped is left out.
func keptDrops(d map[string][]string) map[string][]string {
	out := map[string][]string{}
	for prop, keys := range d {
		if len(keys) > 0 {
			out[prop] = keys
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// duplicatesRefresh is the circle of the top bar (Дб11; s-dup-1, -2, -6):
// the screen compares only what this session has read — no server is asked
// here —, so «2 min ago» is how fresh the records compared are, the oldest
// of the lists they came from, and the circle compares them again; a list
// is read anew where it is opened, as its hint says.
func (s *Server) duplicatesRefresh(sess *session.Session, only view.Kind) view.Refresh {
	q := url.Values{}
	if only != "" {
		q.Set("kind", string(only))
	}
	a := view.Action{ID: "duplicates.refresh", Label: "Refresh", Icon: "i-refresh", Href: s.postQuery("/app/duplicates", q), Method: "GET",
		Enabled: true, Hint: "Compares again what this session has read; open a list to read it anew."}
	var oldest time.Time
	for _, k := range dupKinds {
		if only != "" && k != only {
			continue
		}
		if snap := lastPoll(sess, k); snap != nil && !snap.Finished.IsZero() && (oldest.IsZero() || snap.Finished.Before(oldest)) {
			oldest = snap.Finished
		}
	}
	r := view.Refresh{Action: a}
	if oldest.IsZero() {
		r.Action.Enabled, r.Action.Hint, r.Action.Reason = false, "", "Nothing is loaded yet: open a list first."
		return r
	}
	tf := s.timeFor(sess)
	r.Answered = tf.formatWhen(oldest, true, tf.now)
	return r
}
