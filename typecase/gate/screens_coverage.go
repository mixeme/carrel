package gate

import (
	"fmt"
	"sort"
	"strings"

	"carrel.dev/typecase/screens"
)

// CheckScreensCoverage ensures every catalog widget appears on at least one
// assembled screen page.
func CheckScreensCoverage(catalog []CatalogEntry) error {
	want := catalogWidgetNames(catalog)
	var missing []string
	for _, w := range want {
		found := false
		for _, sc := range screens.All() {
			if screens.HasWidget(sc.Body(), w) {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, w)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("screens-coverage: missing widgets on assembled pages: %s", strings.Join(missing, ", "))
	}
	return nil
}

func catalogWidgetNames(catalog []CatalogEntry) []string {
	seen := make(map[string]bool)
	for _, e := range catalog {
		seen[e.Widget] = true
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
