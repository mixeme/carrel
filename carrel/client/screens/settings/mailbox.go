package settings

import (
	"slices"
	"strconv"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of the mailboxes' screens (U-1.3): the rubrics, the columns of
// the roles' table, the badges, the trace's rubric.
const (
	rubricMailboxes  = "Mailboxes"
	rubricServersOf  = "Servers"
	rubricTried      = "What was tried"
	colRole          = "Role"
	colFolderOnSrv   = "Folder on the server"
	colHowFound      = "How it was found"
	mailOffBadge     = "off on this instance"
	sendOffBadge     = "sending does not work"
	readingWorks     = "reading works"
	mailboxFormID    = "mailbox"
	foldersFormID    = "folders"
	mailboxSendCheck = "mailbox.check"
	sentCopyName     = "sentCopy"
)

// mailboxWidths are the widths the frames give the dialog's fields
// (s-mailbox-22, -27).
var mailboxWidths = map[string]w.FieldWidth{
	"address": w.FieldLG, "label": w.FieldMD, "colour": w.FieldMD, "password": w.FieldMD, "fromName": w.FieldMD,
	"imap.host": w.FieldLG, "imap.port": w.FieldXS, "smtp.host": w.FieldLG, "smtp.port": w.FieldXS,
	"archive": w.FieldMD, "spam": w.FieldMD, "sent": w.FieldMD, "drafts": w.FieldMD, "trash": w.FieldMD,
}

// mailboxActs are the order of a mailbox card's buttons (s-mailbox-26):
// Show password, Test, Edit, Disconnect; off on the instance, Remove.
var mailboxActs = []string{revealID, mailboxSendCheck, "mailbox.edit", "mailbox.remove"}

// roleIcons are the icons of the roles in the card's table (s-mailbox-26).
var roleIcons = map[view.MailRole]w.Icon{
	view.MailSent: "send", view.MailDrafts: "pencil", view.MailArchive: "archive", view.MailJunk: "spam", view.MailTrash: "trash",
}

// Mailbox is a person's own mailbox in a dialog alone in the main zone
// (MailboxPage): connecting it (s-mailbox-22; Microsoft refused by its
// domain, -23), the outcome of one that reads and does not send (-25), its
// edit (-27).
func Mailbox(p *view.MailboxPage) w.Shell {
	head := w.Head{Title: &w.HeadTitle{Text: string(p.Title)}, Subs: bind.Sub(p.Subtitle)}
	items := []w.DialogPart{head}
	for _, n := range p.Notices {
		m := bind.Notice(n, w.MsgFlush)
		items = append(items, m)
	}
	variant := w.Dialog540
	if p.Form == nil {
		// The outcome (s-mailbox-25): saved, what was tried, what to do.
		variant = w.Dialog560
		if len(p.Saved) > 0 {
			items = append(items, w.Msg{Parts: msgParts(p.Saved), Tone: w.MsgAccent, Space: w.MsgFlush})
		}
		if p.Trace != nil {
			sec := w.Sec{Items: []w.SecPart{w.Rubric{Text: rubricTried}, bind.MailTrace(p.Trace.Steps)}, Flush: true}
			if p.TraceNote != "" {
				sec.Items = append(sec.Items, w.SecNote{Text: string(p.TraceNote)})
			}
			items = append(items, sec)
		}
		foot := w.FormFoot{Variant: w.FormFootFlush}
		for _, a := range p.Actions {
			foot.Items = append(foot.Items, bind.Button(a, ""))
		}
		return shell(p.Top, p.Rail, []w.MainPart{w.Dialog{Items: append(items, foot), Variant: variant}})
	}
	// The edit is the wider dialog (s-mailbox-27): it holds the signature.
	if slices.ContainsFunc(bind.Rows(*p.Form), func(r view.FormRow) bool { return r.Name == "signature" }) {
		variant = w.Dialog560
	}
	form := bind.Form(*p.Form, mailboxFormID)
	form.Variant = w.FormFlush
	for _, r := range bind.Rows(*p.Form) {
		switch {
		case r.Name == "password" && len(p.Provider) > 0:
			// What a known provider asks stands before the password
			// (s-mailbox-22).
			form.Items = append(form.Items, providerMsg(p), mailboxField(r))
		default:
			form.Items = append(form.Items, mailboxField(r))
		}
	}
	if p.Closed && len(p.Provider) > 0 {
		// Microsoft (s-mailbox-23): no password, the refusal after the
		// address.
		form.Items = append(form.Items, providerMsg(p))
	}
	if s := p.Servers; s != nil {
		rubric := w.Rubric{Text: rubricServersOf}
		for _, b := range s.Badges {
			rubric.Items = append(rubric.Items, bind.Badge(b))
		}
		fs := w.FieldSet{Rubric: rubric}
		var lines []bind.DiagLine
		for _, l := range s.Lines {
			lines = append(lines, bind.DiagLine{Name: string(l.Role), Value: string(l.Address) + "  " + string(l.Security)})
		}
		if s.From != "" {
			lines = append(lines, bind.DiagLine{Value: string(s.From)})
		}
		d := bind.Diag(lines...)
		d.Place = w.Grown
		b := bind.Button(s.Manual, "")
		b.Small = true
		fs.Items = append(fs.Items, d, b)
		form.Items = append(form.Items, fs)
	}
	if len(p.Escrow) > 0 {
		form.Items = append(form.Items, w.Msg{Parts: msgParts(p.Escrow), Tone: w.MsgAlert, Space: w.MsgFlush, Place: w.Grown})
	}
	if p.Trace != nil {
		form.Items = append(form.Items, w.Field{Parts: []w.FieldPart{bind.MailTrace(p.Trace.Steps)}, Width: w.FieldFull})
	}
	foot := w.FormFoot{Variant: w.FormFootFlush}
	for _, a := range p.Actions {
		switch {
		case a.Method == "GET":
			foot.Items = append(foot.Items, bind.Button(a, ""))
		default:
			foot.Items = append(foot.Items, bind.SubmitForm(a, mailboxFormID))
		}
	}
	return shell(p.Top, p.Rail, []w.MainPart{w.Dialog{Items: append(items, form, foot), Variant: variant}})
}

// providerMsg is what a known provider asks — an app password, in the
// accent (s-mailbox-22) — or why it cannot be connected, in alarm (-23).
func providerMsg(p *view.MailboxPage) w.Msg {
	tone := w.MsgAccent
	if p.Closed {
		tone = w.MsgAlert
	}
	return w.Msg{Parts: msgParts(p.Provider), Tone: tone, Space: w.MsgFlush, Place: w.Grown}
}

// mailboxField is a row of a mailbox's dialog as its frame draws it: the
// colour as swatches, a tick on its own line, the signature the whole
// width, the rest in their widths.
func mailboxField(r view.FormRow) w.FormPart {
	switch {
	case r.Kind == view.InputRadio:
		sw := w.Swatches{Name: r.Name}
		for i, o := range r.Options {
			sw.Colours = append(sw.Colours, o.Value)
			if o.Selected {
				sw.On = i
			}
		}
		return w.Field{Label: string(r.Label), Parts: append([]w.FieldPart{sw}, bind.Hint(r)...), Width: mailboxWidths[r.Name]}
	case r.Kind == view.InputCheck:
		return w.Field{Parts: append([]w.FieldPart{bind.Check(r)}, bind.Hint(r)...), OwnLine: true, Place: w.Grown}
	case r.Kind == view.InputArea:
		return bind.Field(r, w.FieldFull)
	}
	return bind.Field(r, mailboxWidths[r.Name])
}

// MailboxFolders is the question of the folders the server did not name
// (MailboxFoldersPage, s-mailbox-24): a choice a role, what suggested it
// under it, the copy of sent letters; Save and Skip.
func MailboxFolders(p *view.MailboxFoldersPage) w.Shell {
	head := w.Head{Title: &w.HeadTitle{Text: string(p.Title)}}
	if p.Subtitle != "" {
		head.Subs = []w.HeadSub{{Parts: []w.HeadSubPart{w.Text(p.Subtitle)}}}
	}
	form := bind.Form(p.Form, foldersFormID)
	form.Variant = w.FormFlush
	for _, r := range bind.Rows(p.Form) {
		form.Items = append(form.Items, mailboxField(r))
	}
	foot := w.FormFoot{Variant: w.FormFootFlush}
	for _, a := range p.Form.Actions {
		if a.Method == "GET" {
			foot.Items = append(foot.Items, bind.Button(a, ""))
		} else {
			foot.Items = append(foot.Items, bind.SubmitForm(a, foldersFormID))
		}
	}
	return shell(p.Top, p.Rail, []w.MainPart{w.Dialog{Items: []w.DialogPart{head, form, foot}, Variant: w.DialogCanvas420}})
}

// mailboxCards are the rubric Mailboxes of the connections and its cards
// (s-mailbox-26): the label with the mailbox's mark, its buttons, the
// address line, the roles' table, the copy of sent letters in the foot; a
// mailbox that does not send in alarm with what SMTP answered; off on the
// instance — the line above the cards, each with Remove only (-28).
func mailboxCards(p *view.ConnectionsPage) []w.ResultsPart {
	if len(p.Mailboxes) == 0 {
		return nil
	}
	rubric := w.Rubric{Text: rubricMailboxes, Variant: w.RubricSplit}
	if p.MailboxesOff != "" {
		rubric.Items = append(rubric.Items, w.Badge{Label: mailOffBadge})
	}
	rubric.Items = append(rubric.Items, w.Mono{Text: strconv.Itoa(len(p.Mailboxes)), Variant: w.MonoTight})
	out := []w.ResultsPart{rubric}
	if p.MailboxesOff != "" {
		out = append(out, w.Msg{Parts: []w.MsgPart{w.Text(p.MailboxesOff)}, Space: w.MsgFlush})
	}
	for _, m := range p.Mailboxes {
		out = append(out, mailboxCard(m))
	}
	return out
}

func mailboxCard(m view.MailboxCard) w.Card {
	acts := cardButtons(m.Actions, m.Password, mailboxActs)
	if m.Off != "" {
		// Off on the instance (s-mailbox-28): the label, Remove, the
		// address and why it is not used.
		return w.Card{Variant: w.CardFlush, Items: []w.CardPart{
			w.Rubric{Text: string(m.Label), Items: []w.RubricPart{acts}},
			w.Mono{Text: string(m.Address) + lineSep + string(m.Off), Form: w.MonoLine, Variant: w.Mono11_5Dusty},
		}}
	}
	name := w.Line{Items: []w.LinePart{w.Mark{Colour: string(m.Colour), Size: w.MarkH11}, w.Text(string(m.Label))}, Gap: w.LineGapNormal}
	rubric := w.Rubric{Items: []w.RubricPart{name, acts}}
	line := []string{string(m.Address), "IMAP " + string(m.IMAP)}
	if m.SMTP != "" {
		line = append(line, "SMTP "+string(m.SMTP))
	}
	card := w.Card{}
	if m.SendRefused {
		name.Items[1] = w.Text(string(m.Label) + " ")
		name.Items = append(name.Items, w.Badge{Label: sendOffBadge, Tone: w.BadgeAlert})
		rubric = w.Rubric{Items: []w.RubricPart{name, acts}, Variant: w.RubricDanger}
		line = append(line, readingWorks)
		card.Variant = w.CardAlert
		card.Items = append(card.Items, rubric,
			w.Mono{Text: joined(line, lineSep), Form: w.MonoLine, Variant: w.Mono11_5DustyGap})
		if m.Trace != nil {
			card.Items = append(card.Items, bind.MailTrace(m.Trace.Steps))
		}
		if m.Note != "" {
			card.Items = append(card.Items, w.CardText{Kind: w.CardAlertText, Parts: []w.CardTextPart{w.Text(m.Note)}})
		}
		return card
	}
	if m.Limit != "" {
		line = append(line, string(m.Limit))
	}
	if m.Answered != "" {
		line = append(line, lastAnswer+string(m.Answered))
	}
	card.Items = append(card.Items, rubric, w.Mono{Text: joined(line, lineSep), Form: w.MonoLine, Variant: w.Mono11_5Dusty})
	if len(m.Folders) > 0 {
		t := w.Table{Cols: []w.TableCol{{Label: colRole}, {Label: colFolderOnSrv, Main: true}, {Label: colHowFound, Rank: w.Rank3}, {Width: w.ColMenu}}}
		for _, f := range m.Folders {
			folder := string(f.Folder)
			if folder == "" {
				folder = noCount
			}
			change := bind.Button(f.Change, "")
			change.Small = true
			t.Rows = append(t.Rows, w.TableRow{Cells: []w.TableCell{
				{Parts: []w.CellPart{w.TableFile{Icon: roleIcons[f.Role], Name: string(f.Label)}}},
				{Parts: []w.CellPart{w.Text(folder)}, Mono: true},
				{Parts: []w.CellPart{w.Text(f.How)}, Dim: true},
				{Parts: []w.CellPart{change}, Acts: true},
			}})
		}
		card.Items = append(card.Items, t)
	}
	if m.SentToggle != nil {
		// A field of its own form (the toggle's, printed after the page);
		// a check that sends that form when it changes is Typecase's task
		// 300.
		ch := w.Check{On: m.KeepsSent, Name: sentCopyName, Value: "on", Of: bind.TickID(*m.SentToggle),
			Parts: []w.CheckPart{w.Text(m.SentToggle.Label)}}
		card.Items = append(card.Items, w.CardRow{Kind: w.CardFoot, Items: []w.CardRowPart{ch}})
	}
	return card
}

// MailboxForms are the forms of the dialog's page that post with none of
// their own: the top bar's and the notices'.
func MailboxForms(p *view.MailboxPage) []w.PostForm { return forms(p.Top, p.Notices) }

// MailboxFoldersForms are the question's: the top bar's and the notices'.
func MailboxFoldersForms(p *view.MailboxFoldersPage) []w.PostForm { return forms(p.Top, p.Notices) }
