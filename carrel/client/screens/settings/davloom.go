package settings

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of davloom (U-1.3): the address for the client, the columns of
// the devices, the rubrics of the lists, what a device sees by kind, the
// IDs of a device's actions, the form of the dialogs.
const (
	addressLabel   = "Address for the client"
	colDevice      = "Device"
	colScope       = "What it can see and do"
	colCreated     = "Created"
	colLastRequest = "Last request"
	devicesRubric  = "Devices"
	treeRubric     = "What the client sees"
	allKinds       = "all kinds"
	kindsSep       = ", "
	neverSeen      = "—"
	deviceFormID   = "device"
	actsFormID     = "davloom-acts"
	offBadge       = "off"
	scopeID        = "davloom.scope"
	readID         = "davloom.access.read"
	writeID        = "davloom.access.write"
	revokeID       = "davloom.revoke"
)

// scopeKinds are what a device sees, by kind, as its row says it
// (s-davloom-1): tasks and notes live in calendars.
var scopeKinds = []struct {
	kinds []view.Kind
	word  string
}{
	{[]view.Kind{view.KindEvent, view.KindTask, view.KindNote}, "Calendars"},
	{[]view.Kind{view.KindContact}, "Address books"},
	{[]view.Kind{view.KindFile}, "Files"},
}

// treeIcons are the icons of the branches of what the client sees
// (s-davloom-1): the root, the principal, calendars, address books, files.
var treeIcons = map[view.Text]w.Icon{"principal/": "con", "cal/": "cal", "card/": "con", "files/": "file"}

// Davloom is /app/settings/davloom (DavloomPage, s-davloom-1): «off» in
// alarm where it is off on the instance (s-davloom-8); the address for the
// client with Copy and What the client sees, and what to type there; that
// a new password of Carrel disconnects no device; the devices — each its
// name, what it sees, how many collections with Change, read only or read
// and write, when it was made and last asked, Revoke —, or that there is
// none yet (s-davloom-6). Add device is a dialog in place of it
// (s-davloom-2), and the password issued another, shown once (s-davloom-3).
func Davloom(p *view.DavloomPage) w.Shell {
	main := bind.ColumnMsgs(bind.Notices(p.Notices, w.MsgSpacePage))
	if d := p.Dialog; d != nil {
		return shell(p.Top, p.Rail, append(main, deviceDialog(d)))
	}
	t := &w.HeadTitle{Text: string(p.Title)}
	switch {
	case !p.Enabled:
		t.Badge = &w.Badge{Label: offBadge, Tone: w.BadgeAlert}
	case p.Top.Signed.Local:
		t.Badge = &w.Badge{Label: localBadge, Tone: w.BadgeLocal, Title: localHint}
	}
	h := w.Head{Title: t, Subs: bind.Sub(p.Subtitle)}
	var acts []w.ActsPart
	for _, a := range p.Actions {
		acts = append(acts, bind.Button(a, bind.TickID(a)))
	}
	h.Acts = &w.Acts{Items: acts}
	main = append([]w.MainPart{h}, main...)
	if p.Address != "" {
		in := w.Input{Value: string(p.Address), Label: addressLabel, Mono: true, ReadOnly: true, Variant: w.Input11_5Fill}
		line := w.Line{Items: []w.LinePart{in, copyButton(view.Action{Label: copyWord, Enabled: true}, string(p.Address))}, Gap: w.LineGapNormal}
		if p.Sees.ID != "" {
			// The tree is a sheet behind it (s-davloom-9).
			b := bind.Button(p.Sees, "")
			b.Small = true
			line.Items = append(line.Items, b)
		}
		f := w.Field{Label: addressLabel, Parts: []w.FieldPart{line}, Width: w.FieldMax640Fill460}
		if p.AddressHint != "" {
			f.Parts = append(f.Parts, w.FieldHint{Parts: []w.Inline{w.Text(p.AddressHint)}, Variant: w.HintColumn})
		}
		main = append(main, w.Form{Items: []w.FormPart{f}, Variant: w.FormBottom12})
	}
	if len(p.Note) > 0 {
		main = append(main, w.Msg{Column: true, Parts: bind.MsgParts(p.Note), Tone: w.MsgAccent})
	}
	switch {
	case len(p.Devices) > 0:
		main = append(main, w.ListGroup{Title: w.Rubric{Text: devicesRubric, Form: w.RubricInline}, Count: &w.Num{Value: len(p.Devices)}},
			devicesTable(p.Devices))
	case len(p.Empty) > 0:
		main = append(main, w.ListGroup{Title: w.Rubric{Text: devicesRubric, Form: w.RubricInline}},
			w.Empty{Parts: as[w.EmptyPart](bind.Inlines(p.Empty))})
	}
	return shell(p.Top, p.Rail, main)
}

// devicesTable is the devices (s-davloom-1): a row a device.
func devicesTable(devices []view.ProxyDevice) w.Table {
	t := w.Table{Variant: w.TableFit, Cols: []w.TableCol{
		// What it can see and do is what a row's card keeps on a phone
		// (TableCol.Main, Typecase's note 155, Р-66).
		{Label: colDevice}, {Label: colScope, Main: true}, {Label: colCreated, Rank: w.Rank4, Secondary: true}, {Label: colLastRequest, Secondary: true}, {Width: w.Col90},
	}}
	for _, d := range devices {
		marks := w.Line{Gap: w.LineGapTight}
		words := scopeWords(d.Scope)
		if words == allKinds {
			marks.Items = []w.LinePart{w.Mark{Kind: w.MarkMulti}}
		} else {
			for _, c := range d.Marks {
				marks.Items = append(marks.Items, w.Mark{Colour: string(c)})
			}
		}
		wrap := w.CellWrap{Items: []w.CellPart{marks, w.Mono{Text: words, Variant: w.Mono11}}}
		access := w.Seg{Variant: w.SegCompact}
		revoke := w.TableCell{Acts: true}
		for _, a := range d.Actions {
			switch a.ID {
			case scopeID:
				// Its sheet, at the cell's end (s-davloom-7).
				b := bind.Button(a, "")
				b.Small, b.Place = true, w.Pushed
				wrap.Items = append(wrap.Items, b)
			case readID, writeID:
				// Each sends the page's one form to its own address
				// (Typecase's note 102).
				access.Options = append(access.Options, w.SegOption{Label: string(a.Label), On: a.On, Form: actsFormID, To: a.Href})
			case revokeID:
				b := bind.Button(a, bind.TickID(a))
				b.Small, b.Kind = true, w.ButtonDanger
				revoke.Parts = append(revoke.Parts, b)
			}
		}
		if len(access.Options) > 0 {
			wrap.Items = append(wrap.Items, access)
		}
		seen := w.TableCell{Parts: []w.CellPart{w.Text(neverSeen)}, Mono: true}
		if d.LastSeen != "" {
			seen.Parts = []w.CellPart{w.Text(d.LastSeen)}
		}
		t.Rows = append(t.Rows, w.TableRow{Cells: []w.TableCell{
			{Parts: []w.CellPart{w.TableFile{Icon: "device", Name: string(d.Name)}}},
			{Parts: []w.CellPart{wrap}},
			{Parts: []w.CellPart{w.Text(d.Created)}, Mono: true},
			seen,
			revoke,
		}})
	}
	return t
}

// scopeWords is what a device sees, in words: its kinds by what holds them
// — «Calendars, Address books» —, «all kinds» when it sees every one.
func scopeWords(scope []view.Kind) string {
	has := map[view.Kind]bool{}
	for _, k := range scope {
		has[k] = true
	}
	out := ""
	all := true
	for _, kw := range scopeKinds {
		some := false
		for _, k := range kw.kinds {
			if has[k] {
				some = true
			} else {
				all = false
			}
		}
		if some {
			if out != "" {
				out += kindsSep
			}
			out += kw.word
		}
	}
	if all {
		return allKinds
	}
	return out
}

// proxyTree is what a client sees at the address (s-davloom-1): the root,
// its branches each with its icon and what is in it, the collections under
// them with their marks.
func proxyTree(rows []view.ProxyTreeRow) w.Tree {
	t := w.Tree{Variant: w.TreeNarrow}
	for _, r := range rows {
		row := w.TreeRow{Label: string(r.Path), Code: true}
		switch r.Depth {
		case 0:
			row.Icon = "file"
		case 1:
			row.Icon, row.Indent = treeIcons[r.Path], w.TreeLevel22
		default:
			row.Mark, row.Indent = &w.Mark{Colour: string(r.Colour)}, w.TreeLevel44
		}
		if r.Label != "" {
			row.Trail = w.Dim{Parts: []w.Inline{w.Text(r.Label)}, Size: w.Dim11}
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}

// deviceDialog is Add device (s-davloom-2) or the password it issued
// (s-davloom-3), in place of the page.
func deviceDialog(d *view.DeviceDialog) w.Dialog {
	dl := w.Dialog{Variant: w.Dialog520, Items: []w.DialogPart{head(d.Title, d.Subtitle)}}
	if pw := d.Password; pw != nil {
		id := deviceFormID + "-password"
		in := w.Input{Value: string(pw.Value), Label: string(pw.Label), Mono: true, ReadOnly: true, Variant: w.InputKeyFill, ID: id}
		card := w.Card{Variant: w.CardChosen, Items: []w.CardPart{
			w.Rubric{Text: string(pw.Label)},
			w.CardRow{Kind: w.CardTop2, Gap: w.LineGapNormal, Items: []w.CardRowPart{in, w.Button{Label: string(pw.Copy.Label), CopyFrom: id, Copied: copiedWord}}},
		}}
		if pw.Warning != "" {
			card.Items = append(card.Items, w.CardText{Kind: w.CardWarn, Parts: []w.CardTextPart{w.Text(pw.Warning)}})
		}
		dl.Items = append(dl.Items, card)
	}
	form := bind.Form(d.Form, deviceFormID)
	form.Variant = w.FormFlush
	for _, set := range d.Form.Sets {
		form.Items = append(form.Items, deviceSet(set)...)
	}
	dl.Items = append(dl.Items, form)
	if d.Warning != "" {
		dl.Items = append(dl.Items, w.Msg{Column: true, Parts: []w.MsgPart{w.Text(d.Warning)}, Tone: w.MsgAlert, Space: w.MsgFlushSides})
	}
	foot := w.FormFoot{Variant: w.FormFootFlush}
	for _, a := range d.Form.Actions {
		if a.Method == "GET" && !a.Primary {
			foot.Items = append(foot.Items, bind.Button(a, ""))
			continue
		}
		foot.Items = append(foot.Items, bind.SubmitForm(a, deviceFormID))
	}
	dl.Items = append(dl.Items, foot)
	return dl
}

// deviceSet is a set of a device's form (s-davloom-2, -3): the name, what
// it may do as segments, the address and the login read only; a set with a
// legend — what it sees, its collections — one field of ticks in a column.
func deviceSet(set view.FieldSet) []w.FormPart {
	if set.Legend != "" {
		g := w.Group{Form: w.GroupColumn}
		for _, r := range set.Rows {
			if r.Kind == view.InputCheck {
				g.Items = append(g.Items, bind.Check(r))
			}
		}
		f := w.Field{Label: string(set.Legend), Parts: []w.FieldPart{g}, Width: w.FieldW100}
		if set.LegendAside != "" {
			f.LabelAside = w.Dim{Parts: []w.Inline{w.Text(set.LegendAside)}}
		}
		return []w.FormPart{f}
	}
	var out []w.FormPart
	for _, r := range set.Rows {
		switch r.Kind {
		case view.InputHidden:
		case view.InputRadio:
			f := segField(r)
			f.Label, f.Width = string(r.Label), w.FieldW100
			out = append(out, f)
		default:
			f := bind.ColumnField(r, w.FieldW100)
			if r.ReadOnly && r.Name == "address" {
				if in, ok := f.Parts[0].(w.Input); ok {
					in.Variant = w.Input11
					f.Parts[0] = in
				}
			}
			out = append(out, f)
		}
	}
	return out
}

// DavloomForms are the forms of the page's actions that post with none of
// their own: a device's Revoke; the one form a device's access sends, to
// the page's address (s-davloom-1); and the top bar's.
func DavloomForms(p *view.DavloomPage) []w.PostForm {
	out := forms(p.Top, p.Notices)
	if len(p.Devices) > 0 && p.Acts.Action != "" {
		out = append(out, w.PostForm{ID: actsFormID, Action: p.Acts.Action, Fields: bind.Hidden(p.Acts)})
	}
	for _, d := range p.Devices {
		for _, a := range d.Actions {
			if a.ID != readID && a.ID != writeID && a.Method == "POST" && a.Enabled {
				out = append(out, bind.ActionForm(a, bind.TickID(a)))
			}
		}
	}
	return out
}
