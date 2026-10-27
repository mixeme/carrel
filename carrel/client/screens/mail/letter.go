package mail

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of the letter (U-1.3): its title, its ID the recipients' Enter
// sends, what leads the sender's address in the head, the rubric of what is
// attached and Attach, a part as a file and as a link, the cross of a part,
// the keys of the main button and of Cancel, the icon of Attach and the
// rule's place in its menu.
const (
	letterTitle = "Letter"
	letterID    = "letter"
	fromWord    = "from "
	attached    = "Attached"
	attachWord  = "Attach"
	asFile      = "attached"
	asLink      = "as a link"
	removeWord  = "Remove"
	escKey      = "Esc"
	enterKey    = "Enter"
	ctrlEnter   = "Ctrl+Enter"
	clipIcon    = "clip"
	enterWord   = "Add"
	enterName   = "op"
	enterValue  = "add"
)

// The names of the contract the screen places by (C-7.5): the rows of the
// form, its buttons, the attach menu's items by what they attach.
const (
	toField     = "to"
	textField   = "text"
	previewID   = "mail.preview"
	attachMenu  = "mail.attach."
	attachFile  = "mail.attach.file"
	attachTitle = "mail.attach.folder"
)

// kindIcons are the icons of what is attached, by its kind and by what the
// attach menu attaches (s-mail-1): a person, a calendar, a task, a note, a
// file, a folder.
var kindIcons = map[string]string{
	"contact": "con", "book": "con", "event": "cal", "calendar": "cal", "task": "task", "note": "note", "file": "doc", "folder": "file",
}

// LetterTitle is the sheet's title and its page's (s-mail-1).
const LetterTitle = letterTitle

// Letter draws the letter (s-mail-1): one form, sent to /app/mail/send.
//
//   - The head: the icon, «Letter», the address it goes from as a tag
//     pushed right, the cross back where it was opened from.
//   - To: the recipients as tags — the book's mark, the name and the
//     address after it muted, the address its hint; one typed by hand its
//     address — each the cross takes off; the line «Name or address» after
//     them, its suggestions where the model says, its Enter sending the
//     form with op=add; the line under it (how many, that all see all).
//   - Subject, the text.
//   - «Attached» with how many and Attach, its menu what can be attached,
//     each sending the form to its own address, a rule before the files;
//     a row a part — its icon, its title, its format and size or «folder»,
//     «attached» or «as a link» on, a link's terms and Change…, the cross
//     —; the line of why some go as links, in the accent.
//   - The foot: Send with Ctrl+Enter — the text takes Enter —, Preview,
//     how it is signed, Cancel with Esc pushed.
func Letter(p *view.MailSheet) w.Sheet {
	var cancel view.Action
	for _, a := range p.Form.Actions {
		if a.ID == cancelID {
			cancel = a
		}
	}
	var body []w.SheetPart
	for _, set := range p.Form.Sets {
		for _, r := range set.Rows {
			switch {
			case r.Kind == view.InputHidden:
			case r.Name == toField:
				body = append(body, recipients(r))
			case r.Name == textField:
				body = append(body, w.Area{Text: string(r.Value), Variant: w.AreaLetter, Name: r.Name, Label: string(r.Label)})
			default:
				body = append(body, bind.Field(r, w.FieldW100))
			}
		}
	}
	body = append(body, attachments(p))

	var foot []w.SheetFootPart
	for _, a := range p.Form.Actions {
		switch a.ID {
		case sendID:
			b := bind.Submit(a)
			if a.Href != p.Form.Action {
				b.To = a.Href
			}
			b.Key = ctrlEnter // U-7: Send a letter
			foot = append(foot, b, w.Kbd{Keys: ctrlEnter})
		case previewID:
			b := bind.Submit(a)
			b.To = a.Href
			foot = append(foot, b)
		}
	}
	if len(p.ReplyNote) > 0 {
		foot = append(foot, w.Dim{Parts: bind.Inlines(p.ReplyNote), Size: w.Dim11_5, Variant: w.DimWrap})
	}
	foot = append(foot, w.Line{Items: []w.LinePart{bind.Button(cancel, ""), w.Kbd{Keys: escKey}}, Gap: w.LineGap7, Place: w.Pushed})
	return w.Sheet{
		ID: letterID, Icon: mailIcon, Title: letterTitle,
		Head:   []w.SheetHeadPart{w.Tag{Label: fromWord + string(p.From), On: true, Place: w.Pushed}, w.Close{Label: closeWord, Href: cancel.Href}},
		Body:   body,
		Foot:   foot,
		Width:  w.SheetWidth680,
		Action: p.Form.Action, Fields: bind.Hidden(p.Form),
	}
}

// recipients is To (s-mail-1, -3): the tags of those chosen, the line a
// name or an address is typed into, the line under them.
func recipients(r view.FormRow) w.Field {
	in := w.TagInput{Name: r.Name, Focus: true}
	for _, o := range r.Options {
		if !o.Selected {
			continue
		}
		tag := w.Tag{Label: string(o.Label), Removable: true, Title: o.Value, Name: r.Name, Value: o.Value}
		if o.Colour != "" {
			tag.Mark = &w.Mark{Colour: string(o.Colour), Size: w.MarkH11}
		}
		if o.Meta != "" {
			tag.Aside = &w.Dim{Parts: []w.Inline{w.Text(sep[1:] + string(o.Meta))}}
		}
		in.Parts = append(in.Parts, tag)
	}
	if a := r.Add; a != nil {
		in.Add = &w.TagAdd{Name: a.Name, Placeholder: string(a.Placeholder), Label: string(a.Label), Suggest: r.Suggest,
			Enter: &w.TagEnter{Name: enterName, Value: enterValue, Label: enterWord, Of: letterID}}
	}
	f := w.Field{Label: string(r.Label), Parts: []w.FieldPart{w.Drop{Trigger: in, Menu: w.Menu{Items: []w.MenuPart{}, Width: w.MenuWidth420, Pop: w.MenuUnder}}}, Width: w.FieldW100}
	if r.Hint != "" {
		f.Parts = append(f.Parts, w.FieldHint{Parts: []w.Inline{w.Text(r.Hint)}})
	}
	return f
}

// attachments is «Attached» (s-mail-1): the rubric with the count and
// Attach, the parts, why some go as links.
func attachments(p *view.MailSheet) w.Sec {
	var items []w.MenuPart
	for _, a := range p.Actions {
		what := a.ID[len(attachMenu):]
		if a.ID == attachFile {
			items = append(items, w.MenuSep{})
		}
		it := w.MenuItem{Icon: w.Icon(kindIcons[what]), Label: string(a.Label), To: a.Href}
		if a.ID == attachTitle {
			// A folder always goes as a link (s-mail-1).
			it.Trail = w.Tag{Label: linkWord, Place: w.Pushed}
		}
		items = append(items, it)
	}
	attach := w.Drop{Trigger: w.Button{Label: attachWord, Menu: true, Icon: clipIcon, Small: true}, Menu: w.Menu{Items: items, Width: w.MenuWidth212, Pop: w.MenuUnderEnd}}
	rubric := w.Rubric{Text: attached, Items: []w.RubricPart{w.Num{Value: len(p.Attach)}, w.Line{Items: []w.LinePart{attach}, Gap: w.LineGap6, Place: w.Pushed}}, Variant: w.RubricRow}
	files := w.SecFiles{}
	for _, r := range p.Attach {
		size := string(r.Size)
		switch {
		case r.Format == "folder":
			size = string(r.Format)
		case r.Format != "" && r.Kind != view.KindFile:
			size = "." + string(r.Format) + sep + size
		}
		f := w.SecFile{Icon: w.Icon(iconOf(r)), Name: string(r.Title), Size: w.Mono{Text: size, Variant: w.MonoDusty11}, Tag: w.Tag{Label: asFile},
			Remove: w.Button{Icon: "close", Small: true, Title: removeWord, To: r.Remove.Href, Submit: true}}
		if r.AsLink {
			f.Tag = w.Tag{Label: asLink, On: true}
			if r.Terms != "" {
				f.Terms = &w.Mono{Text: string(r.Terms), Variant: w.MonoDusty11}
			}
			if r.Change != nil {
				f.Change = &w.Button{Label: string(r.Change.Label), Small: true, To: r.Change.Href, Submit: true}
			}
		}
		files.Items = append(files.Items, f)
	}
	sec := w.Sec{Items: []w.SecPart{rubric, files}, Flush: true}
	if len(p.Links) > 0 {
		var parts []w.MsgPart
		for _, in := range bind.Inlines(p.Links) {
			if mp, ok := in.(w.MsgPart); ok {
				parts = append(parts, mp)
			}
		}
		sec.Items = append(sec.Items, w.Msg{Parts: parts, Tone: w.MsgAccent, Space: w.MsgFlushTop11})
	}
	return sec
}

// iconOf is the icon of a part by its kind: a folder's its own.
func iconOf(r view.MailAttachRow) string {
	switch {
	case r.Format == "folder":
		return kindIcons["folder"]
	case r.Kind == view.KindContact:
		return kindIcons["contact"]
	case r.Kind == view.KindEvent:
		return kindIcons["event"]
	case r.Kind == view.KindTask:
		return kindIcons["task"]
	case r.Kind == view.KindNote:
		return kindIcons["note"]
	}
	return kindIcons["file"]
}

// linkWord is the tag of Folder in the attach menu: it goes as a link.
const linkWord = "link"
