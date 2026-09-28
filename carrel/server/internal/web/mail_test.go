package web_test

import (
	"io"
	"mime/quotedprintable"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/mail/mailtest"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

type testFragment struct {
	Body view.MailTestResult `json:"body"`
}

// relayed is a harness whose instance sends through a relay in memory.
func (h *harness) relayed() *mailtest.Server {
	h.t.Helper()
	relay := mailtest.New(h.t)
	err := h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		st.Settings.SMTP = store.SMTPSettings{Host: relay.Host, Port: relay.Port, Security: "none", From: "noreply@carrel.example", FromName: "Carrel"}
		return nil, nil
	})
	if err != nil {
		h.t.Fatal(err)
	}
	return relay
}

func (h *harness) confirmAddress(login, email string) {
	h.t.Helper()
	err := h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByLogin(login)
		u.Email, u.EmailConfirmed = email, true
		return nil, nil
	})
	if err != nil {
		h.t.Fatal(err)
	}
}

// letterBody is the text of a letter, quoted-printable undone.
func letterBody(t *testing.T, l mailtest.Letter) (mail.Header, string) {
	t.Helper()
	msg, err := mail.ReadMessage(strings.NewReader(l.Data))
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(quotedprintable.NewReader(msg.Body))
	if err != nil {
		t.Fatal(err)
	}
	return msg.Header, string(b)
}

// S-26: the relay is an administrator's setting. The password is written
// and never shown back; an empty field keeps it; the journal names the
// relay and not the password. The test letter shows the relay's answer.
func TestAdminSetsRelayAndSendsATestLetter(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	alice := h.login("alice", "password123")
	relay := mailtest.New(t)
	relay.NeedAuth = "carrel"

	form := url.Values{"host": {relay.Host}, "port": {strconv.Itoa(relay.Port)}, "security": {"none"},
		"username": {"carrel"}, "password": {"relay-secret"}, "from": {"noreply@carrel.example"}, "fromName": {"Carrel"}}
	if rec := h.do(postForm("/admin/settings", alice, form)); rec.Code != 303 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body)
	}
	form.Set("password", "")
	if rec := h.do(postForm("/admin/settings", alice, form)); rec.Code != 303 {
		t.Fatalf("save again: %d", rec.Code)
	}
	snap := h.st.Snapshot()
	if snap.Settings.SMTP.Password != "relay-secret" {
		t.Errorf("an empty field dropped the password")
	}
	rec := h.do(get("/admin/settings", alice))
	if strings.Contains(rec.Body.String(), "relay-secret") {
		t.Fatal("the page shows the relay's password")
	}
	for _, e := range snap.Audit {
		if strings.Contains(e.Detail, "relay-secret") {
			t.Fatalf("the journal holds the password: %+v", e)
		}
	}
	page := decode[view.AdminSettingsPage](t, rec)
	if page.SMTP.Sets[0].Legend != "Mail" || formFields(page.SMTP).Get("host") != relay.Host {
		t.Errorf("SMTP form = %+v", page.SMTP)
	}
	var senders []string
	for _, row := range page.SMTP.Sets[0].Rows {
		if row.Name == "sender" {
			for _, o := range row.Options {
				senders = append(senders, string(o.Label))
			}
		}
	}
	if strings.Join(senders, " ") != "noreply@carrel.example login@carrel.example" {
		t.Errorf("sender options = %v", senders)
	}
	form.Set("sender", "login")
	if rec := h.do(postForm("/admin/settings", alice, form)); rec.Code != 303 {
		t.Fatalf("save the login sender: %d", rec.Code)
	}
	if snap := h.st.Snapshot(); snap.Settings.SMTP.Sender != "login" || snap.Settings.SMTP.Password != "relay-secret" {
		t.Errorf("sender = %q", snap.Settings.SMTP.Sender)
	}

	result := decode[testFragment](t, h.do(fragment(postForm("/admin/settings/mail-test", alice, url.Values{"to": {"alice@example.org"}}), "main"))).Body
	if !result.OK || !strings.HasPrefix(string(result.Reply), "250") || strings.Contains(string(result.Reply), "relay-secret") {
		t.Errorf("test = %+v", result)
	}
	if got := relay.Letters(); len(got) != 1 || got[0].To[0] != "alice@example.org" || got[0].From != "noreply@carrel.example" {
		t.Errorf("letters = %+v", got)
	}

	relay.Refuse = "nobody@example.org"
	result = decode[testFragment](t, h.do(fragment(postForm("/admin/settings/mail-test", alice, url.Values{"to": {"nobody@example.org"}}), "main"))).Body
	if result.OK || !strings.Contains(string(result.Reply), "550") {
		t.Errorf("a refusal = %+v", result)
	}
}

func TestAdminRelaySettingsAreChecked(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	alice := h.login("alice", "password123")
	for field, form := range map[string]url.Values{
		"host": {"host": {"smtp://mail.example.org"}, "port": {"587"}, "security": {"starttls"}, "from": {"a@example.org"}},
		"port": {"host": {"mail.example.org"}, "port": {"0"}, "security": {"starttls"}, "from": {"a@example.org"}},
		"from": {"host": {"mail.example.org"}, "port": {"587"}, "security": {"starttls"}, "from": {"Carrel <a@example.org>"}},
	} {
		page := decode[view.AdminSettingsPage](t, h.do(postForm("/admin/settings", alice, form)))
		found := false
		for _, row := range page.SMTP.Sets[0].Rows {
			if row.Name == field && row.Error != "" {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: no error on the field", field)
		}
	}
	snap := h.st.Snapshot()
	if snap.Settings.SMTP.Configured() {
		t.Error("a refused relay was stored")
	}
}

// The invitation goes by letter with a link that works from outside, and
// the list says how it went.
func TestInvitationIsMailed(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	alice := h.login("alice", "password123")
	relay := h.relayed()
	relay.Refuse = "carol@example.org"

	for _, who := range []string{"bob", "carol"} {
		if rec := h.do(postForm("/admin/invites", alice, url.Values{"action": {"create"}, "login": {who}, "email": {who + "@example.org"}})); rec.Code >= 400 {
			t.Fatalf("invite %s: %d", who, rec.Code)
		}
	}
	snap := h.st.Snapshot()
	var secret string
	for _, inv := range snap.Invites {
		if inv.Login == "bob" {
			secret = inv.Secret
		}
	}
	letters := relay.Letters()
	if len(letters) != 1 {
		t.Fatalf("letters = %d", len(letters))
	}
	_, body := letterBody(t, letters[0])
	if !strings.Contains(body, "http://example.com/invite/"+secret) {
		t.Errorf("no absolute link in:\n%s", body)
	}
	page := decode[view.AdminInvitesPage](t, h.do(get("/admin/invites", alice)))
	sent := map[string]string{}
	for _, row := range page.Rows {
		sent[string(row.Login)] = string(row.Sent)
	}
	if sent["bob"] != "sent" || sent["carol"] != "failed — 550 no such mailbox" {
		t.Errorf("sent = %v", sent)
	}
}

// The wipe link goes to the confirmed address and nowhere else, and the
// page says it went.
func TestForgotLinkIsMailedToTheConfirmedAddress(t *testing.T) {
	h := newHarness(t)
	adminID := h.admin("alice", "password123")
	h.invite(adminID, "bob", "bobpass123")
	h.confirmAddress("bob", "bob@example.org")
	relay := h.relayed()

	page := decode[view.ForgotPage](t, h.do(h.anonPostForm("/forgot", "/forgot", url.Values{"login": {"bob"}})))
	if !page.Sent {
		t.Fatalf("page = %+v", page)
	}
	letters := relay.Letters()
	if len(letters) != 1 || letters[0].To[0] != "bob@example.org" {
		t.Fatalf("letters = %+v", letters)
	}
	header, body := letterBody(t, letters[0])
	if !strings.Contains(body, "http://example.com/forgot/") || header.Get("Reply-To") != "" {
		t.Errorf("letter:\n%s", body)
	}
}

// S-10.5: a recovery is mailed to the person, always — and the temporary
// password is not in the letter: it goes hand to hand.
func TestEscrowRecoveryIsMailedWithoutThePassword(t *testing.T) {
	h := newHarness(t)
	const master = "sixteen-char-pass!"
	h.enableEscrow("alice", "password123", master)
	alice := h.login("alice", "password123")
	snap := h.st.Snapshot()
	h.invite(snap.UserByLogin("alice").ID, "bob", "bobpass123")
	h.confirmAddress("bob", "bob@example.org")
	relay := h.relayed()

	page := decode[view.AdminEscrowPage](t, h.do(postForm("/admin/escrow", alice, url.Values{"action": {"recover"}, "login": {"bob"}, "master": {master}})))
	if page.Result == nil || page.Result.Mailed != "sent" {
		t.Fatalf("result = %+v", page.Result)
	}
	letters := relay.Letters()
	if len(letters) != 1 || letters[0].To[0] != "bob@example.org" {
		t.Fatalf("letters = %+v", letters)
	}
	if _, body := letterBody(t, letters[0]); strings.Contains(body, string(page.Result.Password)) || !strings.Contains(body, "alice") {
		t.Errorf("letter:\n%s", body)
	}
}
