// Package importer is the import wizard, /app/{kind}/{account}/{col}/import
// and /app/{kind}/import (ImportPage; U-6.9), drawn by frames s-io-3 … -9,
// -14 … -19: one dialog, three steps — the source, the preview, the report —
// under the steps' line, the shell around it.
package importer

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The screen's words (U-1.3): the shell, the rubrics of the step and of the
// report, the field's buttons, and the words the model leaves to the screen
// by kind (the client's scan of the import, ИБ2, ИБ3, ИБ5).
var (
	top = bind.TopWords{
		Brand:  w.Brand{Logo: &w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63"}},
		Search: "Search everything…",
		NewKey: "n", SearchKey: "/",
		Theme: bind.ThemeWords{Label: "Theme", Auto: "Auto", Light: "Light", Dark: "Dark"},
		Keys:  bind.KeysWords{Label: "Keyboard shortcuts", Icon: "search", Key: "?", Sheet: "keys"},
		More:  w.Button{Icon: "chev", Title: "More to create"},
	}
	rail = bind.RailWords{Toggle: "toggle", None: "—", ReadOnly: "ro", Tags: "Tags", Davloom: "davloom"}
	tree = bind.TreeWords{Folder: "file", File: "file", ReadOnly: "ro"}
)

const (
	into       = "Into"
	fromFile   = "From a file"
	fromFolder = "From a WebDAV folder"
	browse     = "Browse"
	fileID     = "import-file"
	noFile     = "No file chosen"
	dash       = "—"
)

// The report's rubrics (s-io-9, -14).
const (
	created  = "Created"
	here     = "Already here, skipped"
	replaced = "Replaced"
	copies   = "Imported as copies"
	newUIDs  = "With a new UID"
	notKept  = "Not kept by the server"
	failed   = "Failed"
	skipped  = "Skipped"
)

// fileLabels are the field of the file by kind (s-io-3, -4, -15).
var fileLabels = map[view.Kind]string{
	view.KindContact: "vCard file", view.KindEvent: "iCalendar file", view.KindTask: "iCalendar file",
	view.KindNote: "Markdown, a zip of them, or .ics",
}

// intoHints are the line under «Into» by kind, on the first step and on
// the preview (s-io-3 … -8, -15 … -17).
var intoHints = map[view.Kind][2]string{
	view.KindContact: {"Any address book you may write to, on every connected server.", "Any address book you may write to. What is already here is counted against it."},
	view.KindEvent:   {"Any calendar you may write to, on every connected server.", "Calendars only: a list of tasks is a different collection."},
	view.KindTask:    {"Any list of tasks you may write to, on every connected server.", "Lists of tasks only: a calendar is a different collection."},
	view.KindNote:    {"The notebook the notes are created in. Every source below lands there.", "Any notebook you may write to. A WebDAV folder from the step before lands in this same table."},
}

// intoUnset is «Into» before it is chosen (s-io-17).
var intoUnset = map[view.Kind]string{
	view.KindContact: "Choose an address book", view.KindEvent: "Choose a calendar",
	view.KindTask: "Choose a list of tasks", view.KindNote: "Choose a notebook",
}

// Screen draws the wizard at its step (ImportPage.Step).
//
//   - The steps' line (Steps), the title, the file and where it went
//     (Subtitle) — on the report with the collection's mark.
//   - The notices of the step in the dialog, under its head: a refusal of
//     the first step (s-io-18), the collection that did not answer with Try
//     again (s-io-19).
//   - The source: «Into» — the collection chosen with its mark, or the
//     words that ask for one (s-io-17) —, the file field with Browse and
//     Preview file in a card (the dialog is the form that sends it), what
//     the kind takes under it; for notes, the folders of WebDAV with Preview
//     folder; the step's message.
//   - The preview: «Into», the question about the records already here (a
//     choice of links), the counts, one table — the result last, a record
//     that failed in alarm, one left out dusty, a stray kind's badge —, the
//     step's message; Import and Cancel in the head.
//   - The report: a card a count — created, already here, replaced, copies,
//     a new UID, not kept, failed, skipped —, each its records under it;
//     Back to list.
func Screen(p *view.ImportPage) w.Shell {
	d := w.Dialog{Variant: w.DialogStep560}
	steps := w.Steps{Items: texts(p.Steps), On: 1}
	switch p.Step {
	case view.ImportStepPreview:
		steps.On, d.Variant = 2, w.DialogWide
	case view.ImportStepReport:
		steps.On, d.Variant = 3, w.DialogStep520
	}
	head := w.Head{Title: &w.HeadTitle{Text: string(p.Title)}}
	if len(p.Subtitle) > 0 {
		head.Subs = []w.HeadSub{subtitle(p.Subtitle)}
	}
	d.Items = append(d.Items, steps)
	if p.Step == view.ImportStepPreview {
		head.Acts = &w.Acts{Items: headActs(p.Actions)}
	}
	d.Items = append(d.Items, head)
	for _, n := range p.Notices {
		d.Items = append(d.Items, bind.Notice(n, w.MsgFlushSides))
		if nd := bind.NoticeDiag(n); nd != nil {
			d.Items = append(d.Items, nd.Diag)
		}
	}
	switch p.Step {
	case view.ImportStepPreview:
		d.Items = append(d.Items, preview(p)...)
	case view.ImportStepReport:
		d.Items = append(d.Items, report(p)...)
	default:
		d = source(p, d)
	}
	t := bind.Top(p.Top, top)
	return w.Shell{Top: &t, Body: w.ShellBody{Rail: railParts(p.Rail), Main: w.Main{Items: []w.MainPart{d}}}}
}

// Forms are the forms the page prints at its end: the top's, the notices',
// and each action that posts on its own — Import, Cancel, Preview folder.
func Forms(p *view.ImportPage) []w.PostForm {
	out := append(bind.TopForms(p.Top), bind.NoticeForms(p.Notices)...)
	acts := p.Actions
	if p.Source.Preview != nil {
		acts = append(acts, *p.Source.Preview)
	}
	for _, a := range acts {
		if a.Method == "POST" && a.Enabled && a.File == "" && a.Href != "" {
			out = append(out, bind.ActionForm(a, a.ID))
		}
	}
	return out
}

func railParts(r view.Rail) []w.RailPart {
	nav := w.Nav{}
	for _, n := range r.Nav {
		// The way back says «Back» on a narrow rail (s-io-3 … -9).
		it := bind.NavItem(n)
		it.Short = "Back"
		nav.Items = append(nav.Items, it)
	}
	parts := []w.RailPart{nav}
	parts = append(parts, bind.Sources(r, rail)...)
	return append(parts, bind.Foot(r, rail))
}

func texts(ts []view.Text) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = string(t)
	}
	return out
}

// subtitle is the line under the title: the file monospaced, its size and
// format; on the report, where it went with its mark.
func subtitle(r view.Runs) w.HeadSub {
	var parts []w.HeadSubPart
	for _, run := range r {
		if run.Colour != "" {
			parts = append(parts, w.Mark{Colour: string(run.Colour), Size: w.MarkH11}, w.Text(run.Text))
			continue
		}
		for _, in := range bind.Inlines(view.Runs{run}) {
			if hp, ok := in.(w.HeadSubPart); ok {
				parts = append(parts, hp)
			}
		}
	}
	return w.HeadSub{Parts: parts}
}

// headActs are the preview's Cancel and Import (s-io-5).
func headActs(actions []view.Action) []w.ActsPart {
	var cancel, run []w.ActsPart
	for _, a := range actions {
		b := bind.Button(a, a.ID)
		if !a.Enabled {
			b.State, b.Reason = w.ButtonDisabled, string(a.Reason)
		}
		if a.Primary {
			run = append(run, b)
		} else {
			cancel = append(cancel, b)
		}
	}
	return append(cancel, run...)
}

// intoField is «Into»: the collection chosen with its mark, or the words
// asking for one; the line under it by kind and step. It opens its menu
// (note 115, s-io-10 … -13), as «Saved in» of an edit: each collection a
// link to the same step with it chosen (TargetOption.Href), the chosen one
// on, one that cannot be written to off with its reason and ro; under the
// rule a new collection of the kind (TargetPicker.New).
func intoField(p *view.ImportPage, step int, width w.FieldWidth) w.Field {
	pick := w.Pick{Label: into, Value: intoUnset[p.Kind], Unset: true}
	var items []w.MenuPart
	for _, o := range p.Into.Options {
		it := w.MenuItem{Label: string(o.Stamp), Mark: &w.Mark{Colour: string(o.Colour)}, Href: o.Href}
		if o.ID == p.Into.Chosen && p.Into.Chosen != "" {
			it.On = true
			pick = w.Pick{Label: into, Value: string(o.Stamp), Mark: &w.Mark{Colour: string(o.Colour)}}
		}
		if o.ReadOnly {
			it.Href, it.Reason, it.Trail = "", string(o.Reason), ro
		}
		items = append(items, it)
	}
	if n := p.Into.New; n != nil {
		items = append(items, w.MenuSep{}, w.MenuItem{Label: string(n.Label), Href: n.Href})
	}
	d := w.Drop{Trigger: pick, Menu: w.Menu{Items: items, Width: w.MenuWidth290, Pop: w.MenuUnder}}
	return w.Field{Label: into, Width: width,
		Parts: []w.FieldPart{d, w.FieldHint{Parts: []w.Inline{w.Text(intoHints[p.Kind][step])}}}}
}

// ro marks a collection that cannot be written to (s-io-3).
var ro = w.Badge{Label: "ro", Place: w.Pushed}

// FormID is the ID of the first step's dialog, the form a drop sheet names
// (Drop).
const FormID = "import-source"

// Drop is the drop sheet of the first step (ImportSource.Drop): the
// server's words and refusals, the dropped file into the dialog's file
// field; none on the other steps, or while nothing sends the file.
func Drop(p *view.ImportPage, icon w.Icon) (w.Scrim, bool) {
	if p.Source.Drop == nil || p.Step == view.ImportStepPreview || p.Step == view.ImportStepReport {
		return w.Scrim{}, false
	}
	for _, a := range p.Actions {
		if a.ID == "import.choose" {
			return bind.ImportSheet(*p.Source.Drop, FormID, icon), true
		}
	}
	return w.Scrim{}, false
}

// source is the first step: «Into», the file, the folders of notes, the
// step's message; the dialog sends the file.
func source(p *view.ImportPage, d w.Dialog) w.Dialog {
	d.Items = append(d.Items, w.Form{Items: []w.FormPart{intoField(p, 0, w.FieldW100)}, Variant: w.FormBottom, Tight: true})
	var choose *view.Action
	for i, a := range p.Actions {
		if a.ID == "import.choose" {
			choose = &p.Actions[i]
		}
	}
	accept := ""
	for i, k := range p.Source.Kinds {
		if i > 0 {
			accept += ","
		}
		accept += string(k)
	}
	label := fileLabels[p.Kind]
	taken := ""
	for _, r := range p.Source.Taken {
		taken += string(r.Text)
	}
	// The name of the file taken by a drop, monospaced (s-io-17); without
	// one, what the field holds when nothing is chosen.
	shown := w.Input{Placeholder: noFile, Label: label, Picked: true, ID: fileID, Variant: w.InputFill}
	if taken != "" {
		shown = w.Input{Value: taken, Label: taken, Mono: true, Picked: true, ID: fileID, Variant: w.InputFill}
	}
	field := w.FileField{Name: "file", Accept: accept, Label: label, Shows: fileID, Many: p.Source.Many}
	submit := w.Button{Label: browse, File: &field}
	preview := w.Button{Label: "Preview file", Kind: w.ButtonPrimary, Submit: true}
	if choose != nil {
		preview.Label = string(choose.Label)
		// The dialog is the form that sends the file (U-8): to the step's
		// address, as the action says, with the session's token it carries
		// (C-11) — without a script nothing else sends it.
		d.Action, d.Method = choose.Href, choose.Method
		d.Fields = bind.ActionForm(*choose, "").Fields
		if p.Source.Drop != nil {
			// A file dropped on the window stands in this field (Typecase's
			// note 209): the drop sheet names the dialog's form (Drop).
			d.ID = FormID
		}
		if !choose.Enabled {
			preview.State, preview.Reason = w.ButtonDisabled, string(choose.Reason)
		}
	}
	card := w.Card{Items: []w.CardPart{
		w.Rubric{Text: fromFile},
		w.Field{Label: label, Width: w.FieldW100,
			Parts: []w.FieldPart{w.Line{Items: []w.LinePart{shown, submit, preview}, Gap: w.LineGapNormal}}},
	}}
	if len(p.Source.Hint) > 0 {
		card.Items = append(card.Items, w.CardHint{Parts: cardText(p.Source.Hint), Line: true})
	}
	d.Items = append(d.Items, card)
	if p.Source.Folder != nil {
		folder := w.Card{Variant: w.CardChosen, Items: []w.CardPart{w.Rubric{Text: fromFolder}, bind.FileTree(*p.Source.Folder, tree)}}
		row := w.CardRow{Kind: w.CardTop8, Gap: w.LineGapNormal, Items: []w.CardRowPart{w.FillHint{Parts: cardText(p.Source.FolderHint)}}}
		if a := p.Source.Preview; a != nil {
			b := bind.Button(*a, a.ID)
			if !a.Enabled {
				b.State, b.Reason = w.ButtonDisabled, string(a.Reason)
			}
			row.Items = append(row.Items, b)
		}
		folder.Items = append(folder.Items, row)
		d.Items = append(d.Items, folder)
	}
	if len(p.Source.Note) > 0 {
		d.Items = append(d.Items, w.Msg{Parts: msgParts(p.Source.Note), Space: w.MsgFlushSides})
	}
	return d
}

// preview is the second step (s-io-5 … -8).
func preview(p *view.ImportPage) []w.DialogPart {
	form := w.Form{Items: []w.FormPart{intoField(p, 1, w.FieldLG)}, Tight: true}
	if e := p.Existing; e != nil {
		seg := w.Seg{}
		for _, o := range e.Options {
			seg.Options = append(seg.Options, w.SegOption{Label: string(o.Label), Href: o.Href, On: o.On})
		}
		parts := []w.FieldPart{seg}
		if e.Hint != "" {
			parts = append(parts, w.FieldHint{Parts: []w.Inline{w.Text(e.Hint)}})
		}
		form.Items = append(form.Items, w.Field{Label: string(e.Note), Parts: parts, OwnLine: true})
	}
	out := []w.DialogPart{form}
	if len(p.Counts) > 0 {
		stats := w.Stats{}
		for _, c := range p.Counts {
			stats.Items = append(stats.Items, w.Stat{Value: itoa(c.Count), Label: string(c.Label), Error: c.Error})
		}
		out = append(out, stats)
	}
	for _, t := range p.Preview {
		out = append(out, table(t, p.Kind))
	}
	if len(p.Source.Note) > 0 {
		out = append(out, w.Msg{Parts: msgParts(p.Source.Note), Space: w.MsgFlushSides})
	}
	return out
}

// table is the preview's table: the file of a set monospaced — one file
// has no such column, the head names it (Typecase's note 157) —, a
// contact's address and a missing value monospaced — a time or a day in
// the card's words is not (Typecase's note 142) —, the result last and the
// main column of a card on a phone — a record that failed in alarm, one
// left out by the kind dusty —, a missing value «—» dusty. The columns are
// read from the end: result, the date or address, the name, the file.
func table(t view.ImportTable, k view.Kind) w.Table {
	out := w.Table{}
	n := len(t.Columns)
	result, detail, name := n-1, n-2, n-3
	for i, c := range t.Columns {
		col := w.TableCol{Label: string(c)}
		switch i {
		case detail:
			col.Rank = w.Rank3
		case result:
			col.Rank, col.Main = w.Rank4, true
		}
		out.Cols = append(out.Cols, col)
	}
	for _, l := range t.Lines {
		row := w.TableRow{}
		for i, c := range l.Cells {
			cell := w.TableCell{Parts: []w.CellPart{w.Text(c)}}
			switch i {
			case name:
				cell.Dim = l.Untitled || c == dash
				if l.Component != "" {
					cell.Parts = append(cell.Parts, w.Badge{Label: string(l.Component)})
				}
			case detail:
				cell.Mono = k == view.KindContact
				if c == "" || c == dash {
					cell.Parts, cell.Tone, cell.Mono = []w.CellPart{w.Text(dash)}, w.CellDusty, true
				}
			case result:
				switch {
				case l.Result == view.ImportFailed:
					cell.Tone = w.CellDanger
				case l.Result == view.ImportSkipped && l.Component != "", l.Result == view.ImportSkipped && startsWith(string(c), "Skipped"):
					cell.Tone = w.CellDusty
				}
			default:
				cell.Mono = true
			}
			row.Cells = append(row.Cells, cell)
		}
		out.Rows = append(out.Rows, row)
	}
	return out
}

// report is the third step (s-io-9, -14): a card a count, the records under
// each, and the way back.
func report(p *view.ImportPage) []w.DialogPart {
	r := p.Report
	if r == nil {
		return nil
	}
	out := []w.DialogPart{w.Card{Items: []w.CardPart{w.Rubric{Text: created}, w.CardText{Kind: w.CardNum, Parts: []w.CardTextPart{w.Text(itoa(r.Created))}}}}}
	for _, c := range []struct {
		title string
		n     int
		lines []view.Text
		bad   bool
	}{
		{here, r.Here, r.Heres, false},
		{replaced, r.Replaced, r.Replaces, false},
		{copies, len(r.Renames), r.Renames, false},
		{newUIDs, len(r.NewUIDs), r.NewUIDs, false},
		{notKept, r.Lost, r.Losts, false},
		{failed, r.Failed, r.Failures, true},
		{skipped, r.Skipped, r.Skips, false},
	} {
		if c.n == 0 {
			continue
		}
		card := w.Card{Items: []w.CardPart{w.Rubric{Text: c.title, Items: []w.RubricPart{w.Mono{Text: itoa(c.n), Variant: w.MonoTight}}}}}
		if len(c.lines) > 0 {
			text := ""
			for i, l := range c.lines {
				if i > 0 {
					text += "\n"
				}
				text += string(l)
			}
			diag := w.Diag{Text: text}
			if c.bad {
				diag.Variant = w.DiagDanger
			}
			card.Items = append(card.Items, diag)
		}
		out = append(out, card)
	}
	var foot []w.FormFootPart
	for _, a := range p.Actions {
		foot = append(foot, bind.Button(a, a.ID))
	}
	return append(out, w.FormFoot{Items: foot, Variant: w.FormFootFlush})
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

// cardText are runs as the text of a card, each in its face.
func cardText(r view.Runs) []w.CardTextPart {
	var out []w.CardTextPart
	for _, in := range bind.Inlines(r) {
		if cp, ok := in.(w.CardTextPart); ok {
			out = append(out, cp)
		}
	}
	return out
}

func startsWith(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }

// itoa is a count in figures.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
