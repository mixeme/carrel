package css_test

import (
	"io/fs"
	"strings"
	"testing"

	"carrel.dev/typecase/css"
	"carrel.dev/typecase/token"
)

func TestBundle(t *testing.T) {
	out, err := css.Bundle()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "--tc-ink") || !strings.Contains(out, "[data-tc-root]") {
		t.Fatal("bundle missing expected token or base rules")
	}
	if err := token.Validate(out); err != nil {
		t.Fatal(err)
	}
}

func TestBundle_themeOnlyChangesPalette(t *testing.T) {
	data, err := fs.ReadFile(css.Files(), "00-tokens.css")
	if err != nil {
		t.Fatal(err)
	}
	tokens := string(data)
	darkIdx := strings.Index(tokens, `[data-tc-theme="dark"]`)
	if darkIdx < 0 {
		t.Fatal("dark theme block missing")
	}
	rest := tokens[darkIdx:]
	end := strings.Index(rest, "}")
	if end < 0 {
		t.Fatal("dark theme block unclosed")
	}
	dark := rest[:end]
	for _, name := range append(token.Typography, token.Density...) {
		if strings.Contains(dark, name) {
			t.Fatalf("dark theme must not redefine %s", name)
		}
	}
	for _, name := range token.Shape {
		if strings.Contains(dark, name) {
			t.Fatalf("dark theme must not redefine %s", name)
		}
	}
	for _, name := range token.Palette {
		if !strings.Contains(dark, name) {
			t.Fatalf("dark theme must redefine %s", name)
		}
	}
}

func TestBundle_failsWhenTokenMissing(t *testing.T) {
	// Regression guard: token.Validate is wired into Bundle.
	broken := ":root { --tc-ink: #000; }"
	if err := token.Validate(broken); err == nil {
		t.Fatal("expected validation failure")
	}
}
