// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import "strings"

// hrefsOf pulls every href out of rendered HTML without a parser: the point is
// the raw attribute value, before anything normalises it. A URL beginning with
// two slashes is the case that matters — the browser reads it as
// scheme-relative and leaves for a host named by the first segment, which is
// what the kind tabs of §1.8 did on every install without a mount prefix.
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
