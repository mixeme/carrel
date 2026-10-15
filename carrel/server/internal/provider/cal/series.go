package cal

import (
	"bytes"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-ical"

	"carrel.dev/carrel/server/internal/model"
)

// A series names its exceptions by the days it has (RFC 5545 §3.8.5.1,
// §3.8.4.4). Shorten it, give it another rule, or stop it repeating, and an
// occurrence changed by hand may stand for a day the series no longer has:
// nothing shows it, and other programs drop it. Carrel asks what becomes of
// such occurrences before it writes (the owner's decision of 11.10.2026, А1;
// Typecase's frames s-agenda-22…-24): kept, each is an event of its own with
// what was changed in it; removed, they go. A day left out that is no day of
// the series any more says nothing, and goes without asking.

// Left is an occurrence changed by hand that its series has no day for.
type Left struct {
	At       time.Time // the day it stood for: its RECURRENCE-ID
	AllDay   bool      // the day is a date
	Start    time.Time // where it stands: its DTSTART; zero — none of its own
	Summary  string    // its own title
	Moved    bool      // it stands elsewhere than the day it stood for
	Retitled bool      // its title is not the series'
}

// LeftWithoutDay are the occurrences of obj changed by hand whose day its
// series does not have, in the order of their days.
func LeftWithoutDay(obj *model.Object, loc *time.Location) ([]Left, error) {
	if obj == nil || !obj.IsCalendar() {
		return nil, nil
	}
	if loc == nil {
		loc = time.UTC
	}
	doc, err := decode(obj.CalendarDoc())
	if err != nil {
		return nil, err
	}
	main := findMain(doc)
	if main == nil {
		return nil, nil
	}
	// The record's own properties are the patched ones; the document holds
	// its components as read.
	main.Props = rawToProps(obj.Props())
	has, err := dayOf(main, loc)
	if err != nil {
		return nil, err
	}
	uid := propText(main.Props, ical.PropUID)
	title := propText(main.Props, ical.PropSummary)
	var out []Left
	for _, c := range doc.Children {
		rid := c.Props.Get(ical.PropRecurrenceID)
		if c == main || c.Name != main.Name || rid == nil {
			continue
		}
		if u := propText(c.Props, ical.PropUID); u != "" && u != uid {
			continue
		}
		at, err := propTime(rid, loc)
		if err != nil || has(at) {
			continue
		}
		l := Left{At: at, AllDay: isDate(rid), Summary: propText(c.Props, ical.PropSummary)}
		if p := c.Props.Get(ical.PropDateTimeStart); p != nil {
			if start, err := propTime(p, loc); err == nil {
				l.Start, l.Moved = start, !start.Equal(at)
			}
		}
		l.Retitled = l.Summary != "" && l.Summary != title
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out, nil
}

// dayOf tells the days a series has, whatever it leaves out: the start of a
// record that does not repeat, else every day of its rule and its extra days.
func dayOf(main *ical.Component, loc *time.Location) (func(time.Time) bool, error) {
	set, err := series(main, loc, false)
	if err != nil {
		return nil, err
	}
	if set == nil {
		start, ok := startOf(main, loc)
		return func(at time.Time) bool { return ok && start.Equal(at) }, nil
	}
	return func(at time.Time) bool {
		return len(set.Between(at.Add(-time.Second), at.Add(time.Second), true)) > 0
	}, nil
}

// DropLeft takes out of obj the occurrences changed by hand that stand for
// the days given, and every day left out (EXDATE) the series no longer has.
// Like SetAlarms it is a document edit; nothing to take out hands obj back.
func DropLeft(obj *model.Object, days []time.Time, loc *time.Location) (*model.Object, error) {
	if loc == nil {
		loc = time.UTC
	}
	doc, err := decode(obj.CalendarDoc())
	if err != nil {
		return nil, err
	}
	main := findMain(doc)
	if main == nil {
		return obj, nil
	}
	main.Props = rawToProps(obj.Props())
	has, err := dayOf(main, loc)
	if err != nil {
		return nil, err
	}
	changed := false
	var kept []ical.Prop
	for _, p := range main.Props[ical.PropExceptionDates] {
		var stays []string
		for _, one := range strings.Split(p.Value, ",") {
			q := ical.Prop{Name: p.Name, Params: p.Params, Value: one}
			if at, err := propTime(&q, loc); err == nil && !has(at) {
				changed = true
				continue
			}
			stays = append(stays, one)
		}
		if len(stays) > 0 {
			p.Value = strings.Join(stays, ",")
			kept = append(kept, p)
		}
	}
	if changed {
		if len(kept) > 0 {
			main.Props[ical.PropExceptionDates] = kept
		} else {
			delete(main.Props, ical.PropExceptionDates)
		}
	}
	children := doc.Children[:0]
	for _, c := range doc.Children {
		if rid := c.Props.Get(ical.PropRecurrenceID); c != main && c.Name == main.Name && rid != nil {
			if at, err := propTime(rid, loc); err == nil && containsTime(days, at) {
				changed = true
				continue
			}
		}
		children = append(children, c)
	}
	doc.Children = children
	if !changed {
		return obj, nil
	}
	return reparse(obj, doc)
}

// Apart is the occurrence of obj changed by hand that stands for the day at,
// as an event of its own: its component under uid, without RECURRENCE-ID,
// with the zones it names. False when obj has no such occurrence.
func Apart(obj *model.Object, at time.Time, uid string, loc *time.Location) (*model.Object, bool, error) {
	if loc == nil {
		loc = time.UTC
	}
	doc, err := decode(obj.CalendarDoc())
	if err != nil {
		return nil, false, err
	}
	main := findMain(doc)
	if main == nil {
		return nil, false, nil
	}
	var one *ical.Component
	for _, c := range doc.Children {
		if rid := c.Props.Get(ical.PropRecurrenceID); c != main && c.Name == main.Name && rid != nil {
			if t, err := propTime(rid, loc); err == nil && t.Equal(at) {
				one = c
				break
			}
		}
	}
	if one == nil {
		return nil, false, nil
	}
	ev := &ical.Component{Name: one.Name, Props: ical.Props{}, Children: one.Children}
	for name, ps := range one.Props {
		if name == ical.PropRecurrenceID {
			continue
		}
		ev.Props[name] = ps
	}
	ev.Props.SetText(ical.PropUID, uid)
	out := ical.NewCalendar()
	out.Props = doc.Props
	for _, c := range doc.Children {
		if c.Name == ical.CompTimezone {
			out.Children = append(out.Children, c)
		}
	}
	out.Children = append(out.Children, ev)
	got, err := reparse(&model.Object{}, out)
	return got, err == nil, err
}

// reparse writes doc and reads it back as obj's record.
func reparse(obj *model.Object, doc *ical.Calendar) (*model.Object, error) {
	var buf bytes.Buffer
	if err := ical.NewEncoder(&buf).Encode(doc); err != nil {
		return nil, err
	}
	return Parse(obj.Path, obj.ETag, bytes.NewReader(model.FoldLines(buf.Bytes())))
}

func containsTime(ts []time.Time, at time.Time) bool {
	for _, t := range ts {
		if t.Equal(at) {
			return true
		}
	}
	return false
}
