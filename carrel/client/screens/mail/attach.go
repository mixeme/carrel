package mail

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of the attach sheets (U-1.3): the search's name, the rubric of
// what is found, the heads of a folder's columns, a folder's size, the line
// at Attach — its pieces, a file or a link —, the IDs of the search's form
// and of a calendar's, the line the calendar's form recounts, the icons of
// Attach, of a server and of a folder, the ceiling's tag on.
const (
	searchWord  = "Search"
	found       = "Found"
	colName     = "Name"
	colSize     = "Size"
	colChanged  = "Changed"
	noSize      = "—"
	goesFile    = "goes as a file"
	goesLink    = "goes as a link"
	searchID    = "attach-search"
	calendarID  = "attach-calendar"
	lineID      = "attach-line"
	serverIcon  = "server"
	folderIcon  = "file"
	linkGoes    = "as a link"
	qField      = "q"
	spanField   = "span"
	attachAdd   = "mail.attach.add"
	attachClose = "mail.attach.cancel"
)

// sheetIcons are the icons of an attach sheet by what it attaches.
var sheetIcons = map[string]string{
	"contact": "con", "book": "con", "event": "cal", "calendar": "cal", "task": "task", "note": "note", "file": "doc", "folder": "file",
}

// AttachTitle is the sheet's title (MailAttachSheet.Title).
func AttachTitle(p *view.MailAttachSheet) string { return string(p.Title) }

// Attach draws an attach sheet (s-mail-5, -6, -10, -12, -13): it stands in
// the letter's place.
//
//   - The head: the icon of what it attaches and «Attach a note»; the
//     search where a record is searched for — a field of a form of its own
//     (Search), so Enter in it asks again —, the crumbs to the folder open
//     for a file or a folder; the cross back to the letter.
//   - Left a tree: «All …» above, a server and its collections of the kind
//     under it, the one chosen on; for a file or a folder the servers'
//     storages and their folders, the open one on.
//   - Right what is found: «Found» with how many, a record a row — a
//     person with their face (s-mail-12), anything else its title, its day
//     and where (s-mail-5) —, the chosen one on; the files of the folder as
//     a table — Name, Size, Changed and whether it goes as a file or a link
//     —; a folder's sheet says what is in it, its folders choosing
//     themselves and its files off; a calendar's what goes in and its days
//     (a form of its own that recounts the line at Attach).
//   - What a note goes as (Send it as); why it goes as a link, in the
//     accent.
//   - The foot: Attach — off with why until something is chosen —, the line
//     of what goes and how, Cancel pushed.
func Attach(p *view.MailAttachSheet) w.Sheet {
	var attach, cancel view.Action
	for _, a := range p.Actions {
		switch a.ID {
		case attachAdd:
			attach = a
		case attachClose:
			cancel = a
		}
	}
	var head []w.SheetHeadPart
	switch {
	case p.Search != nil:
		q := ""
		for _, set := range p.Search.Sets {
			for _, r := range set.Rows {
				if r.Kind != view.InputHidden {
					q = string(r.Value)
				}
			}
		}
		head = append(head, w.Filter{Query: w.Query{Value: q, Label: searchWord, Name: qField, Of: searchID}, Quiet: true, Variant: w.FilterT12W200, Place: w.Pushed})
	case len(p.Crumbs) > 0:
		c := w.Crumbs{Current: string(p.Crumbs[len(p.Crumbs)-1].Label)}
		for _, cr := range p.Crumbs[:len(p.Crumbs)-1] {
			c.Path = append(c.Path, w.Crumb{Label: string(cr.Label), Href: cr.Href})
		}
		head = append(head, c)
	}
	closer := w.Close{Label: closeWord, Href: cancel.Href}
	if len(head) == 0 {
		closer.Place = w.Pushed
	}
	head = append(head, closer)

	pick := w.SheetPick{Short: true}
	if p.Tree != nil {
		pick.Rail = []w.SheetPickRail{fileTree(*p.Tree)}
	} else {
		pick.Rail = []w.SheetPickRail{collectionTree(p)}
	}
	calendar := p.What == "calendar"
	switch {
	case calendar:
		pick.List = []w.SheetPickList{calendarForm(p)}
	case p.Tree != nil:
		pick.List = []w.SheetPickList{fileTable(p)}
	case len(p.Rows) > 0 || p.Search != nil:
		pick.List = []w.SheetPickList{w.ListGroup{Title: w.Rubric{Text: found, Form: w.RubricInline}, Count: &w.Num{Value: len(p.Rows)}}, recordList(p)}
	}
	body := []w.SheetPart{pick}
	// What a note goes as (s-mail-5).
	for _, set := range p.Form.Sets {
		for _, r := range set.Rows {
			if r.Kind == view.InputRadio && !calendar {
				body = append(body, w.Sec{Items: []w.SecPart{w.Field{Label: string(r.Label), Parts: append([]w.FieldPart{segOf(r)}, hintOf(r)...), Width: w.FieldW100}}})
			}
		}
	}
	if len(p.Note) > 0 {
		var parts []w.MsgPart
		for _, in := range bind.Inlines(p.Note) {
			if mp, ok := in.(w.MsgPart); ok {
				parts = append(parts, mp)
			}
		}
		body = append(body, w.Msg{Parts: parts, Tone: w.MsgAccent, Space: w.MsgSheet})
	}

	button := bind.Submit(attach)
	button.Icon = clipIcon
	switch {
	case !attach.Enabled:
		button.Submit, button.State, button.Reason = false, w.ButtonDisabled, string(attach.Reason)
	case calendar:
		button.Submit, button.Form = false, calendarID
	}
	line := w.Dim{Parts: preview(p), Size: w.Dim11_5, Variant: w.DimWrap}
	if calendar {
		line.ID = lineID
	}
	foot := []w.SheetFootPart{button, w.Kbd{Keys: enterKey}, line,
		w.Line{Items: []w.LinePart{bind.Button(cancel, ""), w.Kbd{Keys: escKey}}, Gap: w.LineGap7, Place: w.Pushed}}
	sheet := w.Sheet{
		Icon: w.Icon(sheetIcons[p.What]), Title: string(p.Title),
		Head: head, Body: body, Foot: foot, Flush: true, Width: w.SheetWidth900,
	}
	if !calendar {
		sheet.Action, sheet.Fields = p.Form.Action, bind.Hidden(p.Form)
	}
	return sheet
}

// AttachForms are the forms of the sheet's page: the search's own, sent
// with GET (Typecase's note 110 p. 1), and the page's under the letter.
func AttachForms(p *view.MailAttachSheet) []w.PostForm {
	if p.Search == nil {
		return nil
	}
	return []w.PostForm{{ID: searchID, Action: p.Search.Action, Fields: bind.Hidden(*p.Search), Method: "get"}}
}

// collectionTree is the tree of a record's sheet (s-mail-5, -10, -12):
// «All …» first, then each server and its collections of the kind, a link
// each, the chosen one on.
func collectionTree(p *view.MailAttachSheet) w.Tree {
	t := w.Tree{Variant: w.TreePadded}
	if p.All != nil {
		t.Rows = append(t.Rows, w.TreeRow{Mark: &w.Mark{Kind: w.MarkMulti}, Label: string(p.All.Label), Href: p.All.Href})
		if p.All.On {
			t.On = len(t.Rows)
		}
	}
	for _, sec := range p.Rail.Sections {
		t.Rows = append(t.Rows, w.TreeRow{Icon: serverIcon, Label: string(sec.Account)})
		for _, r := range sec.Rows {
			t.Rows = append(t.Rows, w.TreeRow{Mark: &w.Mark{Colour: string(r.Colour)}, Label: string(r.Label), Indent: w.TreeLevel22, Href: r.Href})
			if r.Tick == view.TickOn {
				t.On = len(t.Rows)
			}
		}
	}
	return t
}

// treeIndents are the indents of a storage's tree by depth: a server, its
// storages, then folders (s-mail-6, -13).
var treeIndents = []w.TreeIndent{0, w.TreeLevel22, w.TreeLevel36, w.TreeLevel44, w.TreeLevel58}

// fileTree is the tree of a file's or a folder's sheet: a server, its
// storages, their folders opened on the way to the open one, each a link,
// the open one on.
func fileTree(ft view.FileTree) w.Tree {
	t := w.Tree{Variant: w.TreePadded}
	var walk func(nodes []view.FileTreeNode, depth int)
	walk = func(nodes []view.FileTreeNode, depth int) {
		for _, n := range nodes {
			row := w.TreeRow{Icon: folderIcon, Label: n.Label, Href: n.Href, Indent: treeIndents[min(depth, len(treeIndents)-1)]}
			if depth == 0 {
				row.Icon = serverIcon
			}
			t.Rows = append(t.Rows, row)
			if n.ID != "" && n.ID == ft.Chosen {
				t.On = len(t.Rows)
			}
			walk(n.Children, depth+1)
		}
	}
	walk(ft.Nodes, 0)
	return t
}

// recordList is what is found: a person by their face (s-mail-12), anything
// else by its title, its day and where it lives (s-mail-5).
func recordList(p *view.MailAttachSheet) w.List {
	l := w.List{}
	for _, r := range p.Rows {
		mark := w.Mark{Colour: string(r.Colour)}
		if r.Kind == view.KindContact {
			row := w.FaceRow{Mark: mark, Title: []w.Inline{w.Text(r.Title)}, Strong: true, Subtitle: string(r.Subtitle), Detail: string(r.Detail),
				Caption: string(r.Caption), Source: string(r.Stamp), On: r.On, Href: r.Href}
			if r.Avatar != nil {
				row.Avatar = bind.Avatar(*r.Avatar)
			}
			if row.Caption == "" {
				row.Caption = noSize
			}
			l.Rows = append(l.Rows, row)
			continue
		}
		l.Rows = append(l.Rows, w.BriefRow{Mark: mark, Title: string(r.Title), Date: w.Mono{Text: string(r.When)}, Where: string(r.Stamp), On: r.On, Href: r.Href})
	}
	return l
}

// fileTable is the folder open (s-mail-6, -13): a file its icon, name, size,
// when it changed and whether it goes as a file or a link — the chosen one
// on —; in a folder's sheet, under what is in it, its folders choosing
// themselves and its files off.
func fileTable(p *view.MailAttachSheet) w.Table {
	folder := p.What == "folder"
	t := w.Table{Cols: []w.TableCol{{Label: colName}, {Label: colSize, Width: w.Col90}, {Label: colChanged, Width: w.Col110, Rank: w.Rank3}}}
	if folder {
		t.Cols[0].Label = string(p.In)
	} else {
		t.Cols = append(t.Cols, w.TableCol{Width: w.ColType, Rank: w.Rank4})
	}
	for _, r := range p.Rows {
		dir := r.Detail == "folder"
		icon := w.Icon(sheetIcons["file"])
		size := string(r.Size)
		if dir {
			icon, size = folderIcon, noSize
		}
		row := w.TableRow{Cells: []w.TableCell{
			{Parts: []w.CellPart{w.TableFile{Icon: icon, Name: string(r.Title), Href: r.Href}}},
			{Parts: []w.CellPart{w.Text(size)}, Mono: true},
			{Parts: []w.CellPart{w.Text(r.Changed)}, Mono: true},
		}}
		if !folder {
			goes := w.Tag{Label: string(r.Goes)}
			if len(r.Goes) >= len(linkGoes) && string(r.Goes[len(r.Goes)-len(linkGoes):]) == linkGoes {
				goes.On = true
			}
			if r.Goes != "" {
				row.Cells = append(row.Cells, w.TableCell{Parts: []w.CellPart{goes}})
			} else {
				row.Cells = append(row.Cells, w.TableCell{})
			}
		}
		switch {
		case r.On:
			row.State = w.RowOn
		case r.Off:
			row.State = w.RowGone
		case dir && folder:
			row.State = w.RowFolder
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}

// calendarForm is a calendar's choice (s-mail-10): the whole of it or its
// days — a form of its own Attach sends, which recounts the line at Attach
// with the live layer.
func calendarForm(p *view.MailAttachSheet) w.Form {
	f := w.Form{ID: calendarID, Action: p.Form.Action, Recount: lineID, Fields: bind.Hidden(p.Form)}
	for _, set := range p.Form.Sets {
		for _, r := range set.Rows {
			switch {
			case r.Kind == view.InputHidden:
			case r.Name == spanField:
				f.Items = append(f.Items, w.Field{Label: string(r.Label), Parts: []w.FieldPart{segOf(r)}, Width: w.FieldW100})
			default:
				f.Items = append(f.Items, bind.Field(r, w.FieldSM))
			}
		}
	}
	return f
}

// segOf is a choice of one among a few (span, format) as a switch under the
// row's name, an option that turns fields off saying which.
func segOf(r view.FormRow) w.Seg {
	s := w.Seg{Name: r.Name, Variant: w.SegStart}
	for _, o := range r.Options {
		s.Options = append(s.Options, w.SegOption{Label: string(o.Label), Value: o.Value, On: o.Selected, Disables: o.Disables})
	}
	return s
}

// hintOf is the line under a row, its pieces in their faces.
func hintOf(r view.FormRow) []w.FieldPart { return bind.Hint(r) }

// preview is the line at Attach (s-mail-5, -6, -10, -12, -13): what is
// chosen, its format monospaced, its size, and whether it goes as a file or
// a link; nothing chosen — nothing.
func preview(p *view.MailAttachSheet) []w.Inline {
	if len(p.Preview) == 0 {
		return nil
	}
	r := p.Preview[0]
	parts := []w.Inline{w.Text(string(r.Title) + sep)}
	if r.Format != "" && r.Format != "folder" && r.Kind != view.KindFile {
		parts = append(parts, w.Mono{Text: "." + string(r.Format)}, w.Text(sep))
	}
	if r.Size != "" {
		parts = append(parts, w.Text(string(r.Size)+sep))
	}
	goes := goesFile
	if r.AsLink {
		goes = goesLink
	}
	return append(parts, w.Text(goes))
}
