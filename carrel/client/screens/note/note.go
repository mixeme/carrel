// Package note is a note, /app/notes/{account}/{col}/{uid}, read — frame
// «Заметка · чтение» (s-notes-2) — and edited at …/{uid}/edit — frame
// «Заметка · правка» (s-notes-20) — and at …/new — frame «Заметка · новая»
// (s-notes-30; edit.go): one view model, NotePage, in either mode (U-6.4).
// The drafts frame2go printed for them, with the frames' data become the
// view model's (U-5.1).
package note

import (
	"strconv"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// printID is Print of the card (U-10): a button of the third rank in the
// head, its twin first in «⋯» on a phone (note 69 of Typecase).
const printID = "record.print"

// The IDs of the page's forms: the deletion posts with no form of its own;
// the edit is a document that is a form, which Save of its bar submits — as
// the frames name it, an edit's (s-notes-20) and a new note's (s-notes-30);
// barSuffix is what the IDs of the forms of the bar's Attach end with — the
// column's Attach has the same items, and a file input each.
const (
	formDelete = "delete"
	// formSend is Send's: the note into a letter by a POST of its own.
	formSend  = "send"
	formEdit  = "note-edit"
	formNew   = "note-new"
	barSuffix = ".bar"
)

// formOf is the ID of the document that is the form of the note's edit: a
// new note's, which has no record yet, or an edit's.
func formOf(p *view.NotePage) string {
	if p.Source.UID == "" {
		return formNew
	}
	return formEdit
}

// The screen's words (U-1.3) for the shell: the mark, the search, the rail.
var (
	top = bind.TopWords{
		Brand:  w.Brand{Logo: &w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63"}},
		Search: "Search everything…",
	}
	rail = bind.RailWords{Toggle: "toggle", None: "—", ReadOnly: "ro", Tags: "Tags", Davloom: "davloom", External: "external"}
)

// The screen's words (U-1.3): the rubrics of the column, the lead of a kind
// and of a difference, what joins a kind and a day, the attachment kept
// inside the note, the rail's rubric of the neighbours and the phone's short
// «Back», «Upload» after Attach's arrow (Typecase's task 109).
const (
	neighbours = "Neighbours"
	back       = "Back"
	lead       = "· "
	sep        = " · "
	inside     = "in the note"
	upload     = "Upload"
)

// The client's controls of the bar over the text (U-6.4): the source, the
// width of the text, the side column — none of them the server's.
const (
	source     = "Source"
	fullWidth  = "Full width"
	readWidth  = "Reading width"
	hideColumn = "Hide the side column"
)

// The names of the contract the screen places by (C-7.5): Attach goes in the
// rubric of attachments.
const attachID = "attach.add"

// faces are the faces the page draws a note's text in (s-notes-2).
var faces = bind.TextFaces{Lead: w.ProseLead, After: w.ProseLeadAfter, Tick: w.TickNudge, Code: w.MonoChip}

// Screen draws the note from its view model, read or in its edit
// (NoteDoc.Mode).
//
//   - The shell: the top bar; the rail of return — «Back to list», the
//     notebooks, the notes beside it (Neighbours, the open one on), the
//     foot.
//   - Read: the head — the notebook's mark, where the note lives and when,
//     monospaced —, Edit, Export Markdown, Print, the client's Source, width
//     and side column, «⋯» with the record's menu (Move, Copy, Publish a
//     link); the notices; the text — its title, its paragraphs, headings,
//     tasks (a tick a sign, U-6.4), lists, quotations, code, tables —; the
//     column: tags, related records, attachments with Attach, the same note in
//     other notebooks, the source, Delete note.
//   - In the edit — edit.go.
func Screen(p *view.NotePage) w.Shell {
	var main []w.MainPart
	if p.Doc.Mode == view.NoteEditing {
		main = edit(p)
	} else {
		main = read(p)
	}
	t := bind.Top(p.Top, top)
	return w.Shell{Top: &t, Body: w.ShellBody{Rail: railParts(p.Rail), Main: w.Main{Items: main}}}
}

// Forms are the forms of the actions that post with none of their own: the
// deletion, Attach «From this computer» with its file, an attachment's
// Remove in the edit, the notices' buttons and the menus' items that post
// (Sign out). The edit's form is its document.
func Forms(p *view.NotePage) []w.PostForm {
	out := append(bind.TopForms(p.Top), bind.MenuForms(p.Meta.Menu)...)
	out = append(out, bind.NoticeForms(p.Notices)...)
	for _, a := range p.Meta.Actions {
		switch {
		case a.ID == attachID:
			out = append(out, bind.MenuForms(a.Arrow)...)
		case a.Danger && a.Method == "POST" && a.Enabled:
			out = append(out, bind.ActionForm(a, formDelete))
		}
	}
	for _, a := range p.Doc.Bar {
		if p.Doc.Mode != view.NoteEditing && a.Method == "POST" && a.Enabled {
			out = append(out, bind.ActionForm(a, formSend))
		}
	}
	if p.Doc.Mode == view.NoteEditing {
		for _, a := range p.Doc.Bar {
			if a.ID == attachID {
				out = append(out, bind.MenuForms(barItems(a.Arrow))...)
			}
		}
		for _, a := range p.Meta.Attach {
			for _, act := range a.Actions {
				if act.Method == "POST" && act.Enabled && !act.Submits {
					out = append(out, bind.ActionForm(act, removeID(act)))
				}
			}
		}
	}
	return out
}

// removeID is the ID of the form of an attachment's Remove: its action's ID
// and address, one per attachment.
func removeID(a view.Action) string {
	return a.ID + a.Href
}

// railParts is the rail of return: «Back to list», with the phone's short
// «Back» (U-1.3), the notebooks, the notes beside the open one, the foot.
func railParts(r view.Rail) []w.RailPart {
	nav := w.Nav{}
	for _, n := range r.Nav {
		it := bind.NavItem(n)
		it.Short = back
		nav.Items = append(nav.Items, it)
	}
	parts := []w.RailPart{nav}
	parts = append(parts, bind.Sources(r, rail)...)
	if len(r.Neighbours) > 0 {
		tree := w.Tree{Variant: w.TreePadded}
		for i, a := range r.Neighbours {
			tree.Rows = append(tree.Rows, bind.TreeRow(a))
			if a.On {
				tree.On = i + 1
			}
		}
		parts = append(parts, w.RailSec{Items: []w.RailSecPart{w.Rubric{Text: neighbours}, tree}})
	}
	return append(parts, bind.Foot(r, rail))
}

// read is the note read: its head, the notices, the text and its column.
func read(p *view.NotePage) []w.MainPart {
	var acts []w.ActsPart
	for _, a := range p.Doc.Bar {
		form := ""
		if a.Method == "POST" {
			form = formSend
		}
		b := bind.Button(a, form)
		b.Tertiary = a.ID == printID
		acts = append(acts, b)
	}
	// ждёт библиотеку: U-5.5 #41 — the client's Source, width and side
	// column do nothing without a script (Typecase's task 104)
	// Source is its icon, the word its hint; the width is of the third rank:
	// on a phone it means nothing (note 69 of Typecase).
	// A linked record's head has Edit and «⋯» alone (s-notes-32): its text
	// is no one copy's source.
	if !p.LinkedRecord {
		acts = append(acts,
			w.Button{Icon: "code", Title: source},
			widthSeg(p.Look, true),
			w.Button{Icon: "panel", Title: hideColumn},
		)
	}
	more := w.Button{Icon: "dots", Title: "More"}
	if p.LinkedRecord {
		more.Variant = w.ButtonPadX3 // s-notes-32
	}
	if len(p.Meta.Menu) > 0 {
		acts = append(acts, w.Drop{
			Trigger: more,
			Menu:    w.Menu{Items: append(bind.Twins(p.Doc.Bar, printID), bind.MenuItems(p.Meta.Menu, rail.ReadOnly)...), Width: w.MenuWidth212, Pop: w.MenuUnderEnd},
		})
	}
	meta := []w.HeadSubPart{w.Mark{Colour: string(p.Source.Colour), Size: w.MarkH12}, w.Text(p.Source.Stamp)}
	switch {
	case p.LinkedRecord:
		// The linked record: where it is kept and the notebook its text
		// came from, its mark before its name (s-notes-32).
		meta = []w.HeadSubPart{w.Text(p.Subtitle)}
		if f := p.TextFrom; f != nil {
			meta = []w.HeadSubPart{w.Text(p.Subtitle + " "), w.Mark{Colour: string(f.Colour), Size: w.MarkH12}, w.Text(f.Name)}
		}
	case p.Meta.Date != "":
		meta = []w.HeadSubPart{meta[0], w.Text(string(p.Source.Stamp) + sep), w.Mono{Text: string(p.Meta.Date)}}
	}
	main := []w.MainPart{w.Head{Meta: &w.HeadSub{Parts: meta}, Acts: &w.Acts{Items: acts}, Pad: w.HeadRule}}
	main = append(main, notices(p)...)
	doc := w.Doc{Items: append([]w.DocPart{w.DocTitle{Text: string(p.Doc.Title)}}, bind.DocText(p.Doc.Rendered, faces)...)}
	var secs []w.DocmetaSec
	if len(p.Meta.Tags) > 0 {
		var tags []w.LinePart
		for _, t := range bind.Tags(p.Meta.Tags) {
			tags = append(tags, t)
		}
		// «linked» at the rubric: another copy holds other tags (s-notes-2).
		title := w.Rubric{Text: "Tags"}
		if b := p.Meta.TagsBadge; b != nil {
			title.Items = []w.RubricPart{bind.Badge(*b)}
		}
		// The notebook a tag kept came from, on a linked record (s-notes-32).
		if f := p.Meta.TagsFrom; f != nil {
			tags = append(tags, w.Text(" "), w.Dim{Mark: &w.Mark{Colour: string(f.Colour), Size: w.MarkH11Inline}, Parts: []w.Inline{w.Text(f.Name)}})
		}
		secs = append(secs, w.DocmetaSec{Title: title, Items: []w.DocmetaPart{w.Line{Block: true, Gap: w.LineGap4, Items: tags}}})
	}
	// The note's link, as an event's and a task's card shows it (Р8 of the
	// kinds-parity list): a reference the note points at, drawn as Related
	// draws its own. No frame draws it yet (Typecase's task 172).
	if l := p.Meta.Link; l != nil {
		secs = append(secs, w.DocmetaSec{Title: w.Rubric{Text: string(l.Label)},
			Items: []w.DocmetaPart{w.DocRefs{Items: []w.DocRef{{Name: string(l.Value), Href: l.Href}}}}})
	}
	if len(p.Meta.Related) > 0 {
		refs := w.DocRefs{}
		for _, r := range p.Meta.Related {
			about := lead + string(r.Kind)
			if r.When != "" {
				about += sep + string(r.When)
			}
			refs.Items = append(refs.Items, w.DocRef{Name: string(r.Title), Href: r.Href, What: w.Dim{Parts: []w.Inline{w.Text(about)}}})
		}
		secs = append(secs, w.DocmetaSec{Title: w.Rubric{Text: "Related"}, Items: []w.DocmetaPart{refs}})
	}
	// The note's people, as an event's and a task's card names them — the
	// organiser first —, drawn as Related draws its records: the frame lets
	// a section stand beside its own only in their shape. The name leads to
	// the card when the books know it, else the address; what each said
	// muted (Р3 of the kinds-parity list; no frame draws them yet —
	// Typecase's task 179).
	if len(p.Meta.Attendees) > 0 {
		refs := w.DocRefs{}
		for _, a := range p.Meta.Attendees {
			name := string(a.Name)
			if a.Href == "" {
				name = string(a.Email)
			}
			refs.Items = append(refs.Items, w.DocRef{Name: name, Href: a.Href, What: w.Dim{Parts: []w.Inline{w.Text(lead + string(a.Status))}}})
		}
		secs = append(secs, w.DocmetaSec{Title: w.Rubric{Text: "Attendees"}, Items: []w.DocmetaPart{refs}})
	}
	secs = append(secs, column(p)...)
	return append(main, w.DocLayout{View: layoutOf(p.Look), Items: []w.DocLayoutPart{doc, w.Docmeta{Sections: secs, Danger: danger(p.Meta.Actions)}}})
}

// notices are the notices over the text and what the server did not keep of
// the last save.
func notices(p *view.NotePage) []w.MainPart {
	out := bind.Notices(p.Notices, w.MsgSpacePage)
	if p.Loss != nil {
		out = append(out, bind.Loss(*p.Loss, w.MsgSpacePage)...)
	}
	return out
}

// column is what the column holds read and in the edit alike: the
// attachments with Attach and its arrow — in the edit each with its Remove
// at the right (DocAttach.Remove, s-notes-20) —, the same note in other
// notebooks, the source — none for a new note, which has no record yet
// (s-notes-30).
func column(p *view.NotePage) []w.DocmetaSec {
	var secs []w.DocmetaSec
	var attach *view.Action
	for _, a := range p.Meta.Actions {
		if a.ID == attachID {
			attach = &a
		}
	}
	if attach != nil || len(p.Meta.Attach) > 0 {
		rubric := w.Rubric{Text: "Attachments"}
		if attach != nil {
			// ждёт библиотеку: U-5.5 #43 — the frame draws the file a field
			// of the edit that goes with Save (FileField.List); it goes by
			// its own form, as the contract has it, until Save takes files
			b := bind.Button(*attach, "")
			b.Menu, b.Small = true, true
			items := append(bind.MenuItems(attach.Arrow, ""), bind.Uploads(attach.Arrow, upload)...)
			for _, it := range items {
				// A new note's file sends the document it is typed in (Submits,
				// FileField.To): the column stands outside it.
				if m, ok := it.(w.MenuItem); ok && m.File != nil && m.File.To != "" {
					m.File.Form = formOf(p)
				}
			}
			// ждёт библиотеку: U-5.5 #36 — a new note's From Files and
			// «Upload» send the form they stand in (MenuItem.To), and the
			// column stands outside the document
			rubric.Items = []w.RubricPart{w.Drop{Trigger: b, Menu: w.Menu{Items: items, Width: w.MenuWidth212, Pop: w.MenuUnder}}}
		}
		var items []w.DocmetaPart
		for _, a := range p.Meta.Attach {
			at := w.DocAttach{Name: string(a.Name), Size: w.Dim{Parts: []w.Inline{w.Text(lead + string(a.Size))}}, Href: a.Href}
			for _, act := range a.Actions {
				if p.Doc.Mode != view.NoteEditing {
					break
				}
				r := bind.Button(act, removeID(act))
				r.Icon, r.Kind, r.Small = "", w.ButtonPlain, true
				if act.Danger {
					r.Kind = w.ButtonDanger
				}
				at.Remove = &r
			}
			switch {
			case a.Inline:
				at.Where = w.Badge{Label: inside}
			case a.Place != "":
				at.Where = w.Badge{Label: string(a.Place)}
			}
			items = append(items, at)
		}
		secs = append(secs, w.DocmetaSec{Title: rubric, Items: items})
	}
	if len(p.Meta.Linked) > 0 {
		refs := w.DocRefs{}
		for _, f := range p.Meta.Linked {
			ref := w.DocRef{Mark: &w.Mark{Colour: string(f.Colour), Size: w.MarkH11Inline}, Name: string(f.Label), Href: f.Href}
			if f.Value != "" {
				ref.What = w.Dim{Parts: []w.Inline{w.Text(lead + string(f.Value))}}
			}
			refs.Items = append(refs.Items, ref)
		}
		secs = append(secs, w.DocmetaSec{
			Title: w.Rubric{Text: keptRubric(p.LinkedRecord), Items: []w.RubricPart{w.Badge{Label: strconv.Itoa(len(p.Meta.Linked)), Tone: w.BadgeLinked}}},
			Items: []w.DocmetaPart{refs},
		})
	}
	if p.Source.UID == "" {
		return secs
	}
	lines := []bind.DiagLine{{Name: "UID", Value: string(p.Source.UID)}}
	read := string(p.Source.LastRead)
	if p.Source.Compared != "" {
		read += sep + string(p.Source.Compared)
	}
	lines = append(lines, bind.DiagLine{Name: "Read", Value: read})
	return append(secs, w.DocmetaSec{Title: w.Rubric{Text: "Source"}, Items: []w.DocmetaPart{bind.Diag(lines...)}})
}

// danger is the deletion at the column's end; nil where there is none (a new
// note).
func danger(actions []view.Action) *w.Danger {
	for _, a := range actions {
		if a.Danger {
			b := bind.Button(a, formDelete)
			b.Icon = ""
			return &w.Danger{Button: b, Space: w.DangerAtEnd}
		}
	}
	return nil
}

// layoutOf is how wide the note stands (Typecase's note 79, task 204): the
// reading measure by default, the page's width where the person chose it
// (Look.Wide, «Open notes in full width», s-notes-2, -3).
func layoutOf(look view.Look) w.DocView {
	if look.Wide {
		return w.DocViewFull
	}
	return w.DocViewMeasure
}

// widthSeg is the switch of the width in a bar, the width in force on:
// of the third rank in the reading bar (on a phone it means nothing, note
// 69 of Typecase), of the second in the edit's.
func widthSeg(look view.Look, reading bool) w.Seg {
	s := w.Seg{Options: []w.SegOption{{Label: fullWidth, On: look.Wide}, {Label: readWidth, On: !look.Wide}}}
	if reading {
		s.Tertiary = true
	} else {
		s.Secondary = true
	}
	return s
}

// keptRubric is the rubric of the other copies: on a linked record's card,
// where it is kept (s-notes-32).
func keptRubric(linked bool) string {
	if linked {
		return "Where this note is kept"
	}
	return "The same note in other notebooks"
}
