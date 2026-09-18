package web

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"carrel.dev/carrel/server/internal/conf"
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

// mailConfigured reports whether the instance can send at all.
func (s *Server) mailConfigured() bool {
	return s.users.SMTP().Configured()
}

// sendSystemMail sends one of the instance's own letters (S-26): from the
// instance's address, with no person's name and no Reply-To. The relay's
// answer goes to the log without the address: it is somebody's mailbox.
func (s *Server) sendSystemMail(ctx context.Context, to, subject, body string) (string, error) {
	relay := relayOf(s.users.SMTP())
	data, err := cmail.Build(relay, cmail.Message{To: []string{to}, Subject: subject, Text: body}, time.Now())
	if err == nil {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		var reply string
		reply, err = cmail.Send(ctx, relay, []string{to}, data)
		if err == nil {
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
	body := s.letter(r, fmt.Sprintf("You are invited to Carrel at %s, as %s.\n\n"+
		"Open this link to choose your password:\n%s\n\n"+
		"The link works until %s and only once. If you did not expect this letter, do nothing.",
		s.publicOrigin(r), login, s.absURL(r, "/invite/"+token), time.Now().Add(conf.InviteTTL).UTC().Format("2006-01-02 15:04 UTC")))
	_, err := s.sendSystemMail(r.Context(), email, "An invitation to Carrel", body)
	_ = s.users.MarkInviteMailed(token, err == nil)
	return err == nil
}
