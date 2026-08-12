// Copyright (C) 2026 Carrel contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"bytes"
	"fmt"
)

// ParsedCalendar is one VCALENDAR taken from an import stream.
type ParsedCalendar struct {
	Object *Object
	Source string
	Error  string
}

// ParseICals reads every calendar object from a body that may hold more than
// one VCALENDAR. A bad calendar is reported and skipped; the rest are returned.
func ParseICals(body []byte) []ParsedCalendar {
	chunks := SplitVCalendars(body)
	if len(chunks) == 0 {
		return nil
	}
	out := make([]ParsedCalendar, 0, len(chunks))
	for i, chunk := range chunks {
		obj, err := ParseICal(fmt.Sprintf("cal-%d", i+1), "", chunk)
		if err != nil {
			out = append(out, ParsedCalendar{Error: err.Error()})
			continue
		}
		out = append(out, ParsedCalendar{Object: obj})
	}
	return out
}

// SplitVCalendars cuts a body into individual BEGIN:VCALENDAR…END:VCALENDAR
// blocks.
func SplitVCalendars(body []byte) [][]byte {
	lines := splitKeepNL(body)
	var out [][]byte
	var cur [][]byte
	in := false
	for _, line := range lines {
		upper := bytes.ToUpper(bytes.TrimSpace(line))
		if bytes.HasPrefix(upper, []byte("BEGIN:VCALENDAR")) {
			if in && len(cur) > 0 {
				out = append(out, bytes.Join(cur, nil))
			}
			cur = [][]byte{line}
			in = true
			continue
		}
		if !in {
			continue
		}
		cur = append(cur, line)
		if bytes.HasPrefix(upper, []byte("END:VCALENDAR")) {
			out = append(out, bytes.Join(cur, nil))
			cur = nil
			in = false
		}
	}
	if in && len(cur) > 0 {
		out = append(out, bytes.Join(cur, nil))
	}
	return out
}
