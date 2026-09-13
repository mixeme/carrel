package web

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/teambition/rrule-go"

	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// timeFmt is C-10 for one person: one instance zone for every time shown, and
// from their account the first day of the week and 12 or 24 hours (unset:
// Monday and 24 hours). Lists show absolute times; the original zone shows
// only where it differs.
type timeFmt struct {
	tz   *time.Location
	week time.Weekday
	h12  bool
}

// timeFor reads the account settings of the session's person. Build it once
// per page: it takes a snapshot of the state.
func (s *Server) timeFor(sess *session.Session) timeFmt {
	f := timeFmt{tz: s.tz(), week: time.Monday}
	if sess == nil {
		return f
	}
	snap := s.store.Snapshot()
	if u := snap.UserByID(sess.AccountID()); u != nil {
		if d, ok := user.WeekStarts[u.WeekStart]; ok {
			f.week = d
		}
		f.h12 = u.Clock12
	}
	return f
}

// weekStart is midnight of the first day of the week holding t, in t's zone.
func (f timeFmt) weekStart(t time.Time) time.Time {
	back := (int(t.Weekday()) - int(f.week) + 7) % 7
	y, m, d := t.Date()
	return time.Date(y, m, d-back, 0, 0, 0, 0, t.Location())
}

func (f timeFmt) date(t time.Time) string {
	return t.In(f.tz).Format("2 January 2006")
}

func (f timeFmt) dayRubric(t time.Time) string {
	return t.In(f.tz).Format("Monday, 2 January")
}

func (f timeFmt) dayKey(t time.Time) string {
	return t.In(f.tz).Format("2006-01-02")
}

func (f timeFmt) clock(t time.Time) string {
	if f.h12 {
		return t.In(f.tz).Format("3:04 PM")
	}
	return t.In(f.tz).Format("15:04")
}

// timeLabel is the time column of an agenda row.
func (f timeFmt) timeLabel(t time.Time, allDay bool) string {
	if allDay {
		return "All day"
	}
	return f.clock(t)
}

// timeOrDate is a due date: the day, and the time when there is one.
func (f timeFmt) timeOrDate(tm cal.Timing) string {
	if tm.AllDay {
		return f.date(tm.Start)
	}
	return f.date(tm.Start) + ", " + f.clock(tm.Start)
}

// when is the When field of a record.
func (f timeFmt) when(tm cal.Timing) string {
	const long = "Monday, 2 January 2006"
	if tm.AllDay {
		first := tm.Start.In(f.tz)
		if !tm.End.IsZero() {
			// DTEND of a day is exclusive: the last day is the one before.
			if last := tm.End.In(f.tz).AddDate(0, 0, -1); last.After(first) {
				return first.Format(long) + " – " + last.Format(long) + ", all day"
			}
		}
		return first.Format(long) + ", all day"
	}
	start := tm.Start.In(f.tz)
	label := start.Format(long) + ", " + f.clock(start)
	if tm.End.IsZero() {
		return label
	}
	end := tm.End.In(f.tz)
	if f.dayKey(start) == f.dayKey(end) {
		return label + " – " + f.clock(end)
	}
	return label + " – " + end.Format(long) + ", " + f.clock(end)
}

// zone is the original zone when it differs from the instance zone, and empty
// otherwise (C-10).
func (f timeFmt) zone(zone string) string {
	if zone == "" {
		return ""
	}
	if loc, err := time.LoadLocation(zone); err == nil && loc.String() == f.tz.String() {
		return ""
	}
	return zone
}

// formatWhen is a When: the day and time, or "2 min ago" where the frame draws
// relative time.
func (f timeFmt) formatWhen(t time.Time, relative bool, now time.Time) view.When {
	if t.IsZero() {
		return ""
	}
	if relative {
		d := now.Sub(t)
		switch {
		case d < time.Minute:
			return "just now"
		case d < time.Hour:
			m := int(d.Minutes())
			if m == 1 {
				return "1 min ago"
			}
			return view.When(fmt.Sprintf("%d min ago", m))
		case d < 24*time.Hour:
			h := int(d.Hours())
			if h == 1 {
				return "1 hour ago"
			}
			return view.When(fmt.Sprintf("%d hours ago", h))
		}
	}
	return view.When(t.In(f.tz).Format("2 January") + ", " + f.clock(t))
}

func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return fmt.Sprintf("%d %ss", n, unit)
}

func humanDuration(d time.Duration) string {
	const day = 24 * time.Hour
	switch {
	case d%(7*day) == 0:
		return plural(int(d/(7*day)), "week")
	case d%day == 0:
		return plural(int(d/day), "day")
	}
	h, m := int(d/time.Hour), int((d%time.Hour)/time.Minute)
	switch {
	case h > 0 && m > 0:
		return plural(h, "hour") + " " + plural(m, "minute")
	case h > 0:
		return plural(h, "hour")
	default:
		return plural(m, "minute")
	}
}

// reminder says when a reminder fires, in words.
func (f timeFmt) reminder(a cal.Alarm) string {
	if a.Absolute {
		if t, err := time.Parse("20060102T150405Z", a.Trigger); err == nil {
			return "At " + f.date(t) + ", " + f.clock(t)
		}
		return a.Trigger
	}
	d, ok := a.Offset()
	if !ok {
		return a.Trigger
	}
	anchor := "the start"
	if a.RelatedEnd {
		anchor = "the end"
	}
	switch {
	case d == 0:
		return "At " + anchor
	case d < 0 && !a.RelatedEnd:
		return humanDuration(-d) + " before"
	case d < 0:
		return humanDuration(-d) + " before " + anchor
	default:
		return humanDuration(d) + " after " + anchor
	}
}

var weekdayNames = map[string]string{
	"MO": "Monday", "TU": "Tuesday", "WE": "Wednesday", "TH": "Thursday",
	"FR": "Friday", "SA": "Saturday", "SU": "Sunday",
}

// weekdayPhrase reads rrule-go's weekday form: "TH", "+2TH", "-1FR".
func weekdayPhrase(s string) string {
	if len(s) < 2 {
		return s
	}
	name := weekdayNames[s[len(s)-2:]]
	if len(s) == 2 {
		return name
	}
	n, err := strconv.Atoi(s[:len(s)-2])
	if err != nil {
		return name
	}
	return ordinal(n) + " " + name
}

func ordinal(n int) string {
	switch n {
	case 1:
		return "first"
	case 2:
		return "second"
	case 3:
		return "third"
	case 4:
		return "fourth"
	case 5:
		return "fifth"
	case -1:
		return "last"
	case -2:
		return "second to last"
	default:
		return strconv.Itoa(n) + "."
	}
}

// recurrence is RecurrenceBlock.Summary: the rule in words. What the words do
// not cover is admitted, and the raw rule stands beside it.
func (f timeFmt) recurrence(raw string) string {
	if raw == "" {
		return "On extra dates"
	}
	opt, err := rrule.StrToROption(raw)
	if err != nil {
		return "Custom rule"
	}
	units := map[rrule.Frequency]string{
		rrule.YEARLY: "year", rrule.MONTHLY: "month", rrule.WEEKLY: "week", rrule.DAILY: "day",
		rrule.HOURLY: "hour", rrule.MINUTELY: "minute", rrule.SECONDLY: "second",
	}
	plain := map[rrule.Frequency]string{
		rrule.YEARLY: "Yearly", rrule.MONTHLY: "Monthly", rrule.WEEKLY: "Weekly", rrule.DAILY: "Daily", rrule.HOURLY: "Hourly",
	}
	var b strings.Builder
	if word, ok := plain[opt.Freq]; ok && opt.Interval <= 1 {
		b.WriteString(word)
	} else if opt.Interval <= 1 {
		b.WriteString("Every " + units[opt.Freq])
	} else {
		fmt.Fprintf(&b, "Every %d %ss", opt.Interval, units[opt.Freq])
	}
	if len(opt.Byweekday) > 0 {
		days := make([]string, 0, len(opt.Byweekday))
		for _, d := range opt.Byweekday {
			days = append(days, weekdayPhrase(d.String()))
		}
		b.WriteString(" on " + strings.Join(days, ", "))
	}
	if len(opt.Bymonthday) > 0 {
		days := make([]string, 0, len(opt.Bymonthday))
		for _, d := range opt.Bymonthday {
			days = append(days, strconv.Itoa(d))
		}
		b.WriteString(" on day " + strings.Join(days, ", "))
	}
	if len(opt.Bymonth) > 0 {
		months := make([]string, 0, len(opt.Bymonth))
		for _, m := range opt.Bymonth {
			if m >= 1 && m <= 12 {
				months = append(months, time.Month(m).String())
			}
		}
		b.WriteString(" in " + strings.Join(months, ", "))
	}
	switch {
	case opt.Count == 1:
		b.WriteString(", once")
	case opt.Count > 1:
		fmt.Fprintf(&b, ", %d times", opt.Count)
	}
	if !opt.Until.IsZero() {
		b.WriteString(", until " + f.date(opt.Until))
	}
	if len(opt.Bysetpos)+len(opt.Byyearday)+len(opt.Byweekno)+len(opt.Byhour)+len(opt.Byminute)+len(opt.Bysecond)+len(opt.Byeaster) > 0 {
		b.WriteString(", with further conditions")
	}
	return b.String()
}
