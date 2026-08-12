// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"os"
	"path/filepath"
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/account"
)

func TestDAVAccountRoundTrip(t *testing.T) {
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

	acc := account.Account{
		BaseURL:  "https://dav.example/dav.php/",
		Username: "mix",
		Password: "secret",
	}
	actor := Actor{ID: admin.ID, Login: admin.Login, IP: "127.0.0.1"}
	if err := s.PutDAVAccount(actor, admin.ID, dek, acc); err != nil {
		t.Fatalf("PutDAVAccount: %v", err)
	}

	count, err := s.DAVAccountCount(admin.ID, dek)
	if err != nil || count != 1 {
		t.Fatalf("DAVAccountCount = %d, err = %v", count, err)
	}

	dek.Zero()
	_ = s.Close()

	s, _ = openTest(t, dir)
	_, dek, err = s.Authenticate("admin", testPassword)
	if err != nil {
		t.Fatal(err)
	}
	defer dek.Zero()

	list, err := s.ListDAVAccounts(admin.ID, dek)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Username != "mix" {
		t.Fatalf("ListDAVAccounts = %+v", list)
	}

	if err := s.DeleteDAVAccount(actor, admin.ID, list[0].ID, dek); err != nil {
		t.Fatal(err)
	}
	count, err = s.DAVAccountCount(admin.ID, dek)
	if err != nil || count != 0 {
		t.Fatalf("after delete count = %d, err = %v", count, err)
	}

	if _, err := os.Stat(filepath.Join(dir, StateFile)); err != nil {
		t.Fatalf("state file missing: %v", err)
	}
}
