package web

import (
	"context"
	"sort"
	"strings"

	"carrel.dev/carrel/server/internal/dupes"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// The card of a linked record (S-16) is the card of the entity: the record
// opened, and what the other members of its group add. The others are read
// from their servers when the card is opened — the card is a read of the
// entity, as the card of one record is a read of it; finding duplicates is
// what asks nobody. Each member is a row under the card with where it lives
// and what it holds differently, one link away. A member its server says is
// not there leaves the group silently; a server that does not answer is said
// in its row, and its member stays.

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

// preferred is the record whose single-valued prop the card shows: the
// member the person chose when linking, when it answered; else the opened
// one if it holds the property; else the first other that does — a field
// only one member has is taken from it (S-16).
func (lc linkedCard) preferred(prop string, own *model.Object) *model.Object {
	if ref, ok := lc.group.Prefs[prop]; ok {
		if ref == lc.self {
			return own
		}
		for _, m := range lc.others {
			if m.ref == ref && m.obj != nil {
				return m.obj
			}
		}
	}
	if len(own.PropText(prop)) > 0 {
		return own
	}
	for _, o := range lc.answered() {
		if len(o.PropText(prop)) > 0 {
			return o
		}
	}
	return own
}

// answered are the others that answered.
func (lc linkedCard) answered() []*model.Object {
	var out []*model.Object
	for _, m := range lc.others {
		if m.obj != nil {
			out = append(out, m.obj)
		}
	}
	return out
}

// mergeFields adds to own the fields of the others it does not show: the
// union without exact repeats, as linking promises (S-16).
func mergeFields(own []view.Field, others [][]view.Field) []view.Field {
	seen := map[string]bool{}
	for _, f := range own {
		seen[string(f.Label)+"\x00"+string(f.Value)] = true
	}
	out := append([]view.Field(nil), own...)
	for _, fs := range others {
		for _, f := range fs {
			key := string(f.Label) + "\x00" + string(f.Value)
			if !seen[key] {
				seen[key] = true
				out = append(out, f)
			}
		}
	}
	return out
}

// memberRows are the rows under the card: every other member with where it
// lives and what it holds differently, or that its server did not answer.
func (s *Server) memberRows(lc linkedCard, own *model.Object, differ func(a, b *model.Object) []string) []view.Field {
	var out []view.Field
	for _, m := range lc.others {
		row := view.Field{Label: text(string(m.c.stamp())), Href: s.route(m.c.recordPath(m.ref.UID))}
		switch {
		case m.obj == nil:
			row.Value = "The server did not answer; the record stays linked"
		default:
			if d := differ(own, m.obj); len(d) > 0 {
				row.Value = text(strings.Join(d, ", ") + " differ")
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
	{"location", []string{"LOCATION"}}, {"text", []string{"DESCRIPTION"}}, {"categories", []string{"CATEGORIES"}},
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

// linkedContact is the card of a linked contact: the chosen name and
// organisation, the fields of all members, the chosen photo, and the rows of
// the others.
func (s *Server) linkedContact(ctx context.Context, c *collectionCtx, uid string, obj *model.Object, page *view.ContactPage) {
	lc, ok := s.linkedOf(ctx, c, uid)
	if !ok {
		return
	}
	name := contactName(lc.preferred("FN", obj))
	page.Shell.Title, page.Name = text(name), text(name)
	page.Avatar.Initials = avatarFor(uid, name).Initials
	if p := lc.preferred("PHOTO", obj); p != obj {
		for _, m := range lc.others {
			if m.obj == p {
				page.Avatar.Href, page.PhotoHref = s.cardPhoto(m.c, m.ref.UID, m.obj)
			}
		}
	}
	// Where the person works and as what is the head of the card, and each
	// half comes from the member the person chose (s-contacts-6, S-16).
	page.Subtitle = text(contactSubtitleOf(lc.preferred("ORG", obj), lc.preferred("TITLE", obj)))
	fieldsOf := func(o *model.Object) []view.Field {
		var out []view.Field
		for _, f := range contactFieldsOf(o) {
			if f.prop == "ORG" || f.prop == "TITLE" {
				continue
			}
			out = append(out, f.Field)
		}
		return out
	}
	var more [][]view.Field
	for _, o := range lc.answered() {
		more = append(more, fieldsOf(o))
	}
	page.Fields = mergeFields(fieldsOf(obj), more)
	page.Books = s.memberRows(lc, obj, differs(contactDiffers))
}

// linkedCalendar is what a linked event, task or note shows of its group:
// the chosen title and place, the fields of all members, the rows of the
// others. Time, repetition and reminders are the opened record's own.
func (s *Server) linkedCalendar(ctx context.Context, c *collectionCtx, uid string, obj *model.Object) (title string, fields func(own []view.Field) []view.Field, rows []view.Field, ok bool) {
	lc, ok := s.linkedOf(ctx, c, uid)
	if !ok {
		return "", nil, nil, false
	}
	title = cal.Text(lc.preferred("SUMMARY", obj), "SUMMARY")
	place := lc.preferred("LOCATION", obj)
	fields = func(own []view.Field) []view.Field {
		keep := func(o *model.Object, fs []view.Field) []view.Field {
			var out []view.Field
			for _, f := range fs {
				if f.Label == "Location" && o != place {
					continue
				}
				out = append(out, f)
			}
			return out
		}
		base := keep(obj, own)
		if place != obj {
			if v := cal.Text(place, "LOCATION"); v != "" {
				base = append(base, view.Field{Label: "Location", Value: text(v)})
			}
		}
		var more [][]view.Field
		for _, o := range lc.answered() {
			more = append(more, keep(o, calendarFields(o)))
		}
		return mergeFields(base, more)
	}
	return title, fields, s.memberRows(lc, obj, differs(calendarDiffers)), true
}
