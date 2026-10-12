package cal

import (
	"bytes"
	"sort"
	"strings"

	"github.com/emersion/go-ical"

	"carrel.dev/carrel/server/internal/model"
)

// TimeProps are the time of a record as one piece: the start with its zone,
// the end or the length, a task's due date, and the repetition with the days
// it adds and leaves out. A part of it taken alone would pair a time with a
// rule made for another, so it is read, compared and taken whole.
var TimeProps = []string{"DTSTART", "DTEND", "DURATION", "DUE", "RRULE", "RDATE", "EXDATE"}

// TimeKey is the time of a record as it is written, for telling two times
// apart: the same time written the same way has the same key. A time written
// differently is another time — nobody here decides that two spellings mean
// one moment.
func TimeKey(obj *model.Object) string {
	props := obj.Props()
	var b strings.Builder
	for _, name := range TimeProps {
		for _, v := range props[name] {
			b.WriteString(name)
			keys := make([]string, 0, len(v.Params))
			for k := range v.Params {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				b.WriteString(";" + k + "=" + v.Params[k])
			}
			b.WriteString(":" + v.Text + "\n")
		}
	}
	return b.String()
}

// TakeTime gives dst the time of src, whole (TimeProps), and returns the
// record to Update. With the time come what belongs to it: the occurrences
// src rewrote by hand, which become dst's — they carry its UID now —, and the
// definitions of the zones src names, where dst has none of that name. The
// occurrences dst rewrote belong to the time it gives up and go with it;
// dropped says how many. Everything else of dst stays, properties already
// patched on it included. The patch signature is untouched: like SetAlarms,
// this is a document edit.
func TakeTime(dst, src *model.Object) (out *model.Object, dropped int, err error) {
	if _, err := mainComponent(dst); err != nil {
		return nil, 0, err
	}
	if _, err := mainComponent(src); err != nil {
		return nil, 0, err
	}
	dcal, err := decode(dst.CalendarDoc())
	if err != nil {
		return nil, 0, err
	}
	scal, err := decode(src.CalendarDoc())
	if err != nil {
		return nil, 0, err
	}
	dmain, smain := findMain(dcal), findMain(scal)

	props, theirs := dst.Props(), src.Props()
	for _, name := range TimeProps {
		delete(props, name)
		if v := theirs[name]; len(v) > 0 {
			props[name] = v
		}
	}
	dmain.Props = rawToProps(props)

	uid := propText(dmain.Props, ical.PropUID)
	zones := map[string]bool{}
	var children []*ical.Component
	for _, child := range dcal.Children {
		switch {
		case child.Name == dmain.Name && child.Props.Get(ical.PropRecurrenceID) != nil:
			dropped++
			continue
		case child.Name == ical.CompTimezone:
			zones[propText(child.Props, ical.PropTimezoneID)] = true
		}
		children = append(children, child)
	}
	suid := propText(smain.Props, ical.PropUID)
	var zonesIn, instances []*ical.Component
	for _, child := range scal.Children {
		switch {
		case child.Name == ical.CompTimezone:
			if id := propText(child.Props, ical.PropTimezoneID); id != "" && !zones[id] {
				zones[id] = true
				zonesIn = append(zonesIn, child)
			}
		case child.Name == smain.Name && child.Props.Get(ical.PropRecurrenceID) != nil:
			if u := propText(child.Props, ical.PropUID); u != "" && u != suid {
				continue
			}
			child.Props.SetText(ical.PropUID, uid)
			instances = append(instances, child)
		}
	}
	// Zones stand before the components that name them.
	dcal.Children = append(append(zonesIn, children...), instances...)

	var buf bytes.Buffer
	if err := ical.NewEncoder(&buf).Encode(dcal); err != nil {
		return nil, 0, err
	}
	return model.NewCalendarObject(dst.Path, dst.ETag, dmain.Name, props, buf.Bytes()), dropped, nil
}
