package proxy_test

import (
	"net/http"
	"strings"
	"testing"
)

// Audit №4, А4-2: davloom runs on the origin of Carrel's pages — on
// carrel.mixfed.ru it is /dav/ of the same host — and hands on what a
// collection's server holds with the type that server gave it. A page of
// HTML in a folder, opened through davloom, ran its script as Carrel's own:
// with the person's session cookie it could read every screen and press
// every button. Nothing davloom answers is a page: every answer carries the
// sandbox a download of the web carries, whatever the far server said.
func TestDavloomAnswersRunNothing(t *testing.T) {
	f := newFixture(t)
	f.far.SeedFile(docs+"page.html", `<!doctype html><script>fetch("/app/settings/connections")</script>`)
	answers := map[string]*http.Response{}
	answers["file"], _ = f.do("GET", f.col(docs)+"page.html", "", nil)
	answers["listing"], _ = f.do("PROPFIND", f.col(docs), `<?xml version="1.0"?><d:propfind xmlns:d="DAV:"><d:prop><d:getetag/></d:prop></d:propfind>`, map[string]string{"Depth": "1"})
	answers["root"], _ = f.do("GET", "/", "", nil)
	answers["refused"], _ = f.do("GET", f.col(docs)+"page.html", "", map[string]string{"Authorization": basic("alice", "wrong")})
	if ct := answers["file"].Header.Get("Content-Type"); answers["file"].StatusCode != 200 || !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("file: %d %q — the fixture serves no page", answers["file"].StatusCode, ct)
	}
	for what, resp := range answers {
		csp := resp.Header.Values("Content-Security-Policy")
		if len(csp) != 1 || !strings.Contains(csp[0], "sandbox") || !strings.Contains(csp[0], "default-src 'none'") {
			t.Errorf("%s: Content-Security-Policy %q, want the sandbox alone", what, csp)
		}
		if got := resp.Header.Get("X-Frame-Options"); got != "DENY" {
			t.Errorf("%s: X-Frame-Options %q", what, got)
		}
	}
}
