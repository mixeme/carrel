//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// S-25.2 live: somebody's Nextcloud folder is subscribed to by the link a
// Nextcloud user sends — /s/{token}, a page for people — and Carrel reads it
// as the share's public WebDAV. The test shares a folder of its own account
// through the OCS API, subscribes to the link as another Carrel user would,
// and takes the share and the folder away after.
func TestNextcloudShareLinkIsSubscribedTo(t *testing.T) {
	endpoint, user, pass := nextcloudConfig(t)
	ctx := context.Background()
	base, err := url.Parse(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	host := base.Scheme + "://" + base.Host
	name := fmt.Sprintf("carrel-share-%d-%d", os.Getpid(), time.Now().Unix()%100000)
	folder := "/remote.php/dav/files/" + url.PathEscape(nextcloudUID(ctx, t, endpoint, user, pass)) + "/" + name + "/"
	liveRequest(ctx, t, endpoint, user, pass, "MKCOL", folder, "")
	t.Cleanup(func() { deleteLive(t, endpoint, user, pass, folder) })
	liveRequest(ctx, t, endpoint, user, pass, "PUT", folder+"hello.txt", "shared by the live test")

	share := ocsShare(t, host, user, pass, "/"+name)
	t.Cleanup(func() { ocsUnshare(t, host, user, pass, share.ID) })
	if !strings.Contains(share.URL, "/s/") {
		t.Fatalf("share url = %q", share.URL)
	}

	app := newLiveApp(t)
	rec := app.post("/app/settings/connections?op=feed.subscribe&kind=folder", url.Values{"url": {share.URL}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("subscribe: %d %s", rec.Code, rec.Body.String())
	}
	key, _ := app.sess.DataKey()
	blob, _ := app.users.Catalog(app.sess.AccountID(), key)
	var sub sealed.Subscription
	if len(blob.Subscriptions) == 1 {
		sub = blob.Subscriptions[0]
	}
	if !strings.HasSuffix(sub.URL, "/public.php/webdav/") || sub.Login != share.Token || len(sub.Kinds) != 1 || sub.Kinds[0] != view.KindFile {
		t.Fatalf("subscription = %+v", sub)
	}
	t.Logf("subscribed to %s as %s", sub.URL, sub.Name)
}

// nextcloudUID is the user's ID, which names the file home: it is not the
// login when the login is an address.
func nextcloudUID(ctx context.Context, t *testing.T, endpoint, user, pass string) string {
	t.Helper()
	c := liveClient(t, endpoint, user, pass)
	u, _ := url.Parse(endpoint)
	ms, err := c.PropFind(ctx, u.Path, dav.DepthZero, []xml.Name{{Space: "DAV:", Local: "current-user-principal"}})
	if err != nil || len(ms.Responses) == 0 {
		t.Fatalf("principal: %v", err)
	}
	raw := string(ms.Responses[0].Props[xml.Name{Space: "DAV:", Local: "current-user-principal"}])
	i := strings.Index(raw, "/principals/users/")
	if i < 0 {
		t.Fatalf("principal = %q", raw)
	}
	rest := raw[i+len("/principals/users/"):]
	uid, _, _ := strings.Cut(rest, "/")
	uid, _ = url.PathUnescape(uid)
	return uid
}

type ocsShareData struct {
	ID    json.Number `json:"id"`
	URL   string      `json:"url"`
	Token string      `json:"token"`
}

func ocsShare(t *testing.T, host, user, pass, path string) struct{ ID, URL, Token string } {
	t.Helper()
	form := url.Values{"path": {path}, "shareType": {"3"}, "permissions": {"1"}}
	req, _ := http.NewRequest(http.MethodPost, host+"/ocs/v2.php/apps/files_sharing/api/v1/shares?format=json", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("OCS-APIRequest", "true")
	req.SetBasicAuth(user, pass)
	resp, err := testGuard(t).HTTPClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	var out struct {
		OCS struct {
			Data ocsShareData `json:"data"`
		} `json:"ocs"`
	}
	if resp.StatusCode != http.StatusOK || json.Unmarshal(body, &out) != nil || out.OCS.Data.Token == "" {
		t.Fatalf("share: %s %s", resp.Status, body)
	}
	return struct{ ID, URL, Token string }{out.OCS.Data.ID.String(), out.OCS.Data.URL, out.OCS.Data.Token}
}

func ocsUnshare(t *testing.T, host, user, pass, id string) {
	req, _ := http.NewRequest(http.MethodDelete, host+"/ocs/v2.php/apps/files_sharing/api/v1/shares/"+id+"?format=json", nil)
	req.Header.Set("OCS-APIRequest", "true")
	req.SetBasicAuth(user, pass)
	if resp, err := testGuard(t).HTTPClient().Do(req); err == nil {
		resp.Body.Close()
	}
}

func deleteLive(t *testing.T, endpoint, user, pass, path string) {
	u, _ := url.Parse(endpoint)
	u.Path = path
	req, _ := http.NewRequest(http.MethodDelete, u.String(), nil)
	req.SetBasicAuth(user, pass)
	if resp, err := testGuard(t).HTTPClient().Do(req); err == nil {
		resp.Body.Close()
	}
}
