package token

import (
	"fmt"
	"regexp"
	"strings"
)

// Palette names the eight overridable palette slots (L-5.1).
var Palette = []string{
	"--tc-ink",
	"--tc-surface",
	"--tc-bg",
	"--tc-card",
	"--tc-rule",
	"--tc-muted",
	"--tc-accent",
	"--tc-danger",
}

// Typography names font families and base size.
var Typography = []string{
	"--tc-font-serif",
	"--tc-font-sans",
	"--tc-font-mono",
	"--tc-font-size",
}

// Density names the two row-height values; there is no third (L-5.1).
var Density = []string{
	"--tc-row-height",
	"--tc-row-height-touch",
}

// Spacing names page-level horizontal padding (L-5.1).
var Spacing = []string{
	"--tc-page-gutter",
	"--tc-page-gutter-narrow",
}

// Shape names corner radii and stroke (L-5.1).
var Shape = []string{
	"--tc-radius",
	"--tc-radius-sm",
	"--tc-stroke",
}

// Required is the full set of custom properties every consumer must define.
// Widget styles must only reference names from Required or names derived in
// the library; an undefined name fails validation.
var Required = concat(Palette, Typography, Density, Spacing, Shape)

var varDecl = regexp.MustCompile(`--tc-[a-z0-9-]+`)

// Defined returns every --tc-* name assigned a value in css.
func Defined(css string) map[string]struct{} {
	seen := make(map[string]struct{})
	for _, name := range varDecl.FindAllString(css, -1) {
		seen[name] = struct{}{}
	}
	return seen
}

// Missing reports required names absent from css.
func Missing(css string) []string {
	defined := Defined(css)
	var missing []string
	for _, name := range Required {
		if _, ok := defined[name]; !ok {
			missing = append(missing, name)
		}
	}
	return missing
}

// Validate reports an error when css does not define every required token.
func Validate(css string) error {
	if missing := Missing(css); len(missing) > 0 {
		return fmt.Errorf("typecase: missing required tokens: %s", strings.Join(missing, ", "))
	}
	return nil
}

// ValidateWidget reports an error when css references a --tc-* name that is
// not in Required. Widgets must not invent silent defaults for unknown tokens.
func ValidateWidget(css string) error {
	defined := Defined(css)
	required := make(map[string]struct{}, len(Required))
	for _, name := range Required {
		required[name] = struct{}{}
	}
	var unknown []string
	for name := range defined {
		if _, ok := required[name]; !ok {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		return fmt.Errorf("typecase: widget references unknown tokens: %s", strings.Join(unknown, ", "))
	}
	return nil
}

func concat(parts ...[]string) []string {
	var out []string
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}
