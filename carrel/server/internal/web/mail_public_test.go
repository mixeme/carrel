package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Audit №4, А4-6, the owner's decision of 25.11.2026: without the public
// address (CARREL_PUBLIC_URL) no letter goes at all — every letter carries a
// link to the instance, and without the address that link is built from the
// Host a request names, which anybody may name (a /forgot letter with the
// attacker's host hands him the wipe token). The relay may be configured:
// nothing is sent, and what would send is off with the reason.
func TestNoLetterGoesWithoutThePublicAddress(t *testing.T) {
	const why = "No letter goes without the public address: set CARREL_PUBLIC_URL."
	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	h.invite(adminID, "bob", "bobpass123")
	h.confirmAddress("bob", "bob@example.org")
	relay := h.relayed()
	h.cfg.PublicURL = ""
	alice := h.login("alice", "password123")

	// /forgot: nothing to send, as for an account without mail.
	page := decode[view.ForgotPage](t, h.do(h.anonPostForm("/forgot", "/forgot", url.Values{"login": {"bob"}})))
	for _, o := range page.Options {
		_ = h.do(h.anonPostForm("/forgot", "/forgot", formFields(o.Form)))
	}
	// An invitation by letter: refused.
	_ = h.do(postForm("/admin/invites?action=send", alice, url.Values{"email": {"carol@example.org"}}))
	// The test letter: refused with the reason.
	test := decode[view.AdminSettingsPage](t, h.do(postForm("/admin/settings/mail-test", alice, settingsForm(t, h, alice, map[string]string{"to": "alice@example.org"}))))
	if test.Test == nil || test.Test.OK || string(test.Test.Reply) != why {
		t.Errorf("the test letter: %+v", test.Test)
	}
	if n := len(relay.Letters()); n != 0 {
		t.Fatalf("%d letters went without the public address", n)
	}
	// What would send is off, with the reason.
	invites := decode[view.AdminInvitesPage](t, h.do(get("/admin/invites", alice)))
	off := false
	for _, c := range []view.AddUserCard{invites.Invite, invites.Temporary} {
		for _, a := range c.Form.Actions {
			if strings.Contains(string(a.Label), "Send") && !a.Enabled && string(a.Reason) == why {
				off = true
			}
		}
	}
	if !off {
		t.Errorf("Send invitation is not off with %q", why)
	}
	settings := decode[view.AdminSettingsPage](t, h.do(get("/admin/settings", alice)))
	for _, set := range settings.Form.Sets {
		for _, r := range set.Rows {
			if r.Name == "people" && string(r.Reason) != why {
				t.Errorf("«Let users send letters» reason = %q, want %q", r.Reason, why)
			}
		}
	}
}
