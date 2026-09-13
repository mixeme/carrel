package web

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/teambition/rrule-go"

	"carrel.dev/carrel/server/internal/provider/cal"
)

// C-10 for records: one instance zone for every time shown, absolute times in
// lists, the original zone only where it differs. The account setting for the
// first day of the week and 12/24 hours does not exist yet; Monday and 24 hours
// stand in for it.

// weekStart is midnight of the Monday of the week holding t, in t's zone.
func weekStart(t time.Time) time.Time {
	back := (int(t.Weekday()) + 6) % 7
	y, m, d := t.Date()
	return time.Date(y, m, d-back, 0, 0, 0, 0, t.Location())
}

func dateLabel(t time.Time, tz *time.Location) string {
	return t.In(tz).Format("2 January 2006")
}

func dayRubric(t time.Time, tz *time.Location) string {
	return t.In(tz).Format("Monday, 2 January")
}

func dayKey(t time.Time, tz *time.Location) string {
	return t.In(tz).Format("2006-01-02")
}

func clockLabel(t time.Time, tz *time.Location) string {
	return t.In(tz).Format("15:04")
}

// timeLabel is the time column of an agenda row.
func timeLabel(t time.Time, allDay bool, tz *time.Location) string {
	if allDay {
		return "All day"
	}
	return clockLabel(t, tz)
}

// timeOrDate is a due date: the day, and the time when there is one.
func timeOrDate(tm cal.Timing, tz *time.Location) string {
	if tm.AllDay {
		return dateLabel(tm.Start, tz)
	}
	return dateLabel(tm.Start, tz) + ", " + clockLabel(tm.Start, tz)
}

// whenLabel is the When field of a record.
func whenLabel(tm cal.Timing, tz *time.Location) string {
	const long = "Monday, 2 January 2006"
	if tm.AllDay {
		first := tm.Start.In(tz)
		if !tm.End.IsZero() {
			// DTEND of a day is exclusive: the last day is the one before.
			if last := tm.End.In(tz).AddDate(0, 0, -1); last.After(first) {
				return first.Format(long) + " – " + last.Format(long) + ", all day"
			}
		}
		return first.Format(long) + ", all day"
	}
	start := tm.Start.In(tz)
	label := start.Format(long + ", 15:04")
	if tm.End.IsZero() {
		return label
	}
	end := tm.End.In(tz)
	if dayKey(start, tz) == dayKey(end, tz) {
		return label + " – " + end.Format("15:04")
	}
	return label + " – " + end.Format(long+", 15:04")
}

// zoneLabel is the original zone when it differs from the instance zone, and
// empty otherwise (C-10).
func zoneLabel(zone string, tz *time.Location) string {
	if zone == "" {
		return ""
	}
	if loc, err := time.LoadLocation(zone); err == nil && loc.String() == tz.String() {
		return ""
	}
	return zone
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

// reminderLabel says when a reminder fires, in words.
func reminderLabel(a cal.Alarm, tz *time.Location) string {
	if a.Absolute {
		if t, err := time.Parse("20060102T150405Z", a.Trigger); err == nil {
			return "At " + dateLabel(t, tz) + ", " + clockLabel(t, tz)
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

// recurrenceSummary is RecurrenceBlock.Summary: the rule in words. What the
// words do not cover is admitted, and the raw rule stands beside it.
func recurrenceSummary(raw string, tz *time.Location) string {
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
		b.WriteString(", until " + dateLabel(opt.Until, tz))
	}
	if len(opt.Bysetpos)+len(opt.Byyearday)+len(opt.Byweekno)+len(opt.Byhour)+len(opt.Byminute)+len(opt.Bysecond)+len(opt.Byeaster) > 0 {
		b.WriteString(", with further conditions")
	}
	return b.String()
}
