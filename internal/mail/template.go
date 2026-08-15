// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package mail

import (
	"fmt"
	"html"
	"time"
)

// InviteContent builds the invite email bodies (§5.3).
func InviteContent(serviceName, invitedBy, inviteURL string, expires time.Time) Message {
	subject := fmt.Sprintf("Invitation to %s", serviceName)
	text := fmt.Sprintf(`You have been invited to %s by %s.

Open this link to choose your password and sign in:
%s

This link expires on %s.

If you did not expect this message, you can ignore it.`,
		serviceName, invitedBy, inviteURL, expires.UTC().Format(time.RFC1123))
	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html><body>
<p>You have been invited to <strong>%s</strong> by %s.</p>
<p><a href="%s">Accept the invitation</a> and choose your password.</p>
<p>This link expires on %s.</p>
<p>If you did not expect this message, you can ignore it.</p>
</body></html>`,
		html.EscapeString(serviceName),
		html.EscapeString(invitedBy),
		html.EscapeString(inviteURL),
		html.EscapeString(expires.UTC().Format(time.RFC1123)))
	return Message{Subject: subject, Text: text, HTML: htmlBody}
}

// EmailChangeContent builds the address-confirmation email (§5.3).
func EmailChangeContent(serviceName, login, confirmURL string, expires time.Time) Message {
	subject := fmt.Sprintf("Confirm your email on %s", serviceName)
	text := fmt.Sprintf(`Confirm the new email address for account %s on %s:

%s

This link expires on %s.

If you did not request this change, you can ignore this message.`,
		login, serviceName, confirmURL, expires.UTC().Format(time.RFC1123))
	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html><body>
<p>Confirm the new email address for account <strong>%s</strong> on %s.</p>
<p><a href="%s">Confirm email address</a></p>
<p>This link expires on %s.</p>
<p>If you did not request this change, you can ignore this message.</p>
</body></html>`,
		html.EscapeString(login),
		html.EscapeString(serviceName),
		html.EscapeString(confirmURL),
		html.EscapeString(expires.UTC().Format(time.RFC1123)))
	return Message{Subject: subject, Text: text, HTML: htmlBody}
}

// RegisterContent builds the self-registration confirmation email (§5.2).
func RegisterContent(serviceName, login, confirmURL string, expires time.Time) Message {
	subject := fmt.Sprintf("Confirm your %s account", serviceName)
	text := fmt.Sprintf(`Finish creating the account %s on %s by opening this link:

%s

This link expires on %s.

If you did not create this account, you can ignore this message.`,
		login, serviceName, confirmURL, expires.UTC().Format(time.RFC1123))
	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html><body>
<p>Finish creating the account <strong>%s</strong> on %s.</p>
<p><a href="%s">Confirm email address</a></p>
<p>This link expires on %s.</p>
<p>If you did not create this account, you can ignore this message.</p>
</body></html>`,
		html.EscapeString(login),
		html.EscapeString(serviceName),
		html.EscapeString(confirmURL),
		html.EscapeString(expires.UTC().Format(time.RFC1123)))
	return Message{Subject: subject, Text: text, HTML: htmlBody}
}

// EscrowRecoveryContent builds the non-optional recovery notice (§5.4).
func EscrowRecoveryContent(serviceName, login, recoveredAt string) Message {
	subject := fmt.Sprintf("Your %s account was recovered", serviceName)
	text := fmt.Sprintf(`An administrator recovered the account %s on %s at %s.

Your data is intact, but you must sign in with the temporary password you were given and choose a new one immediately.

If you did not expect this, contact your administrator.`,
		login, serviceName, recoveredAt)
	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html><body>
<p>An administrator recovered the account <strong>%s</strong> on %s at %s.</p>
<p>Your data is intact, but you must sign in with the temporary password you were given and choose a new one immediately.</p>
<p>If you did not expect this, contact your administrator.</p>
</body></html>`,
		html.EscapeString(login),
		html.EscapeString(serviceName),
		html.EscapeString(recoveredAt))
	return Message{Subject: subject, Text: text, HTML: htmlBody}
}
