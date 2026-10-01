package web

import (
	"context"
	"net/http"

	"github.com/emersion/go-ical"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/view"
)

// S-19: links inside an imported file travel with it; links from outside it
// are not touched. A record of the file may name another of the same file by
// UID — RELATED-TO in a calendar, RELATED, MEMBER or Apple's
// X-ADDRESSBOOKSERVER-MEMBER on a card, bare or as urn:uuid:. When the one
// named gets another identity — from Carrel, because its own was taken, or
// from a server that writes its own (SV-25) — the links to it inside the
// import name the new one. Records already on the servers that name the old
// identity are left alone: where that identity came from is not known, and a
// card of it may live on elsewhere (owner's decision, 24.09.2026). A value
// is read as its form and its UID by splitUIDRef (uidref.go).

// linkProps are the properties by which a record of dest's kind names
// another by UID.
func linkProps(dest *collectionCtx) []string {
	if dest.card != nil {
		return []string{"RELATED", "MEMBER", "X-ADDRESSBOOKSERVER-MEMBER"}
	}
	return []string{ical.PropRelatedTo}
}

// planImportUIDs settles the identity of every record before the first
// write: its own, or a fresh one where the destination or an earlier record
// of the file already holds it, or where it has none. moved is what links
// inside the file must follow: an identity of the file that no record of it
// keeps, to the fresh identity of the first record that had it.
func planImportUIDs(recs []importRecord, taken map[string]bool) (uids []string, errs []error, moved map[string]string) {
	uids, errs = make([]string, len(recs)), make([]error, len(recs))
	moved = map[string]string{}
	held := make(map[string]bool, len(taken)+len(recs))
	for u := range taken {
		held[u] = true
	}
	kept := map[string]bool{}
	for i, rec := range recs {
		uid := rec.UID
		if uid == "" || held[uid] {
			fresh, err := model.NewUID()
			if err != nil {
				errs[i] = err
				continue
			}
			if _, first := moved[rec.UID]; rec.UID != "" && !first {
				moved[rec.UID] = fresh
			}
			uid = fresh
		} else {
			kept[uid] = true
		}
		uids[i], held[uid] = uid, true
	}
	for from := range moved {
		if kept[from] {
			delete(moved, from) // a record of the file keeps it: links to it stand
		}
	}
	return uids, errs, moved
}

// linkPatch points the values of props that name a key of moved at its new
// identity, keeping the form they were written in, or false when obj names
// none of them.
func linkPatch(obj *model.Object, props []string, moved map[string]string) (*model.Patch, bool) {
	if len(moved) == 0 {
		return nil, false
	}
	all := obj.Props()
	var ops []model.PatchOp
	for _, prop := range props {
		vals := all[prop]
		texts := make([]string, len(vals))
		changed := false
		for i, v := range vals {
			texts[i] = v.Text
			prefix, target := splitUIDRef(v.Text)
			if to, ok := moved[target]; ok {
				texts[i], changed = prefix+to, true
			}
		}
		if !changed {
			continue
		}
		ops = append(ops, model.SetValues{Prop: prop, Values: texts})
		for i, v := range vals {
			if texts[i] == v.Text {
				continue // the same value: it keeps its parameters
			}
			for name, value := range v.Params {
				ops = append(ops, model.SetParam{Prop: prop, Index: i, Param: name, Value: value})
			}
		}
	}
	if len(ops) == 0 {
		return nil, false
	}
	p, err := model.NewPatch(ops...)
	if err != nil {
		return nil, false
	}
	return p, true
}

// followTheirs points the links inside an import at the identities a server
// gave its records after they were written (SV-25). theirs maps the identity
// a record was written under to the one it has. Only records whose links
// name one of them are read again and written, with If-Match; the row says
// what became of it.
func followTheirs(ctx context.Context, dest *collectionCtx, written [][]view.Text, made []importMade, theirs map[string]string) {
	if len(theirs) == 0 {
		return
	}
	props := linkProps(dest)
	for _, m := range made {
		if _, ok := linkPatch(m.obj, props, theirs); !ok {
			continue
		}
		now := m.uid
		if t, ok := theirs[m.uid]; ok {
			now = t
		}
		cur, err := dest.find(ctx, now)
		if err == nil {
			p, ok := linkPatch(cur, props, theirs)
			if !ok {
				continue // the server already names the new identities
			}
			if err = p.Apply(cur); err == nil {
				err = dest.update(ctx, cur)
			}
		}
		if err != nil {
			why := err.Error()
			if dav.IsStatus(err, http.StatusPreconditionFailed) {
				why = "it changed on its server after Carrel read it"
			}
			written[m.row][2] += view.Text(". Its links to records of this file still name the identities they were sent under: " + why)
			continue
		}
		dest.noteWrite(cur.Path, cur)
		written[m.row][2] += view.Text(". Its links to records of this file follow the identities the server gave them")
	}
}
