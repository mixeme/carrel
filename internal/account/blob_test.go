// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package account

import (
	"testing"

	"gitea.mixdep.ru/mix/carrel/internal/crypto"
)

func TestSealOpenRoundTrip(t *testing.T) {
	dek := mustDEK(t)
	blob := &Blob{
		Version: blobVersion,
		Accounts: []Account{{
			ID:       "abc",
			BaseURL:  "https://dav.example/dav.php/",
			Username: "mix",
			Password: "hunter2",
			Enabled:  true,
		}},
	}
	sealed, err := Seal(dek, blob)
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	got, err := Open(dek, sealed)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(got.Accounts) != 1 || got.Accounts[0].Username != "mix" {
		t.Fatalf("round trip changed data: %+v", got)
	}
}

func TestOpenEmptyBlob(t *testing.T) {
	dek := mustDEK(t)
	got, err := Open(dek, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Accounts) != 0 {
		t.Fatalf("accounts = %v, want empty", got.Accounts)
	}
}

func mustDEK(t *testing.T) crypto.Key {
	t.Helper()
	k, err := crypto.NewDEK()
	if err != nil {
		t.Fatal(err)
	}
	return k
}
