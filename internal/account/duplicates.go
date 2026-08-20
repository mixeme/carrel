// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package account

import (
	"errors"
	"sort"
	"strings"
	"time"
)

// Verdict is what the person decided about a group of records §15 believes are
// one thing. There is no third state stored: a group nobody has decided about is
// simply absent, and is offered again the next time it is detected.
type Verdict string

const (
	// VerdictLinked means the records are one entity: they are shown as one
	// row with merged fields, and nothing on any server changed.
	VerdictLinked Verdict = "linked"
	// VerdictIgnored means they are different things and the group is not to
	// be offered again.
	VerdictIgnored Verdict = "ignored"
)

// Valid reports whether v is a stored verdict.
func (v Verdict) Valid() bool { return v == VerdictLinked || v == VerdictIgnored }

// Kinds of record a group can be about.
const (
	KindContact = "contact"
	KindEvent   = "event"
	KindTodo    = "todo"
	KindNote    = "note"
)

// Member identifies one object of a group. §15 fixes the key — account,
// collection path, object UID — and warns in the same breath that it is not
// stable: another client can delete or move the object at any time. Everything
// that reads a group therefore has to cope with a member that is not there any
// more, silently (Prune).
type Member struct {
	AccountID  string `json:"account_id"`
	Collection string `json:"collection"`
	UID        string `json:"uid"`
}

// Key is the comparable form of a member.
func (m Member) Key() string {
	return m.AccountID + "|" + normalizePath(m.Collection) + "|" + strings.TrimSpace(m.UID)
}

// Source is the collection the member lives in.
func (m Member) Source() SourceRef {
	return SourceRef{AccountID: m.AccountID, Collection: m.Collection}
}

func (m Member) normalized() Member {
	return Member{
		AccountID:  strings.TrimSpace(m.AccountID),
		Collection: normalizePath(m.Collection),
		UID:        strings.TrimSpace(m.UID),
	}
}

func (m Member) valid() bool {
	n := m.normalized()
	return n.AccountID != "" && n.Collection != "" && n.UID != ""
}

// Group is one decision about one set of records.
type Group struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Verdict Verdict  `json:"verdict"`
	Members []Member `json:"members"`
	// Fields remembers which value won for a property the records disagree
	// about, keyed by property name (§15). It is a preference and not a
	// value: a record that no longer offers it falls back to the merge order.
	Fields    map[string]string `json:"fields,omitempty"`
	DecidedAt time.Time         `json:"decided_at"`
}

// Has reports whether the group contains a member.
func (g Group) Has(member Member) bool {
	key := member.Key()
	for _, m := range g.Members {
		if m.Key() == key {
			return true
		}
	}
	return false
}

// MemberKeys returns the members' keys, sorted, which is how two groups of the
// same records are recognised as the same group.
func (g Group) MemberKeys() []string {
	out := make([]string, 0, len(g.Members))
	for _, m := range g.Members {
		out = append(out, m.Key())
	}
	sort.Strings(out)
	return out
}

// Signature is the identity of a group's member set.
func (g Group) Signature() string { return strings.Join(g.MemberKeys(), "\x00") }

func (g Group) clone() Group {
	out := g
	out.Members = append([]Member(nil), g.Members...)
	if g.Fields != nil {
		out.Fields = make(map[string]string, len(g.Fields))
		for name, value := range g.Fields {
			out.Fields[name] = value
		}
	}
	return out
}

// Duplicates holds the decisions of §15. These are the only data about the
// content of a person's collections that Carrel keeps on disk, and they are
// metadata only: an account, a collection, a UID and which value won a field.
// They are sealed with the same DEK as the credentials, because a list of UIDs
// paired with accounts is not something a stolen volume should give away.
type Duplicates struct {
	Groups []Group `json:"groups,omitempty"`
	// Threshold is the score a pair must reach to be offered as a duplicate
	// group. Zero means the instance default from config (wave 3.7).
	Threshold int `json:"threshold,omitempty"`
}

// EffectiveThreshold returns the stored preference, or instanceDefault when
// none has been chosen yet.
func (d Duplicates) EffectiveThreshold(instanceDefault int) int {
	if d.Threshold > 0 {
		return d.Threshold
	}
	if instanceDefault > 0 {
		return instanceDefault
	}
	return 0
}

// SetThreshold remembers the score a pair must reach. It must be positive.
func (d *Duplicates) SetThreshold(n int) error {
	if d == nil {
		return errors.New("account: no decision store")
	}
	if n <= 0 {
		return errors.New("account: duplicate threshold must be positive")
	}
	d.Threshold = n
	return nil
}

// Clone returns a deep copy, so a caller cannot reach into stored state.
func (d Duplicates) Clone() Duplicates {
	out := Duplicates{Threshold: d.Threshold}
	if len(d.Groups) == 0 {
		return out
	}
	out.Groups = make([]Group, 0, len(d.Groups))
	for _, g := range d.Groups {
		out.Groups = append(out.Groups, g.clone())
	}
	return out
}

// Find returns the group a member belongs to. A member belongs to at most one:
// deciding about it again replaces the earlier decision.
func (d Duplicates) Find(member Member) (Group, bool) {
	member = member.normalized()
	for _, g := range d.Groups {
		if g.Has(member) {
			return g.clone(), true
		}
	}
	return Group{}, false
}

// FindID returns one group by identifier.
func (d Duplicates) FindID(id string) (Group, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Group{}, false
	}
	for _, g := range d.Groups {
		if g.ID == id {
			return g.clone(), true
		}
	}
	return Group{}, false
}

// Verdict returns the decision recorded for exactly this set of members, if any.
// It is what keeps a group the person has rejected from being offered again after
// a restart (§21).
func (d Duplicates) Verdict(members []Member) (Group, bool) {
	want := Group{Members: normalizeMembers(members)}.Signature()
	if want == "" {
		return Group{}, false
	}
	for _, g := range d.Groups {
		if g.Signature() == want {
			return g.clone(), true
		}
	}
	return Group{}, false
}

// Ignored reports whether two members are in one group that was decided against.
// Detection asks this before offering a pair, so "not duplicates" survives both a
// reload and a restart.
func (d Duplicates) Ignored(a, b Member) bool {
	a, b = a.normalized(), b.normalized()
	for _, g := range d.Groups {
		if g.Verdict == VerdictIgnored && g.Has(a) && g.Has(b) {
			return true
		}
	}
	return false
}

// Decide records a verdict about a set of members.
//
// The members are taken out of whatever groups they were in first: a record
// belongs to one group at a time, and a decision about it is the current one.
// A set of fewer than two members is not a group and is refused.
func (d *Duplicates) Decide(id, kind string, verdict Verdict, members []Member, fields map[string]string, now time.Time) (Group, error) {
	if d == nil {
		return Group{}, errors.New("account: no decision store")
	}
	if !verdict.Valid() {
		return Group{}, errors.New("account: unknown duplicate verdict")
	}
	normalised := normalizeMembers(members)
	if len(normalised) < 2 {
		return Group{}, errors.New("account: a duplicate group needs at least two records")
	}
	kind = strings.TrimSpace(kind)
	if kind != KindContact && kind != KindEvent && kind != KindTodo && kind != KindNote {
		return Group{}, errors.New("account: unknown duplicate kind")
	}

	d.release(normalised)

	id = strings.TrimSpace(id)
	if id == "" {
		generated, err := NewID()
		if err != nil {
			return Group{}, err
		}
		id = generated
	}
	group := Group{ID: id, Kind: kind, Verdict: verdict, Members: normalised, DecidedAt: now.UTC()}
	for name, value := range fields {
		group.setField(name, value)
	}
	d.Groups = append(d.Groups, group)
	return group.clone(), nil
}

// SetField remembers which value won a field of a linked group (§15).
func (d *Duplicates) SetField(id, name, value string) bool {
	if d == nil {
		return false
	}
	id = strings.TrimSpace(id)
	for i := range d.Groups {
		if d.Groups[i].ID != id {
			continue
		}
		d.Groups[i].setField(name, value)
		return true
	}
	return false
}

func (g *Group) setField(name, value string) {
	name = strings.ToUpper(strings.TrimSpace(name))
	value = strings.TrimSpace(value)
	if name == "" {
		return
	}
	if value == "" {
		delete(g.Fields, name)
		return
	}
	if g.Fields == nil {
		g.Fields = make(map[string]string, 2)
	}
	g.Fields[name] = value
}

// Remove drops a group by identifier: the link is broken, or the group starts
// being offered again. §15 asks for both to be possible at any time.
func (d *Duplicates) Remove(id string) bool {
	if d == nil {
		return false
	}
	id = strings.TrimSpace(id)
	for i, g := range d.Groups {
		if g.ID == id {
			d.Groups = append(d.Groups[:i], d.Groups[i+1:]...)
			return true
		}
	}
	return false
}

// release takes members out of the groups they are in, dissolving a group left
// with fewer than two.
func (d *Duplicates) release(members []Member) {
	wanted := make(map[string]bool, len(members))
	for _, m := range members {
		wanted[m.Key()] = true
	}
	kept := d.Groups[:0]
	for _, g := range d.Groups {
		remaining := make([]Member, 0, len(g.Members))
		for _, m := range g.Members {
			if !wanted[m.Key()] {
				remaining = append(remaining, m)
			}
		}
		if len(remaining) == len(g.Members) {
			kept = append(kept, g)
			continue
		}
		if len(remaining) < 2 {
			continue
		}
		g.Members = remaining
		kept = append(kept, g)
	}
	d.Groups = kept
}

// Prune drops the members that gone reports as no longer there, and dissolves a
// group left with fewer than two.
//
// §15 is precise about this being silent: an object deleted or moved by another
// client is normal, the identifier was never stable, and there is nothing for the
// person to do about it. It reports whether anything changed, so a caller can
// avoid writing the state file for nothing.
func (d *Duplicates) Prune(gone func(Member) bool) bool {
	if d == nil || gone == nil {
		return false
	}
	changed := false
	kept := make([]Group, 0, len(d.Groups))
	for _, g := range d.Groups {
		remaining := make([]Member, 0, len(g.Members))
		for _, m := range g.Members {
			if gone(m) {
				changed = true
				continue
			}
			remaining = append(remaining, m)
		}
		if len(remaining) < 2 {
			changed = changed || len(remaining) != len(g.Members)
			continue
		}
		g.Members = remaining
		kept = append(kept, g)
	}
	if changed {
		d.Groups = kept
	}
	return changed
}

// PurgeCollection drops duplicate groups touching one collection (§10.1).
func (d *Duplicates) PurgeCollection(ref SourceRef) {
	if d == nil {
		return
	}
	ref.Collection = normalizePath(ref.Collection)
	d.Prune(func(m Member) bool {
		return m.AccountID == ref.AccountID && normalizePath(m.Collection) == ref.Collection
	})
}

func normalizeMembers(members []Member) []Member {
	out := make([]Member, 0, len(members))
	seen := make(map[string]bool, len(members))
	for _, m := range members {
		if !m.valid() {
			continue
		}
		n := m.normalized()
		if seen[n.Key()] {
			continue
		}
		seen[n.Key()] = true
		out = append(out, n)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Key() < out[j].Key() })
	return out
}
