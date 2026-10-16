package user

import (
	"errors"
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

// CheckSMTP is every refusal of a relay as typed, by the row it stands at,
// and the relay made plain: its words trimmed, the service's sender the
// empty one (S-26). An empty host is mail off and refuses nothing; an empty
// password is the stored one, which the form never shows.
func CheckSMTP(in *store.SMTPSettings) []*ErrSMTP {
	in.Host = strings.TrimSpace(in.Host)
	in.Username = strings.TrimSpace(in.Username)
	in.From = strings.TrimSpace(in.From)
	in.FromName = strings.TrimSpace(in.FromName)
	if in.Host == "" {
		// Mail off: the port and the encryption the form shows by default
		// are no relay's.
		in.Port, in.Security = 0, ""
		return nil
	}
	var out []*ErrSMTP
	if strings.ContainsAny(in.Host, " /:@") {
		out = append(out, &ErrSMTP{"host", "A host name only: no scheme, port or path."})
	}
	if in.Port < 1 || in.Port > 65535 {
		out = append(out, &ErrSMTP{"port", "A port from 1 to 65535."})
	}
	switch in.Security {
	case cmail.StartTLS, cmail.TLS, cmail.None:
	default:
		out = append(out, &ErrSMTP{"security", "Choose how the connection is protected."})
	}
	if a, err := mail.ParseAddress(in.From); err != nil || a.Name != "" || a.Address != in.From {
		out = append(out, &ErrSMTP{"from", "The instance's address, like carrel@example.org."})
	}
	if strings.ContainsAny(in.FromName, "\r\n") {
		out = append(out, &ErrSMTP{"fromName", "One line."})
	}
	switch in.Sender {
	case "", cmail.SenderService:
		in.Sender = ""
	case cmail.SenderLogin:
	default:
		out = append(out, &ErrSMTP{"sender", "Choose the sender of people's letters."})
	}
	return out
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
