package guard_test

import (
	"bytes"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"carrel.dev/carrel/client/guard"
	"carrel.dev/carrel/client/screens"
	"carrel.dev/carrel/server/view"
	"carrel.dev/typecase/shape"
	w "carrel.dev/typecase/widget"
)

// A sheet's screen (U-15.2, 3 and 4; the scan of the files section, ФЖ11).
// A sheet answered as a page — without a script, by its address — is drawn
// over the screen it was opened from (the owner's decision of 15.10.2026;
// C-6 Under; Typecase's Overlay): a shell whose body is the Overlay, the
// page under the scrim and the sheet on it. A sheet that came without that
// screen (Under nil) is drawn alone: an Overlay whose scrim holds it. Its
// frames draw more: the route's frame draws the sheet over a
// screen (s-files-4 over a folder, s-notes-11 over the calendar), the frame
// of a state is a fragment — the sheet alone (s-files-11, -13, -16, -19),
// or a part of it (s-files-12, the sheet's notice). What is held:
//
//   - the frame comparison: the sheet of the route's frame against the
//     screen's sheet; each drawing of a fragment against the screen's first
//     element of its kind (its first class) — the sheet against the sheet,
//     the notice against the notice;
//   - the kinship: the Sheet of the route's draft against the screen's
//     Sheet. A fragment prints no draft (frame2go, Р-16), and a state drawn
//     by one is held by the frame comparison alone, as the calendar's
//     fragments were read (plan/client.md, «Раздел календаря»).

// overlayOpen is how an overlay is printed: a sheet over a screen, or over
// nothing.
const overlayOpen = `<div class="tc-overlay">`

// overlaid says whether a sheet's screen is an overlay: alone, or the body
// of a shell — the screen under it.
func overlaid(root screens.Root) bool {
	switch r := root.(type) {
	case w.Overlay:
		return true
	case w.Shell:
		_, ok := r.Body.(w.Overlay)
		return ok
	}
	return false
}

var (
	fragLabel = regexp.MustCompile(`<div class="frame-label">.*?class="frag"`)
	styleTC   = regexp.MustCompile(`(?s)<style[^>]*\bdata-tc\b[^>]*>.*?</style>`)
)

// fragment says whether the frame id is a fragment of the mockup — its label
// says so (frag) —, which frame2go prints no draft of (Р-16).
func fragment(mock, id string) bool {
	return fragLabel.MatchString(frameBlock(mock, id))
}

// parseHTML parses a piece of HTML as the body of a page.
func parseHTML(s string) []*html.Node {
	body := &html.Node{Type: html.ElementNode, Data: "body", DataAtom: atom.Body}
	nodes, err := html.ParseFragment(strings.NewReader(s), body)
	if err != nil {
		return nil
	}
	return nodes
}

// classes are the classes of an element.
func classes(n *html.Node) []string {
	for _, a := range n.Attr {
		if a.Key == "class" {
			return strings.Fields(a.Val)
		}
	}
	return nil
}

// hasClass says whether an element has the class c.
func hasClass(n *html.Node, c string) bool {
	for _, x := range classes(n) {
		if x == c {
			return true
		}
	}
	return false
}

// find is the first element under the nodes, depth first, that ok takes.
func find(nodes []*html.Node, ok func(*html.Node) bool) *html.Node {
	for _, n := range nodes {
		if n.Type == html.ElementNode && ok(n) {
			return n
		}
		var kids []*html.Node
		for k := n.FirstChild; k != nil; k = k.NextSibling {
			kids = append(kids, k)
		}
		if f := find(kids, ok); f != nil {
			return f
		}
	}
	return nil
}

// outer is an element printed.
func outer(n *html.Node) string {
	var b bytes.Buffer
	if err := html.Render(&b, n); err != nil {
		return ""
	}
	return b.String()
}

// sheetParts are what a sheet's screen is held against in the frame id: the
// sheet of a route's frame; the drawings of a fragment, each an element of
// its root.
func sheetParts(mock, id string) []*html.Node {
	nodes := parseHTML(frameBlock(mock, id))
	if !fragment(mock, id) {
		if s := find(nodes, func(n *html.Node) bool { return hasClass(n, "tc-sheet") }); s != nil {
			return []*html.Node{s}
		}
		return nil
	}
	root := find(nodes, func(n *html.Node) bool { return hasClass(n, "tc-root") })
	if root == nil {
		return nil
	}
	var out []*html.Node
	for k := root.FirstChild; k != nil; k = k.NextSibling {
		if k.Type == html.ElementNode {
			out = append(out, k)
		}
	}
	return out
}

// sheetFindings is the frame comparison of a sheet's screen (the comment at
// the top of the file): what the screen printed, an overlay, against the
// sheet of the frame or the drawings of its fragment, each by shape — a
// mockup of that drawing alone, with the mockup's own rules of words.
func sheetFindings(l lib, frame, at, printed string) []string {
	parts := sheetParts(l.mock, frame)
	if len(parts) == 0 {
		return []string{fmt.Sprintf("%s: %s draws no sheet and no part of one", at, frame)}
	}
	// The sheet's own parts are looked for on the scrim: the screen under
	// it has notices of its own.
	screen := parseHTML(printed)
	if scrim := find(screen, func(n *html.Node) bool { return hasClass(n, "tc-scrim") }); scrim != nil {
		screen = []*html.Node{scrim}
	}
	return partFindings(l, frame, at, parts, screen)
}

// pageParts are the drawings of a fragment of a page (account-8: «блок
// Email»): from its main column down while there is one element, what that
// holds — the group drawn alone is the first of its form, and on the page
// it is not; its rubric and its fields are what the fragment shows. A
// fragment drawn without a main column — a notice over the cards of
// Connections and its diagnostic (s-profile-31, -32) — is what its root
// holds.
func pageParts(mock, id string) []*html.Node {
	nodes := parseHTML(frameBlock(mock, id))
	n := find(nodes, func(n *html.Node) bool { return hasClass(n, "tc-main") })
	if n == nil {
		n = find(nodes, func(n *html.Node) bool { return hasClass(n, "tc-root") })
	}
	if n == nil {
		return nil
	}
	for {
		kids := elements(n)
		if len(kids) != 1 {
			return kids
		}
		n = kids[0]
	}
}

// elements are a node's element children.
func elements(n *html.Node) []*html.Node {
	var out []*html.Node
	for k := n.FirstChild; k != nil; k = k.NextSibling {
		if k.Type == html.ElementNode {
			out = append(out, k)
		}
	}
	return out
}

// partFindings holds each drawing of a fragment against the screen: some
// element of its class has its shape.
func partFindings(l lib, frame, at string, parts, screen []*html.Node) []string {
	var out []string
	for i, p := range parts {
		cs := classes(p)
		if len(cs) == 0 {
			out = append(out, fmt.Sprintf("%s: a drawing of %s without a class", at, frame))
			continue
		}
		got := findAll(screen, func(n *html.Node) bool { return hasClass(n, cs[0]) })
		if len(got) == 0 {
			out = append(out, fmt.Sprintf("%s: the screen has no %s, which %s draws", at, cs[0], frame))
			continue
		}
		id := fmt.Sprintf("%s/%d", frame, i)
		mock := styleTC.FindString(l.mock) + `<div class="frame" data-frame="` + id + `">` + outer(p) + `</div>`
		m, err := shape.Parse(mock)
		if err != nil {
			out = append(out, fmt.Sprintf("%s: %v", at, err))
			continue
		}
		// The part is any element of its class that has its shape: a field
		// of a sheet drawn alone (s-admin-16) is one of several fields; the
		// first one's difference is told when none has it.
		var first error
		for _, g := range got {
			if err := m.Check(id, outer(g)); err == nil {
				first = nil
				break
			} else if first == nil {
				first = err
			}
		}
		if first != nil {
			out = append(out, fmt.Sprintf("%s: %v", at, first))
		}
	}
	return out
}

// findAll is every element of nodes and their descendants that ok holds,
// in document order.
func findAll(nodes []*html.Node, ok func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	for _, n := range nodes {
		if n.Type == html.ElementNode && ok(n) {
			out = append(out, n)
		}
		var kids []*html.Node
		for k := n.FirstChild; k != nil; k = k.NextSibling {
			kids = append(kids, k)
		}
		out = append(out, findAll(kids, ok)...)
	}
	return out
}

// sheetOf is the skeleton of the first Sheet in a skeleton, depth first; nil
// where there is none.
func sheetOf(n *guard.Node) *guard.Node {
	if n == nil {
		return nil
	}
	if n.Type == "Sheet" {
		return n
	}
	for _, name := range slices.Sorted(maps.Keys(n.Parts)) {
		for _, k := range n.Parts[name] {
			if s := sheetOf(k); s != nil {
				return s
			}
		}
	}
	return nil
}

// The bites of the sheets' rule: the frame's own sheet, printed over
// nothing, is green against its frame; the sheet without its foot, a
// fragment's notice missing, a sheet's skeleton without its foot, and a
// sheet's screen that draws a shell are red.
func TestSheetBite(t *testing.T) {
	l := load(t)
	if fragment(l.mock, "s-files-4") || !fragment(l.mock, "s-files-11") || !fragment(l.mock, "s-files-12") {
		t.Fatal("the samples moved: s-files-4 is a route's frame, s-files-11 and -12 fragments")
	}
	parts := sheetParts(l.mock, "s-files-4")
	if len(parts) != 1 {
		t.Fatalf("s-files-4 draws %d sheets", len(parts))
	}
	sheet := outer(parts[0])
	none(t, "the frame's own sheet", sheetFindings(l, "s-files-4", "own", overlayOpen+sheet+"</div>"))
	// The sheet is a form since Typecase 7512157 (task 134): its last tag
	// is the form's.
	foot := regexp.MustCompile(`(?s)<div class="tc-sheet__foot">.*?</div>\s*</(div|form)>$`)
	if cut := foot.ReplaceAllString(sheet, "</$1>"); cut == sheet {
		t.Fatal("the sample without its foot changed nothing: the sheet of s-files-4 moved")
	} else if found := sheetFindings(l, "s-files-4", "no foot", overlayOpen+cut+"</div>"); len(found) == 0 {
		t.Error("a sheet without its foot stayed green")
	}
	msg := sheetParts(l.mock, "s-files-12")
	if len(msg) != 1 || !hasClass(msg[0], "tc-msg") {
		t.Fatalf("s-files-12 draws %d parts, want its notice", len(msg))
	}
	none(t, "a fragment's notice", sheetFindings(l, "s-files-12", "notice", overlayOpen+`<div class="tc-sheet">`+outer(msg[0])+`</div></div>`))
	if found := sheetFindings(l, "s-files-12", "no notice", overlayOpen+`<div class="tc-sheet"></div></div>`); len(found) == 0 || !strings.Contains(found[0], "has no tc-msg") {
		t.Errorf("a sheet without the fragment's notice: %q", found)
	}
	if found := sheetFindings(l, "s-files-12", "another notice", overlayOpen+sheet+"</div>"); len(found) == 0 {
		t.Error("a sheet whose notice is not the fragment's stayed green")
	}

	d := drafts(t, l, []screens.Screen{{Model: func() any { return new(view.FileOpSheet) }, Frames: []string{"s-files-4"}}})
	s := sheetOf(d["s-files-4"])
	if s == nil {
		t.Fatal("the draft of s-files-4 has no Sheet")
	}
	if err := guard.Kin(s, s); err != nil {
		t.Errorf("the draft's sheet against itself: %v", err)
	}
	spoiled := *s
	spoiled.Parts = maps.Clone(s.Parts)
	delete(spoiled.Parts, "Foot")
	if guard.Kin(s, &spoiled) == nil {
		t.Error("a sheet without its foot stayed kin of the draft's")
	}

	shell := screens.Screen{
		Model:  func() any { return new(view.FileOpSheet) },
		Frames: []string{"s-files-4", "s-files-11", "s-files-12", "s-files-13"},
		Draw:   func(any) screens.Root { return w.Shell{} },
	}
	if found := frameFindings(l, []screens.Screen{shell}); !slices.ContainsFunc(found, func(f string) bool { return strings.Contains(f, "draws its sheet over the screen") }) {
		t.Errorf("a sheet's screen that draws a shell: %q", found)
	}
}
