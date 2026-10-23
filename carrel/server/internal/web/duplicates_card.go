package web

import (
	"context"
	"slices"
	"sort"
	"strings"

	"carrel.dev/carrel/server/internal/dupes"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// The card of a linked record (S-16) opened from its collection is that
// copy, with what the other members of its group hold differently marked
// and each member a row under it — where it lives and what it holds
// differently, one link away (Typecase's Р-58). The others are read from
// their servers when the card is opened — the card is a read, as the card of
// one record is; finding duplicates is what asks nobody. A member its server
// says is not there leaves the group silently; a server that does not answer
// is said in its row, and its member stays.

// linkedMember is another member of the opened record's group.
type linkedMember struct {
	ref sealed.RecordRef
	c   *collectionCtx
	obj *model.Object // nil when its server did not answer
}

// linkedCard is the group of the opened record as the card needs it.
type linkedCard struct {
	group  sealed.LinkedGroup
	self   sealed.RecordRef
	others []linkedMember
	// apart is the notice of a group found come apart as the card opened:
	// every other member gone (Б16). nil — it did not come apart now.
	apart *view.Notice
}

// linkedOf reads the other members of the group holding the opened record.
// False when the record is in no linked group, or none of the others is
// there any more.
func (s *Server) linkedOf(ctx context.Context, c *collectionCtx, uid string) (linkedCard, bool) {
	self := sealed.RecordRef{AccountID: c.conn.ID, Col: c.col.Path, UID: uid}
	g, ok := dupes.LinkedOf(s.catalogOf(c.sess).Duplicates.Linked, self)
	if !ok || g.Kind != c.spec.kind {
		return linkedCard{}, false
	}
	lc := linkedCard{group: g, self: self}
	others, gone := s.readMembers(ctx, c.sess, c.spec, g, self)
	lc.others = others
	if len(gone) > 0 {
		s.forgetMembers(c.sess, "", gone)
		if len(lc.others) == 0 {
			lc.apart = apartNotice(s.catalogOf(c.sess), gone)
		}
	}
	return lc, len(lc.others) > 0
}

// apartNotice says that a record's linked group came apart because its
// other members are gone (Б16 of the audit of 11.10.2026: it used to come
// apart without a word): where they were, and that the record stands on its
// own now.
func apartNotice(blob sealed.Blob, gone []sealed.RecordRef) *view.Notice {
	var where []string
	for _, ref := range gone {
		conn, okConn := blob.Connection(ref.AccountID)
		col, okCol := blob.Collection(ref.AccountID, ref.Col)
		if okConn && okCol {
			where = append(where, connLabel(conn)+" · "+collectionLabel(col))
		}
	}
	body := "Its other copy is no longer where it was linked; this record stands on its own."
	if len(where) > 0 {
		body = "Its other copy in " + strings.Join(where, ", ") + " is no longer there; this record stands on its own."
	}
	return &view.Notice{Role: view.NoticeInfo, Title: "No longer linked", Body: text(body)}
}

// memberRows are the rows under a copy's card: every other member with
// where it lives and what it holds otherwise than this copy, by the
// group's questions (from-typecase note 117 p. 4: «differ» is relative to
// what the card shows), or that its server did not answer.
func (s *Server) memberRows(lc linkedCard, lr linkedRecord) []view.Field {
	own := lr.valuesAt(lc.self)
	mine := func(q question) []recValue { return own[q.id] }
	var out []view.Field
	for _, m := range lc.others {
		row := view.Field{Label: text(string(m.c.stamp())), Href: s.route(m.c.recordPath(m.ref.UID)), Colour: catalogColour(m.c.col)}
		switch d := differing(lr.qs, mine, lr.valuesAt(m.ref)); {
		case m.obj == nil:
			row.Value = "The server did not answer; the record stays linked"
		case d != "":
			row.Value = text(d)
		default:
			row.Value = "Holds the same"
		}
		out = append(out, row)
	}
	return out
}

// valueSet is every value of the properties, for comparison.
func valueSet(o *model.Object, props ...string) string {
	var vals []string
	for _, p := range props {
		for _, v := range o.PropText(p) {
			vals = append(vals, p+"\x00"+strings.TrimSpace(v))
		}
	}
	sort.Strings(vals)
	return strings.Join(vals, "\n")
}

type differProp struct {
	label string
	props []string
}

var contactDiffers = []differProp{
	{"name", []string{"FN"}}, {"phone", []string{"TEL"}}, {"email", []string{"EMAIL"}},
	{"address", []string{"ADR"}}, {"organisation", []string{"ORG"}}, {"birthday", []string{"BDAY"}},
	{"photo", []string{"PHOTO"}}, {"note", []string{"NOTE"}},
}

var calendarDiffers = []differProp{
	{"title", []string{"SUMMARY"}}, {"time", []string{"DTSTART", "DTEND", "DURATION", "DUE", "RRULE"}},
	{"where", []string{"LOCATION"}}, {"priority", []string{"PRIORITY"}}, {"text", []string{"DESCRIPTION"}},
	{"tags", []string{"CATEGORIES"}},
}

func differs(table []differProp) func(a, b *model.Object) []string {
	return func(a, b *model.Object) []string {
		var out []string
		for _, d := range table {
			if valueSet(a, d.props...) != valueSet(b, d.props...) {
				out = append(out, d.label)
			}
		}
		if len(out) > 0 {
			out[0] = strings.ToUpper(out[0][:1]) + out[0][1:]
		}
		return out
	}
}

// The card a collection opens (Typecase's Р-58, task 153; the owner's
// decisions of 13.10.2026) shows that collection's copy — what its edit
// opens —, never the values of another: a field where a choice could be
// made, one another copy holds differently, carries the badge «linked»
// (s-contacts-6, s-agenda-2, s-tasks-2, s-notes-2). The card of the entity —
// the values the person chose and where each came from (s-contacts-33) — is
// another address, opened from the combined list.

// linkedMark is the badge of a field another copy holds differently.
var linkedMark = view.Badge{Label: "linked", Role: view.BadgeLinked}

// linkedContact is the card of a linked contact opened from its book: its
// own fields, the badge where another copy differs, and the rows of the
// others.
func (s *Server) linkedContact(ctx context.Context, c *collectionCtx, uid string, obj *model.Object, page *view.ContactPage) {
	lc, ok := s.linkedOf(ctx, c, uid)
	if lc.apart != nil {
		page.Shell.Notices = append(page.Shell.Notices, *lc.apart)
	}
	if !ok {
		return
	}
	lr, _ := linkedRecordFrom(c, lc, obj)
	page.Fields = markAsked(page.Fields, lr.qs)
	page.Books = s.memberRows(lc, lr)
	s.editViaCopies(c, uid, page.Actions)
}

// linkedCalendar is what a linked event, task or note opened from its
// collection shows of its group: the badge on the fields another copy holds
// differently (fields), whether another copy holds other tags than a note's
// own (tags) — a note's tags are its column's, not a field —, the rows of
// the others.
func (s *Server) linkedCalendar(ctx context.Context, c *collectionCtx, uid string, obj *model.Object) (fields func(own []view.Field) []view.Field, tags func(own []view.Tag) bool, rows []view.Field, apart *view.Notice, ok bool) {
	lc, ok := s.linkedOf(ctx, c, uid)
	if !ok {
		return nil, nil, nil, lc.apart, false
	}
	lr, _ := linkedRecordFrom(c, lc, obj)
	fields = func(own []view.Field) []view.Field { return markAsked(own, lr.qs) }
	// A note's tags are its column's, not a field: they carry the badge
	// when they were asked about.
	tags = func(own []view.Tag) bool {
		return len(own) > 0 && slices.ContainsFunc(lr.qs, func(q question) bool { return q.prop() == "CATEGORIES" })
	}
	return fields, tags, s.memberRows(lc, lr), nil, true
}
