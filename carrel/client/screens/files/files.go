// Package files is the root of the files section, /app/files — the storages
// ticked in the rail, as top-level folders —, drawn by frame «Файлы · корень
// «Selected collections»» (s-files-6) of the mockup: the draft frame2go
// printed for it, with the frame's data become the view model's (U-5.1,
// U-6.5).
package files

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The IDs of the page's forms: the rail's (its ticks submit it).
const formRail = "rail"

// The screen's words (U-1.3) for the shell: the mark, the search, the rail —
// the root's icon, a storage that can be written (rw).
var (
	top = bind.TopWords{
		Brand:  w.Brand{Logo: &w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63"}},
		Search: "Search everything…",
		NewKey: "n", SearchKey: "/",
		Theme: bind.ThemeWords{Label: "Theme", Auto: "Auto", Light: "Light", Dark: "Dark"},
		Keys:  bind.KeysWords{Label: "Keyboard shortcuts", Icon: "search", Key: "?", Sheet: "keys"},
	}
	rail = bind.RailWords{Form: formRail, Toggle: "toggle", None: "—", ReadOnly: "ro", Tags: "Tags", Davloom: "davloom", External: "external", Root: "file", Writable: "rw"}
)

// The screen's words (U-1.3): the rubric of the shortcuts, the heads of the
// table's columns, a storage's access, the icon of a folder.
const (
	shortcuts = "Shortcuts"
	colName   = "Name"
	colServer = "Server"
	colAccess = "Access"
	colUse    = "Purpose"
	rw        = "rw"
	ro        = "ro"
	folder    = "folder"
)

// Screen draws the root from its view model, in the state the model is in.
//
//   - The shell (U-2.1): the top bar; the rail of sections (Files on), the
//     root — every storage ticked, its icon and how many of them —, the
//     storages by connection with their ticks, which submit the rail's form,
//     rw where one can be written, ro where not; the shortcuts (Attachments,
//     Published); the foot — New folder off with its reason, Connect
//     account, Subscribe to a feed.
//   - The head: what is open — the root's name, as the rail calls it — and
//     what is shown (Subtitle).
//   - The notices.
//   - A row for each storage ticked: a folder, its name, its connection and
//     access monospaced, its purpose as a badge.
//   - The note under the table: what the root is, the storages not ticked
//     by name.
//   - Nothing to show (Empty): what, why, and the button that helps.
func Screen(p *view.FilesRootPage) w.Shell {
	h := w.Head{Title: &w.HeadTitle{Text: rootName(p.Rail), Size: w.HeadTitle18}, Pad: w.HeadPadBottom8}
	if p.Subtitle != "" {
		h.Subs = []w.HeadSub{{Parts: []w.HeadSubPart{w.Text(p.Subtitle)}}}
	}
	main := []w.MainPart{h}
	main = append(main, bind.Notices(p.Notices, w.MsgSpacePage)...)
	if len(p.Collections) > 0 {
		main = append(main, table(p.Collections))
	}
	if len(p.Note) > 0 {
		main = append(main, w.Msg{Parts: bind.MsgParts(p.Note)})
	}
	if p.Empty != nil {
		main = append(main, w.Empty{Parts: bind.EmptyParts(*p.Empty)})
	}
	t := bind.Top(p.Top, top)
	return w.Shell{Top: &t, Body: w.ShellBody{Rail: railParts(p.Rail), Main: w.Main{Items: main}}}
}

// Forms are the page's forms: the rail's, which its ticks submit, the foot's
// that post (New folder), the buttons of the notices and of the empty state
// that post, the items of the menus that post (Sign out).
func Forms(p *view.FilesRootPage) []w.PostForm {
	out := bind.TopForms(p.Top)
	if f := bind.RailForm(p.Rail, formRail); f != nil {
		out = append(out, *f)
	}
	out = append(out, bind.FootForms(p.Rail)...)
	out = append(out, bind.NoticeForms(p.Notices)...)
	if p.Empty != nil {
		out = append(out, bind.NoticeForms([]view.Notice{*p.Empty})...)
	}
	return out
}

// rootName is what the rail calls the root: the name of its row of every
// storage.
func rootName(r view.Rail) string {
	for _, sec := range r.Sections {
		for _, row := range sec.Rows {
			if row.Colour == "" {
				return string(row.Label)
			}
		}
	}
	return ""
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

// table is the storages ticked, a row each: a folder by its name, which
// opens it, its connection and its access monospaced, its purpose as a
// badge. The connection is what a row's card keeps on a phone, the folder
// its name (TableCol.Main, Typecase's note 163, Р-66).
func table(cs []view.FileCollectionRow) w.Table {
	t := w.Table{Cols: []w.TableCol{
		{Label: colName},
		{Label: colServer, Width: w.ColTime, Main: true},
		{Label: colAccess, Width: w.Col80, Rank: w.Rank3},
		{Label: colUse, Width: w.ColTime, Rank: w.Rank4},
	}}
	for _, c := range cs {
		access := rw
		if c.ReadOnly {
			access = ro
		}
		use := w.TableCell{}
		if c.Purpose != "" {
			use.Parts = []w.CellPart{w.Badge{Label: string(c.Purpose)}}
		}
		t.Rows = append(t.Rows, w.TableRow{
			Cells: []w.TableCell{
				{Parts: []w.CellPart{w.TableFile{Icon: folder, Name: string(c.Label), Href: c.Href}}},
				{Parts: []w.CellPart{w.Text(c.Server)}, Mono: true},
				{Parts: []w.CellPart{w.Text(access)}, Mono: true},
				use,
			},
			State: w.RowFolder,
		})
	}
	return t
}
