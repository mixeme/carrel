package model

import (
	"strings"
)

// calendarPayload holds a full iCalendar document. Patch applies only to the
// main component (first VEVENT / VTODO / VJOURNAL without RECURRENCE-ID).
type calendarPayload struct {
	doc      []byte
	mainName string
	main     map[string]propState
}

// NewCalendarObject constructs a calendar record from a parsed main component
// and the full wire document. Siblings (VTIMEZONE, VALARM, RECURRENCE-ID
// instances, VCALENDAR props) survive encode via doc.
func NewCalendarObject(path, etag, mainName string, mainProps map[string][]RawValue, doc []byte) *Object {
	cp := make(map[string]propState, len(mainProps))
	for name, values := range mainProps {
		st := propState{values: make([]propValue, len(values))}
		for i, v := range values {
			st.values[i].text = v.Text
			if len(v.Params) > 0 {
				st.values[i].params = make(map[string]string, len(v.Params))
				for k, pv := range v.Params {
					st.values[i].params[strings.ToUpper(k)] = pv
				}
			}
		}
		cp[strings.ToUpper(name)] = st
	}
	return &Object{
		Path: path,
		ETag: etag,
		cal: &calendarPayload{
			doc:      append([]byte(nil), doc...),
			mainName: strings.ToUpper(mainName),
			main:     cp,
		},
	}
}

// IsCalendar reports whether obj holds an iCalendar document.
func (o *Object) IsCalendar() bool {
	return o != nil && o.cal != nil
}

// CalendarDoc returns the last full document bytes for encode.
func (o *Object) CalendarDoc() []byte {
	if o == nil || o.cal == nil {
		return nil
	}
	return append([]byte(nil), o.cal.doc...)
}

// MainComponent returns VEVENT, VTODO, or VJOURNAL.
func (o *Object) MainComponent() string {
	if o == nil || o.cal == nil {
		return ""
	}
	return o.cal.mainName
}

func (o *Object) activePayload() *payload {
	if o == nil {
		return nil
	}
	if o.cal != nil {
		return &payload{props: o.cal.main}
	}
	return &o.raw
}
