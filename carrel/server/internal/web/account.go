package web

import (
	"errors"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

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
	// FromName and Address are its two halves, said apart on the sheet
	// (Пб17; s-mail-1: «from mix@carrel.example»)
	FromName, Address string
	ReplyTo           string
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
			ls.FromName, ls.Address = ls.Name, own.Address
			ls.From = ls.FromName + " <" + ls.Address + ">"
			return ls
		}
	}
	via := relay.FromName
	if via == "" {
		via = s.users.Settings().InstanceName()
	}
	ls.FromName, ls.Address = ls.Name+" (via "+via+")", relay.From
	ls.From = ls.FromName + " <" + ls.Address + ">"
	return ls
}

// replyNote is the foot of the letter's sheet (s-mail-1): how it is signed,
// the name in bold, and where answers go, or that there will be none.
func (ls letterSender) replyNote() view.Runs {
	if ls.ReplyTo == "" {
		return join("Signed ", strongRun(ls.Name), " · your address is not confirmed, so the letter carries no reply address and says at its foot that it cannot be answered.")
	}
	return join("Signed ", strongRun(ls.Name), " · replies go to "+ls.ReplyTo)
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

// postSettingsAccount saves the name (op=name, Save beside it) or asks to
// confirm a new address (op=email, Change beside it) — both without an op —
// or with op=confirm sends the link for the address waiting again.
func (s *Server) postSettingsAccount(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	if s.postAccountSessions(w, r, sess) {
		return
	}
	var flash accountFlash
	id := sess.AccountID()
	switch r.FormValue("op") {
	case "delete":
		// Only inside the unlock window: the password was asked a moment
		// ago (S-10.7). The last administrator stays.
		if sess.UnlockUntil(time.Now()) == nil {
			flash.notice = &view.Notice{Role: view.NoticeAlert, Title: "Confirm your password first",
				Body: "Deleting the account asks for your password: open Delete account again."}
			break
		}
		if err := s.users.DeleteUser(id, id, sess.Login()); err != nil {
			flash.notice = &view.Notice{Role: view.NoticeAlert, Title: "The account was not deleted", Body: formError(err)}
			break
		}
		s.clearSessionCookie(w, r)
		s.redirect(w, r, "/login")
		return
	case "confirm":
		// Send confirmation, Send again (s-mail-9, account-9): a fresh link
		// for the address waiting. That it went is said by the Email block;
		// only a letter that did not go is a notice.
		if to := s.users.EmailToConfirm(id); to != "" {
			flash.notice, _ = s.confirmationSent(r, sess, to)
		}
	case "email":
		// Change beside the address: the field is edited in place and Change
		// sends it — the address alone, the name untouched (account-1).
		s.changeAddress(r, sess, &flash)
	case "name":
		// Save beside Display name: the name alone, the address untouched.
		if err := s.users.SetDisplayName(id, r.FormValue("name")); err != nil {
			flash.nameErr = err.Error()
		}
	default:
		if err := s.users.SetDisplayName(id, r.FormValue("name")); err != nil {
			flash.nameErr = err.Error()
			break
		}
		s.changeAddress(r, sess, &flash)
	}
	if flash.nameErr != "" || flash.emailErr != "" || flash.notice != nil {
		s.writePage(w, r, http.StatusOK, s.accountPage(sess, flash))
		return
	}
	s.redirect(w, r, "/app/settings/account")
}

// changeAddress asks to confirm the address typed in the form, unless it is
// the one in use and confirmed or the one already waiting.
func (s *Server) changeAddress(r *http.Request, sess *session.Session, flash *accountFlash) {
	email := strings.TrimSpace(r.FormValue("email"))
	snap := s.store.Snapshot()
	u := snap.UserByID(sess.AccountID())
	if u == nil || email == "" || strings.EqualFold(email, u.Email) && (u.EmailConfirmed || u.PendingEmail == "") ||
		strings.EqualFold(email, u.PendingEmail) {
		return
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

// confirmationSent issues a fresh link for to and mails it. A letter the
// relay took is written down with its time (account-9) and needs no notice:
// the Email block says where it went. One that did not go is a notice.
func (s *Server) confirmationSent(r *http.Request, sess *session.Session, to string) (*view.Notice, error) {
	token, err := s.users.RequestEmail(sess.AccountID(), to)
	if err != nil || token == "" {
		return nil, err
	}
	if s.sendConfirmMail(r, sess.Login(), to, token) {
		_ = s.users.ConfirmMailed(token)
		return nil, nil
	}
	return &view.Notice{Role: view.NoticeAlert, Title: "The confirmation letter could not be sent",
		Body: "Mail is not available on this instance now. Ask an administrator; the address waits unconfirmed."}, nil
}

func (s *Server) accountPage(sess *session.Session, flash accountFlash) view.AccountPage {
	page := view.AccountPage{Shell: s.buildShell(sess, "Account")}
	page.Rail = s.settingsRail(sess, "/app/settings/account")
	if flash.notice != nil {
		page.Notices = append(page.Notices, *flash.notice)
	}
	snap := s.store.Snapshot()
	u := snap.UserByID(sess.AccountID())
	if u == nil {
		return page
	}
	tf := s.timeFor(sess)
	// account-1: the login and the day stand under the title, not as a row.
	page.Subtitle = text(u.Login + " · created " + tf.date(u.CreatedAt))
	// The address in use, or — invited by a link, nothing in use yet — the
	// one being confirmed: the field is never empty while an address waits.
	address := u.Email
	if address == "" {
		address = u.PendingEmail
	}
	page.Profile = view.Form{
		Action: s.route("/app/settings/account"), Method: "POST",
		Sets: []view.FieldSet{
			{Legend: "Name", Rows: []view.FormRow{
				csrfField(sess),
				{Name: "name", Label: "Display name", Kind: view.InputText, Value: text(u.DisplayName), Error: text(flash.nameErr),
					Hint: "Shown as the sender and used as the signature in letters you send. Blank means your login.",
					Extra: &view.Action{ID: "account.name.save", Label: "Save", Method: "POST", Enabled: true,
						Href: s.postQuery("/app/settings/account", url.Values{"op": {"name"}})}},
			}},
			{Legend: "Email", Rows: []view.FormRow{
				{Name: "email", Label: "Address", Kind: view.InputEmail, Value: text(address), Error: text(flash.emailErr),
					Hint: "Used for service notices, and as Reply-To on letters you send.",
					Extra: &view.Action{ID: "account.email.change", Label: "Change", Method: "POST", Enabled: true,
						Href: s.postQuery("/app/settings/account", url.Values{"op": {"email"}})}},
			}},
		},
		// Each field is saved by its own button (account-1): Save beside
		// Display name, Change beside the address. The form has none; sent
		// without a button (Enter), it saves the name and asks to confirm a
		// changed address, as both buttons would.
		Actions: []view.Action{},
	}
	page.Email = s.emailStatus(u, tf)
	page.PasswordNote = "Changing your password re-wraps your data key only; saved connections stay readable."
	page.Escrow = s.escrowStatus(sess, u, true)
	page.Actions = []view.Action{
		{ID: "account.password", Label: "Change password", Href: s.route("/app/password"), Method: "GET", Enabled: true},
	}
	page.Sessions = s.sessionRows(sess)
	if len(page.Sessions) > 1 {
		page.Actions = append(page.Actions, view.Action{ID: "account.session.others", Label: "End every other session", Method: "POST", Enabled: true,
			Href: s.postQuery("/app/settings/account", map[string][]string{"op": {"end-others"}})})
	}
	page.Danger = []view.Action{s.deleteAction(sess)}
	page.DangerNote = "Deletes the account and the saved connections. Data on the DAV servers stays: Carrel never touches it."
	return page
}

// emailStatus is the Email block under the address (account-8, account-1,
// s-mail-9, account-9). «Sent» is a letter the relay took whose link still
// lives: once it burns or is used, the block falls back to «not sent» or to
// «confirmed». An address confirmed before the day was written down
// (v0.2.60) says «confirmed» without one.
func (s *Server) emailStatus(u *store.User, tf timeFmt) view.EmailStatus {
	st := view.EmailStatus{State: view.EmailNone}
	to := s.users.EmailToConfirm(u.ID)
	var sentPhrase string
	if sentTo, at, ok := s.users.ConfirmationSent(u.ID); ok && strings.EqualFold(sentTo, to) {
		st.SentTo = text(sentTo)
		st.SentAt, sentPhrase = tf.sentAt(at)
	}
	if u.Email != "" && u.EmailConfirmed {
		st.State = view.EmailConfirmed
		label := "confirmed"
		if u.EmailConfirmedAt != nil {
			st.ConfirmedAt = view.When(tf.dayOfYear(*u.EmailConfirmedAt))
			label += " " + string(st.ConfirmedAt)
		}
		st.Badge = &view.Badge{Label: text(label), Role: view.BadgeLinked}
		if u.PendingEmail != "" {
			// The confirmed address stays in use, so replies still come
			// back: a badge, not a warning (account-1).
			st.State, st.Pending = view.EmailPending, text(u.PendingEmail)
			st.Badge = &view.Badge{Label: text("pending: " + u.PendingEmail), Role: view.BadgeNeutral}
			if st.SentTo == "" {
				// No letter with a live link went to the new address: without
				// a way to send it again the change could not be finished.
				// The frame draws the badge alone (to-typecase task 21).
				send := s.confirmAction("Send again")
				st.Message = &view.Notice{Role: view.NoticeAlert, Actions: []view.Action{send},
					Runs: join("No link has gone to ", monoRun(u.PendingEmail), ": until one is opened, ", monoRun(u.Email), " stays in use.")}
			}
		}
		return st
	}
	if to == "" {
		return st
	}
	send := s.confirmAction("Send confirmation")
	// The message says the consequence, not the status (s-mail-9).
	st.State = view.EmailUnconfirmed
	st.Message = &view.Notice{Role: view.NoticeAlert, Actions: []view.Action{send},
		Body: "Nobody can reply to letters you send: until this address is confirmed, they go out without a reply address and say so at the foot."}
	if st.SentTo != "" {
		// The address in mono, as the frame sets it (account-9; Нс10).
		st.State, send.Label = view.EmailSent, "Send again"
		st.Message = &view.Notice{Role: view.NoticeAlert, Actions: []view.Action{send},
			Runs: join("Nobody can reply to letters you send until you open the link we sent to ", monoRun(string(st.SentTo)), " "+sentPhrase+".")}
	}
	return st
}

// confirmAction sends a fresh link for the address waiting; off with a
// reason when the instance has no mail.
func (s *Server) confirmAction(label view.Text) view.Action {
	a := view.Action{ID: "account.confirm", Label: label, Method: "POST", Enabled: s.mailConfigured(),
		Href: s.postQuery("/app/settings/account", url.Values{"op": {"confirm"}})}
	if !a.Enabled {
		a.Reason = "Mail is not configured on this instance."
	}
	return a
}
