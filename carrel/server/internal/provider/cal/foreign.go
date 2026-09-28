package cal

import (
	"slices"
	"sort"
	"strings"

	"carrel.dev/carrel/server/internal/model"
)

// noteKnown are the properties of a note that its page shows or keeps by
// itself: the form (title, text, date, tags, relation), the attachments, and
// the bookkeeping every calendar object carries. Anything else is the
// property of another client, kept as it is (S-7) and named under the text of
// the edit (s-notes-20).
var noteKnown = map[string]bool{
	"UID": true, "DTSTAMP": true, "CREATED": true, "LAST-MODIFIED": true, "SEQUENCE": true,
	"SUMMARY": true, "DESCRIPTION": true, "DTSTART": true, "CATEGORIES": true,
	"RELATED-TO": true, "ATTACH": true,
}

// NoteForeign are the names of the properties of a note Carrel does not
// know, each once, in the order of their names: the first is the one the
// line under the text of the edit names (NoteDoc.Foreign, s-notes-20).
func NoteForeign(obj *model.Object) []string {
	if obj == nil {
		return nil
	}
	var out []string
	for name := range obj.Props() {
		if upper := strings.ToUpper(name); !noteKnown[upper] && !slices.Contains(out, upper) {
			out = append(out, upper)
		}
	}
	sort.Strings(out)
	return out
}
