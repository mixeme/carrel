package gate

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Name identifies one L-8 gate.
type Name string

const (
	Completeness Name = "completeness"
	Ownership    Name = "ownership"
	NodeNames    Name = "node-names"
	TypedInput   Name = "typed-input"
	Enumerations Name = "enumerations"
	DomainNaming Name = "domain-naming"
	DocFormat    Name = "doc-format"
	Catalog      Name = "catalog"
	Verifiable   Name = "verifiable"
	NoInline     Name = "no-inline"
	A11y         Name = "a11y"
)

var domainWords = []string{
	"dav", "vcard", "ical", "contact", "calendar", "note", "task",
	"collection", "account", "carrel", "caldav", "carddav",
}

var (
	tcRootClass = regexp.MustCompile(`\.tc-([a-z][a-z0-9-]*)\s*[{:,]`)
	tcClass     = regexp.MustCompile(`\btc-([a-z][a-z0-9-]+)\b`)
	inlineBad   = regexp.MustCompile(`(?i)(\bon[a-z]+\s*=|<script\b|<style\b|style\s*=)`)
)

// CatalogEntry is a minimal catalog row for gate checks.
type CatalogEntry struct {
	Widget string
	State  string
	HTML   string
}

// ModulePaths locates typecase module directories from module root.
type ModulePaths struct {
	Root      string
	WidgetDir string
	CSSDir    string
	RepoRoot  string
}

// FindModuleRoot returns the directory containing go.mod.
func FindModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("gate: module root not found")
		}
		dir = parent
	}
}

// Paths resolves standard directories under module root.
func Paths(root string) ModulePaths {
	repo := filepath.Dir(root)
	return ModulePaths{
		Root:      root,
		WidgetDir: filepath.Join(root, "widget"),
		CSSDir:    filepath.Join(root, "css"),
		RepoRoot:  repo,
	}
}

// Run executes one gate against the live module.
func Run(name Name, p ModulePaths, catalog []CatalogEntry) error {
	switch name {
	case Completeness:
		return CheckCompleteness(p)
	case Ownership:
		return CheckOwnership(p)
	case NodeNames:
		return CheckNodeNames(catalog)
	case TypedInput:
		return CheckTypedInput(p.WidgetDir)
	case Enumerations:
		return CheckEnumerations(p.WidgetDir)
	case DomainNaming:
		return CheckDomainNaming(p.WidgetDir, catalog)
	case DocFormat:
		return CheckDocFormat(p.WidgetDir)
	case Catalog:
		return CheckCatalog(p.WidgetDir, catalog)
	case NoInline:
		return CheckNoInline(catalog)
	case A11y:
		return CheckA11y(catalog)
	default:
		return fmt.Errorf("gate: unknown check %q", name)
	}
}

// CheckCompleteness ensures every tc-* root widget has CSS and a Go widget.
func CheckCompleteness(p ModulePaths) error {
	widgets, err := LoadWidgets(p.WidgetDir)
	if err != nil {
		return err
	}
	cssWidgets, err := cssRootWidgets(p.CSSDir)
	if err != nil {
		return err
	}
	goNames := make(map[string]struct{}, len(widgets))
	for name := range widgets {
		goNames[name] = struct{}{}
	}
	var missing []string
	for name := range goNames {
		if !cssWidgets[name] {
			missing = append(missing, "css missing for "+name)
		}
	}
	for name := range cssWidgets {
		if name == "tc-root" || name == "tc-icon" {
			continue
		}
		if _, ok := goNames[name]; !ok && !isVariantOnly(name) {
			missing = append(missing, "widget missing for "+name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("completeness: %s", strings.Join(missing, "; "))
	}
	return nil
}

func isVariantOnly(name string) bool {
	return strings.Contains(name, "--")
}

func cssRootWidgets(dir string) (map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	found := make(map[string]bool)
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".css") || e.Name() == "00-tokens.css" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		for _, m := range tcRootClass.FindAllStringSubmatch(string(data), -1) {
			root := "tc-" + m[1]
			if !strings.Contains(m[1], "__") {
				found[root] = true
			}
		}
	}
	return found, nil
}

// CheckOwnership flags tc-* classes outside the typecase module.
func CheckOwnership(p ModulePaths) error {
	var hits []string
	err := walkFiles(p.RepoRoot, func(path string, data []byte) error {
		if strings.Contains(path, string(filepath.Separator)+"typecase"+string(filepath.Separator)) {
			return nil
		}
		if strings.Contains(path, string(filepath.Separator)+"docs"+string(filepath.Separator)) {
			return nil
		}
		if strings.Contains(path, string(filepath.Separator)+"visual"+string(filepath.Separator)) {
			return nil
		}
		if !isSourceFile(path) {
			return nil
		}
		content := string(data)
		if !strings.Contains(content, "tc-") {
			return nil
		}
		for _, m := range tcClass.FindAllString(content, -1) {
			if strings.HasPrefix(m, "tc-") {
				hits = append(hits, path+": uses "+m)
				break
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(hits) > 0 {
		return fmt.Errorf("ownership: %s", strings.Join(hits, "; "))
	}
	return nil
}

// CheckNodeNames requires BEM element classes on inner nodes in catalog HTML.
func CheckNodeNames(catalog []CatalogEntry) error {
	var bad []string
	for _, e := range catalog {
		if strings.Contains(e.HTML, `class="tc-`) && strings.Contains(e.HTML, "<div") {
			if strings.Contains(e.HTML, `<div class="tc-`) || strings.Contains(e.HTML, `<span class="tc-`) {
				continue
			}
		}
		// Every opening tag inside widget markup should carry tc-*__* or be structural with parent tc-*.
		if strings.Contains(e.HTML, "<button") && !strings.Contains(e.HTML, "tc-") {
			bad = append(bad, e.Widget+"/"+e.State)
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("node-names: missing tc classes in %s", strings.Join(bad, ", "))
	}
	return nil
}

// CheckTypedInput rejects template.HTML-like fields in widget structs.
func CheckTypedInput(widgetDir string) error {
	var err error
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, widgetDir, func(fi os.FileInfo) bool {
		name := fi.Name()
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") && name != "doc.go"
	}, 0)
	if err != nil {
		return err
	}
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				if n == nil {
					return true
				}
				field, ok := n.(*ast.Field)
				if !ok || len(field.Names) == 0 {
					return true
				}
				typ := exprString(field.Type)
				for _, name := range field.Names {
					lower := strings.ToLower(name.Name)
					if typ == "template.HTML" || typ == "html.HTML" || strings.HasSuffix(lower, "html") || lower == "markup" {
						err = fmt.Errorf("typed-input: %s field %s uses markup type", f.Name.Name, name.Name)
						return false
					}
				}
				return true
			})
		}
	}
	return err
}

// CheckEnumerations flags paired bool state fields on widget structs.
func CheckEnumerations(widgetDir string) error {
	var err error
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, widgetDir, func(fi os.FileInfo) bool {
		return strings.HasSuffix(fi.Name(), ".go")
	}, 0)
	if err != nil {
		return err
	}
	pairs := [][]string{
		{"Open", "Closed"},
		{"On", "Off"},
		{"Enabled", "Disabled"},
		{"Shown", "Hidden"},
	}
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				st, ok := n.(*ast.StructType)
				if !ok {
					return true
				}
				boolFields := map[string]bool{}
				for _, field := range st.Fields.List {
					if len(field.Names) == 0 {
						continue
					}
					if exprString(field.Type) != "bool" {
						continue
					}
					for _, name := range field.Names {
						boolFields[name.Name] = true
					}
				}
				for _, pair := range pairs {
					if boolFields[pair[0]] && boolFields[pair[1]] {
						err = fmt.Errorf("enumerations: struct has both %s and %s", pair[0], pair[1])
						return false
					}
				}
				return true
			})
		}
	}
	return err
}

// CheckDomainNaming scans identifiers for forbidden domain words.
func CheckDomainNaming(widgetDir string, catalog []CatalogEntry) error {
	whitelist, err := exportedNames(widgetDir)
	if err != nil {
		return err
	}
	var hits []string
	checkID := func(id string) {
		if whitelist[id] {
			return
		}
		low := strings.ToLower(id)
		for _, w := range domainWords {
			if low == w || strings.Contains(low, w) {
				hits = append(hits, id)
				return
			}
		}
	}
	for _, e := range catalog {
		checkID(e.Widget)
		checkID(e.State)
	}
	if len(hits) > 0 {
		return fmt.Errorf("domain-naming: %s", strings.Join(hits, ", "))
	}
	return nil
}

// CheckDocFormat requires eight L-3.2 sections per widget.
func CheckDocFormat(widgetDir string) error {
	widgets, err := LoadWidgets(widgetDir)
	if err != nil {
		return err
	}
	var bad []string
	for name, spec := range widgets {
		for _, sec := range requiredSections {
			if _, ok := spec.Sections[sec]; !ok {
				bad = append(bad, name+" missing "+sec)
			}
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("doc-format: %s", strings.Join(bad, "; "))
	}
	return nil
}

// CheckCatalog requires every declared state in catalog entries.
func CheckCatalog(widgetDir string, catalog []CatalogEntry) error {
	widgets, err := LoadWidgets(widgetDir)
	if err != nil {
		return err
	}
	seen := make(map[string]map[string]bool)
	for _, e := range catalog {
		if seen[e.Widget] == nil {
			seen[e.Widget] = make(map[string]bool)
		}
		seen[e.Widget][slugState(e.State)] = true
	}
	var missing []string
	for name, spec := range widgets {
		if len(spec.States) == 1 && spec.States[0] == "default" {
			if len(seen[name]) == 0 {
				missing = append(missing, name+"/(any)")
			}
			continue
		}
		for _, st := range spec.States {
			if !seen[name][st] {
				missing = append(missing, name+"/"+st)
			}
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("catalog: missing entries: %s", strings.Join(missing, ", "))
	}
	return nil
}

// CheckNoInline rejects handlers, script/style blocks, and style attributes.
func CheckNoInline(catalog []CatalogEntry) error {
	var bad []string
	for _, e := range catalog {
		if inlineBad.MatchString(e.HTML) {
			bad = append(bad, e.Widget+"/"+e.State)
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("no-inline: %s", strings.Join(bad, ", "))
	}
	return nil
}

// CheckA11y requires accessible names on controls in catalog HTML.
func CheckA11y(catalog []CatalogEntry) error {
	var bad []string
	for _, e := range catalog {
		html := e.HTML
		if strings.Contains(html, "<button") {
			if !hasAccessibleName(html) {
				bad = append(bad, e.Widget+"/"+e.State+": button")
			}
		}
		if strings.Contains(html, `class="tc-in`) || strings.Contains(html, `class="tc-in-wrap`) {
			if !strings.Contains(html, "aria-label=") && !strings.Contains(html, "<label") {
				bad = append(bad, e.Widget+"/"+e.State+": field")
			}
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("a11y: %s", strings.Join(bad, ", "))
	}
	return nil
}

func hasAccessibleName(html string) bool {
	if strings.Contains(html, "aria-label=") {
		return true
	}
	for _, cls := range []string{
		"tc-btn__label", "tc-menu__label", "tc-usermenu__label", "tc-tree__label",
		"tc-chip__label", "tc-close", "tc-tick",
	} {
		if strings.Contains(html, cls) {
			return true
		}
	}
	return false
}

// CheckCompletenessCSS is a bite helper: css text must mention widget class.
func CheckCompletenessCSS(widgetName, css string) error {
	if !strings.Contains(css, "."+widgetName) {
		return fmt.Errorf("completeness: css missing %s", widgetName)
	}
	return nil
}

// CheckDomainInSource is a bite helper for domain word in Go source.
func CheckDomainInSource(src string) error {
	low := strings.ToLower(src)
	for _, w := range domainWords {
		if strings.Contains(low, w) {
			return fmt.Errorf("domain-naming: contains %q", w)
		}
	}
	return nil
}

// CheckDocSections is a bite helper for description sections.
func CheckDocSections(sections map[string]string) error {
	for _, sec := range requiredSections {
		if _, ok := sections[sec]; !ok {
			return fmt.Errorf("doc-format: missing %s", sec)
		}
	}
	return nil
}

func exportedNames(widgetDir string) (map[string]bool, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, widgetDir, nil, 0)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool)
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			for _, decl := range f.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok {
					continue
				}
				for _, spec := range gen.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if s.Name != nil {
							out[s.Name.Name] = true
						}
					case *ast.ValueSpec:
						for _, n := range s.Names {
							out[n.Name] = true
						}
					}
				}
			}
		}
	}
	return out, nil
}

func exprString(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return exprString(t.X) + "." + t.Sel.Name
	default:
		return ""
	}
}

func walkFiles(root string, fn func(path string, data []byte) error) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !isTextFile(path) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return fn(path, data)
	})
}

func isTextFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".html", ".css", ".js", ".md", ".tsx", ".ts", ".jsx", ".vue", ".tmpl":
		return true
	default:
		return false
	}
}

func isSourceFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".html", ".tsx", ".ts", ".jsx", ".vue", ".tmpl":
		return true
	default:
		return false
	}
}
