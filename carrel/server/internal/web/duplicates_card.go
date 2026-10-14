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
	var gone []sealed.RecordRef
	for _, ref := range g.Members {
		if ref == self {
			continue
		}
		mc, ok := s.collectionOf(c.sess, c.spec, ref.AccountID, ref.Col)
		if !ok {
			gone = append(gone, ref)
			continue
		}
		obj, err := mc.find(ctx, ref.UID)
		switch {
		case notFoundErr(err):
			gone = append(gone, ref)
		case err != nil:
			lc.others = append(lc.others, linkedMember{ref: ref, c: mc})
		default:
			lc.others = append(lc.others, linkedMember{ref: ref, c: mc, obj: obj})
		}
	}
	if len(gone) > 0 {
		s.forgetMembers(c.sess, "", gone)
	}
	return lc, len(lc.others) > 0
}

// memberRows are the rows under the card: every other member with where it
// lives and what it holds differently, or that its server did not answer.
func (s *Server) memberRows(lc linkedCard, own *model.Object, differ func(a, b *model.Object) []string) []view.Field {
	var out []view.Field
	for _, m := range lc.others {
		row := view.Field{Label: text(string(m.c.stamp())), Href: s.route(m.c.recordPath(m.ref.UID)), Colour: catalogColour(m.c.col)}
		switch {
		case m.obj == nil:
			row.Value = "The server did not answer; the record stays linked"
		default:
			if d := differ(own, m.obj); len(d) > 0 {
				// «Phone, email differ», capital first (s-contacts-6).
				why := strings.Join(d, ", ") + " differ"
				row.Value = text(strings.ToUpper(why[:1]) + why[1:])
			} else {
				row.Value = "Holds the same"
			}
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

// fieldKey is what a field shows, for comparing copies: its words, not
// where its tags lead — each copy's tags lead into its own collection.
func fieldKey(f view.Field) string {
	parts := []string{string(f.Value)}
	for _, r := range f.Runs {
		parts = append(parts, string(r.Text))
	}
	for _, t := range f.Tags {
		parts = append(parts, string(t.Label))
	}
	return strings.Join(parts, "\x00")
}

// markDiffering gives the badge to each field of own that another copy
// holds under the same label with other values. A field another copy does
// not have is no choice — linking takes it as it is (S-16) —, and a field
// with a badge of its own (the zone it was made in, the end of a series)
// keeps it.
func markDiffering(own []view.Field, others [][]view.Field) []view.Field {
	out := append([]view.Field(nil), own...)
	for i, f := range out {
		if f.Badge.Label != "" {
			continue
		}
		key := fieldKey(f)
		for _, fs := range others {
			var held []string
			for _, o := range fs {
				if o.Label == f.Label {
					held = append(held, fieldKey(o))
				}
			}
			if len(held) > 0 && !slices.Contains(held, key) {
				out[i].Badge = linkedMark
				break
			}
		}
	}
	return out
}

// otherPages are the reading pages of the copies that answered, as their
// collections show them.
func (s *Server) otherPages(lc linkedCard) []any {
	var out []any
	for _, m := range lc.others {
		if m.obj != nil {
			out = append(out, s.recordPage(m.c, m.ref.UID, m.obj))
		}
	}
	return out
}

// linkedContact is the card of a linked contact opened from its book: its
// own fields, the badge where another copy differs, and the rows of the
// others.
func (s *Server) linkedContact(ctx context.Context, c *collectionCtx, uid string, obj *model.Object, page *view.ContactPage) {
	lc, ok := s.linkedOf(ctx, c, uid)
	if !ok {
		return
	}
	var others [][]view.Field
	for _, p := range s.otherPages(lc) {
		if o, ok := p.(view.ContactPage); ok {
			others = append(others, o.Fields)
		}
	}
	page.Fields = markDiffering(page.Fields, others)
	page.Books = s.memberRows(lc, obj, differs(contactDiffers))
}

// linkedCalendar is what a linked event, task or note opened from its
// collection shows of its group: the badge on the fields another copy holds
// differently (fields), whether another copy holds other tags than a note's
// own (tags) — a note's tags are its column's, not a field —, the rows of
// the others.
func (s *Server) linkedCalendar(ctx context.Context, c *collectionCtx, uid string, obj *model.Object) (fields func(own []view.Field) []view.Field, tags func(own []view.Tag) bool, rows []view.Field, ok bool) {
	lc, ok := s.linkedOf(ctx, c, uid)
	if !ok {
		return nil, nil, nil, false
	}
	var others [][]view.Field
	var held []string
	for _, p := range s.otherPages(lc) {
		switch o := p.(type) {
		case view.EventPage:
			others = append(others, o.Fields)
		case view.TaskPage:
			others = append(others, o.Fields)
		case view.NotePage:
			if len(o.Meta.Tags) > 0 {
				held = append(held, tagSet(o.Meta.Tags))
			}
		}
	}
	fields = func(own []view.Field) []view.Field { return markDiffering(own, others) }
	tags = func(own []view.Tag) bool {
		return len(own) > 0 && slices.ContainsFunc(held, func(h string) bool { return h != tagSet(own) })
	}
	return fields, tags, s.memberRows(lc, obj, differs(calendarDiffers)), true
}

// tagSet is the labels of the tags, sorted, for comparing copies.
func tagSet(tags []view.Tag) string {
	labels := make([]string, 0, len(tags))
	for _, t := range tags {
		labels = append(labels, string(t.Label))
	}
	sort.Strings(labels)
	return strings.Join(labels, "\x00")
}
