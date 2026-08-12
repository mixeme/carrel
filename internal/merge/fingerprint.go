// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package merge

import (
	"strings"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/model"
)

// Kind is what a fingerprint describes. The two kinds score on different
// signals and are never compared with each other.
type Kind string

const (
	// KindContact is an address object.
	KindContact Kind = "contact"
	// KindEvent is a calendar event.
	KindEvent Kind = "event"
)

// Fingerprint is the comparable shape of one record: the handful of values §15
// scores on, already folded into the form they are compared in.
//
// It carries no payload and no provenance. That is deliberate: a merged list can
// keep a fingerprint per printed row and mark a candidate group without loading
// anything a second time, and a fingerprint can be built once and scored against
// everything else without touching the card again.
type Fingerprint struct {
	Kind Kind
	// UID is the object identity. A match is strong but rare between
	// servers: two accounts holding the same invitation is the usual way it
	// happens (§15).
	UID string
	// Name is the normalised FN of a card or SUMMARY of an event: lower case,
	// punctuation dropped, words sorted, so "Ivan Petrov" and "Petrov, Ivan"
	// are the same name.
	Name string
	// Emails are normalised addresses (model.NormalizeEmail).
	Emails []string
	// Phones are the comparable tails of the telephone numbers (PhoneKey).
	Phones []string
	// Birthday is the digits of BDAY, so 1985-05-10 and 19850510 agree.
	Birthday string
	// Start is DTSTART of an event, in UTC.
	Start time.Time
}

// FingerprintContact folds a card into its comparable form.
func FingerprintContact(c model.Contact) Fingerprint {
	print := Fingerprint{
		Kind:     KindContact,
		UID:      strings.TrimSpace(c.UID),
		Name:     NormalizeName(c.DisplayName()),
		Emails:   c.NormalizedEmails(),
		Birthday: BirthdayKey(c.Birthday),
	}
	seen := make(map[string]bool, len(c.Phones))
	for _, phone := range c.Phones {
		key := PhoneKey(phone.Value)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		print.Phones = append(print.Phones, key)
	}
	return print
}

// FingerprintEvent folds an event into its comparable form. §15 scores events on
// their UID, and on start plus title when the UIDs differ.
func FingerprintEvent(e model.Event) Fingerprint {
	print := Fingerprint{
		Kind: KindEvent,
		UID:  strings.TrimSpace(e.UID),
		Name: NormalizeName(e.Summary),
	}
	if !e.Start.IsZero() {
		print.Start = e.Start.UTC()
	}
	return print
}

// Empty reports whether a fingerprint carries nothing to score on, which is how
// a card with no name, no address and no number stays out of every group rather
// than matching every other blank one.
func (f Fingerprint) Empty() bool {
	return f.UID == "" && f.Name == "" && len(f.Emails) == 0 && len(f.Phones) == 0 && f.Start.IsZero()
}

// buckets are the keys a fingerprint can be found under. Two records are only
// scored against each other when they share one, which keeps detection linear in
// the usual case instead of comparing every card with every other card.
func (f Fingerprint) buckets() []string {
	keys := make([]string, 0, 4+len(f.Emails)+len(f.Phones))
	if f.UID != "" {
		keys = append(keys, "u:"+f.UID)
	}
	for _, email := range f.Emails {
		keys = append(keys, "e:"+email)
	}
	for _, phone := range f.Phones {
		keys = append(keys, "p:"+phone)
	}
	// A name bucket on the first few letters, so a near match on the name is
	// still scored while a full pairwise sweep is not needed. A typo in the
	// opening letters is the one near match this misses.
	if prefix := namePrefix(f.Name); prefix != "" {
		keys = append(keys, "n:"+prefix)
	}
	if !f.Start.IsZero() {
		keys = append(keys, "t:"+f.Start.Format(time.RFC3339))
	}
	return keys
}

const namePrefixLen = 4

func namePrefix(name string) string {
	letters := make([]rune, 0, namePrefixLen)
	for _, r := range strings.ReplaceAll(name, " ", "") {
		letters = append(letters, r)
		if len(letters) == namePrefixLen {
			break
		}
	}
	return string(letters)
}
