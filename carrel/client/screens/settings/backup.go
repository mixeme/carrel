package settings

import (
	"strconv"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of Backup (U-1.3): the columns of the jobs and of the history,
// the history's rubric, a job never run and one that keeps nothing, the
// row's «⋯», the ID of a job's form, a group's count of what is chosen, the
// days of the sign-in's copy.
const (
	colJob       = "Job"
	colCopies    = "What it copies"
	colWhereTo   = "Where to"
	colWhen      = "When"
	colKeep      = "Keep"
	colLast      = "Last backup"
	colSize      = "Size"
	colCols      = "Collections"
	colVerified  = "Verified"
	colTrigger   = "Trigger"
	historyTitle = "History, all jobs"
	neverRun     = "never"
	keepsNone    = "—"
	moreWord     = "More"
	jobFormID    = "backup-job"
	ofWord       = " of "
	daysWord     = "days"
	readOnly     = "ro"
	downloadID   = "backup.get"
)

// Backup is /app/settings/backup (BackupPage): the jobs as a table and the
// history of all of them (s-backup-1), the page with no job yet
// (s-backup-6), a job on its own page — its form, Delete job under it
// (s-backup-2), the restore on its own page (s-backup-7, -8).
func Backup(p *view.BackupPage) w.Shell {
	if p.Restore.Open {
		sh := shell(p.Top, p.Rail, restoreMain(p))
		if r := p.Restore; r.Ranges != "" || (r.Match != nil && r.Match.Ranges != "") {
			// An archive from this computer (s-backup-15, -16, -20;
			// Typecase's note 109): the page's column is the zone its steps
			// answer into, so the page — and the file picked on it — stays.
			body := sh.Body.(w.ShellBody)
			body.Main.Zone, body.Main.Target = restoreZone, restoreZone
			sh.Body = body
		}
		return sh
	}
	if p.Job != nil || (len(p.Jobs) == 0 && len(p.Empty) == 0) {
		return shell(p.Top, p.Rail, jobPage(p))
	}
	h := w.Head{Title: &w.HeadTitle{Text: string(p.Title)}, Subs: bind.Sub(p.Subtitle)}
	var acts []w.ActsPart
	for _, a := range p.Actions {
		acts = append(acts, bind.Button(a, bind.TickID(a)))
	}
	h.Acts = &w.Acts{Items: acts}
	main := []w.MainPart{h}
	main = append(main, bind.Notices(p.Notices, w.MsgSpacePage)...)
	if len(p.Jobs) == 0 {
		parts := as[w.EmptyPart](bind.Inlines(p.Empty))
		if len(p.EmptyNote) > 0 {
			parts = append(parts, w.Text(" "), w.Aside{Face: w.FaceLarge, Parts: bind.Inlines(p.EmptyNote)})
		}
		return shell(p.Top, p.Rail, append(main, w.Empty{Parts: parts}))
	}
	main = append(main, jobsTable(p.Jobs))
	if p.JobsHint != "" {
		main = append(main, w.MainHint{Text: string(p.JobsHint)})
	}
	if len(p.History) > 0 {
		main = append(main, w.ListGroup{Title: w.Rubric{Text: historyTitle, Form: w.RubricInline}, Count: &w.Num{Value: len(p.History)}},
			historyTable(p.History))
	}
	return shell(p.Top, p.Rail, main)
}

// jobsTable is the jobs (s-backup-1): a row a job — its tick turning it on
// and off, its name, the marks of what it copies and how much, where to,
// when, how many copies it keeps, its last copy and how it ended; Run and
// Edit, and in «⋯» their twins and External trigger. A job that is off is
// faded.
func jobsTable(jobs []view.BackupJob) w.Table {
	t := w.Table{Variant: w.TableWide, Cols: []w.TableCol{
		{Width: w.ColTick}, {Label: colJob}, {Label: colCopies, Rank: w.Rank3, Secondary: true}, {Label: colWhereTo, Rank: w.Rank4, Secondary: true},
		{Label: colWhen}, {Label: colKeep, Secondary: true}, {Label: colLast, Main: true}, {Width: w.ColTime},
	}}
	for _, j := range jobs {
		tick := w.TickedOn
		if !j.On {
			tick = w.TickedOff
		}
		pick := w.TableCell{Pick: &tick}
		if a := j.Toggle; a != nil && a.Enabled && a.Method == "POST" {
			pick.Send = w.Send{Form: bind.TickID(*a)}
		}
		copies := w.Line{Gap: w.LineGap3}
		for _, c := range j.Marks {
			copies.Items = append(copies.Items, w.Mark{Colour: string(c)})
		}
		copies.Items = append(copies.Items, w.Mono{Text: string(j.Copies), Variant: w.Mono11Indent})
		keep := w.TableCell{Parts: []w.CellPart{w.Text(keepsNone)}, Mono: true}
		if j.Keep > 0 {
			keep.Parts = []w.CellPart{w.Text(strconv.Itoa(j.Keep))}
		}
		last := w.TableCell{Parts: []w.CellPart{w.Text(neverRun)}, Mono: true, Tone: w.CellDusty}
		if j.LastRun != "" {
			last = w.TableCell{Parts: twoLines(string(j.LastRun), bind.Inlines(j.LastNote)), Mono: true}
		}
		row := w.TableRow{Cells: []w.TableCell{
			pick,
			{Parts: []w.CellPart{w.TableFile{Icon: "archive", Name: string(j.Name)}}},
			{Parts: []w.CellPart{copies}},
			{Parts: twoLines(string(j.Where), dimText(j.WhereNote)), Mono: true},
			whenCell(j),
			keep,
			last,
			rowActs(j),
		}}
		if !j.On {
			row.State = w.RowFaded
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}

// rowActs are a job's Run and Edit, and «⋯» with their twins and External
// trigger after them (s-backup-1).
func rowActs(j view.BackupJob) w.TableCell {
	c := w.TableCell{Acts: true}
	menu := w.Menu{Width: w.MenuWidth212, Pop: w.MenuUnderEnd}
	for _, a := range j.Actions {
		b := bind.Button(a, bind.TickID(a))
		b.Small, b.Secondary = true, true
		c.Parts = append(c.Parts, b)
		menu.Items = append(menu.Items, menuTwin(a))
	}
	if e := j.External; e.ID != "" {
		menu.Items = append(menu.Items, menuTwin(e))
	}
	c.Drop = &w.Drop{Trigger: w.Button{Icon: "dots", Small: true, Variant: w.ButtonPadX3, Title: moreWord}, Row: true, Menu: menu}
	return c
}

// menuTwin is an action as an item of a row's «⋯»: a link where it goes
// somewhere, the form it posts by otherwise; off with its reason.
func menuTwin(a view.Action) w.MenuItem {
	it := w.MenuItem{Label: string(a.Label)}
	switch {
	case !a.Enabled:
		it.Reason = string(a.Reason)
	case a.Method == "POST":
		it.Form = bind.TickID(a)
	default:
		it.Href = a.Href
	}
	return it
}

// historyTable is «History, all jobs» (s-backup-1): a run a row — when,
// which job, how big, how many collections went in whole (in alarm when not
// all), whether it was read back and what was not copied, what started it
// and, for an external run, with which password and from where; Restore and
// Download of a copy there is.
func historyTable(rows []view.BackupHistoryRow) w.Table {
	t := w.Table{Cols: []w.TableCol{
		{Label: colWhen}, {Label: colJob}, {Label: colSize, Rank: w.Rank3, Secondary: true}, {Label: colCols, Rank: w.Rank4},
		{Label: colVerified}, {Label: colTrigger, Secondary: true}, {Width: w.ColTime},
	}}
	for _, r := range rows {
		cols := w.TableCell{Parts: []w.CellPart{w.Text(r.Collections)}, Mono: true}
		if !r.Whole {
			cols.Tone = w.CellDanger
		}
		acts := w.TableCell{Acts: true}
		for _, a := range r.Actions {
			b := bind.Button(a, "")
			b.Small = true
			if a.ID == downloadID {
				b.Label, b.Title, b.Variant = "", string(a.Label), w.ButtonPadX3
			}
			acts.Parts = append(acts.Parts, b)
		}
		t.Rows = append(t.Rows, w.TableRow{Cells: []w.TableCell{
			{Parts: []w.CellPart{w.Text(r.At)}, Mono: true},
			{Parts: []w.CellPart{w.Text(r.Job)}},
			{Parts: []w.CellPart{w.Text(r.Size)}, Mono: true},
			cols,
			{Parts: as[w.CellPart](hinted(r.Verified))},
			{Parts: twoLines(string(r.Trigger), dimText(r.Via)), Mono: true},
			acts,
		}})
	}
	return t
}

// twoLines is a cell's value and, when there is one, the muted line under
// it.
func twoLines(first string, second []w.Inline) []w.CellPart {
	out := []w.CellPart{w.Text(first)}
	if len(second) > 0 {
		out = append(out, w.Break{}, w.Dim{Parts: second})
	}
	return out
}

// dimText is words as the pieces of a muted line; none for none.
func dimText(t view.Text) []w.Inline {
	if t == "" {
		return nil
	}
	return []w.Inline{w.Text(t)}
}

// hinted is a sentence in its pieces (bind.Inlines) whose piece in alarm
// says in full what it is short for (Run.Hint: «1 not copied» names the
// collection and why).
func hinted(r view.Runs) []w.Inline {
	out := bind.Inlines(r)
	for i, p := range r {
		if p.Mark == view.RunDanger && p.Hint != "" {
			out[i] = w.Span{Face: w.FaceDanger, Text: string(p.Text), Title: string(p.Hint)}
		}
	}
	return out
}

// as is parts of one widget as the parts of another that holds them: the
// pieces of a sentence in an empty page or a cell, a set's fields loose in
// its form.
func as[T, S any](in []S) []T {
	out := make([]T, 0, len(in))
	for _, p := range in {
		if t, ok := any(p).(T); ok {
			out = append(out, t)
		}
	}
	return out
}

// jobPage is a job on its own page (s-backup-2) — a new one too: its name
// and its last copy over the form, Cancel and Save in the head; the form
// in its rubrics — what to copy, where to, how the archive is closed,
// when —; Delete job under it with what stays.
func jobPage(p *view.BackupPage) []w.MainPart {
	h := w.Head{Title: &w.HeadTitle{Text: string(p.Title)}, Subs: bind.Sub(p.Subtitle)}
	var acts []w.ActsPart
	for _, a := range p.Add.Actions {
		if a.Method == "GET" {
			acts = append(acts, bind.Button(a, ""))
		}
	}
	for _, a := range p.Add.Actions {
		if a.Method != "GET" {
			acts = append(acts, bind.SubmitForm(a, jobFormID))
		}
	}
	h.Acts = &w.Acts{Items: acts}
	main := []w.MainPart{h}
	main = append(main, bind.Notices(p.Notices, w.MsgSpacePage)...)
	form := bind.Form(p.Add, jobFormID)
	for _, set := range p.Add.Sets {
		form.Items = append(form.Items, jobSet(set)...)
	}
	main = append(main, form)
	if d := p.Delete; d != nil {
		b := bind.Button(*d, bind.TickID(*d))
		main = append(main, w.Danger{Button: b, Note: string(p.DeleteNote)})
	}
	return main
}

// jobSet is a set of a job's form as the frame lays it (s-backup-2): the
// first — the name and «Job is on» — loose; the others in their rubrics,
// what the set means after its fields.
func jobSet(set view.FieldSet) []w.FormPart {
	var items []w.FieldSetPart
	var after []w.FieldSetPart
	for _, r := range set.Rows {
		switch {
		case r.Kind == view.InputHidden:
		case r.Kind == view.InputCheck && len(r.Options) > 1:
			items = append(items, sourceGrid(r))
		case r.Kind == view.InputRadio:
			// Where to: the segments on a line of their own; what they
			// mean closes the set (s-backup-2).
			f := segField(view.FormRow{Name: r.Name, Options: r.Options})
			f.OwnLine, f.Place = true, w.Grown
			items = append(items, f)
			if len(r.HintRuns) > 0 {
				after = append(after, w.Msg{Parts: bind.MsgParts(r.HintRuns), Space: w.MsgFlushTop4, Place: w.Grown})
			}
		case r.Kind == view.InputSelect:
			items = append(items, w.Field{Label: string(r.Label), Parts: []w.FieldPart{destPick(r)}, Width: w.FieldLG})
		case r.Kind == view.InputCheck && r.Name == "on":
			items = append(items, w.Field{Parts: []w.FieldPart{bind.Check(r)}})
		case r.Kind == view.InputCheck && r.Hint != "":
			items = append(items, w.Field{Parts: append([]w.FieldPart{bind.Check(r)}, bind.Hint(r)...), OwnLine: true, Place: w.Grown})
		case r.Kind == view.InputCheck:
			// «On sign-in, if the last copy is older than» and its days on
			// one line (s-backup-2).
			items = append(items, signinField(r, set.Rows))
		case r.Kind == view.InputNumber && r.Name == "signin":
		case r.Kind == view.InputPassword:
			in := bind.Input(r)
			items = append(items, w.Field{Label: string(r.Label), Parts: []w.FieldPart{in}, Width: w.FieldLG})
			if r.Hint != "" {
				items = append(items, w.Msg{Parts: []w.MsgPart{w.Text(r.Hint)}, Tone: w.MsgAlert, Space: w.MsgFlushTop2, Place: w.Grown})
			}
		case r.Extra != nil:
			in := bind.Input(r)
			in.Variant = w.InputFill
			line := w.Line{Items: []w.LinePart{in, bind.Button(*r.Extra, "")}, Gap: w.LineGap8}
			items = append(items, w.Field{Label: string(r.Label), Parts: []w.FieldPart{line}, Width: w.FieldLG})
		case r.Kind == view.InputNumber:
			items = append(items, w.Field{Label: string(r.Label), Parts: []w.FieldPart{bind.Input(r)}, Width: w.FieldXS})
			if len(r.HintRuns) > 0 {
				items = append(items, w.Field{Parts: []w.FieldPart{w.FieldHint{Parts: bind.Inlines(r.HintRuns)}}, Place: w.Grown})
			}
		default:
			items = append(items, w.Field{Label: string(r.Label), Parts: []w.FieldPart{bind.Input(r)}, Width: w.FieldLG})
		}
	}
	if len(set.HintRuns) > 0 {
		after = append(after, w.Msg{Parts: bind.MsgParts(set.HintRuns), Space: w.MsgFlushTop4, Place: w.Grown})
	}
	items = append(items, after...)
	if set.Legend == "" {
		return as[w.FormPart](items)
	}
	r := w.Rubric{Text: string(set.Legend)}
	if set.LegendAside != "" {
		r.Items = []w.RubricPart{w.Plain{Text: string(set.LegendAside)}}
	}
	return []w.FormPart{w.FieldSet{Rubric: r, Items: items}}
}

// sourceGrid is What to copy (s-backup-2): a column a connection, its
// tick over its collections — on when all are chosen, part when some —
// and how many are; a row a collection, ticked where it is chosen, its
// mark, its name and what it is.
func sourceGrid(r view.FormRow) w.FieldGrid {
	var g w.FieldGrid
	var col *w.FieldGridColumn
	chosen, all := 0, 0
	flush := func() {
		if col == nil {
			return
		}
		head := &col.Items[0]
		head.Meta = strconv.Itoa(chosen) + ofWord + strconv.Itoa(all)
		switch {
		case chosen == 0:
			head.Ticked = w.TickedOff
		case chosen < all:
			head.Ticked = w.TickedPart
		}
		g.Items = append(g.Items, *col)
	}
	for _, o := range r.Options {
		if o.Group != "" {
			flush()
			col = &w.FieldGridColumn{Items: []w.TickRow{{Form: w.TickRowGroup, Mark: w.Mark{Kind: w.MarkMulti}, Label: string(o.Group)}}}
			chosen, all = 0, 0
		}
		if col == nil {
			continue
		}
		row := w.TickRow{Send: w.Send{Name: r.Name, Value: o.Value}, Form: w.TickRowNested, Mark: w.Mark{Colour: string(o.Colour)}, Label: string(o.Label), Meta: string(o.Meta)}
		if !o.Selected {
			row.Ticked = w.TickedOff
		} else {
			chosen++
		}
		all++
		col.Items = append(col.Items, row)
	}
	flush()
	return g
}

// destPick is the collection a job writes to (s-backup-2): the chosen one
// with its mark; the menu a collection an item, its mark, under the row's
// name, one read-only off with its reason.
func destPick(r view.FormRow) w.Drop {
	p := w.Pick{}
	menu := w.Menu{Width: w.MenuWidth290, Pop: w.MenuUnder}
	for _, o := range r.Options {
		mark := &w.Mark{Colour: string(o.Colour)}
		if o.Selected {
			p.Value, p.Mark = string(o.Label), mark
		}
		it := w.MenuItem{Mark: mark, Label: string(o.Label), On: o.Selected, Name: r.Name, Value: o.Value}
		if o.Disabled {
			// A collection this account cannot write (s-backup-2).
			it.Reason, it.Trail = string(o.Reason), w.Badge{Label: readOnly, Place: w.Pushed}
		}
		menu.Items = append(menu.Items, it)
	}
	return w.Drop{Trigger: p, Menu: menu}
}

// signinField is the copy at sign-in (s-backup-2): its tick and its days
// on one line.
func signinField(r view.FormRow, rows []view.FormRow) w.Field {
	line := w.Line{Items: []w.LinePart{bind.Check(r)}, Gap: w.LineGap7, Block: true}
	for _, d := range rows {
		if d.Kind == view.InputNumber && d.Name == "signin" {
			in := bind.Input(d)
			in.Variant = w.InputChip
			line.Items = append(line.Items, in, w.Faced{Face: w.FaceLarge, Items: []w.LinePart{w.Text(daysWord)}})
		}
	}
	return w.Field{Parts: []w.FieldPart{line}, Gap: 8, Place: w.Grown}
}

// BackupForms are the forms of the page's actions that post with none of
// their own: a job's tick and Run, Delete job, and the top bar's.
func BackupForms(p *view.BackupPage) []w.PostForm {
	out := forms(p.Top, p.Notices)
	post := func(a view.Action) {
		if a.Method == "POST" && a.Enabled && !a.Submits {
			out = append(out, bind.ActionForm(a, bind.TickID(a)))
		}
	}
	for _, j := range p.Jobs {
		if j.Toggle != nil {
			post(*j.Toggle)
		}
		for _, a := range j.Actions {
			post(a)
		}
	}
	if d := p.Delete; d != nil {
		post(*d)
	}
	return out
}

// triggerBadge marks a job whose external trigger password is issued
// (s-backup-1).
const triggerBadge = "trigger"

// whenCell is a job's When: only what is set — the sign-in rule in words,
// an issued trigger password its badge, else «—» (s-backup-1, Typecase's
// note 95).
func whenCell(j view.BackupJob) w.TableCell {
	var c w.TableCell
	if j.When != "" {
		c = w.TableCell{Parts: twoLines(string(j.When), dimText(j.WhenNote)), Mono: true}
	}
	if j.Triggered {
		if len(c.Parts) > 0 {
			c.Parts = append(c.Parts, w.Text(" "))
		}
		c.Parts = append(c.Parts, w.Badge{Label: triggerBadge})
	}
	if len(c.Parts) == 0 {
		c = w.TableCell{Parts: []w.CellPart{w.Text(keepsNone)}, Mono: true}
	}
	return c
}
