package cal

import (
	"bytes"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-ical"

	"carrel.dev/carrel/server/internal/model"
)

// A series names its exceptions by the start they would have had: a day left
// out is an EXDATE of that start, a day changed apart is a component beside
// the series whose RECURRENCE-ID is that start (RFC 5545 §3.8.5.1, §3.8.4.4).
// Give the series another time or another zone and those starts are no
// longer anybody's: the day left out comes back and the day changed hangs
// loose. So the exceptions go with the series (the owner's decision of
// 11.10.2026).

// stamp is a date or a date-time as iCalendar writes it: the wall clock, and
// what it is the wall clock of.
type stamp struct {
	wall time.Time      // the clock as written, kept in UTC
	day  bool           // a whole day
	utc  bool           // written with Z
	zone string         // TZID, as written
	loc  *time.Location // the zone, when the database knows it
}

func readStamp(text string, params map[string]string) (stamp, bool) {
	text = strings.TrimSpace(text)
	var s stamp
	switch {
	case len(text) == len("20060102"):
		t, err := time.Parse("20060102", text)
		if err != nil {
			return s, false
		}
		s.wall, s.day = t, true
	case strings.HasSuffix(text, "Z"):
		t, err := time.Parse("20060102T150405Z", text)
		if err != nil {
			return s, false
		}
		s.wall, s.utc, s.loc = t, true, time.UTC
	default:
		t, err := time.Parse("20060102T150405", text)
		if err != nil {
			return s, false
		}
		s.wall, s.zone = t, params["TZID"]
		if s.zone != "" {
			if loc, err := time.LoadLocation(s.zone); err == nil {
				s.loc = loc
			}
		}
	}
	return s, true
}

// instant is the moment a stamp names; a floating time and a whole day are
// taken as written, in UTC — two of them compare as their clocks do.
func (s stamp) instant() time.Time {
	if s.loc == nil || s.day {
		return s.wall
	}
	w := s.wall
	return time.Date(w.Year(), w.Month(), w.Day(), w.Hour(), w.Minute(), w.Second(), 0, s.loc).UTC()
}

// in is the wall clock of s in the zone of series: an exception written in
// UTC beside a series in a zone names an occurrence by its instant.
func (s stamp) in(series stamp) time.Time {
	if s.day || s.loc == nil || series.loc == nil || series.day {
		return s.wall
	}
	t := s.instant().In(series.loc)
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
}

// write is wall written the way s is: a day, a UTC time, a time in the zone
// of s or a floating one.
func (s stamp) write(wall time.Time) (text string, params map[string]string) {
	switch {
	case s.day:
		return wall.Format("20060102"), map[string]string{"VALUE": "DATE"}
	case s.utc:
		return wall.Format("20060102T150405Z"), nil
	case s.zone != "":
		return wall.Format("20060102T150405"), map[string]string{"TZID": s.zone}
	}
	return wall.Format("20060102T150405"), nil
}

// retimer moves the start of an exception with its series: the clock it had
// in the series' old zone, shifted as the series' start was, written the way
// the new start is.
type retimer struct{ was, now stamp }

func (r retimer) move(p *ical.Prop) int {
	n := 0
	var out []string
	var params map[string]string
	for _, one := range strings.Split(p.Value, ",") {
		tz := ""
		if v := p.Params.Get(ical.ParamTimezoneID); v != "" {
			tz = v
		}
		s, ok := readStamp(one, map[string]string{"TZID": tz})
		if !ok {
			return 0
		}
		wall := s.in(r.was).Add(r.now.wall.Sub(r.was.wall))
		if r.now.day {
			wall = time.Date(wall.Year(), wall.Month(), wall.Day(), 0, 0, 0, 0, time.UTC)
		}
		var text string
		text, params = r.now.write(wall)
		out = append(out, text)
		n++
	}
	p.Value = strings.Join(out, ",")
	p.Params.Del(ical.ParamTimezoneID)
	p.Params.Del(ical.ParamValue)
	for k, v := range params {
		p.Params.Set(k, v)
	}
	return n
}

// RetimeExceptions takes the exceptions of a series to the start it was just
// given. obj is the record with its new DTSTART already patched, was — the
// DTSTART it had. Every EXDATE and RDATE of the series and the RECURRENCE-ID
// of every occurrence changed apart is moved as the start was and written
// the way the start now is; the times an occurrence was moved to are the person's own
// and stay. Like SetAlarms it is a document edit. n is how many exceptions
// were moved; 0 hands obj back — the start did not change, or there is
// nothing to move.
func RetimeExceptions(obj *model.Object, was model.RawValue) (*model.Object, int, error) {
	if _, err := mainComponent(obj); err != nil {
		return nil, 0, err
	}
	cur := obj.Props()["DTSTART"]
	if len(cur) == 0 {
		return obj, 0, nil
	}
	old, ok1 := readStamp(was.Text, was.Params)
	now, ok2 := readStamp(cur[0].Text, cur[0].Params)
	if !ok1 || !ok2 || (old.wall.Equal(now.wall) && old.day == now.day && old.utc == now.utc && old.zone == now.zone) {
		return obj, 0, nil
	}
	doc, err := decode(obj.CalendarDoc())
	if err != nil {
		return nil, 0, err
	}
	main := findMain(doc)
	main.Props = rawToProps(obj.Props())
	uid := propText(main.Props, ical.PropUID)
	r := retimer{was: old, now: now}
	n := 0
	for i := range main.Props[ical.PropExceptionDates] {
		n += r.move(&main.Props[ical.PropExceptionDates][i])
	}
	// A day added to the series is written by the series' clock as a day
	// left out is, and goes with it the same way (Б24 of the audit of
	// 11.10.2026); a period (VALUE=PERIOD) is not a start and stays.
	for i := range main.Props[ical.PropRecurrenceDates] {
		n += r.move(&main.Props[ical.PropRecurrenceDates][i])
	}
	for _, c := range doc.Children {
		if c == main || c.Name != main.Name {
			continue
		}
		rid := c.Props.Get(ical.PropRecurrenceID)
		if rid == nil {
			continue
		}
		if u := propText(c.Props, ical.PropUID); u != "" && u != uid {
			continue
		}
		n += r.move(rid)
	}
	if n == 0 {
		return obj, 0, nil
	}
	var buf bytes.Buffer
	if err := ical.NewEncoder(&buf).Encode(doc); err != nil {
		return nil, 0, err
	}
	out, err := Parse(obj.Path, obj.ETag, bytes.NewReader(model.FoldLines(buf.Bytes())))
	if err != nil {
		return nil, 0, err
	}
	return out, n, nil
}

// LostDays are the exceptions of sent that got no longer has, each by its
// day (Typecase's Р-60, states-36): a day left out that came back, and an
// occurrence changed apart that is gone or no longer tied to its day — with
// where it had been moved, when its start was another. In the order of the
// series.
func LostDays(sent, got *model.Object) []model.LostDay {
	sd, sm := exceptionStamps(sent)
	gd, gm := exceptionStamps(got)
	var out []model.LostDay
	for at, s := range sd {
		if _, ok := gd[at]; !ok {
			out = append(out, model.LostDay{At: at, Day: s.at.day})
		}
	}
	for at, m := range sm {
		if _, ok := gm[at]; ok {
			continue
		}
		d := model.LostDay{At: at, Day: m.at.day, Moved: true}
		if m.moved && m.to.instant() != at {
			d.To, d.ToDay = m.to.instant(), m.to.day
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out
}

// exception is an exception of a series: the start it names, and for an
// occurrence changed apart the start it has now (to; moved — it has one).
type exception struct {
	at, to stamp
	moved  bool
}

// exceptionStamps are a record's exceptions by the instant each names: the
// days left out and the occurrences changed apart.
func exceptionStamps(obj *model.Object) (days, moved map[time.Time]exception) {
	days, moved = map[time.Time]exception{}, map[time.Time]exception{}
	if obj == nil || !obj.IsCalendar() {
		return
	}
	doc, err := decode(obj.CalendarDoc())
	if err != nil {
		return
	}
	main := findMain(doc)
	if main == nil {
		return
	}
	stamps := func(p *ical.Prop) []stamp {
		var out []stamp
		for _, one := range strings.Split(p.Value, ",") {
			if s, ok := readStamp(one, map[string]string{"TZID": p.Params.Get(ical.ParamTimezoneID)}); ok {
				out = append(out, s)
			}
		}
		return out
	}
	for i := range main.Props[ical.PropExceptionDates] {
		for _, s := range stamps(&main.Props[ical.PropExceptionDates][i]) {
			days[s.instant()] = exception{at: s}
		}
	}
	for _, c := range doc.Children {
		if c == main || c.Name != main.Name {
			continue
		}
		rid := c.Props.Get(ical.PropRecurrenceID)
		if rid == nil {
			continue
		}
		for _, s := range stamps(rid) {
			e := exception{at: s}
			if start := c.Props.Get(ical.PropDateTimeStart); start != nil {
				if to := stamps(start); len(to) == 1 {
					e.to, e.moved = to[0], true
				}
			}
			moved[s.instant()] = e
		}
	}
	return
}

// MissingExceptions counts the exceptions of sent that got — the record as
// its server keeps it after the write — no longer has: days left out and
// occurrences changed apart, each known by the instant it names, however the
// server wrote it. A changed occurrence the server kept without the
// RECURRENCE-ID that ties it to its series is missing: nothing says any more
// which day it stands for.
func MissingExceptions(sent, got *model.Object) (days, moved int) {
	for _, d := range LostDays(sent, got) {
		if d.Moved {
			moved++
		} else {
			days++
		}
	}
	return
}

// ExceptionLoss corrects the list of properties a server did not keep
// (model.CompareAfterPut, which reads the series alone and compares what is
// written) by what became of the exceptions: «EXDATE» stands in it when a day
// left out is no longer left out, and only then — the same days written in
// UTC are the same days —, and «RECURRENCE-ID» when an occurrence changed
// apart is gone or no longer tied to its day. The order of the rest is kept.
func ExceptionLoss(lost []string, sent, got *model.Object) []string {
	days, moved := MissingExceptions(sent, got)
	var out []string
	for _, p := range lost {
		if p != ical.PropExceptionDates && p != ical.PropRecurrenceID {
			out = append(out, p)
		}
	}
	if days > 0 {
		out = append(out, ical.PropExceptionDates)
	}
	if moved > 0 {
		out = append(out, ical.PropRecurrenceID)
	}
	return out
}
