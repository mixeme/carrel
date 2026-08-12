// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import "strings"

// gmailDomains treat dots in the local part as insignificant.
var gmailDomains = map[string]bool{"gmail.com": true, "googlemail.com": true}

// NormalizeEmail folds an address to the form two records are compared under:
// a mailto: prefix and surrounding angle brackets removed, case dropped, and
// dots in the local part of a Gmail address ignored.
//
// The mail address is the only bridge between iCalendar and vCard there is
// (§23.9), so matching an ATTENDEE to a card and scoring duplicates (§15) have
// to agree about what counts as the same address; they use this.
func NormalizeEmail(value string) string {
	s := strings.TrimSpace(value)
	s = strings.TrimPrefix(s, "<")
	s = strings.TrimSuffix(s, ">")
	if i := strings.Index(s, ":"); i >= 0 {
		if scheme := strings.ToLower(s[:i]); scheme == "mailto" {
			s = s[i+1:]
		}
	}
	// Drop any RFC 6068 parameters a mailto: URI may carry.
	if i := strings.IndexAny(s, "?"); i >= 0 {
		s = s[:i]
	}
	s = strings.ToLower(strings.TrimSpace(s))
	local, domain, ok := strings.Cut(s, "@")
	if !ok {
		return s
	}
	if gmailDomains[domain] {
		local = strings.ReplaceAll(local, ".", "")
		if plus := strings.Index(local, "+"); plus > 0 {
			local = local[:plus]
		}
	}
	if local == "" {
		return ""
	}
	return local + "@" + domain
}

// AttendeeEmails returns the normalised addresses of an event's attendees,
// deduplicated. ATTENDEE with a mailto: URI is the whole of the bridge to a
// vCard: an event without one matches no card (§23.9).
func (e Event) AttendeeEmails() []string {
	seen := make(map[string]bool, len(e.Attendees))
	out := make([]string, 0, len(e.Attendees))
	for _, a := range e.Attendees {
		if norm := NormalizeEmail(a.URI); norm != "" && !seen[norm] {
			seen[norm] = true
			out = append(out, norm)
		}
	}
	return out
}

// NormalizedEmails returns the addresses a contact carries, folded for
// comparison.
func (c Contact) NormalizedEmails() []string {
	seen := make(map[string]bool, len(c.Emails))
	out := make([]string, 0, len(c.Emails))
	for _, e := range c.Emails {
		if norm := NormalizeEmail(e.Value); norm != "" && !seen[norm] {
			seen[norm] = true
			out = append(out, norm)
		}
	}
	return out
}

// Mentions reports whether the note's body, title or categories name any of the
// given addresses. §23.9 is explicit that this is a mention and not a
// guaranteed link: a note that never spells the address out will not match.
func (n Note) Mentions(addresses []string) bool {
	if len(addresses) == 0 {
		return false
	}
	haystack := strings.ToLower(n.Summary + "\n" + n.Description + "\n" + strings.Join(n.Categories, "\n"))
	for _, addr := range addresses {
		if addr != "" && strings.Contains(haystack, addr) {
			return true
		}
	}
	return false
}
