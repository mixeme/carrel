package gate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSpecL4Widgets_multiNameHeaders(t *testing.T) {
	root, err := FindModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	p := Paths(root)
	specPath := filepath.Join(p.RepoRoot, "docs", "spec", "typecase.md")
	declared, err := LoadSpecL4Widgets(specPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"tc-stat", "tc-stats", "tc-card", "tc-sec", "tc-fields", "tc-grid", "tc-tile"} {
		if !declared[name] {
			t.Fatalf("expected %s in L-4 registry", name)
		}
	}
}

func TestLoadSpecL4Widgets_requiresPath(t *testing.T) {
	if _, err := LoadSpecL4Widgets(""); err == nil {
		t.Fatal("expected error for empty spec path")
	}
}

func TestLoadSpecL4Widgets_missingFile(t *testing.T) {
	if _, err := LoadSpecL4Widgets(filepath.Join(t.TempDir(), "nope.md")); err == nil {
		t.Fatal("expected error for missing spec")
	}
}

func TestLoadSpecL4Widgets_noL4Section(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "spec.md")
	if err := os.WriteFile(path, []byte("## L-5. Only\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSpecL4Widgets(path); err == nil {
		t.Fatal("expected error when L-4 section missing")
	}
}
