package gate

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"carrel.dev/typecase/css"
)

var (
	tokenPx   = regexp.MustCompile(`(--tc-[a-z0-9-]+)\s*:\s*([0-9]+(?:\.[0-9]+)?)px`)
	ruleBlock = regexp.MustCompile(`(?s)(\.[^{]+)\{([^}]*)\}`)
	propPx    = regexp.MustCompile(`([a-z-]+)\s*:\s*([0-9]+(?:\.[0-9]+)?)px`)
)

// CheckGeometry verifies bundle numbers match the mockup-aligned values in
// L-4 and L-5.1 (±1 px).
func CheckGeometry(_ ModulePaths) error {
	bundle, err := css.Bundle()
	if err != nil {
		return err
	}
	return CheckGeometryCSS(bundle)
}

// CheckGeometryCSS is a bite helper for geometry numbers in css text.
func CheckGeometryCSS(cssText string) error {
	tokens := tokenValues(cssText)
	var bad []string

	expectToken := map[string]float64{
		"--tc-row-height":         38,
		"--tc-font-size":          13,
		"--tc-page-gutter":        20,
		"--tc-page-gutter-narrow": 14,
		"--tc-radius":             3,
		"--tc-radius-sm":          2,
	}
	for name, want := range expectToken {
		got, ok := tokens[name]
		if !ok {
			bad = append(bad, name+" missing")
			continue
		}
		if !closeEnough(got, want) {
			bad = append(bad, fmt.Sprintf("%s=%gpx want %gpx", name, got, want))
		}
	}

	ruleChecks := []struct {
		selector string
		prop     string
		want     float64
	}{
		{".tc-row", "gap", 12},
		{".tc-tickrow", "padding-left", 16},
		{".tc-tickrow", "padding-right", 16},
		{".tc-tickrow", "min-height", 26},
		{".tc-nav__item", "min-height", 29},
		{".tc-top", "min-height", 52},
		{".tc-btn", "min-height", 26},
		{".tc-btn--form", "min-height", 28},
		{".tc-list", "border-top-width", 0},
	}
	rules := ruleValues(cssText)
	for _, rc := range ruleChecks {
		got, ok := rules[rc.selector+"|"+rc.prop]
		if !ok {
			bad = append(bad, rc.selector+"."+rc.prop+" missing")
			continue
		}
		if !closeEnough(got, rc.want) {
			bad = append(bad, fmt.Sprintf("%s %s=%gpx want %gpx", rc.selector, rc.prop, got, rc.want))
		}
	}

	if len(bad) > 0 {
		return fmt.Errorf("geometry: %s", strings.Join(bad, "; "))
	}
	return nil
}

func tokenValues(cssText string) map[string]float64 {
	out := make(map[string]float64)
	for _, m := range tokenPx.FindAllStringSubmatch(cssText, -1) {
		v, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			continue
		}
		out[m[1]] = v
	}
	return out
}

func ruleValues(cssText string) map[string]float64 {
	out := make(map[string]float64)
	for _, m := range ruleBlock.FindAllStringSubmatch(cssText, -1) {
		selectors := strings.Split(m[1], ",")
		for _, sel := range selectors {
			sel = strings.TrimSpace(sel)
			for _, pm := range propPx.FindAllStringSubmatch(m[2], -1) {
				prop := pm[1]
				v, err := strconv.ParseFloat(pm[2], 64)
				if err != nil {
					continue
				}
				out[sel+"|"+prop] = v
				if prop == "padding" {
					parts := strings.Fields(pm[0])
					if len(parts) >= 2 {
						// padding: 0 16px → left/right
						if strings.HasSuffix(parts[len(parts)-1], "px") {
							if vRight, err := strconv.ParseFloat(strings.TrimSuffix(parts[len(parts)-1], "px"), 64); err == nil {
								out[sel+"|padding-right"] = vRight
								if len(parts) == 2 {
									out[sel+"|padding-left"] = vRight
								}
							}
						}
					}
				}
			}
		}
	}
	return out
}

func closeEnough(got, want float64) bool {
	diff := got - want
	if diff < 0 {
		diff = -diff
	}
	return diff <= 1
}
