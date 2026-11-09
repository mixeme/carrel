// Package fileop is the sheet of an operation on files,
// /app/files/{account}/{col}/op/{path…} — move, copy, rename, delete, New
// folder —, drawn by frame «Перемещение и копирование» (s-files-4) of the
// mockup and its states: the copy (s-files-11), another storage
// (s-files-12), New folder of the tree refused (s-files-13), the rename
// (s-files-21), the deletion (s-files-22) and of one folder (s-files-27),
// New folder of the head refused (s-files-23). A sheet carries no shell: the screen draws it over nothing,
// which is the sheet's own page without a script (C-7.2, U-8), and it is
// held against the frame's sheet (guard/sheets_test.go).
package fileop

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// openID is «Open the folder» (C-7.5), and openName and openValue what it
// sends (C-5.5: open=1).
const (
	openID    = "file.op.open"
	openName  = "open"
	openValue = "1"
)

// formSheet is the ID of the sheet, which is the form of its operation
// (s-files-4: the tree's nodes are its radios, the foot's buttons its
// submits); a node's link swaps it with the live layer.
const formSheet = "op-sheet"

// The screen's words (U-1.3): the rubric of the tree and of what points at
// what a deletion takes, the name of the choice of a server, the keys of the
// foot, the badge of a node that cannot be chosen, the note of the folder the
// items are in, the icon of a folder.
const (
	where    = "Where to put it"
	pointing = "What is pointing at it now"
	server   = "Server"
	enter    = "Enter"
	esc      = "Esc"
	readOnly = "read-only"
	here     = "the items are here"
	folder   = "file"
)

// icons are the sheet's icons by its operation (C-8 FileOp): the frames'
// move (s-files-4) and copy (s-files-11), the rename's, the deletion's, the
// new folder's.
var icons = map[view.FileOp]w.Icon{view.FileMove: "move", view.FileCopy: "dup", view.FileRename: "pencil", view.FileDelete: "trash", view.FileNewFolder: "plus"}

// Screen draws the sheet from its view model, in the state the model is in,
// over nothing.
//
//   - The head: the operation's icon, its title («Move 3 items»), the
//     choice of a server (move, copy: a pick with its menu of links) and
//     where from.
//   - The body of a move or a copy: «Where to put it» with New folder, or,
//     opened, the line of the new folder's name — its field, Create (sends
//     the sheet to the address that makes the folder), Cancel — and under it
//     why New folder made nothing; the tree, each node a radio of the sheet
//     and a link to the sheet with it chosen, one that cannot be chosen off
//     with its badge; what the operation will do (Warning), a command as
//     code.
//   - The body of a rename or a new folder: the field of the name, then why
//     it was refused or what the new name does.
//   - The body of a deletion: that it cannot be undone, in alarm; what
//     points at it, as a collection's sheet lists it; what nobody looked
//     for.
//   - The foot: the operation's button with its key — a deletion's has
//     none (Р-81) —, Cancel and its key at the end.
func Screen(p *view.FileOpSheet) w.Overlay {
	tree := p.Op == view.FileMove || p.Op == view.FileCopy
	s := w.Sheet{Icon: icons[p.Op], Title: string(p.Title), Width: w.SheetWidth480, ID: formSheet}
	if tree {
		s.Width = w.SheetWidth520
	}
	if p.Op == view.FileDelete {
		s.Tone = w.SheetIconDanger
	}
	if f := p.Form; f != nil {
		s.Action, s.Fields = f.Action, bind.Hidden(*f)
	}
	s.Head = head(p)
	if t := p.Target; t != nil {
		s.Body = append(s.Body, treeParts(*t, p.Failure)...)
	}
	if f := p.Form; f != nil && !tree {
		for _, r := range bind.Rows(*f) {
			s.Body = append(s.Body, bind.Field(r, w.FieldW100))
		}
		if p.Failure != nil {
			s.Body = append(s.Body, bind.Msg(*p.Failure, w.MsgFlush))
		}
	}
	if len(p.Alert) > 0 {
		s.Body = append(s.Body, w.Msg{Parts: bind.MsgParts(p.Alert), Tone: w.MsgAlert, Space: w.MsgFlush})
	}
	if len(p.References) > 0 {
		b := w.SheetBlock{Title: w.Rubric{Text: pointing, Variant: w.RubricBottom7}}
		for _, r := range p.References {
			b.Items = append(b.Items, w.SheetLine{Name: string(r.What), Note: w.Dim{Parts: []w.Inline{w.Text("— " + string(r.Then))}}})
		}
		s.Body = append(s.Body, b)
	}
	if len(p.Warning) > 0 {
		space := w.MsgFlush
		if tree {
			space = w.MsgFlushTop2
		}
		s.Body = append(s.Body, w.Msg{Parts: code(bind.MsgParts(p.Warning)), Space: space})
	}
	s.Foot = foot(p)
	return w.Overlay{Scrim: w.Scrim{Content: s, Pad: w.ScrimPad26}}
}

// head is the sheet's head after its title: the choice of a server — the
// one on, the others links to the sheet over their storages —, where from.
func head(p *view.FileOpSheet) []w.SheetHeadPart {
	var out []w.SheetHeadPart
	if len(p.Servers) > 0 {
		out = append(out, bind.ServerDrop(p.Servers, server, formSheet))
	}
	if p.Subject != "" {
		d := w.Dim{Parts: []w.Inline{w.Text(p.Subject)}, Size: w.Dim11_5, Variant: w.DimWrap}
		if len(out) == 0 {
			d.Place = w.Pushed
		}
		out = append(out, d)
	}
	return out
}

// treeParts are where a move or a copy puts things: the rubric with New
// folder or the line of the new folder's name, why New folder made nothing,
// the tree.
func treeParts(t view.FileTree, failure *view.Notice) []w.SheetPart {
	rubric := w.Rubric{Text: where}
	if t.New != nil && t.Name == nil {
		b := bind.Button(*t.New, "")
		b.Small = true
		rubric.Items = []w.RubricPart{b}
	}
	out := []w.SheetPart{rubric}
	if t.Name != nil {
		out = append(out, nameLine(*t.Name))
	}
	if failure != nil {
		out = append(out, bind.Msg(*failure, w.MsgFlushTop2))
	}
	return append(out, nodes(t))
}

// nameLine is the line of the new folder's name over the tree (FileTree.Name,
// s-files-13): its field with the cursor, filling the line; Create, which
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

// nodes is the tree of folders the sheet chooses in, depth first: each node
// a folder by its name, its depth its indent, a radio of the sheet's field
// (FileTree.Field) and a link to the sheet with it chosen, which the live
// layer swaps in; one that cannot be chosen is off with its badge; the folder
// the items are in says so, chosen or not (FileTreeNode.Here, s-files-4,
// -11, -13); the one chosen on.
func nodes(t view.FileTree) w.Tree {
	out := w.Tree{}
	indents := []w.TreeIndent{w.TreeLevel0, w.TreeLevel22, w.TreeLevel40, w.TreeLevel58, w.TreeLevel76}
	var walk func(ns []view.FileTreeNode, depth int)
	walk = func(ns []view.FileTreeNode, depth int) {
		for _, n := range ns {
			row := w.TreeRow{Icon: folder, Label: n.Label, Indent: indents[min(depth, len(indents)-1)]}
			if t.Field != "" {
				row.Send = w.Send{Name: t.Field, Value: n.ID, One: true}
			}
			if n.ReadOnly {
				row.Unavailable, row.Trail = true, w.Badge{Label: readOnly}
			} else if n.Href != "" {
				row.Href, row.Swap = n.Href, formSheet
			}
			if n.Here {
				row.Trail = w.TreeNote{Text: here}
			}
			out.Rows = append(out.Rows, row)
			if n.ID != "" && n.ID == t.Chosen {
				out.On = len(out.Rows)
			}
			walk(n.Children, depth+1)
		}
	}
	walk(t.Nodes, 0)
	return out
}

// code is a sentence with a command the server names monospaced as code
// (s-files-4: MOVE).
func code(parts []w.MsgPart) []w.MsgPart {
	for i, part := range parts {
		if m, ok := part.(w.Mono); ok {
			m.Form = w.MonoCode
			parts[i] = m
		}
	}
	return parts
}

// Forms are the forms of the sheet's buttons that post with none of their
// own: the deletion's, which has no form of the sheet.
func Forms(p *view.FileOpSheet) []w.PostForm {
	var out []w.PostForm
	for _, a := range p.Actions {
		if a.Method == "POST" && a.Enabled && p.Form == nil {
			out = append(out, bind.ActionForm(a, a.ID))
		}
	}
	return out
}

// foot is the sheet's foot: the operation — a submit of the sheet where it
// is a form, else of its own form — and its key but where it deletes;
// Cancel and its key at the end.
func foot(p *view.FileOpSheet) []w.SheetFootPart {
	var out []w.SheetFootPart
	var cancel []w.LinePart
	for _, a := range p.Actions {
		if a.Method != "POST" {
			cancel = append(cancel, bind.Button(a, ""), w.Kbd{Keys: esc})
			continue
		}
		b := bind.Button(a, a.ID)
		if p.Form != nil && a.Enabled {
			b = bind.Submit(a)
			if a.Href == p.Form.Action {
				b.To = ""
			}
		}
		if a.ID == openID {
			// The form's second button on a page without a script: the
			// folder ticked, opened (note 61 of Typecase).
			b.Name, b.Value, b.NoScript = openName, openValue, true
			out = append(out, b)
			continue
		}
		out = append(out, b)
		if !a.Danger {
			out = append(out, w.Kbd{Keys: enter})
		}
	}
	if len(cancel) > 0 {
		out = append(out, w.Line{Gap: w.LineGap7, Place: w.Pushed, Items: cancel})
	}
	return out
}
