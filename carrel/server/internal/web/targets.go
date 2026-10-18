package web

import (
	"net/url"
	"strings"

	"carrel.dev/carrel/server/render"
)

// The zone a fragment answer of an action or a form replaces (C-7.2;
// Action.Target, Form.Target — stage 4 of the client's plan, С1). The
// client cannot ask for a fragment where the frame draws a move to another
// page (C-7.1), and the frame is not something the client reads at run
// time: the target is data. It follows C-5: an address that answers with a
// sheet is aimed at sheet, one that answers with a fragment at the zone of
// its view model (the rail at rail, the poll bar at poll, the queue at
// queue, a folder at main); every other address is a move, and its target
// is empty. A row of a list that opens in the panel is the list's (the
// row's own Target, set where the row is built), not its address's: the
// same address is a move from a menu's Open.
//
// The table is C-5's, written again; view.TestFixtureTargetsAreC5 holds
// every action and form of every fixture against C-5 read from the spec, so
// the two cannot part silently.
var fragmentRoutes = []fragmentRoute{
	// C-5.3
	{"", "/app/search/progress/{task}/poll", render.TargetPoll},
	{"", "/app/search/progress/{task}/retry", render.TargetPoll},
	{"", "/app/search/progress/{task}/cancel", render.TargetPoll},
	{"POST", "/app/sources/{kind}", render.TargetRail},
	{"", "/app/publish", render.TargetSheet},
	{"", "/app/mail/compose", render.TargetSheet},
	{"", "/app/mail/attach", render.TargetSheet},
	// C-5.4
	{"", "/app/{kind}/{account}/{col}/{uid}/move", render.TargetSheet},
	{"GET", "/app/{kind}/{account}/{col}/{uid}/copies", render.TargetSheet},
	// Delete answers with its sheet when the record has something to lose
	// (note 69 of Typecase, task 166), else moves to the list.
	{"POST", "/app/{kind}/{account}/{col}/{uid}/delete", render.TargetSheet},
	{"", "/app/tasks/{account}/{col}/{uid}/done", render.TargetSheet},
	{"GET", "/app/contacts/{account}/{col}/{uid}/rewrite", render.TargetSheet},
	{"GET", "/app/{kind}/export", render.TargetSheet},
	{"", "/app/{kind}/{account}/{col}/new/quick", render.TargetSheet},
	{"GET", "/app/{kind}/quick", render.TargetSheet},
	// C-5.5
	{"POST", "/app/files/{account}/{col}/upload/{path…}", render.TargetQueue},
	{"POST", "/app/files/{account}/{col}/mkcol/{path…}", render.TargetSheet},
	{"", "/app/files/{account}/{col}/op/{path…}", render.TargetSheet},
	{"", "/app/files/picker", render.TargetSheet},
	// C-5.6
	{"", "/app/collections/new", render.TargetSheet},
	{"", "/app/collections/rename", render.TargetSheet},
	{"", "/app/collections/move", render.TargetSheet},
	{"", "/app/collections/delete", render.TargetSheet},
	// C-5.7
	{"GET", "/app/settings/escrow/deposit", render.TargetSheet},
	{"GET", "/app/settings/escrow/withdraw", render.TargetSheet},
	{"", "/app/settings/connections/disconnect", render.TargetSheet},
	{"", "/app/settings/unlock", render.TargetSheet},
	// C-5.8
	{"", "/admin/reset", render.TargetSheet},
	{"GET", "/admin/delete", render.TargetSheet},
}

// fragmentRoute is a row of C-5 answered with a fragment or a sheet: the
// method it holds for ("" — every method of the row), the route as C-5
// writes it, and the zone of C-7.2 its answer replaces.
type fragmentRoute struct {
	method, route, target string
}

// recordKindsInRoutes are what {kind} of C-5.4 stands for.
var recordKindsInRoutes = map[string]bool{"contacts": true, "calendar": true, "tasks": true, "notes": true}

// routeMatch reports whether path (without the base path and the query)
// is route, and how many of its segments are words of the route rather
// than its parameters: the more, the more particular the route — as the
// server's own mux picks one (a folder called «op» is the operation).
func routeMatch(route, path string) (literal int, ok bool) {
	rs := strings.Split(strings.TrimPrefix(route, "/"), "/")
	ps := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for i, r := range rs {
		if strings.HasSuffix(r, "…}") { // the rest of the path, maybe nothing
			return literal, i <= len(ps)
		}
		if i >= len(ps) {
			return 0, false
		}
		switch {
		case r == "{kind}":
			// The rail's form of files is a kind too (C-5.3).
			if !recordKindsInRoutes[ps[i]] && !(ps[i] == "files" && strings.HasPrefix(route, "/app/sources/")) {
				return 0, false
			}
		case strings.HasPrefix(r, "{"):
			if ps[i] == "" {
				return 0, false
			}
		case r == ps[i]:
			literal++
		default:
			return 0, false
		}
	}
	return literal, len(rs) == len(ps)
}

// answerZone is the zone of C-7.2 a fragment answer of method on href
// replaces, by C-5; empty — the address is a move. href is as the view
// model carries it: under the base path, with its query.
func answerZone(base, method, href string) string {
	if href == "" || strings.Contains(href, "://") {
		return ""
	}
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	// Escaped: a collection's path is one segment of the address, %2F and
	// all, and must not split into more.
	path := u.EscapedPath()
	if base != "" {
		if !strings.HasPrefix(path, base+"/") {
			return ""
		}
		path = strings.TrimPrefix(path, base)
	}
	best, target := -1, ""
	for _, fr := range fragmentRoutes {
		if fr.method != "" && !strings.EqualFold(fr.method, method) {
			continue
		}
		if n, ok := routeMatch(fr.route, path); ok && n > best {
			best, target = n, fr.target
		}
	}
	return target
}
