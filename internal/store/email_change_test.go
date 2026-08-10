// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"errors"
	"testing"
)

func TestResendInviteRotatesToken(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	a := actorOf(admin)

	_, oldToken, err := s.CreateInvite(a, "ada", "ada@example.org", RoleUser, 0)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	id := s.Invites()[0].ID

	newToken, err := s.ResendInvite(a, id)
	if err != nil {
		t.Fatalf("ResendInvite: %v", err)
	}
	if newToken == oldToken {
		t.Fatal("resend returned the same token")
	}
	if _, err := s.LookupInvite(oldToken); !errors.Is(err, ErrInviteInvalid) {
		t.Errorf("old token still works: %v", err)
	}
	if _, err := s.LookupInvite(newToken); err != nil {
		t.Errorf("new token: %v", err)
	}
}

func TestEmailChangeLifecycle(t *testing.T) {
	s, c := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	a := actorOf(admin)

	token, err := s.RequestEmailChange(a, admin.ID, "new@example.org", 0)
	if err != nil {
		t.Fatalf("RequestEmailChange: %v", err)
	}

	u, err := s.ConfirmEmailChange(token)
	if err != nil {
		t.Fatalf("ConfirmEmailChange: %v", err)
	}
	if u.Email != "new@example.org" {
		t.Errorf("email = %q, want new@example.org", u.Email)
	}

	if _, err := s.ConfirmEmailChange(token); !errors.Is(err, ErrInviteInvalid) {
		t.Errorf("second confirm: got %v, want ErrInviteInvalid", err)
	}

	c.advance(2 * DefaultEmailChangeTTL)
	token2, err := s.RequestEmailChange(a, admin.ID, "again@example.org", 0)
	if err != nil {
		t.Fatalf("RequestEmailChange: %v", err)
	}
	c.advance(2 * DefaultEmailChangeTTL)
	if _, err := s.ConfirmEmailChange(token2); !errors.Is(err, ErrInviteInvalid) {
		t.Errorf("expired token: got %v, want ErrInviteInvalid", err)
	}
}
