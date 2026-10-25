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

// printID is Print of the card (U-10): a button of the third rank in the
// head, its twin first in «⋯» on a phone (note 69 of Typecase).
const printID = "record.print"

// The IDs of the page's forms: the deletion posts with no form of its own.
// The IDs of the page's forms: the deletion's, and Send's — the record into
// a letter by a POST of its own (Р23 of the server's kinds-parity list).
const (
	formDelete = "delete"
	formSend   = "send"
)

// The screen's words (U-1.3) for the shell: the mark, the search, the rail.
var (
	top = bind.TopWords{
		Brand:  w.Brand{Logo: &w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63"}},
		Search: "Search everything…",
		NewKey: "n", SearchKey: "/",
		Theme: bind.ThemeWords{Label: "Theme", Auto: "Auto", Light: "Light", Dark: "Dark"},
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
	h := w.Head{Title: title, Subs: []w.HeadSub{bind.SourceSub(p.Source.Colour, view.Text(p.Source.Stamp))}}
	if p.LinkedRecord {
		// The linked record: where it is kept, no one calendar (s-agenda-25).
		h.Subs = []w.HeadSub{{Parts: []w.HeadSubPart{w.Text(p.Subtitle)}}}
	}
	var acts []w.ActsPart
	var attach *view.Action
	var danger []w.Button
	for _, a := range p.Actions {
		switch {
		case a.ID == attachID:
			attach = &a
		case a.Danger:
			danger = append(danger, bind.Button(a, formDelete))
		case a.Method == "POST":
			acts = append(acts, bind.Button(a, formSend))
		default:
			b := bind.Button(a, "")
			b.Tertiary = a.ID == printID
			acts = append(acts, b)
		}
	}
	if len(p.Menu) > 0 {
		acts = append(acts, w.Drop{
			Trigger: moreButton(p.LinkedRecord),
			Menu:    w.Menu{Items: append(bind.Twins(p.Actions, printID), bind.MenuItems(p.Menu, rail.ReadOnly)...), Width: w.MenuWidth212, Pop: w.MenuUnderEnd},
		})
	}
	h.Acts = &w.Acts{Items: acts}
	main := []w.MainPart{h}
	main = append(main, bind.Notices(p.Notices, w.MsgSpacePage)...)
	if p.Loss != nil {
		main = append(main, bind.Loss(*p.Loss, w.MsgSpacePage)...)
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
			b.Menu, b.Small = true, true
			rubric.Items = []w.RubricPart{w.Drop{Trigger: b, Menu: w.Menu{Items: append(bind.MenuItems(attach.Arrow, ""), bind.Uploads(attach.Arrow, upload)...), Width: w.MenuWidth212, Pop: w.MenuUnder}}}
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
			lines = append(lines, bind.WithFrom([]w.SecItemPart{
				w.Span{Face: w.FaceAccent, Text: string(r.Title), Href: r.Href},
				w.Text(" "),
				w.Dim{Parts: []w.Inline{w.Text(about(r))}},
			}, r.From))
		}
		main = append(main, w.Sec{Items: []w.SecPart{w.Rubric{Text: "Related"}, w.SecList{Items: lines}}})
	}
	if len(p.Linked) > 0 {
		var lines [][]w.SecItemPart
		for _, f := range p.Linked {
			lines = append(lines, bind.Book(f, lead))
		}
		main = append(main, w.Sec{Items: []w.SecPart{
			w.Rubric{Text: keptRubric(p.LinkedRecord), Items: []w.RubricPart{w.Badge{Label: strconv.Itoa(len(p.Linked)), Tone: w.BadgeLinked}}},
			w.SecList{Items: lines, Gap: 5},
		}})
	}
	if p.LinkedRecord {
		// The source and the deletion are a copy's (s-agenda-25).
		t := bind.Top(p.Top, top)
		return w.Shell{Top: &t, Body: w.ShellBody{Rail: railParts(p.Rail), Main: w.Main{Items: main}}}
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
		case a.Method == "POST" && a.Enabled:
			out = append(out, bind.ActionForm(a, formSend))
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
			// The badge is the last but what a linked record names after
			// it (Field.From).
			for i := len(it.Value) - 1; i >= 0; i-- {
				if b, ok := it.Value[i].(w.Badge); ok {
					b.Title = zoneHint
					it.Value[i] = b
					break
				}
			}
		}
		out.Items = append(out.Items, it)
	}
	return out
}

// attachments are the lines of the event's attachments: the name, the type
// and size muted, and where the file is kept — «in the event» for data
// inside it (S-20). The name is a link where it goes (Href: data inside the record
// downloads, a file in Files opens there; task 93); an attachment is taken
// off on the edit only (s-agenda-3), and the card carries no action for it.
func attachments(rows []view.AttachRow) [][]w.SecItemPart {
	var out [][]w.SecItemPart
	for _, a := range rows {
		var line []w.SecItemPart
		if a.Href != "" {
			line = append(line, w.Link{Text: string(a.Name), Href: a.Href}, w.Text(" "))
		} else {
			line = append(line, w.Text(string(a.Name)+" "))
		}
		line = append(line, w.Dim{Parts: []w.Inline{w.Text(string(a.Type) + " · " + string(a.Size))}})
		if a.Inline {
			line = append(line, w.Text(" "), w.Badge{Label: inside})
		}
		out = append(out, bind.WithFrom(line, a.From))
	}
	return out
}

// upload is the screen's word (U-1.3) for the item after Attach's arrow that
// sends the file picked on a page without a script (Typecase's task 109).
const upload = "Upload"

// moreButton is the «⋯» of the head: on a linked record's card the narrow
// icon button its frame draws (s-contacts-33, s-agenda-25, s-tasks-17).
func moreButton(linked bool) w.Button {
	b := w.Button{Icon: "dots", Title: "More"}
	if linked {
		b.Variant = w.ButtonPadX3
	}
	return b
}

// keptRubric is the rubric of the other copies: on a linked record's card,
// where it is kept (s-agenda-25).
func keptRubric(linked bool) string {
	if linked {
		return "Where this event is kept"
	}
	return "The same event in other calendars"
}

// about is what a related record is, and its day where it has one: «· event
// · 11 Aug» (s-tasks-20; Typecase's note 130 p. 3), as a note's panel says
// it.
func about(r view.TimelineRow) string {
	if r.When == "" {
		return lead + string(r.Kind)
	}
	return lead + string(r.Kind) + " " + lead + string(r.When)
}
