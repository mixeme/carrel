// Package folder is a folder of a storage, /app/files/{account}/{col}/{path…},
// drawn by frame «Файлы · папка, свойства выбранного» (s-files-1) of the
// mockup — and by its states, three selected with the queue (s-files-2),
// tiles (s-files-5): the drafts frame2go printed for them, with the frames'
// data become the view model's (U-5.1, U-6.5).
package folder

import (
	"slices"

	"strconv"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The IDs of the page's forms: the rail's (its ticks submit it), the
// selection's (each row's tick and the column's submit it).
const (
	formRail   = "rail"
	formSelect = "select"
)

// The screen's words (U-1.3) for the shell: the mark, the search, the rail —
// the root's icon, a storage that can be written (rw) —; what the choice of
// columns sends (C-6.4 Columns).
var (
	top = bind.TopWords{
		Brand:  w.Brand{Logo: &w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63"}},
		Search: "Search everything…",
		NewKey: "n", SearchKey: "/",
		Theme: bind.ThemeWords{Label: "Theme", Auto: "Auto", Light: "Light", Dark: "Dark"},
		Keys:  bind.KeysWords{Label: "Keyboard shortcuts", Icon: "search", Key: "?", Sheet: "keys"},
	}
	rail    = bind.RailWords{Form: formRail, Toggle: "toggle", None: "—", ReadOnly: "ro", Tags: "Tags", Davloom: "davloom", External: "external", Root: "file", Writable: "rw"}
	columns = bind.ColumnWords{Name: "col", Reset: "default", Drag: "drag"}
	queue   = bind.QueueWords{Done: "done", Taken: "name taken", Sep: " · ", Percent: "%", Send: "Send",
		Fold: &w.QueueFold{Hide: "Hide", Show: "Show", Total: "{percent}%"}}
)

// The words of the queue the library's live layer fills while an upload goes
// (Queue.Progress, s-files-1, states-32; Typecase's note 108): its head and
// total, Hide, a file's row and the row that asks about a name taken — the
// words in braces are the layer's —, and the answers, each a value of
// «answer» the layer sends as «taken.<name>» before the file.
const (
	uploading = "Uploading {done} of {count}"
	sentSoFar = "{sent}"
	hide      = "Hide"
	rowName   = "{name}"
	rowMeta   = "{size} · {percent}%"
)

var answers = []w.Button{
	{Label: "Keep both", Small: true, Name: "answer", Value: "keep"},
	{Label: "Replace", Small: true, Name: "answer", Value: "replace"},
	{Label: "Skip", Small: true, Name: "answer", Value: "skip"},
}

// The screen's words (U-1.3): the rubric of the shortcuts, the badge of the
// attachment folder, the names of the buttons of an icon, the filter's
// pattern, the heads of the columns, the row that goes up, the dash of a
// folder's size, what joins the pieces of a line, a count of rows, the line
// under the list, the panel's cross, its rubric of links and the line under
// it, «Upload» after Upload's arrow.
const (
	shortcuts    = "Shortcuts"
	attachments  = "attachment folder"
	readOnlyWord = "read-only"
	refresh      = "Refresh"
	details      = "Details"
	more         = "More"
	sendWord     = "Send"
	download     = "Download"
	filter       = "Filter this folder"
	colName      = "Name"
	colSize      = "Size"
	colType      = "Type"
	colChanged   = "Changed"
	up           = "up one folder"
	none         = "—"
	sep          = " · "
	selected     = " selected"
	items        = " items"
	oneItem      = " item"
	dropHint     = "Drop files onto the window to upload them into this folder. A name that is already taken is refused rather than overwritten, and the queue says so."
	noDropHint   = "Dropping files onto the window and the upload queue need scripts, and this page runs none. Upload sends the files; a name that is already taken is refused rather than overwritten."
	closePanel   = "Close the panel"
	links        = "Links"
	linksNote    = "Notes or events may hold it as an attachment: Carrel does not look through the records."
	upload       = "Upload"
	sizeField    = "size"
	takenOp      = "?op=taken"
	folderIcon   = "file"
	upIcon       = "back"
	dotsIcon     = "dots"
	refreshIcon  = "refresh"
)

// The names of the contract the screen places by (C-7.5): what stands in the
// head, what in the bar of the selected, Print in the head's «⋯», the
// views and the orders of the bar, a row's Download, the panel's Download.
const (
	uploadID = "file.upload"
	// uploadFolderID is Upload's Folder (C-7.5): the same POST, a folder
	// picked.
	uploadFolderID = "file.upload.folder"
	newID          = "folder.new"
	detailsID      = "files.details"
	printID        = "list.print"
	viewID         = "files.view."
	sortID         = "files.sort."
	fileGet        = "file.download"
	fileOpen       = "file.open"
	// selectAllID is the selection's Select all: with a script only
	// (s-files-1, Typecase's note 160).
	selectAllID = "files.select.all"
)

// selectionActs are the actions of the bar of the selected, in its order
// (s-files-1).
var selectionActs = []string{"file.download", "file.rename", "file.move", "file.copy", "file.delete"}

// Screen draws the folder from its view model, in the state the model is
// in.
//
//   - The shell (U-2.1): the top bar; the rail — the root, the storages by
//     connection (the folder's on), the shortcuts, the foot with New folder.
//   - The head: the folder's name, the way to it (Crumbs), the connection
//     and the badge of the attachment folder; Upload with its arrow (Files,
//     Folder), New folder, Refresh, Details, «⋯» with Print.
//   - The notices.
//   - The bar of the selected while rows are selected: their size, Download,
//     Rename, Move, Copy, Delete, how many, Select all, Clear selection.
//   - The bar: the filter, the order, the view, Columns, «⋯» with the
//     order; on its right how many and how large.
//   - The rows (a list) or the tiles: a row goes up; each row its tick, the
//     icon of its kind and its name, its size, its type — a folder's with
//     what it holds —, when it changed, Download of a file and «⋯» with its
//     menu.
//   - The line of what a drop does, where Upload can be pressed; the queue of
//     transfers.
//   - The panel of the file selected (Shell.Details): its name, its storage,
//     the cross, its preview, Download, Open, Rename, Move, its fields, the
//     links' rubric, Delete.
func Screen(p *view.FilesPage) w.Shell {
	p = queued(p)
	main := []w.MainPart{head(p)}
	main = append(main, bind.Notices(p.Notices, w.MsgSpacePage)...)
	// The bar of the selected stands whenever the folder has its form:
	// without a script always, as nothing picks rows then but the ticks and
	// this bar's buttons (the owner's decision of 09.10.2026); with one only
	// while rows are selected (Bar.NoScript, states-34).
	if p.Select != nil && len(p.Rows) > 0 {
		main = append(main, selection(p))
	}
	// An empty folder has neither bar nor table, its empty state says so
	// (states-32); a folder not read, its message alone (states-33).
	switch {
	case p.Empty != nil:
		main = append(main, w.Empty{Parts: emptyParts(*p.Empty)})
	case p.Unread:
	case p.View == view.FileTiles:
		main = append(main, bar(p.Bar), tiles(p.Rows))
	default:
		main = append(main, bar(p.Bar), table(p))
	}
	if up, ok := action(p.Bar.Actions, uploadID); ok && up.Enabled && len(p.Rows) > 0 && p.Queue == nil && p.View != view.FileTiles {
		// The line of what a drop does stands under the list (s-files-1);
		// the queue takes its place (s-files-2), and the tiles have none
		// (s-files-5). A drop needs the library's script: without one the
		// line is not there (Msg.ScriptOnly, Typecase's task 90).
		// Without it, what a page with none does instead (s-files-1).
		main = append(main, w.Msg{Parts: []w.MsgPart{w.Text(dropHint)}, ScriptOnly: true},
			w.Msg{Parts: []w.MsgPart{w.Text(noDropHint)}, NoScript: true})
	}
	up, live := action(p.Bar.Actions, uploadID)
	live = live && up.Enabled && up.Method == "POST" && p.View != view.FileTiles && !p.Unread
	switch {
	case p.Queue != nil && live:
		main = append(main, progress(bind.Queue(*p.Queue, queue)))
	case p.Queue != nil:
		main = append(main, bind.Queue(*p.Queue, queue))
	case live:
		// No queue yet: the library's live layer shows this one when the
		// upload's form goes with files; until then it waits hidden
		// (s-files-1, states-32).
		main = append(main, progress(w.Queue{
			Title:  uploading,
			Total:  w.Mono{Text: sentSoFar, Variant: w.MonoDusty, Place: w.Pushed},
			Toggle: w.Button{Label: hide, Small: true},
			Fold:   queue.Fold,
		}))
	}
	body := w.ShellBody{Rail: railParts(p.Rail), Main: w.Main{Items: main}}
	if p.Queue != nil {
		body.Main.Variant = w.MainRoomy
	}
	if p.Details != nil {
		body.Side = side(*p.Details)
	}
	t := bind.Top(p.Top, top)
	return w.Shell{Top: &t, Body: body}
}

// Forms are the page's forms: the rail's, which its ticks submit, the foot's
// that post (New folder), the head's and the selection's that post (Upload
// and its arrow with their files, New folder, Rename, Move, Copy, Delete),
// the rows' menus and the panel's, the queue's answers, the buttons of the
// notices, the items of the menus that post (Sign out).
func Forms(p *view.FilesPage) []w.PostForm {
	p = queued(p)
	out := bind.TopForms(p.Top)
	if p.Select != nil {
		// The selection's form (s-files-5): the rows' picks are its fields,
		// the bar's operations its buttons.
		out = append(out, bind.PostForm(*p.Select, formSelect))
	}
	if f := bind.RailForm(p.Rail, formRail); f != nil {
		out = append(out, *f)
	}
	out = append(out, bind.FootForms(p.Rail)...)
	for _, a := range p.Bar.Actions {
		switch {
		case a.ID == uploadID:
			// The one form of an upload, its file fields in the head and in
			// an empty folder's state (s-files-1, states-32): what Files and
			// Folder pick goes with it, and the live queue follows it.
			if a.Method == "POST" && a.Enabled {
				f := bind.ActionForm(a, uploadID)
				f.File, f.Multipart = "", true
				out = append(out, f)
			}
		case a.Method == "POST" && a.Enabled:
			out = append(out, bind.ActionForm(a, a.ID))
		}
	}
	for _, r := range p.Rows {
		out = append(out, bind.MenuFormsAt(r.Menu, bind.FormID)...)
	}
	if d := p.Details; d != nil {
		for _, a := range d.Actions {
			if a.Method == "POST" && a.Enabled {
				out = append(out, bind.ActionForm(a, bind.FormID(a)))
			}
		}
	}
	if p.Queue != nil {
		out = append(out, bind.QueueForms(*p.Queue)...)
	}
	return unique(append(out, bind.NoticeForms(p.Notices)...))
}

// uploadField is a file field of the upload: it picks into the upload's one
// form, the sizes of its files go before them, and the library asks the
// upload's address which of their names are taken before they go (?op=taken,
// FileField.Taken; s-files-8, states-32).
func uploadField(f w.FileField, a view.Action) *w.FileField {
	f.Form, f.To, f.Size, f.Taken = uploadID, "", sizeField, a.Href+takenOp
	return &f
}

// progress is a queue the library's live layer fills while the upload's form
// goes (Queue.Progress, Typecase's note 108): a row for each file from Row,
// and the question of a name taken from Taken with its answers.
// queued is the folder with its upload answering into the queue only where
// the folder draws one — while an upload may go, or what goes; the tiles and
// a folder not read have none, and their upload goes the plain way, the
// folder answered whole.
func queued(p *view.FilesPage) *view.FilesPage {
	up, live := action(p.Bar.Actions, uploadID)
	live = live && up.Enabled && up.Method == "POST" && p.View != view.FileTiles && !p.Unread
	if p.Queue != nil || live {
		return p
	}
	out := *p
	out.Bar.Actions = plainUpload(p.Bar.Actions)
	if p.Empty != nil {
		empty := *p.Empty
		empty.Actions = plainUpload(p.Empty.Actions)
		out.Empty = &empty
	}
	return &out
}

// plainUpload are actions with the upload's zone taken off.
func plainUpload(actions []view.Action) []view.Action {
	out := slices.Clone(actions)
	for i := range out {
		if out[i].ID == uploadID {
			out[i].Target = ""
		}
	}
	return out
}

func progress(q w.Queue) w.Queue {
	q.Progress = uploadID
	q.Row = &w.QueueItem{Name: rowName, Meta: rowMeta, Meter: &w.Meter{Percent: 0, Track: 100}}
	q.Taken = &w.QueueItem{Name: rowName, Meta: queue.Taken, Failed: true, Acts: answers}
	return q
}

// unique are the forms each once: the panel's Rename posts the same as the
// row's Rename of its file.
func unique(forms []w.PostForm) []w.PostForm {
	var out []w.PostForm
	seen := map[string]bool{}
	for _, f := range forms {
		if !seen[f.ID] {
			seen[f.ID] = true
			out = append(out, f)
		}
	}
	return out
}

// railParts is the rail: the sections, the root and the storages, the
// shortcuts, the foot.
func railParts(r view.Rail) []w.RailPart {
	nav := w.Nav{}
	for _, n := range r.Nav {
		nav.Items = append(nav.Items, bind.NavItem(n))
		if n.On {
			nav.On = len(nav.Items)
		}
	}
	parts := []w.RailPart{nav}
	parts = append(parts, bind.Sources(r, rail)...)
	if len(r.Shortcuts) > 0 {
		parts = append(parts, w.RailSec{Items: []w.RailSecPart{w.Rubric{Text: shortcuts}, bind.Shortcuts(r.Shortcuts)}})
	}
	return append(parts, bind.Foot(r, rail))
}

// action is the action of the ID among actions, if there is one.
func action(actions []view.Action, id string) (view.Action, bool) {
	for _, a := range actions {
		if a.ID == id {
			return a, true
		}
	}
	return view.Action{}, false
}

// head is the folder's head: its name and the way to it, its connection and
// the badge of the attachment folder; Upload with its arrow, New folder,
// Refresh, Details, «⋯» with Print.
func head(p *view.FilesPage) w.Head {
	h := w.Head{Title: &w.HeadTitle{Text: string(p.Title)}}
	// The way to the folder from its storage's root; the root itself has
	// none (states-33, -34).
	if len(p.Crumbs) > 1 {
		h.Crumbs = bind.Crumbs(p.Crumbs)
	}
	// The line under the name: the connection, the badge of the attachment
	// folder, how many items, why nothing can be made here — each after the
	// one before it (s-files-1, states-34).
	var parts []w.HeadSubPart
	add := func(p w.HeadSubPart) {
		if len(parts) > 0 {
			parts = append(parts, w.Text(sep))
		}
		parts = append(parts, p)
	}
	if p.Subtitle != "" {
		add(w.Text(p.Subtitle))
	}
	if p.Attachments {
		add(w.Badge{Label: attachments})
	}
	if p.Count != "" {
		add(w.Text(p.Count))
	}
	if p.ReadOnly {
		add(w.Badge{Label: readOnlyWord})
	}
	if len(parts) > 0 {
		h.Subs = []w.HeadSub{{Parts: parts}}
	}
	var acts []w.ActsPart
	if a, ok := action(p.Bar.Actions, uploadID); ok {
		b := bind.Button(a, "")
		b.Menu = true
		items := bind.MenuItems(a.Arrow, "")
		for i, it := range items {
			// Files picks several files at once and Folder a folder, its
			// files going with their paths (s-files-8; S-17.5;
			// FileField.Folder, Typecase's task 119): a browser does both
			// without a script.
			if m, ok := it.(w.MenuItem); ok && m.File != nil && i < len(a.Arrow) {
				f := *m.File
				switch a.Arrow[i].ID {
				case uploadFolderID:
					f.Folder = true
				case uploadID:
					f.Many = true
				}
				m.File = uploadField(f, a.Arrow[i].Action)
				items[i] = m
			}
		}
		// Both pick into the one upload form, and one Upload sends it
		// without a script (s-files-8).
		if a.Enabled && a.Method == "POST" {
			items = append(items, w.MenuItem{Label: upload, NoScript: true, Form: uploadID})
		}
		acts = append(acts, w.Drop{Trigger: b, Menu: w.Menu{Items: items, Width: w.MenuWidth212, Pop: w.MenuUnder}})
	}
	if a, ok := action(p.Bar.Actions, newID); ok {
		acts = append(acts, bind.Button(a, a.ID))
	}
	if r := p.Top.Refresh; r.Action.ID != "" {
		b := bind.Button(r.Action, "")
		b.Label, b.Icon, b.Title = "", refreshIcon, refresh
		acts = append(acts, b)
	}
	if a, ok := action(p.Bar.Actions, detailsID); ok {
		b := bind.Button(a, "")
		b.Label, b.Title = "", details
		if a.On {
			b.Variant = w.ButtonOnCard
		}
		acts = append(acts, b)
	}
	if a, ok := action(p.Bar.Actions, printID); ok {
		acts = append(acts, w.Drop{
			Trigger: w.Button{Icon: dotsIcon, Title: more},
			Menu:    w.Menu{Items: bind.MenuItems([]view.MenuItem{{Action: a}}, ""), Width: w.MenuWidth212, Pop: w.MenuUnderEnd},
		})
	}
	h.Acts = &w.Acts{Items: acts}
	return h
}

// selection is the bar of the selected rows: their size, Download, Rename,
// Move, Copy, Delete, how many, Select all, Clear selection — icon and
// label, the label a title where the bar is narrow (Typecase's note 160);
// with none selected a bar
// of a page without a script only.
func selection(p *view.FilesPage) w.Bar {
	// How large the selected are, while some are (states-34 draws the bar
	// of none from its line on).
	var items []w.BarPart
	if p.Bar.Selected > 0 {
		items = append(items, w.Mono{Text: string(p.Bar.SelectedSize), Variant: w.MonoMuted})
	}
	items = append(items, w.BarSep{})
	for _, id := range selectionActs {
		a, ok := action(p.Bar.Actions, id)
		if !ok {
			continue
		}
		b := bind.Button(a, a.ID)
		if a.Submits && a.Enabled {
			// An operation of the selection sends its form to its own
			// address (Action.Submits): the bar stands outside it.
			b.Form = formSelect
		}
		b.Folds, b.Title = true, string(a.Label)
		items = append(items, b)
	}
	b := w.Bar{Items: items, Count: &w.BarCount{Text: strconv.Itoa(p.Bar.Selected) + selected}, Variant: w.BarSelection, NoScript: p.Bar.Selected == 0}
	if p.Select != nil && len(p.Select.Actions) > 0 {
		b.Right = &w.BarRight{}
		for _, a := range p.Select.Actions {
			c := bind.Button(a, "")
			c.Folds, c.Small, c.Title, c.ScriptOnly = true, true, string(a.Label), a.ID == selectAllID
			b.Right.Items = append(b.Right.Items, c)
		}
	}
	return b
}

// bar is the folder's bar: the filter, the order, the view by its icons,
// Columns with its menu, «⋯» with the order's twins; on its right how many
// rows and how large.
func bar(b view.ListBar) w.Bar {
	var parts []w.BarPart
	if b.FilterForm != nil {
		f := bind.Filter(b, filter)
		f.Variant = w.FilterH26W190
		parts = append(parts, f)
	}
	order, views := w.Seg{Secondary: true}, w.Seg{}
	var twins []view.MenuItem
	for _, a := range b.Segments {
		switch {
		case len(a.ID) > len(viewID) && a.ID[:len(viewID)] == viewID:
			o := bind.SegOption(a)
			o.Title, o.Label = o.Label, ""
			views.Options = append(views.Options, o)
		case len(a.ID) > len(sortID) && a.ID[:len(sortID)] == sortID:
			order.Options = append(order.Options, bind.SegOption(a))
			twins = append(twins, view.MenuItem{Action: a})
		}
	}
	if len(order.Options) > 0 {
		parts = append(parts, order)
	}
	if len(views.Options) > 0 {
		parts = append(parts, views)
	}
	if b.Columns != nil {
		cols := bind.Columns(*b.Columns, columns)
		cols.Menu.Pop = w.MenuUnder
		parts = append(parts, cols)
	}
	if len(twins) > 0 {
		// «⋯» holds the twins of what leaves the bar as it narrows
		// (components-19, Typecase's task 117): the order.
		parts = append(parts, w.Drop{
			Trigger: w.Button{Icon: dotsIcon, Title: more},
			More:    true,
			Menu:    w.Menu{Items: bind.MenuItems(twins, ""), Pop: w.MenuUnderEnd, Inline: true},
		})
	}
	right := count(b.Loaded)
	if b.Size != "" {
		right += sep + string(b.Size)
	}
	return w.Bar{Items: parts, Right: &w.BarRight{Items: []w.BarPart{w.Mono{Text: right}}, Secondary: true}}
}

// count is how many rows, in words: «14 items», «1 item».
func count(n int) string {
	if n == 1 {
		return strconv.Itoa(n) + oneItem
	}
	return strconv.Itoa(n) + items
}

// ticked is the state of a tick: on or off.
func ticked(on bool) *w.Ticked {
	t := w.TickedOff
	if on {
		t = w.TickedOn
	}
	return &t
}

// table is the rows of the folder: the column of ticks — the head's the
// library's script's «every row», each row's a field of the selection's
// form (Send.Of), on where the row is selected —, a row that goes up out of
// a folder, and a row for each folder and file.
func table(p *view.FilesPage) w.Table {
	// The head's tick says whether every row is selected, some of them
	// (s-files-2) or none.
	all := w.TickedOff
	switch {
	case p.Bar.Selected > 0 && p.Bar.Selected == len(p.Rows):
		all = w.TickedOn
	case p.Bar.Selected > 0:
		all = w.TickedPart
	}
	t := w.Table{Cols: []w.TableCol{
		{Width: w.ColTick, Tick: &all},
		{Label: colName},
		{Label: colSize, Width: w.ColSize, Rank: w.Rank3},
		{Label: colType, Width: w.ColType, Rank: w.Rank4, Secondary: true},
		// What a row's card keeps on a phone (TableCol.Main, Typecase's
		// note 155, Р-66).
		{Label: colChanged, Width: w.ColTime, Main: true},
		{Width: w.ColNum},
	}}
	if n := len(p.Crumbs); n > 1 {
		// The row that goes up leads where the crumb before the open folder
		// does (s-files-1).
		t.Rows = append(t.Rows, w.TableRow{Cells: []w.TableCell{{}, {Parts: []w.CellPart{w.TableFile{Icon: upIcon, Name: up, Accent: true, Href: p.Crumbs[n-2].Href}}, Span: len(t.Cols) - 1}}})
	}
	for _, r := range p.Rows {
		size, kind := string(r.Size), string(r.Type)
		if r.IsDir {
			size = none
			if r.Items != "" {
				kind += sep + string(r.Items)
			}
		}
		acts := w.TableCell{Acts: true}
		if a, ok := menuAction(r.Menu, fileGet); ok && !r.IsDir {
			b := bind.Button(a, "")
			b.Label, b.Small, b.Title = "", true, download
			acts.Parts = append(acts.Parts, b)
		}
		if len(r.Menu) > 0 {
			acts.Parts = append(acts.Parts, w.Drop{
				Trigger: w.Button{Icon: dotsIcon, Small: true, Title: more},
				Menu:    w.Menu{Items: bind.MenuItemsAt(r.Menu, "", bind.FormID), Width: w.MenuWidth212, Pop: w.MenuUnderEnd},
			})
		}
		row := w.TableRow{Cells: []w.TableCell{
			{Pick: ticked(r.Selected), Send: w.Send{Of: formSelect, Name: p.SelectField, Value: string(r.Name)}},
			{Parts: []w.CellPart{w.TableFile{Icon: bind.Icon(r.Icon), Name: string(r.Name), Href: opens(r)}}},
			{Parts: []w.CellPart{w.Text(size)}, Mono: true},
			{Parts: []w.CellPart{w.Text(kind)}, Mono: true},
			{Parts: []w.CellPart{w.Text(r.Modified)}, Mono: true},
			acts,
		}}
		switch {
		case r.Selected:
			row.State = w.RowPicked
		case r.IsDir:
			row.State = w.RowFolder
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}

// emptyParts are what an empty folder says (states-32) and its buttons:
// Upload picks files as the head's does — the label of its file input, the
// form of its ID —, with its arrow drawn where it has one; the rest as the
// empty state of any screen has them (bind.EmptyParts).
func emptyParts(n view.Notice) []w.EmptyPart {
	var out []w.EmptyPart
	i := 0
	for _, p := range bind.EmptyParts(n) {
		if _, ok := p.(w.Button); !ok {
			out = append(out, p)
			continue
		}
		a := n.Actions[i]
		i++
		if a.File == "" {
			out = append(out, p)
			continue
		}
		b := bind.FileButton(a, a.ID)
		if b.File != nil && a.ID == uploadID {
			f := *b.File
			f.Many = true
			b.File = uploadField(f, a)
		}
		b.Menu = len(a.Arrow) > 0
		out = append(out, b)
		// Without a script the file field sends nothing by itself: Send
		// beside it (states-32, note 83 of Typecase).
		if send := bind.Upload(a, a.ID, sendWord); send != nil {
			out = append(out, *send)
		}
	}
	return out
}

// opens is where a row's name leads (TableFile.Href, s-files-1): a folder
// opens, a file is chosen — the folder with it selected, which is its menu's
// Open (FileRow.Href of a file is its download).
func opens(r view.FileRow) string {
	if r.IsDir {
		return r.Href
	}
	if a, ok := menuAction(r.Menu, fileOpen); ok {
		return a.Href
	}
	return r.Href
}

// menuAction is the item of the ID in a row's menu, as an action.
func menuAction(items []view.MenuItem, id string) (view.Action, bool) {
	for _, it := range items {
		if it.ID == id {
			return it.Action, true
		}
	}
	return view.Action{}, false
}

// tiles are the rows as tiles: a picture's thumbnail where there is one, a
// folder large without a card, the icon of its kind on a card where not; the
// name; the size with the
// picture's size or why it has no thumbnail, or what a folder holds.
func tiles(rows []view.FileRow) w.Grid {
	g := w.Grid{}
	for _, r := range rows {
		th := w.Thumb{Icon: bind.Icon(r.Icon), Src: r.Thumb, Alt: string(r.Name)}
		switch {
		case r.IsDir:
			th.Form = w.ThumbFolder // no card, a large folder (s-files-5)
		case r.Thumb == "":
			th.Form = w.ThumbOnCard
		}
		meta := string(r.Size)
		switch {
		case r.IsDir:
			meta = string(r.Items)
		case r.Dimensions != "":
			meta += sep + string(r.Dimensions)
		case r.Note != "":
			meta += sep + string(r.Note)
		}
		g.Tiles = append(g.Tiles, w.Tile{Thumb: th, Name: string(r.Name), Meta: meta, Selected: r.Selected, Href: opens(r)})
	}
	return g
}

// side is the panel of the file selected: its name, its storage, the cross;
// its preview; Download, Open, Rename, Move; its fields — the path small —;
// the links' rubric with what the server does not keep; Delete.
func side(d view.Details) *w.Side {
	h := w.Head{
		Title: &w.HeadTitle{Text: string(d.Title), Size: w.HeadTitle17},
		Acts:  &w.Acts{Items: []w.ActsPart{w.Close{Label: closePanel, Href: d.Close}}},
		Pad:   w.HeadPadBottom8,
	}
	if d.Subtitle != "" {
		h.Subs = []w.HeadSub{{Parts: []w.HeadSubPart{w.Text(d.Subtitle)}}}
	}
	items := []w.SidePart{h}
	if d.Preview != nil {
		items = append(items, w.Pad{Items: []w.PadPart{w.Thumb{Icon: bind.Icon(d.Preview.Icon), Src: d.Preview.Href, Alt: string(d.Title), Form: w.ThumbPreview}}})
	}
	var acts []w.Button
	var danger *w.Button
	for _, a := range d.Actions {
		b := bind.Button(a, bind.FormID(a))
		if a.Danger {
			danger = &b
			continue
		}
		acts = append(acts, b)
	}
	if len(acts) > 0 {
		items = append(items, w.SideActs{Items: acts})
	}
	// The path is the last field, small (s-files-1).
	fields := w.Fields{}
	for i, f := range d.Fields {
		face := w.ValueMono
		if i == len(d.Fields)-1 {
			face = w.ValueMonoSmall
		}
		fields.Items = append(fields.Items, w.FieldsItem{Term: string(f.Label), Value: []w.ValuePart{w.Text(f.Value)}, Face: face})
	}
	if len(fields.Items) > 0 {
		items = append(items, fields)
	}
	items = append(items, w.Sec{Items: []w.SecPart{w.Rubric{Text: links}, w.SecNote{Text: linksNote}}})
	if danger != nil {
		items = append(items, w.Danger{Button: *danger})
	}
	return &w.Side{Items: items}
}

// Queue is the queue of transfers alone, as the folder draws it while an
// upload may go (progress): an upload's answer brings it (C-7.3), and the
// next upload moves its rows in place.
func Queue(q view.TransferQueue) (w.Queue, []w.PostForm) {
	return progress(bind.Queue(q, queue)), bind.QueueForms(q)
}
