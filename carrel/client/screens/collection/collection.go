// Package collection is the four sheets of a collection (C-5.6, U-6.10):
// New collection, Rename and colour, Move or Copy to another server, Delete
// on the server — /app/collections/{new,rename,move,delete} — drawn by
// frames s-profile-5, -7, -8, -9 and their states (Typecase's note 112). They
// are opened from Connections (Р-75), and the screen under each is the page
// it was opened from (Under), drawn by that page's screen — the registry
// puts the two together (screens.over).
package collection

import (
	"slices"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The screens' words (U-1.3): the address beside its field, the keys of a
// sheet's two answers.
const (
	addressLabel = "Address"
	feedLabel    = "Feed"
	stays        = "— stays as it is"
	enter        = "Enter"
	esc          = "Esc"
)

// Rename draws «Rename and colour» (s-profile-7): a form
// (CollectionRenameSheet.Form) that writes the name and the colour.
//
//   - The head: the pencil, the title («Rename and colour», «Rename» for a
//     book or a folder), the server — the storage and the folder for a
//     folder, «feed» for a subscription —, muted, pushed right (Stamp).
//   - A refusal of the server over the fields, in alarm, its answer under
//     it (Failure; s-profile-30): the fields keep what was typed.
//   - A feed's address first, read-only («Feed», s-profile-33).
//   - Name; Colour as swatches; each with its line under it.
//   - The address, read-only, «— stays as it is» (s-profile-7) — none for a
//     folder, whose name is its address (s-profile-18).
//   - What renaming does and does not (Note), but over a refusal.
//   - The foot: Save (Rename for a folder) — the form's button — with
//     Enter, but for a folder; Cancel with Esc.
func Rename(p *view.CollectionRenameSheet) w.Sheet {
	folder := p.Kind == view.KindFile
	s := w.Sheet{Icon: "pencil", Title: string(p.Title), Width: w.SheetWidth520,
		Action: p.Form.Action, Method: p.Form.Method, Fields: bind.Hidden(p.Form)}
	if folder || p.Feed {
		s.Width = w.SheetWidth480
	}
	if p.Stamp != "" {
		s.Head = []w.SheetHeadPart{w.Dim{Parts: []w.Inline{w.Text(p.Stamp)}, Size: w.Dim11_5, Variant: w.DimWrap, Place: w.Pushed}}
	}
	if p.Failure != nil {
		s.Body = append(s.Body, refusal(*p.Failure)...)
	}
	if p.Feed && p.Address != "" {
		s.Body = append(s.Body, readOnly(feedLabel, "", p.Address))
	}
	for _, set := range p.Form.Sets {
		for _, r := range set.Rows {
			if r.Kind == view.InputHidden {
				continue
			}
			f := field(r)
			if p.Failure != nil {
				// What was typed and refused is shown, not taken up again
				// (s-profile-30).
				f = unfocused(f)
			}
			s.Body = append(s.Body, f)
		}
	}
	if !p.Feed && p.Address != "" {
		s.Body = append(s.Body, readOnly(addressLabel, stays, p.Address))
	}
	if p.Note != "" && p.Failure == nil {
		space := w.MsgFlushTop2
		if folder {
			space = w.MsgFlush
		}
		s.Body = append(s.Body, w.Msg{Parts: []w.MsgPart{w.Text(p.Note)}, Space: space})
	}
	s.Foot = foot(p.Actions, !folder)
	return s
}

// field is a row of a sheet's form as its field, the whole width: a colour
// as swatches, a line under it small and dusty (s-profile-33).
func field(r view.FormRow) w.Field {
	if r.Kind == view.InputRadio && r.Name == "colour" {
		sw := w.Swatches{Name: r.Name}
		for i, o := range r.Options {
			sw.Colours = append(sw.Colours, o.Value)
			if o.Selected {
				sw.On = i
			}
		}
		f := w.Field{Label: string(r.Label), Parts: []w.FieldPart{sw}, Width: w.FieldW100}
		return withHint(f, r)
	}
	f := bind.Field(r, w.FieldW100)
	f.Parts = f.Parts[:1]
	return withHint(f, r)
}

// withHint puts a row's line under its field: an error in its tone, else
// what the value means, small and dusty; the part an address takes from
// the name follows it as it is typed (SlugOf, s-profile-5).
func withHint(f w.Field, r view.FormRow) w.Field {
	if r.Error == "" && slices.ContainsFunc(r.HintRuns, func(run view.Run) bool { return run.Mark == view.RunSlug }) {
		var parts []w.Inline
		for _, run := range r.HintRuns {
			if run.Mark == view.RunSlug {
				parts = append(parts, w.SlugOf{Of: r.Name, Text: string(run.Text)})
				continue
			}
			parts = append(parts, bind.Inlines(view.Runs{run})...)
		}
		f.Parts = append(f.Parts, w.FieldHint{Parts: parts, Variant: w.HintSmall})
		return f
	}
	for _, h := range bind.Hint(r) {
		if fh, ok := h.(w.FieldHint); ok && fh.Variant == w.HintPlain {
			fh.Variant = w.HintSmall
			h = fh
		}
		f.Parts = append(f.Parts, h)
	}
	return f
}

// unfocused is a field whose input does not take the focus.
func unfocused(f w.Field) w.Field {
	for i, part := range f.Parts {
		if in, ok := part.(w.Input); ok {
			in.Focus = false
			f.Parts[i] = in
		}
	}
	return f
}

// readOnly is an address shown and not sent: monospaced, dusty, with no
// name (C-5.6, note 112 p. 3).
func readOnly(label, aside string, value view.Text) w.Field {
	f := w.Field{Label: label, Width: w.FieldW100,
		Parts: []w.FieldPart{w.Input{Value: string(value), Label: label, Mono: true, ReadOnly: true, Variant: w.InputDusty}}}
	if aside != "" {
		f.LabelAside = w.Dim{Parts: []w.Inline{w.Text(aside)}}
	}
	return f
}

// refusal is a server's refusal in a sheet (s-profile-6, -30): what did not
// happen in alarm, and the server's answer under it a line each.
func refusal(n view.Notice) []w.SheetPart {
	out := []w.SheetPart{bind.Msg(n, w.MsgFlush)}
	if n.Details != nil && len(n.Details.Lines) > 0 {
		text := ""
		for i, l := range n.Details.Lines {
			if i > 0 {
				text += "\n"
			}
			text += string(l)
		}
		out = append(out, w.Diag{Text: text})
	}
	return out
}

// foot is a sheet's foot: the main answer — the form's button — with
// Enter when the frame keys it, and the way out pushed right with Esc.
func foot(actions []view.Action, keyed bool) []w.SheetFootPart {
	var out []w.SheetFootPart
	for _, a := range actions {
		if a.Method == "GET" && !a.Primary {
			out = append(out, w.Line{Items: []w.LinePart{bind.Button(a, ""), w.Kbd{Keys: esc}}, Gap: w.LineGap7, Place: w.Pushed})
			continue
		}
		b := bind.Submit(a)
		if a.Method == "GET" {
			b = bind.Button(a, "")
		}
		if !a.Enabled {
			b.State, b.Reason = w.ButtonDisabled, string(a.Reason)
		}
		out = append(out, b)
		if a.Primary && keyed {
			out = append(out, w.Kbd{Keys: enter})
		}
	}
	return out
}

// pointing is the rubric over what points at a collection (s-profile-9).
const pointing = "What is pointing at it now"

// Delete draws «Delete on the server» (s-profile-9; a folder, -19): a form
// (CollectionDeleteSheet.Form) confirmed by typing the name.
//
//   - The head: the bin in alarm, the title, the server and the collection,
//     muted, pushed right (Subject).
//   - What goes and that Carrel holds no copy, in alarm, the count in bold;
//     a calendar off a server that schedules ends with what its attendees
//     may hear (Warning).
//   - What points at it — its links, backup jobs, devices, the attachment
//     folder —, each in bold and what becomes of it muted (References).
//   - When it was last in an archive and what a restore does not do
//     (Archive).
//   - Download ‹.ics› first — Open in Files for a folder — with what it
//     gives, muted (Export, ExportNote).
//   - The name to type; Delete on the server — the form's button, the solid
//     danger — and Cancel with Esc. The sheet is 480 wide for a folder.
func Delete(p *view.CollectionDeleteSheet) w.Sheet {
	s := w.Sheet{Icon: "trash", Tone: w.SheetIconDanger, Title: string(p.Title),
		Action: p.Form.Action, Method: p.Form.Method, Fields: bind.Hidden(p.Form)}
	if p.Kind == view.KindFile {
		s.Width = w.SheetWidth480
	}
	if p.Subject != "" {
		s.Head = []w.SheetHeadPart{w.Dim{Parts: []w.Inline{w.Text(p.Subject)}, Size: w.Dim11_5, Variant: w.DimWrap, Place: w.Pushed}}
	}
	if len(p.Warning) > 0 {
		s.Body = append(s.Body, w.Msg{Parts: msgParts(p.Warning), Tone: w.MsgAlert, Space: w.MsgFlush})
	}
	if len(p.References) > 0 {
		b := w.SheetBlock{Title: w.Rubric{Text: pointing, Variant: w.RubricBottom7}}
		for _, r := range p.References {
			b.Items = append(b.Items, w.SheetLine{Name: string(r.What), Note: w.Dim{Parts: []w.Inline{w.Text("— " + string(r.Then))}}})
		}
		s.Body = append(s.Body, b)
	}
	if len(p.Archive) > 0 {
		s.Body = append(s.Body, w.Msg{Parts: msgParts(p.Archive), Space: w.MsgFlush})
	}
	if p.Export != nil {
		line := w.Line{Items: []w.LinePart{exportButton(*p.Export)}, Gap: w.LineGap8, Block: true}
		if p.ExportNote != "" {
			line.Items = append(line.Items, w.Dim{Parts: []w.Inline{w.Text(p.ExportNote)}, Size: w.Dim11_5, Variant: w.DimWrap})
		}
		s.Body = append(s.Body, line)
	} else if p.ExportNote != "" {
		s.Body = append(s.Body, w.Dim{Parts: []w.Inline{w.Text(p.ExportNote)}, Size: w.Dim11_5, Variant: w.DimWrap})
	}
	for _, set := range p.Form.Sets {
		for _, r := range set.Rows {
			if r.Kind != view.InputHidden {
				s.Body = append(s.Body, field(r))
			}
		}
	}
	s.Foot = foot(p.Actions, false)
	return s
}

// msgParts are runs as the parts of a message, each in its face.
func msgParts(r view.Runs) []w.MsgPart {
	var out []w.MsgPart
	for _, in := range bind.Inlines(r) {
		if mp, ok := in.(w.MsgPart); ok {
			out = append(out, mp)
		}
	}
	return out
}

// exportButton is «Download ‹.ics› first» (s-profile-9): the extension in
// its words monospaced and small, the word after it after it. A label with
// no extension is the button's own words (Open in Files, -19).
func exportButton(a view.Action) w.Button {
	b := bind.Button(a, "")
	label := string(a.Label)
	start := -1
	for i := 1; i < len(label); i++ {
		if label[i] == '.' && label[i-1] == ' ' {
			start = i
			break
		}
	}
	if start < 0 {
		return b
	}
	end := start
	for end < len(label) && label[end] != ' ' {
		end++
	}
	b.Label = label[:start-1]
	b.Aside = &w.Mono{Text: label[start:end], Variant: w.Mono11}
	if end < len(label) {
		b.After = label[end+1:]
	}
	return b
}

// writeOnto is the rubric over the servers a collection can go to
// (s-profile-8).
const writeOnto = "Write onto"

// Move draws «Move calendar» (s-profile-8; a folder, -34; the whole
// storage, -35): a form (CollectionMoveSheet.Form) that sends the server
// chosen.
//
//   - The head: the icon, the title («Move calendar», «Copy folder»), the
//     server and the collection, muted, pushed right (Subject).
//   - Under «Write onto», every connection as a radio under Target.Field:
//     the one it is on — «already there» —, one that cannot take it with
//     why, the others with what they hold, the chosen one on. Each radio's
//     address is the sheet with it chosen (TargetOption.Href).
//   - What the move does, the count in bold (Warning); for the whole
//     storage, what the server may not let delete (Stays).
//   - The foot: Move or Copy — the form's button — with Enter, Cancel with
//     Esc.
func Move(p *view.CollectionMoveSheet) w.Sheet {
	s := w.Sheet{Icon: "move", Title: string(p.Title), Width: w.SheetWidth520,
		Action: p.Form.Action, Method: p.Form.Method, Fields: bind.Hidden(p.Form)}
	if p.Subject != "" {
		s.Head = []w.SheetHeadPart{w.Dim{Parts: []w.Inline{w.Text(p.Subject)}, Size: w.Dim11_5, Variant: w.DimWrap, Place: w.Pushed}}
	}
	tree := w.Tree{}
	for i, o := range p.Target.Options {
		row := w.TreeRow{Icon: "server", Label: string(o.Label), Unavailable: o.ReadOnly,
			Send: w.Send{Name: p.Target.Field, Value: o.Value, One: true}, Href: o.Href}
		trail := string(o.Stamp)
		if o.ReadOnly {
			trail = string(o.Reason)
		}
		if trail != "" {
			row.Trail = w.Mono{Text: trail, Variant: w.Mono10Dusty}
		}
		if o.ID == p.Target.Chosen {
			tree.On = i + 1
		}
		tree.Rows = append(tree.Rows, row)
	}
	s.Body = append(s.Body, w.Rubric{Text: writeOnto}, tree)
	if len(p.Warning) > 0 {
		s.Body = append(s.Body, w.Msg{Parts: msgParts(p.Warning), Space: w.MsgFlushTop2})
	}
	if p.Stays != "" {
		s.Body = append(s.Body, w.Msg{Parts: []w.MsgPart{w.Text(p.Stays)}, Space: w.MsgFlush})
	}
	s.Foot = foot(p.Actions, true)
	return s
}

// New draws «New calendar» (s-profile-5) and its states: a form
// (CollectionNewSheet.Form) that makes a collection on the server chosen.
//
//   - The head: the plus, the title by the kind («New collection» without
//     one, s-profile-29), the server muted and pushed right where the sheet
//     names one (Stamp: the account's card, a refusal).
//   - Nowhere to create (s-profile-21, -28): the message in alarm and no
//     fields — the sheet is no form —, «Try again» or «Connect account».
//   - A refusal (s-profile-6): what did not happen in alarm, the server's
//     answer under it, what it means muted (Explain); what was typed rides
//     hidden with «Back to the form» and «Try again».
//   - Else the fields, the whole width: the server, what it is (a choice of
//     two, s-profile-29), the name, the colour, what goes in it, the
//     address — each with its line under it.
//   - The foot: Create — the form's button — with Enter, Cancel with Esc.
//
// The sheet is 520 wide without a kind, 480 for a folder or with nothing to
// ask, else its own.
func New(p *view.CollectionNewSheet) w.Sheet {
	s := w.Sheet{Icon: "plus", Title: string(p.Title)}
	if p.Stamp != "" {
		s.Head = []w.SheetHeadPart{w.Dim{Parts: []w.Inline{w.Text(p.Stamp)}, Size: w.Dim11_5, Variant: w.DimWrap, Place: w.Pushed}}
	}
	if p.Form.Action == "" {
		// Nothing to send (s-profile-21, -28).
		s.Width = w.SheetWidth480
		if p.Note != "" {
			s.Body = append(s.Body, w.Msg{Parts: []w.MsgPart{w.Text(p.Note)}, Tone: w.MsgAlert, Space: w.MsgFlush})
		}
		s.Foot = foot(p.Actions, false)
		return s
	}
	s.Action, s.Method, s.Fields = p.Form.Action, p.Form.Method, bind.Hidden(p.Form)
	switch {
	case p.Kind == view.KindFile:
		s.Width = w.SheetWidth480
	case kindAsked(p.Form):
		s.Width = w.SheetWidth520
	}
	if p.Failure != nil {
		// A refusal is the sheet's content, its icon in alarm (s-profile-6).
		s.Tone = w.SheetIconDanger
		s.Body = append(s.Body, refusal(*p.Failure)...)
		if p.Explain != "" {
			s.Body = append(s.Body, w.SheetNote{Text: string(p.Explain)})
		}
		s.Foot = foot(p.Actions, false)
		return s
	}
	derived := map[string]bool{}
	for _, set := range p.Form.Sets {
		for _, r := range set.Rows {
			if r.From != "" {
				derived[r.From] = true
			}
		}
	}
	for _, set := range p.Form.Sets {
		for _, r := range set.Rows {
			if r.Kind == view.InputHidden {
				continue
			}
			f := newField(r)
			if derived[r.Name] {
				// The field another is derived from is found by its id as
				// well as its name (Typecase's task 224).
				f = withID(f, r.Name)
			}
			s.Body = append(s.Body, f)
		}
	}
	s.Foot = foot(p.Actions, true)
	return s
}

// withID gives a field's input an id.
func withID(f w.Field, id string) w.Field {
	for i, part := range f.Parts {
		if in, ok := part.(w.Input); ok {
			in.ID = id
			f.Parts[i] = in
		}
	}
	return f
}

// kindAsked says whether the sheet asks what the collection is.
func kindAsked(f view.Form) bool {
	for _, set := range f.Sets {
		for _, r := range set.Rows {
			if r.Name == "kind" {
				return true
			}
		}
	}
	return false
}

// newField is a row of New collection as its field: what it is as a choice
// of two (Seg), what goes in it as ticks, the rest as the other sheets draw
// them (field).
func newField(r view.FormRow) w.Field {
	switch {
	case r.Kind == view.InputRadio && r.Name != "colour":
		seg := w.Seg{Name: r.Name, Variant: w.SegStart}
		for _, o := range r.Options {
			seg.Options = append(seg.Options, w.SegOption{Label: string(o.Label), Value: o.Value, On: o.Selected, Disables: o.Disables})
		}
		return withHint(w.Field{Label: string(r.Label), Parts: []w.FieldPart{seg}, Width: w.FieldW100}, r)
	case r.Kind == view.InputCheck && len(r.Options) > 0:
		g := w.Group{}
		for _, o := range r.Options {
			g.Items = append(g.Items, w.Check{On: o.Selected, Name: r.Name, Value: o.Value, Parts: []w.CheckPart{w.Text(o.Label)}})
		}
		return withHint(w.Field{Label: string(r.Label), Parts: []w.FieldPart{g}, Width: w.FieldW100}, r)
	}
	return field(r)
}
