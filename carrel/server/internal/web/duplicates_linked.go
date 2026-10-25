package web

import (
	"context"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"

	"carrel.dev/carrel/server/internal/dupes"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// The record of a linked group itself (from-typecase note 117 p. 5; the
// owner's decision of 23.10.2026): /app/{kind}/linked/{uid} is the card of
// the entity, what the combined list opens. It shows what the person chose
// on the group's card — the record a merge would write, unticked values
// left out —, each value that was a choice marked with the collection it
// came from (s-contacts-33). Edit opens «Which copy to edit?», «⋯» holds
// Print, Open in Duplicates and Unlink; Move, Copy, Export, Publish and
// Delete are a copy's, on its own card. Under it, where the record is kept,
// each copy with what it holds otherwise than shown.

// readMembers reads the members of g from their servers, in the rail's
// order (railOrder), all but skip. A member its server says is not there is
// gone; one whose server does not answer stays, without its record.
func (s *Server) readMembers(ctx context.Context, sess *session.Session, spec kindSpec, g sealed.LinkedGroup, skip sealed.RecordRef) (members []linkedMember, gone []sealed.RecordRef) {
	for _, ref := range railOrder(s.catalogOf(sess), g.Members) {
		if ref == skip {
			continue
		}
		mc, ok := s.collectionOf(sess, spec, ref.AccountID, ref.Col)
		if !ok {
			gone = append(gone, ref)
			continue
		}
		obj, err := mc.find(ctx, ref.UID)
		switch {
		case notFoundErr(err):
			gone = append(gone, ref)
		case err != nil:
			members = append(members, linkedMember{ref: ref, c: mc})
		default:
			members = append(members, linkedMember{ref: ref, c: mc, obj: obj})
		}
	}
	return members, gone
}

// linkedRecord is a linked group read whole.
type linkedRecord struct {
	group   sealed.LinkedGroup
	members []linkedMember
	asked   []groupMember
	qs      []question
	// base is the first member that answered: the card stands in its
	// collection, and the record chosen is built on it
	base   linkedMember
	chosen *model.Object
}

// linkedRecordOf reads g and builds the record the person chose. False when
// no member answered.
func (s *Server) linkedRecordOf(ctx context.Context, sess *session.Session, spec kindSpec, g sealed.LinkedGroup) (linkedRecord, bool) {
	members, gone := s.readMembers(ctx, sess, spec, g, sealed.RecordRef{})
	if len(gone) > 0 {
		s.forgetMembers(sess, "", gone)
	}
	return buildLinked(s.timeFor(sess), spec.kind, g, members)
}

// linkedRecordFrom is the record chosen of the group of a copy already
// read: the copy opened in c and the others linkedOf read.
func linkedRecordFrom(c *collectionCtx, lc linkedCard, obj *model.Object) (linkedRecord, bool) {
	var members []linkedMember
	for _, ref := range lc.group.Members {
		if ref == lc.self {
			members = append(members, linkedMember{ref: ref, c: c, obj: obj})
			continue
		}
		for _, m := range lc.others {
			if m.ref == ref {
				members = append(members, m)
			}
		}
	}
	return buildLinked(c.tf, c.spec.kind, lc.group, members)
}

// buildLinked asks the questions among the members that answered and builds
// on the first of them the record the person chose. False when none
// answered.
func buildLinked(tf timeFmt, k view.Kind, g sealed.LinkedGroup, members []linkedMember) (linkedRecord, bool) {
	lr := linkedRecord{group: g, members: members}
	var refs []sealed.RecordRef
	var objs []*model.Object
	for _, m := range members {
		gm := groupMember{ref: m.ref, col: collectionLabel(m.c.col), colour: catalogColour(m.c.col), stamp: m.c.stamp(), loaded: m.obj != nil}
		if m.obj != nil {
			gm.values = valuesOf(tf, k, m.obj)
			if len(objs) == 0 {
				lr.base = m
			}
			refs, objs = append(refs, m.ref), append(objs, m.obj)
		}
		lr.asked = append(lr.asked, gm)
	}
	if len(objs) == 0 {
		return lr, false
	}
	lr.qs = questionsOf(k, lr.asked, g.Prefs, g.Dropped)
	merged, _, err := dupes.MergeKeeping(objs[0], objs[1:], mergeChoices(refs, objs, lr.qs), g.Dropped)
	if err != nil {
		merged = objs[0]
	}
	lr.chosen = merged
	return lr, true
}

// valuesAt are the values of the member at ref; nil when it did not answer.
func (lr linkedRecord) valuesAt(ref sealed.RecordRef) recValues {
	for _, m := range lr.asked {
		if m.ref == ref {
			return m.values
		}
	}
	return nil
}

// mergeProp is the merge's name of a question: a note's text is its
// DESCRIPTION, a time — when or due — the merge's When, whole.
func mergeProp(prop string) string {
	switch prop {
	case "TEXT":
		return "DESCRIPTION"
	case "WHEN", "DUE":
		return dupes.When
	}
	return prop
}

// mergeChoices are the answers of the questions as the merge takes them
// (dupes.Merge: an option's index, Leave): the member whose value stays.
// refs and objs are the members that answered, the kept one first.
func mergeChoices(refs []sealed.RecordRef, objs []*model.Object, qs []question) map[string]int {
	out := map[string]int{}
	_, probe, err := dupes.Merge(objs[0], objs[1:], nil)
	if err != nil {
		return out
	}
	for _, q := range qs {
		if q.many() {
			continue
		}
		o, prop := q.opts[q.chosen], mergeProp(q.prop())
		if o.none {
			if prop != dupes.When {
				out[prop] = dupes.Leave
			}
			continue
		}
		for _, pq := range probe.Questions {
			if pq.Prop != prop {
				continue
			}
			for i, po := range pq.Options {
				if o.holds(refs[po.From]) {
					out[prop] = i
					break
				}
			}
		}
	}
	return out
}

// fieldQuestions are the questions a card's field answers, by its label: a
// phone by its type, an address by «Email · …», a single value by its
// property's field («Organisation», «When», «Where», «Due», «Priority»).
var fieldOfQuestion = map[string]string{
	"ORG": "Organisation", "TITLE": "Title", "BDAY": "Birthday", "NOTE": "Note",
	"WHEN": "When", "LOCATION": "Where", "DUE": "Due", "PRIORITY": "Priority",
}

// questionOf is the question a field of a card answers and the key of its
// value there; false when the field was no choice.
func questionOf(f view.Field, qs []question) (question, string, bool) {
	label := string(f.Label)
	for _, q := range qs {
		switch prop := q.prop(); {
		case prop == "TEL" && label == askedLabel(q.id):
			return q, dupes.ValueKey(prop, string(f.Value)), true
		case prop == "ADR" && label == askedLabel(q.id):
			return q, strings.ToLower(string(f.Value)), true
		case prop == "EMAIL" && (label == "Email" || strings.HasPrefix(label, "Email · ")):
			return q, dupes.ValueKey(prop, string(f.Value)), true
		case prop == "CATEGORIES" && label == "Tags":
			return q, "", true
		case !q.many() && fieldOfQuestion[prop] == label:
			if o := q.opts[q.chosen]; !o.none {
				return q, o.value.Key, true
			}
			return q, "", true
		}
	}
	return question{}, "", false
}

// optionOf is the option of q holding the value key; -1 when none does (a
// value every member holds).
func (q question) optionOf(key string) int {
	for i, o := range q.opts {
		if !o.none && o.value.Key == key {
			return i
		}
	}
	return -1
}

// markAsked gives the badge «linked» to the fields of a copy's card where a
// choice is made (s-contacts-6: Phone · work, Email · second; s-agenda-2,
// s-tasks-2): a single value asked about, a value of many that not every
// copy holds, the tags when they were asked about. A field with a badge of
// its own (the zone it was made in, the end of a series) keeps it.
func markAsked(fields []view.Field, qs []question) []view.Field {
	out := append([]view.Field(nil), fields...)
	for i, f := range out {
		if f.Badge.Label != "" {
			continue
		}
		q, key, ok := questionOf(f, qs)
		if !ok || (q.many() && q.prop() != "CATEGORIES" && q.optionOf(key) < 0) {
			continue
		}
		out[i].Badge = linkedMark
	}
	return out
}

// markFrom names, after each value of the linked record that was a choice,
// the collection it came from (s-contacts-33: «+7 495 120-45-77 Команда»).
func (lr linkedRecord) markFrom(fields []view.Field) []view.Field {
	out := append([]view.Field(nil), fields...)
	for i, f := range out {
		q, key, ok := questionOf(f, lr.qs)
		if !ok {
			continue
		}
		if q.prop() == "CATEGORIES" {
			out[i].From = lr.tagsFrom(q)
			continue
		}
		if from := q.shownFrom(key); from != "" {
			out[i].From = lr.from(from)
		}
	}
	return out
}

// tagsFrom is the collection of the first tag kept that was a choice.
func (lr linkedRecord) tagsFrom(q question) *view.FieldFrom {
	for i, o := range q.opts {
		if q.kept[i] {
			return lr.from(o.holders[0].col)
		}
	}
	return nil
}

func (lr linkedRecord) from(col string) *view.FieldFrom {
	for _, m := range lr.asked {
		if m.col == col {
			return &view.FieldFrom{Name: text(col), Colour: m.colour}
		}
	}
	return nil
}

// kept are the rows under the card: every copy, where it lives — a link to
// its own card — and what it holds otherwise than shown (s-contacts-33:
// «Phone, email differ»); a copy holding the same says nothing, one whose
// server did not answer says so.
func (s *Server) keptRows(lr linkedRecord, shown func(question) []recValue) []view.Field {
	var out []view.Field
	for i, m := range lr.members {
		row := view.Field{Label: text(string(m.c.stamp())), Href: s.route(m.c.recordPath(m.ref.UID)), Colour: catalogColour(m.c.col)}
		if m.obj == nil {
			row.Value = "The server did not answer; the record stays linked"
		} else {
			row.Value = text(differing(lr.qs, shown, lr.asked[i].values))
		}
		out = append(out, row)
	}
	return out
}

// linkedPlaces are the head of the card of a linked record: the combined
// view it belongs to (s-contacts-33, s-agenda-25, s-tasks-17, s-notes-32).
var linkedPlaces = map[view.Kind]string{
	view.KindContact: "All address books", view.KindEvent: "Combined calendar",
	view.KindTask: "All lists", view.KindNote: "All notebooks",
}

// linkedHead is the line under the title: «All address books · in 3
// books».
func (lr linkedRecord) linkedHead(k view.Kind) string {
	return linkedPlaces[k] + " · in " + strconv.Itoa(len(lr.members)) + " " + copyHolders[k][1]
}

// textFrom is where a note's text came from, when it was a choice
// (s-notes-32, «text from ▪ Записи»).
func (lr linkedRecord) textFrom() *view.FieldFrom {
	for _, q := range lr.qs {
		if q.prop() == "TEXT" {
			return lr.from(q.opts[q.chosen].holders[0].col)
		}
	}
	return nil
}

// linkedActions are the head of the card: Edit — «Which copy to edit?» —,
// and «⋯» with Print, Open in Duplicates and, under a line, Unlink
// (s-contacts-33, -36).
func (s *Server) linkedActions(sess *session.Session, segment, uid, group string) ([]view.Action, []view.MenuItem) {
	here := "/app/" + segment + "/linked/" + url.PathEscape(uid)
	edit := view.Action{ID: "record.edit", Label: "Edit", Icon: "i-pencil", Href: s.route(here + "/copies"),
		Method: "GET", Primary: true, Enabled: true, Target: render.TargetSheet}
	// Unlink is the form of the decision, the group its button's
	// (s-contacts-36).
	unlink := view.Action{ID: "duplicates.unlink", Label: "Unlink", Icon: "i-link", Method: "POST", Enabled: true,
		Href: s.postQuery("/app/duplicates/decide", url.Values{"do": {"unlink"}}), Name: "group", Value: group}
	menu := []view.MenuItem{
		{Action: view.Action{ID: "record.print", Label: "Print", Icon: "i-print", Href: s.route(here + printQuery), Method: "GET", Enabled: true}},
		{Action: view.Action{ID: "duplicates.open", Label: "Open in Duplicates", Icon: "i-dup", Href: s.route("/app/duplicates"), Method: "GET", Enabled: true}},
		{Separator: true, Action: unlink},
	}
	return []view.Action{edit}, menu
}

// linkedPage is the card of the linked record.
func (s *Server) linkedPage(sess *session.Session, spec kindSpec, lr linkedRecord) any {
	c, uid := lr.base.c, lr.base.ref.UID
	head := lr.linkedHead(spec.kind)
	shown := func(q question) []recValue { return q.shownValues() }
	actions, menu := s.linkedActions(sess, spec.segment, uid, lr.group.ID)
	switch p := s.recordPage(c, uid, lr.chosen).(type) {
	// What any copy holds is shown (Р-78, note 123 p. 10): the timeline,
	// what is kept as the server sent it, the attachments and Related —
	// those of one copy marked with its collection. Only the Source (each
	// copy has its own path) and Delete are a copy's.
	case view.ContactPage:
		p.LinkedRecord, p.Subtitle, p.Fields, p.Books = true, text(head), lr.markFrom(p.Fields), s.keptRows(lr, shown)
		p.Actions, p.Menu, p.ReadOnly, p.Source = actions, menu, false, view.SourceBlock{}
		return p
	case view.EventPage:
		p.LinkedRecord, p.Subtitle, p.Fields, p.Linked = true, text(head), lr.markFrom(p.Fields), s.keptRows(lr, shown)
		p.Actions, p.Menu, p.ReadOnly, p.Source = actions, menu, false, view.SourceBlock{}
		lr.markCopies(p.Attach, p.Related)
		return p
	case view.TaskPage:
		p.LinkedRecord, p.Subtitle, p.Fields, p.Linked = true, view.Runs{{Text: text(head)}}, lr.markFrom(p.Fields), s.keptRows(lr, shown)
		p.Actions, p.Menu, p.ReadOnly, p.Source = actions, menu, false, view.SourceBlock{}
		lr.markCopies(p.Attach, p.Related)
		return p
	case view.NotePage:
		lr.markCopies(p.Meta.Attach, p.Meta.Related)
		p.LinkedRecord, p.Subtitle, p.Meta.Linked = true, text(head), s.keptRows(lr, shown)
		if p.TextFrom = lr.textFrom(); p.TextFrom != nil {
			p.Subtitle += " · text from"
		}
		for _, q := range lr.qs {
			if q.prop() == "CATEGORIES" {
				p.Meta.TagsFrom = lr.tagsFrom(q)
			}
		}
		p.Doc.Bar, p.Meta.Actions, p.Meta.Menu, p.Source = actions, nil, menu, view.SourceBlock{}
		return p
	default:
		return p
	}
}

// getLinkedRecord is the card of a linked record, named by the UID of a
// record of it: the group of the kind holding a member with that UID.
func (s *Server) getLinkedRecord(w http.ResponseWriter, r *http.Request, kind string) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	spec, ok := recordKinds[kind]
	if !ok {
		s.notFound(w, r)
		return
	}
	g, ok := linkedByUID(s.catalogOf(sess), spec.kind, r.PathValue("uid"))
	if !ok {
		s.notFound(w, r)
		return
	}
	lr, ok := s.linkedRecordOf(r.Context(), sess, spec, g)
	if !ok {
		s.notFound(w, r)
		return
	}
	s.writePage(w, r, http.StatusOK, s.linkedPage(sess, spec, lr))
}

// linkedByUID is the linked group of the kind with a member of this UID.
func linkedByUID(blob sealed.Blob, k view.Kind, uid string) (sealed.LinkedGroup, bool) {
	for _, g := range blob.Duplicates.Linked {
		if g.Kind != k {
			continue
		}
		for _, m := range g.Members {
			if m.UID == uid {
				return g, true
			}
		}
	}
	return sealed.LinkedGroup{}, false
}

// linkedDetails is the panel of a linked row of a combined view
// (from-typecase notes 117 p. 5, 123 p. 10; s-contacts-1, s-tasks-1,
// s-notes-1): the linked record, not its first copy — «in 3 books», the
// values chosen with where they came from, «text from» of a note, Edit to
// «Which copy to edit?», where it is kept —, everything any copy holds
// (Р-78), and neither the Delete nor the Source of one copy. d is the
// panel of the row's copy, kept when the group cannot be read.
func (s *Server) linkedDetails(ctx context.Context, sess *session.Session, spec kindSpec, it listed, d view.Details) view.Details {
	ref := sealed.RecordRef{AccountID: it.from.conn.ID, Col: it.from.col.Path, UID: listedUID(spec.kind, it)}
	g, ok := dupes.LinkedOf(s.catalogOf(sess).Duplicates.Linked, ref)
	if !ok || g.Kind != spec.kind {
		return d
	}
	lr, ok := s.linkedRecordOf(ctx, sess, spec, g)
	if !ok {
		return d
	}
	out := s.recordDetails(lr.base.c, lr.base.ref.UID, lr.chosen)
	out.Close, out.LinkedRecord = d.Close, true
	out.Subtitle = text("in " + strconv.Itoa(len(lr.members)) + " " + copyHolders[spec.kind][1])
	if spec.kind == view.KindNote {
		if out.TextFrom = lr.textFrom(); out.TextFrom != nil {
			out.Subtitle += " · text from"
		}
	}
	out.Fields = lr.markFrom(out.Fields)
	lr.markCopies(out.Attach, out.Related)
	// What is kept as the server sent it stays: the panel shows it, as the
	// card does (s-contacts-1, -33; Typecase's note 125 p. 2). The source
	// and the deletion are a copy's.
	out.Source, out.DeleteNote = view.SourceBlock{}, ""
	out.Books = s.keptRows(lr, func(q question) []recValue { return q.shownValues() })
	actions, menu := s.linkedActions(sess, spec.segment, lr.base.ref.UID, g.ID)
	// The panel's Edit is a small button, as its neighbours (s-contacts-1).
	edit := actions[0]
	edit.Primary, edit.Icon = false, ""
	kept := []view.Action{edit}
	for _, a := range out.Actions {
		if a.ID == "contact.timeline" || a.ID == "record.open" {
			kept = append(kept, a)
		}
	}
	out.Actions, out.Menu = kept, menu
	return out
}

// markCopies names, after an attachment and a Related row of the linked
// record that only some copies hold, the collection of the first that does
// (Р-78: «DocAttach.From», «DocRef.From»).
func (lr linkedRecord) markCopies(attach []view.AttachRow, related []view.TimelineRow) {
	var loaded []linkedMember
	for _, m := range lr.members {
		if m.obj != nil {
			loaded = append(loaded, m)
		}
	}
	if len(loaded) < 2 {
		return
	}
	mark := func(holds func(*model.Object) bool) *view.FieldFrom {
		var first *linkedMember
		n := 0
		for i := range loaded {
			if holds(loaded[i].obj) {
				if first == nil {
					first = &loaded[i]
				}
				n++
			}
		}
		if first == nil || n == len(loaded) {
			return nil
		}
		return lr.from(collectionLabel(first.c.col))
	}
	for i := range attach {
		name := string(attach[i].Name)
		attach[i].From = mark(func(o *model.Object) bool {
			for _, a := range cal.Attachments(o) {
				base, err := url.PathUnescape(path.Base(a.URI))
				if a.FileName == name || (a.URI != "" && err == nil && base == name) {
					return true
				}
			}
			return false
		})
	}
	targets := cal.RelatedTo(lr.chosen)
	for i := range related {
		if i >= len(targets) {
			break
		}
		target := targets[i]
		related[i].From = mark(func(o *model.Object) bool { return slices.Contains(cal.RelatedTo(o), target) })
	}
}

// combinedBase reports whether a list's address is a combined view's —
// «/app/contacts» —, not one collection's.
func combinedBase(base string) bool {
	return strings.Count(strings.Trim(base, "/"), "/") == 1
}
