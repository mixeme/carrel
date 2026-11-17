package davtest

import "strings"

// SplitRecords cuts a file of vCards or of iCalendar into one body per
// record: the DAV servers of the world hold one object per address, and so
// does davtest. A calendar's record keeps the head of the file — its own
// lines and its VTIMEZONEs.
func SplitRecords(body string) []string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	if strings.Contains(body, "BEGIN:VCARD") {
		var out []string
		for _, part := range strings.SplitAfter(body, "END:VCARD\n") {
			if strings.Contains(part, "BEGIN:VCARD") {
				out = append(out, strings.TrimLeft(part, "\n"))
			}
		}
		return out
	}
	// The head is everything before the first component of the set: the
	// calendar's own lines and its VTIMEZONEs. Cutting at the first
	// «BEGIN:V» left the zones behind, and an event whose TZID nothing
	// defines is an event nobody can place.
	cut := len(body)
	for _, comp := range []string{"BEGIN:VEVENT", "BEGIN:VTODO", "BEGIN:VJOURNAL"} {
		if at := strings.Index(body, comp); at >= 0 && at < cut {
			cut = at
		}
	}
	head := body[:cut]
	var out []string
	for _, comp := range []string{"VEVENT", "VTODO", "VJOURNAL"} {
		for _, part := range strings.SplitAfter(body, "END:"+comp+"\n") {
			if !strings.Contains(part, "BEGIN:"+comp) {
				continue
			}
			at := strings.Index(part, "BEGIN:"+comp)
			out = append(out, head+part[at:]+"END:VCALENDAR\n")
		}
	}
	return out
}
