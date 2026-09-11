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

func TestBite_completeness(t *testing.T) {
	if err := gate.CheckCompletenessCSS("tc-foo", readBroken(t, "completeness.css")); err == nil {
		t.Fatal("expected completeness gate to fail")
	}
}

func TestBite_domainNaming(t *testing.T) {
	if err := gate.CheckDomainInSource(readBroken(t, "domain.go")); err == nil {
		t.Fatal("expected domain gate to fail")
	}
}

func TestBite_docFormat(t *testing.T) {
	if err := gate.CheckDocSections(map[string]string{"Роль": "x"}); err == nil {
		t.Fatal("expected doc-format gate to fail")
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
	outside := gate.ModulePaths{Root: p.Root, WidgetDir: p.WidgetDir, CSSDir: p.CSSDir, RepoRoot: filepath.Dir(tmp)}
	if err := gate.CheckOwnership(outside); err == nil {
		t.Fatal("expected ownership gate to fail")
	}
}

func TestBite_typedInput(t *testing.T) {
	tmp := t.TempDir()
	src := readBroken(t, "typed.go")
	if err := os.WriteFile(filepath.Join(tmp, "bad.go"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	if err := gate.CheckTypedInput(tmp); err == nil {
		t.Fatal("expected typed-input gate to fail")
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
