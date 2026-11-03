package settings

import (
	"strconv"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of a new device (U-1.3): the unlock mark, the rubrics of the
// columns, what a code holds, the columns of the address table, the words
// of an account's row.
const (
	unlockedMark   = "credentials unlocked · "
	profileGoes    = "What goes into the profile"
	davx5Sees      = "Collections DAVx5 will see"
	rubricProfile  = "Profile"
	inTheCode      = "Address and login are in the code."
	colType        = "Type"
	colURL         = "URL"
	loginWord      = "login"
	passwordWord   = "password"
	insideMark     = "· "
	typesSep       = ", "
	oneCollection  = "1 collection"
	collectionsOf  = " collections"
	qrPaper        = "#FFFDF7"
	generateID     = "device.profile"
	davx5ID        = "device.davx5"
	passwordCopyID = "device.password.copy"
	passwordLabel  = "Password"
	localBadge     = "local"
	localHint      = "Local instance: may be unreachable from outside"
	copiedWord     = "Copied"
)

// Devices is /app/settings/devices (DevicesPage, s-plan-1, -2, -4, -5): the
// line under the title and, while the window is open, its mark and Lock
// now; the tabs; Apple — the accounts that go into the profile, ticked, the
// warning, Generate profile and the link issued; Android — the accounts and
// a code each with its password; Thunderbird and anything else — a row an
// account with its login and password, a row a collection with its address.
func Devices(p *view.DevicesPage) w.Shell {
	h := head(p.Title, p.Subtitle)
	unlockMark(&h, p.Top)
	seg := w.Seg{}
	var generate *view.Action
	for i, a := range p.Actions {
		if a.ID == generateID {
			generate = &p.Actions[i]
			continue
		}
		seg.Options = append(seg.Options, bind.SegOption(a))
	}
	h.Acts = &w.Acts{Items: []w.ActsPart{seg}}
	main := []w.MainPart{h}
	main = append(main, bind.ColumnMsgs(bind.Notices(p.Notices, w.MsgSpacePage))...)
	switch p.Tab {
	case view.TabApple:
		main = append(main, appleForm(p, generate))
	case view.TabAndroid:
		main = append(main, androidForm(p))
	default:
		main = append(main, addressTable(p.Accounts))
		switch {
		case len(p.UnlockNote) > 0:
			main = append(main, w.Msg{Column: true, Icon: "shield", Parts: msgParts(p.UnlockNote)})
		case p.Warning.Body != "" || len(p.Warning.Runs) > 0:
			m := bind.Msg(p.Warning, w.MsgSpacePage)
			m.Icon, m.Column = "shield", true
			main = append(main, m)
		}
	}
	return shell(p.Top, p.Rail, main)
}

// accountList is the accounts as slim rows: each ticked where it goes in —
// pressed, it is left out or put back —, its mark, its name, its included
// collections after it, its protocols at the end.
func accountList(accounts []view.DeviceAccount) w.List {
	l := w.List{Inset: true}
	for _, a := range accounts {
		r := w.SlimRow{Mark: w.Mark{Kind: w.MarkMulti}, Title: string(a.Title), Reason: string(a.Reason)}
		if !a.Included {
			r.Ticked = w.TickedOff
		}
		if t := a.Toggle; t != nil && t.Enabled {
			r.Send = w.Send{Form: bind.TickID(*t)}
		}
		// How many collections the account brings, and its protocols
		// (s-plan-1; Typecase's note 94); one refused says so instead.
		var types []string
		seen := map[string]bool{}
		for _, row := range a.Rows {
			if !seen[string(row.Type)] {
				seen[string(row.Type)] = true
				types = append(types, string(row.Type))
			}
		}
		switch n := len(a.Rows); {
		case n == 1:
			r.Inside = insideMark + oneCollection
		case n > 1:
			r.Inside = insideMark + strconv.Itoa(n) + collectionsOf
		}
		r.Aside = joined(types, typesSep)
		if a.Refused != "" {
			r.Aside = string(a.Refused)
		}
		l.Rows = append(l.Rows, r)
	}
	return l
}

// shieldMsg is the warning of a tab: what the profile or the codes hold.
func shieldMsg(n view.Notice) w.Msg {
	m := bind.Msg(n, w.MsgFlush)
	m.Icon, m.Column = "shield", true
	return m
}

// appleForm is the Apple tab (s-plan-1): the accounts and the warning in
// the wide column; Generate profile and the link issued in the other.
func appleForm(p *view.DevicesPage, generate *view.Action) w.Form {
	wide := w.FormColumn{Wide: true, Items: []w.FormColumnPart{
		w.Card{Items: []w.CardPart{w.Rubric{Text: profileGoes}, accountList(p.Accounts)}},
		shieldMsg(p.Warning),
	}}
	side := w.FormColumn{}
	if generate != nil {
		card := w.Card{Items: []w.CardPart{w.Rubric{Text: rubricProfile}}}
		if p.Note != "" {
			card.Items = append(card.Items, w.CardHint{Parts: []w.CardTextPart{w.Text(p.Note)}})
		}
		b := bind.Button(*generate, bind.TickID(*generate))
		b.Kind = w.ButtonDanger
		card.Items = append(card.Items, w.CardRow{Kind: w.CardActs, Items: []w.CardRowPart{b}})
		side.Items = append(side.Items, card)
	}
	if l := p.Link; l != nil {
		r := w.Rubric{Text: string(l.Title)}
		if l.Local {
			r.Items = append(r.Items, w.Badge{Label: localBadge, Tone: w.BadgeLocal, Title: localHint})
		}
		card := w.Card{Items: []w.CardPart{r}}
		if l.Hint != "" {
			card.Items = append(card.Items, w.CardHint{Parts: []w.CardTextPart{w.Text(l.Hint)}})
		}
		row := w.CardRow{Kind: w.CardTop9, Gap: 6}
		for _, a := range l.Actions {
			b := bind.Button(a, bind.TickID(a))
			if a.Danger {
				b.Kind = w.ButtonDanger
			}
			row.Items = append(row.Items, b)
		}
		card.Items = append(card.Items, row)
		side.Items = append(side.Items, card)
	}
	return w.Form{Items: []w.FormPart{wide, side}, Variant: w.FormFlushBottom}
}

// androidForm is the Android tab (s-plan-2): the accounts DAVx5 will see
// and what a code holds; a card an included account — its code, Open in
// DAVx5, and its password to copy.
func androidForm(p *view.DevicesPage) w.Form {
	wide := w.FormColumn{Wide: true, Items: []w.FormColumnPart{
		w.Card{Items: []w.CardPart{w.Rubric{Text: davx5Sees}, accountList(p.Accounts)}},
		bind.Msg(p.Warning, w.MsgFlush),
	}}
	if m, ok := wide.Items[1].(w.Msg); ok {
		m.Icon, m.Column = "shield", true
		wide.Items[1] = m
	}
	side := w.FormColumn{}
	for _, a := range p.Accounts {
		if !a.Included || a.QR == "" {
			continue
		}
		qr := w.CardQR{Colour: qrPaper, Src: a.QR, Side: []w.CardQRPart{w.Dim{Parts: []w.Inline{w.Text(inTheCode)}, Size: w.Dim11_5, Variant: w.DimWrap}}}
		var copyPw *view.Action
		for i, act := range a.Actions {
			switch act.ID {
			case davx5ID:
				b := bind.Button(act, "")
				b.Small = true
				qr.Side = append(qr.Side, b)
			case passwordCopyID:
				copyPw = &a.Actions[i]
			}
		}
		card := w.Card{Items: []w.CardPart{w.Rubric{Text: string(a.Title)}, qr}}
		if a.Password.Shown {
			in := w.Input{Value: string(a.Password.Value), Label: passwordLabel, Mono: true, ReadOnly: true, Variant: w.InputFill}
			line := w.Line{Items: []w.LinePart{in}, Gap: w.LineGap6}
			if copyPw != nil {
				line.Items = append(line.Items, copyButton(*copyPw, string(a.Password.Value)))
			}
			card.Items = append(card.Items, w.SpacedField{Field: w.Field{Label: passwordLabel, Parts: []w.FieldPart{line}, Width: w.FieldW100}})
		}
		side.Items = append(side.Items, card)
	}
	return w.Form{Items: []w.FormPart{wide, side}, Variant: w.FormFlushBottom}
}

// copyButton is a Copy that puts text on the clipboard, small; without a
// script the text stands beside it to be selected.
func copyButton(a view.Action, text string) w.Button {
	b := w.Button{Label: string(a.Label), Small: true, Copy: text, Copied: copiedWord}
	if !a.Enabled {
		b.State, b.Reason = w.ButtonDisabled, string(a.Reason)
	}
	return b
}

// addressTable is Thunderbird and anything else (s-plan-4, -5): a row an
// account — its name, its login and its password with Copy password —, a
// row a collection — its mark and name, its protocol, its address, Copy.
// The protocol is what a row's card keeps on a phone: where to paste the
// address Copy copies (TableCol.Main, Typecase's note 160, Р-66).
func addressTable(accounts []view.DeviceAccount) w.Table {
	t := w.Table{Cols: []w.TableCol{{Label: colCollection}, {Label: colType, Main: true}, {Label: colURL, Rank: w.Rank3}, {Width: w.ColNum, Rank: w.Rank4}}}
	for _, a := range accounts {
		line := w.Line{Gap: w.LineGap8, Items: []w.LinePart{w.Strong(a.Label), w.Dim{Parts: []w.Inline{w.Text(loginWord)}}, w.Mono{Text: string(a.Login)}}}
		if a.Refused != "" {
			// Its row alone, in alarm (s-plan-4, -5).
			line.Items = append(line.Items, w.Faced{Face: w.FaceDanger, Items: []w.LinePart{w.Text(a.Refused)}})
		} else if a.Password.Shown {
			line.Items = append(line.Items, w.Dim{Parts: []w.Inline{w.Text(passwordWord)}}, w.Mono{Text: string(a.Password.Value)})
			for _, act := range a.Actions {
				if act.ID == passwordCopyID {
					line.Items = append(line.Items, copyButton(act, string(a.Password.Value)))
				}
			}
		} else if a.Password.Reveal.ID != "" {
			line.Items = append(line.Items, w.Dim{Parts: []w.Inline{w.Text(passwordWord)}}, bind.Button(a.Password.Reveal, ""))
		}
		t.Rows = append(t.Rows, w.TableRow{Cells: []w.TableCell{{Parts: []w.CellPart{line}, Span: 4}}})
		for _, r := range a.Rows {
			if !r.Included {
				continue
			}
			c := copyButton(r.Copy, string(r.URL))
			t.Rows = append(t.Rows, w.TableRow{Cells: []w.TableCell{
				{Parts: []w.CellPart{w.TableFile{Mark: &w.Mark{Colour: string(r.Colour)}, Name: string(r.Label)}}},
				typeCell(r),
				{Parts: []w.CellPart{w.Text(r.URL)}, Mono: true, Small: true},
				{Parts: []w.CellPart{c}, Acts: true},
			}})
		}
	}
	return t
}

// unlockMark is, while the window of the credentials is open, its mark
// after the line under the title and Lock now (s-plan-1, s-profile-4).
func unlockMark(h *w.Head, top view.TopBar) {
	left := top.UnlockLeft
	if left == "" || len(h.Subs) == 0 {
		return
	}
	h.Subs[0].Parts = append(h.Subs[0].Parts, w.Text(" "), w.Badge{Label: unlockedMark + string(left), Accent: true})
	if a := top.Lock; a != nil {
		b := bind.Button(*a, bind.TickID(*a))
		b.Small = true
		h.Subs[0].Parts = append(h.Subs[0].Parts, b)
	}
}

// DevicesForms are the forms of the page's actions that post with none of
// their own: Lock now, the accounts' ticks, Generate profile, the link's
// Revoke, and the top bar's.
func DevicesForms(p *view.DevicesPage) []w.PostForm {
	out := forms(p.Top, p.Notices)
	post := func(a view.Action) {
		if a.Method == "POST" && a.Enabled {
			out = append(out, bind.ActionForm(a, bind.TickID(a)))
		}
	}
	if a := p.Top.Lock; a != nil {
		post(*a)
	}
	for _, a := range p.Accounts {
		if a.Toggle != nil {
			post(*a.Toggle)
		}
	}
	for _, a := range p.Actions {
		post(a)
	}
	if l := p.Link; l != nil {
		for _, a := range l.Actions {
			post(a)
		}
	}
	return out
}

// typeCell is a collection's protocol, «ro» after it where the collection
// is read only (s-plan-5).
func typeCell(r view.DeviceRow) w.TableCell {
	c := w.TableCell{Parts: []w.CellPart{w.Text(r.Type)}, Mono: true}
	if r.ReadOnly {
		c.Parts = []w.CellPart{w.Text(string(r.Type) + " "), w.Badge{Label: readOnly}}
	}
	return c
}
