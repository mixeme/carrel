package web

import (
	"slices"
	"strings"

	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// What a guest of a published calendar, event, task list or task sees is
// decided when the link is made, as for a contact (Б39 of the audit of
// 11.10.2026; the owner's decision of 12.10.2026): the time always, and of
// the rest only what is ticked — the title by default. The page, the rows of
// a calendar and of a list, and the file are all made of a record built of
// what is allowed (cal.Visitor). A link made before the rule shows what it
// did: everything but, when it was asked to, the descriptions.

// calendarSeesTime is the tick that stands for the time — always on — and
// marks a link made under the rule.
const calendarSeesTime = "time"

// calendarVisitorFields are the ticks of a calendar's, an event's or a
// task's link, by name: the label of the sheet, the properties they let
// through, and whether the sheet ticks it before anyone does.
var calendarVisitorFields = []struct {
	name, label string
	props       []string
	on          bool
}{
	{"title", "Title", []string{"SUMMARY"}, true},
	{"location", "Location", []string{"LOCATION", "GEO"}, false},
	{"description", "Description", []string{"DESCRIPTION"}, false},
	{"attendees", "Attendees", []string{"ATTENDEE", "ORGANIZER"}, false},
	{"attachments", "Attachments", []string{"ATTACH"}, false},
	{"tags", "Tags", []string{"CATEGORIES"}, false},
}

// calendarVisitorAlways are what every such link gives: what the record is,
// when it is, and — for a task — whether it is done.
var calendarVisitorAlways = []string{
	"UID", "DTSTAMP", "DTSTART", "DTEND", "DURATION", "DUE", "RRULE", "RDATE", "EXDATE", "RECURRENCE-ID",
	"SEQUENCE", "STATUS", "TRANSP", "COMPLETED", "PERCENT-COMPLETE",
}

// calendarUntitled is the title of a record whose title the link does not
// show: a time taken, not what it is.
const calendarUntitled = "Busy"

// picksCalendarFields says whether a link of a calendar kind was made under
// the rule of what the guest sees.
func picksCalendarFields(t user.PublishTarget) bool {
	return slices.Contains(t.Shows, calendarSeesTime)
}

// isCalendarKind says whether a kind's links are asked what the guest sees
// by the calendar's ticks: events and tasks.
func isCalendarKind(k view.Kind) bool {
	return k == view.KindEvent || k == view.KindTask
}

// visitorRecord is a record of a calendar kind as the guest of the link may
// have it; nil — it could not be built, and the guest gets nothing of it.
// A link made before the rule gets the record, its descriptions taken out
// when it was asked to.
func visitorRecord(o *model.Object, t user.PublishTarget) *model.Object {
	if o == nil {
		return nil
	}
	if !picksCalendarFields(t) {
		if t.HideDescriptions {
			if out := withoutDescriptions([]*model.Object{o}); len(out) == 1 {
				return out[0]
			}
			return nil
		}
		return o
	}
	allowed := map[string]bool{}
	for _, p := range calendarVisitorAlways {
		allowed[p] = true
	}
	for _, f := range calendarVisitorFields {
		if slices.Contains(t.Shows, f.name) {
			for _, p := range f.props {
				allowed[p] = true
			}
		}
	}
	out, err := cal.Visitor(o, func(p string) bool { return allowed[strings.ToUpper(p)] }, calendarUntitled)
	if err != nil {
		return nil
	}
	return out
}

// visitorRecords are the records the guest may have, each built so; one that
// could not be is left out.
func visitorRecords(objs []*model.Object, t user.PublishTarget) []*model.Object {
	out := make([]*model.Object, 0, len(objs))
	for _, o := range objs {
		if v := visitorRecord(o, t); v != nil {
			out = append(out, v)
		}
	}
	return out
}

// calendarSees is the set «What the visitor sees» of the sheet of a
// calendar, an event, a task list or a task: the time, which always stands
// and cannot be taken off — «always shown» beside it —, then the ticks, the
// title on, and under them what the visitor sees without the title
// (s-publish-2, -19). noun is what the visitor sees one of, «an event» of a
// calendar, «the event» of one.
func calendarSees(noun string) view.FieldSet {
	sees := view.FieldSet{Legend: "What the visitor sees", Hint: text("Without the title " + noun + " shows as “Busy”."), Rows: []view.FormRow{
		// Off with its reason (s-publish-2, -19: «Always shown»): it cannot
		// be taken off, and so it is not sent.
		{Name: calendarSeesTime, Label: "Time", LabelAside: "always shown", Kind: view.InputCheck, Value: "on", ReadOnly: true, Reason: "Always shown"}}}
	for _, f := range calendarVisitorFields {
		row := view.FormRow{Name: f.name, Label: text(f.label), Kind: view.InputCheck}
		if f.on {
			row.Value = "on"
		}
		sees.Rows = append(sees.Rows, row)
	}
	return sees
}
