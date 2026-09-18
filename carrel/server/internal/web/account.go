package web

import (
	"errors"
	"net/http"
	"net/mail"
	"strings"

	cmail "carrel.dev/carrel/server/internal/mail"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// letterSender is how a person's letter goes out (S-26.1): the name it is
// sent and signed with, the From the recipient sees, and where replies go —
// empty when the person's address is not confirmed.
type letterSender struct {
	Name      string
	FromLogin string // set when the instance sends as login@domain
	From      string // the From line as the recipient sees it
	ReplyTo   string
}

func (s *Server) senderOf(u *store.User) letterSender {
	relay := s.users.SMTP()
	ls := letterSender{Name: u.DisplayName}
	if ls.Name == "" {
		ls.Name = u.Login
	}
	if u.EmailConfirmed {
		ls.ReplyTo = u.Email
	}
	domain := relay.From[strings.LastIndex(relay.From, "@")+1:]
	if relay.Sender == cmail.SenderLogin {
		if own, err := mail.ParseAddress(u.Login + "@" + domain); err == nil && !strings.ContainsAny(u.Login, "@<> ") {
			ls.FromLogin = u.Login
			ls.From = ls.Name + " <" + own.Address + ">"
			return ls
		}
	}
	via := relay.FromName
	if via == "" {
		via = "Carrel"
	}
	ls.From = ls.Name + " (via " + via + ") <" + relay.From + ">"
	return ls
}

// replyNote says where answers go, or that there will be none.
func (ls letterSender) replyNote() view.Text {
	if ls.ReplyTo == "" {
		return "Your address is not confirmed, so the letter carries no reply address and says at its foot that it cannot be answered."
	}
	return text("Replies go to " + ls.ReplyTo + ".")
}

type accountFlash struct {
	nameErr, emailErr string
	notice            *view.Notice
}

func (s *Server) getSettingsAccount(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	s.writePage(w, r, http.StatusOK, s.accountPage(sess, accountFlash{}))
}

// postSettingsAccount saves the name and asks to confirm a new address, or
// with op=confirm sends the link for the address waiting again.
func (s *Server) postSettingsAccount(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	var flash accountFlash
	id := sess.AccountID()
	switch r.FormValue("op") {
	case "confirm":
		to := s.users.EmailToConfirm(id)
		if to == "" {
			break
		}
		flash.notice, _ = s.confirmationSent(r, sess, to)
	default:
		if err := s.users.SetDisplayName(id, r.FormValue("name")); err != nil {
			flash.nameErr = err.Error()
			break
		}
		email := strings.TrimSpace(r.FormValue("email"))
		snap := s.store.Snapshot()
		u := snap.UserByID(id)
		if u == nil || email == "" || strings.EqualFold(email, u.Email) && (u.EmailConfirmed || u.PendingEmail == "") ||
			strings.EqualFold(email, u.PendingEmail) {
			break
		}
		notice, err := s.confirmationSent(r, sess, email)
		switch {
		case errors.Is(err, user.ErrBadAddress):
			flash.emailErr = err.Error()
		case err != nil:
			flash.emailErr = string(formError(err))
		}
		flash.notice = notice
	}
	if flash.nameErr != "" || flash.emailErr != "" || flash.notice != nil {
		s.writePage(w, r, http.StatusOK, s.accountPage(sess, flash))
		return
	}
	s.redirect(w, r, "/app/settings/account")
}

// confirmationSent issues a fresh link for to and mails it; the notice says
// whether it went.
func (s *Server) confirmationSent(r *http.Request, sess *session.Session, to string) (*view.Notice, error) {
	token, err := s.users.RequestEmail(sess.AccountID(), to)
	if err != nil || token == "" {
		return nil, err
	}
	if s.sendConfirmMail(r, sess.Login(), to, token) {
		return &view.Notice{Role: view.NoticeInfo, Title: text("A confirmation link went to " + to),
			Body: "The address is used once you open the link. Until then the one before stays."}, nil
	}
	return &view.Notice{Role: view.NoticeAlert, Title: "The confirmation letter could not be sent",
		Body: "Mail is not available on this instance now. Ask an administrator; the address waits unconfirmed."}, nil
}

func (s *Server) accountPage(sess *session.Session, flash accountFlash) view.AccountPage {
	page := view.AccountPage{Shell: s.buildShell(sess, "Account")}
	if flash.notice != nil {
		page.Notices = append(page.Notices, *flash.notice)
	}
	snap := s.store.Snapshot()
	u := snap.UserByID(sess.AccountID())
	if u == nil {
		return page
	}
	tf := s.timeFor(sess)
	var status view.Text
	switch {
	case u.PendingEmail != "":
		status = text("pending: " + u.PendingEmail)
	case u.Email != "" && u.EmailConfirmed:
		status = "confirmed"
	case u.Email != "":
		status = "not confirmed"
	}
	emailHint := view.Text("Used for service notices, and as Reply-To on letters you send.")
	if status != "" {
		emailHint = status + ". " + emailHint
	}
	page.Profile = view.Form{
		Action: s.route("/app/settings/account"), Method: "POST",
		Sets: []view.FieldSet{{Legend: text("Account " + u.Login + " · created " + tf.date(u.CreatedAt)), Rows: []view.FormRow{
			csrfField(sess),
			{Name: "login", Label: "Login", Kind: view.InputText, Value: text(u.Login), ReadOnly: true},
			{Name: "name", Label: "Display name", Kind: view.InputText, Value: text(u.DisplayName), Error: text(flash.nameErr),
				Hint: "Shown as the sender and used as the signature in letters you send. Blank means your login."},
			{Name: "email", Label: "Email", Kind: view.InputEmail, Value: text(u.Email), Error: text(flash.emailErr), Hint: emailHint},
		}}},
		Actions: []view.Action{{ID: "account.save", Label: "Save", Method: "POST", Enabled: true}},
	}
	if s.users.EmailToConfirm(u.ID) != "" {
		page.Profile.Actions = append(page.Profile.Actions, view.Action{
			ID: "account.confirm", Label: "Send confirmation", Method: "POST", Enabled: s.mailConfigured(),
			Href: s.postQuery("/app/settings/account", map[string][]string{"op": {"confirm"}}),
			Reason: func() view.Text {
				if s.mailConfigured() {
					return ""
				}
				return "Mail is not configured on this instance."
			}(),
		})
	}
	if s.cfg.MailEnabled && s.mailConfigured() {
		ls := s.senderOf(u)
		page.Mail = view.Form{Sets: []view.FieldSet{{Legend: "Letters you send", Rows: []view.FormRow{
			{Name: "from", Label: "Sent as", Kind: view.InputText, Value: text(ls.From), ReadOnly: true,
				Hint: "The administrator chose the form of the address; the name is yours."},
			{Name: "replyTo", Label: "Replies go to", Kind: view.InputText, Value: text(ls.ReplyTo), ReadOnly: true,
				Hint: ls.replyNote()},
		}}}}
	}
	page.Actions = []view.Action{
		{ID: "account.password", Label: "Change password", Href: s.route("/app/password"), Method: "GET", Enabled: true},
		{ID: "account.escrow", Label: "Key escrow", Href: s.route("/app/settings/escrow"), Method: "GET", Enabled: true},
	}
	return page
}
