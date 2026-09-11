package gate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"carrel.dev/typecase/css"
	"carrel.dev/typecase/gate"
)

func readBroken(t *testing.T, name string) string {
	root, err := gate.FindModuleRoot()
	if err != nil {
		t.Fatalf("module root: %v", err)
	}
	path := filepath.Join(root, "gate", "testdata", "broken", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

func brokenModule(t *testing.T) gate.ModulePaths {
	tmp := t.TempDir()
	widgetDir := filepath.Join(tmp, "widget")
	cssDir := filepath.Join(tmp, "css")
	if err := os.MkdirAll(widgetDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cssDir, 0755); err != nil {
		t.Fatal(err)
	}
	return gate.ModulePaths{Root: tmp, WidgetDir: widgetDir, CSSDir: cssDir, RepoRoot: tmp}
}

func TestBite_completeness(t *testing.T) {
	p := brokenModule(t)
	widgetSrc := `package widget

// Foo — tc-foo
//
// Роль. Test widget.
// Вход. Foo{}.
// Состояния. default.
// Инварианты. none.
// Поведение. none.
// Геометрия. none.
// Содержимое. none.
// Чего нет. none.
type Foo struct{}
`
	if err := os.WriteFile(filepath.Join(p.WidgetDir, "foo.go"), []byte(widgetSrc), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.CSSDir, "01-base.css"), []byte(readBroken(t, "completeness.css")), 0644); err != nil {
		t.Fatal(err)
	}
	if err := gate.Run(gate.Completeness, p, nil); err == nil {
		t.Fatal("expected completeness gate to fail")
	}
}

func TestBite_completeness_elementClass(t *testing.T) {
	p := brokenModule(t)
	widgetSrc := `package widget

// Foo — tc-foo
//
// Роль. Test widget.
// Вход. Foo{}.
// Состояния. default.
// Инварианты. none.
// Поведение. none.
// Геометрия. none.
// Содержимое. none.
// Чего нет. none.
type Foo struct{}
func (Foo) Render() string { return "<div class=\"tc-foo\"><span class=\"tc-foo__orphan\">x</span></div>" }
`
	if err := os.WriteFile(filepath.Join(p.WidgetDir, "foo.go"), []byte(widgetSrc), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.CSSDir, "01-base.css"), []byte(".tc-foo { display: block; }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cat := []gate.CatalogEntry{{Widget: "tc-foo", State: "default", HTML: `<div class="tc-foo"><span class="tc-foo__orphan">x</span></div>`}}
	if err := gate.Run(gate.Completeness, p, cat); err == nil {
		t.Fatal("expected completeness gate to fail on element class without css")
	}
}

func TestBite_domainNaming(t *testing.T) {
	p := brokenModule(t)
	for _, name := range []string{"domain.go", "enumvalue.go"} {
		if err := os.WriteFile(filepath.Join(p.WidgetDir, name), []byte(readBroken(t, name)), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := gate.Run(gate.DomainNaming, p, nil); err == nil {
		t.Fatal("expected domain gate to fail")
	}
}

func TestBite_docFormat(t *testing.T) {
	p := brokenModule(t)
	widgetSrc := `package widget

// Foo — tc-foo
//
// Роль. Test widget.
// Вход. Foo{}.
type Foo struct{}
`
	if err := os.WriteFile(filepath.Join(p.WidgetDir, "foo.go"), []byte(widgetSrc), 0644); err != nil {
		t.Fatal(err)
	}
	if err := gate.Run(gate.DocFormat, p, nil); err == nil {
		t.Fatal("expected doc-format gate to fail")
	}
}

func TestBite_docInput(t *testing.T) {
	p := brokenModule(t)
	if err := os.WriteFile(filepath.Join(p.WidgetDir, "foo.go"), []byte(readBroken(t, "docinput.go")), 0644); err != nil {
		t.Fatal(err)
	}
	if err := gate.Run(gate.DocFormat, p, nil); err == nil {
		t.Fatal("expected doc-format gate to fail on вход mismatch")
	}
}

func TestBite_noInline(t *testing.T) {
	html := readBroken(t, "inline.html")
	if !strings.Contains(html, "onclick=") {
		t.Fatal("broken fixture must contain onclick")
	}
	cat := []gate.CatalogEntry{{Widget: "tc-btn", State: "bad", HTML: html}}
	if err := gate.CheckNoInline(cat); err == nil {
		t.Fatal("expected no-inline gate to fail")
	}
}

func TestBite_a11y(t *testing.T) {
	html := readBroken(t, "a11y.html")
	cat := []gate.CatalogEntry{{Widget: "tc-btn", State: "bad", HTML: html}}
	if err := gate.CheckA11y(cat); err == nil {
		t.Fatal("expected a11y gate to fail")
	}
}

func TestBite_catalog(t *testing.T) {
	root, err := gate.FindModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	p := gate.Paths(root)
	cat := []gate.CatalogEntry{{Widget: "tc-btn", State: "only", HTML: `<button class="tc-btn">x</button>`}}
	if err := gate.CheckCatalog(p.WidgetDir, cat); err == nil {
		t.Fatal("expected catalog gate to fail")
	}
}

func TestBite_ownership(t *testing.T) {
	root, err := gate.FindModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	p := gate.Paths(root)
	modAbs, err := filepath.Abs(p.Root)
	if err != nil {
		t.Fatal(err)
	}
	repoAbs, err := filepath.Abs(p.RepoRoot)
	if err != nil {
		t.Fatal(err)
	}

	inside := gate.PathsWithOwnership(p.Root, filepath.Join(p.Root, "widget"))
	if err := gate.CheckOwnership(inside); err == nil {
		t.Fatal("expected ownership gate to reject search root inside module")
	}

	if repoAbs == modAbs {
		if err := gate.CheckOwnership(p); err != nil {
			t.Fatalf("standalone module must pass when no outside violations: %v", err)
		}
		return
	}

	outsideDir := filepath.Join(repoAbs, ".gate_bite_ownership")
	if err := os.MkdirAll(outsideDir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(outsideDir) })
	stolen := filepath.Join(outsideDir, "stolen.go")
	if err := os.WriteFile(stolen, []byte("package stolen\n\nconst markup = `<div class=\"tc-stolen\"></div>`\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := gate.Run(gate.Ownership, p, nil); err == nil {
		t.Fatal("expected standard Paths ownership gate to fail on tc-* outside module")
	}
}

func TestBite_gofmt(t *testing.T) {
	tmp := t.TempDir()
	src := readBroken(t, "unformatted.go")
	if err := os.WriteFile(filepath.Join(tmp, "bad.go"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	if err := gate.CheckGofmt(tmp); err == nil {
		t.Fatal("expected gofmt gate to fail on unformatted file")
	}
}

func TestBite_vet(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte("module vetbite\n\ngo 1.27\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "bad.go"), []byte("package vetbite\n\nfunc f() { fmt.Println(undefined) }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := gate.CheckVet(tmp); err == nil {
		t.Fatal("expected vet gate to fail on undefined identifier")
	}
}

func TestBite_typedInput(t *testing.T) {
	tmp := t.TempDir()
	for _, name := range []string{"typed.go", "typedclass.go"} {
		src := readBroken(t, name)
		if err := os.WriteFile(filepath.Join(tmp, name), []byte(src), 0644); err != nil {
			t.Fatal(err)
		}
		if err := gate.CheckTypedInput(tmp); err == nil {
			t.Fatalf("expected typed-input gate to fail on %s", name)
		}
	}
}

func TestBite_enumerations(t *testing.T) {
	tmp := t.TempDir()
	src := readBroken(t, "enums.go")
	if err := os.WriteFile(filepath.Join(tmp, "bad.go"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	if err := gate.CheckEnumerations(tmp); err == nil {
		t.Fatal("expected enumerations gate to fail")
	}
}

func TestBite_nodeNames(t *testing.T) {
	html := readBroken(t, "nodes.html")
	cat := []gate.CatalogEntry{{Widget: "tc-btn", State: "bad", HTML: html}}
	if err := gate.CheckNodeNames(cat); err == nil {
		t.Fatal("expected node-names gate to fail")
	}
}

func TestBite_verifiable(t *testing.T) {
	root, err := gate.FindModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	p := gate.Paths(root)
	broken := filepath.Join(p.Root, "browser", "testdata", "broken", "ellipsis.html")
	orig, err := os.ReadFile(broken)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(broken); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.WriteFile(broken, orig, 0644)
	})
	if err := gate.Run(gate.Verifiable, p, nil); err == nil {
		t.Fatal("expected verifiable gate to fail without browser fixture")
	}
}

func TestBite_invariantCoverage(t *testing.T) {
	p := brokenModule(t)
	widgetSrc := `package widget

// Foo — tc-foo
//
// Роль. Test widget.
// Вход. Foo{}.
// Состояния. default.
// Инварианты. One; two.
// Поведение. none.
// Геометрия. none.
// Содержимое. none.
// Чего нет. none.
type Foo struct{}
`
	if err := os.WriteFile(filepath.Join(p.WidgetDir, "foo.go"), []byte(widgetSrc), 0644); err != nil {
		t.Fatal(err)
	}
	if err := gate.CheckInvariantCoverage(p.WidgetDir, nil); err == nil {
		t.Fatal("expected invariant coverage to fail without bindings")
	}
}

func TestBite_tableColumns(t *testing.T) {
	cat := []gate.CatalogEntry{{
		Widget: "tc-table", State: "bad",
		HTML: `<div class="tc-table"><table class="tc-table__grid"><thead><tr><th class="tc-table__th">A</th><th class="tc-table__th">B</th></tr></thead><tbody><tr><td class="tc-table__td">1</td></tr></tbody></table></div>`,
	}}
	if err := gate.CheckTableColumns(cat); err == nil {
		t.Fatal("expected table-columns gate to fail")
	}
}

func TestBite_uniqueIDs(t *testing.T) {
	cat := []gate.CatalogEntry{
		{Widget: "tc-btn", State: "a", HTML: `<button id="dup" class="tc-btn">A</button>`},
		{Widget: "tc-btn", State: "b", HTML: `<button id="dup" class="tc-btn">B</button>`},
	}
	if err := gate.CheckUniqueIDs(cat); err == nil {
		t.Fatal("expected unique-ids check to fail on duplicates")
	}
}

func TestBite_specL4(t *testing.T) {
	p := brokenModule(t)
	specDir := filepath.Join(p.RepoRoot, "docs", "spec")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "typecase.md"), []byte(readBroken(t, "spec-l4.md")), 0644); err != nil {
		t.Fatal(err)
	}
	widgetSrc := `package widget

// Foo — tc-foo
//
// Роль. Test widget.
// Вход. Foo{}.
// Состояния. default.
// Инварианты. none.
// Поведение. none.
// Геометрия. none.
// Содержимое. none.
// Чего нет. none.
type Foo struct{}
`
	if err := os.WriteFile(filepath.Join(p.WidgetDir, "foo.go"), []byte(widgetSrc), 0644); err != nil {
		t.Fatal(err)
	}
	if err := gate.Run(gate.SpecL4, p, nil); err == nil {
		t.Fatal("expected spec-l4 gate to fail on extra declared widget")
	}
}

func TestBite_specL4_missingSpec(t *testing.T) {
	p := brokenModule(t)
	if err := gate.Run(gate.SpecL4, p, nil); err == nil {
		t.Fatal("expected spec-l4 gate to fail when spec unavailable")
	}
}

func TestBite_geometry(t *testing.T) {
	broken := readBroken(t, "geometry.css")
	live, err := css.Bundle()
	if err != nil {
		t.Fatal(err)
	}
	if err := gate.CheckGeometryCSS(live); err != nil {
		t.Fatalf("live bundle should pass geometry: %v", err)
	}
	if err := gate.CheckGeometryCSS(live + broken); err == nil {
		t.Fatal("expected geometry gate to fail on broken css")
	}
}

func TestBite_screensCoverage(t *testing.T) {
	entries := catalogEntries()
	if err := gate.CheckScreensCoverage(entries); err != nil {
		t.Fatalf("live screens should cover catalog widgets: %v", err)
	}
	entries = append(entries, gate.CatalogEntry{Widget: "tc-not-a-widget", HTML: `<div class="tc-not-a-widget"></div>`})
	if err := gate.CheckScreensCoverage(entries); err == nil {
		t.Fatal("expected screens-coverage gate to fail on extra catalog widget")
	}
}

func TestBite_screensCompose(t *testing.T) {
	root, err := gate.FindModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	p := gate.Paths(root)
	if err := gate.CheckScreensCompose(p); err != nil {
		t.Fatalf("live notes.go should pass: %v", err)
	}
	tmp := t.TempDir()
	screensDir := filepath.Join(tmp, "screens")
	if err := os.MkdirAll(screensDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(screensDir, "notes.go"), []byte("package screens\n\nfunc Notes() string {\n\treturn `<main class=\"tc-main\"`\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	p.Root = tmp
	if err := gate.CheckScreensCompose(p); err == nil {
		t.Fatal("expected screens-compose gate to fail on manual tc-main")
	}
}

func TestBite_screensCSS(t *testing.T) {
	root, err := gate.FindModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	p := gate.Paths(root)
	if err := gate.CheckScreensCSS(p); err != nil {
		t.Fatalf("live screens.css should pass: %v", err)
	}
	tmp := t.TempDir()
	screensDir := filepath.Join(tmp, "screens")
	if err := os.MkdirAll(screensDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(screensDir, "screens.css"), []byte(readBroken(t, "screens-css.css")), 0644); err != nil {
		t.Fatal(err)
	}
	p.Root = tmp
	if err := gate.CheckScreensCSS(p); err == nil {
		t.Fatal("expected screens-css gate to fail on .tc- selector")
	}
}

func readInvBroken(t *testing.T, name string) string {
	root, err := gate.FindModuleRoot()
	if err != nil {
		t.Fatalf("module root: %v", err)
	}
	path := filepath.Join(root, "gate", "testdata", "inv", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

func TestBite_invChecks(t *testing.T) {
	root, err := gate.FindModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	widgetDir := filepath.Join(root, "widget")
	for _, name := range gate.RunnableInvGateNames() {
		t.Run(name, func(t *testing.T) {
			html := readInvBroken(t, name+".html")
			widget := gate.InvGateWidget(name)
			if widget == "" {
				t.Fatalf("no widget for check %s", name)
			}
			cat := []gate.CatalogEntry{{Widget: widget, State: "broken", HTML: html}}
			if err := gate.CheckInvGate(name, widget, cat, widgetDir); err == nil {
				t.Fatalf("%s: expected check to fail on broken fixture", name)
			}
		})
	}
}
