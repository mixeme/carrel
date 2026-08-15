// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"errors"
	"testing"
)

func TestRegisterRequiresTheSetting(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	mustAdmin(t, s)

	if _, _, err := s.Register("ada", "ada@example.org", testPassword, "10.0.0.2"); !errors.Is(err, ErrRegistrationClosed) {
		t.Fatalf("Register while closed: got %v, want ErrRegistrationClosed", err)
	}
}

func TestRegisterLifecycle(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	if err := s.UpdateSettings(actorOf(admin), func(cfg *Settings) { cfg.SelfRegistration = true }); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	if _, _, err := s.Register("ada", "", testPassword, ""); err == nil {
		t.Fatal("Register accepted an empty email")
	}

	user, token, err := s.Register("ada", "ada@example.org", testPassword, "10.0.0.2")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if !user.Unconfirmed {
		t.Fatal("a self-registered account must wait for email confirmation")
	}
	if _, _, err := s.Authenticate("ada", testPassword); !errors.Is(err, ErrUserUnconfirmed) {
		t.Fatalf("unconfirmed login: got %v, want ErrUserUnconfirmed", err)
	}

	confirmed, registration, err := s.ConfirmEmailChange(token)
	if err != nil {
		t.Fatalf("ConfirmEmailChange: %v", err)
	}
	if !registration {
		t.Error("confirming a sign-up was not reported as registration")
	}
	if confirmed.Unconfirmed {
		t.Error("confirmation left the account unconfirmed")
	}
	if confirmed.Email != "ada@example.org" {
		t.Errorf("email = %q", confirmed.Email)
	}

	if _, _, err := s.Authenticate("ada", testPassword); err != nil {
		t.Fatalf("Authenticate after confirm: %v", err)
	}
}

func TestRegisterResendReplacesAnUnconfirmedAccount(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	if err := s.UpdateSettings(actorOf(admin), func(cfg *Settings) { cfg.SelfRegistration = true }); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	_, oldToken, err := s.Register("ada", "ada@example.org", testPassword, "")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	const next = "a different horse battery"
	_, newToken, err := s.Register("ada", "ada@example.org", next, "")
	if err != nil {
		t.Fatalf("Register again: %v", err)
	}
	if newToken == oldToken {
		t.Fatal("resend returned the same token")
	}
	if _, _, err := s.ConfirmEmailChange(oldToken); !errors.Is(err, ErrInviteInvalid) {
		t.Errorf("old token still works: %v", err)
	}
	if _, registration, err := s.ConfirmEmailChange(newToken); err != nil || !registration {
		t.Fatalf("new token: err=%v registration=%v", err, registration)
	}
	if _, _, err := s.Authenticate("ada", next); err != nil {
		t.Fatalf("Authenticate with resent password: %v", err)
	}

	if _, _, err := s.Register("ada", "ada@example.org", testPassword, ""); !errors.Is(err, ErrLoginTaken) {
		t.Fatalf("Register after confirm: got %v, want ErrLoginTaken", err)
	}
}

func TestRegisterDoesNotReuseADifferentEmail(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	if err := s.UpdateSettings(actorOf(admin), func(cfg *Settings) { cfg.SelfRegistration = true }); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}
	if _, _, err := s.Register("ada", "ada@example.org", testPassword, ""); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if _, _, err := s.Register("ada", "other@example.org", testPassword, ""); !errors.Is(err, ErrLoginTaken) {
		t.Fatalf("Register different email: got %v, want ErrLoginTaken", err)
	}
}
