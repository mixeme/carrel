package admin

import (
	"strconv"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of the journal (U-1.3): how many lines it holds, and the muted
// word before the day it is read from.
const (
	records = " records"
	since   = "since"
)

var columns = bind.ColumnWords{Name: "col", Reset: "default", Drag: "drag"}

// The journal's columns (C-6.4 Columns, the ids the server names them by):
// their place in the row, and how soon each leaves as the page narrows —
// Actor and Target first (s-admin-7).
var auditCols = []struct {
	id   string
	rank w.ColRank
}{
	{"when", 0}, {"action", 0}, {"actor", w.Rank3}, {"target", w.Rank4}, {"detail", 0},
}

// Audit is the journal of the instance, /admin/audit (AdminAuditPage,
// s-admin-7, -10):
//
//   - the head: «Audit log», how many lines the journal holds, Export CSV;
//   - the bar: the filter, a GET form of its own (BarForm) — the action
//     chosen from a menu of radios («All actions» first, a line, then each
//     action), the muted «since» and the day, the segment shown as a hidden
//     field, Show sending it without a script —; the segments as links; the
//     columns;
//   - the table: a line a row, the moment and the action monospaced, an
//     action that did not happen in alarm (Failure), who and on whom, the
//     detail monospaced; the columns as the person chose them.
func Audit(p *view.AdminAuditPage) w.Shell {
	h := w.Head{Title: &w.HeadTitle{Text: string(p.Title)}}
	if p.Bar.Total != nil {
		h.Subs = []w.HeadSub{{Parts: []w.HeadSubPart{w.Text(strconv.Itoa(*p.Bar.Total) + records)}}}
	}
	var acts []w.ActsPart
	for _, a := range p.Bar.Actions {
		acts = append(acts, bind.Button(a, a.ID))
	}
	if len(acts) > 0 {
		h.Acts = &w.Acts{Items: acts}
	}
	main := []w.MainPart{h}
	main = append(main, bind.Notices(p.Notices, w.MsgSpacePage)...)
	main = append(main, auditBar(p), auditTable(p))
	return shell(p.Top, p.Rail, main)
}

// auditBar is the bar over the journal: its filter, its segments, its
// columns.
func auditBar(p *view.AdminAuditPage) w.Bar {
	var items []w.BarPart
	if f := filter(p.Filter); f != nil {
		items = append(items, *f)
	}
	if len(p.Bar.Segments) > 0 {
		seg := w.Seg{}
		for _, a := range p.Bar.Segments {
			seg.Options = append(seg.Options, bind.SegOption(a))
		}
		items = append(items, seg)
	}
	if p.Bar.Columns != nil {
		cols := bind.Columns(*p.Bar.Columns, columns)
		cols.Menu.Pop = w.MenuUnder
		// The button folds to its icon as the bar narrows, its words in its
		// title (s-admin-7).
		if t, ok := cols.Trigger.(w.Button); ok {
			t.Folds, t.Title = true, t.Label
			cols.Trigger = t
		}
		items = append(items, cols)
	}
	return w.Bar{Items: items}
}

// filter is the journal's filter as the bar's own form (BarForm, Typecase's
// note 78): a choice of a row with options as a menu of radios under its
// name, the chosen one's words on the trigger, muted while none is chosen; a
// day as a monospaced date field after the muted «since»; the hidden rows;
// Show from the form's action. nil — the page has none.
func filter(f view.Form) *w.BarForm {
	if f.Action == "" {
		return nil
	}
	bf := w.BarForm{Action: f.Action, Hidden: bind.Hidden(f)}
	for _, r := range bind.Rows(f) {
		switch r.Kind {
		case view.InputSelect:
			bf.Items = append(bf.Items, choice(r))
		case view.InputDate:
			in := bind.Input(r)
			in.Variant = w.InputH26W120
			bf.Items = append(bf.Items, w.Dim{Parts: []w.Inline{w.Text(since)}}, in)
		}
	}
	for _, a := range f.Actions {
		bf.Show = bind.Submit(a)
	}
	return &bf
}

// choice is a select of the filter as a menu of radios (s-admin-10): each
// option a radio of the row's name, the chosen one on, a line before one
// that has it; the trigger the chosen one's words, muted where the chosen
// one is the empty value («All actions»).
func choice(r view.FormRow) w.Drop {
	pick := w.Pick{Label: string(r.Label), Variant: w.PickH26W190, Unset: r.Value == ""}
	var items []w.MenuPart
	for _, o := range r.Options {
		if o.Separator && len(items) > 0 {
			items = append(items, w.MenuSep{})
		}
		items = append(items, w.MenuItem{Label: string(o.Label), On: o.Selected, Name: r.Name, Value: o.Value})
		if o.Selected {
			pick.Value = string(o.Label)
		}
	}
	return w.Drop{Trigger: pick, Menu: w.Menu{Items: items, Width: w.MenuWidth290, Pop: w.MenuUnder}}
}

// auditTable is the journal's lines in the columns shown.
func auditTable(p *view.AdminAuditPage) w.Table {
	shown := map[string]string{}
	var order []string
	if p.Bar.Columns != nil {
		for _, it := range p.Bar.Columns.Items {
			if it.On {
				shown[it.ID] = string(it.Label)
			}
		}
	}
	t := w.Table{}
	for _, c := range auditCols {
		if label, ok := shown[c.id]; ok {
			t.Cols = append(t.Cols, w.TableCol{Label: label, Rank: c.rank})
			order = append(order, c.id)
		}
	}
	for _, r := range p.Rows {
		var cells []w.TableCell
		for _, id := range order {
			cells = append(cells, auditCell(r, id))
		}
		t.Rows = append(t.Rows, w.TableRow{Cells: cells})
	}
	return t
}

// auditCell is a line's cell of the column id.
func auditCell(r view.AuditRow, id string) w.TableCell {
	text := func(t view.Text, mono bool) w.TableCell {
		return w.TableCell{Parts: []w.CellPart{w.Text(t)}, Mono: mono}
	}
	switch id {
	case "when":
		return text(view.Text(r.When), true)
	case "action":
		c := text(r.What, true)
		if r.Failure {
			c.Tone = w.CellDanger
		}
		return c
	case "actor":
		return text(r.Who, false)
	case "target":
		return text(r.Whom, false)
	}
	return text(r.Detail, true)
}

// AuditForms are the forms of the journal's actions that post with none of
// their own.
func AuditForms(p *view.AdminAuditPage) []w.PostForm {
	return forms(p.Top, p.Notices)
}
