package web

import "testing"

// The notice of a save Carrel would not make says each thing that went wrong
// directly (the owner's words of 02.11.2026): a field of one short line with
// what it would hold instead of what was entered, any other field by its
// name, a property by its name; and the details are promised only when there
// are some.
func TestFaultNoticeSaysWhatWentWrong(t *testing.T) {
	const tail = " Nothing sent to the server; the form keeps your entry. Report this as a Carrel bug"
	for _, c := range []struct {
		fault editFault
		body  string
	}{
		{editFault{fields: []faultField{{label: "Location", entered: "Room 4, floor 2", saved: "Room 4", shown: true}}, props: []string{"X-ALT-DESC"}, lines: []string{"- X-ALT-DESC:x"}},
			"Pre-save check failed: the field Location would be saved as «Room 4» instead of «Room 4, floor 2»; the property X-ALT-DESC would change, though not edited." + tail + " — details below."},
		{editFault{fields: []faultField{{label: "Notes", entered: "", saved: "old", shown: true}, {label: "Attendees"}}},
			"Pre-save check failed: the field Notes would be saved as «old» instead of empty; the field Attendees would not be saved as entered." + tail + "."},
		{editFault{restored: []string{"CATEGORIES"}, props: []string{"GEO", "URL"}, lines: []string{"+ GEO:1;2"}},
			"Pre-save check failed: the property CATEGORIES would not be restored as the version you opened had it; the properties GEO and URL would change, though not edited." + tail + " — details below."},
		{editFault{unread: true},
			"Pre-save check failed: the record would not read back after the change." + tail + "."},
	} {
		n := faultNotice(&c.fault)
		if n.Title != "Internal error: edit not saved" || string(n.Body) != c.body {
			t.Errorf("notice = %q / %q,\nwant %q", n.Title, n.Body, c.body)
		}
		if (n.Details != nil) != (len(c.fault.lines) > 0) {
			t.Errorf("details = %+v for lines %q", n.Details, c.fault.lines)
		}
	}
}
