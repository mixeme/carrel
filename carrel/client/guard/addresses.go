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

// Addresses reads C-5 of the common spec: every address the server answers
// with a page or a sheet, as C-5 writes it — the addresses a screen draws.
func Addresses(commonSpec []byte) ([]string, error) {
	text := strings.ReplaceAll(string(commonSpec), "\r\n", "\n")
	i := strings.Index(text, "\n## C-5. ")
	j := strings.Index(text, "\n## C-6. ")
	if i < 0 || j < i {
		return nil, fmt.Errorf("no section C-5")
	}
	var out []string
	for _, l := range strings.Split(text[i:j], "\n") {
		if !strings.HasPrefix(l, "| `/") {
			continue
		}
		cells, _ := tableRow(l)
		if len(cells) != 5 {
			return nil, fmt.Errorf("C-5 row of %d cells: %s", len(cells), l)
		}
		if strings.Contains(cells[3], "страница") || strings.Contains(cells[3], "лист") {
			out = append(out, strings.Trim(cells[0], "`"))
		}
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
func addressOf(route string, c5 []string) (string, bool) {
	if slices.Contains(c5, route) {
		return route, true
	}
	k := kindOf.ReplaceAllString(route, "/app/{kind}/")
	if slices.Contains(c5, k) {
		return k, true
	}
	return "", false
}

// routeCell is the address a row of the list «ждёт библиотеку» names in its
// cell «Экран».
var routeCell = regexp.MustCompile("`(/[^`]*)`")

// CheckAddresses holds the screens built against C-5 of the common spec and
// the list «ждёт библиотеку» of U-5.5 in the client spec (U-15.2, 5): each
// route built is a page or sheet address of C-5 (or one kind of it) and is
// built once; each row of the list names such an address in its cell
// «Экран»; no address is both built and waiting. What it returns besides the
// findings is the addresses of C-5 that are neither built nor waiting — the
// work left, which is a finding only at acceptance (U-15.4).
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
	done := map[string]bool{}
	seen := map[string]bool{}
	for _, r := range built {
		a, ok := addressOf(r, c5)
		switch {
		case !ok:
			findings = append(findings, Finding{Path: commonName, Rule: RuleAddress, What: fmt.Sprintf("screen %s: no page or sheet of C-5 at this address", r)})
		case seen[r]:
			findings = append(findings, Finding{Path: commonName, Rule: RuleAddress, What: fmt.Sprintf("screen %s: built twice", r)})
		default:
			done[a] = true
		}
		seen[r] = true
	}
	rows, found := readRegistry(client, clientName, Registries[0])
	findings = append(findings, found...)
	waiting := map[string]bool{}
	for _, row := range rows {
		m := routeCell.FindStringSubmatch(row.cells[1])
		if m == nil {
			findings = append(findings, Finding{Path: clientName, Line: row.line, Rule: RuleAddress, What: fmt.Sprintf("U-5.5 #%d: the cell «Экран» names no address", row.num)})
			continue
		}
		a, ok := addressOf(m[1], c5)
		if !ok {
			findings = append(findings, Finding{Path: clientName, Line: row.line, Rule: RuleAddress, What: fmt.Sprintf("U-5.5 #%d: %s is no page or sheet of C-5", row.num, m[1])})
			continue
		}
		if seen[m[1]] {
			findings = append(findings, Finding{Path: clientName, Line: row.line, Rule: RuleAddress, What: fmt.Sprintf("U-5.5 #%d: %s is built and waiting at once", row.num, m[1])})
		}
		waiting[a] = true
	}
	for _, a := range c5 {
		if !done[a] && !waiting[a] {
			left = append(left, a)
		}
	}
	return findings, left, nil
}
