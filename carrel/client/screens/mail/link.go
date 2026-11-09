// Package mail is the letter of a person and its sheets: «Send this link»
// (/app/mail/link, s-mail-4), the letter (/app/mail/compose, s-mail-1…-3)
// and its attach sheets (/app/mail/attach, s-mail-5, -6, -10, -12, -13) —
// the drafts frame2go printed for them, their data become the view models'
// (U-5.1). Each stands over the page it was opened from (Under), which the
// registry draws (screens.over).
package mail

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of «Send this link» (U-1.3): the cross, the rubric of who sends,
// the lead of where replies go and what joins it to its link, a sender
// without a confirmed address, the rubric of what the letter carries.
const (
	closeWord  = "Close"
	sentAs     = "Sent as"
	repliesTo  = "Replies go to "
	sep        = " · "
	noReplies  = "No address for replies: the letter says a reply reaches nobody"
	carriesHdr = "The letter carries"
	mailIcon   = "mail"
)

// The names of the contract the screen places by (C-7.5): the letter's own
// button, «Copy link instead», Cancel.
const (
	sendID   = "mail.send"
	copyID   = "mail.link.copy"
	cancelID = "mail.cancel"
)

// LinkSheet draws «Send this link» (s-mail-4): one form that sends a letter
// of one published link, or copies it instead.
//
//   - The head: the icon, «Send this link», what the link is — its
//     collection's mark or its file's icon, its title — and the cross,
//     back where it was opened from.
//   - To and «A line from you», full width.
//   - «Sent as»: the name strong and the address monospaced; where replies
//     go, monospaced, and «change in Account»; without such an address, that
//     a reply reaches nobody.
//   - «The letter carries»: its three lines, muted.
//   - The foot: Send, «Copy link instead» to its own address, Cancel pushed.
func LinkSheet(p *view.MailLinkSheet) w.Sheet {
	subject := w.SheetSubject{Text: string(p.Subject)}
	if p.Colour != "" {
		subject.Mark = w.Mark{Colour: string(p.Colour)}
	}
	var cancel view.Action
	for _, a := range p.Form.Actions {
		if a.ID == cancelID {
			cancel = a
		}
	}
	var body []w.SheetPart
	for _, set := range p.Form.Sets {
		for _, r := range set.Rows {
			if r.Kind == view.InputHidden {
				continue
			}
			body = append(body, bind.Field(r, w.FieldW100))
		}
	}
	from := w.SheetFrom{Label: sentAs, Lines: [][]w.Inline{small(bind.Inlines(p.Sender))}}
	switch {
	case p.ReplyTo != "":
		reply := []w.Inline{w.Text(repliesTo), w.Mono{Text: string(p.ReplyTo), Variant: w.Mono11}}
		if p.Account != nil {
			reply = append(reply, w.Text(sep), bind.Link(*p.Account))
		}
		from.Lines = append(from.Lines, reply)
	default:
		from.Lines = append(from.Lines, []w.Inline{w.Text(noReplies)})
	}
	body = append(body, from)
	var carries [][]w.SecItemPart
	for _, c := range p.Carries {
		carries = append(carries, []w.SecItemPart{w.Text(c)})
	}
	body = append(body, w.Sec{Items: []w.SecPart{w.Rubric{Text: carriesHdr}, w.SecList{Items: carries, Gap: w.StackGapTight, Muted: true}}, Flush: true})

	var foot []w.SheetFootPart
	for _, a := range p.Form.Actions {
		switch a.ID {
		case sendID:
			b := bind.Submit(a)
			b.Busy = sending
			if a.Href != p.Form.Action {
				b.To = a.Href
			}
			foot = append(foot, b, w.Kbd{Keys: enterKey})
		case copyID:
			b := bind.Submit(a)
			b.To = a.Href
			foot = append(foot, b)
		case cancelID:
			foot = append(foot, w.Line{Items: []w.LinePart{bind.Button(a, ""), w.Kbd{Keys: escKey}}, Gap: w.LineGapNormal, Place: w.Pushed})
		}
	}
	return w.Sheet{
		Icon: mailIcon, Title: LinkTitle,
		Head:   []w.SheetHeadPart{subject, w.Close{Label: closeWord, Href: cancel.Href}},
		Body:   body,
		Foot:   foot,
		Width:  w.SheetWidth680,
		Action: p.Form.Action, Fields: bind.Hidden(p.Form),
	}
}

// small are a line's pieces with its monospaced ones at 11 px, as the
// addresses of «Sent as» are (s-mail-4).
func small(in []w.Inline) []w.Inline {
	for i, p := range in {
		if m, ok := p.(w.Mono); ok {
			m.Variant = w.Mono11
			in[i] = m
		}
	}
	return in
}

// LinkTitle is the sheet's title and its page's (s-mail-4): the model
// carries none.
const LinkTitle = "Send this link"
