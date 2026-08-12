// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-vcard"
)

// volatileProperties are the ones a server is expected to rewrite when it
// stores an object, so a difference in them is not a loss.
var volatileProperties = map[string]bool{
	vcard.FieldRevision:  true,
	vcard.FieldProductID: true,
}

// PropertyLoss is what a server did to an object that was not asked for: the
// difference between what was sent and what reading it back returns (§8).
//
// It is a notification and never a block. Losing properties silently is the
// failure that gets noticed months later, when the original is gone; saying so
// at the time leaves the decision with the person making the edit.
type PropertyLoss struct {
	// Missing properties are gone from the stored object entirely.
	Missing []string
	// Reduced properties survived, but with fewer instances than were sent —
	// a second phone number dropped, say.
	Reduced []string
	// Changed properties came back with the same number of instances but
	// different content, parameters included.
	Changed []string
}

// Empty reports whether the server stored what it was given.
func (l PropertyLoss) Empty() bool {
	return len(l.Missing) == 0 && len(l.Reduced) == 0 && len(l.Changed) == 0
}

// Names returns every affected property, sorted and deduplicated.
func (l PropertyLoss) Names() []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(l.Missing)+len(l.Reduced)+len(l.Changed))
	for _, group := range [][]string{l.Missing, l.Reduced, l.Changed} {
		for _, name := range group {
			if seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// String is the message shown to the person who saved the object.
func (l PropertyLoss) String() string {
	if l.Empty() {
		return ""
	}
	var parts []string
	if len(l.Missing) > 0 {
		parts = append(parts, fmt.Sprintf("without %s", strings.Join(l.Missing, ", ")))
	}
	if len(l.Reduced) > 0 {
		parts = append(parts, fmt.Sprintf("with fewer %s", strings.Join(l.Reduced, ", ")))
	}
	if len(l.Changed) > 0 {
		parts = append(parts, fmt.Sprintf("with %s changed", strings.Join(l.Changed, ", ")))
	}
	return "the server returned the object " + strings.Join(parts, ", ")
}

// Compare reports what the stored object lost against the one that was sent.
// The same comparison is what the tests for unknown-property preservation rest
// on; §8 only asks that its result also reach the person.
func Compare(sent, stored *Object) (PropertyLoss, error) {
	if sent == nil || stored == nil {
		return PropertyLoss{}, fmt.Errorf("model: cannot compare a missing object")
	}
	if sent.kind != stored.kind {
		return PropertyLoss{}, fmt.Errorf("model: cannot compare %s with %s", sent.kind, stored.kind)
	}

	var loss PropertyLoss
	for _, name := range sent.Names() {
		if volatileProperties[name] {
			continue
		}
		want := signatures(sent.Property(name))
		got := signatures(stored.Property(name))
		switch {
		case len(got) == 0:
			loss.Missing = append(loss.Missing, name)
		case len(got) < len(want):
			loss.Reduced = append(loss.Reduced, name)
		case !sameSignatures(want, got):
			loss.Changed = append(loss.Changed, name)
		}
	}
	return loss, nil
}

// signatures reduces the instances of one property to comparable strings.
// Parameter order carries no meaning, so it is normalised away; the values
// themselves are compared exactly.
func signatures(values []Value) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		var b strings.Builder
		b.WriteString(v.Group)
		b.WriteByte('|')
		keys := make([]string, 0, len(v.Params))
		for k := range v.Params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			params := append([]string(nil), v.Params[k]...)
			sort.Strings(params)
			b.WriteString(k)
			b.WriteByte('=')
			b.WriteString(strings.Join(params, ","))
			b.WriteByte(';')
		}
		b.WriteByte('|')
		b.WriteString(v.Text)
		out = append(out, b.String())
	}
	sort.Strings(out)
	return out
}

func sameSignatures(want, got []string) bool {
	if len(want) != len(got) {
		return false
	}
	for i := range want {
		if want[i] != got[i] {
			return false
		}
	}
	return true
}

// systematicThreshold is how many writes a property has to be lost by before it
// is called a trait of the server rather than an accident.
const systematicThreshold = 2

// LossRegistry aggregates property loss per DAV account.
//
// A server that drops X- properties drops them from every object, and saying so
// after each save is noise that trains people to dismiss the message. The first
// occurrence is reported inline; after that it belongs in the account's details,
// which is what Report is for (§8).
type LossRegistry struct {
	now func() time.Time

	mu       sync.Mutex
	accounts map[string]*lossState
}

type lossState struct {
	writes      int
	lossyWrites int
	names       map[string]int
	firstSeen   time.Time
	lastSeen    time.Time
}

// NewLossRegistry returns an empty registry.
func NewLossRegistry(now func() time.Time) *LossRegistry {
	if now == nil {
		now = time.Now
	}
	return &LossRegistry{now: now, accounts: make(map[string]*lossState)}
}

// Record notes one verified write against an account. It reports whether this
// write lost a property the account had not lost before — the one case where the
// person needs to be told then and there.
func (r *LossRegistry) Record(accountID string, loss PropertyLoss) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	state := r.accounts[accountID]
	if state == nil {
		state = &lossState{names: make(map[string]int)}
		r.accounts[accountID] = state
	}
	state.writes++
	if loss.Empty() {
		return false
	}

	now := r.now()
	if state.firstSeen.IsZero() {
		state.firstSeen = now
	}
	state.lastSeen = now
	state.lossyWrites++

	novel := false
	for _, name := range loss.Names() {
		if state.names[name] == 0 {
			novel = true
		}
		state.names[name]++
	}
	return novel
}

// Report returns what is known about one account's losses.
func (r *LossRegistry) Report(accountID string) LossReport {
	if r == nil {
		return LossReport{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	state := r.accounts[accountID]
	if state == nil {
		return LossReport{}
	}
	report := LossReport{
		Writes:      state.writes,
		LossyWrites: state.lossyWrites,
		FirstSeen:   state.firstSeen,
		LastSeen:    state.lastSeen,
	}
	for name, count := range state.names {
		report.Properties = append(report.Properties, LossCount{
			Name:       name,
			Writes:     count,
			Systematic: count >= systematicThreshold,
		})
	}
	sort.Slice(report.Properties, func(i, j int) bool {
		if report.Properties[i].Writes != report.Properties[j].Writes {
			return report.Properties[i].Writes > report.Properties[j].Writes
		}
		return report.Properties[i].Name < report.Properties[j].Name
	})
	return report
}

// Forget drops what was recorded for an account, for when it is disconnected.
func (r *LossRegistry) Forget(accountID string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.accounts, accountID)
}

// LossReport is one account's loss history, shown in its details.
type LossReport struct {
	Writes      int
	LossyWrites int
	Properties  []LossCount
	FirstSeen   time.Time
	LastSeen    time.Time
}

// LossCount is how often one property was lost by an account.
type LossCount struct {
	Name   string
	Writes int
	// Systematic marks a property lost by more than one write: a trait of
	// this server rather than a one-off.
	Systematic bool
}

// Empty reports whether the account has lost nothing so far.
func (r LossReport) Empty() bool { return len(r.Properties) == 0 }

// Summary is the sentence shown in the account's details.
func (r LossReport) Summary() string {
	if r.Empty() {
		return ""
	}
	names := make([]string, 0, len(r.Properties))
	systematic := make([]string, 0, len(r.Properties))
	for _, p := range r.Properties {
		names = append(names, p.Name)
		if p.Systematic {
			systematic = append(systematic, p.Name)
		}
	}
	msg := fmt.Sprintf("this server dropped %s on %d of %d saves",
		strings.Join(names, ", "), r.LossyWrites, r.Writes)
	if len(systematic) > 0 {
		msg += fmt.Sprintf("; %s go every time", strings.Join(systematic, ", "))
	}
	return msg
}
