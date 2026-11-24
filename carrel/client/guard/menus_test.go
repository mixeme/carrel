package guard_test

import (
	"sort"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"carrel.dev/carrel/client/document"
	"carrel.dev/carrel/client/screens"
)

// Menus stand as the frames draw them (Typecase's task 283, the owner's
// «системное решение» of 24.11.2026). The frame comparison takes a closed
// menu for its button, so a menu placed otherwise passed: New's arrow menu
// came as a popping menu (is-pop) inside its drop panel and stood to the
// right of the split. A menu's place is its container — the element it
// stands in, by its tag and first class — and how it is placed, its is-pop
// and is-right. Every menu of every screen, on every fixture, is placed in
// its container as the frames place a menu there; a container the frames
// never draw a menu in is told too.
func TestMenusStandAsTheFramesDraw(t *testing.T) {
	l := load(t)
	frames := placements(parseHTML(l.mock))
	found := map[string]string{}
	for _, scr := range screens.All {
		for _, path := range l.fixtures(scr) {
			model, err := decode(scr, path)
			if err != nil {
				t.Fatal(err)
			}
			doc := document.Of(scr, model, "")
			for where, places := range placements(parseHTML(doc.Root.Render())) {
				for place := range places {
					drawn, ok := frames[where]
					switch {
					case !ok:
						found[where+" "+place] = label(scr) + " on " + key(path) + ": a menu in " + where + ", where no frame draws one"
					case !drawn[place]:
						found[where+" "+place] = label(scr) + " on " + key(path) + ": a menu in " + where + " placed «" + place + "», the frames place it «" + strings.Join(keys(drawn), "» or «") + "»"
					}
				}
			}
		}
	}
	var out []string
	for _, f := range found {
		out = append(out, f)
	}
	sort.Strings(out)
	for _, f := range out {
		t.Error(f)
	}
}

// placements are the menus under nodes: by container, the ways they are
// placed.
func placements(nodes []*html.Node) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for _, m := range findAll(nodes, func(n *html.Node) bool { return hasClass(n, "tc-menu") }) {
		p := m.Parent
		if p == nil || p.Type != html.ElementNode {
			continue
		}
		where := p.Data
		if cs := classes(p); len(cs) > 0 {
			where += "." + cs[0]
		}
		var place []string
		for _, c := range []string{"is-pop", "is-right"} {
			if hasClass(m, c) {
				place = append(place, c)
			}
		}
		if out[where] == nil {
			out[where] = map[string]bool{}
		}
		out[where][strings.Join(place, " ")] = true
	}
	return out
}

func keys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
