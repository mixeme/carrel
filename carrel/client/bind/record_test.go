package bind_test

import (
	"testing"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

func TestSub(t *testing.T) {
	r := view.Runs{{Text: "6 of 8 calendars · 10 – 16 August 2026 · "}, {Text: "Europe/Moscow", Mark: view.RunMono}}
	same(t, "the window and the zone", bind.Sub(r), []w.HeadSub{{Parts: []w.HeadSubPart{w.Text("6 of 8 calendars · 10 – 16 August 2026 · "), w.Mono{Text: "Europe/Moscow"}}}})
	same(t, "nothing", bind.Sub(nil), []w.HeadSub(nil))
}

func TestAttendee(t *testing.T) {
	known := view.AttendeeRow{Name: "Анна Ковалёва", Email: "a.kovaleva@example.org", Status: "accepted", Href: "/app/contacts/a/k/4f2b", Colour: "#4a6b52", Value: "mailto:a.kovaleva@example.org"}
	stranger := view.AttendeeRow{Name: "Виктор Гончаров", Email: "v@example.org", Status: "no reply", Value: "mailto:v@example.org"}
	same(t, "a person the books know", bind.Attendee(known, "· "), []w.SecItemPart{
		w.Mark{Colour: "#4a6b52", Size: w.MarkH11Inline}, w.Text(" "), w.Link{Text: "Анна Ковалёва", Href: "/app/contacts/a/k/4f2b"}, w.Text(" "), w.Dim{Parts: []w.Inline{w.Text("· accepted")}},
	})
	same(t, "a stranger: the address", bind.Attendee(stranger, "· "), []w.SecItemPart{
		w.Mono{Text: "v@example.org"}, w.Text(" "), w.Dim{Parts: []w.Inline{w.Text("· no reply")}},
	})
	organiser := view.AttendeeRow{Name: "Михаил", Email: "mix@example.org", Status: "organiser", Value: "mailto:mix@example.org", Fixed: true}
	field := view.AttendeeField{
		Name:    "attendee",
		Rows:    []view.AttendeeRow{organiser, known, stranger},
		Add:     &view.FormRow{Name: "attendee.add", Label: "Add attendee", Placeholder: "Name or address", Kind: view.InputText, Value: "Ве"},
		Suggest: "/app/contacts/suggest",
	}
	same(t, "the organiser's field", bind.Attendees(field, "· "), w.TagInput{
		Columns: true,
		Name:    "attendee",
		Parts: []w.TagInputPart{
			w.Tag{Address: &w.Mono{Text: "mix@example.org"}, Aside: &w.Dim{Parts: []w.Inline{w.Text("· organiser")}}, Title: "mix@example.org", Fixed: true, Value: "mailto:mix@example.org"},
			w.Tag{Label: "Анна Ковалёва", Mark: &w.Mark{Colour: "#4a6b52", Size: w.MarkH11}, Aside: &w.Dim{Parts: []w.Inline{w.Text("· accepted")}}, Title: "a.kovaleva@example.org", Removable: true, Value: "mailto:a.kovaleva@example.org"},
			w.Tag{Address: &w.Mono{Text: "v@example.org"}, Aside: &w.Dim{Parts: []w.Inline{w.Text("· no reply")}}, Title: "v@example.org", Removable: true, Value: "mailto:v@example.org"},
		},
		Add: &w.TagAdd{Name: "attendee.add", Placeholder: "Name or address", Label: "Add attendee", Value: "Ве", Suggest: "/app/contacts/suggest"},
	})
	// The invited (s-agenda-14): the tags without crosses, nothing sent, no
	// line.
	field.ReadOnly = true
	same(t, "the invited's field", bind.Attendees(field, "· "), w.TagInput{
		Columns: true,
		Parts: []w.TagInputPart{
			w.Tag{Address: &w.Mono{Text: "mix@example.org"}, Aside: &w.Dim{Parts: []w.Inline{w.Text("· organiser")}}, Title: "mix@example.org", Fixed: true},
			w.Tag{Label: "Анна Ковалёва", Mark: &w.Mark{Colour: "#4a6b52", Size: w.MarkH11}, Aside: &w.Dim{Parts: []w.Inline{w.Text("· accepted")}}, Title: "a.kovaleva@example.org", Fixed: true},
			w.Tag{Address: &w.Mono{Text: "v@example.org"}, Aside: &w.Dim{Parts: []w.Inline{w.Text("· no reply")}}, Title: "v@example.org", Fixed: true},
		},
	})
}

func TestCheckAndDays(t *testing.T) {
	same(t, "a tick", bind.Check(view.FormRow{Name: "start.allday", Label: "All day", Kind: view.InputCheck, Value: "on"}),
		w.Check{On: true, Name: "start.allday", Parts: []w.CheckPart{w.Text("All day")}})
	same(t, "a tick off", bind.Check(view.FormRow{Name: "start.allday", Label: "All day", Kind: view.InputCheck}).On, false)
	days := view.FormRow{Name: "repeat.on", Label: "On", Kind: view.InputCheck, Options: []view.Option{{Value: "MO", Label: "MO", Selected: true}, {Value: "TU", Label: "TU"}}}
	same(t, "the days", bind.Days(days), w.Seg{Name: "repeat.on", Multiple: true, Options: []w.SegOption{{Label: "MO", Value: "MO", On: true}, {Label: "TU", Value: "TU"}}})
}

func TestSubAlert(t *testing.T) {
	r := view.Runs{{Text: "nextcloud · Дела · "}, {Text: "overdue by 3 days", Mark: view.RunDanger}}
	same(t, "the alarm a badge", bind.SubAlert(r), []w.HeadSub{{Parts: []w.HeadSubPart{w.Text("nextcloud · Дела · "), w.Badge{Label: "overdue by 3 days", Tone: w.BadgeAlert}}}})
	same(t, "the alarm in words where Sub draws it", bind.Sub(r), []w.HeadSub{{Parts: []w.HeadSubPart{w.Text("nextcloud · Дела · "), w.Span{Face: w.FaceDanger, Text: "overdue by 3 days"}}}})
	same(t, "nothing", bind.SubAlert(nil), []w.HeadSub(nil))
}

func TestTick(t *testing.T) {
	done := view.Action{ID: "task.done", Label: "Mark done", Method: "POST", Enabled: true, Href: "/app/tasks/a/dela/aa10/edit?done=on&op=done", CSRF: "c"}
	same(t, "a tick that posts", bind.Tick(&done, false, "Done"), w.Tick{Label: "Done", Send: w.Send{Form: "task.done/app/tasks/a/dela/aa10/edit?done=on&op=done"}})
	same(t, "its form", bind.ActionForm(done, bind.TickID(done)), w.PostForm{ID: bind.TickID(done), Action: done.Href, Fields: []w.Hidden{{Name: "_csrf", Value: "c"}}})
	same(t, "a task done", bind.Tick(&done, true, "Done").Done, true)
	same(t, "a row read only", bind.Tick(nil, true, "Done"), w.Tick{Done: true, Label: "Done"})
	off := done
	off.Enabled, off.Reason = false, "Read only"
	same(t, "a tick switched off", bind.Tick(&off, false, "Done"), w.Tick{Label: "Done"})
}

func TestRelated(t *testing.T) {
	r := view.FormRow{Name: "related", Label: "Related to", Kind: view.InputSelect, Placeholder: "UID of the record", Suggest: "/app/tasks/a/dela/aa10/related", Value: "1d55", Options: []view.Option{
		{Value: "bb21", Label: "Дописать раздел про кэш", Colour: "#3f6e8c", Meta: "task · 14 Aug"},
		{Value: "1d55", Label: "Забрать документы", Colour: "#4a6b52", Meta: "event · 11 Aug", Selected: true},
		{Label: "By UID…", Separator: true, Typed: true, Hint: "By UID: a record that is not among the loaded ones."},
	}}
	same(t, "the record chosen", bind.Related(r), w.TagInput{Name: "related", Parts: []w.TagInputPart{w.Tag{Label: "Забрать документы", Removable: true, Value: "1d55"}}})
	r.Options[1].Selected, r.Value = false, ""
	same(t, "none chosen: the line of a UID", bind.Related(r), w.TagInput{Name: "related", Add: &w.TagAdd{Name: "related", Placeholder: "UID of the record", Label: "Related to", Suggest: "/app/tasks/a/dela/aa10/related"}})
}

func TestPickMenu(t *testing.T) {
	tz := view.FormRow{Name: "tz", Label: "Time zone", Kind: view.InputSelect, Placeholder: "Find a zone", ValueAside: "as sent", Options: []view.Option{
		{Value: "Asia/Yekaterinburg", Label: "Asia/Yekaterinburg", Selected: true, Group: "As sent"},
		{Value: "Africa/Abidjan", Label: "Africa/Abidjan", Separator: true},
		{Value: "UTC", Label: "UTC"},
		{Value: "floating", Label: "Floating — the same clock time anywhere", Separator: true},
		{Value: "x", Label: "X", Disabled: true, Reason: "Not here"},
	}}
	same(t, "the zone", bind.PickMenu(tz), w.Drop{
		Trigger: w.Pick{Label: "Time zone", Value: "Asia/Yekaterinburg", Aside: &w.Dim{Parts: []w.Inline{w.Text("as sent")}, Variant: w.DimIndent}},
		Menu: w.Menu{Items: []w.MenuPart{
			w.Input{Kind: w.InputSearch, Placeholder: "Find a zone", Label: "Find a zone"},
			w.Rubric{Text: "As sent", Form: w.RubricInline},
			w.MenuItem{Label: "Asia/Yekaterinburg", On: true, Name: "tz", Value: "Asia/Yekaterinburg"},
			w.MenuSep{},
			w.MenuItem{Label: "Africa/Abidjan", Name: "tz", Value: "Africa/Abidjan"},
			w.MenuItem{Label: "UTC", Name: "tz", Value: "UTC"},
			w.MenuSep{},
			w.MenuItem{Label: "Floating — the same clock time anywhere", Name: "tz", Value: "floating"},
			w.MenuItem{Label: "X", Name: "tz", Value: "x", Reason: "Not here"},
		}},
	})
}
