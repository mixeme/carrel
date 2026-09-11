package gate_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	tmp := filepath.Join(t.TempDir(), "outside.html")
	if err := os.WriteFile(tmp, []byte(`<div class="tc-stolen"></div>`), 0644); err != nil {
		t.Fatal(err)
	}
	parent := gate.ModulePaths{Root: p.Root, WidgetDir: p.WidgetDir, CSSDir: p.CSSDir, RepoRoot: filepath.Dir(tmp)}
	if err := gate.CheckOwnership(parent); err == nil {
		t.Fatal("expected ownership gate to fail when scan root is outside module")
	}
	if err := gate.CheckOwnership(p); err != nil {
		t.Fatalf("standalone module must not scan parent dirs: %v", err)
	}
	wide := gate.PathsWithOwnership(p.Root, filepath.Dir(tmp))
	if err := gate.CheckOwnership(wide); err == nil {
		t.Fatal("expected ownership gate to fail on stolen class in widened scope")
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
