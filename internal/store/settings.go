// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"fmt"

	"gitea.mixdep.ru/mix/carrel/internal/crypto"
)

// Settings returns a copy of the global settings.
func (s *Store) Settings() Settings {
	var out Settings
	s.read(func(state *State) { out = state.Settings.Clone() })
	return out
}

// UpdateSettings applies fn to the settings and commits the result. The SMTP
// password is not reachable through it: use SetSMTPPassword, which encrypts.
func (s *Store) UpdateSettings(actor Actor, fn func(*Settings)) error {
	return s.update(func(state *State) error {
		next := state.Settings.Clone()
		fn(&next)
		if err := validateSettings(next); err != nil {
			return err
		}
		// Escrow key material is managed by the escrow calls, not by a
		// general settings edit.
		next.Escrow.Config = state.Settings.Escrow.Config
		next.SMTP.Password = state.Settings.SMTP.Password
		state.Settings = next
		appendAudit(state, s.now(), AuditEntry{
			Action:     ActionSettings,
			ActorID:    actor.ID,
			ActorLogin: actor.Login,
			IP:         actor.IP,
		})
		return nil
	})
}

func validateSettings(s Settings) error {
	if !s.CreationMode.Valid() {
		return fmt.Errorf("store: unknown creation mode %q", s.CreationMode)
	}
	if s.SMTP.Host != "" {
		if !s.SMTP.TLS.Valid() {
			return fmt.Errorf("store: unknown SMTP TLS mode %q", s.SMTP.TLS)
		}
		if s.SMTP.Port < 1 || s.SMTP.Port > 65535 {
			return fmt.Errorf("store: SMTP port must be between 1 and 65535, got %d", s.SMTP.Port)
		}
		if err := ValidateEmail(s.SMTP.FromAddress); err != nil {
			return err
		}
	}
	return nil
}

// SetSMTPPassword stores the relay password sealed with the server key, so it
// is not lying in the clear on the volume (§5.3). An empty password clears it.
func (s *Store) SetSMTPPassword(actor Actor, password string) error {
	return s.update(func(state *State) error {
		if password == "" {
			state.Settings.SMTP.Password = nil
			return nil
		}
		plaintext := []byte(password)
		defer crypto.Zero(plaintext)

		sealed, err := crypto.SealState(s.key, plaintext)
		if err != nil {
			return err
		}
		state.Settings.SMTP.Password = sealed
		appendAudit(state, s.now(), AuditEntry{
			Action:     ActionSettings,
			ActorID:    actor.ID,
			ActorLogin: actor.Login,
			IP:         actor.IP,
			Detail:     "smtp password",
		})
		return nil
	})
}

// SMTPPassword decrypts the relay password for use by the mail client. The
// result must not be logged or echoed back into the admin UI (§24.6).
func (s *Store) SMTPPassword() (string, error) {
	var sealed []byte
	s.read(func(state *State) { sealed = cloneBytes(state.Settings.SMTP.Password) })
	if len(sealed) == 0 {
		return "", nil
	}
	plaintext, err := crypto.OpenState(s.key, sealed)
	if err != nil {
		return "", err
	}
	defer crypto.Zero(plaintext)
	return string(plaintext), nil
}
