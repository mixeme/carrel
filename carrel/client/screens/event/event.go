// Package event is an event, /app/calendar/{account}/{col}/{uid}, drawn by
// frame «Событие · карточка» (s-agenda-2) of the mockup: the draft frame2go
// printed for it, with the frame's data become the view model's (U-5.1,
// U-6.2). It is read: here one comes from a search, a link in another record,
// a person's timeline, a narrow screen.
package event

import (
	"strconv"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The IDs of the page's forms: the deletion posts with no form of its own.
const formDelete = "delete"

// The screen's words (U-1.3) for the shell: the mark, the search, the rail.
var (
	top = bind.TopWords{
		Brand:  w.Brand{Logo: &w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63"}},
		Search: "Search everything…",
	}
	rail = bind.RailWords{Toggle: "toggle", None: "—", ReadOnly: "ro", Tags: "Tags", Davloom: "davloom", External: "external"}
)

// The screen's words (U-1.3): the lead of a status, a kind and a difference,
// what a badge of a time made in another zone says, the attachment kept
// inside the event.
const (
	lead     = "· "
	zoneHint = "Created in another time zone"
	inside   = "in the event"
)

// attachID is the name of the action Attach (C-7.5): the screen puts it in
// the rubric of attachments, not with the other actions.
const attachID = "attach.add"

// Screen draws the event from its view model. It is read, not edited.
//
//   - The shell: the top bar; the rail of return — «Back to list», the
//     calendar's sources, the foot.
//   - The head: the calendar's mark, the title, the calendar; the actions on
//     view — Edit, New task, Print — and «⋯» with the record's menu (Move,
//     Copy, Export .ics, Publish a link).
//   - The notices, and what the server did not keep of the last save.
//   - The fields: When in its pieces, the zone it was made in as a badge;
//     Repeats with its end as a badge; Reminders, Where, Status, the tags.
//   - The description; the attendees — a person the books know by the
//     book's mark and a link to the card, a stranger by the address —, their
//     number in the rubric; the attachments with Attach and its arrow, each
//     where it is kept; the related records; the same event in other
//     calendars; the source.
//   - The deletion at the foot, and under it the line of a series.
func Screen(p *view.EventPage) w.Shell {
	title := &w.HeadTitle{Text: string(p.Summary)}
	if p.Source.Colour != "" {
		title.Mark = &w.Mark{Colour: string(p.Source.Colour), Size: w.MarkH19}
	}
	h := w.Head{Title: title, Subs: []w.HeadSub{{Parts: []w.HeadSubPart{w.Text(p.Source.Stamp)}}}}
	var acts []w.ActsPart
	var attach *view.Action
	var danger []w.Button
	for _, a := range p.Actions {
		switch {
		case a.ID == attachID:
			attach = &a
		case a.Danger:
			danger = append(danger, bind.Button(a, formDelete))
		default:
			acts = append(acts, bind.Button(a, ""))
		}
	}
	if len(p.Menu) > 0 {
		acts = append(acts, w.Drop{
			Trigger: w.Button{Icon: "dots", Title: "More"},
			Menu:    w.Menu{Items: bind.MenuItems(p.Menu, rail.ReadOnly), Width: w.MenuWidth212, Pop: w.MenuUnderEnd},
		})
	}
	h.Acts = &w.Acts{Items: acts}
	main := []w.MainPart{h}
	main = append(main, bind.Notices(p.Notices, w.MsgSpacePage)...)
	if p.Loss != nil {
		main = append(main, bind.Loss(*p.Loss, w.MsgSpacePage))
	}
	main = append(main, fields(p.Fields))
	if p.Body != "" {
		main = append(main, w.Sec{Items: []w.SecPart{w.Rubric{Text: "Description"}, w.Prose{Parts: []w.Inline{w.Text(p.Body)}}}})
	}
	if len(p.Attendees) > 0 {
		var lines [][]w.SecItemPart
		for _, a := range p.Attendees {
			lines = append(lines, bind.Attendee(a, lead))
		}
		main = append(main, w.Sec{Items: []w.SecPart{
			w.Rubric{Text: "Attendees", Items: []w.RubricPart{w.Mono{Text: strconv.Itoa(len(p.Attendees)), Variant: w.MonoTight}}},
			w.SecList{Items: lines, Columns: true},
		}})
	}
	if attach != nil || len(p.Attach) > 0 {
		rubric := w.Rubric{Text: "Attachments"}
		if attach != nil {
			b := bind.Button(*attach, "")
			b.Icon, b.Menu, b.Small = "", true, true
			rubric.Items = []w.RubricPart{w.Drop{Trigger: b, Menu: w.Menu{Items: bind.MenuItems(attach.Arrow, ""), Width: w.MenuWidth212, Pop: w.MenuUnder}}}
		}
		sec := []w.SecPart{rubric}
		if len(p.Attach) > 0 {
			sec = append(sec, w.SecList{Items: attachments(p.Attach), Gap: 6})
		}
		main = append(main, w.Sec{Items: sec})
	}
	if len(p.Related) > 0 {
		var lines [][]w.SecItemPart
		for _, r := range p.Related {
			lines = append(lines, []w.SecItemPart{
				w.Span{Face: w.FaceAccent, Text: string(r.Title), Href: r.Href},
				w.Text(" "),
				w.Dim{Parts: []w.Inline{w.Text(lead + string(r.Kind))}},
			})
		}
		main = append(main, w.Sec{Items: []w.SecPart{w.Rubric{Text: "Related"}, w.SecList{Items: lines}}})
	}
	if len(p.Linked) > 0 {
		var lines [][]w.SecItemPart
		for _, f := range p.Linked {
			lines = append(lines, bind.Book(f, lead))
		}
		main = append(main, w.Sec{Items: []w.SecPart{
			w.Rubric{Text: "The same event in other calendars", Items: []w.RubricPart{w.Badge{Label: strconv.Itoa(len(p.Linked)), Tone: w.BadgeLinked}}},
			w.SecList{Items: lines, Gap: 5},
		}})
	}
	read := string(p.Source.LastRead)
	if p.Source.Compared != "" {
		read += " · " + string(p.Source.Compared)
	}
	main = append(main, w.Sec{Items: []w.SecPart{
		w.Rubric{Text: "Source"},
		bind.Diag(bind.DiagLine{Name: "Path", Value: string(p.Source.Path)}, bind.DiagLine{Name: "Read", Value: read}),
	}})
	for _, b := range danger {
		main = append(main, w.Danger{Button: b, Note: string(p.Recurrence.Note)})
	}
	t := bind.Top(p.Top, top)
	return w.Shell{Top: &t, Body: w.ShellBody{Rail: railParts(p.Rail), Main: w.Main{Items: main}}}
}

// Forms are the forms of the actions that post with none of their own: the
// deletion, Attach «From this computer» with its file, the notices' buttons
// and the menus' items that post (Sign out).
func Forms(p *view.EventPage) []w.PostForm {
	out := append(bind.TopForms(p.Top), bind.MenuForms(p.Menu)...)
	out = append(out, bind.NoticeForms(p.Notices)...)
	for _, a := range p.Actions {
		switch {
		case a.ID == attachID:
			out = append(out, bind.MenuForms(a.Arrow)...)
		case a.Danger && a.Method == "POST" && a.Enabled:
			out = append(out, bind.ActionForm(a, formDelete))
		}
	}
	return out
}

// railParts is the rail of return: «Back to list», with the phone's short
// «Back» (U-1.3), the calendar's sources, the foot.
func railParts(r view.Rail) []w.RailPart {
	nav := w.Nav{}
	for _, n := range r.Nav {
		it := bind.NavItem(n)
		it.Short = "Back"
		nav.Items = append(nav.Items, it)
	}
	parts := []w.RailPart{nav}
	parts = append(parts, bind.Sources(r, rail)...)
	return append(parts, bind.Foot(r, rail))
}

// fields are the event's fields as the lines of a card: When in its pieces,
// a badge of a time made in another zone saying so where a pointer rests.
func fields(fs []view.Field) w.Fields {
	out := w.Fields{}
	for _, f := range fs {
		it := bind.FieldsItem(f, lead)
		if len(f.Runs) > 0 && f.Badge.Label != "" {
			b := it.Value[len(it.Value)-1].(w.Badge)
			b.Title = zoneHint
			it.Value[len(it.Value)-1] = b
		}
		out.Items = append(out.Items, it)
	}
	return out
}

// attachments are the lines of the event's attachments: the name, the type
// and size muted, and where the file is kept — «in the event» for data
// inside it (S-20).
//
// ждёт библиотеку: U-5.5 #29 — no frame draws a line's actions
// (AttachRow.Actions, Remove): an attachment is not taken off here
func attachments(rows []view.AttachRow) [][]w.SecItemPart {
	var out [][]w.SecItemPart
	for _, a := range rows {
		line := []w.SecItemPart{w.Text(string(a.Name) + " "), w.Dim{Parts: []w.Inline{w.Text(string(a.Type) + " · " + string(a.Size))}}}
		if a.Inline {
			line = append(line, w.Text(" "), w.Badge{Label: inside})
		}
		out = append(out, line)
	}
	return out
}
