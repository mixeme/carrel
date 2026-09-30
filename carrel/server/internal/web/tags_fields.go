package web

import (
	"strings"
	"time"

	"carrel.dev/carrel/server/view"
)

// A record's categories are one row of its fields, «Tags», with the tags
// for a value (C-6 Field.Tags): the frames draw them where they draw the
// other rows — a contact's between Birthday and Note, the last row of the
// other kinds (s-contacts-6, s-agenda-2, s-tasks-2, the panels).

// plainTags are tags without a way to a list: a public page, a member of a
// linked group.
func plainTags(labels []string) []view.Tag {
	out := make([]view.Tag, 0, len(labels))
	for _, l := range labels {
		out = append(out, view.Tag{Label: text(l)})
	}
	return out
}

// tagsField is the row of tags, or none.
func tagsField(tags []view.Tag) []view.Field {
	if len(tags) == 0 {
		return nil
	}
	return []view.Field{{Label: "Tags", Tags: tags}}
}

// withTagLinks puts the tags that lead to the list filtered by them on the
// row of tags.
func withTagLinks(fields []view.Field, tags []view.Tag) []view.Field {
	for i := range fields {
		if fields[i].Label == "Tags" && len(fields[i].Tags) > 0 {
			fields[i].Tags = tags
		}
	}
	return fields
}

// withBirthdayAside puts by the birthday how far the next one is — «in 7
// months», «in 12 days», «tomorrow», «today» (s-contacts-6, -7).
func withBirthdayAside(fields []view.Field, tf timeFmt) []view.Field {
	for i := range fields {
		if fields[i].Label == "Birthday" {
			fields[i].Aside = text(birthdayIn(string(fields[i].Value), tf))
		}
	}
	return fields
}

func birthdayIn(shown string, tf timeFmt) string {
	v := strings.TrimPrefix(shown, "--")
	if len(v) == 10 {
		v = v[5:]
	}
	day, err := time.Parse("01-02", v)
	if err != nil {
		return ""
	}
	now := tf.now.In(tf.tz)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, tf.tz)
	next := time.Date(today.Year(), day.Month(), day.Day(), 0, 0, 0, 0, tf.tz)
	if next.Before(today) {
		next = next.AddDate(1, 0, 0)
	}
	days := int(next.Sub(today).Hours()/24 + 0.5)
	switch {
	case days == 0:
		return "today"
	case days == 1:
		return "tomorrow"
	case days < 30:
		return "in " + plural(days, "day")
	}
	months := (next.Year()-today.Year())*12 + int(next.Month()-today.Month())
	if next.Day() < today.Day() {
		months--
	}
	return "in " + plural(months, "month")
}
