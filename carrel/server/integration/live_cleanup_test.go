//go:build integration

package integration

import (
	"context"
	"encoding/xml"
	"io"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/provider/file"
)

// seedFile writes what a test needs before it begins. Carrel never sends a
// write a second time — the server may have taken the first one (SV-14) — and
// that rule stays where it matters: in the paths under test. A seed is not one
// of them. Eleven megabytes across the open internet are cut now and then,
// mid-body, and a setup that lost that race is not a finding about Carrel; the
// name is fresh, so writing it again either works or says it is taken.
func seedFile(ctx context.Context, t *testing.T, p *file.Provider, path string, body func() io.Reader) {
	t.Helper()
	_, err := p.UploadOverwrite(ctx, path, body())
	if err != nil && connectionLost(err) {
		t.Logf("seed %s: %v; writing it again (SV-9, SV-14)", path, err)
		_, err = p.UploadOverwrite(ctx, path, body())
	}
	if err != nil {
		t.Fatalf("seed %s: %v", path, err)
	}
}

// connectionLost reports the endings a connection has when it goes away rather
// than answering: closed while idle, reset, or cut mid-body.
func connectionLost(err error) bool {
	s := err.Error()
	for _, mark := range []string{"EOF", "forcibly closed", "connection reset", "broken pipe"} {
		if strings.Contains(s, mark) {
			return true
		}
	}
	return false
}

// What a live test makes on someone's real account, it takes away — including
// the runs that fail, which is where the leftovers came from: by 16.09.2026 the
// SFTPGo account held folders and files of runs going back weeks.
//
// Two things went wrong and both are answered here. A `defer` does not run when
// the process is killed and, worse, `defer client.Delete(...)` threw the error
// away: the guard never sends a DELETE a second time (SV-14 — the server may
// have taken the first one), so a cleanup that met a closed connection failed
// silently and the file stayed for good. Here the cleanup is sent again once,
// and a cleanup that still fails says so in the test log instead of leaving
// the next person to find it on the server.
func removeLater(t *testing.T, c *dav.Client, paths ...string) {
	t.Helper()
	for _, p := range paths {
		path := p
		t.Cleanup(func() {
			ctx := context.Background()
			err := c.Delete(ctx, path, "")
			if err != nil && strings.Contains(err.Error(), "EOF") {
				err = c.Delete(ctx, path, "")
			}
			if err != nil && !file.NotFound(err) {
				t.Logf("left on the server: %s (%v)", path, err)
			}
		})
	}
}

// makeCollection creates one and answers with the address it actually got. A
// server may put it somewhere of its own (docs/servers.md, SV-19): Yandex
// names a new calendar events-<id> whatever was asked for, and a cleanup aimed
// at the asked-for address then deletes nothing. That is how four calendars
// were left on a real account — and nobody heard it, because the cleanups
// wrote `_ = client.Delete(...)`. The cleanup goes to removeLater, which sends
// it again and says out loud what it could not take away.
func makeCollection(ctx context.Context, t *testing.T, client *dav.Client, endpoint, user, pass, method, want, body string) string {
	t.Helper()
	home := want[:strings.LastIndex(strings.TrimSuffix(want, "/"), "/")+1]
	before := collectionChildren(ctx, t, client, home)
	liveRequest(ctx, t, endpoint, user, pass, method, want, body)

	resourceType := []xml.Name{{Space: "DAV:", Local: "resourcetype"}}
	if _, err := client.PropFind(ctx, want, dav.DepthZero, resourceType); err == nil {
		removeLater(t, client, want)
		return want
	}
	var fresh []string
	for p := range collectionChildren(ctx, t, client, home) {
		if !before[p] {
			fresh = append(fresh, p)
		}
	}
	if len(fresh) != 1 {
		t.Fatalf("%s %s: nothing at that address and %d new collections in %s — %v",
			method, want, len(fresh), home, fresh)
	}
	t.Logf("%s went to %s, not %s (SV-19)", method, fresh[0], want)
	removeLater(t, client, fresh[0])
	return fresh[0]
}

// collectionChildren is the set of collections one level under home.
func collectionChildren(ctx context.Context, t *testing.T, client *dav.Client, home string) map[string]bool {
	t.Helper()
	ms, err := client.PropFind(ctx, home, dav.DepthOne, []xml.Name{{Space: "DAV:", Local: "resourcetype"}})
	if err != nil {
		t.Fatalf("PROPFIND %s: %v", home, err)
	}
	self := strings.TrimSuffix(hrefPathOf(home), "/")
	out := map[string]bool{}
	for _, r := range ms.Responses {
		p := hrefPathOf(r.Href)
		if p == "" || strings.TrimSuffix(p, "/") == self {
			continue
		}
		out[strings.TrimSuffix(p, "/")+"/"] = true
	}
	return out
}

// hrefPathOf reduces an href to its path: servers answer with absolute URLs or
// paths, percent-encoded or not.
func hrefPathOf(href string) string {
	if u, err := url.Parse(href); err == nil {
		return u.Path
	}
	return href
}
