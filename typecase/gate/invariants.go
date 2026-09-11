package gate

import (
	"fmt"
	"strings"
)

// invariantBindings maps each widget to executable checks for its invariant clauses
// (semicolon-separated in L-3.2 «Инварианты»). Order matches clause order.
var invariantBindings = map[string][]string{
	"tc-avatar":   {"todo:pending", "todo:pending"},
	"tc-badge":    {"todo:pending", "todo:pending"},
	"tc-bar":      {"browser:bar-one-row", "browser:bar-one-row"},
	"tc-brand":    {"todo:pending", "todo:pending"},
	"tc-btn":      {"gate:enumerations", "gate:a11y", "gate:a11y"},
	"tc-card":     {"todo:pending"},
	"tc-check":    {"gate:a11y", "gate:enumerations"},
	"tc-chip":     {"todo:pending"},
	"tc-close":    {"gate:a11y", "gate:a11y"},
	"tc-crop":     {"todo:pending", "todo:pending"},
	"tc-crumbs":   {"gate:crumbs-last-not-link", "todo:collapse"},
	"tc-danger":   {"gate:a11y", "gate:a11y"},
	"tc-diag":     {"todo:pending", "todo:pending"},
	"tc-dialog":   {"browser:esc-layers", "gate:a11y"},
	"tc-diff":     {"todo:pending", "todo:pending"},
	"tc-doc":      {"todo:pending", "todo:pending"},
	"tc-docbar":   {"todo:pending", "todo:pending"},
	"tc-docmeta":  {"todo:pending"},
	"tc-empty":    {"gate:a11y"},
	"tc-field":    {"gate:a11y", "gate:a11y"},
	"tc-fields":   {"browser:ellipsis", "todo:pending"},
	"tc-form":     {"gate:a11y", "gate:a11y", "gate:a11y"},
	"tc-formfoot": {"gate:a11y", "gate:a11y"},
	"tc-fset":     {"todo:pending"},
	"tc-grid":     {"todo:pending", "todo:pending"},
	"tc-head":     {"todo:pending"},
	"tc-in":       {"gate:a11y", "gate:a11y"},
	"tc-kbd":      {"todo:pending"},
	"tc-list":     {"gate:a11y", "todo:pending"},
	"tc-main":     {"todo:pending"},
	"tc-mark":     {"gate:a11y", "gate:a11y"},
	"tc-menu":     {"browser:menu-label-arrow", "browser:popover-anchor", "browser:focus-return"},
	"tc-meter":    {"todo:pending"},
	"tc-msg":      {"gate:a11y", "gate:a11y"},
	"tc-nav":      {"todo:pending"},
	"tc-num":      {"gate:typed-input", "todo:pending"},
	"tc-overlay":  {"todo:pending", "browser:popover-anchor"},
	"tc-prose":    {"gate:typed-input"},
	"tc-queue":    {"todo:pending", "todo:pending"},
	"tc-rail":     {"todo:pending", "todo:pending"},
	"tc-railfoot": {"todo:pending", "todo:pending"},
	"tc-railsec":  {"todo:pending"},
	"tc-range":    {"gate:range-order", "todo:field-width"},
	"tc-row":      {"todo:pending", "todo:pending", "todo:pending"},
	"tc-rubric":   {"todo:pending"},
	"tc-runbar":   {"todo:pending", "todo:pending"},
	"tc-scrim":    {"todo:pending", "browser:esc-layers"},
	"tc-search":   {"todo:pending", "todo:pending"},
	"tc-sec":      {"todo:pending"},
	"tc-seg":      {"gate:seg-one-chosen", "todo:narrow-menu"},
	"tc-sheet":    {"browser:esc-layers", "gate:a11y", "gate:a11y"},
	"tc-shell":    {"todo:pending", "browser:theme-geometry"},
	"tc-side":     {"browser:esc-layers"},
	"tc-skel":     {"todo:pending"},
	"tc-steps":    {"todo:pending", "todo:pending"},
	"tc-table":    {"todo:column-width", "gate:table-number-columns", "todo:scroll"},
	"tc-tag":      {"todo:pending"},
	"tc-thumb":    {"todo:pending", "todo:pending"},
	"tc-tick":     {"gate:a11y", "gate:enumerations"},
	"tc-tickrow":  {"gate:enumerations", "todo:pending"},
	"tc-tile":     {"todo:pending"},
	"tc-top":      {"todo:pending", "todo:pending"},
	"tc-tree":     {"todo:pending", "gate:a11y"},
	"tc-usermenu": {"browser:focus-return", "todo:pending"},
}

func splitInvariantClauses(text string) []string {
	text = strings.TrimSpace(text)
	low := strings.ToLower(text)
	if text == "" || low == "нет" || strings.HasPrefix(low, "нет ") {
		return nil
	}
	parts := strings.Split(text, ";")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func checkRefValid(ref string) bool {
	switch {
	case strings.HasPrefix(ref, "gate:"):
		name := strings.TrimPrefix(ref, "gate:")
		switch name {
		case "seg-one-chosen", "crumbs-last-not-link", "range-order", "table-number-columns":
			return true
		default:
			return gateImplemented(Name(name))
		}
	case strings.HasPrefix(ref, "browser:"):
		name := strings.TrimPrefix(ref, "browser:")
		for _, c := range browserChecks {
			if c == name {
				return true
			}
		}
		return false
	case strings.HasPrefix(ref, "todo:"):
		return true
	case ref == "widget:smoke":
		return false
	default:
		return false
	}
}

// CheckInvariantCoverage ensures every invariant clause has a named executable check.
func CheckInvariantCoverage(widgetDir string, catalog []CatalogEntry) error {
	widgets, err := LoadWidgets(widgetDir)
	if err != nil {
		return err
	}
	var problems []string
	todos := 0
	for name, spec := range widgets {
		clauses := splitInvariantClauses(spec.Sections["Инварианты"])
		if len(clauses) == 0 {
			continue
		}
		checks, ok := invariantBindings[name]
		if !ok {
			problems = append(problems, name+": no invariant bindings")
			continue
		}
		if len(checks) != len(clauses) {
			problems = append(problems, fmt.Sprintf("%s: %d clauses, %d checks", name, len(clauses), len(checks)))
			continue
		}
		for i, ref := range checks {
			if strings.HasPrefix(ref, "todo:") {
				todos++
				continue
			}
			if !checkRefValid(ref) {
				problems = append(problems, fmt.Sprintf("%s[%d]: unknown check %q", name, i, ref))
				continue
			}
			if strings.HasPrefix(ref, "gate:") {
				switch strings.TrimPrefix(ref, "gate:") {
				case "seg-one-chosen", "crumbs-last-not-link", "range-order", "table-number-columns":
					if err := checkInvariantGate(ref, name, catalog); err != nil {
						problems = append(problems, err.Error())
					}
				}
			}
		}
	}
	if todos > maxInvariantTodos {
		problems = append(problems, fmt.Sprintf("%d todos exceed ceiling %d", todos, maxInvariantTodos))
	}
	if len(problems) > 0 {
		return fmt.Errorf("invariant-coverage: %s", strings.Join(problems, "; "))
	}
	return nil
}
