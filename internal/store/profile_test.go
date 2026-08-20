// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"testing"
)

func TestForbiddenLoginRejected(t *testing.T) {
	if err := ValidateLogin(ForbiddenLogin); err == nil {
		t.Fatal("ValidateLogin accepted reserved login")
	}
}

func TestDisplayNameNormalization(t *testing.T) {
	if got := NormalizeDisplayName("  Ada\r\nLovelace  "); got != "AdaLovelace" {
		t.Fatalf("NormalizeDisplayName = %q", got)
	}
}

func TestSenderNameAndReplyAddress(t *testing.T) {
	withName := &User{Login: "ada", DisplayName: "Ada Lovelace", Email: "ada@example.org", EmailConfirmed: true}
	if got := withName.SenderName(); got != "Ada Lovelace" {
		t.Fatalf("SenderName = %q", got)
	}
	if got := withName.ReplyAddress(); got != "ada@example.org" {
		t.Fatalf("ReplyAddress = %q", got)
	}

	fallback := &User{Login: "ada", Email: "ada@example.org", EmailConfirmed: true}
	if got := fallback.SenderName(); got != "ada" {
		t.Fatalf("SenderName fallback = %q", got)
	}

	unconfirmed := &User{Login: "ada", Email: "ada@example.org"}
	if got := unconfirmed.ReplyAddress(); got != "" {
		t.Fatalf("ReplyAddress unconfirmed = %q, want empty", got)
	}
}

func TestSetDisplayName(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)

	if err := s.SetDisplayName(admin.ID, "  Mix Dep  "); err != nil {
		t.Fatalf("SetDisplayName: %v", err)
	}
	user, err := s.User(admin.ID)
	if err != nil {
		t.Fatalf("User: %v", err)
	}
	if user.DisplayName != "Mix Dep" {
		t.Fatalf("DisplayName = %q", user.DisplayName)
	}
}

func TestLinkInviteLeavesEmailUnconfirmed(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	a := actorOf(admin)

	_, token, err := s.CreateInvite(a, RoleUser, InviteDeliveryLink, "", 0)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	user, err := s.AcceptInvite(token, "linkuser", "typed@example.org", testPassword, "")
	if err != nil {
		t.Fatalf("AcceptInvite: %v", err)
	}
	if user.Email != "typed@example.org" {
		t.Fatalf("email = %q", user.Email)
	}
	if user.EmailConfirmed {
		t.Fatal("link invitation must leave the address unconfirmed")
	}
	if user.ReplyAddress() != "" {
		t.Fatal("ReplyAddress must stay empty until confirmation")
	}
}

func TestEmailInviteConfirmsAddress(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	a := actorOf(admin)

	_, token, err := s.CreateInvite(a, RoleUser, InviteDeliveryEmail, "mail@example.org", 0)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	user, err := s.AcceptInvite(token, "mailuser", "", testPassword, "")
	if err != nil {
		t.Fatalf("AcceptInvite: %v", err)
	}
	if !user.EmailConfirmed {
		t.Fatal("email invitation must confirm the address")
	}
}

func TestResendConfirmationForUnconfirmedAddress(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	a := actorOf(admin)

	_, token, err := s.CreateInvite(a, RoleUser, InviteDeliveryLink, "", 0)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	user, err := s.AcceptInvite(token, "linkuser", "typed@example.org", testPassword, "")
	if err != nil {
		t.Fatalf("AcceptInvite: %v", err)
	}

	resend, err := s.RequestEmailChange(a, user.ID, user.Email, 0)
	if err != nil {
		t.Fatalf("RequestEmailChange resend: %v", err)
	}
	if resend == "" {
		t.Fatal("expected a confirmation token")
	}

	confirmed, _, err := s.ConfirmEmailChange(resend)
	if err != nil {
		t.Fatalf("ConfirmEmailChange: %v", err)
	}
	if !confirmed.EmailConfirmed {
		t.Fatal("confirmation must mark the address confirmed")
	}
}

func TestConfirmedAddressCannotResendSameEmail(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	a := actorOf(admin)
	if err := s.SetEmail(a, admin.ID, "admin@example.org"); err != nil {
		t.Fatalf("SetEmail: %v", err)
	}

	if _, err := s.RequestEmailChange(a, admin.ID, "admin@example.org", 0); err == nil {
		t.Fatal("RequestEmailChange accepted unchanged confirmed address")
	}
}

func TestRegistrationConfirmsOnLink(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	if err := s.UpdateSettings(actorOf(admin), func(cfg *Settings) { cfg.SelfRegistration = true }); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	_, token, err := s.Register("ada", "ada@example.org", testPassword, "")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	user, _, err := s.ConfirmEmailChange(token)
	if err != nil {
		t.Fatalf("ConfirmEmailChange: %v", err)
	}
	if !user.EmailConfirmed {
		t.Fatal("self-registration must confirm the address on the link")
	}
}

func TestSetEmailClearsConfirmationWhenEmpty(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	a := actorOf(admin)

	if err := s.SetEmail(a, admin.ID, ""); err != nil {
		t.Fatalf("SetEmail: %v", err)
	}
	user, err := s.User(admin.ID)
	if err != nil {
		t.Fatalf("User: %v", err)
	}
	if user.EmailConfirmed {
		t.Fatal("empty address must not be confirmed")
	}
}

func TestCreateUserRejectsNoreplyLogin(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	a := actorOf(admin)

	_, err := s.CreateUserWithPassword(a, ForbiddenLogin, "svc@example.org", RoleUser, testPassword)
	if err == nil {
		t.Fatal("CreateUserWithPassword accepted reserved login")
	}
}
