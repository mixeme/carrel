//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/discover"
	"carrel.dev/carrel/server/view"
)

// A book larger than one answer may be (CARREL_DAV_MAX_RESPONSE_BYTES) opens
// on a real server: 260 cards with a 45 KB photo each are about 12 MB, and the
// list read them in parts rather than calling the server unavailable.
func TestBaikalBigBook(t *testing.T) {
	endpoint, user, pass := baikalConfig(t)
	ctx := context.Background()
	g := testGuard(t)
	client, err := dav.NewClient(g, endpoint, user, pass, "")
	if err != nil {
		t.Fatal(err)
	}
	res, _, err := discover.Run(ctx, g, endpoint, user, pass, "")
	if err != nil || res.AddressBookHome == "" {
		t.Fatalf("discovery: %v %+v", err, res)
	}
	suffix := fmt.Sprintf("carrel-big-%d-%d", os.Getpid(), time.Now().Unix()%100000)
	book := makeCollection(ctx, t, client, endpoint, user, pass, "MKCOL",
		strings.TrimSuffix(res.AddressBookHome, "/")+"/"+suffix+"/", `<?xml version="1.0" encoding="utf-8"?>
<D:mkcol xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav">
  <D:set><D:prop>
    <D:resourcetype><D:collection/><C:addressbook/></D:resourcetype>
    <D:displayname>Carrel big book</D:displayname>
  </D:prop></D:set>
</D:mkcol>`)
	photo := strings.Repeat("QUJD", 45_000/4)
	const n = 260
	var wg sync.WaitGroup
	errs := make(chan error, n)
	sem := make(chan struct{}, 8)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			card := fmt.Sprintf("BEGIN:VCARD\r\nVERSION:3.0\r\nUID:big-%03d\r\nFN:Person %03d\r\nPHOTO;ENCODING=b;TYPE=JPEG:%s\r\nEND:VCARD\r\n", i, i, photo)
			if _, err := client.PutCreate(ctx, fmt.Sprintf("%sbig-%03d.vcf", book, i), dav.Typed(strings.NewReader(card), "text/vcard; charset=utf-8")); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("seed: %v", err)
	}

	app := newLiveApp(t)
	redirected(t, "connect", app.post("/app/settings/connections", url.Values{
		"server": {endpoint}, "login": {user}, "password": {pass},
	}))
	key, _ := app.sess.DataKey()
	blob, _ := app.users.Catalog(app.sess.AccountID(), key)
	col := catalogEntry(t, blob, book)
	page := liveDecode[view.ContactsListPage](t, "list", app.get("/app/contacts/"+blob.Connections[0].ID+"/"+url.PathEscape(col.Path)))
	rows := 0
	for _, g := range page.Groups {
		rows += len(g.Rows)
	}
	if rows != n || len(page.Notices) > 0 {
		t.Errorf("rows %d of %d, notices %+v", rows, n, page.Notices)
	}
}
