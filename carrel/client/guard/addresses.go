package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// RuleAddress is the rule of CheckAddresses.
const RuleAddress = "address"

// Address is a page or sheet address of C-5 and the view models it can
// answer with: the cell «Вью-модель» of its row — the route's own model and,
// where C-5 says so, another answer on the same route (a dead secret's
// DeadLinkPage, C-5.2; the card RewriteSheet's POST answers with).
type Address struct {
	Route  string   // as C-5 writes it: "/login", "/app/{kind}/{account}/{col}"
	Models []string // the view models, in the order of the cell
	Line   int      // the row's line in the common spec
}

// modelName is a view model the cell «Вью-модель» names: a page or a sheet
// of C-6 in backquotes. What else a cell quotes — a parameter (`?plan=`), a
// field (`Issued`), a value (`role=admin`) — is not a model.
var modelName = regexp.MustCompile("`([A-Z][A-Za-z0-9]*(?:Page|Sheet))`")

// sameAsBelow is how C-5 says a row answers with the models of the row under
// it (`/app/{kind}`).
const sameAsBelow = "те же, что строкой ниже"

// Addresses reads C-5 of the common spec: every address the server answers
// with a page or a sheet, as C-5 writes it — the addresses a screen draws —,
// with the view models it can answer with. A row that names no model has
// none, and CheckAddresses names it: an address answered by nothing cannot
// be built.
func Addresses(commonSpec []byte) ([]Address, error) {
	text := strings.ReplaceAll(string(commonSpec), "\r\n", "\n")
	i := strings.Index(text, "\n## C-5. ")
	j := strings.Index(text, "\n## C-6. ")
	if i < 0 || j < i {
		return nil, fmt.Errorf("no section C-5")
	}
	first := strings.Count(text[:i], "\n") + 2 // the line of «## C-5.»
	var rows []Address
	var cells [][]string
	for n, l := range strings.Split(text[i+1:j], "\n") {
		if !strings.HasPrefix(l, "| `/") {
			continue
		}
		c, _ := tableRow(l)
		if len(c) != 5 {
			return nil, fmt.Errorf("C-5 row of %d cells: %s", len(c), l)
		}
		rows = append(rows, Address{Route: strings.Trim(c[0], "`"), Line: first + n})
		cells = append(cells, c)
	}
	var out []Address
	for k := range rows {
		answer, model := cells[k][3], cells[k][4]
		if !strings.Contains(answer, "страница") && !strings.Contains(answer, "лист") {
			continue
		}
		a := rows[k]
		from := k
		if strings.Contains(model, sameAsBelow) {
			from = k + 1
		}
		if from < len(rows) {
			for _, m := range modelName.FindAllStringSubmatch(cells[from][4], -1) {
				if !slices.Contains(a.Models, m[1]) {
					a.Models = append(a.Models, m[1])
				}
			}
		}
		out = append(out, a)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("C-5: no page or sheet rows")
	}
	return out, nil
}

// kindOf writes an address of a kind of records as C-5 writes it:
// /app/contacts/… is /app/{kind}/… (docs/spec/check.py, as_kind).
var kindOf = regexp.MustCompile(`^/app/(contacts|calendar|tasks|notes)/`)

// addressOf is the address of C-5 a route belongs to: itself, or its
// template of kinds.
func addressOf(route string, c5 []Address) (Address, bool) {
	k := kindOf.ReplaceAllString(route, "/app/{kind}/")
	for _, r := range []string{route, k} {
		for _, a := range c5 {
			if a.Route == r {
				return a, true
			}
		}
	}
	return Address{}, false
}

// What a row of the list «ждёт библиотеку» names in its cell «Экран» (the
// owner's decision of 01.10.2026, U-5.5): an address that waits whole
// (routeCell); a state of a screen, by its fixture «directory/state»
// (fixtureCell), drawn and not held against a frame; a control of a screen,
// by the screen's view model (modelName) and the words after it, whose
// finding a mark at the control excuses.
var (
	routeCell   = regexp.MustCompile("`(/[^`]*)`")
	fixtureCell = regexp.MustCompile("`([a-z0-9-]+/[a-z0-9-]+)`")
)

// WaitRow is a row of the list «ждёт библиотеку» that waits for less than an
// address: the fixtures it names (states drawn without their frame) and the
// view models whose controls it names.
type WaitRow struct {
	Num, Line int
	Fixtures  []string // «app-contacts/empty»
	Models    []string // «ContactPage»
}

// WaitRows reads the rows of the list «ждёт библиотеку» of the client spec
// that name a fixture or a screen's control (WaitRow), by their numbers; a
// row that names an address is CheckAddresses'. What CheckAddresses and
// CheckRegistries find in the list is theirs too, and not repeated here.
func WaitRows(clientSpecPath string) (map[int]WaitRow, error) {
	client, err := os.ReadFile(clientSpecPath)
	if err != nil {
		return nil, err
	}
	rows, _ := readRegistry(client, filepath.Base(clientSpecPath), Registries[0])
	out := map[int]WaitRow{}
	for _, r := range rows {
		cell := r.cells[1]
		if routeCell.MatchString(cell) {
			continue
		}
		w := WaitRow{Num: r.num, Line: r.line}
		for _, m := range fixtureCell.FindAllStringSubmatch(cell, -1) {
			w.Fixtures = append(w.Fixtures, m[1])
		}
		for _, m := range modelName.FindAllStringSubmatch(cell, -1) {
			w.Models = append(w.Models, m[1])
		}
		if len(w.Fixtures) > 0 || len(w.Models) > 0 {
			out[r.num] = w
		}
	}
	return out, nil
}

// CheckAddresses holds the screens built against C-5 of the common spec and
// the list «ждёт библиотеку» of U-5.5 in the client spec (U-15.2, 5). A
// screen is a screen of a view model — the renderer picks it by the type of
// the body the server hands it —, so built are the view models that have a
// screen, and an address of C-5 is built when every view model it can answer
// with has one (Addresses). Each model built is one some page or sheet of
// C-5 answers with, and is built once; each row of the list names a page or
// sheet address of C-5 (or one kind of it) in its cell «Экран»; no address
// is both built and waiting. What it returns besides the findings is the
// addresses of C-5 that are neither built nor waiting — the work left, which
// is a finding only at acceptance (U-15.4).
func CheckAddresses(commonSpecPath, clientSpecPath string, built []string) (findings []Finding, left []string, err error) {
	common, err := os.ReadFile(commonSpecPath)
	if err != nil {
		return nil, nil, err
	}
	client, err := os.ReadFile(clientSpecPath)
	if err != nil {
		return nil, nil, err
	}
	c5, err := Addresses(common)
	if err != nil {
		return nil, nil, err
	}
	commonName, clientName := filepath.Base(commonSpecPath), filepath.Base(clientSpecPath)
	answered := map[string]bool{}
	for _, a := range c5 {
		if len(a.Models) == 0 {
			findings = append(findings, Finding{Path: commonName, Line: a.Line, Rule: RuleAddress, What: fmt.Sprintf("%s: a page or sheet with no view model", a.Route)})
		}
		for _, m := range a.Models {
			answered[m] = true
		}
	}
	has := map[string]bool{}
	for _, m := range built {
		switch {
		case !answered[m]:
			findings = append(findings, Finding{Path: commonName, Rule: RuleAddress, What: fmt.Sprintf("screen of %s: no page or sheet of C-5 answers with it", m)})
		case has[m]:
			findings = append(findings, Finding{Path: commonName, Rule: RuleAddress, What: fmt.Sprintf("screen of %s: built twice", m)})
		}
		has[m] = true
	}
	done := func(a Address) bool {
		if len(a.Models) == 0 {
			return false
		}
		for _, m := range a.Models {
			if !has[m] {
				return false
			}
		}
		return true
	}
	rows, found := readRegistry(client, clientName, Registries[0])
	findings = append(findings, found...)
	waiting := map[string]bool{}
	for _, row := range rows {
		m := routeCell.FindStringSubmatch(row.cells[1])
		if m == nil {
			if !fixtureCell.MatchString(row.cells[1]) && !modelName.MatchString(row.cells[1]) {
				findings = append(findings, Finding{Path: clientName, Line: row.line, Rule: RuleAddress, What: fmt.Sprintf("U-5.5 #%d: the cell «Экран» names no address, fixture or screen", row.num)})
			}
			continue
		}
		a, ok := addressOf(m[1], c5)
		if !ok {
			findings = append(findings, Finding{Path: clientName, Line: row.line, Rule: RuleAddress, What: fmt.Sprintf("U-5.5 #%d: %s is no page or sheet of C-5", row.num, m[1])})
			continue
		}
		if done(a) {
			findings = append(findings, Finding{Path: clientName, Line: row.line, Rule: RuleAddress, What: fmt.Sprintf("U-5.5 #%d: %s is built (%s) and waiting at once", row.num, m[1], strings.Join(a.Models, ", "))})
		}
		waiting[a.Route] = true
	}
	for _, a := range c5 {
		if !done(a) && !waiting[a.Route] {
			left = append(left, a.Route)
		}
	}
	return findings, left, nil
}
