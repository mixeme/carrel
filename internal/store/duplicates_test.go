// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"testing"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/account"
)

// TestDuplicateDecisionsSurviveRestart is the acceptance criterion of §21: a group
// marked "not duplicates" stays marked after the process is restarted.
func TestDuplicateDecisionsSurviveRestart(t *testing.T) {
	dir := t.TempDir()
	s, _ := openTest(t, dir)

	admin, err := s.CreateFirstAdmin("admin", "", testPassword, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	_, dek, err := s.Authenticate("admin", testPassword)
	if err != nil {
		t.Fatal(err)
	}

	left := account.Member{AccountID: "acc-1", Collection: "/books/one/", UID: "one"}
	right := account.Member{AccountID: "acc-2", Collection: "/books/two/", UID: "two"}
	err = s.UpdateDuplicates(admin.ID, dek, func(d *account.Duplicates) error {
		_, decideErr := d.Decide("", account.KindContact, account.VerdictIgnored,
			[]account.Member{left, right}, nil, time.Now())
		return decideErr
	})
	if err != nil {
		t.Fatalf("UpdateDuplicates: %v", err)
	}

	dek.Zero()
	_ = s.Close()

	s, _ = openTest(t, dir)
	_, dek, err = s.Authenticate("admin", testPassword)
	if err != nil {
		t.Fatal(err)
	}
	defer dek.Zero()

	decisions, err := s.Duplicates(admin.ID, dek)
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions.Groups) != 1 {
		t.Fatalf("groups = %+v", decisions.Groups)
	}
	if !decisions.Ignored(left, right) {
		t.Fatal("the verdict did not survive the restart")
	}

	// What is read is a copy: reaching into it must not change what is stored.
	decisions.Groups[0].Members[0].UID = "changed"
	again, err := s.Duplicates(admin.ID, dek)
	if err != nil {
		t.Fatal(err)
	}
	if !again.Ignored(left, right) {
		t.Fatal("a caller reached into the stored decisions")
	}

	// A failed update leaves the state as it was.
	wantErr := errTest
	if err := s.UpdateDuplicates(admin.ID, dek, func(d *account.Duplicates) error {
		d.Groups = nil
		return wantErr
	}); err != wantErr {
		t.Fatalf("UpdateDuplicates error = %v, want %v", err, wantErr)
	}
	kept, err := s.Duplicates(admin.ID, dek)
	if err != nil {
		t.Fatal(err)
	}
	if len(kept.Groups) != 1 {
		t.Fatalf("a failed update was committed: %+v", kept.Groups)
	}

	if err := s.UpdateDuplicates("nobody", dek, func(*account.Duplicates) error { return nil }); err == nil {
		t.Fatal("an unknown user was accepted")
	}
}

// errTest is a sentinel for the failure path above.
var errTest = errSentinel("test")

type errSentinel string

func (e errSentinel) Error() string { return string(e) }
