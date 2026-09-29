// Package guard holds the guards of U-15.2 that read the client's sources:
// what the packages of screens import, what the string constants of the
// module hold, which files lie in it, and whether the marks in code and the
// rows of the registries of U-5.5 and U-5.7 answer each other. Beside them:
// the kinship of a screen with its draft (kin.go) and the addresses of C-5
// built or waiting (addresses.go); the frame comparison, which needs
// Typecase's package shape, is a test (screens_test.go).
//
// They are tripwires, not the wall. The wall is the compiler (U-15.2, 1):
// the slots of Typecase's widgets are closed, and markup of the client's own
// has nowhere to stand in the tree. These guards catch the attempt earlier
// and by name, before a screen is ever drawn.
//
// Each guard is a function over a directory, so that its test runs it on the
// real module, where it must be green, and on a deliberately broken sample in
// testdata, where it must be red (guard_test.go).
//
// The one thing that excuses a finding of Imports or Strings is a mark of a
// row of the registry of screen specifics — «специфика экрана: U-5.7 #N» —
// on the same line or on the line right above; for Files, the mark anywhere in
// the file. Whether the row exists and carries the owner's decision is for
// Registries to say.
package guard

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Finding is one breach: where, which rule, and what was found.
type Finding struct {
	Path string // slash-separated, relative to the directory walked
	Line int    // 0 when the finding is about a whole file or a row of a spec
	Rule string
	What string
}

func (f Finding) String() string {
	if f.Line > 0 {
		return fmt.Sprintf("%s:%d: %s: %s", f.Path, f.Line, f.Rule, f.What)
	}
	return fmt.Sprintf("%s: %s: %s", f.Path, f.Rule, f.What)
}

// Rules.
const (
	RuleImport   = "import"
	RuleString   = "string"
	RuleFile     = "file"
	RuleRegistry = "registry"
)

// AllowedImports are the only packages a package under screens/ imports
// (U-15.2, 2), with why:
//
//   - carrel.dev/carrel/server/view — what a screen shows;
//   - carrel.dev/typecase/widget — how it is shown (U-5.1); Typecase's
//     other packages — shape, cmd/frame2go — are for the checks, not for a
//     screen;
//   - strconv — a number of the view model into a widget's text;
//   - slices — ordering and choosing among the view model's rows.
//
// Not allowed, and why: fmt, strings, bytes, io, os — they build or write text
// and bytes, and a screen hands values to widgets, it writes nothing;
// html, html/template, text/template — templating is the markup this rule is
// against; embed — a file brought into the binary by a screen is an asset
// of the screen's own (U-5.7); sort, maps and the rest — not needed by a
// screen that only calls widgets; add one when a real screen needs it, in
// its own commit, with its reason here. Another package of the client,
// screens/… included, is not allowed either: what two screens share is
// common, and common is Typecase's (U-5.7). The one exception is the
// registry's import of a screen (ScreensPath).
var AllowedImports = []string{
	"carrel.dev/carrel/server/view",
	"carrel.dev/typecase/widget",
	"slices",
	"strconv",
}

// ScreensPath is the import path of the screens: the registry (the files of
// package screens itself) imports the package of each screen below it, to
// list it in All — and only the registry: a screen that imports another
// screen shares with it, and what screens share is common (U-5.7).
const ScreensPath = "carrel.dev/carrel/client/screens/"

// SpecificMark is how code marks a row of the registry of screen specifics,
// and WaitMark a row of the list «ждёт библиотеку».
var (
	markRe = regexp.MustCompile(`(специфика экрана: U-5\.7|ждёт библиотеку: U-5\.5) #(\d+)`)
)

const (
	specificMark = "специфика экрана: U-5.7"
	waitMark     = "ждёт библиотеку: U-5.5"
)

// excused reports whether line n (1-based) of lines, or the line above it,
// carries a mark of the registry of screen specifics.
func excused(lines []string, n int) bool {
	for _, i := range []int{n - 1, n - 2} {
		if i >= 0 && i < len(lines) {
			for _, m := range markRe.FindAllStringSubmatch(lines[i], -1) {
				if m[1] == specificMark {
					return true
				}
			}
		}
	}
	return false
}

// walkGo calls fn for every Go file under root that the go command builds:
// no _test.go file, nothing in a testdata directory or in a directory named
// in skip (slash paths relative to root).
func walkGo(root string, skip []string, fn func(rel string, src []byte) error) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := relSlash(root, path)
		if d.IsDir() {
			if rel != "." && (d.Name() == "testdata" || slices.Contains(skip, rel)) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return fn(rel, src)
	})
}

func relSlash(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

func lines(src []byte) []string {
	return strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")
}

// Imports checks every Go file under screens (the screens/ directory of the
// client) against AllowedImports; a file of the registry — right in screens/
// — may import a screen's package too, one level below (ScreensPath).
func Imports(screens string) ([]Finding, error) {
	var out []Finding
	err := walkGo(screens, nil, func(rel string, src []byte) error {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, rel, src, parser.ImportsOnly|parser.ParseComments)
		if err != nil {
			return err
		}
		ls := lines(src)
		for _, spec := range f.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			line := fset.Position(spec.Pos()).Line
			registry := !strings.Contains(rel, "/") && strings.HasPrefix(path, ScreensPath) && !strings.Contains(strings.TrimPrefix(path, ScreensPath), "/")
			if slices.Contains(AllowedImports, path) || registry || excused(ls, line) {
				continue
			}
			out = append(out, Finding{Path: rel, Line: line, Rule: RuleImport, What: fmt.Sprintf("%q is not a package a screen imports", path)})
		}
		return nil
	})
	return out, err
}

// What a string or rune constant must not hold (U-15.2, 2). Markup is «<»
// before a letter, «/», «!», «?» or a verb «%», or a constant ending in «<»
// — a tag split over a concatenation.
var forbidden = []struct {
	re   *regexp.Regexp
	what string
}{
	{regexp.MustCompile(`<[A-Za-z/!?%]|<$`), "markup"},
	{regexp.MustCompile(`\btc-`), "a name of the library's space «tc-»"},
	{regexp.MustCompile(`(?i)\bclass\s*=`), "«class=»"},
	{regexp.MustCompile(`(?i)\bstyle\s*=`), "«style=»"},
	{regexp.MustCompile(`(?i)data-tc`), "«data-tc»"},
}

// Strings checks every string and rune constant of every Go file under root,
// except those in a directory named in skip. Import paths are not constants
// of the program and are left to Imports.
func Strings(root string, skip ...string) ([]Finding, error) {
	var out []Finding
	err := walkGo(root, skip, func(rel string, src []byte) error {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, rel, src, parser.ParseComments)
		if err != nil {
			return err
		}
		paths := map[*ast.BasicLit]bool{}
		for _, spec := range f.Imports {
			paths[spec.Path] = true
		}
		ls := lines(src)
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || paths[lit] || (lit.Kind != token.STRING && lit.Kind != token.CHAR) {
				return true
			}
			value, err := strconv.Unquote(lit.Value)
			if err != nil {
				value = lit.Value
			}
			line := fset.Position(lit.Pos()).Line
			if excused(ls, line) {
				return true
			}
			for _, bad := range forbidden {
				if bad.re.MatchString(value) {
					out = append(out, Finding{Path: rel, Line: line, Rule: RuleString, What: fmt.Sprintf("%s in %s", bad.what, lit.Value)})
				}
			}
			return true
		})
		return nil
	})
	return out, err
}

// assetExt are the stylesheets, pages, templates, scripts and pictures that
// have no place in the client module: they are Typecase's (U-5.7).
var assetExt = []string{
	".css", ".scss", ".sass", ".less",
	".html", ".htm", ".xhtml", ".tmpl", ".tpl", ".gohtml",
	".js", ".mjs", ".cjs", ".ts", ".jsx", ".tsx",
	".svg",
}

// Files names every stylesheet, page, template, script or picture file under
// root, except those in a directory named in skip, that does not carry a
// mark of the registry of screen specifics.
func Files(root string, skip ...string) ([]Finding, error) {
	var out []Finding
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := relSlash(root, path)
		if d.IsDir() {
			if rel != "." && slices.Contains(skip, rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if !slices.Contains(assetExt, strings.ToLower(filepath.Ext(path))) {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range markRe.FindAllSubmatch(src, -1) {
			if string(m[1]) == specificMark {
				return nil
			}
		}
		out = append(out, Finding{Path: rel, Rule: RuleFile, What: "a stylesheet, page, script or picture of the client's own"})
		return nil
	})
	return out, err
}

// Registry is one of the two registries of carrel-client.md: the section it
// lives in, the mark that points at its rows, and the columns of its table.
type Registry struct {
	Section string // heading prefix: "### U-5.5 "
	Mark    string // «ждёт библиотеку: U-5.5»
	Columns []string
}

// Registries are the list «ждёт библиотеку» of U-5.5 and the registry of
// screen specifics of U-5.7.
var Registries = []Registry{
	{Section: "### U-5.5 ", Mark: waitMark, Columns: []string{"#", "Экран", "Кадр", "Чего нет", "Задача Typecase", "Решение"}},
	{Section: "### U-5.7 ", Mark: specificMark, Columns: []string{"#", "Экран", "Что", "Почему не в Typecase", "Решение"}},
}

// row is a row of a registry: its number, its line in the spec and its cells.
type row struct {
	num, line int
	cells     []string
}

// readRegistry finds the table of reg in its section of spec — the table
// whose header is reg.Columns — and returns its rows. A section or a table
// that is not there, or a header of another shape, is a finding: a registry
// that cannot be read cannot be held against the code.
func readRegistry(spec []byte, specName string, reg Registry) ([]row, []Finding) {
	ls := lines(spec)
	start := -1
	for i, l := range ls {
		if strings.HasPrefix(l, reg.Section) {
			start = i
			break
		}
	}
	fail := func(what string) ([]row, []Finding) {
		return nil, []Finding{{Path: specName, Rule: RuleRegistry, What: strings.TrimSpace(reg.Section) + ": " + what}}
	}
	if start < 0 {
		return fail("no such section")
	}
	end := len(ls)
	for i := start + 1; i < len(ls); i++ {
		if strings.HasPrefix(ls[i], "## ") || strings.HasPrefix(ls[i], "### ") {
			end = i
			break
		}
	}
	for i := start + 1; i < end; i++ {
		cells, ok := tableRow(ls[i])
		if !ok || len(cells) == 0 || cells[0] != "#" {
			continue
		}
		if !slices.Equal(cells, reg.Columns) {
			return fail(fmt.Sprintf("the table's columns are %q, want %q", cells, reg.Columns))
		}
		var rows []row
		var out []Finding
		seen := map[int]bool{}
		for j := i + 2; j < end; j++ { // i+1 is the rule under the header
			cells, ok := tableRow(ls[j])
			if !ok {
				break
			}
			at := fmt.Sprintf("%s: row %q", strings.TrimSpace(reg.Section), ls[j])
			if len(cells) != len(reg.Columns) {
				out = append(out, Finding{Path: specName, Line: j + 1, Rule: RuleRegistry, What: at + ": wrong number of cells"})
				continue
			}
			n, err := strconv.Atoi(strings.TrimPrefix(cells[0], "#"))
			if err != nil || n <= 0 {
				out = append(out, Finding{Path: specName, Line: j + 1, Rule: RuleRegistry, What: at + ": # is not a row number"})
				continue
			}
			if seen[n] {
				out = append(out, Finding{Path: specName, Line: j + 1, Rule: RuleRegistry, What: at + ": the number is taken"})
				continue
			}
			seen[n] = true
			decision := cells[len(cells)-1]
			if decision == "" || decision == "—" || decision == "-" {
				out = append(out, Finding{Path: specName, Line: j + 1, Rule: RuleRegistry, What: at + ": no owner's decision"})
			}
			rows = append(rows, row{num: n, line: j + 1, cells: cells})
		}
		return rows, out
	}
	return fail("no table with the columns " + strings.Join(reg.Columns, " | "))
}

// tableRow splits a Markdown table row into its trimmed cells.
func tableRow(l string) ([]string, bool) {
	l = strings.TrimSpace(l)
	if !strings.HasPrefix(l, "|") || !strings.HasSuffix(l, "|") || len(l) < 2 {
		return nil, false
	}
	cells := strings.Split(l[1:len(l)-1], "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	return cells, true
}

// mark is one mark found in a file.
type mark struct {
	path string
	line int
	num  int
}

// CheckRegistries holds the registries of the spec at specPath against the
// marks in every file under root, except those in a directory named in skip:
// a mark without a row, a row without a mark and a row without the owner's
// decision are findings (U-15.2, 5).
func CheckRegistries(specPath, root string, skip ...string) ([]Finding, error) {
	spec, err := os.ReadFile(specPath)
	if err != nil {
		return nil, err
	}
	specName := filepath.Base(specPath)
	marks := map[string][]mark{}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := relSlash(root, path)
		if d.IsDir() {
			if rel != "." && slices.Contains(skip, rel) {
				return filepath.SkipDir
			}
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sc := bufio.NewScanner(bytes.NewReader(src))
		sc.Buffer(make([]byte, 0, 64*1024), len(src)+1)
		for n := 1; sc.Scan(); n++ {
			for _, m := range markRe.FindAllStringSubmatch(sc.Text(), -1) {
				num, _ := strconv.Atoi(m[2])
				marks[m[1]] = append(marks[m[1]], mark{path: rel, line: n, num: num})
			}
		}
		return sc.Err()
	})
	if err != nil {
		return nil, err
	}
	var out []Finding
	for _, reg := range Registries {
		rows, found := readRegistry(spec, specName, reg)
		out = append(out, found...)
		name := strings.TrimSpace(reg.Section)
		marked := map[int]bool{}
		for _, m := range marks[reg.Mark] {
			marked[m.num] = true
			if !slices.ContainsFunc(rows, func(r row) bool { return r.num == m.num }) {
				out = append(out, Finding{Path: m.path, Line: m.line, Rule: RuleRegistry, What: fmt.Sprintf("mark «%s #%d» has no row in %s", reg.Mark, m.num, name)})
			}
		}
		for _, r := range rows {
			if !marked[r.num] {
				out = append(out, Finding{Path: specName, Line: r.line, Rule: RuleRegistry, What: fmt.Sprintf("%s #%d is marked nowhere in the code", name, r.num)})
			}
		}
	}
	return out, nil
}
