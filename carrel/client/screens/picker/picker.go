// Package picker is the sheet that chooses a folder or a file of Files,
// /app/files/picker — where an upload goes (?for=upload), a file to attach to
// a record (?for=attach) —, drawn by frame «Upload с выбором папки» of the
// mockup (s-notes-11) and its states, From Files (s-files-16), New folder
// refused (s-files-19). A sheet carries no shell: the screen draws it over
// nothing, which is the sheet's own page without a script (C-7.2, U-8), and
// it is held against the frame's sheet (guard/sheets_test.go).
package picker

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// formSheet is the ID of the sheet of an upload, which is the upload's form
// (s-notes-11): the files picked in it go with its button; a node's link
// swaps it with the live layer.
const formSheet = "upload-sheet"

// The screen's words (U-1.3): the rubric of the tree, the name of the
// choice of a server, the keys of the foot, the cross's name, the place a
// file is dropped in and its words, a picked file's Remove, the name of the
// file input, where the picked files are listed, the field of their sizes.
const (
	where     = "Where to put it"
	server    = "Server"
	enter     = "Enter"
	esc       = "Esc"
	dropHere  = "Drop files here, or "
	choose    = "choose on the computer"
	taken     = "A name already taken is refused, not overwritten"
	remove    = "Remove"
	files     = "Files"
	listID    = "upload-files"
	sizeField = "size"
)

// The icons of the sheet by what it chooses for: an upload's folder, a file
// to attach (s-notes-11, s-files-16).
const (
	uploadIcon = "upload"
	attachIcon = "clip"
)

// tree is what the tree of folders takes from the screen (bind.TreeWords).
var tree = bind.TreeWords{Folder: "file", File: "doc", ReadOnly: "read-only", Attachments: "attachment folder", Fresh: "just created", Sep: " · "}

// The names of the contract the screen places by (C-7.5): the sheet of an
// upload has Upload, of From Files Attach.
const (
	uploadID = "files.picker.upload"
	attachID = "files.picker.attach"
)

// action is the action of the ID among actions, if there is one.
func action(actions []view.Action, id string) (view.Action, bool) {
	for _, a := range actions {
		if a.ID == id {
			return a, true
		}
	}
	return view.Action{}, false
}

// Screen draws the sheet from its view model, in the state the model is in,
// over nothing.
//
//   - An upload's (s-notes-11, s-files-19): a form that sends its files to
//     the folder chosen (Upload's address); the head — the icon, the title,
//     the choice of a server, the folder chosen, the cross —; the place a
//     file is dropped in, whose words pick files, and the list of those
//     picked; the card of the tree — its rubric with New folder, or the line
//     of the new folder's name and why New folder made nothing — each node a
//     link to the sheet with it chosen; why this folder (Purpose); the foot —
//     Upload, Enter, Cancel.
//   - From Files (s-files-16): the head — the icon, the title, the choice of
//     a server, what the file is for —; why (Purpose); the tree with the
//     files of the open folder, each its size and where it lands; the foot —
//     Attach, off until a file is chosen and saying so, Cancel.
func Screen(p *view.FilePickerSheet) w.Overlay {
	var cancel *w.Button
	for _, a := range p.Actions {
		if a.Method == "GET" {
			b := bind.Button(a, "")
			cancel = &b
		}
	}
	s := w.Sheet{Title: string(p.Title)}
	up, uploading := action(p.Actions, uploadID)
	if uploading {
		s.Icon, s.ID = uploadIcon, formSheet
		if up.Enabled {
			s.Action = up.Href
			if up.CSRF != "" {
				s.Fields = []w.Hidden{{Name: "_csrf", Value: up.CSRF}}
			}
		}
		// No cross: a sheet of its own address closes by Cancel (s-notes-11,
		// note 83 of Typecase).
		s.Head = append(serverPick(p.Servers, formSheet), w.Dim{Parts: []w.Inline{w.Text(p.Where)}, Size: w.Dim11_5, Variant: w.DimWrap})
		if p.Tree.Name == nil && p.Failure == nil {
			s.Body = append(s.Body, drop(up))
		}
		s.Body = append(s.Body, w.Card{Variant: w.CardFlush, Items: treeParts(p)})
		if p.Purpose != "" {
			s.Body = append(s.Body, w.Dim{Parts: []w.Inline{w.Text(p.Purpose)}, Size: w.Dim11_5, Variant: w.DimWrap})
		}
	} else {
		s.Icon, s.Width = attachIcon, w.SheetWidth520
		// The attach sheet has no ID of its own to swap: its servers are
		// plain links (s-files-16).
		s.Head = serverPick(p.Servers, "")
		if p.Subject != "" {
			d := w.Dim{Parts: []w.Inline{w.Text(p.Subject)}, Size: w.Dim11_5, Variant: w.DimWrap}
			if len(s.Head) == 0 {
				d.Place = w.Pushed
			}
			s.Head = append(s.Head, d)
		}
		if p.Purpose != "" {
			s.Body = append(s.Body, w.SheetNote{Text: string(p.Purpose)})
		}
		if p.Failure != nil {
			s.Body = append(s.Body, bind.Msg(*p.Failure, w.MsgFlushTop2))
		}
		// The open folder is where the files are shown, not a choice: a row
		// is on once a file is chosen (s-files-16).
		t := bind.FileTree(p.Tree, tree)
		if t.On > 0 && t.Rows[t.On-1].Icon != tree.File {
			t.On = 0
		}
		s.Body = append(s.Body, t)
	}
	s.Foot = foot(p.Actions, cancel, uploading)
	return w.Overlay{Scrim: w.Scrim{Content: s, Pad: w.ScrimPad26}}
}

// serverPick is the choice of a server in the head: the one on.
func serverPick(servers []view.FileServer, swap string) []w.SheetHeadPart {
	if len(servers) == 0 {
		return nil
	}
	return []w.SheetHeadPart{bind.ServerDrop(servers, server, swap)}
}

// drop is the place a file is dropped in (s-notes-11): its words, of which
// «choose on the computer» picks files — the file input of the sheet, the
// list of those picked, a row each with its size and Remove —, what a name
// already taken does, and the list.
func drop(up view.Action) w.Empty {
	row := &w.Line{Gap: w.LineGap8, Place: w.Grown, Items: []w.LinePart{
		w.Faced{Face: w.FaceLarge, Items: []w.LinePart{w.Text("{name} "), w.Dim{Parts: []w.Inline{w.Text("{size}")}}}},
		w.Button{Label: remove, Kind: w.ButtonDanger, Small: true, Place: w.Pushed, Removes: true},
	}}
	pick := w.FilePick{Label: choose, File: w.FileField{Form: formSheet, Name: up.File, Label: files, List: listID, Size: sizeField, Many: true}, Row: row}
	return w.Empty{Variant: w.EmptyDashed, Parts: []w.EmptyPart{
		w.Text(dropHere), pick, w.Text(" "),
		w.Aside{Face: w.FaceSmall, Parts: []w.Inline{w.Text(taken)}}, w.Text(" "),
		w.Line{Gap: w.LineGap8, Block: true, ID: listID, Place: w.Grown},
	}}
}

// treeParts are the card of an upload's tree: its rubric with New folder, or
// the line of the new folder's name and why New folder made nothing, and the
// tree.
func treeParts(p *view.FilePickerSheet) []w.CardPart {
	rubric := w.Rubric{Text: where}
	if p.Tree.New != nil && p.Tree.Name == nil {
		b := bind.Button(*p.Tree.New, "")
		b.Small = true
		rubric.Items = []w.RubricPart{b}
	}
	out := []w.CardPart{rubric}
	if p.Tree.Name != nil {
		out = append(out, nameLine(*p.Tree.Name))
	}
	// A refusal of New folder in the card stands 8 px from the line of the
	// name and from the tree (s-files-19, note 83 of Typecase).
	if p.Failure != nil {
		out = append(out, bind.Msg(*p.Failure, w.MsgFlushY8))
	}
	t := bind.FileTree(p.Tree, tree)
	for i := range t.Rows {
		if t.Rows[i].Href != "" {
			t.Rows[i].Swap = formSheet
		}
	}
	return append(out, t)
}

// nameLine is the line of the new folder's name over the tree (FileTree.Name,
// s-files-19): its field with the cursor, filling the line; Create, which
// sends the sheet to the address that makes the folder; Cancel.
func nameLine(f view.Form) w.Line {
	line := w.Line{Gap: w.LineGap8}
	for _, r := range bind.Rows(f) {
		if r.Kind == view.InputHidden {
			continue
		}
		in := bind.Input(r)
		in.Variant = w.InputFill
		line.Items = append(line.Items, in)
	}
	for _, a := range f.Actions {
		b := bind.Button(a, "")
		if a.Method == "POST" {
			b = bind.Submit(a)
			b.To = f.Action
		}
		b.Small = true
		line.Items = append(line.Items, b)
	}
	return line
}

// Forms are the forms of the sheet's buttons that post with none of their
// own: Attach.
func Forms(p *view.FilePickerSheet) []w.PostForm {
	var out []w.PostForm
	for _, a := range p.Actions {
		if a.ID != uploadID && a.Method == "POST" && a.Enabled {
			out = append(out, bind.ActionForm(a, a.ID))
		}
	}
	return out
}

// foot is the sheet's foot: Upload — the sheet's submit — or Attach, off
// with why after its Enter while no file is chosen; Cancel and Esc at the
// end.
func foot(actions []view.Action, cancel *w.Button, uploading bool) []w.SheetFootPart {
	var out []w.SheetFootPart
	for _, a := range actions {
		if a.Method != "POST" {
			continue
		}
		b := bind.Button(a, a.ID)
		if uploading && a.Enabled {
			b = bind.Submit(a)
			b.To = ""
		}
		out = append(out, b, w.Kbd{Keys: enter})
		if !a.Enabled && a.Reason != "" {
			out = append(out, w.Dim{Parts: []w.Inline{w.Text(a.Reason)}, Size: w.Dim11_5})
		}
	}
	if cancel != nil {
		out = append(out, w.Line{Gap: w.LineGap7, Place: w.Pushed, Items: []w.LinePart{*cancel, w.Kbd{Keys: esc}}})
	}
	return out
}
