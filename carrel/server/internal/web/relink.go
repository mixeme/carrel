package web

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/emersion/go-ical"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/fanout"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// A task or a note made from a person's timeline is tied to the person by one
// thing only: RELATED-TO, holding the UID of their card (S-15.4). An event is
// tied by the person's address in ATTENDEE as well, so an event survives what
// follows; a task and a note do not. Three things Carrel does change the UID a
// card is known by:
//
//   - a card moved to another book of SoGo comes out with a UID of the
//     server's own (docs/servers.md, SV-18);
//   - a card written to Google gets its UID rewritten (SV-25);
//   - a merge of duplicates keeps one card and deletes the others (S-16).
//
// Each time the records that pointed at the old UID point at nothing, and
// drop out of the person's timeline without a word. The owner caught it
// (24.09.2026) and decided where to mend it: in the ticked collections, the
// very ones the timeline reads, so what it would have lost is exactly what
// is put back.
//
// A record is read whole, not by window: a link has to be whole whether or
// not the timeline's range reaches it today. Every value of RELATED-TO that
// named an old UID is made to name the new one, keeping its parameters —
// RELTYPE=SIBLING stays a sibling; a bare value would read as PARENT, which
// is another relation. A record that already points at the new UID loses
// the old line instead of gaining a second. Each write is conditional on the
// version read (S-8).

// relinkReport is what the mending did, for the notice of the operation that
// needed it.
type relinkReport struct {
	changed int
	skipped []string // a record that points at an old UID and was not changed, with why
	unread  []string // a collection that could not be read, so was not looked in
}

func (r relinkReport) nothing() bool {
	return r.changed == 0 && len(r.skipped) == 0 && len(r.unread) == 0
}

// lines are the report for a notice's details.
func (r relinkReport) lines() []view.Text {
	var out []view.Text
	if r.changed > 0 {
		out = append(out, text("Now pointing at the kept card: "+plural(r.changed, "linked record")))
	}
	for _, s := range r.skipped {
		out = append(out, text("Still pointing at the old card: "+s))
	}
	for _, u := range r.unread {
		out = append(out, text("Not looked in: "+u))
	}
	if len(out) > 0 {
		out = append(out, text("Not looked in: collections that are not ticked. A link in one of them still points at the old card."))
	}
	return out
}

// sentence is the report in one line for a notice body, or "".
func (r relinkReport) sentence() string {
	switch {
	case r.nothing():
		return ""
	case len(r.skipped) == 0 && len(r.unread) == 0:
		if r.changed == 1 {
			return " The one record linked to it now points at the card that stays."
		}
		return " " + plural(r.changed, "record") + " linked to it now point at the card that stays."
	default:
		return " Some notes and tasks linked to it could not be pointed at the card that stays; the details say which."
	}
}

// relink points every RELATED-TO naming one of from at to, across the ticked
// collections.
func (s *Server) relink(ctx context.Context, sess *session.Session, blob sealed.Blob, from []string, to string) relinkReport {
	old := map[string]bool{}
	for _, u := range from {
		if u != "" && u != to {
			old[u] = true
		}
	}
	var rep relinkReport
	if len(old) == 0 || to == "" {
		return rep
	}
	for _, c := range s.tickedCollections(sess, blob) {
		label := collectionLabel(c.col) + " · " + connLabel(c.conn)
		if c.broken != nil {
			rep.unread = append(rep.unread, label+" — "+c.broken.Error())
			continue
		}
		for _, k := range c.kinds {
			if k == view.KindContact {
				s.relinkCards(ctx, sess, c, label, old, to, &rep)
				continue
			}
			if k != view.KindEvent && k != view.KindTask && k != view.KindNote {
				continue
			}
			items, err := c.records(ctx, k, fanout.Query{})
			if err != nil && !errors.Is(err, fanout.ErrIncomplete) {
				rep.unread = append(rep.unread, label+" — "+err.Error())
				continue
			}
			for _, it := range items {
				obj := it.Object
				if obj == nil {
					continue
				}
				patch, ok := relinkPatch(obj, old, to)
				if !ok {
					continue
				}
				what := titleOr(cal.Text(obj, ical.PropSummary), "Untitled") + " · " + label
				if c.col.ReadOnly || c.feed != nil || c.cal == nil {
					rep.skipped = append(rep.skipped, what+" — the collection is read-only")
					continue
				}
				if err := patch.Apply(obj); err != nil {
					rep.skipped = append(rep.skipped, what+" — "+err.Error())
					continue
				}
				if _, err := c.cal.Update(ctx, obj); err != nil {
					why := err.Error()
					if dav.IsStatus(err, http.StatusPreconditionFailed) {
						why = "it changed on its server after Carrel read it"
					}
					rep.skipped = append(rep.skipped, what+" — "+why)
					continue
				}
				sess.Cache().InvalidatePath(c.conn.ID, obj.Path)
				dropWholeReads(sess, c.conn.ID, c.col.Path)
				rep.changed++
			}
		}
	}
	return rep
}

// relinkPatch is the patch that points obj's RELATED-TO from old at to, or
// false when obj points at none of old.
func relinkPatch(obj *model.Object, old map[string]bool, to string) (*model.Patch, bool) {
	vals := obj.Props()[ical.PropRelatedTo]
	hasTo := false
	for _, v := range vals {
		if v.Text == to {
			hasTo = true
		}
	}
	var texts []string
	var keep []map[string]string // the parameters of a value whose text changed
	changed := false
	for _, v := range vals {
		switch {
		case old[v.Text] && hasTo:
			changed = true // folded into the line that already names to
		case old[v.Text]:
			texts = append(texts, to)
			keep = append(keep, v.Params)
			changed, hasTo = true, true
		default:
			texts = append(texts, v.Text)
			keep = append(keep, nil)
		}
	}
	if !changed {
		return nil, false
	}
	ops := []model.PatchOp{model.SetValues{Prop: ical.PropRelatedTo, Values: texts}}
	for i, params := range keep {
		for name, value := range params {
			ops = append(ops, model.SetParam{Prop: ical.PropRelatedTo, Index: i, Param: name, Value: value})
		}
	}
	p, err := model.NewPatch(ops...)
	if err != nil {
		return nil, false
	}
	return p, true
}

// relinkCards mends the links of the cards of a ticked book the way relink
// mends RELATED-TO (Б33 of the audit of 11.10.2026): RELATED of a person
// and MEMBER of a group that named an old UID — bare or as urn:uuid: — name
// the kept one, written the same way and with their parameters; a card that
// already names it loses the old line instead of gaining a second.
func (s *Server) relinkCards(ctx context.Context, sess *session.Session, c *collectionSource, label string, old map[string]bool, to string, rep *relinkReport) {
	items, err := c.contacts(ctx, fanout.Query{})
	if err != nil && !errors.Is(err, fanout.ErrIncomplete) {
		rep.unread = append(rep.unread, label+" — "+err.Error())
		return
	}
	for _, it := range items {
		obj := it.Object
		if obj == nil {
			continue
		}
		var ops []model.PatchOp
		for _, prop := range []string{"RELATED", "MEMBER"} {
			ops = append(ops, cardRelinkOps(obj, prop, old, to)...)
		}
		if len(ops) == 0 {
			continue
		}
		what := contactName(obj) + " · " + label
		if c.col.ReadOnly || c.feed != nil || c.card == nil {
			rep.skipped = append(rep.skipped, what+" — the book is read-only")
			continue
		}
		patch, err := model.NewPatch(ops...)
		if err == nil {
			err = patch.Apply(obj)
		}
		if err != nil {
			rep.skipped = append(rep.skipped, what+" — "+err.Error())
			continue
		}
		if _, err := c.card.Update(ctx, obj); err != nil {
			why := err.Error()
			if dav.IsStatus(err, http.StatusPreconditionFailed) {
				why = "it changed on its server after Carrel read it"
			}
			rep.skipped = append(rep.skipped, what+" — "+why)
			continue
		}
		sess.Cache().InvalidatePath(c.conn.ID, obj.Path)
		dropWholeReads(sess, c.conn.ID, c.col.Path)
		rep.changed++
	}
}

// cardRelinkOps are the patch of one property of a card that names an old
// UID — «urn:uuid:<uid>» or the UID alone —, pointed at to and written the
// same way; nil when it names none.
func cardRelinkOps(obj *model.Object, prop string, old map[string]bool, to string) []model.PatchOp {
	vals := obj.Props()[prop]
	const urn = "urn:uuid:"
	target := func(v string) (string, bool) {
		bare := strings.TrimPrefix(v, urn)
		if !old[bare] {
			return v, false
		}
		if strings.HasPrefix(v, urn) {
			return urn + to, true
		}
		return to, true
	}
	names := func(v string) bool { return strings.TrimPrefix(v, urn) == to }
	has := false
	for _, v := range vals {
		has = has || names(v.Text)
	}
	var texts []string
	var keep []map[string]string
	changed := false
	for _, v := range vals {
		next, moved := target(v.Text)
		switch {
		case moved && has:
			changed = true
		case moved:
			texts, keep = append(texts, next), append(keep, v.Params)
			changed, has = true, true
		default:
			texts, keep = append(texts, v.Text), append(keep, v.Params)
		}
	}
	if !changed {
		return nil
	}
	ops := []model.PatchOp{model.SetValues{Prop: prop, Values: texts}}
	for i, params := range keep {
		for name, value := range params {
			ops = append(ops, model.SetParam{Prop: prop, Index: i, Param: name, Value: value})
		}
	}
	return ops
}
