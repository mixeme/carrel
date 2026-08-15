// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package mail

import (
	"strings"
	"testing"
	"time"
)

func TestInviteContentHasNoExternalResources(t *testing.T) {
	msg := InviteContent("Carrel", "root", "https://example.org/invite/abc", time.Now().Add(24*time.Hour))
	if !strings.Contains(msg.Text, "https://example.org/invite/abc") {
		t.Error("plain text missing invite URL")
	}
	if strings.Contains(msg.HTML, "http://") && !strings.Contains(msg.HTML, "https://example.org") {
		t.Error("unexpected external resource in HTML")
	}
	if msg.Subject == "" {
		t.Error("empty subject")
	}
}

func TestEmailChangeContent(t *testing.T) {
	msg := EmailChangeContent("Carrel", "ada", "https://example.org/confirm/x", time.Now())
	if !strings.Contains(msg.Text, "ada") || !strings.Contains(msg.Text, "https://example.org/confirm/x") {
		t.Error("confirmation email missing expected content")
	}
}

func TestRegisterContent(t *testing.T) {
	msg := RegisterContent("Carrel", "ada", "https://example.org/confirm/x", time.Now())
	if !strings.Contains(msg.Text, "ada") || !strings.Contains(msg.Text, "https://example.org/confirm/x") {
		t.Error("registration email missing expected content")
	}
	if strings.Contains(msg.HTML, "<img") {
		t.Error("registration email must not carry images")
	}
}
