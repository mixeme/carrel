package gate

import (
	"fmt"
	"strings"
)

// invariantBindings maps each widget to executable checks for its invariant clauses
// (semicolon-separated in L-3.2 «Инварианты»). Order matches clause order.
var invariantBindings = map[string][]string{
	"tc-avatar":   {"gate:inv-avatar-not-empty", "gate:inv-avatar-square"},
	"tc-badge":    {"gate:inv-badge-has-text", "gate:inv-badge-not-link"},
	"tc-bar":      {"browser:bar-one-row", "browser:bar-one-row"},
	"tc-brand":    {"gate:inv-brand-single-mark", "gate:inv-brand-no-dup-word"},
	"tc-btn":      {"gate:enumerations", "gate:a11y", "gate:a11y"},
	"tc-card":     {"gate:inv-card-has-body"},
	"tc-check":    {"gate:a11y", "gate:enumerations"},
	"tc-chip":     {"gate:inv-chip-state-class"},
	"tc-close":    {"gate:a11y", "gate:a11y"},
	"tc-crop":     {"gate:inv-crop-has-frame", "gate:inv-crop-has-zoom"},
	"tc-crumbs":   {"gate:crumbs-last-not-link", "gate:inv-crumbs-has-root"},
	"tc-danger":   {"gate:a11y", "gate:a11y"},
	"tc-diag":     {"gate:inv-diag-collapsed-default", "gate:inv-diag-mono"},
	"tc-dialog":   {"browser:esc-layers", "gate:a11y"},
	"tc-diff":     {"gate:inv-diff-options-column", "gate:inv-diff-one-chosen"},
	"tc-doc":      {"gate:inv-doc-mode-switch", "gate:inv-doc-no-silent-save"},
	"tc-docbar":   {"gate:inv-docbar-actions", "gate:inv-docbar-dirty-flag"},
	"tc-docmeta":  {"gate:inv-docmeta-sections"},
	"tc-empty":    {"gate:a11y"},
	"tc-field":    {"gate:a11y", "gate:a11y"},
	"tc-fields":   {"browser:ellipsis", "gate:inv-fields-dl-structure"},
	"tc-form":     {"gate:a11y", "gate:a11y", "gate:a11y"},
	"tc-formfoot": {"gate:a11y", "gate:a11y"},
	"tc-fset":     {"gate:inv-fset-rubric"},
	"tc-grid":     {"gate:inv-grid-tiles", "gate:inv-grid-uniform"},
	"tc-head":     {"gate:inv-head-title"},
	"tc-in":       {"gate:a11y", "gate:a11y"},
	"tc-kbd":      {"gate:inv-kbd-not-button"},
	"tc-list":     {"gate:a11y", "gate:inv-list-empty-action"},
	"tc-main":     {"gate:inv-main-body"},
	"tc-mark":     {"gate:a11y", "gate:a11y"},
	"tc-menu":     {"browser:menu-label-arrow", "browser:popover-anchor", "browser:focus-return"},
	"tc-meter":    {"gate:inv-meter-has-value"},
	"tc-msg":      {"gate:a11y", "gate:a11y"},
	"tc-nav":      {"gate:inv-nav-one-current"},
	"tc-num":      {"gate:typed-input", "gate:inv-num-unknown-hidden"},
	"tc-overlay":  {"gate:inv-overlay-kind-class", "browser:popover-anchor"},
	"tc-prose":    {"gate:typed-input"},
	"tc-queue":    {"gate:inv-queue-question", "gate:inv-queue-meter"},
	"tc-rail":     {"gate:inv-rail-foot", "gate:inv-rail-drawer-class"},
	"tc-railfoot": {"gate:inv-railfoot-build", "gate:inv-railfoot-actions"},
	"tc-railsec":  {"gate:inv-railsec-state", "gate:inv-railsec-untitled-no-title"},
	"tc-range":    {"gate:range-order", "gate:inv-range-field-width"},
	"tc-row":      {"gate:inv-row-mark", "gate:inv-row-tail", "gate:inv-row-menu-in-tail"},
	"tc-rubric":   {"gate:inv-rubric-not-heading"},
	"tc-runbar":   {"gate:inv-runbar-meter-hidden-idle", "gate:inv-runbar-states"},
	"tc-scrim":    {"gate:inv-scrim-for-attr", "browser:esc-layers"},
	"tc-search":   {"gate:inv-search-popover-below", "gate:inv-search-no-empty-submit"},
	"tc-sec":      {"gate:inv-sec-like-card"},
	"tc-seg":      {"gate:seg-one-chosen", "gate:inv-seg-narrow-menu"},
	"tc-sheet":    {"browser:esc-layers", "gate:a11y", "gate:a11y"},
	"tc-shell":    {"gate:inv-shell-four-zones", "browser:theme-geometry"},
	"tc-side":     {"browser:esc-layers"},
	"tc-skel":     {"gate:inv-skel-shape"},
	"tc-stat":     {"gate:inv-stat-string-value"},
	"tc-stats":    {"gate:inv-stats-no-wrap"},
	"tc-steps":    {"gate:inv-steps-one-current", "gate:inv-steps-count"},
	"tc-table":    {"gate:inv-table-column-kind", "gate:table-number-columns", "gate:inv-table-scrollable"},
	"tc-tag":      {"gate:inv-tag-not-editable"},
	"tc-thumb":    {"gate:inv-thumb-placeholder", "gate:inv-thumb-icon-fallback"},
	"tc-tick":     {"gate:a11y", "gate:enumerations"},
	"tc-tickrow":  {"gate:enumerations", "gate:inv-tickrow-count-optional"},
	"tc-tile":     {"gate:inv-tile-has-name"},
	"tc-top":      {"gate:inv-top-brand-once", "gate:inv-top-search"},
	"tc-tree":     {"gate:inv-tree-one-chosen", "gate:a11y"},
	"tc-usermenu": {"browser:focus-return", "gate:inv-usermenu-open-state"},
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
		if isRunnableInvGate(name) {
			return true
		}
		return gateImplemented(Name(name))
	case strings.HasPrefix(ref, "browser:"):
		name := strings.TrimPrefix(ref, "browser:")
		for _, c := range browserChecks {
			if c == name {
				return true
			}
		}
		return false
	case strings.HasPrefix(ref, "todo:"):
		return false
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
				gateName := strings.TrimPrefix(ref, "gate:")
				if isRunnableInvGate(gateName) {
					if err := checkInvariantGate(ref, name, catalog, widgetDir); err != nil {
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
