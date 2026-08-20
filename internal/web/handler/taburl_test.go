// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"net/url"
	"strings"
	"testing"
)

// Every link a page renders has to stay on this host. A URL that begins with
// two slashes does not: the browser reads it as scheme-relative and goes to a
// host named by the first segment. The kind tabs of §1.8 were built by
// concatenating a base of "/" with "/app/duplicates", so on a default install
// they pointed at http://app/duplicates and the filter tabs left the site.
func TestFilterTabsStayOnThisHost(t *testing.T) {
	a := newApp(t, nil)
	a.setupAdmin("root", "root@example.org", testPassword)

	for _, path := range []string{"/app/duplicates", "/app/search?q=a"} {
		rec := a.get(path)
		if rec.Code != 200 {
			t.Errorf("GET %s = %d, want 200", path, rec.Code)
			continue
		}
		hrefs := hrefsOf(rec.Body.String())
		if len(hrefs) == 0 {
			t.Errorf("GET %s rendered no links at all", path)
		}
		for _, href := range hrefs {
			if strings.HasPrefix(href, "//") {
				t.Errorf("GET %s links to %q, which the browser reads as another host", path, href)
				continue
			}
			u, err := url.Parse(href)
			if err != nil {
				t.Errorf("GET %s: unparseable href %q: %v", path, href, err)
				continue
			}
			if u.Host != "" {
				t.Errorf("GET %s links off-site to %q", path, href)
			}
		}
	}
}

// hrefsOf pulls every href out of rendered HTML without a parser: the point is
// the raw attribute value, before anything normalises it.
func hrefsOf(body string) []string {
	var out []string
	rest := body
	for {
		i := strings.Index(rest, `href="`)
		if i < 0 {
			return out
		}
		rest = rest[i+len(`href="`):]
		j := strings.IndexByte(rest, '"')
		if j < 0 {
			return out
		}
		href := rest[:j]
		rest = rest[j:]
		if href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "mailto:") {
			continue
		}
		if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
			// Links the pages deliberately point outward, such as the licence.
			continue
		}
		out = append(out, href)
	}
}
