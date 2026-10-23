package cal

import (
	"fmt"
	"strings"
	"time"

	"github.com/teambition/rrule-go"
)

// The times and the rule of a record are read before it is assembled
// (Typecase's notes 111 p. 6, 113; s-io-7): one that cannot be read
// refuses that record, with the line of the file it stands on, and no
// other.

// timedProps are the properties whose value is a date or a date and time.
var timedProps = map[string]bool{"DTSTART": true, "DTEND": true, "DUE": true, "RECURRENCE-ID": true}

// checkTimes reads the times and the rule of a component's own lines (its
// subcomponents — an alarm — are not its times): the first one it cannot
// read, as «line 88: cannot read DUE «2026-13-01»».
func checkTimes(c chunk) error {
	depth := 0
	for i, line := range c.lines {
		if i == 0 || isFold(line) {
			continue
		}
		full := unfoldAt(c.lines, i)
		if _, ok := beginName(full); ok {
			depth++
			continue
		}
		if isEnd(full) {
			depth--
			continue
		}
		if depth > 0 {
			continue
		}
		name, value, ok := propOf(full)
		if !ok {
			continue
		}
		bad := false
		switch {
		case timedProps[name]:
			bad = !readableTime(paramsOf(full), value)
		case name == "RRULE":
			_, err := rrule.StrToROption(value)
			bad = err != nil
		}
		if bad {
			return fmt.Errorf("line %d: cannot read %s «%s»", c.at+i+1, name, value)
		}
	}
	return nil
}

// readableTime says whether a DATE or DATE-TIME value can be read.
func readableTime(params, value string) bool {
	value = strings.TrimSpace(value)
	if strings.Contains(strings.ToUpper(params), "VALUE=DATE") && !strings.Contains(strings.ToUpper(params), "VALUE=DATE-TIME") {
		_, err := time.Parse("20060102", value)
		return err == nil
	}
	for _, layout := range []string{"20060102T150405Z", "20060102T150405", "20060102"} {
		if _, err := time.Parse(layout, value); err == nil {
			return true
		}
	}
	return false
}

// paramsOf is what stands between a content line's name and its colon.
func paramsOf(line string) string {
	colon := strings.IndexByte(line, ':')
	if colon < 0 {
		return ""
	}
	head := line[:colon]
	if semi := strings.IndexByte(head, ';'); semi >= 0 {
		return head[semi+1:]
	}
	return ""
}
