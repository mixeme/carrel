// Package guest is the page a public link opens, /p/{token}
// (PublishedPage), drawn by frames «Публичная страница · …» (s-publish-6 …
// -39) of the mockup: what was published — a calendar, a book, a list of
// tasks, a notebook, one record, a note, a file, a folder — read only, for a
// visitor without an account. There is no shell: the banner with the mark,
// the page, the line of how long the link lives.
package guest

import (
	"strconv"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// listMain is the ID of the main the filter narrows as one types (Typecase's
// note 185).
const listMain = "list-main"

// The screen's words (U-1.3): the badge of what the visitor may do, what
// Copy says once it copied, «How to mount», the rubrics of a record, the
// heads of a folder's columns, the dash of a value there is none of, what
// joins a person to what they said, the lead of a size.
const (
	readOnly    = "read-only"
	copyWord    = "Copy"
	copied      = "Copied"
	openApp     = "Open in calendar app"
	howToMount  = "How to mount"
	description = "Description"
	attendees   = "Attendees"
	attachments = "Attachments"
	colName     = "Name"
	colSize     = "Size"
	colType     = "Type"
	colChanged  = "Changed"
	none        = "—"
	lead        = "· "
	percent     = "%"
)

var logo = w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63"}

// Screen draws the page from its view model:
//
//   - The banner: the mark, «read-only», and at its end the width of a text
//     (Width, links) and the downloads (Actions); a file the browser cannot
//     show has its download in the middle of the page instead.
//   - A calendar's and a book's address for a client (Strip) under the
//     banner, a folder's drive under its head.
//   - The head: the title, the line under it (Lede); a contact's initials;
//     a folder's crumbs with what it holds; the filter (Filter) at its end.
//   - What was published: the days of a calendar, ← Today → of a whole one;
//     the cards of a book by letter; the tasks open and done; the notes by
//     month; one record's fields, description and people; a note's text and
//     its files; a file's picture, text or what it is; a folder's table.
//     What was not found or is no longer there, said (Notices, Empty).
//   - The foot: how long the link lives (Foot).
func Screen(p *view.PublishedPage) w.Shell {
	main := []w.MainPart{banner(p)}
	if p.Strip != nil && p.Folder == nil {
		main = append(main, strip(*p.Strip))
	}
	switch {
	case p.Note != nil:
		main = append(main, note(p))
	case p.File != nil && stage(p.File):
		main = append(main, w.Stage{Thumb: w.Thumb{Icon: "doc", Form: w.ThumbSquare}, Title: string(p.File.Name),
			Meta: w.Mono{Text: bind.Words(p.Lede)}, Download: download(p.Actions), Note: string(p.File.Note)})
	default:
		main = append(main, head(p))
		main = append(main, bind.Notices(p.Notices, w.MsgSpacePage)...)
		if p.Strip != nil && p.Folder != nil {
			main = append(main, strip(*p.Strip))
		}
		main = append(main, body(p)...)
	}
	if len(p.Foot) > 0 {
		main = append(main, w.MainFoot{Text: bind.Words(p.Foot)})
	}
	return w.Shell{Body: w.ShellBody{Main: w.Main{Items: main, ID: bind.MainID(p.Filter, listMain)}}}
}

// stage says a file is shown by what it is, not by itself: the browser
// cannot show it, or it is too large for the page (PublishedFile.Note).
func stage(f *view.PublishedFile) bool {
	return f.Note != "" || f.Preview == view.PreviewNone
}

// banner is the head of the page: the mark, what the visitor may do, and
// at the end the width and the downloads — pushed right as one line, one
// button too (Typecase's note 126 p. 8). A calendar's or a book's page,
// with its address under the banner, has more room between its pieces.
func banner(p *view.PublishedPage) w.Banner {
	b := w.Banner{Logo: logo, Wide: p.Strip != nil && p.Folder == nil}
	if p.ReadOnly {
		b.Badge = &w.Badge{Label: readOnly}
	}
	if p.File != nil && stage(p.File) {
		return b
	}
	var items []w.LinePart
	if len(p.Width) > 0 {
		seg := w.Seg{}
		for _, a := range p.Width {
			seg.Options = append(seg.Options, w.SegOption{Label: string(a.Label), Href: a.Href, On: a.On})
		}
		items = append(items, seg)
	}
	for _, a := range p.Actions {
		items = append(items, bind.Button(a, ""))
	}
	if len(items) > 0 {
		b.End = w.Line{Gap: w.LineGapNormal, Place: w.Pushed, Items: items}
	}
	return b
}

// download is the one download of a file shown by what it is.
func download(actions []view.Action) w.Button {
	if len(actions) == 0 {
		return w.Button{}
	}
	return bind.Button(actions[0], "")
}

// strip is the address the page gives a client: the field to copy, the
// calendar app's link, a drive's «How to mount», what the address gives.
func strip(s view.PublishedStrip) w.Strip {
	label := string(s.Label)
	items := []w.StripPart{
		w.Input{Value: string(s.Address), Label: label, Mono: true, ReadOnly: true, Variant: w.InputLineFill},
		w.Button{Label: copyWord, Small: true, Copy: string(s.Address), Copied: copied},
	}
	if s.Open != "" {
		items = append(items, w.Button{Label: openApp, Small: true, Href: s.Open})
	}
	if len(s.Mount) > 0 {
		menu := w.Menu{Width: w.MenuWidth290, Pop: w.MenuUnder}
		for _, m := range s.Mount {
			menu.Items = append(menu.Items, w.MenuText{Title: string(m.Title), Text: string(m.Text)})
		}
		items = append(items, w.Drop{Trigger: w.Button{Label: howToMount, Menu: true, Small: true}, Menu: menu})
	}
	if s.Note != "" {
		items = append(items, w.Dim{Parts: []w.Inline{w.Text(s.Note)}, Size: w.Dim11})
	}
	return w.Strip{Label: label, Items: items}
}

// head is the page's head: a file's or a folder's title larger, a
// contact's initials before it, the line under it, a folder's crumbs, the
// filter at the end.
func head(p *view.PublishedPage) w.Head {
	h := w.Head{Title: &w.HeadTitle{Text: string(p.Title)}, Pad: w.HeadSnug}
	switch {
	case p.File != nil:
		h.Title.Size, h.Pad = w.HeadTitle19, w.HeadSnug
	case p.Folder != nil:
		h.Title.Size = w.HeadTitle19
	case p.Avatar != nil:
		h.Title.Size = w.HeadTitle19
		h.Photo = &w.HeadPhoto{Initials: string(p.Avatar.Initials), Src: p.Avatar.Href}
	}
	if len(p.Lede) > 0 {
		h.Subs = bind.Sub(p.Lede)
	}
	if f := p.Folder; f != nil {
		h.Crumbs = bind.Crumbs(f.Crumbs)
		if h.Crumbs != nil && f.Total != "" {
			h.Crumbs.Aside = &w.Dim{Parts: []w.Inline{w.Text(lead + string(f.Total))}}
		}
	}
	if p.Filter != nil {
		h.Acts = &w.Acts{Items: []w.ActsPart{filter(*p.Filter)}}
	}
	return h
}

// filter is «Filter by word»: a GET form of its one line, what it keeps
// hidden going with it (the folder it is in). Waiting, its hint gives way
// to «Filter» where the whole one does not fit (Typecase's note 207).
func filter(f view.Form) w.Filter {
	out := w.Filter{Action: f.Action, Hidden: bind.Hidden(f), Variant: w.FilterH26W190}
	for _, r := range bind.Rows(f) {
		out.Query = w.Query{Label: string(r.Label), Name: r.Name, Value: string(r.Value)}
		if r.Value == "" {
			out.Query.Placeholder, out.Query.Short, out.Waiting = string(r.Label), "Filter", true
		}
	}
	// The cross while it holds a word (Typecase's note 129 p. 2); as one
	// types, the page's main narrows (Typecase's note 185).
	out.Clear, out.Swap = f.Clear, listMain
	return out
}

// body is what was published under the head, or what is said instead.
func body(p *view.PublishedPage) []w.MainPart {
	var out []w.MainPart
	if r := p.Range; r != nil {
		bar := w.Bar{}
		for _, a := range r.Presets {
			b := bind.Button(a, "")
			if a.Icon != "" {
				b.Label, b.Title = "", string(a.Label)
			}
			bar.Items = append(bar.Items, b)
		}
		out = append(out, bar)
	}
	if p.Empty != "" {
		return append(out, w.Empty{Parts: []w.EmptyPart{w.Strong(p.Empty)}})
	}
	for _, d := range p.Days {
		list := w.List{}
		for _, r := range d.Rows {
			t := w.TimeRow{Mark: w.Mark{Colour: string(r.Colour)}, Time: string(r.Time), Title: []w.Inline{w.Text(r.Summary)},
				Detail: string(r.Location), Single: true, Href: r.Href}
			if p.Places {
				// The link shows places: a row without one says so, as a
				// list does (s-publish-6); hidden, the column is empty
				// (s-publish-24).
				t.Detail = or(r.Location)
			}
			for _, b := range r.Badges {
				t.Badges = append(t.Badges, bind.Badge(b))
			}
			list.Rows = append(list.Rows, t)
		}
		out = append(out, group(d.Rubric, d.Count), list)
	}
	for _, g := range p.Contacts {
		list := w.List{}
		for _, r := range g.Rows {
			list.Rows = append(list.Rows, w.FaceRow{Mark: w.Mark{Colour: string(r.Colour)}, Avatar: bind.Avatar(r.Avatar),
				Title: []w.Inline{w.Text(r.Name)}, Subtitle: or(r.Phone), Detail: or(r.Email), Caption: or(r.Org), Single: true, Href: r.Href})
		}
		out = append(out, group(g.Rubric, g.Count), list)
	}
	for _, g := range p.Tasks {
		list := w.List{}
		for _, r := range g.Rows {
			list.Rows = append(list.Rows, w.DueRow{Tick: w.Tick{Done: r.Done, Sign: true}, Mark: w.Mark{Colour: string(r.Colour)},
				Title: []w.Inline{w.Text(r.Summary)}, Due: or(r.Due), Progress: progress(r.Percent), Single: true,
				Overdue: r.Overdue, Href: r.Href})
		}
		out = append(out, group(g.Rubric, g.Count), list)
	}
	for _, g := range p.Notes {
		list := w.List{}
		for _, r := range g.Rows {
			list.Rows = append(list.Rows, w.ExcerptRow{Mark: w.Mark{Colour: string(r.Colour)}, Title: []w.Inline{w.Text(r.Title)},
				Excerpt: string(r.Excerpt), Date: string(r.Date), Single: true, Href: r.Href})
		}
		out = append(out, group(g.Rubric, g.Count), list)
	}
	if len(p.Fields) > 0 {
		fs := w.Fields{}
		for _, f := range p.Fields {
			fs.Items = append(fs.Items, bind.FieldsItem(f, lead))
		}
		out = append(out, fs)
	}
	if p.Body != "" {
		out = append(out, w.Sec{Items: []w.SecPart{w.Rubric{Text: description}, w.Prose{Parts: []w.Inline{w.Text(p.Body)}}}})
	}
	if len(p.Attendees) > 0 {
		var lines [][]w.SecItemPart
		for _, a := range p.Attendees {
			lines = append(lines, person(a))
		}
		count := w.Mono{Text: strconv.Itoa(len(p.Attendees)), Variant: w.MonoTight}
		out = append(out, w.Sec{Items: []w.SecPart{w.Rubric{Text: attendees, Items: []w.RubricPart{count}}, w.SecList{Items: lines, Columns: true}}})
	}
	if f := p.File; f != nil {
		switch f.Preview {
		case view.PreviewImage:
			out = append(out, w.MainThumbs{Thumb: w.Thumb{Icon: "img", Form: w.ThumbWide, Src: f.Href, Alt: string(f.Name)}})
		case view.PreviewText, view.PreviewMarkdown:
			out = append(out, text(f))
		default:
			out = append(out, w.MainThumbs{Thumb: w.Thumb{Icon: "doc", Form: w.ThumbWide}})
		}
	}
	if p.Folder != nil {
		out = append(out, table(p.Folder.Rows))
	}
	return out
}

// faces are the faces of a published text: a paragraph after a heading
// leads, code in a chip (s-publish-8, -10).
var faces = bind.TextFaces{Lead: w.ProseLead, After: w.ProseLeadAfter, Tick: w.TickNudge, Code: w.MonoChip}

// text is a published text as the page reads it (s-publish-10): its
// headings and lists as a note's, a paragraph after a section in the body's
// face; a plain text one body.
func text(f *view.PublishedFile) w.Doc {
	if len(f.Blocks) == 0 {
		return w.Doc{Text: true, Items: []w.DocPart{w.DocBody{Text: string(f.Body)}}}
	}
	items := bind.DocText(f.Blocks, faces)
	for i, it := range items {
		if pr, ok := it.(w.Prose); ok && pr.Variant == faces.After {
			if words, plain := plainWords(pr.Parts); plain {
				items[i] = w.DocBody{Text: words}
			}
		}
	}
	return w.Doc{Text: true, Items: items}
}

// plainWords are the words of inlines that are words only.
func plainWords(parts []w.Inline) (string, bool) {
	out := ""
	for _, in := range parts {
		t, ok := in.(w.Text)
		if !ok {
			return "", false
		}
		out += string(t)
	}
	return out, true
}

// note is a published note as its page: the title, when it was written
// (Lede), the text, the files given with it or how many were not.
func note(p *view.PublishedPage) w.DocLayout {
	items := []w.DocPart{w.DocTitle{Text: string(p.Note.Title)}}
	if len(p.Lede) > 0 {
		var sub []w.DocSubPart
		for _, in := range bind.Inlines(p.Lede) {
			if part, ok := in.(w.DocSubPart); ok {
				sub = append(sub, part)
			}
		}
		items = append(items, w.DocSub{Parts: sub})
	}
	items = append(items, bind.DocText(p.Note.Rendered, faces)...)
	if len(p.Attachments) > 0 {
		files := w.DocFiles{Title: w.Rubric{Text: attachments, Variant: w.RubricBelow}}
		for _, a := range p.Attachments {
			files.Items = append(files.Items, w.DocFile{Name: string(a.Name), Size: w.Dim{Parts: []w.Inline{w.Text(lead + string(a.Size))}}, Href: a.Href})
		}
		items = append(items, files)
	}
	if p.AttachNote != "" {
		items = append(items, w.DocNote{Parts: []w.Inline{w.Text(p.AttachNote)}})
	}
	layout := w.DocViewFocus
	for _, a := range p.Width {
		if a.On && a.ID == "width.reading" {
			layout = w.DocViewMeasure
		}
	}
	return w.DocLayout{Items: []w.DocLayoutPart{w.Doc{Items: items}}, View: layout}
}

// table is a published folder's listing: the name leading to the file or
// the folder in this link, its size, type and day, its own download.
func table(rows []view.FileRow) w.Table {
	t := w.Table{Cols: []w.TableCol{
		{Label: colName},
		{Label: colSize, Width: w.ColSize},
		{Label: colType, Width: w.ColChanged, Rank: w.Rank3},
		// What a row's card keeps on a phone (TableCol.Main, Typecase's
		// note 155, Р-66).
		{Label: colChanged, Width: w.ColTime, Rank: w.Rank4, Main: true},
		{Width: w.Col90},
	}}
	for _, r := range rows {
		row := w.TableRow{Cells: []w.TableCell{
			{Parts: []w.CellPart{w.TableFile{Icon: bind.Icon(r.Icon), Name: string(r.Name), Href: r.Open}}},
			{Parts: []w.CellPart{w.Text(or(view.Text(string(r.Size))))}, Mono: true},
			{Parts: []w.CellPart{w.Text(or(r.Type))}, Mono: true},
			{Parts: []w.CellPart{w.Text(or(view.Text(string(r.Modified))))}, Mono: true},
			{Parts: []w.CellPart{w.Button{Label: "Download", Small: true, Href: r.Href}}, Acts: true},
		}}
		if r.IsDir {
			row.State = w.RowFolder
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}

// group is the rubric over a group of rows with how many it holds.
func group(rubric view.Text, n int) w.ListGroup {
	return w.ListGroup{Title: w.Rubric{Text: string(rubric), Form: w.RubricInline}, Count: &w.Num{Value: n}}
}

// person is one of an event's people: the name, or the address
// monospaced, and what they said muted.
func person(a view.AttendeeRow) []w.SecItemPart {
	var out []w.SecItemPart
	if a.Name != "" {
		out = append(out, w.Text(string(a.Name)+" "))
	} else {
		out = append(out, w.Mono{Text: string(a.Email)}, w.Text(" "))
	}
	if a.Status != "" {
		out = append(out, w.Dim{Parts: []w.Inline{w.Text(lead + string(a.Status))}})
	}
	return out
}

// or is a value's words, the dash when there is none.
func or(t view.Text) string {
	if t == "" {
		return none
	}
	return string(t)
}

// progress is a task's share done, the dash without one.
func progress(p *int) string {
	if p == nil {
		return none
	}
	return strconv.Itoa(*p) + percent
}
