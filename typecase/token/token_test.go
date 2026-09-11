package token

import (
	"strings"
	"testing"
)

func TestValidate_complete(t *testing.T) {
	css := strings.Join([]string{
		":root {",
		"--tc-ink: #000; --tc-surface: #111; --tc-bg: #222; --tc-card: #777; --tc-rule: #333;",
		"--tc-muted: #444; --tc-accent: #555; --tc-danger: #666;",
		"--tc-font-serif: a; --tc-font-sans: b; --tc-font-mono: c; --tc-font-size: 13px;",
		"--tc-row-height: 38px; --tc-row-height-touch: 52px;",
		"--tc-page-gutter: 20px; --tc-page-gutter-narrow: 14px;",
		"--tc-radius: 3px; --tc-radius-sm: 2px; --tc-stroke: 0.5px;",
		"}",
	}, " ")
	if err := Validate(css); err != nil {
		t.Fatal(err)
	}
}

func TestValidate_missing(t *testing.T) {
	err := Validate(":root { --tc-ink: #000; }")
	if err == nil {
		t.Fatal("expected error for missing tokens")
	}
	if !strings.Contains(err.Error(), "--tc-surface") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateWidget_unknown(t *testing.T) {
	err := ValidateWidget(".tc-btn { color: var(--tc-ink); border: var(--tc-stroke) solid var(--tc-secret); }")
	if err == nil {
		t.Fatal("expected error for unknown token reference")
	}
	if !strings.Contains(err.Error(), "--tc-secret") {
		t.Fatalf("unexpected error: %v", err)
	}
}
