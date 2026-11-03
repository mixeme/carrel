package web

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/view"
)

// The days of a series a server did not keep are said as days (Typecase's
// Р-60, the owner's answer of 14.10.2026; states-36): each by the start it
// has in the series and what became of it — a day left out is back, an
// occurrence moved is back on its day and hour. EXDATE and RECURRENCE-ID are
// what the server lost, not what the person did, and the report does not
// name them. A changed occurrence the server untied from its day is, for
// the person, the same as a lost one.

// lossProperties are the properties a report names: without the exceptions
// of a series when its days are said as days.
func lossProperties(loss *model.Loss) []string {
	if len(loss.Days) == 0 {
		return loss.Properties
	}
	return slices.DeleteFunc(slices.Clone(loss.Properties), func(p string) bool {
		return p == "EXDATE" || p == "RECURRENCE-ID"
	})
}

// lossSaid is what a report outside the edit — a move, a merge, an import,
// a rewrite of an address — says a write lost, in one line: the properties,
// and the days of a series counted as the edit's message counts them («two
// days of the series»), not EXDATE and RECURRENCE-ID.
func lossSaid(loss *model.Loss) string {
	said := slices.Clone(lossProperties(loss))
	if n := len(loss.Days); n > 0 {
		said = append(said, countWords(n, "day")+" of the series")
	}
	return strings.Join(said, ", ")
}

// lostDaysNotice is the message of the days of a series a server did not
// keep (LossReport.Days); nil when it kept them all. also: the report says
// lost properties before it, and this message follows on from that one.
func lostDaysNotice(days []model.LostDay, kind view.Kind, tf timeFmt, also bool) *view.Notice {
	if len(days) == 0 {
		return nil
	}
	count := countWords(len(days), "day")
	title := "The change was saved, but the server did not keep " + count + " of the series."
	if also {
		title = "The server also did not keep " + count + " of the series."
	}
	noun := "meeting"
	if kind == view.KindTask {
		noun = "task"
	}
	items := make([]string, 0, len(days))
	for _, d := range days {
		of := "the " + noun + " of " + lostDate(tf, d.At, d.Day)
		switch {
		case !d.Moved:
			items = append(items, of+", which was skipped, is back")
		case d.To.IsZero():
			items = append(items, of+", which was changed on its own, is back as the series has it")
		default:
			back := "is back on its day"
			if !d.Day {
				back += " at " + tf.clock(d.At)
			}
			to := lostDate(tf, d.To, d.ToDay)
			if !d.ToDay {
				to += ", " + tf.clock(d.To)
			}
			items = append(items, of+", which was moved to "+to+", "+back)
		}
	}
	return &view.Notice{Role: view.NoticeInfo, Title: text(title),
		Body: text("Other clients will not see them either: " + strings.Join(items, "; ") + ".")}
}

// lostDate is a day of a series as the report says it: «24 August»; a whole
// day's date as written, a moment's in the person's zone.
func lostDate(tf timeFmt, t time.Time, day bool) string {
	if day {
		return t.UTC().Format("2 January")
	}
	return t.In(tf.tz).Format("2 January")
}

// countWords is a count of a noun in words up to ten — «two days» —, in
// figures after.
func countWords(n int, noun string) string {
	words := []string{"no", "one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten"}
	num := strconv.Itoa(n)
	if n < len(words) {
		num = words[n]
	}
	if n == 1 {
		return num + " " + noun
	}
	return num + " " + noun + "s"
}
