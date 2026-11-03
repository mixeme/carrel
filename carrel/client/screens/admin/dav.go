package admin

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of the DAV validator (U-1.3): the columns of the second stage,
// and the ID of its form.
const (
	davID        = "dav"
	colOperation = "Operation"
	exerciseRun  = "admin.dav.exercise"
)

// davWidths are the widths the frame gives the fields, by their names
// (s-admin-5).
var davWidths = map[string]w.FieldWidth{"server": w.FieldXL, "login": w.FieldMD, "password": w.FieldMD, "certFingerprint": w.FieldFull}

// Dav is the DAV validator, /admin/dav (AdminDavPage, s-admin-5; empty,
// -27; discovery stopped, -28; the second stage run, -29): the line of the
// form in the head; the server, the login, the password asked each time,
// the fingerprint, and Test discovery; where discovery stopped, in alarm;
// its trace, a line a step; the agreement to write and Run the full check;
// what the second stage did, a row a kind.
func Dav(p *view.AdminDavPage) w.Shell {
	h := w.Head{Title: &w.HeadTitle{Text: string(p.Title)}}
	// The form's one legend is the page's line (s-admin-5).
	for _, set := range p.Form.Sets {
		if set.Legend != "" {
			h.Subs = []w.HeadSub{{Parts: []w.HeadSubPart{w.Text(set.Legend)}}}
		}
	}
	f := bind.Form(p.Form, davID)
	for _, r := range bind.Rows(p.Form) {
		f.Items = append(f.Items, bind.ColumnField(r, davWidths[r.Name]))
	}
	var full *view.Action
	for _, a := range p.Actions {
		if a.ID == exerciseRun {
			full = &a
			continue
		}
		f.Items = append(f.Items, w.Field{Parts: []w.FieldPart{submit(a, p.Form)}})
	}
	main := []w.MainPart{h}
	main = append(main, bind.ColumnMsgs(bind.Notices(p.Notices, w.MsgSpacePage))...)
	main = append(main, f)
	if t := p.Trace; t != nil {
		if t.Stopped != "" {
			main = append(main, w.Msg{Column: true, Parts: []w.MsgPart{w.Strong(t.Stopped)}, Tone: w.MsgAlert})
		}
		main = append(main, w.MainDiag{Diag: trace(t.Steps)})
	}
	if p.Agree != nil {
		c := bind.Check(*p.Agree)
		c.Value, c.Of = "on", davID
		second := w.Form{Items: []w.FormPart{w.Field{Parts: append([]w.FieldPart{c}, bind.Column(bind.Hint(*p.Agree))...), Width: w.FieldFull}}}
		if full != nil {
			b := bind.SubmitForm(*full, davID)
			// It writes to the server: in the danger's colour, not filled
			// (s-admin-5).
			b.Kind = w.ButtonDanger
			second.Items = append(second.Items, w.Field{Parts: []w.FieldPart{b}})
		}
		main = append(main, second)
	}
	if p.Full != nil {
		main = append(main, exerciseTable(p.Full.Rows))
	}
	return shell(p.Top, p.Rail, main)
}

// trace is discovery's steps as the block draws them: a line a step, its
// name in brackets, what the server answered after it in one column.
func trace(steps []view.TraceStep) w.Diag {
	var lines []bind.DiagLine
	for _, s := range steps {
		lines = append(lines, bind.DiagLine{Name: "[" + string(s.Name) + "]", Value: string(s.Detail)})
	}
	return bind.Diag(lines...)
}

// exerciseTable is the second stage (s-admin-29): a row a kind — created,
// read back and deleted —, what was asked and answered monospaced; the one
// refused in alarm, one skipped grey. What happened is what a row's card
// keeps on a phone, the operation its name (TableCol.Main, Typecase's note
// 161, Р-66).
func exerciseTable(rows []view.ExerciseRow) w.Table {
	t := w.Table{Cols: []w.TableCol{{Width: w.ColMenu}, {Label: colOperation, Width: w.Col26}, {Label: colHappened, Rank: w.Rank3, Main: true}}}
	for _, r := range rows {
		st := view.CheckUnknown
		switch r.Result {
		case "ok":
			st = view.CheckOK
		case "failed":
			st = view.CheckBad
		}
		mark, name, row := checkRow(st, r.Operation)
		row.Cells = []w.TableCell{{Parts: []w.CellPart{mark}}, name, {Parts: []w.CellPart{w.Text(r.Detail)}, Mono: true}}
		t.Rows = append(t.Rows, row)
	}
	return t
}

// DavForms are the forms of the page's actions that post with none of their
// own: the top bar's and the notices'.
func DavForms(p *view.AdminDavPage) []w.PostForm {
	return forms(p.Top, p.Notices)
}
