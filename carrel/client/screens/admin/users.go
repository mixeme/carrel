package admin

import (
	"strconv"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// listMain is the ID of the main the filter narrows as one types (Typecase's
// note 185).
const listMain = "list-main"

// The words of the list of people (U-1.3): the filter's, the badges of a
// row, the escrow cell of an account without a copy, the row's «⋯».
const (
	filterLogin = "Filter by login"
	disabled    = "disabled"
	deposited   = "deposited"
	noCopy      = "—"
	more        = "More"
)

// The list's columns (C-6.4 Columns, the ids the server names them by):
// their place in the row, how soon each leaves as the page narrows, whether
// it is of the second rank (s-admin-2), whether it is what a row's card
// keeps on a phone — Role (TableCol.Main, Typecase's note 155, Р-66).
var userCols = []struct {
	id     string
	rank   w.ColRank
	second bool
	main   bool
}{
	{"login", 0, false, false}, {"role", 0, false, true}, {"created", w.Rank3, true, false}, {"last-login", w.Rank4, false, false},
	{"dav", 0, false, false}, {"sessions", 0, false, false}, {"escrow", 0, false, false}, {"address", 0, false, false},
}

// Users is the list of people, /admin/ (AdminUsersPage, s-admin-2, -12):
//
//   - the head: «Users», how many accounts, administrators and copies in
//     escrow; Add user (primary) and Reset a password;
//   - the bar: «Filter by login», a GET form; the segments as links; the
//     columns, folding to their icon;
//   - the table: a person a row — the login (one's own bold, a disabled
//     account with its badge, its row faded), the role, the days and the
//     counts monospaced, «deposited» or a dash —, and «⋯» with what can be
//     done to the account (s-admin-12).
func Users(p *view.AdminUsersPage) w.Shell {
	h := w.Head{Title: &w.HeadTitle{Text: string(p.Title)}}
	if p.Subtitle != "" {
		h.Subs = []w.HeadSub{{Parts: []w.HeadSubPart{w.Text(p.Subtitle)}}}
	}
	var acts []w.ActsPart
	for _, a := range p.Add {
		acts = append(acts, bind.Button(a, a.ID))
	}
	if len(acts) > 0 {
		h.Acts = &w.Acts{Items: acts}
	}
	main := []w.MainPart{h}
	main = append(main, bind.ColumnMsgs(bind.Notices(p.Notices, w.MsgSpacePage))...)
	main = append(main, usersBar(p.Bar), usersTable(p))
	s := shell(p.Top, p.Rail, main)
	body := s.Body.(w.ShellBody)
	body.Main.ID = bind.MainID(p.Bar.FilterForm, listMain)
	s.Body = body
	return s
}

// usersBar is the bar over the list: the filter, the segments, the columns.
func usersBar(b view.ListBar) w.Bar {
	var items []w.BarPart
	if b.FilterForm != nil {
		f := bind.Narrows(bind.Filter(b, filterLogin), listMain)
		f.Variant = w.FilterH26W200
		items = append(items, f)
	}
	if len(b.Segments) > 0 {
		seg := w.Seg{}
		for _, a := range b.Segments {
			seg.Options = append(seg.Options, bind.SegOption(a))
		}
		items = append(items, seg)
	}
	if b.Columns != nil {
		cols := bind.Columns(*b.Columns, columns)
		cols.Menu.Pop = w.MenuUnder
		if t, ok := cols.Trigger.(w.Button); ok {
			t.Folds, t.Title = true, t.Label
			cols.Trigger = t
		}
		items = append(items, cols)
	}
	return w.Bar{Items: items}
}

// usersTable is the people in the columns shown, and «⋯» of each.
func usersTable(p *view.AdminUsersPage) w.Table {
	shown := map[string]string{}
	if p.Bar.Columns != nil {
		for _, it := range p.Bar.Columns.Items {
			if it.On {
				shown[it.ID] = string(it.Label)
			}
		}
	}
	t := w.Table{}
	var order []string
	for _, c := range userCols {
		if label, ok := shown[c.id]; ok {
			t.Cols = append(t.Cols, w.TableCol{Label: label, Rank: c.rank, Secondary: c.second, Main: c.main})
			order = append(order, c.id)
		}
	}
	t.Cols = append(t.Cols, w.TableCol{})
	for _, r := range p.Rows {
		var cells []w.TableCell
		for _, id := range order {
			cells = append(cells, userCell(r, id, p.Top.Signed.Login == r.Login))
		}
		// «⋯» is always shown: the row has no buttons it would twin
		// (s-admin-12: the actions are equal, none is the main one).
		cells = append(cells, w.TableCell{Acts: true, Parts: []w.CellPart{w.Drop{
			Trigger: w.Button{Icon: "dots", Small: true, Variant: w.ButtonPadX3, Title: more},
			Menu:    w.Menu{Items: bind.MenuItems(rowMenu(r), ""), Width: w.MenuWidth212, Pop: w.MenuUnderEnd},
		}}})
		row := w.TableRow{Cells: cells}
		if r.Disabled {
			row.State = w.RowGone
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}

// userCell is a person's cell of the column id.
func userCell(r view.AdminUserRow, id string, self bool) w.TableCell {
	text := func(s string, mono bool) w.TableCell {
		return w.TableCell{Parts: []w.CellPart{w.Text(s)}, Mono: mono}
	}
	switch id {
	case "login":
		c := w.TableCell{Parts: []w.CellPart{w.Text(r.Login)}, Name: self}
		if r.Disabled {
			c.Parts = []w.CellPart{w.Text(string(r.Login) + " "), w.Badge{Label: disabled}}
		}
		return c
	case "role":
		return text(string(r.Role), false)
	case "created":
		return text(string(r.Created), true)
	case "last-login":
		return text(string(r.LastLogin), true)
	case "dav":
		return text(strconv.Itoa(r.Accounts), true)
	case "sessions":
		return text(strconv.Itoa(r.Sessions), true)
	case "escrow":
		if r.Escrowed {
			return w.TableCell{Parts: []w.CellPart{w.Badge{Label: deposited}}}
		}
		return text(noCopy, false)
	}
	return text(string(r.Address), true)
}

// UsersForms are the forms of the list's actions that post — each item of
// a row's «⋯» by its action and address —, and the top bar's.
func UsersForms(p *view.AdminUsersPage) []w.PostForm {
	out := forms(p.Top, p.Notices)
	for _, r := range p.Rows {
		out = append(out, bind.MenuForms(rowMenu(r))...)
	}
	return out
}

// rowMenu is a row's «⋯», each item's ID made its own: the same actions
// stand in every row, the login tells them apart.
func rowMenu(r view.AdminUserRow) []view.MenuItem {
	out := make([]view.MenuItem, len(r.Menu))
	for i, it := range r.Menu {
		it.ID += "/" + string(r.Login)
		out[i] = it
	}
	return out
}
