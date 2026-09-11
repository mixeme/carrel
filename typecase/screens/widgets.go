package screens

import (
	"regexp"
	"sort"
)

var screenWidgetClass = regexp.MustCompile(`\btc-([a-z][a-z0-9-]*)\b`)

// WidgetNames returns every catalog widget name present in one HTML fragment.
func WidgetNames(html string) []string {
	found := make(map[string]bool)
	for _, m := range screenWidgetClass.FindAllStringSubmatch(html, -1) {
		found["tc-"+m[1]] = true
	}
	out := make([]string, 0, len(found))
	for name := range found {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// HasWidget reports whether html contains a catalog widget name as a class token.
func HasWidget(html, widget string) bool {
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(widget) + `\b`)
	return re.MatchString(html)
}
