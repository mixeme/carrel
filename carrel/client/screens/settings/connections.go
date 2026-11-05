package settings

import (
	"strconv"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of the connections (U-1.3): the rubrics of the two lists, the
// columns, the badge of a feed, a collection's kinds, the words of the
// address line, «⋯».
const (
	rubricServers   = "DAV servers"
	rubricFeeds     = "Subscriptions · external, read-only"
	colCollection   = "Collection"
	colFolder       = "Folder"
	colWhatsIn      = "What is in it"
	colObjects      = "Objects"
	colChanged      = "Changed on the server"
	colLandsIn      = "Lands in"
	feedBadge       = "feed"
	roBadge         = "ro"
	lastAnswer      = "last answer "
	lastRefused     = "last answer refused"
	readWord        = "read "
	lastRead        = "last read "
	lineSep         = " · "
	moreTitle       = "More"
	kindSep         = " · "
	newCollectionID = "collection.new"
	revealID        = "connection.password"
	rediscoverID    = "connection.rediscover"
	picksID         = "conn-picks"
	pickName        = "pick"
	newBadge        = "new"
	disconnectID    = "connection.disconnect"
)

// kindWords are a collection's kinds as the list says them (s-profile-1).
var kindWords = map[view.Kind]string{
	view.KindEvent: "calendar", view.KindTask: "tasks", view.KindNote: "journal", view.KindContact: "address book", view.KindFile: "files",
}

// headActs are the order of a card's buttons in the frame: what is made
// there, Show password, Test, Edit, Disconnect (s-profile-1). «Look for
// changes» has no frame (Typecase's task 193 p. 7).
var headActs = []string{newCollectionID, "folder.new", revealID, "connection.test", rediscoverID, "connection.edit", disconnectID}

// feedActs are a feed card's: Show password, Check, Edit, Unsubscribe.
var feedActs = []string{revealID, "feed.recheck", "feed.edit", "feed.unsubscribe"}

// Connections is /app/settings/connections (ConnectionsPage, s-profile-1):
// who is signed in and how much is connected; Connect account and
// Subscribe to a feed; a card a server — its buttons, its address line, a
// row a collection, ticked where it is polled, its «⋯» —, a refused one in
// alarm with its trace and what it means; then the feeds, each with where
// it lands.
func Connections(p *view.ConnectionsPage) w.Shell {
	h := w.Head{Title: &w.HeadTitle{Text: string(p.Title)}, Subs: bind.Sub(p.Subtitle)}
	unlockMark(&h, p.Top)
	var acts []w.ActsPart
	for _, a := range p.Actions {
		acts = append(acts, bind.Button(a, bind.TickID(a)))
	}
	if len(acts) > 0 {
		h.Acts = &w.Acts{Items: acts}
	}
	res := w.MainResults{}
	if len(p.Accounts) > 0 {
		res.Items = append(res.Items, w.Rubric{Text: rubricServers, Items: []w.RubricPart{w.Mono{Text: strconv.Itoa(len(p.Accounts)), Variant: w.MonoTight}}, Variant: w.RubricSplit})
	}
	// With the panel beside it the page is narrower: Objects goes first
	// (s-profile-10).
	narrow := p.Add != nil && p.DialogTitle == ""
	for _, c := range p.Accounts {
		res.Items = append(res.Items, serverCard(c, narrow))
	}
	if len(p.Subscriptions) > 0 {
		res.Items = append(res.Items, w.Rubric{Text: rubricFeeds, Items: []w.RubricPart{w.Mono{Text: strconv.Itoa(len(p.Subscriptions)), Variant: w.MonoTight}}, Variant: w.RubricSplit})
	}
	for _, f := range p.Subscriptions {
		res.Items = append(res.Items, feedCard(f, narrow))
	}
	// The dialog of a connection stands alone in the main column
	// (s-profile-2, -16).
	if p.Add != nil && p.DialogTitle != "" {
		// The dialog in place of the page; what the server said stands in
		// it, over the fields (s-profile-24).
		return shell(p.Top, p.Rail, []w.MainPart{connectDialog(p)})
	}
	main := []w.MainPart{h}
	main = append(main, bind.ColumnMsgs(bind.Notices(p.Notices, ConnectionsNotices(p)))...)
	if len(p.Empty) > 0 {
		// Nothing connected yet (s-profile-25).
		main = append(main, w.Empty{Parts: as[w.EmptyPart](bind.Inlines(p.Empty))})
	} else {
		main = append(main, res)
	}
	if len(p.Note) > 0 {
		main = append(main, w.Msg{Column: true, Parts: msgParts(p.Note)})
	}
	sh := shell(p.Top, p.Rail, main)
	if p.Add != nil {
		// A subscription's panel beside the page (s-profile-10).
		if b, ok := sh.Body.(w.ShellBody); ok {
			b.Side = feedPanel(p)
			sh.Body = b
		}
	}
	return sh
}

// ConnectionsNotices is where the page's notices stand: in the dialog of a
// connection, flush with its sides, over its fields (s-profile-24); over
// the cards otherwise, 20 px in (s-profile-27).
func ConnectionsNotices(p *view.ConnectionsPage) w.MsgSpace {
	if p.Add != nil && p.DialogTitle != "" {
		return w.MsgFlushSides
	}
	return w.MsgInset20
}

// connectID is the ID of the dialog's form, which its foot submits.
const connectID = "connect"

// connectWidths are the widths the frame gives the dialog's fields.
var connectWidths = map[string]w.FieldWidth{"label": w.FieldMD, "server": w.FieldFull, "login": w.FieldMD, "password": w.FieldMD, "certFingerprint": w.FieldFull}

// connectDialog is Connect account or Edit connection (s-profile-2, -16):
// its title and what one connection gives; the label, the server's
// address, the login, the password — and the tick of a server that asks
// none in its field —, the fingerprint; what a test found under them;
// Connect or Save, Test only and Cancel.
func connectDialog(p *view.ConnectionsPage) w.Dialog {
	f := *p.Add
	head := w.Head{Title: &w.HeadTitle{Text: string(p.DialogTitle)}}
	if p.DialogSubtitle != "" {
		head.Subs = []w.HeadSub{{Parts: []w.HeadSubPart{w.Text(p.DialogSubtitle)}}}
	}
	form := bind.Form(f, connectID)
	form.Variant = w.FormFlush
	rows := bind.Rows(f)
	for i := 0; i < len(rows); i++ {
		r := rows[i]
		fl := bind.ColumnField(r, connectWidths[r.Name])
		if r.Kind == view.InputRadio {
			// What becomes of the published links (s-profile-22, -23): a
			// radio an answer, how many use the connection, and why it came
			// back, in alarm, under it.
			fl = w.Field{Label: string(r.Label), Width: w.FieldFull, Hidden: r.Hidden, Changed: r.Changed}
			for _, o := range r.Options {
				fl.Parts = append(fl.Parts, w.Check{Name: r.Name, Value: o.Value, Radio: true, On: o.Selected, Parts: []w.CheckPart{w.Text(o.Label)}})
			}
			if r.Hint != "" {
				fl.Parts = append(fl.Parts, w.FieldHint{Parts: []w.Inline{w.Text(r.Hint)}, Variant: w.HintColumn})
			}
			if r.Error != "" {
				fl.Parts = append(fl.Parts, w.FieldHint{Parts: []w.Inline{w.Text(r.Error)}, Variant: w.HintDanger})
			}
		}
		if r.Kind == view.InputPassword && i+1 < len(rows) && rows[i+1].Kind == view.InputCheck {
			// «The server asks no password» in the password's field, after
			// it (s-profile-16).
			fl.Parts = append(fl.Parts, bind.Check(rows[i+1]))
			i++
		}
		form.Items = append(form.Items, fl)
	}
	items := []w.DialogPart{head}
	// A refusal and its trace come before the fields (s-profile-24); the
	// trace of Test only, after them.
	for _, n := range p.Notices {
		m := bind.Notice(n, w.MsgFlushSides)
		m.Column = true
		items = append(items, m)
		if d := bind.NoticeDiag(n); d != nil {
			items = append(items, d.Diag)
		}
	}
	if p.Trace != nil && len(p.Notices) > 0 {
		items = append(items, trace(p.Trace.Steps))
	}
	items = append(items, form)
	if p.Trace != nil && len(p.Notices) == 0 {
		items = append(items, trace(p.Trace.Steps))
	}
	foot := w.FormFoot{Variant: w.FormFootFlush}
	for _, a := range f.Actions {
		switch {
		case a.Method == "GET":
			foot.Items = append(foot.Items, bind.Button(a, ""))
		case a.Primary:
			foot.Items = append(foot.Items, bind.SubmitForm(a, connectID))
		default:
			foot.Items = append(foot.Items, bind.SubmitForm(a, connectID))
		}
	}
	return w.Dialog{Items: append(items, foot), Variant: w.Dialog540}
}

// msgParts are runs as the parts of a message.
func msgParts(r view.Runs) []w.MsgPart {
	var out []w.MsgPart
	for _, p := range bind.Inlines(r) {
		out = append(out, p.(w.MsgPart))
	}
	return out
}

// cardButtons are a card's buttons by the order ids gives them, small;
// Show password — or Hide password, shown — from its secret where there is
// one; Update collection list sends the page's one form to its address
// (s-profile-1, Typecase's note 103); a destructive one in the danger's
// colour.
func cardButtons(actions []view.Action, reveal *view.SecretField, ids []string) w.Line {
	by := map[string]view.Action{}
	for _, a := range actions {
		by[a.ID] = a
	}
	if reveal != nil {
		by[revealID] = reveal.Reveal
	}
	line := w.Line{Gap: w.LineGap6}
	for _, id := range ids {
		a, ok := by[id]
		if !ok {
			continue
		}
		b := bind.Button(a, bind.TickID(a))
		if a.ID == rediscoverID && a.Enabled {
			b.Form, b.To = picksID, a.Href
		}
		b.Small = true
		if a.Danger {
			b.Kind = w.ButtonDanger
		}
		line.Items = append(line.Items, b)
	}
	return line
}

// serverCard is a server: its name and buttons, the line of its address,
// its login and when it last answered; a row a collection — or, refused,
// the trace and what the refusal means, the card in alarm.
func serverCard(c view.ConnectionCard, narrow bool) w.Card {
	r := w.Rubric{Text: string(c.Title), Items: []w.RubricPart{cardButtons(c.Actions, c.Password, headActs)}}
	line := []string{string(c.Server), string(c.Login)}
	if c.Access != "" {
		line = append(line, string(c.Access))
	}
	if c.Plain != "" {
		line = append(line, string(c.Plain))
	}
	switch {
	case c.Refused:
		line = append(line, lastRefused)
	case c.Answered != "":
		line = append(line, lastAnswer+string(c.Answered))
	}
	addr := w.Mono{Text: joined(line, lineSep), Form: w.MonoLine, Variant: w.Mono11_5Dusty}
	card := w.Card{Items: []w.CardPart{r, addr}}
	if pw := c.Password; pw != nil && pw.Shown {
		// The password under the address, with Copy; the card on an accent
		// edge (s-profile-4).
		id := "pw-" + c.AccountID
		in := w.Input{Value: string(pw.Value), Label: passwordLabel, Mono: true, ReadOnly: true, Variant: w.InputKeySmFill, ID: id}
		card.Items = append(card.Items, w.CardRow{Kind: w.CardTop8, Gap: 8,
			Items: []w.CardRowPart{in, w.Button{Label: copyWord, Small: true, CopyFrom: id, Copied: copiedWord}}})
		card.Variant = w.CardChosen
	}
	if c.Refused {
		r.Variant, addr.Variant = w.RubricDanger, w.Mono11_5DustyGap
		card.Items = []w.CardPart{r, addr}
		if c.Trace != nil {
			card.Items = append(card.Items, trace(c.Trace.Steps))
		}
		if c.Note != "" {
			card.Items = append(card.Items, w.CardText{Kind: w.CardAlertText, Parts: []w.CardTextPart{w.Text(c.Note)}})
		}
		card.Variant = w.CardAlert
		return card
	}
	if len(c.Collections) > 0 {
		card.Items = append(card.Items, collectionsTable(c.Collections, narrow))
	}
	return card
}

// collectionsTable is a server's collections: the tick that polls it —
// pressed, it posts —, its mark and name, what is in it, how many, when it
// changed on the server — what a row's card keeps on a phone (TableCol.Main,
// Typecase's note 155, Р-66) —, and its «⋯»; one not polled faded.
func collectionsTable(rows []view.CollectionRow, narrow bool) w.Table {
	name := colCollection
	if allFiles(rows) {
		name = colFolder
	}
	t := w.Table{Cols: []w.TableCol{{Width: w.ColTickSmall}, {Label: name}, {Label: colWhatsIn, Rank: w.Rank3},
		{Label: colObjects, Width: w.ColNum, Rank: w.Rank4, Secondary: narrow}, {Label: colChanged, Width: w.ColChanged, Main: true}, {Width: w.ColMenu}}}
	for _, r := range rows {
		tick := bind.Ticked(r.Tick)
		pick := w.TableCell{Pick: &tick}
		if r.Toggle.Enabled {
			// A button of the page's one form (Typecase's note 103).
			pick.Send = w.Send{Form: picksID, Name: pickName, Value: r.Pick}
		}
		var kinds []string
		for _, k := range r.Kinds {
			kinds = append(kinds, kindWords[k])
		}
		what := w.TableCell{Parts: []w.CellPart{w.Text(joined(kinds, kindSep))}, Mono: true}
		switch {
		case r.New:
			// Just found by Update collection list (s-profile-27).
			what.Parts = []w.CellPart{w.Text(joined(kinds, kindSep) + " "), w.Badge{Label: newBadge}}
		case r.ReadOnly:
			what.Parts = []w.CellPart{w.Text(joined(kinds, kindSep) + " "), w.Badge{Label: roBadge}}
		}
		objects := w.TableCell{Mono: true}
		if r.Objects != nil {
			objects.Parts = []w.CellPart{w.Text(strconv.Itoa(*r.Objects))}
		}
		row := w.TableRow{Cells: []w.TableCell{
			pick,
			{Parts: []w.CellPart{w.TableFile{Mark: &w.Mark{Colour: string(r.Colour)}, Name: string(r.Label)}}},
			what, objects,
			{Parts: []w.CellPart{w.Text(r.Changed)}, Mono: true},
			{Acts: true, Parts: []w.CellPart{w.Drop{
				Trigger: w.Button{Icon: "dots", Small: true, Variant: w.ButtonPadX3, Title: moreTitle},
				Menu:    w.Menu{Items: bind.MenuItems(r.Menu, roBadge), Width: w.MenuWidth212, Pop: w.MenuUnderEnd},
			}}},
		}}
		if r.Tick == view.TickOff {
			row.State = w.RowOff
		}
		t.Rows = append(t.Rows, row)
	}
	return t
}

// allFiles reports whether a server holds only file collections: its rows
// are folders (s-profile-1, sftpgo).
func allFiles(rows []view.CollectionRow) bool {
	for _, r := range rows {
		if len(r.Kinds) != 1 || r.Kinds[0] != view.KindFile {
			return false
		}
	}
	return len(rows) > 0
}

// feedCard is a subscription: its name, the badge «feed» and its buttons;
// its address, whether it has a password, when, how much and how fast it
// was read; where its kinds land, each ticked where it is polled, when it
// changed said once for all of them, Objects what a row's card keeps on a
// phone, the collection its name (TableCol.Main, Typecase's note 164) — or,
// refused, the trace and what that means, in alarm.
func feedCard(f view.SubscriptionCard, narrow bool) w.Card {
	r := w.Rubric{Text: string(f.Title), Items: []w.RubricPart{w.Badge{Label: feedBadge}, cardButtons(f.Actions, f.Password, feedActs)}}
	line := []string{string(f.URL)}
	if f.Access != "" {
		line = append(line, string(f.Access))
	}
	refused := f.State == view.CollectionUnavailable
	switch {
	case refused && f.Checked != "":
		line = append(line, lastRead+string(f.Checked))
	case f.Checked != "":
		line = append(line, readWord+string(f.Checked))
	}
	if !refused {
		for _, s := range []view.Text{f.Size, f.Took} {
			if s != "" {
				line = append(line, string(s))
			}
		}
	}
	addr := w.Mono{Text: joined(line, lineSep), Form: w.MonoLine, Variant: w.Mono11_5Dusty}
	if refused {
		r.Variant, addr.Variant = w.RubricDanger, w.Mono11_5DustyGap
		card := w.Card{Items: []w.CardPart{r, addr}, Variant: w.CardAlert}
		if f.Trace != nil {
			card.Items = append(card.Items, trace(f.Trace.Steps))
		}
		if f.Note != "" {
			card.Items = append(card.Items, w.CardText{Kind: w.CardAlertText, Parts: []w.CardTextPart{w.Text(f.Note)}})
		}
		return card
	}
	t := w.Table{Cols: []w.TableCol{{Width: w.ColTickSmall}, {Label: colLandsIn}, {Label: colObjects, Width: w.ColNum, Rank: w.Rank3, Secondary: narrow, Main: true},
		{Label: colChanged, Width: w.ColChanged, Rank: w.Rank4}}}
	for i, l := range f.Lands {
		tick := bind.Ticked(l.Tick)
		pick := w.TableCell{Pick: &tick}
		if l.Toggle.Enabled {
			pick.Send = w.Send{Form: picksID, Name: pickName, Value: l.Pick}
		}
		objects := w.TableCell{Mono: true}
		if l.Objects != nil {
			objects.Parts = []w.CellPart{w.Text(strconv.Itoa(*l.Objects))}
		}
		cells := []w.TableCell{pick, {Parts: []w.CellPart{w.TableFile{Mark: &w.Mark{Colour: string(f.Colour), Kind: w.MarkExternal}, Name: string(l.Label)}}}, objects}
		if i == 0 {
			// One «changed» for the whole feed, spanning its rows.
			cells = append(cells, w.TableCell{Parts: []w.CellPart{w.Text(f.Changed)}, Mono: true, Rows: len(f.Lands)})
		}
		row := w.TableRow{Cells: cells}
		if l.Tick == view.TickOff {
			row.State = w.RowOff
		}
		t.Rows = append(t.Rows, row)
	}
	return w.Card{Items: []w.CardPart{r, addr, t}}
}

// ConnectionsForms are the forms of the page's actions that post with none
// of their own: the cards' buttons; the page's one form, which each
// collection's and land's tick and Update collection list send (conn-picks,
// Typecase's note 103); and the top bar's.
func ConnectionsForms(p *view.ConnectionsPage) []w.PostForm {
	out := forms(p.Top, p.Notices)
	if a := p.Top.Lock; a != nil && a.Method == "POST" && a.Enabled {
		out = append(out, bind.ActionForm(*a, bind.TickID(*a)))
	}
	post := func(a view.Action) {
		if a.Method == "POST" && a.Enabled && !a.Submits && a.ID != rediscoverID {
			out = append(out, bind.ActionForm(a, bind.TickID(a)))
		}
	}
	picks := false
	pick := func(a view.Action) {
		if a.Enabled && !picks {
			out, picks = append(out, bind.ActionForm(a, picksID)), true
		}
	}
	for _, c := range p.Accounts {
		for _, a := range c.Actions {
			post(a)
		}
		for _, r := range c.Collections {
			pick(r.Toggle)
		}
	}
	for _, f := range p.Subscriptions {
		for _, a := range f.Actions {
			post(a)
		}
		for _, l := range f.Lands {
			pick(l.Toggle)
		}
	}
	return out
}

// trace is a check's steps as the block draws them: a line a step, its
// name in brackets, what the server answered after it.
func trace(steps []view.TraceStep) w.Diag {
	var lines []bind.DiagLine
	for _, s := range steps {
		lines = append(lines, bind.DiagLine{Name: "[" + string(s.Name) + "]", Value: string(s.Detail)})
	}
	return bind.Diag(lines...)
}

// joined are words with sep between them.
func joined(words []string, sep string) string {
	out := ""
	for i, w := range words {
		if i > 0 {
			out += sep
		}
		out += w
	}
	return out
}

// The words of the feed's panel (U-1.3): its title and the line under it,
// the cross's name, the rubric of what a feed carries.
const (
	feedTitle    = "Subscribe to a feed"
	feedSub      = "Somebody else’s calendar or address book, read-only"
	feedFolder   = "Subscribe to a folder"
	closePanel   = "Close the panel"
	subscribeID  = "subscribe"
	feedCheckID  = "feed.check"
	feedCancelID = "feed.cancel"
)

// feedPanel is the panel of a subscription beside the page (s-profile-10):
// the address and Check the address, what the check was answered, the
// name, the colour of its label, whether it asks for a password — the login
// and the password while it does —, what it carries; why a feed is read
// by the server, in alarm; Subscribe and Cancel.
func feedPanel(p *view.ConnectionsPage) *w.Side {
	f := *p.Add
	title := feedTitle
	form := bind.Form(f, subscribeID)
	for _, r := range bind.Rows(f) {
		switch {
		case r.Name == "url":
			if r.Label != "Feed address" {
				title = feedFolder
			}
			in := bind.Input(r)
			in.Variant = w.Input11_5
			form.Items = append(form.Items, w.Field{Label: string(r.Label), Parts: append([]w.FieldPart{in}, bind.Column(bind.Hint(r))...), Width: w.FieldW100})
			for _, a := range f.Actions {
				if a.ID == feedCheckID {
					b := bind.Button(a, "")
					if a.Enabled {
						b.Form, b.Href, b.Submit, b.To = "", "", true, a.Href
					}
					form.Items = append(form.Items, w.Field{Parts: []w.FieldPart{b}, Width: w.FieldW100, OwnLine: true})
				}
			}
			if p.Trace != nil {
				form.Items = append(form.Items, w.Field{Parts: []w.FieldPart{trace(p.Trace.Steps)}, Width: w.FieldW100})
			}
		case r.Kind == view.InputRadio:
			// The label's colour: a choice that sends without a script
			// (Swatches, Typecase's task 135).
			sw := w.Swatches{Name: r.Name}
			for i, o := range r.Options {
				sw.Colours = append(sw.Colours, o.Value)
				if o.Selected {
					sw.On = i
				}
			}
			// What the colour comes from, muted under the swatches
			// (s-profile-10, note 94).
			parts := []w.FieldPart{sw}
			if r.ValueAside != "" {
				parts = append(parts, w.Dim{Parts: []w.Inline{w.Text(r.ValueAside)}, Size: w.Dim11})
			}
			form.Items = append(form.Items, w.Field{Label: string(r.Label), Parts: append(parts, bind.Column(bind.Hint(r))...), Width: w.FieldW100})
		case r.Kind == view.InputCheck:
			form.Items = append(form.Items, w.Field{Parts: append([]w.FieldPart{bind.Check(r)}, bind.Column(bind.Hint(r))...), Width: w.FieldW100, OwnLine: true})
		case r.Multiple && r.ReadOnly:
			// What the feed carries: shown, not chosen.
			tags := w.TagInput{}
			for _, o := range r.Options {
				tags.Parts = append(tags.Parts, w.Tag{Label: string(o.Label)})
			}
			form.Items = append(form.Items, w.Field{Label: string(r.Label), Parts: append([]w.FieldPart{tags}, bind.Column(bind.Hint(r))...), Width: w.FieldW100})
		default:
			form.Items = append(form.Items, bind.ColumnField(r, w.FieldW100))
		}
	}
	side := &w.Side{Items: []w.SidePart{
		w.Head{Title: &w.HeadTitle{Text: title, Size: w.HeadTitle17}, Subs: []w.HeadSub{{Parts: []w.HeadSubPart{w.Text(feedSub)}}},
			Acts: &w.Acts{Items: []w.ActsPart{w.Close{Label: closePanel, Href: cancelHref(f.Actions)}}}, Pad: w.HeadPadBottom8},
		form,
	}}
	foot := w.FormFoot{}
	for _, a := range f.Actions {
		switch {
		case a.ID == feedCheckID:
		case a.Method == "GET":
			foot.Items = append(foot.Items, bind.Button(a, ""))
		default:
			foot.Items = append(foot.Items, bind.SubmitForm(a, subscribeID))
		}
	}
	side.Items = append(side.Items, foot)
	return side
}

// cancelHref is where Cancel of a form goes: the cross of its panel too.
func cancelHref(actions []view.Action) string {
	for _, a := range actions {
		if a.Method == "GET" {
			return a.Href
		}
	}
	return ""
}
