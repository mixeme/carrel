package web_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/mailbox/mailboxtest"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

// yandexAutoconfig is Yandex's file of autoconfig (s-mailbox-22): IMAP and
// SMTP over TLS.
const yandexAutoconfig = `<clientConfig version="1.1"><emailProvider id="yandex.ru"><domain>yandex.ru</domain>
<incomingServer type="imap"><hostname>imap.yandex.ru</hostname><port>993</port><socketType>SSL</socketType><username>%EMAILADDRESS%</username></incomingServer>
<outgoingServer type="smtp"><hostname>smtp.yandex.ru</hostname><port>465</port><socketType>SSL</socketType><username>%EMAILADDRESS%</username></outgoingServer>
</emailProvider></clientConfig>`

// mailboxWorld is the mailboxes of the frames (s-mailbox-22…28): mix's home
// mailbox, whose server names Sent, Drafts and Trash and not the archive and
// spam; the office's, whose server keeps a separate password for sending;
// Yandex found by its autoconfig file.
func mailboxWorld(t *testing.T, app *mockupApp) (home, office *mailboxtest.Servers) {
	t.Helper()
	ownMailboxes(t, app, true)
	home = mailboxtest.Start(t, "mix@home.example", "secret", nil, false)
	home.IMAP = mailboxtest.Announcing(t, homeFolders...)
	home.SetSize(25 << 20)
	office = mailboxtest.Start(t, "m.yenuchenko@office.example", "secret", nil, false)
	office.IMAP = mailboxtest.Announcing(t)
	office.SetSize(52428800)
	office.RefuseSMTP(true)
	yandex := mailboxtest.Start(t, "mix.letters@yandex.ru", "secret", nil, false)
	srv := map[string][]*net.SRV{
		"imaps home.example":         {{Target: "mail.home.example.", Port: 993}},
		"submissions home.example":   {{Target: "mail.home.example.", Port: 465}},
		"imaps office.example":       {{Target: "mail.office.example.", Port: 993}},
		"submissions office.example": {{Target: "mail.office.example.", Port: 465}},
	}
	p := app.h.srv.MailProber()
	p.SRV = func(_ context.Context, service, domain string) ([]*net.SRV, error) {
		if recs, ok := srv[service+" "+domain]; ok {
			return recs, nil
		}
		return nil, errors.New("no such host")
	}
	p.Fetch = func(_ context.Context, address string) ([]byte, error) {
		if strings.HasPrefix(address, "https://autoconfig.yandex.ru/") {
			return []byte(yandexAutoconfig), nil
		}
		return nil, errors.New("404 Not Found")
	}
	p.Dial = func(ctx context.Context, host string, port int, implicitTLS bool, pin string) (net.Conn, error) {
		switch {
		case strings.HasSuffix(host, "home.example"):
			return home.Dial(ctx, host, port, implicitTLS, pin)
		case strings.HasSuffix(host, "office.example"):
			return office.Dial(ctx, host, port, implicitTLS, pin)
		case strings.HasSuffix(host, "yandex.ru"):
			return yandex.Dial(ctx, host, port, implicitTLS, pin)
		}
		return nil, fmt.Errorf("no mail server %s", host)
	}
	return home, office
}

// mailboxID is the {mailbox} of the label, mapped to the fixtures' «m-…».
func mailboxID(t *testing.T, app *mockupApp, label string) string {
	t.Helper()
	key, _ := app.sess.DataKey()
	blob, _ := app.h.users.Catalog(app.sess.AccountID(), key)
	for _, m := range blob.Mailboxes {
		if m.Label == label {
			app.byAccount[m.ID] = "m-" + label
			return m.ID
		}
	}
	t.Fatalf("no mailbox %s", label)
	return ""
}

// The mailboxes' screens (s-mailbox-22…28) on the set of the frames.
func TestFixtureMailboxes(t *testing.T) {
	app := newMockupApp(t)
	seedMockupWhole(t, app)
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	settingsState(t, app)
	mailboxWorld(t, app)
	// mix's data key deposited: the dialog says what escrow opens
	// (s-mailbox-22, S-29.2). Set in the state: the service refuses a
	// deposit nobody else could open, and mix is the only administrator.
	if err := app.h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		if !store.EscrowActive(st) {
			t.Fatal("escrow is not on in the set of the frames")
		}
		st.UserByID(app.sess.AccountID()).EscrowDeposited = []byte("sealed copy")
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}

	// Connect a mailbox, the address typed: Yandex found by its file, the
	// app password said before the field (s-mailbox-22).
	app.compare(t, "app-settings-mailboxes-new/normal.json", app.h.do(get("/app/settings/mailboxes/new?"+url.Values{
		"address": {"mix.letters@yandex.ru"}}.Encode(), app.sess)))
	// Microsoft by its domain (s-mailbox-23).
	app.compare(t, "app-settings-mailboxes-new/closed.json", app.h.do(get("/app/settings/mailboxes/new?"+url.Values{
		"address": {"mix.home@outlook.com"}}.Encode(), app.sess)))
	// The office's mailbox reads and does not send: saved, said so
	// (s-mailbox-25).
	saved := app.h.do(postForm("/app/settings/mailboxes/new", app.sess, url.Values{
		"address": {"m.yenuchenko@office.example"}, "label": {"office"}, "colour": {"#3f6e8c"}, "password": {"secret"}}))
	mailboxID(t, app, "office") // its ID in the answer is «m-office»
	app.compare(t, "app-settings-mailboxes-new/saved.json", saved)

	// home: its server did not name the archive and spam (s-mailbox-24).
	rec := app.h.do(postForm("/app/settings/mailboxes/new", app.sess, url.Values{
		"address": {"mix@home.example"}, "label": {"home"}, "colour": {"#a05a2c"}, "password": {"secret"}}))
	if rec.Code != 303 {
		t.Fatalf("home: %d %s", rec.Code, rec.Body)
	}
	home := mailboxID(t, app, "home")
	app.compare(t, "app-settings-mailboxes-folders/normal.json", app.h.do(get("/app/settings/mailboxes/"+home+"/folders", app.sess)))
	if rec := app.h.do(postForm("/app/settings/mailboxes/"+home+"/folders", app.sess, url.Values{"archive": {"Архив"}, "spam": {"Junk"}, "sentCopy": {"on"}})); rec.Code != 303 {
		t.Fatalf("folders: %d", rec.Code)
	}
	if rec := app.h.do(postForm("/app/settings/mailboxes/"+home+"/edit", app.sess, url.Values{"label": {"home"}, "colour": {"#a05a2c"},
		"fromName": {"Михаил Енученко"}, "signature": {"Михаил Енученко\nвёрстка и допечатная подготовка"}})); rec.Code != 303 {
		t.Fatalf("edit: %d %s", rec.Code, rec.Body)
	}

	// The edit of home (s-mailbox-27).
	app.compare(t, "app-settings-mailboxes-edit/normal.json", app.h.do(get("/app/settings/mailboxes/"+home+"/edit", app.sess)))
	// Connections with the mailboxes (s-mailbox-26) has no fixture: the
	// frame folds the servers' and the feeds' cards to their address lines,
	// and a screen of the whole set is not that frame (Typecase's task 300
	// p. 6). The cards are held to the frame's words here.
	conns := decode[view.ConnectionsPage](t, app.h.do(get("/app/settings/connections", app.sess)))
	var cards []string
	for _, m := range conns.Mailboxes {
		cards = append(cards, fmt.Sprintf("%s %s %s %s %v %d", m.Label, m.IMAP, m.SMTP, m.Limit, m.SendRefused, len(m.Folders)))
	}
	if strings.Join(cards, "; ") != "office mail.office.example:993 mail.office.example:465  true 5; home mail.home.example:993 mail.home.example:465 up to 25 MB false 5" {
		t.Errorf("cards = %q", cards)
	}
	// Off on the instance (s-mailbox-28).
	ownMailboxes(t, app, false)
	page := decode[view.ConnectionsPage](t, app.h.do(get("/app/settings/connections", app.sess)))
	if page.MailboxesOff == "" {
		t.Fatal("no line of mail off")
	}
	app.compare(t, "app-settings-connections/mailboxes-off.json", app.h.do(get("/app/settings/connections", app.sess)))
}

// ownMailboxes is the administrator's tick «Users’ own mailboxes» as
// stored: the settings of the frames' instance were saved with the form,
// and what is stored stands over CARREL_MAILBOX_ENABLED (M-2).
func ownMailboxes(t *testing.T, app *mockupApp, on bool) {
	t.Helper()
	if err := app.h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		st.Settings.Mailboxes = &on
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
}
