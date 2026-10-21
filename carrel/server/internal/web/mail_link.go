package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	cmail "carrel.dev/carrel/server/internal/mail"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// «Send this link» (s-mail-4; the owner's decision of 21.10.2026). A link
// given out goes by a letter of its own: To, a line from the person, and
// the link — nothing of what it opens. The letter being written is not
// touched: it waits as it was. Send of a row of Published links and «Send
// by letter» of a link just made open this sheet.

// publishedLink is one of the person's live links, by its ID.
func (s *Server) publishedLink(sess *session.Session, id string) (user.PublishedLink, bool) {
	if id == "" || s.linkToken(sess, id) == "" {
		return user.PublishedLink{}, false
	}
	for _, l := range s.users.PublishedLinks(sess.AccountID()) {
		if l.ID == id {
			return l, true
		}
	}
	return user.PublishedLink{}, false
}

// mailLinkBack is the page the sheet stands over and Cancel goes to:
// back, else Published links. The ready sheet of a new link is the answer
// of a form and no page to go back to, so a Referer is not taken.
func (s *Server) mailLinkBack(r *http.Request) string {
	if p := s.sheetBack(r, r.FormValue("back"), false); p != "" {
		return p
	}
	return "/app/files/published"
}

func (s *Server) getMailLink(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	l, ok := s.publishedLink(sess, r.URL.Query().Get("id"))
	if !s.canSendLetters(sess) || !ok {
		s.notFound(w, r)
		return
	}
	s.writeMailSheet(w, r, s.mailLinkSheet(r, sess, l, url.Values{}, ""))
}

// mailLinkSheet is the sheet of one link, with what was typed and why it did
// not go, when it did not.
func (s *Server) mailLinkSheet(r *http.Request, sess *session.Session, l user.PublishedLink, typed url.Values, why string) view.MailLinkSheet {
	sheet := view.MailLinkSheet{Subject: text(publishedTitle(l.Target))}
	kind, icon := publishedKind(l.Target)
	sheet.Icon = icon
	if col, ok := s.catalogOf(sess).Collection(l.Target.AccountID, l.Target.Col); ok && icon == "" {
		sheet.Colour = catalogColour(col)
	}
	if u := s.letterUser(sess); u != nil {
		ls := s.senderOf(u)
		sheet.From, sheet.FromName, sheet.ReplyTo = text(ls.Address), text(ls.FromName), text(ls.ReplyTo)
	}
	// Three things people mix up (s-mail-4): the letter carries no content,
	// a revoke reaches the letter already sent, the link's term is the
	// letter's.
	expires := "The link has no expiry: it works until it is revoked"
	if l.ExpiresAt != nil {
		expires = "It expires on " + s.timeFor(sess).dayOfYear(*l.ExpiresAt) + ", as the link does"
	}
	sheet.Carries = []view.Text{
		text("The link and your line — nothing of the " + kind + " itself"),
		"Revoking the link closes it for this recipient too",
		text(expires),
	}
	back := s.mailLinkBack(r)
	q := url.Values{"id": {l.ID}}
	if back != "/app/files/published" {
		q.Set("back", back)
	}
	sendTo := s.postQuery("/app/mail/send", q)
	q.Set("op", "link")
	to := view.FormRow{Name: "to", Label: "To", Kind: view.InputText, Value: text(typed.Get("to")), Focus: true, Error: text(why)}
	sheet.Form = s.plainForm("/app/mail/send", "POST", csrfField(sess), to,
		view.FormRow{Name: "line", Label: "A line from you", Kind: view.InputText, Value: text(typed.Get("line")), Placeholder: "optional"})
	sheet.Form.Action, sheet.Form.Target = sendTo, render.TargetSheet
	sheet.Actions = []view.Action{
		{ID: "mail.send", Label: "Send", Icon: "i-mail", Method: "POST", Primary: true, Enabled: true, Href: sendTo, Target: render.TargetSheet},
		{ID: "mail.link.copy", Label: "Copy link instead", Method: "POST", Enabled: true, Href: s.postQuery("/app/mail/send", q), Target: render.TargetSheet},
		{ID: "mail.cancel", Label: "Cancel", Method: "GET", Enabled: true, Href: s.route(back)},
	}
	sheet.Form.Actions = sheet.Actions
	sheet.Under = s.underOf(r, back, "/app/files/published")
	return sheet
}

// postMailSend is the form of a letter. With id it is «Send this link»:
// the letter of that link goes, or, with op=link («Copy link instead»),
// nothing goes and Published links shows the link whole to copy. Without
// id it is the letter being written (postMailCompose).
func (s *Server) postMailSend(w http.ResponseWriter, r *http.Request) {
	id := r.FormValue("id")
	if id == "" {
		s.postMailCompose(w, r)
		return
	}
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	l, ok := s.publishedLink(sess, id)
	if !s.canSendLetters(sess) || !ok {
		s.notFound(w, r)
		return
	}
	if r.FormValue("op") == "link" {
		s.redirect(w, r, s.mailLinkBack(r))
		return
	}
	n, err := s.sendLinkLetter(r, sess, l)
	if err != nil {
		s.writeMailSheet(w, r, s.mailLinkSheet(r, sess, l, r.PostForm, err.Error()))
		return
	}
	back, _ := url.Parse(s.mailLinkBack(r))
	q := back.Query()
	q.Set("sent", strconv.Itoa(n))
	back.RawQuery = q.Encode()
	s.redirect(w, r, back.String())
}

// sendLinkLetter sends the letter of one link: the line typed, the link
// under the person's name. It counts against the letters of the day as any
// letter does, and the link is the same one — its term, its revoke.
func (s *Server) sendLinkLetter(r *http.Request, sess *session.Session, l user.PublishedLink) (int, error) {
	u := s.letterUser(sess)
	if u == nil {
		return 0, errors.New("no account")
	}
	to, err := cmail.Addresses(splitRecipients([]string{r.FormValue("to")}))
	switch {
	case err != nil:
		return 0, errors.New("A recipient is not an address: " + strings.TrimPrefix(err.Error(), "mail: "))
	case len(to) == 0:
		return 0, errors.New("Add a recipient.")
	case len(to) > s.mailPolicy().Recipients:
		return 0, fmt.Errorf("A letter goes to %d recipients at most.", s.mailPolicy().Recipients)
	}
	token := s.linkToken(sess, l.ID)
	if token == "" {
		return 0, errors.New("The link was revoked or has expired.")
	}
	if err := s.users.ReserveLetter(u.ID, s.mailPolicy().PerDay); err != nil {
		return 0, err
	}
	title := publishedTitle(l.Target)
	links := []string{title + " — " + s.absURL(r, "/p/"+token)}
	if err := s.relayLetter(r, s.senderOf(u), to, title, r.FormValue("line"), links, nil); err != nil {
		s.users.ReleaseLetter(u.ID)
		return 0, err
	}
	s.users.LetterSent(u.ID, to, l.Target.Kind+" link ×1")
	return len(to), nil
}
