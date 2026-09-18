package web_test

import (
	"net/http"
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

// The account page lists the person's sessions — browser, address, when —
// by a name that is not the cookie, and ends one or every other.
func TestAccountSessions(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	loginAs := func(ua string) *http.Cookie {
		req := h.anonPostForm("/login", "/login", url.Values{"login": {"alice"}, "password": {"password123"}})
		req.Header.Set("User-Agent", ua)
		rec := h.do(req)
		for _, c := range rec.Result().Cookies() {
			if c.Name == "carrel_sid" {
				return c
			}
		}
		t.Fatalf("no session: %d", rec.Code)
		return nil
	}
	firefox := loginAs("Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:130.0) Gecko/20100101 Firefox/130.0")
	loginAs("Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0 Mobile Safari/537.36")
	loginAs("curl/8.5.0")
	sess, _ := h.mgr.Get(firefox.Value)

	page := decode[view.AccountPage](t, h.do(get("/app/settings/account", sess)))
	var agents []string
	var other string
	for _, row := range page.Sessions {
		agents = append(agents, string(row.Agent))
		if strings.Contains(row.End.Href, firefox.Value) {
			t.Fatal("a session is named by its cookie")
		}
		if row.Current && row.End.Enabled {
			t.Error("the current session offers End")
		}
		if row.Agent == "Chrome · Android" {
			other = row.End.Href
		}
	}
	if len(page.Sessions) != 3 || !strings.Contains(strings.Join(agents, "|"), "Firefox · Windows") {
		t.Fatalf("sessions = %v", agents)
	}
	if rec := h.do(postForm(other, sess, url.Values{})); rec.Code != 303 {
		t.Fatalf("end: %d", rec.Code)
	}
	if n := h.mgr.LiveCount(sess.AccountID()); n != 2 {
		t.Errorf("live after End = %d", n)
	}
	if rec := h.do(postForm(actionHref(t, page.Actions, "account.session.others"), sess, url.Values{})); rec.Code != 303 {
		t.Fatalf("end others: %d", rec.Code)
	}
	if n := h.mgr.LiveCount(sess.AccountID()); n != 1 {
		t.Errorf("live after End every other = %d", n)
	}
}

// Owner's decision 19.09.2026: Delete account asks the password first (the
// unlock sheet), then deletes; the last administrator stays.
func TestAccountDeletesItself(t *testing.T) {
	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	h.invite(adminID, "bob", "bobpass123")
	bob := h.login("bob", "bobpass123")

	page := decode[view.AccountPage](t, h.do(get("/app/settings/account", bob)))
	if len(page.Danger) != 1 || page.Danger[0].Method != "GET" || !strings.HasSuffix(page.Danger[0].Href, "/app/settings/unlock") {
		t.Fatalf("danger before unlock = %+v", page.Danger)
	}
	if rec := h.do(postForm("/app/settings/account?op=delete", bob, url.Values{})); rec.Code != 200 {
		t.Fatalf("delete without the password: %d", rec.Code)
	}
	if snap := h.st.Snapshot(); snap.UserByLogin("bob") == nil {
		t.Fatal("deleted without the password")
	}
	h.do(fragment(postForm("/app/settings/unlock", bob, url.Values{"password": {"bobpass123"}}), "sheet"))
	page = decode[view.AccountPage](t, h.do(get("/app/settings/account", bob)))
	rec := h.do(postForm(actionHref(t, page.Danger, "account.delete"), bob, url.Values{}))
	if rec.Code != 303 || rec.Header().Get("Location") != "/login" {
		t.Fatalf("delete: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	if snap := h.st.Snapshot(); snap.UserByLogin("bob") != nil {
		t.Error("bob is still there")
	}

	alice := h.login("alice", "password123")
	h.do(fragment(postForm("/app/settings/unlock", alice, url.Values{"password": {"password123"}}), "sheet"))
	page = decode[view.AccountPage](t, h.do(postForm("/app/settings/account?op=delete", alice, url.Values{})))
	if len(page.Notices) != 1 || !strings.Contains(string(page.Notices[0].Body), "last administrator") {
		t.Errorf("notices = %+v", page.Notices)
	}
}
