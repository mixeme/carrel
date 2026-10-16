// Package task is a task, /app/tasks/{account}/{col}/{uid}, drawn by frame
// «Задача · чтение» (s-tasks-2) of the mockup: the draft frame2go printed for
// it, with the frame's data become the view model's (U-5.1, U-6.3). It is
// read; the edit has its own address. Marking it done is no edit of the form
// but the same three properties the list's tick changes.
package task

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
	}
	rail = bind.RailWords{Toggle: "toggle", None: "—", ReadOnly: "ro", Tags: "Tags", Davloom: "davloom", External: "external"}
)

// The screen's words (U-1.3): the lead of a kind and a difference, what
// joins a related record's kind and day, the attachment kept inside the task.
const (
	lead   = "· "
	sep    = " · "
	inside = "in the task"
)

// The names of the contract the screen places by (C-7.5): Attach goes in the
// rubric of attachments, Mark done carries the tick's glyph.
const (
	attachID = "attach.add"
	doneID   = "task.done"
)

// Screen draws the task from its view model. It is read, not edited.
//
//   - The shell: the top bar; the rail of return — «Back to list», the
//     list's sources, the foot.
//   - The head: the title, where the task lives and — overdue — by how long,
//     in alarm; the actions on view — Edit, Mark done with its tick, Print —
//     and «⋯» with the record's menu (Move, Copy, Export .ics, Publish a
//     link).
//   - The notices, and what the server did not keep of the last save.
//   - The fields: Status, Due (overdue in alarm), Starts, Complete and
//     Priority monospaced, Reminders, Where, Link, the tags.
//   - The description; the attachments with Attach and its arrow, each where
//     it is kept; the related records with their kind and day; the people,
//     the organiser first, as an event's card names them (Р3 of the server's
//     kinds-parity list; no frame draws them yet — Typecase's task 179); the
//     same task in other lists; the source.
//   - The deletion at the foot.
func Screen(p *view.TaskPage) w.Shell {
	h := w.Head{Title: &w.HeadTitle{Text: string(p.Summary)}, Subs: bind.Sub(p.Subtitle)}
	var acts []w.ActsPart
	var attach *view.Action
	var danger []w.Button
	for _, a := range p.Actions {
		switch {
		case a.ID == attachID:
			attach = &a
		case a.Danger:
			danger = append(danger, bind.Button(a, formDelete))
		case a.ID == doneID:
			b := bind.Button(a, bind.TickID(a))
			b.Tick = &w.Tick{Glyph: true, Done: p.Done}
			acts = append(acts, b)
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
			Trigger: w.Button{Icon: "dots", Title: "More"},
			Menu:    w.Menu{Items: append(bind.Twins(p.Actions, printID), bind.MenuItems(p.Menu, rail.ReadOnly)...), Width: w.MenuWidth212, Pop: w.MenuUnderEnd},
		})
	}
	h.Acts = &w.Acts{Items: acts}
	main := []w.MainPart{h}
	main = append(main, bind.Notices(p.Notices, w.MsgSpacePage)...)
	if p.Loss != nil {
		main = append(main, bind.Loss(*p.Loss, w.MsgSpacePage)...)
	}
	fields := w.Fields{}
	for _, f := range p.Fields {
		fields.Items = append(fields.Items, bind.FieldsItem(f, lead))
	}
	main = append(main, fields)
	if p.Body != "" {
		main = append(main, w.Sec{Items: []w.SecPart{w.Rubric{Text: "Description"}, w.Prose{Parts: []w.Inline{w.Text(p.Body)}}}})
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
			about := lead + string(r.Kind)
			if r.When != "" {
				about += sep + string(r.When)
			}
			lines = append(lines, []w.SecItemPart{
				w.Span{Face: w.FaceAccent, Text: string(r.Title), Href: r.Href},
				w.Text(" "),
				w.Dim{Parts: []w.Inline{w.Text(about)}},
			})
		}
		main = append(main, w.Sec{Items: []w.SecPart{w.Rubric{Text: "Related"}, w.SecList{Items: lines}}})
	}
	// The people, drawn as Related draws its records (bind.Person).
	if len(p.Attendees) > 0 {
		var lines [][]w.SecItemPart
		for _, a := range p.Attendees {
			lines = append(lines, bind.Person(a, lead))
		}
		main = append(main, w.Sec{Items: []w.SecPart{w.Rubric{Text: "Attendees"}, w.SecList{Items: lines}}})
	}
	if len(p.Linked) > 0 {
		var lines [][]w.SecItemPart
		for _, f := range p.Linked {
			lines = append(lines, bind.Book(f, lead))
		}
		main = append(main, w.Sec{Items: []w.SecPart{
			w.Rubric{Text: "The same task in other lists", Items: []w.RubricPart{w.Badge{Label: strconv.Itoa(len(p.Linked)), Tone: w.BadgeLinked}}},
			w.SecList{Items: lines, Gap: 5},
		}})
	}
	read := string(p.Source.LastRead)
	if p.Source.Compared != "" {
		read += sep + string(p.Source.Compared)
	}
	main = append(main, w.Sec{Items: []w.SecPart{
		w.Rubric{Text: "Source"},
		bind.Diag(bind.DiagLine{Name: "Path", Value: string(p.Source.Path)}, bind.DiagLine{Name: "Read", Value: read}),
	}})
	for _, b := range danger {
		main = append(main, w.Danger{Button: b})
	}
	t := bind.Top(p.Top, top)
	return w.Shell{Top: &t, Body: w.ShellBody{Rail: railParts(p.Rail), Main: w.Main{Items: main}}}
}

// Forms are the forms of the actions that post with none of their own: Mark
// done, the deletion, Attach «From this computer» with its file, the
// notices' buttons and the menus' items that post (Sign out).
func Forms(p *view.TaskPage) []w.PostForm {
	out := append(bind.TopForms(p.Top), bind.MenuForms(p.Menu)...)
	out = append(out, bind.NoticeForms(p.Notices)...)
	for _, a := range p.Actions {
		switch {
		case a.ID == attachID:
			out = append(out, bind.MenuForms(a.Arrow)...)
		case a.Method != "POST" || !a.Enabled:
		case a.Danger:
			out = append(out, bind.ActionForm(a, formDelete))
		case a.ID == doneID:
			out = append(out, bind.ActionForm(a, bind.TickID(a)))
		default:
			out = append(out, bind.ActionForm(a, formSend))
		}
	}
	return out
}

// railParts is the rail of return: «Back to list», with the phone's short
// «Back» (U-1.3), the list's sources, the foot.
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

// attachments are the lines of the task's attachments: the name, a link
// where it goes (data inside the task downloads, a file in Files opens
// there), the type and size muted, and «in the task» for data inside it
// (S-20). An attachment is taken off on the edit only.
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
		out = append(out, line)
	}
	return out
}

// upload is the screen's word (U-1.3) for the item after Attach's arrow that
// sends the file picked on a page without a script (Typecase's task 109).
const upload = "Upload"
