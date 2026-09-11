package gate

import (
	"fmt"
	"regexp"
	"strings"
)

const maxInvariantTodos = 65

var (
	segChosenRe    = regexp.MustCompile(`aria-pressed="true"`)
	crumbsCurrentRe = regexp.MustCompile(`class="tc-crumbs__current"`)
)

func checkInvariantGate(ref, widget string, catalog []CatalogEntry) error {
	name := strings.TrimPrefix(ref, "gate:")
	var html []string
	for _, e := range catalog {
		if e.Widget == widget {
			html = append(html, e.HTML)
		}
	}
	if len(html) == 0 {
		return fmt.Errorf("%s: no catalog HTML for gate %s", widget, name)
	}
	switch name {
	case "seg-one-chosen":
		for _, h := range html {
			if strings.Count(h, `aria-pressed="true"`) != 1 {
				return fmt.Errorf("%s: seg must have exactly one chosen option", widget)
			}
		}
	case "crumbs-last-not-link":
		for _, h := range html {
			if !crumbsCurrentRe.MatchString(h) {
				return fmt.Errorf("%s: crumbs last item must not be a link", widget)
			}
		}
	case "range-order":
		for _, h := range html {
			if strings.Contains(h, "is-invalid") {
				return fmt.Errorf("%s: range must not be invalid in catalog", widget)
			}
		}
	case "table-number-columns":
		for _, h := range html {
			if strings.Contains(h, "tc-table__th--number") && !strings.Contains(h, "tc-table__td--number") {
				return fmt.Errorf("%s: number columns must align in header and body", widget)
			}
		}
	default:
		return nil
	}
	return nil
}
