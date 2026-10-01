// Package contactedit is the edit of a contact,
// /app/contacts/{account}/{col}/{uid}/edit, and the full form of a new one,
// /app/contacts/{account}/{col}/new — one view model (ContactEditPage) —,
// drawn by frame «Контакт · правка, фотография» (s-contacts-3) of the
// mockup, the edit without a photo (s-contacts-4) and the new card's form
// (s-contacts-27): the drafts frame2go printed for them, with the frames'
// data become the view model's (U-5.1, U-6.1).
package contactedit

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// formID names the card's form for Save, which stands in the head, outside
// it (Button.Form).
const formID = "edit"

// none is the screen's word (U-1.3) in an empty field the model gives no
// pattern for: the dash the frames draw as its placeholder (s-contacts-3,
// -4, -27).
const none = "—"

// field is a row of the form as a field of the width of its name, an empty
// line showing the dash where the model gives no pattern of its own — but
// the one with the cursor, which the frames draw bare (s-contacts-27).
func field(r view.FormRow) w.Field {
	f := bind.Field(r, width(r.Name))
	if in, ok := f.Parts[0].(w.Input); ok && in.Placeholder == "" && !in.Focus {
		in.Placeholder = none
		f.Parts[0] = in
	}
	return f
}

// The screen's words (U-1.3) for the shell: the mark, the search, the rail.
var (
	top = bind.TopWords{
		Brand:  w.Brand{Logo: &w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63"}},
		Search: "Search everything…",
	}
	rail = bind.RailWords{Toggle: "toggle", None: "—", ReadOnly: "ro", Tags: "Tags", Davloom: "davloom"}
)

// widths are the widths of the form's fields by the last part of their name
// (C-6.1: the view model carries no widths; the layout is the screen's, from
// its frame): «tel.0.type» is a type, «adr.0.street» a street.
var widths = map[string]w.FieldWidth{
	"fn": w.FieldXL, "given": w.FieldSM, "family": w.FieldSM, "nickname": w.FieldSM,
	"org": w.FieldMD, "title": w.FieldMD, "bday": w.FieldSM,
	"type": w.FieldXS, "tel": w.FieldMD, "email": w.FieldLG,
	"street": w.FieldLG, "city": w.FieldSM, "region": w.FieldSM, "code": w.FieldXS, "country": w.FieldSM,
	"categories": w.FieldLG, "note": w.FieldFull,
}

// width is the width of a field by its name: its last part that is no
// number, «tel.0» is a phone.
func width(name string) w.FieldWidth {
	var parts []string
	start := 0
	for i := 0; i <= len(name); i++ {
		if i == len(name) || name[i] == '.' {
			parts = append(parts, name[start:i])
			start = i + 1
		}
	}
	for i := len(parts) - 1; i >= 0; i-- {
		if fw, ok := widths[parts[i]]; ok {
			return fw
		}
	}
	return w.FieldAuto
}

// Screen draws the edit from its view model, in the state the model is in.
//
//   - The shell: the top bar; the rail of return — «Back to the card» (or
//     «Back to list» for a new card), the book's sources, the foot.
//   - The head: the name (a new card's «New contact»), the book, Cancel and
//     Save, which submits the form by its name.
//   - What the server lost of the last save (Loss): a notice over the form,
//     not a lock (C-9).
//   - The form: the photo with what can be done to it, and what was done
//     to it (Note); the head of the card side by side — Display name with
//     the cursor, Given, Family, Nickname, Organisation, Title, Birthday
//     typed and monospaced with its pattern; then the rubrics — phones,
//     emails, addresses, other —, their labels over the first line only,
//     each line closed by Remove (it sends remove=<line> without scripts),
//     «Add …» adding the lines of its template in place; the folded rubric
//     as its button «Show N empty fields», which unfolds them (Unfold).
//   - Under the form of a card that exists: properties Carrel does not know
//     are kept (U-6.1); a new card has none (s-contacts-27).
func Screen(p *view.ContactEditPage) w.Shell {
	var plain, primary []w.ActsPart
	for _, a := range p.Form.Actions {
		b := bind.Button(a, formID)
		if a.Primary {
			primary = append(primary, b)
		} else {
			plain = append(plain, b)
		}
	}
	h := w.Head{
		Title: &w.HeadTitle{Text: string(p.Name)},
		Acts:  &w.Acts{Items: append(plain, primary...)},
	}
	if p.Subtitle != "" {
		h.Subs = []w.HeadSub{{Parts: []w.HeadSubPart{w.Text(p.Subtitle)}}}
	}
	main := []w.MainPart{h}
	main = append(main, bind.Notices(p.Notices, w.MsgSpacePage)...)
	if p.Loss != nil {
		main = append(main, bind.Loss(*p.Loss, w.MsgFlushTop2))
	}
	form := bind.Form(p.Form, formID)
	form.Items = append(form.Items, photo(p.Photo))
	for i, set := range p.Form.Sets {
		rows := visible(set.Rows)
		switch {
		case set.Folded:
			show := w.Button{Label: string(set.Legend)}
			if set.Unfold != nil {
				show = bind.Button(*set.Unfold, "")
				show.Label = string(set.Legend)
			}
			form.Items = append(form.Items, w.Field{Parts: []w.FieldPart{show}})
		case i == 0 && set.Legend == "":
			cols := w.FormCols{}
			for _, r := range rows {
				cols.Items = append(cols.Items, field(r))
			}
			form.Items = append(form.Items, cols)
		default:
			form.Items = append(form.Items, fieldSet(set.Legend, rows))
		}
	}
	main = append(main, form)
	if p.UID != "" {
		main = append(main, w.FormFoot{Items: []w.FormFootPart{
			w.Dim{Parts: []w.Inline{w.Text("Properties Carrel does not know are kept unchanged.")}, Size: w.Dim11_5},
		}})
	}
	t := bind.Top(p.Top, top)
	return w.Shell{Top: &t, Body: w.ShellBody{Rail: railParts(p.Rail, p.UID == ""), Main: w.Main{Items: main}}}
}

// Forms are the forms of the photo's actions, which post with none of their
// own, each under its action's ID, of the notices' buttons and of the menus'
// items that post (Sign out).
func Forms(p *view.ContactEditPage) []w.PostForm {
	out := append(bind.TopForms(p.Top), bind.NoticeForms(p.Notices)...)
	for _, a := range p.Photo.Actions {
		if a.Method == "POST" && a.Enabled {
			out = append(out, bind.ActionForm(a, a.ID))
		}
	}
	return out
}

// visible are the rows of a set a person sees: all but the hidden ones.
func visible(rows []view.FormRow) []view.FormRow {
	var out []view.FormRow
	for _, r := range rows {
		if r.Kind != view.InputHidden {
			out = append(out, r)
		}
	}
	return out
}

// photo is the photo's field: the picture or the initials, and beside it
// what can be done to it and what was done to it (Note).
func photo(b view.PhotoBlock) w.Field {
	var side []w.FieldPart
	var line []w.LinePart
	for _, a := range b.Actions {
		btn := bind.Button(a, a.ID)
		switch {
		case a.Danger:
			btn.Kind = w.ButtonDanger
			if len(line) > 0 {
				side = append(side, w.Line{Items: line, Gap: w.LineGap6})
				line = nil
			}
			side = append(side, btn)
		default:
			line = append(line, btn)
		}
	}
	if len(line) == 1 {
		side = append(side, line[0].(w.Button))
	} else if len(line) > 1 {
		side = append(side, w.Line{Items: line, Gap: w.LineGap6})
	}
	if b.Note != "" {
		side = append(side, w.FieldHint{Parts: []w.Inline{w.Text(b.Note)}, Variant: w.HintNarrow})
	}
	ph := w.Photo{Side: side, Src: b.Href}
	if b.Href == "" {
		ph.Initials = string(b.Initials)
	}
	return w.Field{Label: "Photo", LabelSpan: true, Parts: []w.FieldPart{ph}, Width: w.FieldFixAuto}
}

// fieldSet is a rubric of the form: its lines, labels over the first only,
// each line closed by its Remove; «Add …» with the template of its lines.
func fieldSet(legend view.Text, rows []view.FormRow) w.FieldSet {
	set := w.FieldSet{Rubric: w.Rubric{Text: string(legend)}}
	first := true
	for _, r := range rows {
		switch r.Kind {
		case view.InputRemove:
			set.Items = append(set.Items, bind.Remove(r), w.Grow{})
			first = false
		case view.InputAdd:
			add, tmpl := bind.Add(r, func(t view.FormRow) []w.FieldSetPart {
				if t.Kind == view.InputRemove {
					return []w.FieldSetPart{bind.Remove(t), w.Grow{}}
				}
				f := field(t)
				f.Label = ""
				return []w.FieldSetPart{f}
			})
			add.Icon, add.Small = "plus", true
			set.Items = append(set.Items, add, tmpl)
		default:
			f := field(r)
			if !first {
				f.Label = ""
			}
			set.Items = append(set.Items, f)
		}
	}
	return set
}

// railParts is the rail of return: the way back — to the card, or to the
// list for a new card, which has the phone's short «Back» (U-1.3) —, the
// book's sources, the foot.
func railParts(r view.Rail, list bool) []w.RailPart {
	nav := w.Nav{}
	for _, n := range r.Nav {
		it := bind.NavItem(n)
		if list {
			it.Short = "Back"
		}
		nav.Items = append(nav.Items, it)
	}
	parts := []w.RailPart{nav}
	parts = append(parts, bind.Sources(r, rail)...)
	return append(parts, bind.Foot(r, rail))
}
