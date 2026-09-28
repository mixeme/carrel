package cal

import (
	"sort"
	"strings"

	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/view"
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

// NoteForeign are the properties of a note Carrel does not know, as they are
// on the server: the name and the value, for NoteDoc.Foreign (C-6).
func NoteForeign(obj *model.Object) []view.Field {
	if obj == nil {
		return nil
	}
	var out []view.Field
	for name, values := range obj.Props() {
		upper := strings.ToUpper(name)
		if noteKnown[upper] {
			continue
		}
		for _, v := range values {
			out = append(out, view.Field{Label: view.Text(upper), Value: view.Text(v.Text), Mono: true})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Label != out[j].Label {
			return out[i].Label < out[j].Label
		}
		return out[i].Value < out[j].Value
	})
	return out
}
