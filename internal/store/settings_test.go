// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSettingsDefaults(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	got := s.Settings()

	if got.CreationMode != CreationInvite {
		t.Errorf("creation mode = %q, want %q", got.CreationMode, CreationInvite)
	}
	if got.SelfRegistration {
		t.Error("self-registration must be off by default")
	}
	if got.InviteTTL() != DefaultInviteTTL {
		t.Errorf("invite TTL = %v, want %v", got.InviteTTL(), DefaultInviteTTL)
	}
	if got.Escrow.Enabled {
		t.Error("escrow must be off by default")
	}
}

func TestUpdateSettings(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)

	err := s.UpdateSettings(actorOf(admin), func(cfg *Settings) {
		cfg.CreationMode = CreationAdminPassword
		cfg.InviteTTLSeconds = int64(24 * time.Hour / time.Second)
		cfg.SMTP.Host = "localhost"
		cfg.SMTP.Port = 25
		cfg.SMTP.TLS = TLSNone
		cfg.SMTP.FromAddress = "carrel@example.org"
	})
	if err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	got := s.Settings()
	if got.CreationMode != CreationAdminPassword {
		t.Errorf("creation mode = %q", got.CreationMode)
	}
	if got.InviteTTL() != 24*time.Hour {
		t.Errorf("invite TTL = %v, want 24h", got.InviteTTL())
	}
	if !got.SMTP.Configured() {
		t.Error("SMTP should now count as configured")
	}

	bad := s.UpdateSettings(actorOf(admin), func(cfg *Settings) { cfg.SMTP.Port = 0 })
	if bad == nil {
		t.Error("invalid SMTP port was accepted")
	}
	if s.Settings().SMTP.Port != 25 {
		t.Error("a rejected update leaked into the stored settings")
	}
}

func TestSMTPPasswordIsSealed(t *testing.T) {
	dir := t.TempDir()
	s, _ := openTest(t, dir)
	admin := mustAdmin(t, s)

	const password = "relay-secret-value"
	if err := s.SetSMTPPassword(actorOf(admin), password); err != nil {
		t.Fatalf("SetSMTPPassword: %v", err)
	}

	got, err := s.SMTPPassword()
	if err != nil {
		t.Fatalf("SMTPPassword: %v", err)
	}
	if got != password {
		t.Errorf("SMTPPassword = %q, want %q", got, password)
	}

	raw, err := os.ReadFile(filepath.Join(dir, StateFile))
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if containsBytes(raw, password) {
		t.Error("SMTP password is on the volume in the clear")
	}

	if err := s.SetSMTPPassword(actorOf(admin), ""); err != nil {
		t.Fatalf("clear SMTP password: %v", err)
	}
	if got, _ := s.SMTPPassword(); got != "" {
		t.Errorf("cleared password = %q, want empty", got)
	}
}

// A general settings edit must not be able to drop the escrow key pair or the
// sealed SMTP password by writing zero values over them.
func TestUpdateSettingsPreservesKeyMaterial(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	a := actorOf(admin)

	if err := s.SetSMTPPassword(a, "relay-secret-value"); err != nil {
		t.Fatalf("SetSMTPPassword: %v", err)
	}
	if err := s.EnableEscrow(a, "master password for escrow"); err != nil {
		t.Fatalf("EnableEscrow: %v", err)
	}

	if err := s.UpdateSettings(a, func(cfg *Settings) {
		cfg.SMTP.Password = nil
		cfg.Escrow.Config = nil
	}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	if got, _ := s.SMTPPassword(); got != "relay-secret-value" {
		t.Error("settings update wiped the SMTP password")
	}
	if s.Settings().Escrow.Config == nil {
		t.Error("settings update wiped the escrow key pair")
	}
}

func TestDisableEscrowKeepsExistingCopies(t *testing.T) {
	s, _ := openTest(t, t.TempDir())
	admin := mustAdmin(t, s)
	a := actorOf(admin)

	if err := s.EnableEscrow(a, "master password for escrow"); err != nil {
		t.Fatalf("EnableEscrow: %v", err)
	}
	_, token, err := s.CreateInvite(a, RoleUser, InviteDeliveryLink, "", 0)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	user, err := s.AcceptInvite(token, "ada", "", testPassword, "")
	if err != nil {
		t.Fatalf("AcceptInvite: %v", err)
	}

	if err := s.DisableEscrow(a); err != nil {
		t.Fatalf("DisableEscrow: %v", err)
	}
	stored, err := s.User(user.ID)
	if err != nil {
		t.Fatalf("User: %v", err)
	}
	// Silently dropping the copy would make the profile's answer to "who can
	// recover your data" wrong; withdrawing is the user's own action (§5.4).
	if len(stored.EscrowDEK) == 0 {
		t.Error("disabling escrow removed a copy the user was told about")
	}
	if s.Settings().Escrow.Active() {
		t.Error("escrow still active after being disabled")
	}
}
