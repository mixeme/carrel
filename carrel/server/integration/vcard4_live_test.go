//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/discover"
	"carrel.dev/carrel/server/view"
)

// S-30.2 for S-7: a vCard 4.0 card edited through Carrel stays 4.0 and keeps
// what only 4.0 holds. sabre/dav (Baikal, Nextcloud) converts a card to 3.0 for
// a REPORT that names no version; Carrel then wrote 3.0 back and KIND was gone.
// The card is read back asking for 4.0: a card the server holds as sent comes
// without sabre's PRODID, a converted one with it.
func TestRadicaleVCard4Edit(t *testing.T) {
	runVCard4Edit(t, radicaleEndpoint(t), radicaleUser, radicalePass)
}

func TestBaikalVCard4Edit(t *testing.T) {
	endpoint, user, pass := baikalConfig(t)
	runVCard4Edit(t, endpoint, user, pass)
}

func runVCard4Edit(t *testing.T, endpoint, user, pass string) {
	ctx := context.Background()
	g := testGuard(t)
	client, err := dav.NewClient(g, endpoint, user, pass, "")
	if err != nil {
		t.Fatal(err)
	}
	res, trace, err := discover.Run(ctx, g, endpoint, user, pass, "")
	if err != nil || res.AddressBookHome == "" {
		t.Fatalf("discovery: %v %+v %+v", err, res, trace)
	}
	suffix := fmt.Sprintf("carrel-s7v4-%d-%d", os.Getpid(), time.Now().Unix()%100000)
	bookPath := strings.TrimSuffix(res.AddressBookHome, "/") + "/" + suffix + "-book/"
	liveRequest(ctx, t, endpoint, user, pass, "MKCOL", bookPath, `<?xml version="1.0" encoding="utf-8"?>
<D:mkcol xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
  <D:set><D:prop>
    <D:resourcetype><D:collection/><C:addressbook/></D:resourcetype>
    <D:displayname>Carrel S-7 vCard 4.0</D:displayname>
  </D:prop></D:set>
</D:mkcol>`)
	t.Cleanup(func() {
		if err := freshClient(t, endpoint, user, pass).Delete(context.Background(), bookPath, ""); err != nil {
			t.Logf("cleanup %s: %v", bookPath, err)
		}
	})

	uid := "four-" + suffix
	file := bookPath + uid + ".vcf"
	card := "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:" + uid + "\r\nFN:Four Card\r\nKIND:individual\r\nGENDER:F\r\nX-CUSTOM:keep-me\r\nEND:VCARD\r\n"
	if _, err := client.PutCreate(ctx, file, dav.Typed(strings.NewReader(card), "text/vcard; charset=utf-8")); err != nil {
		t.Fatalf("seed: %v", err)
	}

	app := newLiveApp(t)
	redirected(t, "connect", app.post("/app/settings/connections", url.Values{
		"server": {endpoint}, "login": {user}, "password": {pass},
	}))
	key, _ := app.sess.DataKey()
	blob, _ := app.users.Catalog(app.sess.AccountID(), key)
	book := catalogEntry(t, blob, bookPath)
	rec := "/app/contacts/" + blob.Connections[0].ID + "/" + url.PathEscape(book.Path) + "/" + url.PathEscape(uid)

	form := liveForm(liveDecode[view.ContactEditPage](t, "edit", app.get(rec+"/edit")).Form)
	form.Set("fn", "Four Card Edited")
	redirected(t, "edit", app.post(rec+"/edit", form))

	got := rawCard(ctx, t, endpoint, user, pass, file, "text/vcard; version=4.0")
	// The version number is the server's business: SoGo hands the card back as
	// 3.0 whatever the Accept says, and keeps every 4.0 property in it
	// (docs/servers.md, SV-17). What must survive an edit is the properties —
	// that is what a loss report is about (S-7.1).
	mustHave(t, "after the edit", got, "FN:Four Card Edited", "KIND:individual", "GENDER:F", "X-CUSTOM:keep-me", "UID:"+uid)
	if strings.Contains(got, "PRODID:-//Sabre") {
		t.Errorf("the server holds a converted card: Carrel wrote another version back\n%s", got)
	}
}
