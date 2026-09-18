package web_test

import (
	"net/url"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

func profileRow(f view.Form, name string) view.FormRow {
	for _, set := range f.Sets {
		for _, row := range set.Rows {
			if row.Name == name {
				return row
			}
		}
	}
	return view.FormRow{}
}

// S-26.1: a new address waits until its link is opened; the one in use —
// and its Reply-To — stays until then. The link goes to the new address.
func TestAccountNameAndAddressConfirmedByLink(t *testing.T) {
	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	h.invite(adminID, "bob", "bobpass123")
	h.confirmAddress("bob", "bob@example.org")
	relay := h.relayed()
	bob := h.login("bob", "bobpass123")

	page := decode[view.AccountPage](t, h.do(postForm("/app/settings/account", bob, url.Values{"name": {"Боб Строитель"}, "email": {"new@example.org"}})))
	if len(page.Notices) != 1 || !strings.Contains(string(page.Notices[0].Title), "new@example.org") {
		t.Fatalf("notices = %+v", page.Notices)
	}
	if row := profileRow(page.Profile, "email"); row.Value != "bob@example.org" || !strings.Contains(string(row.Hint), "pending: new@example.org") {
		t.Errorf("email row = %+v", row)
	}
	if row := profileRow(page.Profile, "name"); row.Value != "Боб Строитель" {
		t.Errorf("name row = %+v", row)
	}
	snap := h.st.Snapshot()
	if u := snap.UserByLogin("bob"); !u.EmailConfirmed || u.Email != "bob@example.org" {
		t.Errorf("the address in use changed before the link: %+v", u)
	}
	letters := relay.Letters()
	if len(letters) != 1 || letters[0].To[0] != "new@example.org" {
		t.Fatalf("letters = %+v", letters)
	}
	_, body := letterBody(t, letters[0])
	link := regexp.MustCompile(`http://example\.com(/confirm-email/[A-Za-z0-9_-]+)`).FindStringSubmatch(body)
	if link == nil {
		t.Fatalf("no link in:\n%s", body)
	}
	if rec := h.do(get(link[1], nil)); rec.Code != 200 {
		t.Fatalf("confirm: %d", rec.Code)
	}
	snap = h.st.Snapshot()
	if u := snap.UserByLogin("bob"); !u.EmailConfirmed || u.Email != "new@example.org" || u.PendingEmail != "" {
		t.Errorf("after the link: %+v", u)
	}
}

// Without a confirmed address the page says replies cannot come back, and
// the From shows the form the administrator chose.
func TestAccountSaysWhereRepliesGo(t *testing.T) {
	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	h.invite(adminID, "bob", "bobpass123")
	h.relayed()
	h.cfg.MailEnabled = true
	h.rebuild()
	bob := h.login("bob", "bobpass123")

	page := decode[view.AccountPage](t, h.do(get("/app/settings/account", bob)))
	if row := profileRow(page.Mail, "replyTo"); row.Value != "" || !strings.Contains(string(row.Hint), "no reply address") {
		t.Errorf("replyTo = %+v", row)
	}
	if row := profileRow(page.Mail, "from"); row.Value != "bob (via Carrel) <noreply@carrel.example>" {
		t.Errorf("from = %q", row.Value)
	}

	h.confirmAddress("bob", "bob@example.org")
	err := h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		st.Settings.SMTP.Sender = "login"
		st.UserByLogin("bob").DisplayName = "Боб"
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	page = decode[view.AccountPage](t, h.do(get("/app/settings/account", bob)))
	if row := profileRow(page.Mail, "from"); row.Value != "Боб <bob@carrel.example>" {
		t.Errorf("from = %q", row.Value)
	}
	if row := profileRow(page.Mail, "replyTo"); row.Value != "bob@example.org" {
		t.Errorf("replyTo = %+v", row)
	}
}

// An invitation proves the address only when it went to it by letter.
func TestInvitationConfirmsOnlyAMailedAddress(t *testing.T) {
	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	alice := h.login("alice", "password123")
	secret, err := h.users.CreateInvite(adminID, "carol", "carol@example.org")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.users.CompleteInvite(t.Context(), secret, "carolpass123"); err != nil {
		t.Fatal(err)
	}
	snap := h.st.Snapshot()
	if carol := snap.UserByLogin("carol"); carol.EmailConfirmed || carol.Email != "" || carol.PendingEmail != "carol@example.org" {
		t.Errorf("a link handed over confirmed the address beside it: %+v", carol)
	}
	if _, _, _, ok := h.users.Authenticate(t.Context(), "carol", "carolpass123"); !ok {
		t.Error("an unproven address locked the invitee out")
	}

	h.relayed()
	h.do(postForm("/admin/invites", alice, url.Values{"action": {"create"}, "login": {"dan"}, "email": {"dan@example.org"}}))
	snap = h.st.Snapshot()
	for _, inv := range snap.Invites {
		if inv.Login == "dan" {
			secret = inv.Secret
		}
	}
	if err := h.users.CompleteInvite(t.Context(), secret, "danpass1234"); err != nil {
		t.Fatal(err)
	}
	snap = h.st.Snapshot()
	if !snap.UserByLogin("dan").EmailConfirmed {
		t.Error("a mailed invitation did not confirm its address")
	}
}

// A first address typed on the account page waits for its link; sign-in is
// not locked meanwhile.
func TestFirstAddressDoesNotLockOut(t *testing.T) {
	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	h.invite(adminID, "bob", "bobpass123")
	h.relayed()
	bob := h.login("bob", "bobpass123")
	h.do(postForm("/app/settings/account", bob, url.Values{"name": {""}, "email": {"bob@example.org"}}))
	snap := h.st.Snapshot()
	if u := snap.UserByLogin("bob"); u.Email != "" || u.PendingEmail != "bob@example.org" {
		t.Errorf("bob = %+v", u)
	}
	if _, _, _, ok := h.users.Authenticate(t.Context(), "bob", "bobpass123"); !ok {
		t.Error("a pending address locked bob out")
	}
}
