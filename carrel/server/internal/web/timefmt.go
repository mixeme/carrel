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
	// now is what this answer calls "now": «2 min ago», the week it opens
	// on, the stamp under a printed page. A record's own dates are the
	// record's and never come from here.
	now  time.Time
	tz   *time.Location
	week time.Weekday
	h12  bool
}

// timeFor reads the account settings of the session's person. Build it once
// per page: it takes a snapshot of the state.
func (s *Server) timeFor(sess *session.Session) timeFmt {
	f := timeFmt{tz: s.tz(), week: time.Monday, now: s.now()}
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

// numDate and numStamp are a day and a minute in numbers, as the tables of
// files and of links draw them monospaced (s-files-1, s-publish-1): «2026-
// 08-10», «2026-08-10 08:41». The owner's word of 09.10.2026: as the frame
// draws them, for now — C-10 says dates in words everywhere else.
func (f timeFmt) numDate(t time.Time) string { return t.In(f.tz).Format("2006-01-02") }

func (f timeFmt) numStamp(t time.Time) view.When {
	if t.IsZero() {
		return ""
	}
	return view.When(t.In(f.tz).Format("2006-01-02 15:04"))
}

// dayOfYear is a day without its year when it is this year's: «14
// February» (account-8), «14 February 2025» otherwise.
func (f timeFmt) dayOfYear(t time.Time) string {
	if t.In(f.tz).Year() == f.now.In(f.tz).Year() {
		return t.In(f.tz).Format("2 January")
	}
	return f.date(t)
}

// shortDay is a day in a list (C-10): the columns of a row and the list of
// a field say it the short way, «12 Aug» (s-tasks-10), with the year only
// when it is not this year's, «12 Aug 2025»; the record itself says it in
// full.
func (f timeFmt) shortDay(t time.Time) string {
	if t.In(f.tz).Year() == f.now.In(f.tz).Year() {
		return t.In(f.tz).Format("2 Jan")
	}
	return t.In(f.tz).Format("2 Jan 2006")
}

// shortStamp is a moment in a row's column: the short day and the time,
// «10 Aug 11:02» — when a record last changed (s-tasks-13, s-notes-24).
func (f timeFmt) shortStamp(t time.Time) string {
	return f.shortDay(t) + " " + f.clock(t)
}

// shortDue is a due date in a row's column: the short day, and the time
// after it when there is one, «12 Aug 18:00» (s-tasks-1).
func (f timeFmt) shortDue(tm cal.Timing) string {
	if tm.AllDay {
		return f.shortDay(tm.Start)
	}
	return f.shortDay(tm.Start) + " " + f.clock(tm.Start)
}

// sentAt is when a letter went: the clock alone today («11:42», account-9),
// the day and the clock before. The phrase is how a sentence says it: «at
// 11:42», «on 12 February at 11:42».
func (f timeFmt) sentAt(t time.Time) (view.When, string) {
	if f.dayKey(t) == f.dayKey(f.now) {
		return view.When(f.clock(t)), "at " + f.clock(t)
	}
	return view.When(f.dayOfYear(t) + ", " + f.clock(t)), "on " + f.dayOfYear(t) + " at " + f.clock(t)
}

// dayRubric is the rubric of a day of the agenda: «Monday · 10 August»
// (s-agenda-1, C-10).
func (f timeFmt) dayRubric(t time.Time) string {
	return t.In(f.tz).Format("Monday · 2 January")
}

// window is the days of an agenda's window in one phrase, as the line under
// its title says it: «10 – 16 August 2026», «28 September – 4 October 2026»,
// «28 December 2026 – 3 January 2027», one day «10 August 2026» (s-agenda-1).
// to is the day after the last.
func (f timeFmt) window(from, to time.Time) string {
	a, b := from.In(f.tz), to.In(f.tz).AddDate(0, 0, -1)
	switch {
	case f.dayKey(a) == f.dayKey(b) || b.Before(a):
		return a.Format("2 January 2006")
	case a.Year() != b.Year():
		return a.Format("2 January 2006") + " – " + b.Format("2 January 2006")
	case a.Month() != b.Month():
		return a.Format("2 January") + " – " + b.Format("2 January 2006")
	}
	return a.Format("2") + " – " + b.Format("2 January 2006")
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

// timeLabel is the time column of an agenda row: the clock, or «all day»
// (s-agenda-1).
func (f timeFmt) timeLabel(t time.Time, allDay bool) string {
	if allDay {
		return "all day"
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

// historyWhen is when a search of the history was made (s-find-2, Пк20):
// today as a relative time, «2 hours ago»; yesterday — «yesterday»; within
// the week — its day, «Monday»; before — the day, «9 August».
func (f timeFmt) historyWhen(t, now time.Time) view.When {
	if t.IsZero() {
		return ""
	}
	day := func(x time.Time) time.Time {
		x = x.In(f.tz)
		return time.Date(x.Year(), x.Month(), x.Day(), 0, 0, 0, 0, f.tz)
	}
	days := int(day(now).Sub(day(t)).Hours()/24 + 0.5)
	switch {
	case days <= 0:
		return f.formatWhen(t, true, now)
	case days == 1:
		return "yesterday"
	case days < 7:
		return view.When(t.In(f.tz).Format("Monday"))
	case t.In(f.tz).Year() == now.In(f.tz).Year():
		return view.When(t.In(f.tz).Format("2 January"))
	}
	return view.When(f.date(t))
}

// ago is a past moment the way a head says it: «just now», «5 min ago»,
// «2 hours ago», «2 days ago» up to a month, then the day.
func (f timeFmt) ago(t, now time.Time) string {
	if d := now.Sub(t); d >= 24*time.Hour {
		if days := int(d / (24 * time.Hour)); days < 31 {
			return plural(days, "day") + " ago"
		}
		return f.date(t)
	}
	return string(f.formatWhen(t, true, now))
}

func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return fmt.Sprintf("%d %ss", n, unit)
}

// verbs is the verb that agrees with a count: one for 1, many for the rest.
func verbs(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
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
// not cover is admitted, and the raw rule stands beside it. Until is the last
// day an occurrence can start on (ruleUntil); start is DTSTART.
func (f timeFmt) recurrence(raw string, start time.Time) string {
	words, ok := f.ruleWords(raw, false)
	if !ok {
		return words
	}
	opt, _ := rrule.StrToROption(raw)
	switch {
	case opt.Count == 1:
		words += ", once"
	case opt.Count > 1:
		words += fmt.Sprintf(", %d times", opt.Count)
	}
	if day, ok := ruleUntil(raw, start, f.tz); ok {
		words += ", until " + day.Format("2 January 2006")
	} else if !opt.Until.IsZero() {
		words += ", until " + f.date(opt.Until)
	}
	return words + ruleMore(opt)
}

// repeats is the row Repeats of an event's fields (s-agenda-1, s-agenda-2):
// the rule in short words — «Weekly on Mon» —, and its end as a badge,
// «until 2026-12-28» (the last day it can start on, ruleUntil), «5 times»,
// «once»; no badge for a series without an end.
func (f timeFmt) repeats(raw string, start time.Time) (string, view.Badge) {
	words, ok := f.ruleWords(raw, true)
	if !ok {
		return words, view.Badge{}
	}
	opt, _ := rrule.StrToROption(raw)
	words += ruleMore(opt)
	var end string
	switch {
	case opt.Count == 1:
		end = "once"
	case opt.Count > 1:
		end = fmt.Sprintf("%d times", opt.Count)
	}
	if day, ok := ruleUntil(raw, start, f.tz); ok {
		end = "until " + day.Format("2006-01-02")
	}
	if end == "" {
		return words, view.Badge{}
	}
	return words, view.Badge{Label: text(end), Role: view.BadgeNeutral}
}

// ruleWords is a rule's frequency and its days in words, without its end;
// short writes the days as the frames abbreviate them, «Mon». ok is false
// when the words are all there is to say: no rule, or one RFC 5545 does not
// read.
func (f timeFmt) ruleWords(raw string, short bool) (string, bool) {
	if raw == "" {
		return "On extra dates", false
	}
	opt, err := rrule.StrToROption(raw)
	if err != nil {
		return "Custom rule", false
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
			day := weekdayPhrase(d.String())
			if short {
				day = shortWeekday(day)
			}
			days = append(days, day)
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
	return b.String(), true
}

// ruleMore admits the parts of a rule the words do not say.
func ruleMore(opt *rrule.ROption) string {
	if opt == nil {
		return ""
	}
	if len(opt.Bysetpos)+len(opt.Byyearday)+len(opt.Byweekno)+len(opt.Byhour)+len(opt.Byminute)+len(opt.Bysecond)+len(opt.Byeaster) > 0 {
		return ", with further conditions"
	}
	return ""
}

// shortWeekday is a day's name as the frames abbreviate it, «Mon»; an
// ordinal before it stays, «first Mon».
func shortWeekday(phrase string) string {
	for _, name := range weekdayNames {
		if strings.HasSuffix(phrase, name) {
			return strings.TrimSuffix(phrase, name) + name[:3]
		}
	}
	return phrase
}

// whenRuns is the When of an event as the frames write it (s-agenda-1,
// s-agenda-2): the clock monospaced, then the day — «‹10:00 – 11:00›, Monday
// 10 August»; the year only when it is not this year's. A day is «all day,
// Monday 10 August».
func (f timeFmt) whenRuns(tm cal.Timing) view.Runs {
	day := func(t time.Time) string {
		t = t.In(f.tz)
		if t.Year() == f.now.In(f.tz).Year() {
			return t.Format("Monday 2 January")
		}
		return t.Format("Monday 2 January 2006")
	}
	mono := func(s string) view.Run { return view.Run{Text: text(s), Mark: view.RunMono} }
	plain := func(s string) view.Run { return view.Run{Text: text(s)} }
	if tm.AllDay {
		first := tm.Start.In(f.tz)
		words := day(first)
		if !tm.End.IsZero() {
			// DTEND of a day is exclusive: the last day is the one before.
			if last := tm.End.In(f.tz).AddDate(0, 0, -1); last.After(first) {
				words += " – " + day(last)
			}
		}
		return view.Runs{plain("all day, " + words)}
	}
	start, end := tm.Start, tm.End
	switch {
	case end.IsZero():
		return view.Runs{mono(f.clock(start)), plain(", " + day(start))}
	case f.dayKey(start) == f.dayKey(end):
		return view.Runs{mono(f.clock(start) + " – " + f.clock(end)), plain(", " + day(start))}
	}
	return view.Runs{mono(f.clock(start)), plain(", " + day(start) + " – "), mono(f.clock(end)), plain(", " + day(end))}
}

// createdIn is the badge by the When of a time made in another zone
// (s-agenda-1, s-agenda-2): «created 12:00 Asia/Yekaterinburg», the start's
// clock there. None for the instance zone, a day or a floating time.
func (f timeFmt) createdIn(tm cal.Timing) view.Badge {
	zone := f.zone(tm.Zone)
	if zone == "" || tm.AllDay {
		return view.Badge{}
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return view.Badge{Label: text("created in " + zone), Role: view.BadgeNeutral}
	}
	layout := "15:04"
	if f.h12 {
		layout = "3:04 PM"
	}
	return view.Badge{Label: text("created " + tm.Start.In(loc).Format(layout) + " " + zone), Role: view.BadgeNeutral}
}
