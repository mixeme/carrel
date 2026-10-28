package mail

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of the preview (U-1.3): its title and icon, what it is, the
// headers' names as a letter has them, what stands between a link's title
// and its address.
const (
	previewTitle = "Preview"
	previewIcon  = "eye"
	previewAside = "as the recipients get it"
	fromSlot     = "From"
	replySlot    = "Reply-To"
	toSlot       = "To"
	subjectSlot  = "Subject"
	linkDash     = " — "
	editID       = "mail.edit"
)

// PreviewTitle is the sheet's title and its page's (s-mail-14).
const PreviewTitle = previewTitle

// Preview draws the letter as the recipients get it (s-mail-14): the
// sheet's form posts to /app/mail/send, the draft the server's.
//
//   - The head: the eye, «Preview», «as the recipients get it» pushed.
//   - One card: the headers — From, Reply-To when there is one, To,
//     Subject — each its name in the slot, a name and the address after
//     it in muted mono; the text with the signature after «-- », and the
//     line of a letter nobody can answer after it; the files as tags with
//     their icons; the links, a line each, the address in the accent.
//   - The foot: Send with Enter, Edit, Cancel with Esc pushed, sending the
//     form: with text the server asks first (s-mail-15).
func Preview(p *view.MailPreviewSheet) w.Sheet {
	lines := []w.CardHeadLine{{Slot: fromSlot, Parts: headParts(p.From)}}
	if len(p.ReplyTo) > 0 {
		lines = append(lines, w.CardHeadLine{Slot: replySlot, Parts: headParts(p.ReplyTo)})
	}
	lines = append(lines, w.CardHeadLine{Slot: toSlot, Parts: headParts(p.To)}, w.CardHeadLine{Slot: subjectSlot, Parts: []w.CardTextPart{w.Text(p.Subject)}})
	text := string(p.Text)
	if p.NoReply != "" {
		text += "\n\n" + string(p.NoReply)
	}
	card := []w.CardPart{w.CardHead{Lines: lines}, w.Prose{Parts: []w.Inline{w.Text(text)}, Variant: w.Prose13Card}}
	if len(p.Files) > 0 {
		var files []w.CardRowPart
		for _, f := range p.Files {
			files = append(files, w.Tag{Label: string(f.Name), Icon: w.Icon(kindIcons[string(f.Kind)])})
		}
		card = append(card, w.CardRow{Kind: w.CardAttached, Gap: 6, Items: files})
	}
	if len(p.Links) > 0 {
		var links []w.CardTextPart
		for i, l := range p.Links {
			if i > 0 {
				links = append(links, w.Break{})
			}
			links = append(links, w.Text(string(l.Title)+linkDash), w.Span{Face: w.FaceAccent, Text: string(l.Address)})
		}
		card = append(card, w.CardText{Kind: w.CardCaption, Parts: links})
	}

	var foot []w.SheetFootPart
	var cancel w.Button
	for _, a := range p.Form.Actions {
		switch a.ID {
		case sendID:
			b := bind.Submit(a)
			if a.Href != p.Form.Action {
				b.To = a.Href
			}
			b.Busy = sending
			foot = append(foot, b, w.Kbd{Keys: enterKey})
		case editID:
			foot = append(foot, bind.Button(a, ""))
		case cancelID:
			cancel = bind.Submit(a)
			cancel.To = a.Href
		}
	}
	foot = append(foot, w.Line{Items: []w.LinePart{cancel, w.Kbd{Keys: escKey}}, Gap: w.LineGap7, Place: w.Pushed})
	return w.Sheet{
		Icon: previewIcon, Title: previewTitle,
		Head:   []w.SheetHeadPart{w.Dim{Parts: []w.Inline{w.Text(previewAside)}, Size: w.Dim11_5, Variant: w.DimWrap, Place: w.Pushed}},
		Body:   []w.SheetPart{w.Card{Items: card, Variant: w.CardFlush}},
		Foot:   foot,
		Width:  w.SheetWidth680,
		Action: p.Form.Action, Fields: bind.Hidden(p.Form),
	}
}

// headParts is a header of the letter as the card sets it (s-mail-14): a
// name, strong in From; an address, in angle brackets, in muted mono.
func headParts(r view.Runs) []w.CardTextPart {
	out := make([]w.CardTextPart, 0, len(r))
	for _, p := range r {
		switch p.Mark {
		case view.RunStrong:
			out = append(out, w.Strong(p.Text))
		case view.RunMono:
			out = append(out, w.Mono{Text: string(p.Text), Variant: w.Mono11Muted})
		default:
			out = append(out, w.Text(p.Text))
		}
	}
	return out
}
