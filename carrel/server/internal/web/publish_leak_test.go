package web_test

import (
	"net/url"
	"strings"
	"testing"
)

// C-11: a public page gives nothing but the thing — "ни имени владельца
// сверх необходимого". The rows of a published calendar are built by the
// agenda of the app, whose rows carry the stamp of their origin, "host ·
// collection", and the sources of a linked group; here they must not
// (audit №3 checked this and it held — the guard stays).
func TestPublishedCalendarNamesNoServer(t *testing.T) {
	f := publishFixture(t)
	link := publishLink(t, f, f.publishHref("calendar", searchWork, ""), false)
	rec := f.h.do(anon(link + "?from=2026-08-10"))
	host := ""
	if u, err := url.Parse(f.srv.URL); err == nil {
		host = u.Host
	}
	if host == "" || strings.Contains(rec.Body.String(), host) {
		t.Errorf("the public page names the owner's server %q:\n%s", host, rec.Body.String())
	}
	page := publicPage(t, f, link+"?from=2026-08-10")
	rows := 0
	for _, d := range page.Days {
		for _, r := range d.Rows {
			rows++
			if r.Stamp != "" || len(r.Sources) != 0 || r.Href != "" || len(r.Menu) != 0 {
				t.Errorf("row leads back: %+v", r)
			}
		}
	}
	if rows == 0 {
		t.Fatal("the week of the event shows no rows")
	}
}
