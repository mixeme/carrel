package web_test

import (
	"context"
	"errors"
	"net"
	"net/url"
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/mailbox/mailboxtest"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// homeFolders are the folders of the frames' home mailbox (s-mailbox-24,
// -26): Sent, Drafts and Trash announced, Архив and Junk not.
var homeFolders = []string{
	`(\HasNoChildren) "/" INBOX`,
	`(\HasNoChildren \Sent) "/" Sent`,
	`(\HasNoChildren \Drafts) "/" Drafts`,
	`(\HasNoChildren \Trash) "/" Trash`,
	`(\HasNoChildren) "/" &BBAEQARFBDgEMg-`,
	`(\HasNoChildren) "/" Junk`,
}

// mailboxHarness is a harness with people's mailboxes on, alice signed in,
// and the mail servers of home.example: its SRV records name
// mail.home.example at 993 and 465, as in s-mailbox-26.
func mailboxHarness(t *testing.T) (*harness, *session.Session, *mailboxtest.Servers) {
	t.Helper()
	h := newHarness(t)
	h.admin("alice", "password123")
	alice := h.login("alice", "password123")
	h.cfg.MailboxEnabled = true
	ms := mailboxtest.Start(t, "mix@home.example", "secret", nil, false)
	ms.IMAP = mailboxtest.Announcing(t, homeFolders...)
	ms.SetSize(25 << 20)
	p := h.srv.MailProber()
	p.SRV = func(_ context.Context, service, domain string) ([]*net.SRV, error) {
		if domain != "home.example" {
			return nil, errors.New("no such host")
		}
		switch service {
		case "imaps":
			return []*net.SRV{{Target: "mail.home.example.", Port: 993}}, nil
		case "submissions":
			return []*net.SRV{{Target: "mail.home.example.", Port: 465}}, nil
		}
		return nil, errors.New("no such host")
	}
	p.Fetch = func(context.Context, string) ([]byte, error) { return nil, errors.New("404 Not Found") }
	p.Dial = ms.Dial
	return h, alice, ms
}

func mbRow(f *view.Form, name string) *view.FormRow {
	if f == nil {
		return nil
	}
	for i := range f.Sets {
		for j := range f.Sets[i].Rows {
			if f.Sets[i].Rows[j].Name == name {
				return &f.Sets[i].Rows[j]
			}
		}
	}
	return nil
}

// Connecting a mailbox (M-3, s-mailbox-22, -24, -26): the address first, the
// servers found by SRV and shown before the password, IMAP and SMTP signed
// in to, the roles the server did not name asked once, then the card.
func TestAMailboxIsConnected(t *testing.T) {
	h, alice, _ := mailboxHarness(t)

	page := decode[view.MailboxPage](t, h.do(get("/app/settings/mailboxes/new", alice)))
	if page.Title != "Connect a mailbox" || mbRow(page.Form, "address") == nil || mbRow(page.Form, "password") == nil ||
		len(page.Actions) != 3 || page.Actions[0].Label != "Connect" {
		t.Fatalf("dialog = %+v", page)
	}

	// No password: the servers are found and shown, the password asked.
	page = decode[view.MailboxPage](t, h.do(postForm("/app/settings/mailboxes/new", alice, url.Values{"address": {"mix@home.example"}})))
	if page.Servers == nil || len(page.Servers.Lines) != 2 || page.Servers.Lines[0].Address != "mail.home.example:993" ||
		page.Servers.Lines[1].Security != "TLS" || page.Servers.From != "found by SRV" {
		t.Fatalf("servers = %+v", page.Servers)
	}
	if r := mbRow(page.Form, "password"); r == nil || r.Error == "" {
		t.Errorf("password = %+v: asked", r)
	}
	if r := mbRow(page.Form, "label"); r == nil || r.Value != "home" {
		t.Errorf("label = %+v: home of home.example", r)
	}

	rec := h.do(postForm("/app/settings/mailboxes/new", alice, url.Values{"address": {"mix@home.example"}, "password": {"secret"}}))
	if rec.Code != 303 || !strings.HasSuffix(rec.Header().Get("Location"), "/folders") {
		t.Fatalf("connect: %d %s %s", rec.Code, rec.Header().Get("Location"), rec.Body)
	}
	folders := rec.Header().Get("Location")
	key, _ := alice.DataKey()
	blob, _ := h.users.Catalog(alice.AccountID(), key)
	if len(blob.Mailboxes) != 1 {
		t.Fatalf("mailboxes = %+v", blob.Mailboxes)
	}
	m := blob.Mailboxes[0]
	if m.Folders.Sent != "Sent" || m.Folders.Trash != "Trash" || m.Folders.Archive != "" || m.MaxLetter != 25<<20 ||
		m.IMAP.Port != 993 || m.SMTP.Port != 465 || m.ServersFrom != "found by SRV" || !m.CanSend() {
		t.Fatalf("stored = %+v", m)
	}

	// The question (s-mailbox-24): the two roles, suggested by the names.
	q := decode[view.MailboxFoldersPage](t, h.do(get(folders, alice)))
	arch, spam := mbRow(&q.Form, "archive"), mbRow(&q.Form, "spam")
	if q.Title != "Which folders are these?" || q.Subtitle != "home did not say where it keeps its archive and spam." ||
		arch == nil || arch.Value != "Архив" || arch.Hint != "Suggested by the name." || spam == nil || spam.Value != "Junk" ||
		mbRow(&q.Form, "sent") != nil || mbRow(&q.Form, "sentCopy") == nil {
		t.Fatalf("question = %+v", q)
	}
	if rec := h.do(postForm(folders, alice, url.Values{"archive": {"Архив"}, "spam": {"Junk"}})); rec.Code != 303 {
		t.Fatalf("folders: %d %s", rec.Code, rec.Body)
	}

	// The card (s-mailbox-26).
	conns := decode[view.ConnectionsPage](t, h.do(get("/app/settings/connections", alice)))
	if len(conns.Mailboxes) != 1 {
		t.Fatalf("cards = %+v", conns.Mailboxes)
	}
	c := conns.Mailboxes[0]
	if c.Label != "home" || c.Address != "mix@home.example" || c.IMAP != "mail.home.example:993" || c.Limit != "up to 25 MB" ||
		c.SendRefused || c.Password == nil || c.SentToggle == nil || len(c.Actions) != 3 {
		t.Errorf("card = %+v", c)
	}
	var how []string
	for _, f := range c.Folders {
		how = append(how, string(f.Label)+" "+string(f.Folder)+" "+string(f.How))
	}
	if strings.Join(how, "; ") != "Sent Sent announced by the server; Drafts Drafts announced by the server; Archive Архив you chose it; Spam Junk you chose it; Trash Trash announced by the server" {
		t.Errorf("roles = %q", how)
	}
	if !strings.Contains(runsText(conns.Subtitle), "1 mailbox") || !slices.ContainsFunc(conns.Actions, func(a view.Action) bool { return a.Label == "Connect a mailbox" }) {
		t.Errorf("subtitle %q, actions %+v", runsText(conns.Subtitle), conns.Actions)
	}
}


// Reading without sending is kept (s-mailbox-25, -26): the mailbox is saved,
// the outcome says so with the trace; the card is in alarm, Test again, and
// what SMTP answered under it. Test passing puts it right.
func TestAMailboxThatDoesNotSendIsKept(t *testing.T) {
	h, alice, ms := mailboxHarness(t)
	ms.RefuseSMTP(true)
	page := decode[view.MailboxPage](t, h.do(postForm("/app/settings/mailboxes/new", alice, url.Values{"address": {"mix@home.example"}, "password": {"secret"}, "label": {"office"}})))
	if len(page.Saved) == 0 || runsText(page.Saved) != "Saved. Reading works, sending does not: the server refused the password for sending." ||
		page.Trace == nil || page.Subtitle != "office · mix@home.example" || len(page.Actions) != 3 || page.Actions[0].Label != "Done" {
		t.Fatalf("outcome = %+v", page)
	}
	last := page.Trace.Steps[len(page.Trace.Steps)-1]
	if last.Name != "smtp" || !strings.Contains(string(last.Detail), "AUTH PLAIN → 535 5.7.8 Error: authentication failed") {
		t.Errorf("trace = %+v", page.Trace.Steps)
	}
	c := decode[view.ConnectionsPage](t, h.do(get("/app/settings/connections", alice))).Mailboxes[0]
	if !c.SendRefused || c.Actions[0].Label != "Test again" || c.Note != "In «From» it stays off until Test passes." || c.Trace == nil {
		t.Fatalf("card = %+v", c)
	}
	ms.RefuseSMTP(false)
	if rec := h.do(postForm("/app/settings/mailboxes/"+c.ID+"/test", alice, url.Values{})); rec.Code != 303 {
		t.Fatalf("test: %d", rec.Code)
	}
	if c := decode[view.ConnectionsPage](t, h.do(get("/app/settings/connections", alice))).Mailboxes[0]; c.SendRefused || c.Actions[0].Label != "Test" {
		t.Errorf("after Test: %+v", c)
	}
}

// A refused password is the dialog again, nothing saved; Microsoft is
// refused by its domain before anything is asked (s-mailbox-23).
func TestAMailboxRefused(t *testing.T) {
	h, alice, _ := mailboxHarness(t)
	// The server in memory checks the password; the announcing one lets
	// anyone in.
	checking := mailboxtest.Start(t, "mix@home.example", "secret", nil, false)
	h.srv.MailProber().Dial = checking.Dial
	page := decode[view.MailboxPage](t, h.do(postForm("/app/settings/mailboxes/new", alice, url.Values{"address": {"mix@home.example"}, "password": {"wrong"}})))
	if r := mbRow(page.Form, "password"); r == nil || r.Error != "The server refused the login and password." || page.Trace == nil {
		t.Errorf("refused = %+v", page)
	}
	page = decode[view.MailboxPage](t, h.do(postForm("/app/settings/mailboxes/new", alice, url.Values{"address": {"mix.home@outlook.com"}, "password": {"x"}})))
	if page.Provider == nil || page.Provider.Role != view.NoticeAlert || page.Actions[0].Enabled || mbRow(page.Form, "password") != nil {
		t.Errorf("Microsoft = %+v", page)
	}
	key, _ := alice.DataKey()
	if blob, _ := h.users.Catalog(alice.AccountID(), key); len(blob.Mailboxes) != 0 {
		t.Errorf("saved: %+v", blob.Mailboxes)
	}
}

// Off on the instance (M-2, s-mailbox-28): the addresses answer as missing,
// the cards stay with Remove only and the line above them, Remove works.
func TestMailboxesOffKeepTheCards(t *testing.T) {
	h, alice, _ := mailboxHarness(t)
	if rec := h.do(postForm("/app/settings/mailboxes/new", alice, url.Values{"address": {"mix@home.example"}, "password": {"secret"}})); rec.Code != 303 {
		t.Fatalf("connect: %d", rec.Code)
	}
	h.cfg.MailboxEnabled = false
	if rec := h.do(get("/app/settings/mailboxes/new", alice)); rec.Code != 404 {
		t.Errorf("new while off: %d", rec.Code)
	}
	conns := decode[view.ConnectionsPage](t, h.do(get("/app/settings/connections", alice)))
	if conns.MailboxesOff == "" || len(conns.Mailboxes) != 1 {
		t.Fatalf("off = %+v", conns)
	}
	c := conns.Mailboxes[0]
	if c.Off != "not used while mail is off" || len(c.Actions) != 1 || c.Actions[0].Label != "Remove" || c.Password != nil || len(c.Folders) != 0 {
		t.Errorf("card = %+v", c)
	}
	if slices.ContainsFunc(conns.Actions, func(a view.Action) bool { return a.Label == "Connect a mailbox" }) {
		t.Error("Connect a mailbox while off")
	}
	if rec := h.do(postForm("/app/settings/mailboxes/"+c.ID+"/remove", alice, url.Values{})); rec.Code != 303 {
		t.Fatalf("remove: %d", rec.Code)
	}
	key, _ := alice.DataKey()
	if blob, _ := h.users.Catalog(alice.AccountID(), key); len(blob.Mailboxes) != 0 {
		t.Errorf("kept: %+v", blob.Mailboxes)
	}
}

// The edit (s-mailbox-27): name in From by default the profile's, the
// signature, the autosave of drafts into its Drafts; kept without a check
// when neither the password nor the servers changed.
func TestAMailboxIsEdited(t *testing.T) {
	h, alice, _ := mailboxHarness(t)
	h.do(postForm("/app/settings/mailboxes/new", alice, url.Values{"address": {"mix@home.example"}, "password": {"secret"}}))
	key, _ := alice.DataKey()
	blob, _ := h.users.Catalog(alice.AccountID(), key)
	id := blob.Mailboxes[0].ID
	page := decode[view.MailboxPage](t, h.do(get("/app/settings/mailboxes/"+id+"/edit", alice)))
	if page.Title != "home" || page.Subtitle != "mix@home.example · mailbox" || mbRow(page.Form, "fromName") == nil ||
		mbRow(page.Form, "autosave").Hint != "Into home · Drafts, the new version replacing the last." ||
		page.Servers == nil || page.Servers.From != "found by SRV" || page.Actions[0].Label != "Save" {
		t.Fatalf("edit = %+v", page)
	}
	rec := h.do(postForm("/app/settings/mailboxes/"+id+"/edit", alice, url.Values{"label": {"дом"}, "fromName": {"Михаил Енученко"},
		"signature": {"Михаил Енученко\r\nвёрстка"}, "autosave": {"on"}}))
	if rec.Code != 303 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body)
	}
	blob, _ = h.users.Catalog(alice.AccountID(), key)
	if m := blob.Mailboxes[0]; m.Label != "дом" || m.FromName != "Михаил Енученко" || m.Signature != "Михаил Енученко\nвёрстка" || !m.Autosave ||
		m.Credentials.Password != "secret" {
		t.Errorf("stored = %+v", m)
	}
}
