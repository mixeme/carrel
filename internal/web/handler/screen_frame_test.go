// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package handler

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// interface-rebuild.md §3.1. Three acceptances of wave 2.5 failed for one
// reason: what got checked was what the checker remembered. The list of
// screens is known to the machine, and so is the list of frames, so the
// comparison is computed rather than recalled.
//
// What is compared is the skeleton: the m-* classes with their modifiers, the
// order of the nodes, the nesting, and the text of the controls. Not the tag —
// the mockups draw <span class="m-btn"> where the app answers with <button>
// and <a>, and a gate that compared tags would go red on the first frame for a
// reason that has nothing to do with anything. Not the data, not the
// attributes, and not the number of repeats: a run of identical siblings
// collapses to "one or more", because the mockup draws a book of 248 cards
// with five rows and that is not a discrepancy.
//
// The trap that killed the previous method does not work here. A mockup frame
// is narrow (~535 px), `@container (max-width:620px)` fires inside it, and the
// computed styles legitimately differ. This gate compares markup, so the width
// of the frame does not enter into it.

// screenFrame is where one template's frame lives: the route the frame is
// addressed by, and the part of it this template answers with. Every template
// needs an entry — a fragment with no frame drops the test, which is the risk
// §9 names by hand ("screens with no frame of their own quietly fall out").
type screenFrame struct {
	Route string
	Part  string
	// Omit names the parts the frame draws that this template does not
	// render: the list screens leave the details column empty and htmx fills
	// it, so the panel belongs to *_panel and not to the screen around it.
	Omit []string
	// NoFrame states why a template has no frame at all. An entry that has
	// one is not compared and not counted as done — it is a debt with a name.
	NoFrame string
}

// screenFrames is the join between the template glob and the mockups. Both
// ends are machine-known; this is the only part that cannot be derived, so it
// is written out in full and checked from both sides: a template without an
// entry fails, and an entry that resolves to no node in the mockups fails too.
var screenFrames = map[string]screenFrame{
	"base.html": {NoFrame: "the page frame, not a screen; its nodes are in every screen's frame (О2)"},

	"contacts.html":      {Route: "/app/contacts/{account}/{col}", Omit: []string{"panel"}},
	"contacts_page.html": {Route: "/app/contacts/{account}/{col}", Part: "page"},
	"contact_panel.html": {Route: "/app/contacts/{account}/{col}", Part: "panel"},
	"contact.html":       {Route: "/app/contacts/{account}/{col}/{uid}/edit"},
	"person.html":        {Route: "/app/contacts/{account}/{col}/{uid}"},
	"contact_crop.html":  {Route: "/app/contacts/{account}/{col}/{uid}/photo-preview"},

	"agenda.html":      {Route: "/app/calendar/{account}/{col}", Omit: []string{"panel"}},
	"event_panel.html": {Route: "/app/calendar/{account}/{col}", Part: "panel"},
	"event.html":       {Route: "/app/calendar/{account}/{col}/{uid}"},

	"tasks.html":      {Route: "/app/tasks/{account}/{col}", Omit: []string{"panel"}},
	"task_panel.html": {Route: "/app/tasks/{account}/{col}", Part: "panel"},
	"task.html":       {Route: "/app/tasks/{account}/{col}/{uid}"},

	"notes.html":      {Route: "/app/notes/{account}/{col}", Omit: []string{"panel"}},
	"note_panel.html": {Route: "/app/notes/{account}/{col}", Part: "panel"},
	"note.html":       {Route: "/app/notes/{account}/{col}/{uid}"},
	"note_quick.html": {Route: "/app/notes/quick"},

	"files.html":                  {Route: "/app/files/{account}/{col}"},
	"folder_picker.html":          {Route: "/app/files/picker", Omit: []string{"children"}},
	"folder_picker_children.html": {Route: "/app/files/picker", Part: "children"},
	"files_published.html":        {Route: "/app/files/published"},

	"search.html":       {Route: "/app/search", Omit: []string{"results"}},
	"find_results.html": {Route: "/app/search", Part: "results"},

	"duplicates.html":        {Route: "/app/duplicates", Omit: []string{"results"}},
	"duplicate_results.html": {Route: "/app/duplicates", Part: "results"},
	"duplicate_merge.html":   {Route: "/app/duplicates/merge"},

	"conflict.html": {Route: "/app/contacts/{account}/{col}/{uid}/conflict"},

	"contacts_import.html":        {Route: "/app/contacts/{account}/{col}/import", Omit: []string{"report"}},
	"contacts_import_report.html": {Route: "/app/contacts/{account}/{col}/import", Part: "report"},
	"calendar_import.html":        {Route: "/app/calendar/{account}/{col}/import", Omit: []string{"report"}},
	"calendar_import_report.html": {Route: "/app/calendar/{account}/{col}/import", Part: "report"},
	"notes_import.html":           {Route: "/app/notes/{account}/{col}/import"},
	"notes_import_report.html":    {Route: "/app/notes/{account}/{col}/import", Part: "report"},

	"settings_connections.html": {Route: "/app/settings/connections"},
	"settings_account.html":     {Route: "/app/settings/account"},
	"settings_appearance.html":  {Route: "/app/settings/appearance"},
	"settings_attachments.html": {NoFrame: "the mockups draw the rail entry (7.10) but no frame of the panel; " +
		"a decision for §6 before С4 touches it, not something a gate may invent"},
	"collection_form.html": {Route: "/app/collections/new"},

	"admin.html":          {Route: "/admin/"},
	"admin_invites.html":  {Route: "/admin/invites"},
	"admin_settings.html": {Route: "/admin/settings"},
	"admin_dav.html":      {Route: "/admin/dav"},
	"admin_audit.html":    {Route: "/admin/audit"},
	"admin_escrow.html":   {Route: "/admin/escrow"},
	"admin_install.html":  {Route: "/admin/install"},

	"login.html":           {Route: "/login"},
	"setup.html":           {Route: "/setup"},
	"register.html":        {Route: "/register", Omit: []string{"sent"}},
	"register_sent.html":   {Route: "/register", Part: "sent"},
	"invite.html":          {Route: "/invite/{token}", Omit: []string{"sent"}},
	"invite_invalid.html":  {Route: "/invite/{token}", Part: "invalid"},
	"email_confirmed.html": {Route: "/confirm-email/{token}", Part: "confirmed"},
	"about.html":           {Route: "/about", Part: "about"},
	"forgot.html":          {Route: "/forgot"},
	"password.html":        {Route: "/app/password"},
}

// screensNotYetRebuilt is the ratchet. A screen leaves this list in the commit
// that rebuilds it from the library, and the constant below goes down by one
// in the same commit — equality, not "at most", so the number cannot be left
// behind by accident. Everything still on the list is measured and reported,
// but not failed: this is where the rebuild starts, and on the day it started
// every screen was here.
const screensStillOnTheOldMarkup = 53

var screensNotYetRebuilt = map[string]bool{
	"about.html": true, "admin.html": true, "admin_audit.html": true,
	"admin_dav.html": true, "admin_escrow.html": true, "admin_install.html": true,
	"admin_invites.html": true, "admin_settings.html": true, "agenda.html": true,
	"calendar_import.html": true, "calendar_import_report.html": true,
	"collection_form.html": true, "conflict.html": true, "contact.html": true,
	"contact_crop.html": true, "contact_panel.html": true, "contacts.html": true,
	"contacts_import.html": true, "contacts_import_report.html": true,
	"contacts_page.html": true, "duplicate_merge.html": true,
	"duplicate_results.html": true, "duplicates.html": true,
	"email_confirmed.html": true, "event.html": true, "event_panel.html": true,
	"files.html": true, "files_published.html": true, "find_results.html": true,
	"folder_picker.html": true, "folder_picker_children.html": true,
	"forgot.html": true, "invite.html": true, "invite_invalid.html": true,
	"login.html": true, "note.html": true, "note_panel.html": true,
	"note_quick.html": true, "notes.html": true, "notes_import.html": true,
	"notes_import_report.html": true, "password.html": true, "person.html": true,
	"register.html": true, "register_sent.html": true, "search.html": true,
	"settings_account.html": true, "settings_appearance.html": true,
	"settings_connections.html": true, "setup.html": true, "task.html": true,
	"task_panel.html": true, "tasks.html": true,
}

// ---------------------------------------------------------------- skeletons

// skel is one node of a screen's structure. Tags are absent on purpose.
type skel struct {
	Class    string
	Text     string
	Children []*skel
}

var (
	isModifier   = regexp.MustCompile(`^is-[a-z0-9-]+$`)
	spaceRun     = regexp.MustCompile(`\s+`)
	repeatedSlot = regexp.MustCompile(`\{[a-z]+\}`)
)

// structuralClasses keeps the m-* names and their is-* modifiers and throws
// the rest away: a screen's own .contacts-layout is not part of the shape the
// frame prescribes, and neither is .no-print.
func structuralClasses(n *html.Node) []string {
	var out []string
	for _, a := range n.Attr {
		if a.Key != "class" {
			continue
		}
		for _, f := range strings.Fields(a.Val) {
			if strings.HasPrefix(f, "m-") || isModifier.MatchString(f) {
				out = append(out, f)
			}
		}
	}
	sort.Strings(out)
	return out
}

func attr(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

func hasClass(n *html.Node, class string) bool {
	v, _ := attr(n, "class")
	for _, f := range strings.Fields(v) {
		if f == class {
			return true
		}
	}
	return false
}

// controlText is the text of the controls §3.1 names, and only those: a
// button, an option of a segment, a labelled checkbox, the placeholder of a
// field, a rail link. Counters and tails nested inside them are left out —
// .m-num and .m-meta carry data, and this gate does not compare data. Getting
// this right is what catches both "there is no Export" and "Import where the
// frame says Export", which §2.6.E honestly handed to a person.
func controlText(n *html.Node, classes []string) string {
	set := map[string]bool{}
	for _, c := range classes {
		set[c] = true
	}
	if set["m-in"] {
		if p, ok := attr(n, "placeholder"); ok {
			return normalizeText(p)
		}
		// The mockups draw a field as a span with its placeholder as text.
		if n.DataAtom != atom.Input && n.DataAtom != atom.Select && n.DataAtom != atom.Textarea {
			return normalizeText(gatherText(n))
		}
		return ""
	}
	if set["m-btn"] || set["m-check"] || isSegOption(n) || isRailLink(n) {
		return normalizeText(gatherText(n))
	}
	return ""
}

func isSegOption(n *html.Node) bool {
	return n.Parent != nil && n.Parent.Type == html.ElementNode && hasClass(n.Parent, "m-seg")
}

func isRailLink(n *html.Node) bool {
	return n.DataAtom == atom.A && n.Parent != nil && n.Parent.Type == html.ElementNode &&
		hasClass(n.Parent, "m-nav")
}

func gatherText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(cur *html.Node) {
		if cur.Type == html.TextNode {
			b.WriteString(cur.Data)
			return
		}
		if cur.Type != html.ElementNode {
			return
		}
		if cur != n {
			// Data hanging off a control is data, not the control's name.
			if hasClass(cur, "m-num") || hasClass(cur, "m-meta") || hasClass(cur, "m-badge") {
				return
			}
			if cur.DataAtom == atom.Svg {
				return
			}
		}
		for c := cur.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func normalizeText(s string) string {
	s = strings.ReplaceAll(s, " ", " ")
	s = strings.ReplaceAll(s, "–", "-")
	s = strings.ReplaceAll(s, "‑", "-")
	return strings.TrimSpace(spaceRun.ReplaceAllString(s, " "))
}

// buildSkel walks an element and returns the structural nodes under it.
// A node with no m-* class of its own is transparent: its children are lifted
// into its parent. That is what makes the gate blind to the ten hand-written
// <div>s a list template still carries — those are TestScreenTemplateIsAssemblyOnly's
// business, and a shape gate that tripped over them would report the same
// defect twice and agree with neither report.
func buildSkel(n *html.Node, omit map[string]bool) []*skel {
	var out []*skel
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		if _, planned := attr(c, "data-planned"); planned {
			continue
		}
		if part, ok := attr(c, "data-part"); ok && omit[part] {
			continue
		}
		classes := structuralClasses(c)
		kids := buildSkel(c, omit)
		// A bare is-* modifier is not a node of the shape: the shell carries
		// is-admin, a print root carries no-print, and neither is a primitive.
		// Only a node the library draws — or an option inside a segment, or a
		// rail link, which the mockups draw as bare children — takes part.
		if !hasLibraryClass(classes) && !isSegOption(c) && !isRailLink(c) {
			out = append(out, kids...)
			continue
		}
		out = append(out, &skel{
			Class:    strings.Join(classes, " "),
			Text:     controlText(c, classes),
			Children: kids,
		})
	}
	return collapseRepeats(out)
}

// collapseRepeats folds a run of identical siblings into one. The mockup draws
// a book of 248 cards with five rows; the app answers with 248. Neither is
// wrong, so neither is compared.
func collapseRepeats(nodes []*skel) []*skel {
	var out []*skel
	for _, n := range nodes {
		if len(out) > 0 && sameShape(out[len(out)-1], n) {
			continue
		}
		out = append(out, n)
	}
	return out
}

// sameShape ignores text, because two rows of a list differ by their data and
// are still the same row.
func sameShape(a, b *skel) bool {
	if a.Class != b.Class || len(a.Children) != len(b.Children) {
		return false
	}
	for i := range a.Children {
		if !sameShape(a.Children[i], b.Children[i]) {
			return false
		}
	}
	return true
}

func hasLibraryClass(classes []string) bool {
	for _, c := range classes {
		if strings.HasPrefix(c, "m-") {
			return true
		}
	}
	return false
}

func (s *skel) describe() string {
	name := "option"
	if s.Class != "" {
		name = "." + strings.ReplaceAll(s.Class, " ", ".")
	}
	if s.Text == "" {
		return name
	}
	return name + " “" + s.Text + "”"
}

// diffSkel reports what the screen is missing against its frame, deepest
// context first. Only the frame's demands are reported: a screen may hold more
// than the frame draws — an error banner, a read-only hint — and that is not
// this gate's complaint.
func diffSkel(path string, want, got []*skel, out *[]string) {
	gi := 0
	for _, w := range want {
		match := -1
		for j := gi; j < len(got); j++ {
			if got[j].Class == w.Class {
				match = j
				break
			}
		}
		if match < 0 {
			*out = append(*out, fmt.Sprintf("%s > %s is missing", path, w.describe()))
			continue
		}
		if w.Text != "" && got[match].Text != w.Text {
			*out = append(*out, fmt.Sprintf("%s > %s reads %q", path, w.describe(), got[match].Text))
		}
		diffSkel(path+" > ."+strings.ReplaceAll(w.Class, " ", "."), w.Children, got[match].Children, out)
		gi = match + 1
	}
}

// ------------------------------------------------------------------- frames

// frameIndex is the mockups, parsed once: every frame by every route it
// carries, and the parts inside it by name.
type frameIndex struct {
	byRoute map[string]*html.Node
	doc     *html.Node
}

func loadFrames(t *testing.T) *frameIndex {
	t.Helper()
	path := filepath.Join("..", "..", "..", "docs", "visual", "carrel-ui-mockups.html")
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open mockups: %v", err)
	}
	defer f.Close()
	doc, err := html.Parse(f)
	if err != nil {
		t.Fatalf("parse mockups: %v", err)
	}
	idx := &frameIndex{byRoute: map[string]*html.Node{}, doc: doc}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if routes, ok := attr(n, "data-screen"); ok && !hasPart(n) {
				for _, r := range strings.Fields(routes) {
					if idx.byRoute[r] != nil {
						t.Errorf("two nodes claim %s whole, with no data-part between them; "+
							"one of them is a state of the other and should not carry data-screen", r)
					}
					idx.byRoute[r] = n
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if len(idx.byRoute) < 30 {
		t.Fatalf("only %d addressed frames found in the mockups; the parse is wrong, not the file",
			len(idx.byRoute))
	}
	return idx
}

func hasPart(n *html.Node) bool {
	_, ok := attr(n, "data-part")
	return ok
}

// frameSkeleton is the shape one template's frame prescribes. A part may sit
// on several siblings at once — the search results are a poll bar and three
// groups with no wrapper of their own — and then the part is all of them, in
// order.
func (idx *frameIndex) frameSkeleton(t *testing.T, name string, sf screenFrame) ([]*skel, bool) {
	t.Helper()
	// A node that carries both a route and a part is its own address — the
	// report of an import is a frame of its own, the three one-card pages of
	// §7.13 are three cards inside one. A part with no route of its own is
	// looked for inside the frame that claims the route whole.
	host := idx.byRoute[sf.Route]
	var partNodes []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && sf.Part != "" {
			if routes, ok := attr(n, "data-screen"); ok && fieldContains(routes, sf.Route) {
				if part, hasIt := attr(n, "data-part"); hasIt && part == sf.Part {
					partNodes = append(partNodes, n)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(idx.doc)

	if sf.Part == "" {
		if host == nil {
			t.Errorf("%s is addressed to %s, and no frame in the mockups carries "+
				"data-screen=%q; either the route is wrong or the frame was never marked",
				name, sf.Route, sf.Route)
			return nil, false
		}
		omit := map[string]bool{}
		for _, p := range sf.Omit {
			omit[p] = true
		}
		return buildSkel(host, omit), true
	}

	if host != nil {
		collectParts(host, sf.Part, &partNodes)
	}
	if len(partNodes) == 0 {
		t.Errorf("%s is addressed to %s part %q, and no node inside that frame carries "+
			"data-part=%q; mark the region the fragment answers with, or fix the address",
			name, sf.Route, sf.Part, sf.Part)
		return nil, false
	}
	var out []*skel
	for _, n := range partNodes {
		out = append(out, &skel{
			Class:    strings.Join(structuralClasses(n), " "),
			Text:     controlText(n, structuralClasses(n)),
			Children: buildSkel(n, nil),
		})
	}
	return collapseRepeats(out), true
}

func collectParts(n *html.Node, part string, out *[]*html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		if p, ok := attr(c, "data-part"); ok && p == part {
			*out = append(*out, c)
			continue
		}
		collectParts(c, part, out)
	}
}

func fieldContains(list, want string) bool {
	for _, f := range strings.Fields(list) {
		if f == want {
			return true
		}
	}
	return false
}

// ------------------------------------------------------------------- the gate

func TestScreenMatchesItsFrame(t *testing.T) {
	idx := loadFrames(t)
	names := screenTemplateNames(t)
	routes := registeredRoutes(t)

	if len(screensNotYetRebuilt) != screensStillOnTheOldMarkup {
		t.Errorf("screensNotYetRebuilt holds %d screens and screensStillOnTheOldMarkup says %d; "+
			"the two move together, one down at a time, in the commit that rebuilds a screen",
			len(screensNotYetRebuilt), screensStillOnTheOldMarkup)
	}

	a := newApp(t, nil)
	a.setupAdmin("root", "root@example.org", testPassword)
	seedMergedViewAccount(t, a)

	var done, pending int
	for _, name := range names {
		sf, ok := screenFrames[name]
		if !ok {
			t.Errorf("%s has no entry in screenFrames; every template is addressed to a frame, "+
				"or says in its entry why it has none — a fragment with no frame is how a screen "+
				"quietly falls out of the acceptance", name)
			continue
		}
		if sf.NoFrame != "" {
			if screensNotYetRebuilt[name] {
				t.Errorf("%s is both on the rebuild list and marked as having no frame; pick one", name)
			}
			t.Logf("%s: no frame — %s", name, sf.NoFrame)
			continue
		}
		if !routes[sf.Route] {
			t.Errorf("%s is addressed to %s, which the router does not register; "+
				"the address is the route pattern out of routes.go", name, sf.Route)
			continue
		}
		want, ok := idx.frameSkeleton(t, name, sf)
		if !ok {
			continue
		}
		if len(want) == 0 {
			t.Errorf("%s resolves to an empty frame skeleton; the frame has no m-* nodes "+
				"under it, which means the address points at the wrong node", name)
			continue
		}

		body, rendered := renderScreen(t, a, sf)
		if !rendered {
			t.Logf("%s: frame reads %d top-level nodes; the screen needs a record fixture "+
				"to render and is compared from О3 on", name, len(want))
			pending++
			continue
		}
		got := screenSkeleton(t, body, sf)
		var problems []string
		diffSkel(sf.Route, want, got, &problems)

		if screensNotYetRebuilt[name] {
			pending++
			t.Logf("%s: %d nodes of its frame are still missing (not rebuilt yet)", name, len(problems))
			continue
		}
		done++
		for _, p := range problems {
			t.Errorf("%s does not match its frame: %s.\n%s", name, p, adviceFor(t, p))
		}
	}
	t.Logf("screens compared against their frame: %d rebuilt, %d still on the old markup", done, pending)
}

// renderScreen answers with the screen's body, or says it could not be
// rendered. Routes with a record in the path need a fixture record, which
// arrives with the reference screens in О3; until then the frame side is still
// resolved and reported, so a wrong address is caught today.
func renderScreen(t *testing.T, a *app, sf screenFrame) (string, bool) {
	t.Helper()
	if repeatedSlot.MatchString(sf.Route) {
		return "", false
	}
	path := sf.Route
	if path == "/admin/" {
		path = "/admin/users"
	}
	rec := a.get(path)
	// A redirect is an answer, but it is not this screen: /login sends a
	// signed-in caller on to /app, and comparing that with the sign-in frame
	// would report a difference that is not one.
	if rec.Code >= 300 {
		return "", false
	}
	body := rec.Body.String()
	if body == "" {
		return "", false
	}
	return body, true
}

// screenSkeleton reads the answer the same way the frame is read. A fragment
// is compared as it arrives — htmx swaps it in whole — so it is parsed as a
// body of its own rather than looked up inside a page.
func screenSkeleton(t *testing.T, body string, sf screenFrame) []*skel {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		t.Fatalf("parse rendered %s: %v", sf.Route, err)
	}
	omit := map[string]bool{}
	for _, p := range sf.Omit {
		omit[p] = true
	}
	return buildSkel(doc, omit)
}

// registeredRoutes is every pattern routes.go hands to HandleFunc, method and
// {$} stripped. A screen answered only by POST — the conflict page is one — is
// a route all the same.
func registeredRoutes(t *testing.T) map[string]bool {
	t.Helper()
	src, err := os.ReadFile("routes.go")
	if err != nil {
		t.Fatalf("read routes.go: %v", err)
	}
	out := map[string]bool{}
	for _, m := range regexp.MustCompile(`s\.Path\("([^"]+)"\)`).FindAllStringSubmatch(string(src), -1) {
		p := strings.TrimSuffix(m[1], "{$}")
		out[p] = true
		out[strings.TrimSuffix(p, "/")] = true
	}
	for _, section := range []string{
		adminSectionUsers, adminSectionInvites, adminSectionSettings,
		adminSectionInstall, adminSectionDAV, adminSectionEscrow, adminSectionAudit,
	} {
		out["/admin/"+section] = true
	}
	delete(out, "")
	if len(out) < 40 {
		t.Fatalf("only %d routes parsed out of routes.go; the parse is wrong, not the router", len(out))
	}
	return out
}

// adviceFor turns "there is no .m-bar > .m-in" into what to do about it: the
// component that draws the class, and the call that draws the component. §12
// О4 — a weak model does literally what the failure message says, so the
// message is part of the tooling and not decoration.
func adviceFor(t *testing.T, problem string) string {
	t.Helper()
	m := regexp.MustCompile(`\.(m-[a-z0-9-]+)`).FindAllStringSubmatch(problem, -1)
	if len(m) == 0 {
		return ""
	}
	class := m[len(m)-1][1]
	owner, input := componentOf(t, class)
	switch {
	case owner == "":
		return fmt.Sprintf("  %s is drawn by no component in internal/web/component — "+
			"which makes it a gap in the library, not in this screen: add the three files "+
			"(template, styles, row in README.md) first.", class)
	case input == "" || input == "—":
		return fmt.Sprintf("  assemble it with {{template %q}}; see «Состав» in "+
			"internal/web/component/README.md.", owner)
	default:
		return fmt.Sprintf("  assemble it with {{template %q (%s …)}}; see «Состав» in "+
			"internal/web/component/README.md.", owner, input)
	}
}

var (
	readmeClassRow     = regexp.MustCompile("(?m)^\\|\\s*`\\.(m-[a-z0-9-]+)`\\s*\\|\\s*([^|]*)\\|")
	readmeComponentRow = regexp.MustCompile("(?m)^\\|\\s*`(m-[a-z0-9-]+)`[^|]*\\|\\s*`?([a-zA-Z—-]*)`?\\s*\\|")
)

// componentOf reads the library's own README rather than repeating it: the
// class table says which component owns a class, the composition table says
// which function builds its input.
func componentOf(t *testing.T, class string) (component, input string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "component", "README.md"))
	if err != nil {
		return "", ""
	}
	readme := string(b)
	for _, row := range readmeClassRow.FindAllStringSubmatch(readme, -1) {
		if row[1] != class {
			continue
		}
		owner := strings.TrimSpace(row[2])
		owner = strings.Trim(strings.SplitN(owner, ";", 2)[0], " `")
		component = owner
		break
	}
	if component == "" {
		return "", ""
	}
	for _, row := range readmeComponentRow.FindAllStringSubmatch(readme, -1) {
		if row[1] == component {
			return component, strings.TrimSpace(row[2])
		}
	}
	return component, ""
}
