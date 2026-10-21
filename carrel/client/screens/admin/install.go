package admin

import (
	"strconv"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of the installation check (U-1.3): the tally of a run, the
// accessible name of a row's mark, the columns.
const (
	installID   = "install"
	checksWord  = " checks · "
	fineWord    = " fine"
	brokenWord  = " broken"
	unmeasured  = " not measured"
	tallySep    = " · "
	checkMark   = "Done"
	colCheck    = "Check"
	colHappened = "What happened"
	colToDo     = "What to do"
)

// Install is the installation check, /admin/install (AdminInstallPage,
// s-admin-4; before a run, -25; an address refused, -26): in the head the
// address asked and Run the check; the tally of the run and its time, a
// row a check — its mark, its name, what happened, what to do — the broken
// in alarm, the unmeasured grey; under it what the check does.
func Install(p *view.AdminInstallPage) w.Shell {
	h := w.Head{Title: &w.HeadTitle{Text: string(p.Title)}}
	if p.Subtitle != "" {
		h.Subs = []w.HeadSub{{Parts: []w.HeadSubPart{w.Text(p.Subtitle)}}}
	}
	var acts []w.ActsPart
	for _, r := range bind.Rows(p.Form) {
		in := bind.Input(r)
		in.Of, in.Variant = installID, w.Input11_5
		acts = append(acts, w.Field{Parts: []w.FieldPart{in}, Width: w.FieldW280})
	}
	for _, a := range p.Form.Actions {
		acts = append(acts, bind.Button(a, installID))
	}
	h.Acts = &w.Acts{Items: acts}
	main := []w.MainPart{h}
	main = append(main, bind.Notices(p.Notices, w.MsgSpacePage)...)
	if len(p.Rows) > 0 {
		main = append(main, tally(p.Rows, p.Took), checksTable(p.Rows))
	}
	main = append(main, bind.PostForm(p.Form, installID))
	if p.Note != "" {
		main = append(main, w.Msg{Parts: []w.MsgPart{w.Text(p.Note)}})
	}
	return shell(p.Top, p.Rail, main)
}

// tally is the line over the table: how many checks, how many fine,
// broken and not measured, and how long the run took (s-admin-4).
func tally(rows []view.InstallRow, took view.Text) w.Runbar {
	var ok, bad, unknown int
	for _, r := range rows {
		switch r.State {
		case view.CheckOK:
			ok++
		case view.CheckBad:
			bad++
		default:
			unknown++
		}
	}
	parts := []w.RunbarPart{w.Text(strconv.Itoa(len(rows)) + checksWord), w.Span{Face: w.FaceAccent, Text: strconv.Itoa(ok) + fineWord}}
	if bad > 0 {
		parts = append(parts, w.Text(tallySep), w.Span{Face: w.FaceDanger, Text: strconv.Itoa(bad) + brokenWord})
	}
	if unknown > 0 {
		parts = append(parts, w.Text(tallySep+strconv.Itoa(unknown)+unmeasured))
	}
	if took != "" {
		parts = append(parts, w.Mono{Text: string(took), Place: w.Pushed})
	}
	return w.Runbar{Meter: w.Meter{Percent: 100}, Parts: parts}
}

// checksTable is a row a check: a passed one ticked; a broken one marked
// at its edge in alarm, its name too, the row picked out; one that cannot
// be measured from outside grey; what to do small and monospaced.
func checksTable(rows []view.InstallRow) w.Table {
	t := w.Table{Cols: []w.TableCol{{Width: w.ColMenu}, {Label: colCheck, Width: w.Col26}, {Label: colHappened, Rank: w.Rank3}, {Label: colToDo, Width: w.Col26, Rank: w.Rank4}}}
	for _, r := range rows {
		mark, name, row := checkRow(r.State, r.Check)
		advice := w.TableCell{}
		if r.Advice != "" {
			advice = w.TableCell{Parts: []w.CellPart{w.Text(r.Advice)}, Mono: true, Small: true}
		}
		row.Cells = []w.TableCell{{Parts: []w.CellPart{mark}}, name, {Parts: []w.CellPart{w.Text(r.Value)}, Mono: true}, advice}
		t.Rows = append(t.Rows, row)
	}
	return t
}

// checkRow is the start of a row of a check (s-admin-4, -29): its mark — a
// passed one ticked, a broken one marked at its edge in alarm, one not
// measured or skipped grey —, its name in the same tone, and the row,
// picked out where it broke.
func checkRow(st view.CheckState, check view.Text) (w.Tick, w.TableCell, w.TableRow) {
	mark := w.Tick{Label: checkMark, Sign: true}
	name := w.TableCell{Parts: []w.CellPart{w.Text(check)}}
	row := w.TableRow{}
	switch st {
	case view.CheckOK:
		mark.Done = true
	case view.CheckBad:
		mark.Variant, name.Tone, row.State = w.TickEdgeDanger, w.CellDanger, w.RowPicked
	default:
		mark.Variant, name.Tone = w.TickEdgeDusty, w.CellMuted
	}
	return mark, name, row
}

// InstallForms are the forms of the page's actions that post with none of
// their own: the top bar's and the notices'.
func InstallForms(p *view.AdminInstallPage) []w.PostForm {
	return forms(p.Top, p.Notices)
}
