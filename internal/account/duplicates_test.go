// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package account

import (
	"testing"
	"time"
)

func member(accountID, collection, uid string) Member {
	return Member{AccountID: accountID, Collection: collection, UID: uid}
}

func TestDecideAndFind(t *testing.T) {
	var d Duplicates
	now := time.Now()
	group, err := d.Decide("", KindContact, VerdictLinked, []Member{
		member("a", "/books/one", "one"),
		member("b", "/books/two/", "two"),
	}, map[string]string{"fn": "Ada Lovelace"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if group.ID == "" || group.Verdict != VerdictLinked {
		t.Fatalf("group = %+v", group)
	}
	// Preferences are keyed by the upper-case property name whatever the form
	// called it.
	if group.Fields["FN"] != "Ada Lovelace" {
		t.Fatalf("fields = %+v", group.Fields)
	}
	// A trailing slash on a collection is the same collection.
	found, ok := d.Find(member("b", "/books/two", "two"))
	if !ok || found.ID != group.ID {
		t.Fatalf("Find = %+v %v", found, ok)
	}
	if _, ok := d.Find(member("c", "/books/three", "three")); ok {
		t.Fatal("an unrelated record was found in a group")
	}
	if _, ok := d.FindID(group.ID); !ok {
		t.Fatal("FindID missed the group it just stored")
	}
}

func TestDecideRefusesNonsense(t *testing.T) {
	var d Duplicates
	now := time.Now()
	pair := []Member{member("a", "/books/one", "one"), member("b", "/books/two", "two")}
	if _, err := d.Decide("", KindContact, Verdict("maybe"), pair, nil, now); err == nil {
		t.Fatal("an unknown verdict was accepted")
	}
	if _, err := d.Decide("", "planet", VerdictLinked, pair, nil, now); err == nil {
		t.Fatal("an unknown kind was accepted")
	}
	if _, err := d.Decide("", KindContact, VerdictLinked, pair[:1], nil, now); err == nil {
		t.Fatal("a group of one was accepted")
	}
	// The same record twice is one record, so it is still a group of one.
	if _, err := d.Decide("", KindContact, VerdictLinked, []Member{
		member("a", "/books/one", "one"), member("a", "/books/one/", "one"),
	}, nil, now); err == nil {
		t.Fatal("a record paired with itself was accepted")
	}
	if len(d.Groups) != 0 {
		t.Fatalf("refused decisions were stored: %+v", d.Groups)
	}
}

// TestDecideMovesMembers covers what §15 requires of a second decision: a record
// belongs to one group, and the current decision is the one that counts.
func TestDecideMovesMembers(t *testing.T) {
	var d Duplicates
	now := time.Now()
	first, err := d.Decide("", KindContact, VerdictLinked, []Member{
		member("a", "/books/one", "one"),
		member("b", "/books/two", "two"),
		member("c", "/books/three", "three"),
	}, nil, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Decide("", KindContact, VerdictIgnored, []Member{
		member("a", "/books/one", "one"),
		member("d", "/books/four", "four"),
	}, nil, now); err != nil {
		t.Fatal(err)
	}
	if len(d.Groups) != 2 {
		t.Fatalf("groups = %+v", d.Groups)
	}
	old, ok := d.FindID(first.ID)
	if !ok || len(old.Members) != 2 {
		t.Fatalf("the first group should have lost its member: %+v", old)
	}
	if old.Has(member("a", "/books/one", "one")) {
		t.Fatal("the moved record is still in its old group")
	}
	// A group left with one member dissolves rather than lingering.
	if _, err := d.Decide("", KindContact, VerdictLinked, []Member{
		member("b", "/books/two", "two"),
		member("e", "/books/five", "five"),
	}, nil, now); err != nil {
		t.Fatal(err)
	}
	if _, ok := d.FindID(first.ID); ok {
		t.Fatal("a group of one survived")
	}
}

func TestIgnoredAndVerdict(t *testing.T) {
	var d Duplicates
	left, right := member("a", "/books/one", "one"), member("b", "/books/two", "two")
	if _, err := d.Decide("", KindContact, VerdictIgnored, []Member{left, right}, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	if !d.Ignored(left, right) || !d.Ignored(right, left) {
		t.Fatal("a rejected pair is not reported as rejected")
	}
	if d.Ignored(left, member("c", "/books/three", "three")) {
		t.Fatal("an unrelated pair was reported as rejected")
	}
	// Verdict answers about exactly this set, whatever order it arrives in.
	if _, ok := d.Verdict([]Member{right, left}); !ok {
		t.Fatal("Verdict missed the set it stored")
	}
	if _, ok := d.Verdict([]Member{left, member("c", "/books/three", "three")}); ok {
		t.Fatal("Verdict answered about a different set")
	}
}

func TestPruneDropsGoneMembers(t *testing.T) {
	var d Duplicates
	three := []Member{
		member("a", "/books/one", "one"),
		member("b", "/books/two", "two"),
		member("c", "/books/three", "three"),
	}
	group, err := d.Decide("", KindContact, VerdictLinked, three, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if d.Prune(func(Member) bool { return false }) {
		t.Fatal("nothing was gone, but Prune reported a change")
	}
	if !d.Prune(func(m Member) bool { return m.UID == "three" }) {
		t.Fatal("Prune did not report the member it dropped")
	}
	left, ok := d.FindID(group.ID)
	if !ok || len(left.Members) != 2 {
		t.Fatalf("group after prune = %+v", left)
	}
	// Down to one member the group has nothing left to describe.
	if !d.Prune(func(m Member) bool { return m.UID == "two" }) {
		t.Fatal("Prune did not report the dissolved group")
	}
	if len(d.Groups) != 0 {
		t.Fatalf("groups = %+v", d.Groups)
	}
}

func TestRemoveAndSetField(t *testing.T) {
	var d Duplicates
	group, err := d.Decide("", KindContact, VerdictLinked, []Member{
		member("a", "/books/one", "one"), member("b", "/books/two", "two"),
	}, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !d.SetField(group.ID, "tel", "+7 495 123-45-67") {
		t.Fatal("SetField missed the group")
	}
	stored, _ := d.FindID(group.ID)
	if stored.Fields["TEL"] != "+7 495 123-45-67" {
		t.Fatalf("fields = %+v", stored.Fields)
	}
	if !d.SetField(group.ID, "TEL", "  ") {
		t.Fatal("clearing a field failed")
	}
	stored, _ = d.FindID(group.ID)
	if _, ok := stored.Fields["TEL"]; ok {
		t.Fatalf("an empty value was stored: %+v", stored.Fields)
	}
	if !d.Remove(group.ID) || len(d.Groups) != 0 {
		t.Fatalf("Remove left %+v", d.Groups)
	}
	if d.Remove(group.ID) {
		t.Fatal("Remove reported success twice")
	}
}

func TestCloneIsDeep(t *testing.T) {
	var d Duplicates
	group, err := d.Decide("", KindContact, VerdictLinked, []Member{
		member("a", "/books/one", "one"), member("b", "/books/two", "two"),
	}, map[string]string{"FN": "Ada"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	clone := d.Clone()
	clone.Groups[0].Members[0].UID = "changed"
	clone.Groups[0].Fields["FN"] = "changed"
	stored, _ := d.FindID(group.ID)
	if stored.Members[0].UID == "changed" || stored.Fields["FN"] != "Ada" {
		t.Fatalf("a clone reached into stored state: %+v", stored)
	}
}

// TestDuplicatesSurviveSealing is the requirement of §21: a decision outlives the
// session it was made in, and it travels sealed with the credentials.
func TestDuplicatesSurviveSealing(t *testing.T) {
	dek := mustDEK(t)
	blob := &Blob{}
	if _, err := blob.Duplicates.Decide("", KindEvent, VerdictIgnored, []Member{
		member("a", "/cals/one", "evt"), member("b", "/cals/two", "evt"),
	}, nil, time.Now()); err != nil {
		t.Fatal(err)
	}
	sealed, err := Seal(dek, blob)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := Open(dek, sealed)
	if err != nil {
		t.Fatal(err)
	}
	if len(opened.Duplicates.Groups) != 1 {
		t.Fatalf("groups after sealing = %+v", opened.Duplicates.Groups)
	}
	if !opened.Duplicates.Ignored(member("a", "/cals/one", "evt"), member("b", "/cals/two", "evt")) {
		t.Fatal("the verdict did not survive sealing")
	}
	if opened.Duplicates.Groups[0].Kind != KindEvent {
		t.Fatalf("kind = %q", opened.Duplicates.Groups[0].Kind)
	}
}
