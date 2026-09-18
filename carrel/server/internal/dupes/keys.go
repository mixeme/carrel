package dupes

import (
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/view"
)

// KeysOf folds a record into what it is compared by (S-16). Contacts: every
// email, every phone, the name, the birthday. Events: title and start —
// the instant, or the day of an all-day event. Tasks: title and the day
// they are due. Notes: title and day, never the body. Days are read in loc,
// the instance zone (C-10).
func KeysOf(k view.Kind, obj *model.Object, loc *time.Location) Keys {
	var keys Keys
	if obj == nil {
		return keys
	}
	if k == view.KindContact {
		for _, v := range obj.PropText("EMAIL") {
			if e := NormEmail(v); e != "" && !contains(keys.Emails, e) {
				keys.Emails = append(keys.Emails, e)
			}
		}
		for _, v := range obj.PropText("TEL") {
			if p := NormPhone(v); p != "" && !contains(keys.Phones, p) {
				keys.Phones = append(keys.Phones, p)
			}
		}
		keys.Name = FoldName(contactName(obj))
		if b := obj.PropText("BDAY"); len(b) > 0 {
			keys.Birthday = NormBirthday(b[0])
		}
		return keys
	}
	switch k {
	case view.KindEvent:
		keys.Title = Fold(cal.Text(obj, "SUMMARY"))
		if tm, err := cal.TimingOf(obj, loc); err == nil && !tm.Start.IsZero() {
			if tm.AllDay {
				keys.When = "d" + tm.Start.Format("2006-01-02")
			} else {
				keys.When = tm.Start.UTC().Format("20060102T150405Z")
			}
		}
	case view.KindTask:
		keys.Title = Fold(cal.Text(obj, "SUMMARY"))
		if due, err := cal.DueOf(obj, loc); err == nil && !due.Start.IsZero() {
			keys.When = "d" + due.Start.In(loc).Format("2006-01-02")
		}
	default:
		keys.Title = Fold(noteTitle(obj))
		for _, prop := range []string{"DTSTART", "LAST-MODIFIED", "DTSTAMP"} {
			if t, ok := cal.TimeOf(obj, prop, loc); ok {
				keys.When = "d" + t.In(loc).Format("2006-01-02")
				break
			}
		}
	}
	return keys
}

// contactName is FN, else the given and family names of N.
func contactName(obj *model.Object) string {
	if fn := obj.PropText("FN"); len(fn) > 0 && strings.TrimSpace(fn[0]) != "" {
		return fn[0]
	}
	if n := obj.PropText("N"); len(n) > 0 {
		parts := strings.Split(n[0], ";")
		if len(parts) > 1 {
			return parts[1] + " " + parts[0]
		}
		return parts[0]
	}
	return ""
}

// noteTitle is the note's SUMMARY, else the first line of its body — what the
// list shows as its title.
func noteTitle(obj *model.Object) string {
	if s := cal.Text(obj, "SUMMARY"); strings.TrimSpace(s) != "" {
		return s
	}
	for _, line := range strings.Split(cal.Description(obj), "\n") {
		if line = strings.TrimSpace(strings.TrimLeft(line, "# ")); line != "" {
			return line
		}
	}
	return ""
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
