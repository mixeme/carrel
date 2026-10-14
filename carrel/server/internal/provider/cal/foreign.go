package cal

import (
	"slices"
	"sort"
	"strings"

	"carrel.dev/carrel/server/internal/model"
)

// noteKnown are the properties of a note that its page shows or keeps by
// itself: the form (title, text, date, tags, link, relation), the
// attachments, and the bookkeeping every calendar object carries. Anything
// else is the property of another client, kept as it is (S-7) and named under
// the text of the edit (s-notes-20).
var noteKnown = map[string]bool{
	"UID": true, "DTSTAMP": true, "CREATED": true, "LAST-MODIFIED": true, "SEQUENCE": true,
	"SUMMARY": true, "DESCRIPTION": true, "DTSTART": true, "CATEGORIES": true, "URL": true,
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

// eventKnown are the properties of an event its card and its form show or
// keep by themselves: the fields, the times and the rule, the people, the
// attachments and links, and the bookkeeping every calendar object carries.
// SEQUENCE is not among them: Carrel does not schedule, and the form shows
// it as kept (s-agenda-3).
var eventKnown = map[string]bool{
	"UID": true, "DTSTAMP": true, "CREATED": true, "LAST-MODIFIED": true,
	"SUMMARY": true, "LOCATION": true, "STATUS": true, "CATEGORIES": true, "DESCRIPTION": true, "URL": true,
	"DTSTART": true, "DTEND": true, "DURATION": true, "RRULE": true, "RDATE": true, "EXDATE": true, "RECURRENCE-ID": true,
	"ORGANIZER": true, "ATTENDEE": true, "ATTACH": true, "RELATED-TO": true,
}

// EventForeign are the names of the properties of an event Carrel does not
// know, each once, in the order of their names: «Kept as the server sent
// them» of its form (s-agenda-3).
func EventForeign(obj *model.Object) []string {
	if obj == nil {
		return nil
	}
	var out []string
	for name := range obj.Props() {
		if upper := strings.ToUpper(name); !eventKnown[upper] && !slices.Contains(out, upper) {
			out = append(out, upper)
		}
	}
	sort.Strings(out)
	return out
}

// taskKnown are the properties of a task its card and its form show or keep
// by themselves: the fields, the times, the progress, the rule it repeats
// by, the organiser its card names, the attachments and links, and the
// bookkeeping every calendar object carries. SEQUENCE is shown as kept, as
// an event's is.
var taskKnown = map[string]bool{
	"UID": true, "DTSTAMP": true, "CREATED": true, "LAST-MODIFIED": true,
	"SUMMARY": true, "DESCRIPTION": true, "LOCATION": true, "CATEGORIES": true, "URL": true,
	"DTSTART": true, "DUE": true, "DURATION": true, "RRULE": true, "RDATE": true, "EXDATE": true, "RECURRENCE-ID": true,
	"STATUS": true, "PRIORITY": true, "PERCENT-COMPLETE": true, "COMPLETED": true,
	"ORGANIZER": true, "ATTACH": true, "RELATED-TO": true,
}

// TaskForeign are the names of the properties of a task Carrel does not
// know, each once, in the order of their names: «Kept as the server sent
// them» of its form, as an event's (Р17 of the kinds-parity list: a task
// showed them nowhere).
func TaskForeign(obj *model.Object) []string {
	if obj == nil {
		return nil
	}
	var out []string
	for name := range obj.Props() {
		if upper := strings.ToUpper(name); !taskKnown[upper] && !slices.Contains(out, upper) {
			out = append(out, upper)
		}
	}
	sort.Strings(out)
	return out
}
