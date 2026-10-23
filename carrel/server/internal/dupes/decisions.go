package dupes

import (
	"slices"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// A member of a group is not stable: another client may delete or move the
// record. Such a member leaves its linked group silently, and a group left
// with one member is no group (S-16). Only a successful answer says that a
// record is gone: a collection whose server did not answer is simply not in
// present, and its members stay — silence of a failed source is not a
// deletion, or groups would come apart whenever a server is down.

// CollectionRef is the records of one kind in one collection, as a member
// names them: a calendar collection read for its tasks says nothing about its
// notes.
type CollectionRef struct {
	AccountID string
	Col       string
	Kind      view.Kind
}

// Prune drops from the linked groups every member whose collection is in
// present, for the group's kind, without that member's UID, and every group
// left with fewer than two. changed says whether anything went.
func Prune(linked []sealed.LinkedGroup, present map[CollectionRef]map[string]bool) ([]sealed.LinkedGroup, bool) {
	changed := false
	out := make([]sealed.LinkedGroup, 0, len(linked))
	for _, g := range linked {
		var kept []sealed.RecordRef
		for _, m := range g.Members {
			uids, known := present[CollectionRef{m.AccountID, m.Col, g.Kind}]
			if known && !uids[m.UID] {
				changed = true
				continue
			}
			kept = append(kept, m)
		}
		if len(kept) < 2 {
			changed = true
			continue
		}
		g.Members = kept
		out = append(out, g)
	}
	return out, changed
}

// Link joins refs into one linked group. Linked groups that share a member
// with refs are absorbed into it, answers and all: a record is one entity
// with one set of others, never in two groups at once. An answer given with
// g wins over one the absorbed groups held — a property's dropped values
// too, as one answer.
func Link(linked []sealed.LinkedGroup, g sealed.LinkedGroup) []sealed.LinkedGroup {
	members := append([]sealed.RecordRef(nil), g.Members...)
	in := map[sealed.RecordRef]bool{}
	for _, m := range members {
		in[m] = true
	}
	prefs := map[string]sealed.RecordRef{}
	dropped := map[string][]string{}
	var out []sealed.LinkedGroup
	for _, have := range linked {
		overlap := false
		for _, m := range have.Members {
			if in[m] {
				overlap = true
				break
			}
		}
		if !overlap || have.Kind != g.Kind {
			out = append(out, have)
			continue
		}
		for _, m := range have.Members {
			if !in[m] {
				in[m] = true
				members = append(members, m)
			}
		}
		for prop, ref := range have.Prefs {
			prefs[prop] = ref
		}
		for prop, keys := range have.Dropped {
			for _, k := range keys {
				if !slices.Contains(dropped[prop], k) {
					dropped[prop] = append(dropped[prop], k)
				}
			}
		}
	}
	for prop, ref := range g.Prefs {
		prefs[prop] = ref // the answer given now wins over an older one
	}
	for prop, keys := range g.Dropped {
		dropped[prop] = keys
	}
	g.Prefs, g.Dropped = nil, nil
	if len(prefs) > 0 {
		g.Prefs = prefs
	}
	if len(dropped) > 0 {
		g.Dropped = dropped
	}
	g.Members = members
	g.ID = Signature(g.Kind, members)
	return append(out, g)
}

// LinkedOf is the linked group holding ref, if any.
func LinkedOf(linked []sealed.LinkedGroup, ref sealed.RecordRef) (sealed.LinkedGroup, bool) {
	for _, g := range linked {
		for _, m := range g.Members {
			if m == ref {
				return g, true
			}
		}
	}
	return sealed.LinkedGroup{}, false
}

// Covered reports whether every member of a found group is in one linked
// group: then it is shown as that group, not offered again.
func Covered(linked []sealed.LinkedGroup, g Group) bool {
	if len(g.Members) == 0 {
		return false
	}
	first, ok := LinkedOf(linked, g.Members[0].Ref)
	if !ok {
		return false
	}
	for _, m := range g.Members[1:] {
		other, ok := LinkedOf(linked, m.Ref)
		if !ok || other.ID != first.ID {
			return false
		}
	}
	return true
}
