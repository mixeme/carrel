package user

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"

	cmail "carrel.dev/carrel/server/internal/mail"
	"carrel.dev/carrel/server/internal/store"
)

// ErrSMTP is a relay setting that cannot be used; the message names the field.
type ErrSMTP struct{ Field, Msg string }

func (e *ErrSMTP) Error() string { return e.Msg }

// SMTP is the relay as stored, with its password.
func (s *Service) SMTP() store.SMTPSettings {
	snap := s.st.Snapshot()
	return snap.Settings.SMTP
}

// SetSMTP stores the relay (S-26). An empty password keeps the stored one:
// the form never shows it, so an empty field means "unchanged". Clearing
// the host turns mail off. The journal names the relay, never the password.
func (s *Service) SetSMTP(actorID string, in store.SMTPSettings) error {
	in.Host = strings.TrimSpace(in.Host)
	in.Username = strings.TrimSpace(in.Username)
	in.From = strings.TrimSpace(in.From)
	in.FromName = strings.TrimSpace(in.FromName)
	if in.Host != "" {
		if strings.ContainsAny(in.Host, " /:@") {
			return &ErrSMTP{"host", "A host name only: no scheme, port or path."}
		}
		if in.Port < 1 || in.Port > 65535 {
			return &ErrSMTP{"port", "A port from 1 to 65535."}
		}
		switch in.Security {
		case cmail.StartTLS, cmail.TLS, cmail.None:
		default:
			return &ErrSMTP{"security", "Choose how the connection is protected."}
		}
		a, err := mail.ParseAddress(in.From)
		if err != nil || a.Name != "" || a.Address != in.From {
			return &ErrSMTP{"from", "The instance's address, like carrel@example.org."}
		}
		if strings.ContainsAny(in.FromName, "\r\n") {
			return &ErrSMTP{"fromName", "One line."}
		}
		switch in.Sender {
		case "", cmail.SenderService:
			in.Sender = ""
		case cmail.SenderLogin:
		default:
			return &ErrSMTP{"sender", "Choose the sender of people's letters."}
		}
	}
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		if in.Password == "" && in.Username != "" {
			in.Password = st.Settings.SMTP.Password
		}
		if in.Username == "" {
			in.Password = ""
		}
		st.Settings.SMTP = in
		detail := "off"
		if in.Host != "" {
			detail = fmt.Sprintf("%s:%d %s", in.Host, in.Port, in.Security)
			if in.Sender == cmail.SenderLogin {
				detail += ", people send as login@"
			}
		}
		return []store.AuditEntry{{
			When: s.now(), Who: actorLogin(st, actorID), What: "settings.smtp", Detail: detail,
		}}, nil
	})
}

// MarkInviteMailed records how the invitation's letter went and, when it did
// not, what the relay answered.
func (s *Service) MarkInviteMailed(secret string, sent bool, reply string) error {
	if secret == "" {
		return errors.New("user: no invitation")
	}
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		for i := range st.Invites {
			if st.Invites[i].Secret == secret {
				st.Invites[i].Mailed, st.Invites[i].MailReply = "failed", reply
				if sent {
					st.Invites[i].Mailed, st.Invites[i].MailReply = "sent", ""
				}
			}
		}
		return nil, nil
	})
}

// ConfirmedAddress is the confirmed address of the account named by login or
// address, or "" — a letter with a secret goes nowhere else.
func (s *Service) ConfirmedAddress(loginOrEmail string) string {
	loginOrEmail = strings.TrimSpace(loginOrEmail)
	snap := s.st.Snapshot()
	for i := range snap.Users {
		u := &snap.Users[i]
		if strings.EqualFold(u.Login, loginOrEmail) || (u.Email != "" && strings.EqualFold(u.Email, loginOrEmail)) {
			if u.EmailConfirmed {
				return u.Email
			}
			return ""
		}
	}
	return ""
}
