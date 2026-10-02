package view

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The fields the search, the duplicates, the conflict, the import and the
// collection sheets read are C-5's: a client knows them from the map, not
// from the server's code (the client's scan of these sections, ПБ9, ДбБ9,
// РБ9, ИБ10, КА4). Each is looked for in the row of its route.
func TestC5NamesTheFieldsOfTheCrossScreens(t *testing.T) {
	src, err := os.ReadFile(specFile)
	if err != nil {
		t.Fatal(err)
	}
	spec := strings.ReplaceAll(string(src), "\r\n", "\n")
	rowOf := func(route string) string {
		for _, line := range strings.Split(spec, "\n") {
			if strings.HasPrefix(line, "| `"+route+"` |") {
				return line
			}
		}
		t.Fatalf("C-5 has no row for %s", route)
		return ""
	}
	for route, fields := range map[string][]string{
		"/app/search":                                {"q", "kind", "filter", "again"},
		"/app/duplicates/decide":                     {"do", "group", "choice.<свойство>", "value", "weight.<признак>", "threshold", "weights", "defaults", "link", "choose", "unlink", "ignore", "again"},
		"/app/duplicates/merge":                      {"group", "into", "choice.<свойство>", "v.<место>"},
		"/app/{kind}/{account}/{col}/{uid}/conflict": {"op", "combine", "mine", "theirs", "cancel", "etag", "choice.<n>"},
		"/app/{kind}/{account}/{col}/import":         {"to", "into", "op", "run", "cancel", "folder", "path", "in", "file"},
		"/app/collections/new":                       {"back", "kind", "a", "components", "op", "account", "name", "colour", "address"},
		"/app/collections/rename":                    {"a", "c", "back", "name", "colour"},
		"/app/collections/move":                      {"a", "c", "to", "copy", "back"},
		"/app/collections/delete":                    {"a", "c", "back", "name"},
	} {
		row := rowOf(route)
		for _, field := range fields {
			// A field is named in backticks, alone or as a parameter or a
			// value: `name`, `?view=…`, `do=threshold`.
			if !regexp.MustCompile(`[\x60=]\??` + regexp.QuoteMeta(field) + `[\x60=]`).MatchString(row) {
				t.Errorf("C-5 does not name %s of %s", field, route)
			}
		}
	}
}
