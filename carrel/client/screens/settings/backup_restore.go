package settings

import (
	"strconv"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of the restore (U-1.3): its steps, the rubric of the archive's
// password, the columns of the matching, how many rows are ticked, an empty
// target.
const (
	stepArchive   = "1 · Archive"
	stepMatch     = "2 · Match"
	stepReport    = "3 · Report"
	archivePw     = "Archive password"
	whereArchive  = "Where the archive is"
	colInArchive  = "In the archive"
	colRecords    = "Records"
	colInto       = "Restore into"
	colWill       = "What will happen"
	selectedWord  = " selected"
	noTarget      = "—"
	restoreAllID  = "restore.all"
	restoreRunID  = "restore.run"
	restoreIntoID = "restore.into"
	restoreSkipID = "restore.skip"
	restoreNewVal = "new"
	restoreNoVal  = "skip"
	chooseFile    = "Choose a file"
	colHappened   = "What happened"
	colRestored   = "Restored into"
	// restoreZone is the zone of the page's column (C-7.2 main) the steps
	// of an archive from this computer answer into.
	restoreZone = "main"
)

// sourceWidths are the widths the frames give the fields of a source
// (s-backup-15, -17).
var sourceWidths = map[string]w.FieldWidth{"file": w.FieldFull, "store": w.FieldMD, "folder": w.FieldLG, "path": w.FieldXL}

// restoreMain is the restore on its own page (RestoreBlock): where the
// archive is and its password (s-backup-7), or the archive opened and what
// of it goes where (s-backup-8).
func restoreMain(p *view.BackupPage) []w.MainPart {
	r := p.Restore
	main := bind.Notices(p.Notices, w.MsgSpacePage)
	if rep := r.Report; rep != nil {
		return append(main, reportDialog(rep))
	}
	if m := r.Match; m != nil {
		return append(main, matchDialog(r, m))
	}
	return append(main, sourceDialog(r))
}

// reportDialog is the third step (s-backup-18): what was restored from the
// archive's day, the sums under it, Done; a row a collection — its mark and
// name, where it went, how many were created, what happened, a refusal in
// alarm, «no differences» muted.
func reportDialog(rep *view.RestoreReport) w.Dialog {
	h := w.Head{Title: &w.HeadTitle{Text: string(rep.Title)}, Acts: &w.Acts{Items: []w.ActsPart{bind.Button(rep.Done, "")}}}
	if rep.Subtitle != "" {
		h.Subs = []w.HeadSub{{Parts: []w.HeadSubPart{w.Text(rep.Subtitle)}}}
	}
	t := w.Table{Variant: w.TableFit, Cols: []w.TableCol{
		{Label: colInArchive}, {Label: colRestored, Width: w.Col26, Rank: w.Rank4}, {Label: colCreated, Rank: w.Rank3, Secondary: true}, {Label: colHappened},
	}}
	for _, r := range rep.Rows {
		happened := w.TableCell{Parts: as[w.CellPart](bind.Inlines(r.Happened))}
		if r.Same {
			happened.Dim = true
		}
		t.Rows = append(t.Rows, w.TableRow{Cells: []w.TableCell{
			{Parts: []w.CellPart{w.TableFile{Mark: &w.Mark{Colour: string(r.Colour)}, Name: string(r.Name)}}},
			{Parts: []w.CellPart{w.Text(r.Into)}, Mono: true},
			{Parts: []w.CellPart{w.Text(strconv.Itoa(r.Created))}, Mono: true},
			happened,
		}})
	}
	return w.Dialog{Variant: w.DialogWide, Items: []w.DialogPart{w.Steps{Items: []string{stepArchive, stepMatch, stepReport}, On: 3}, h, t}}
}

// sourceDialog is the first step (s-backup-7): where the archive is — one
// of three, a link each —, what that one asks — a job's copies a row each,
// a server's storage and path —, the archive's password, Open the manifest
// and Cancel.
func sourceDialog(r view.RestoreBlock) w.Dialog {
	d := w.Dialog{Variant: w.DialogStep760, Action: r.Source.Action, Method: r.Source.Method, Fields: bind.Hidden(r.Source)}
	d.Items = append(d.Items, w.Steps{Items: []string{stepArchive, stepMatch, stepReport}, On: 1},
		w.Head{Title: &w.HeadTitle{Text: string(r.Title)}, Subs: bind.Sub(r.Subtitle)})
	from := w.Seg{Variant: w.SegStart}
	for _, a := range r.From {
		from.Options = append(from.Options, bind.SegOption(a))
	}
	f := w.Field{Label: whereArchive, Parts: []w.FieldPart{from}, OwnLine: true}
	if r.FromHint != "" {
		f.Parts = append(f.Parts, w.FieldHint{Parts: []w.Inline{w.Text(r.FromHint)}})
	}
	d.Items = append(d.Items, f)
	// A card a set (s-backup-7, -15, -17): the source's, its name its
	// rubric, a refusal at its top; the archive's password.
	for _, set := range r.Source.Sets {
		if set.Legend == "" {
			continue
		}
		card := w.Card{Items: []w.CardPart{w.Rubric{Text: string(set.Legend)}}}
		if len(r.Refusal) > 0 && set.Legend != archivePw {
			card.Items = append(card.Items, w.Msg{Parts: bind.MsgParts(r.Refusal), Tone: w.MsgAlert, Space: w.MsgFlushBottom12})
		}
		fields := w.CardFields{Wide: true, Top: true}
		for _, row := range set.Rows {
			switch {
			case row.Kind == view.InputHidden:
			case row.Kind == view.InputRadio:
				card.Items = append(card.Items, copiesTable(row))
			case row.Kind == view.InputPassword:
				pw := bind.Input(row)
				pw.Focus = true
				f := w.Field{Parts: []w.FieldPart{pw}, Width: w.FieldLG}
				if row.Hint != "" {
					// The hint is the field's (s-backup-7, Typecase's note 97).
					f.Parts = append(f.Parts, w.FieldHint{Parts: []w.Inline{w.Text(row.Hint)}})
				}
				fields.Wide, fields.Top = false, false
				fields.Items = append(fields.Items, f)
			case row.Kind == view.InputFile:
				// The archive picked on this computer goes with the form
				// (s-backup-15).
				// With the live layer it goes in the parts the server names
				// (FileField.Ranges, Typecase's notes 98, 109).
				b := w.Button{Label: chooseFile, File: &w.FileField{Name: row.Name, Accept: row.Accept, Label: string(row.Label), Ranges: r.Ranges}}
				fields.Items = append(fields.Items, w.Field{Label: string(row.Label), Width: sourceWidths[row.Name],
					Parts: []w.FieldPart{w.Line{Items: []w.LinePart{b}, Gap: w.LineGap8}}})
			case row.Extra != nil:
				// The archive's path with Browse (s-backup-17).
				in := bind.Input(row)
				in.Variant = w.InputFill
				fields.Items = append(fields.Items, w.Field{Label: string(row.Label), Width: sourceWidths[row.Name],
					Parts: append([]w.FieldPart{w.Line{Items: []w.LinePart{in, bind.Button(*row.Extra, "")}, Gap: w.LineGap8}}, bind.Hint(row)...)})
			default:
				fields.Items = append(fields.Items, bind.Field(row, sourceWidths[row.Name]))
			}
		}
		if len(fields.Items) > 0 {
			card.Items = append(card.Items, fields)
		}
		d.Items = append(d.Items, card)
	}
	foot := w.FormFoot{}
	for _, a := range r.Source.Actions {
		switch {
		case a.File != "":
			foot.Items = append(foot.Items, bind.FileButton(a, ""))
		case a.Method == "POST":
			b := bind.Submit(a)
			if a.Href != r.Source.Action {
				b.To = a.Href
			}
			foot.Items = append(foot.Items, b)
		default:
			foot.Items = append(foot.Items, bind.Button(a, ""))
		}
	}
	d.Items = append(d.Items, foot)
	return d
}

// copiesTable is a job's copies (s-backup-7): a row a job, its radio — the
// one chosen picked —, its name, the day, the size and how many collections.
func copiesTable(r view.FormRow) w.Table {
	var t w.Table
	for _, o := range r.Options {
		tick := w.TickedOff
		if o.Selected {
			tick = w.TickedOn
		}
		row := w.TableRow{Cells: []w.TableCell{
			{Pick: &tick, Send: w.Send{Name: r.Name, Value: o.Value, One: true}},
			{Parts: []w.CellPart{w.Text(o.Label)}},
		}}
		if o.Selected {
			row.State = w.RowPicked
			row.Cells[0].Width = w.ColTick
		}
		for i, c := range o.Columns {
			cell := w.TableCell{Parts: []w.CellPart{w.Text(c)}, Mono: true}
			switch i {
			case 0:
				cell.Rank = w.Rank3
			case 1:
				cell.Rank = w.Rank4
			}
			row.Cells = append(row.Cells, cell)
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}

// matchDialog is the archive opened (s-backup-8), one form: its day in the
// title, the job, what the manifest holds and where it lies under it,
// Cancel and «Restore 3 collections» in the head; the bar over the ticked
// rows — how many, Restore into, Skip, Select all —; a row a collection of
// the archive — ticked to restore, its mark and name, how many records,
// where it goes, what will happen —; that restoring creates and erases
// nothing, in alarm.
func matchDialog(r view.RestoreBlock, m *view.RestoreMatch) w.Dialog {
	// An archive from this computer opened in parts (s-backup-20): Restore
	// sends the parts of the collections chosen from the file step 1 kept.
	d := w.Dialog{Variant: w.DialogWide, Action: m.Form.Action, Method: m.Form.Method, Fields: bind.Hidden(m.Form), Ranges: m.Ranges, RangesOf: m.RangesOf}
	h := w.Head{Title: &w.HeadTitle{Text: string(r.Title)}, Subs: bind.Sub(r.Subtitle), Acts: &w.Acts{}}
	for _, a := range r.Actions {
		h.Acts.Items = append(h.Acts.Items, submitTo(a, m.Form.Action))
	}
	bar := w.Bar{Variant: w.BarSelection, Count: &w.BarCount{Text: strconv.Itoa(m.Selected) + selectedWord}, Items: []w.BarPart{w.BarSep{}}}
	for _, a := range m.Form.Actions {
		b := submitTo(a, m.Form.Action)
		switch a.ID {
		case restoreRunID:
			b.Icon = "restore"
			h.Acts.Items = append(h.Acts.Items, b)
		case restoreIntoID:
			// Beside it, the same choice for the ticked rows (s-backup-8,
			// Typecase's note 102).
			b.Icon = "restore"
			bar.Items = append(bar.Items, b, intoPick(m.Into))
		case restoreSkipID:
			bar.Items = append(bar.Items, b)
		case restoreAllID:
			// With a script it ticks the rows in place (s-backup-8).
			b.Small, b.ScriptOnly = true, true
			bar.Right = &w.BarRight{Items: []w.BarPart{b}}
		}
	}
	d.Items = append(d.Items, w.Steps{Items: []string{stepArchive, stepMatch, stepReport}, On: 2}, h, bar, matchTable(m))
	if len(r.Note) > 0 {
		d.Items = append(d.Items, w.Msg{Parts: bind.MsgParts(r.Note), Tone: w.MsgAlert})
	}
	return d
}

// submitTo is an action that sends the form it stands in to its own
// address; off with its reason.
func submitTo(a view.Action, action string) w.Button {
	b := bind.Submit(a)
	if a.Href != action {
		b.To = a.Href
	}
	if !a.Enabled {
		b.Submit, b.To, b.State, b.Reason = false, "", w.ButtonDisabled, string(a.Reason)
	}
	return b
}

// matchTable is the collections of the archive (s-backup-8): the head's
// tick — all, some or none ticked —; a row each, its tick sending pick.N,
// its mark and name, its count, «Restore into» its own choice under into.N
// — the collections of its kind, a rule, New collection, Skip this one —,
// and what will happen, in alarm where it is one; a row skipped is
// unticked, its target «—», what will happen muted.
func matchTable(m *view.RestoreMatch) w.Table {
	ticked := 0
	for _, r := range m.Rows {
		if r.Pick.Value != "" {
			ticked++
		}
	}
	head := w.TickedPart
	switch ticked {
	case 0:
		head = w.TickedOff
	case len(m.Rows):
		head = w.TickedOn
	}
	t := w.Table{Variant: w.TableFit, Cols: []w.TableCol{
		{Width: w.ColTick, Tick: &head}, {Label: colInArchive}, {Label: colRecords, Rank: w.Rank3, Secondary: true},
		{Label: colInto, Width: w.Col26, Rank: w.Rank4}, {Label: colWill},
	}}
	for _, r := range m.Rows {
		tick := w.TickedOff
		if r.Pick.Value != "" {
			tick = w.TickedOn
		}
		row := w.TableRow{Cells: []w.TableCell{
			{Pick: &tick, Send: w.Send{Name: r.Pick.Name, Value: "on"}},
			{Parts: []w.CellPart{w.TableFile{Mark: &w.Mark{Colour: string(r.Colour)}, Name: string(r.Name)}}},
			{Parts: []w.CellPart{w.Text(strconv.Itoa(r.Count))}, Mono: true},
		}}
		skipped := false
		for _, o := range r.Into.Options {
			if o.Selected && o.Value == restoreNoVal {
				skipped = true
			}
		}
		if skipped {
			row.Cells = append(row.Cells, w.TableCell{Parts: []w.CellPart{w.Text(noTarget)}, Mono: true, Unset: true, Tone: w.CellDusty},
				w.TableCell{Parts: []w.CellPart{w.Text(r.Will)}, Dim: true})
		} else {
			will := w.TableCell{Parts: []w.CellPart{w.Text(r.Will)}}
			if r.Danger {
				will.Tone = w.CellDanger
			}
			row.Cells = append(row.Cells, w.TableCell{Parts: []w.CellPart{intoPick(r.Into)}}, will)
		}
		if tick == w.TickedOn {
			row.State = w.RowPicked
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}

// intoPick is a row's «Restore into», or the bar's for the ticked rows
// (s-backup-8): the target chosen with its mark; the
// menu the collections of its kind, each with its mark, a rule, New
// collection, Skip this one muted — each a radio of the form under the
// row's name.
func intoPick(r view.FormRow) w.Drop {
	p := w.Pick{}
	menu := w.Menu{Width: w.MenuWidth290, Pop: w.MenuUnder}
	for _, o := range r.Options {
		if o.Selected {
			p.Value = string(o.Label)
			if o.Colour != "" {
				p.Mark = &w.Mark{Colour: string(o.Colour)}
			}
		}
		if o.Separator {
			menu.Items = append(menu.Items, w.MenuSep{})
		}
		it := w.MenuItem{Label: string(o.Label), On: o.Selected, Name: r.Name, Value: o.Value}
		switch o.Value {
		case restoreNewVal:
		case restoreNoVal:
			it.Tone = w.MenuToneDusty
		default:
			it.Mark = &w.Mark{Colour: string(o.Colour)}
		}
		menu.Items = append(menu.Items, it)
	}
	return w.Drop{Trigger: p, Menu: menu}
}
