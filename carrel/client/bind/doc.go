package bind

import (
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// A note (C-6.9, U-6.4): its text, which the server hands parsed — blocks,
// not markup —, as the parts of a document (the page, s-notes-2, -25) or of
// a panel (s-notes-1); the edit as the document that is its form (Typecase's
// task 98); the notebook chosen with its mark (task 103); a neighbour of the
// rail. The faces the frames draw the text in — the lead's spacing, the
// rubric over tasks, the tick, the chip of inline code — are the screen's
// (TextFaces).

// TextFaces are what the frame draws a note's text in, the screen's: the
// first paragraphs (Lead: ProseLead on the page, ProseRead in a panel), a
// paragraph after a section (After), a task's tick (Tick), inline code
// (Code). The heading over a list of tasks is the text's own heading
// (Typecase's note 122 p. 16).
type TextFaces struct {
	Lead  w.ProseVariant
	After w.ProseVariant
	Tick  w.TickVariant
	Code  w.MonoVariant
}

// paraSep is what joins the paragraphs of one prose, lineSep the lines of
// one paragraph.
const (
	paraSep = "\n\n"
	lineSep = "\n"
)

// Spans are the words of a block that holds text (a paragraph, a heading, an
// item, a cell) in their faces: emphasis italic, strong bold, a link — only
// one the server let go, C-6.9 — leading where it goes, inline code
// monospaced in the screen's face; the rest plain words.
func Spans(b view.Block, f TextFaces) []w.Inline {
	if len(b.Children) == 0 {
		if b.Text == "" {
			return nil
		}
		return []w.Inline{span(b, f)}
	}
	var out []w.Inline
	for _, c := range b.Children {
		out = addInline(out, span(c, f))
	}
	return out
}

// span is one piece of a paragraph's words in its face.
func span(b view.Block, f TextFaces) w.Inline {
	switch b.Kind {
	case view.BlockEmph:
		return w.Em(flat(b))
	case view.BlockStrong:
		return w.Strong(flat(b))
	case view.BlockLink:
		return w.Link{Text: flat(b), Href: b.Href}
	case view.BlockCode:
		return w.Mono{Text: string(b.Text), Variant: f.Code}
	}
	return w.Text(flat(b))
}

// flat is the words of a block without their faces: a task's line, a
// heading, a link's words.
func flat(b view.Block) string {
	s := string(b.Text)
	for _, c := range b.Children {
		s += flat(c)
	}
	return s
}

// addInline appends a piece to running words, plain words to plain words
// as one: the frames draw a run of paragraphs as one piece of text.
func addInline(out []w.Inline, in w.Inline) []w.Inline {
	if t, ok := in.(w.Text); ok && len(out) > 0 {
		if last, ok := out[len(out)-1].(w.Text); ok {
			out[len(out)-1] = last + t
			return out
		}
	}
	return append(out, in)
}

// joinInlines appends a paragraph's words to a prose, sep between them.
func joinInlines(out []w.Inline, sep string, more []w.Inline) []w.Inline {
	if len(out) > 0 && len(more) > 0 {
		out = addInline(out, w.Text(sep))
	}
	for _, in := range more {
		out = addInline(out, in)
	}
	return out
}

// isTasks tells whether a list is a list of tasks («- [ ]»): every item a
// task.
func isTasks(b view.Block) bool {
	if b.Kind != view.BlockList || len(b.Children) == 0 {
		return false
	}
	for _, c := range b.Children {
		if c.Kind != view.BlockTask {
			return false
		}
	}
	return true
}

// tasks are the items of a list of tasks, each its tick — a sign that it is
// done or not, which is not pressed: a note is changed in its edit
// (Typecase's task 99, U-6.4) — and its line.
func tasks(b view.Block, f TextFaces) []w.DocTask {
	out := make([]w.DocTask, 0, len(b.Children))
	for _, c := range b.Children {
		out = append(out, w.DocTask{Tick: w.Tick{Done: c.Checked, Variant: f.Tick, Sign: true}, Text: flat(c)})
	}
	return out
}

// blockish tells whether an item's parts are blocks — a paragraph and a
// list nested in it — rather than the words of its one paragraph.
func blockish(b view.Block) bool {
	for _, c := range b.Children {
		switch c.Kind {
		case view.BlockList, view.BlockQuote, view.BlockTable, view.BlockHeading:
			return true
		case view.BlockPara:
			if len(c.Children) > 0 {
				return true
			}
		}
	}
	return false
}

// bullets are the items of a list, each its words and, at its end, the list
// nested in it (Typecase's task 102).
func bullets(b view.Block, f TextFaces) w.DocBullets {
	var out w.DocBullets
	for _, it := range b.Children {
		if !blockish(it) {
			out.Items = append(out.Items, Spans(it, f))
			continue
		}
		var line []w.Inline
		for _, c := range it.Children {
			if c.Kind == view.BlockList {
				line = append(line, bullets(c, f))
				continue
			}
			line = joinInlines(line, lineSep, Spans(c, f))
		}
		out.Items = append(out.Items, line)
	}
	return out
}

// quote is a quotation's paragraphs as its words.
func quote(b view.Block, f TextFaces) w.DocQuote {
	var parts []w.Inline
	for _, c := range b.Children {
		parts = joinInlines(parts, paraSep, Spans(c, f))
	}
	return w.DocQuote{Parts: parts}
}

// aligns are a table's columns' alignments (C-8 CellAlign) as the library's:
// as written, centred, at the end.
var aligns = map[view.CellAlign]w.DocAlign{view.AlignCenter: w.DocAlignCenter, view.AlignRight: w.DocAlignEnd}

// table is a table of the text: its head (the row with Header), its rows,
// each column aligned as its cells say, over the switch Wrap / One line at
// Wrap.
func table(b view.Block, f TextFaces) w.DocTable {
	t := w.DocTable{Switch: w.DocTableSwitch(false)}
	for _, r := range b.Children {
		var cells [][]w.Inline
		for i, c := range r.Children {
			cells = append(cells, Spans(c, f))
			if i >= len(t.Align) {
				t.Align = append(t.Align, aligns[c.Align])
			}
		}
		if r.Header {
			t.Head = cells
		} else {
			t.Rows = append(t.Rows, cells)
		}
	}
	return t
}

// DocText is a note's text (C-6.9 Block) as the parts of its document: the
// paragraphs before any other part one prose in the face of the lead, a
// paragraph after a section in its own face; a heading over a list of tasks
// the rubric of the tasks, another heading a heading; a list of tasks with
// their ticks, another list bulleted with its nested lists; a quotation; a
// block of code as written; a table (s-notes-2, -21, -25). The title is the
// screen's, above them.
func DocText(blocks []view.Block, f TextFaces) []w.DocPart {
	var out []w.DocPart
	var prose []w.Inline
	flush := func() {
		if len(prose) == 0 {
			return
		}
		v := f.After
		if len(out) == 0 {
			v = f.Lead
		}
		out = append(out, w.Prose{Parts: prose, Variant: v})
		prose = nil
	}
	for i := 0; i < len(blocks); i++ {
		b := blocks[i]
		if b.Kind == view.BlockPara {
			prose = joinInlines(prose, paraSep, Spans(b, f))
			continue
		}
		flush()
		switch {
		case b.Kind == view.BlockHeading && i+1 < len(blocks) && isTasks(blocks[i+1]):
			out = append(out, w.DocTasks{Title: flat(b), Items: tasks(blocks[i+1], f)})
			i++
		case b.Kind == view.BlockHeading:
			out = append(out, w.DocHeading{Text: flat(b)})
		case isTasks(b):
			out = append(out, w.DocTasks{Items: tasks(b, f)})
		case b.Kind == view.BlockList:
			out = append(out, bullets(b, f))
		case b.Kind == view.BlockQuote:
			out = append(out, quote(b, f))
		case b.Kind == view.BlockCode:
			out = append(out, w.DocCode{Text: string(b.Text)})
		case b.Kind == view.BlockTable:
			out = append(out, table(b, f))
		default:
			prose = joinInlines(prose, paraSep, Spans(b, f))
		}
	}
	flush()
	return out
}

// SecText is a note's text as the parts of a panel (s-notes-1, -28): what
// its page has (Typecase's task 125) — its paragraphs a prose in the face of
// the lead, a heading, a list of tasks with their ticks, another list
// bulleted with its nested lists, a quotation, a block of code as written,
// a table. A panel's tasks have no rubric: a heading over them is a heading.
func SecText(blocks []view.Block, f TextFaces) []w.SecPart {
	var out []w.SecPart
	var prose []w.Inline
	flush := func() {
		if len(prose) > 0 {
			out = append(out, w.Prose{Parts: prose, Variant: f.Lead})
			prose = nil
		}
	}
	for _, b := range blocks {
		if b.Kind == view.BlockPara {
			prose = joinInlines(prose, paraSep, Spans(b, f))
			continue
		}
		flush()
		switch {
		case b.Kind == view.BlockHeading:
			out = append(out, w.DocHeading{Text: flat(b)})
		case isTasks(b):
			out = append(out, w.SecTasks{Items: tasks(b, f)})
		case b.Kind == view.BlockList:
			out = append(out, bullets(b, f))
		case b.Kind == view.BlockQuote:
			out = append(out, quote(b, f))
		case b.Kind == view.BlockCode:
			out = append(out, w.DocCode{Text: string(b.Text)})
		case b.Kind == view.BlockTable:
			out = append(out, table(b, f))
		default:
			prose = joinInlines(prose, paraSep, Spans(b, f))
		}
	}
	flush()
	return out
}

// DocForm is a note's edit as the document that is its form (NoteDoc.Form,
// Typecase's task 98): where and how it sends, its hidden rows (the token,
// the version), under the ID the edit bar's Save names (SubmitForm); its
// parts — the title, the fields, the text — are the screen's. A note read
// has no form: an empty document.
func DocForm(f *view.Form, id string) w.Doc {
	if f == nil {
		return w.Doc{}
	}
	return w.Doc{Action: f.Action, Method: f.Method, Fields: Hidden(*f), ID: id}
}

// SubmitForm is an action that sends a form (Submits) from outside it — Save
// of the edit bar, which stands over the document it sends (s-notes-20) —
// as the button that submits the form of the ID to the action's own address;
// switched off, it says why and sends nothing.
func SubmitForm(a view.Action, form string) w.Button {
	b := w.Button{Label: string(a.Label), Icon: Icon(a.Icon), Kind: Kind(a)}
	if !a.Enabled {
		b.State, b.Reason = w.ButtonDisabled, string(a.Reason)
		return b
	}
	b.Form, b.To = form, a.Href
	return b
}

// SavedIn is where a record is saved, in the head of its edit left of
// Cancel (Typecase 434ac87, note 72): the collection's choice (Marks) as a
// field with no label — the row's name is the choice's title —, its menu
// 290 wide under it; it stands outside the edit's form, so each radio names
// it (MenuItem.Of, Typecase 96b9973, note 76).
func SavedIn(r view.FormRow, ro w.Badge, form string) w.Field {
	d := Marks(r, ro)
	d.Menu.Width, d.Menu.Pop = w.MenuWidth290, w.MenuUnder
	for i, p := range d.Menu.Items {
		if it, ok := p.(w.MenuItem); ok {
			it.Of = form
			d.Menu.Items[i] = it
		}
	}
	return w.Field{Parts: []w.FieldPart{d}, Width: w.FieldMD}
}

// Marks is a choice of a collection (C-6.8 InputSelect whose options carry
// their colour: a note's notebook, s-notes-15, -20; Typecase's task 103) as a
// choice that opens its menu: the chosen one's words after its mark; the
// menu each option a radio item of the form under the row's name, its mark,
// the chosen one on, a rule before one that has it; one that cannot be chosen
// (Disabled: read-only) shown and off, its reason its hint, ro — the screen's
// badge — at its end. How wide the menu is and where it pops are the
// screen's.
func Marks(r view.FormRow, ro w.Badge) w.Drop {
	p := w.Pick{Label: string(r.Label), Focus: r.Focus}
	var items []w.MenuPart
	for _, o := range r.Options {
		if o.Separator && len(items) > 0 {
			items = append(items, w.MenuSep{})
		}
		it := w.MenuItem{Label: string(o.Label), On: o.Selected, Name: r.Name, Value: o.Value}
		if o.Colour != "" {
			it.Mark = &w.Mark{Colour: string(o.Colour)}
		}
		if o.Disabled {
			it.Reason, it.Trail = string(o.Reason), ro
		}
		if o.Selected {
			p.Value = string(o.Label)
			if it.Mark != nil {
				m := *it.Mark
				p.Mark = &m
			}
		}
		items = append(items, it)
	}
	return w.Drop{Trigger: p, Menu: w.Menu{Items: items}}
}

// TreeRow is a record of the rail's tree (Rail.Neighbours: the notes of the
// notebook beside the one open, s-notes-2) as its row: its icon, its words,
// where it goes.
func TreeRow(a view.Action) w.TreeRow {
	return w.TreeRow{Icon: Icon(a.Icon), Label: string(a.Label), Href: a.Href}
}
