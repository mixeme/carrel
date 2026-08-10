// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"errors"
	"testing"
	"time"

	"gitea.mixdep.ru/mix/carrel/internal/crypto"
)

func TestInviteLifecycle(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)

	inv, token, err := s.CreateInvite(actorOf(admin), "ada", "ada@example.org", RoleUser, 0)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	if got := inv.Status(inv.CreatedAt); got != InvitePending {
		t.Fatalf("fresh invite is %q, want pending", got)
	}

	got, err := s.LookupInvite(token)
	if err != nil {
		t.Fatalf("LookupInvite: %v", err)
	}
	if got.ID != inv.ID {
		t.Errorf("looked up invite %q, want %q", got.ID, inv.ID)
	}

	user, err := s.AcceptInvite(token, testPassword, "10.0.0.2")
	if err != nil {
		t.Fatalf("AcceptInvite: %v", err)
	}
	if user.Login != "ada" || user.Email != "ada@example.org" || user.Role != RoleUser {
		t.Errorf("accepted user = %+v, want the invited login, email and role", user)
	}
	if !user.Activated() {
		t.Error("accepted user has no credentials")
	}
	if _, _, err := s.Authenticate("ada", testPassword); err != nil {
		t.Errorf("invited user cannot log in: %v", err)
	}

	// One shot only.
	if _, err := s.AcceptInvite(token, testPassword, ""); !errors.Is(err, ErrInviteInvalid) {
		t.Errorf("second acceptance: got %v, want ErrInviteInvalid", err)
	}
}

// The three ways an invite can be unusable must be indistinguishable from an
// invented token, or probing the endpoint becomes worthwhile (§24.3).
func TestInviteFailuresAreIndistinguishable(t *testing.T) {
	s, c := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	a := actorOf(admin)

	_, revokedToken, err := s.CreateInvite(a, "revoked", "", RoleUser, 0)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	revoked := s.Invites()[0]
	if err := s.RevokeInvite(a, revoked.ID); err != nil {
		t.Fatalf("RevokeInvite: %v", err)
	}

	_, expiredToken, err := s.CreateInvite(a, "expired", "", RoleUser, time.Hour)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	_, acceptedToken, err := s.CreateInvite(a, "accepted", "", RoleUser, 0)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	if _, err := s.AcceptInvite(acceptedToken, testPassword, ""); err != nil {
		t.Fatalf("AcceptInvite: %v", err)
	}

	c.advance(2 * time.Hour)

	unknown, err := crypto.NewToken()
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	for name, token := range map[string]string{
		"unknown":  unknown,
		"revoked":  revokedToken,
		"expired":  expiredToken,
		"accepted": acceptedToken,
		"empty":    "",
	} {
		if _, err := s.LookupInvite(token); !errors.Is(err, ErrInviteInvalid) {
			t.Errorf("%s token: got %v, want ErrInviteInvalid", name, err)
		}
		if _, err := s.AcceptInvite(token, testPassword, ""); !errors.Is(err, ErrInviteInvalid) {
			t.Errorf("%s token on accept: got %v, want ErrInviteInvalid", name, err)
		}
	}
}

func TestInviteStoresOnlyDigest(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)

	_, token, err := s.CreateInvite(actorOf(admin), "ada", "", RoleUser, 0)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	inv := s.Invites()[0]
	if string(inv.TokenHash) == token {
		t.Fatal("the token itself is stored")
	}
	if !crypto.Equal(inv.TokenHash, crypto.HashToken(token)) {
		t.Error("stored digest does not match the issued token")
	}
}

func TestInviteExtend(t *testing.T) {
	s, c := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	a := actorOf(admin)

	_, token, err := s.CreateInvite(a, "ada", "", RoleUser, time.Hour)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	id := s.Invites()[0].ID

	c.advance(2 * time.Hour)
	if _, err := s.LookupInvite(token); !errors.Is(err, ErrInviteInvalid) {
		t.Fatalf("expired invite: got %v, want ErrInviteInvalid", err)
	}

	if err := s.ExtendInvite(a, id, 24*time.Hour); err != nil {
		t.Fatalf("ExtendInvite: %v", err)
	}
	// Extending does not reissue the token, so a link already handed over
	// keeps working.
	if _, err := s.LookupInvite(token); err != nil {
		t.Errorf("extended invite: %v", err)
	}
}

// Mail is a courtesy, not a dependency: an invite created with SMTP unset is
// still usable through the link the admin copies out (§21).
func TestInviteWorksWithoutSMTP(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)

	if s.Settings().SMTP.Configured() {
		t.Fatal("SMTP is configured on a fresh volume")
	}
	inv, token, err := s.CreateInvite(actorOf(admin), "ada", "ada@example.org", RoleUser, 0)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	if inv.SendStatus != SendNotConfigured {
		t.Errorf("send status = %q, want %q", inv.SendStatus, SendNotConfigured)
	}
	if _, err := s.AcceptInvite(token, testPassword, ""); err != nil {
		t.Errorf("accepting an invite that was never mailed: %v", err)
	}
}

func TestRecordInviteSendKeepsInviteValid(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)

	_, token, err := s.CreateInvite(actorOf(admin), "ada", "ada@example.org", RoleUser, 0)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	id := s.Invites()[0].ID
	if err := s.RecordInviteSend(id, SendFailed, "connection refused"); err != nil {
		t.Fatalf("RecordInviteSend: %v", err)
	}
	if _, err := s.LookupInvite(token); err != nil {
		t.Errorf("invite died with the delivery attempt: %v", err)
	}
}
