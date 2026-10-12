package web

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

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
	found     []dupes.Group
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

var dupKinds = []view.Kind{view.KindContact, view.KindEvent, view.KindTask, view.KindNote}

// loadedRecords reads the index of every catalog collection the session has
// loaded. present holds, per collection and kind, the UIDs of a list read
// whole: only such a list says a member is gone. The agenda reads a week, so
// an event missing from it may simply be in another week. A collection the
// catalog no longer holds, or holds as gone, has nothing in it.
func (s *Server) loadedRecords(sess *session.Session, blob sealed.Blob) ([]loadedRecord, map[dupes.CollectionRef]map[string]bool) {
	var out []loadedRecord
	present := map[dupes.CollectionRef]map[string]bool{}
	for _, col := range blob.Collections {
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
	return out, present
}

// readDuplicates finds the groups among what is loaded. A linked member the
// loaded lists say is gone leaves its group silently, and the blob is written
// without a journal line (S-16).
func (s *Server) readDuplicates(sess *session.Session, prune bool) dupState {
	blob := s.catalogOf(sess)
	loaded, present := s.loadedRecords(sess, blob)
	if prune {
		if kept, changed := dupes.Prune(blob.Duplicates.Linked, present); changed {
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
	st := dupState{blob: blob, records: map[sealed.RecordRef]loadedRecord{}, threshold: s.dupThreshold(blob)}
	recs := make([]dupes.Record, 0, len(loaded))
	for _, l := range loaded {
		if _, seen := st.records[l.rec.Ref]; seen {
			continue
		}
		st.records[l.rec.Ref] = l
		recs = append(recs, l.rec)
	}
	st.compared = len(recs)
	st.found = dupes.Find(recs, s.dupWeights(blob), st.threshold)
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
// still to decide among what is loaded.
func (s *Server) duplicateCount(sess *session.Session) *int {
	n := len(s.readDuplicates(sess, false).candidates())
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
			ignored = append(ignored, dg)
		default:
			todo = append(todo, dg)
		}
	}
	for _, l := range st.blob.Duplicates.Linked {
		dg := dupGroup{id: l.ID, kind: l.Kind, refs: l.Members, decision: view.DecisionLinked}
		for _, g := range st.found {
			if dupes.Covered([]sealed.LinkedGroup{l}, g) && len(g.Members) == len(l.Members) {
				dg.score, dg.reasons = g.Score, g.Reasons
			}
		}
		linked = append(linked, dg)
	}
	return append(append(todo, linked...), ignored...)
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
		m.Title, m.Detail = text(l.entry.Title), text(l.entry.Detail)
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

func (s *Server) decideAction(id, label, group, do string) view.Action {
	return view.Action{
		ID: id, Label: text(label), Method: "POST", Enabled: true,
		Href: s.postQuery("/app/duplicates/decide", url.Values{"group": {group}, "do": {do}}),
	}
}

func (s *Server) groupView(sess *session.Session, st dupState, g dupGroup) view.DuplicateGroup {
	out := view.DuplicateGroup{ID: g.id, Kind: g.kind, Score: g.score, Decision: g.decision}
	for _, r := range g.reasons {
		out.Reasons = append(out.Reasons, text(reasonText(g.kind, r)))
	}
	for _, ref := range g.refs {
		out.Rows = append(out.Rows, s.member(st, g.kind, ref))
	}
	merge := view.Action{
		ID: "duplicates.merge", Label: "Merge on the server", Method: "GET", Danger: true, Enabled: true,
		Href: s.postQuery("/app/duplicates/merge", url.Values{"group": {g.id}}),
	}
	if reason := mergeBlock(st.blob, g.refs); reason != "" {
		merge.Enabled, merge.Reason = false, text(reason)
	}
	cs := st.choices(g)
	prefs := st.linkedPrefs(g.id)
	for _, c := range cs {
		row := view.MergeRow{Property: text(choiceLabel(c.prop)), Name: "choice." + c.prop, Chosen: chosen(c, prefs)}
		for _, o := range c.opts {
			row.Options = append(row.Options, text(o.label))
		}
		out.Choices = append(out.Choices, row)
	}
	switch g.decision {
	case view.DecisionLinked:
		out.Actions = []view.Action{s.decideAction("duplicates.unlink", "Unlink", g.id, "unlink"), merge}
		if len(cs) > 0 {
			choose := s.decideAction("duplicates.choose", "Keep these values", g.id, "choose")
			choose.Primary = true // s-dup-1
			out.Actions = append([]view.Action{choose}, out.Actions...)
		}
	case view.DecisionIgnored:
		out.Actions = []view.Action{s.decideAction("duplicates.again", "Offer again", g.id, "again")}
	default:
		// What a group still to decide is first offered is the frame's main
		// button (s-dup-1).
		link := s.decideAction("duplicates.link", "Link them", g.id, "link")
		link.Primary = true
		out.Actions = []view.Action{link, s.decideAction("duplicates.ignore", "Not duplicates", g.id, "ignore"), merge}
	}
	// With something to choose, the card is a form (U-8, s-dup-1): its
	// radios stand in it, and every decision is a button of it sent to its
	// own address — a decision of its own form inside it would be a form
	// in a form. Merge on the server stays a link.
	if len(cs) > 0 {
		out.Form = &view.Form{Action: s.route("/app/duplicates/decide"), Method: "POST", Sets: []view.FieldSet{{Rows: []view.FormRow{
			csrfField(sess),
			{Name: "group", Kind: view.InputHidden, Value: text(g.id)},
		}}}}
		for i := range out.Actions {
			if out.Actions[i].Method == "POST" {
				out.Actions[i].Submits = true
			}
		}
	}
	return out
}

// choiceLabels name a property asked about or joined, as the frame labels
// a question (s-dup-1: «Display name», «Organisation», «Photo»): the
// property is the file's, the label the person's. One Carrel does not
// label is named as the file names it.
var choiceLabels = map[string]string{
	"FN": "Display name", "N": "Name", "ORG": "Organisation", "PHOTO": "Photo", "BDAY": "Birthday",
	"ANNIVERSARY": "Anniversary", "GENDER": "Gender", "TEL": "Phone numbers", "EMAIL": "Email addresses",
	"ADR": "Addresses", "NOTE": "Note", "CATEGORIES": "Tags", "URL": "Web address",
	"SUMMARY": "Title", "DESCRIPTION": "Description", "LOCATION": "Location", "STATUS": "Status",
	"PRIORITY": "Priority", "PERCENT-COMPLETE": "Percent complete", "COMPLETED": "Completed",
	"ORGANIZER": "Organiser", "ATTENDEE": "Attendees", "CLASS": "Class", "TRANSP": "Busy", "COLOR": "Colour",
	dupes.When: "When",
}

func choiceLabel(prop string) string {
	if l, ok := choiceLabels[prop]; ok {
		return l
	}
	return prop
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
	for _, col := range st.blob.Collections {
		if col.Gone || !holdsRecords(col) {
			continue
		}
		// One kind at a time, as the screen shows them: the books of
		// contacts are not where a duplicate event would be found. And
		// only what is ticked: an unticked collection is never polled,
		// so naming it a source would promise a comparison that cannot
		// happen.
		if only != "" && (!col.Has(only) || !isTicked(st.blob, only, col.AccountID, col.Path)) {
			continue
		}
		live++
		ref := dupes.CollectionRef{AccountID: col.AccountID, Col: col.Path}
		row := view.SourceRow{
			Label: text(collectionLabel(col)), Colour: catalogColour(col),
			Tick: view.TickOff, ReadOnly: col.ReadOnly, State: view.CollectionOK,
		}
		if n, ok := held[ref]; ok {
			loaded++
			row.Tick, row.Count = view.TickOn, &n
		}
		sec.Rows = append(sec.Rows, row)
	}
	if len(sec.Rows) == 0 {
		return nil
	}
	sec.Account = text(fmt.Sprintf("Sources %d / %d", loaded, live))
	return []view.RailSection{sec}
}

// holdsRecords: a folder of files holds no records to compare.
func holdsRecords(col sealed.Collection) bool {
	for _, k := range dupKinds {
		if col.Has(k) {
			return true
		}
	}
	return false
}

func (s *Server) duplicatesPage(sess *session.Session, st dupState, only view.Kind) view.DuplicatesPage {
	page := view.DuplicatesPage{Shell: s.buildShell(sess, "Duplicates")}
	page.Rail = view.Rail{Nav: s.navRows(sess, st.blob, "duplicates"), About: s.aboutAction(), Davloom: s.davloomMark(sess)}
	page.Rail.Sections = dupSources(st, only)
	all := st.groups()
	counts := map[view.Kind]int{}
	for _, g := range all {
		if g.decision == view.DecisionNone {
			counts[g.kind]++
		}
		if only == "" || g.kind == only {
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
	page.Bar.Loaded = st.compared
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
			Signal: string(sig), Name: "weight." + string(sig), Label: text(signalLabels[sig]), Value: dupes.Weight(mine, sig),
			Default: dupes.Weight(s.cfg.DuplicatesWeights, sig), Own: own,
		})
	}
	// The menu of weights is a form (s-dup-3): Save sends the fields as
	// they stand; Back to defaults sends the same form to do=defaults,
	// which is the instance's weight for every signal. The address says
	// what to do — a hidden do would be read before it.
	reset := view.Action{ID: "duplicates.weights.defaults", Label: "Back to defaults", Href: s.postQuery("/app/duplicates/decide", url.Values{"do": {"defaults"}}),
		Method: "POST", Enabled: owned, Submits: true}
	if !owned {
		reset.Href, reset.Reason, reset.Submits = "", "Every weight is the default already.", false
	}
	page.Threshold.WeightForm = view.Form{
		Action: s.postQuery("/app/duplicates/decide", url.Values{"do": {"weights"}}), Method: "POST",
		Sets: []view.FieldSet{{Rows: []view.FormRow{csrfField(sess)}}},
		Actions: []view.Action{
			{ID: "duplicates.weights.save", Label: "Save", Method: "POST", Primary: true, Enabled: true},
			reset,
		},
	}
	if st.blob.Duplicates.Threshold > 0 {
		page.Threshold.Note = text(fmt.Sprintf("Your own number, kept with your decisions. The instance uses %d.", s.cfg.DuplicatesThreshold))
	} else {
		page.Threshold.Note = "The instance's number. Set your own to change what is offered."
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
			s.duplicatesRefused(w, r, sess, "The threshold was not changed",
				fmt.Sprintf("«%s» is not a number from 0 to %d.", raw, conf.MaxDuplicatesPoints))
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
		if r.ParseForm() == nil {
			_ = s.users.EditDuplicates(sess.AccountID(), key, "duplicates.weights", "", func(d *sealed.Duplicates) error {
				for _, sig := range dupes.Signals {
					vals, sent := r.Form["weight."+string(sig)]
					if !sent || len(vals) == 0 {
						continue
					}
					raw := strings.TrimSpace(vals[0])
					if raw == "" {
						delete(d.Weights, string(sig))
						continue
					}
					n, err := strconv.Atoi(raw)
					if err != nil || n < 0 || n > conf.MaxDuplicatesPoints {
						refused = append(refused, "«"+raw+"»")
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
			s.duplicatesRefused(w, r, sess, text(plural(len(refused), "weight")+" not changed"),
				fmt.Sprintf("%s: a weight is a number from 0 to %d. The other weights were saved.", strings.Join(refused, ", "), conf.MaxDuplicatesPoints))
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
	case do == "link" && g.decision != view.DecisionLinked:
		prefs := answers(r, st.choices(g))
		err = s.users.EditDuplicates(sess.AccountID(), key, "duplicates.link", string(g.kind), func(d *sealed.Duplicates) error {
			d.Linked = dupes.Link(d.Linked, sealed.LinkedGroup{Kind: g.kind, Members: g.refs, Prefs: prefs})
			d.Ignored = without(d.Ignored, g.id)
			return nil
		})
	case do == "choose" && g.decision == view.DecisionLinked:
		prefs := answers(r, st.choices(g))
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
			d.Ignored = append(without(d.Ignored, g.id), g.id)
			return nil
		})
	case do == "again" && g.decision == view.DecisionIgnored:
		err = s.users.EditDuplicates(sess.AccountID(), key, "duplicates.again", string(g.kind), func(d *sealed.Duplicates) error {
			d.Ignored = without(d.Ignored, g.id)
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
// taken: the screen as it stands, and why.
func (s *Server) duplicatesRefused(w http.ResponseWriter, r *http.Request, sess *session.Session, title view.Text, body string) {
	page := s.duplicatesPage(sess, s.readDuplicates(sess, false), kindFilter(r))
	page.Notices = append(page.Notices, view.Notice{Role: view.NoticeAlert, Title: title, Body: text(body)})
	s.writePage(w, r, http.StatusOK, page)
}

func without(list []string, v string) []string {
	var out []string
	for _, s := range list {
		if s != v {
			out = append(out, s)
		}
	}
	return out
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

// choiceProps are the single-valued properties a row of the kind shows —
// the ones asked about when linked members disagree (S-16).
var choiceProps = map[view.Kind][]string{
	view.KindContact: {"FN", "ORG", "PHOTO"},
	view.KindEvent:   {"SUMMARY", "LOCATION"},
	view.KindTask:    {"SUMMARY"},
	view.KindNote:    {"SUMMARY"},
}

// choiceFields are the values of choiceProps as the record holds them; a
// photo is a digest, enough to tell two apart.
func choiceFields(k view.Kind, o *model.Object) map[string]string {
	out := map[string]string{}
	for _, prop := range choiceProps[k] {
		vals := o.PropText(prop)
		if len(vals) == 0 || strings.TrimSpace(vals[0]) == "" {
			continue
		}
		v := vals[0]
		if prop == "PHOTO" {
			sum := sha256.Sum256([]byte(v))
			v = hex.EncodeToString(sum[:8])
		} else if k != view.KindContact {
			v = calUnescape.Replace(v)
		}
		out[prop] = v
	}
	return out
}

// choice is one disagreement of a group: the property and, per distinct
// value, the first member that holds it.
type choice struct {
	prop string
	opts []choiceOption
}

type choiceOption struct {
	label string
	ref   sealed.RecordRef
}

// choices are the single-valued properties the loaded members of g hold
// differently. A member whose list is not loaded has nothing to offer.
func (st dupState) choices(g dupGroup) []choice {
	var out []choice
	for _, prop := range choiceProps[g.kind] {
		c := choice{prop: prop}
		seen := map[string]bool{}
		for _, ref := range g.refs {
			l, ok := st.records[ref]
			if !ok {
				continue
			}
			v, ok := l.entry.Fields[prop]
			if !ok || seen[v] {
				continue
			}
			seen[v] = true
			label := v
			if prop == "PHOTO" {
				// A photo is told by where it lives: the server, as the
				// frame names it (s-dup-1, «nextcloud», «radicale · vps»).
				label = connLabel(l.from.conn)
			}
			c.opts = append(c.opts, choiceOption{label: label, ref: ref})
		}
		if len(c.opts) > 1 {
			out = append(out, c)
		}
	}
	return out
}

// chosen is the option the linked group's answer names, else the first.
func chosen(c choice, prefs map[string]sealed.RecordRef) int {
	if ref, ok := prefs[c.prop]; ok {
		for i, o := range c.opts {
			if o.ref == ref {
				return i
			}
		}
	}
	return 0
}

// answers read choice.<PROP>= of the request against the group's choices.
func answers(r *http.Request, cs []choice) map[string]sealed.RecordRef {
	out := map[string]sealed.RecordRef{}
	for _, c := range cs {
		n, err := strconv.Atoi(r.FormValue("choice." + c.prop))
		if err != nil || n < 0 || n >= len(c.opts) {
			continue
		}
		out[c.prop] = c.opts[n].ref
	}
	return out
}

func (st dupState) linkedPrefs(id string) map[string]sealed.RecordRef {
	for _, l := range st.blob.Duplicates.Linked {
		if l.ID == id {
			return l.Prefs
		}
	}
	return nil
}
