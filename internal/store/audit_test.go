// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAuditRecordsAdminActions(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	a := actorOf(admin)

	user, err := s.CreateUserWithPassword(a, "ada", "", RoleUser, testPassword)
	if err != nil {
		t.Fatalf("CreateUserWithPassword: %v", err)
	}
	if err := s.SetDisabled(a, user.ID, true); err != nil {
		t.Fatalf("SetDisabled: %v", err)
	}

	entries := s.Audit(AuditFilter{})
	if len(entries) < 3 {
		t.Fatalf("audit has %d entries, want bootstrap + create + disable", len(entries))
	}
	// Newest first.
	if entries[0].Action != ActionUserDisable {
		t.Errorf("newest entry = %q, want %q", entries[0].Action, ActionUserDisable)
	}
	if entries[0].Seq <= entries[1].Seq {
		t.Error("sequence numbers do not increase with time")
	}

	only := s.Audit(AuditFilter{Action: ActionUserCreate})
	if len(only) != 1 || only[0].TargetLogin != "ada" {
		t.Errorf("filtered audit = %+v, want one user_create for ada", only)
	}
	if got := s.Audit(AuditFilter{TargetID: user.ID}); len(got) != 2 {
		t.Errorf("entries for the new user = %d, want 2", len(got))
	}
	if got := s.Audit(AuditFilter{Limit: 1}); len(got) != 1 {
		t.Errorf("limited audit returned %d entries, want 1", len(got))
	}
}

// A rejected change must leave neither the record nor its audit entry behind:
// they are written in the same atomic commit.
func TestFailedChangeLeavesNoAuditEntry(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	a := actorOf(admin)

	before := len(s.Audit(AuditFilter{}))
	if err := s.SetRole(a, admin.ID, RoleUser); err == nil {
		t.Fatal("demoting the last administrator was allowed")
	}
	if after := len(s.Audit(AuditFilter{})); after != before {
		t.Errorf("audit grew from %d to %d on a rejected change", before, after)
	}
}

func TestAuditKeepsNoSecrets(t *testing.T) {
	dir := t.TempDir()
	s, _ := openTest(t, dir)
	admin := mustAdmin(t, s)
	a := actorOf(admin)

	_, token, err := s.CreateInvite(a, "ada", "", RoleUser, 0)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	if err := s.LogLoginFailure("ada", "10.0.0.9", "bad password"); err != nil {
		t.Fatalf("LogLoginFailure: %v", err)
	}

	for _, e := range s.Audit(AuditFilter{}) {
		if e.Detail == testPassword || e.Detail == token {
			t.Fatalf("audit entry %q carries a secret", e.Action)
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, StateFile))
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if containsBytes(raw, token) || containsBytes(raw, testPassword) {
		t.Error("state file holds a token or a password verbatim")
	}
}

func TestAuditIsBounded(t *testing.T) {
	s, _ := openTest(t, t.TempDir())

	// Fill past the cap directly: going through the public API this many
	// times would mean thousands of Argon2id runs.
	err := s.update(func(state *State) error {
		for i := 0; i < MaxAuditEntries+10; i++ {
			appendAudit(state, s.now(), AuditEntry{Action: ActionLogin})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	entries := s.Audit(AuditFilter{})
	if len(entries) != MaxAuditEntries {
		t.Fatalf("audit holds %d entries, want the cap of %d", len(entries), MaxAuditEntries)
	}
	// The oldest go, not the newest: the last sequence number must survive.
	if entries[0].Seq != int64(MaxAuditEntries+10) {
		t.Errorf("newest sequence = %d, want %d", entries[0].Seq, MaxAuditEntries+10)
	}
}
