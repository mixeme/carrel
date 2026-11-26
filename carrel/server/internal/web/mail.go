package web

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	cmail "carrel.dev/carrel/server/internal/mail"
	"carrel.dev/carrel/server/internal/store"
)

// relay is the instance's relay as internal/mail wants it.
func relayOf(st store.SMTPSettings) cmail.Relay {
	return cmail.Relay{
		Host: st.Host, Port: st.Port, Security: st.Security,
		Username: st.Username, Password: st.Password,
		From: st.From, FromName: st.FromName,
	}
}

// relay is the instance's relay; an empty From name is the instance's name.
func (s *Server) relay() cmail.Relay {
	r := relayOf(s.users.SMTP())
	if r.FromName == "" {
		r.FromName = s.users.Settings().InstanceName()
	}
	return r
}

// mailPolicy is what people's letters may be (C-12): the administrator's
// settings, and where one is not set, the CARREL_MAIL_* of the start.
type mailPolicy struct {
	People             bool
	Attach, Letter     int64
	Recipients, PerDay int
}

// mailboxesOn says whether people may connect and read their own mailboxes
// (carrel-mailbox.md M-2): the administrator's choice on /admin/settings,
// else CARREL_MAILBOX_ENABLED of the start. Off, there is no mail of
// mailboxes anywhere — one check at the entry of the section, not one per
// screen.
func (s *Server) mailboxesOn() bool {
	if set := s.users.Settings(); set.Mailboxes != nil {
		return *set.Mailboxes
	}
	return s.cfg.MailboxEnabled
}

func (s *Server) mailPolicy() mailPolicy {
	set := s.users.Settings()
	p := mailPolicy{People: s.cfg.MailEnabled, Attach: s.cfg.MailMaxAttachBytes, Letter: s.cfg.MailMaxMessageBytes,
		Recipients: s.cfg.MailMaxRecipients, PerDay: s.cfg.MailMaxPerDay}
	if set.PeopleMail != nil {
		p.People = *set.PeopleMail
	}
	if set.MailMaxAttachMB > 0 {
		p.Attach = int64(set.MailMaxAttachMB) << 20
	}
	if set.MailMaxMessageMB > 0 {
		p.Letter = int64(set.MailMaxMessageMB) << 20
	}
	if set.MailMaxRecipients > 0 {
		p.Recipients = set.MailMaxRecipients
	}
	if set.MailMaxPerDay > 0 {
		p.PerDay = set.MailMaxPerDay
	}
	return p
}

// mailConfigured reports whether the instance can send at all: a relay,
// and the public address — every letter carries a link to the instance, and
// without the address that link would come from the Host a request names,
// which anybody may name (audit №4, А4-6; the owner's decision of
// 25.11.2026: without it no letter goes at all).
func (s *Server) mailConfigured() bool {
	return s.cfg.PublicURL != "" && s.users.SMTP().Configured()
}

// noPublicAddress is why nothing is sent on an instance without its public
// address (А4-6).
const noPublicAddress = "No letter goes without the public address: set CARREL_PUBLIC_URL."

// errNoPublicAddress refuses a letter on such an instance.
var errNoPublicAddress = errors.New(noPublicAddress)

// mailOffReason is why mail is off: why, the relay's own reason, while there
// is no relay; with one, the public address missing (А4-6).
func (s *Server) mailOffReason(why string) string {
	if s.users.SMTP().Configured() && s.cfg.PublicURL == "" {
		return noPublicAddress
	}
	return why
}

// sendSystemMail sends one of the instance's own letters (S-26): from the
// instance's address, with no person's name and no Reply-To. The relay's
// answer goes to the log without the address: it is somebody's mailbox.
func (s *Server) sendSystemMail(ctx context.Context, to, subject, body string) (string, error) {
	return s.sendWith(ctx, s.relay(), to, subject, body)
}

// sendWith sends one of the instance's own letters through relay: the stored
// one, or the one typed in the settings for the test letter (Ад37).
func (s *Server) sendWith(ctx context.Context, relay cmail.Relay, to, subject, body string) (string, error) {
	if s.cfg.PublicURL == "" {
		return "", errNoPublicAddress
	}
	data, err := cmail.Build(relay, cmail.Message{To: []string{to}, Subject: subject, Text: body}, time.Now())
	if err == nil {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		var reply string
		reply, err = cmail.Send(ctx, relay, []string{to}, data)
		if err == nil {
			s.markSent()
			return reply, nil
		}
	}
	slog.Warn("mail: a letter was not sent", "subject", subject, "error", err)
	return "", err
}

// absURL is path as a link that works from a letter.
func (s *Server) absURL(r *http.Request, path string) string {
	return s.publicOrigin(r) + s.route(path)
}

const letterFooter = "\n\n-- \nThis letter was sent by Carrel at %s. Do not reply to it: nobody reads this address.\n"

func (s *Server) letter(r *http.Request, body string) string {
	return body + fmt.Sprintf(letterFooter, s.publicOrigin(r))
}

// sendForgotWipeMail sends the wipe link to the account's confirmed address,
// and to nothing else; false when there is none or the relay failed.
func (s *Server) sendForgotWipeMail(r *http.Request, loginOrEmail, token string) bool {
	to := s.users.ConfirmedAddress(loginOrEmail)
	if to == "" || !s.mailConfigured() {
		return false
	}
	body := s.letter(r, "Somebody asked to reset the password of your Carrel account.\n\n"+
		"Carrel cannot recover your encrypted data: a reset removes the saved connections and their passwords, "+
		"and you set up the connections again. Your contacts, calendars and files stay on their servers.\n\n"+
		"To go on, open this link. It works for a limited time and only once:\n"+
		s.absURL(r, "/forgot/"+token)+"\n\n"+
		"If it was not you, do nothing: without the link nothing changes.")
	_, err := s.sendSystemMail(r.Context(), to, "Resetting your Carrel password", body)
	return err == nil
}

// sendEscrowRecoveryMail tells the person their deposited key was opened
// (S-10.5: always, whatever the administrator wants). The temporary
// password is not in it: that goes hand to hand.
func (s *Server) sendEscrowRecoveryMail(r *http.Request, login, actor string) bool {
	to := s.users.ConfirmedAddress(login)
	if to == "" || !s.mailConfigured() {
		return false
	}
	body := s.letter(r, fmt.Sprintf("The administrator %s opened the copy of your data key deposited on this instance, "+
		"and set a temporary password for your account %s.\n\n"+
		"They give you that password in person. When you sign in with it, Carrel asks you to choose a new one. "+
		"Your connections are intact.\n\n"+
		"Every such recovery is written to the instance's journal. If you did not ask for it, talk to the administrator.", actor, login))
	_, err := s.sendSystemMail(r.Context(), to, "Your Carrel account was recovered by an administrator", body)
	return err == nil
}

// sendConfirmMail sends the link that confirms a new address.
func (s *Server) sendConfirmMail(r *http.Request, login, email, token string) bool {
	if email == "" || !s.mailConfigured() {
		return false
	}
	body := s.letter(r, fmt.Sprintf("This address was given for the Carrel account %s.\n\n"+
		"To confirm it, open this link:\n%s\n\n"+
		"If you did not create the account, do nothing: an unconfirmed account cannot be used.", login, s.absURL(r, "/confirm-email/"+token)))
	_, err := s.sendSystemMail(r.Context(), email, "Confirm your address for Carrel", body)
	return err == nil
}

// sendInviteMail sends an invitation; the result is kept with the invitation
// for the administrator's list.
func (s *Server) sendInviteMail(r *http.Request, login, email, token string) bool {
	if email == "" || !s.mailConfigured() {
		return false
	}
	// An invitation of s-admin-1 names no login: the person chooses it.
	invited := fmt.Sprintf("You are invited to %s at %s.\n\nOpen this link to choose your login and password:",
		s.users.Settings().InstanceName(), s.publicOrigin(r))
	if login != "" {
		invited = fmt.Sprintf("You are invited to %s at %s, as %s.\n\nOpen this link to choose your password:",
			s.users.Settings().InstanceName(), s.publicOrigin(r), login)
	}
	body := s.letter(r, fmt.Sprintf("%s\n%s\n\n"+
		"The link works until %s and only once. If you did not expect this letter, do nothing.",
		invited, s.absURL(r, "/invite/"+token), time.Now().Add(s.users.InviteTTL()).UTC().Format("2006-01-02 15:04 UTC")))
	_, err := s.sendSystemMail(r.Context(), email, "An invitation to Carrel", body)
	_ = s.users.MarkInviteMailed(token, err == nil, cmail.Reply(err))
	return err == nil
}
