package user

import (
	"errors"
	"strings"

	"carrel.dev/carrel/server/internal/publish"
	"carrel.dev/carrel/server/internal/store"
)

// ErrLetterLimit is the day's letters spent (C-12).
var ErrLetterLimit = errors.New("You have sent as many letters today as this instance allows. The count starts again at midnight UTC.")

// LettersLeft is how many letters the person may still send today.
func (s *Service) LettersLeft(userID string, perDay int) int {
	snap := s.st.Snapshot()
	u := snap.UserByID(userID)
	if u == nil {
		return 0
	}
	if u.MailDay != s.now().UTC().Format("2006-01-02") {
		return perDay
	}
	return max(perDay-u.MailSent, 0)
}

// ReserveLetter takes one of the day's letters before sending; a letter the
// relay refused is given back with ReleaseLetter. Two sends at once cannot
// both take the last one.
func (s *Service) ReserveLetter(userID string, perDay int) error {
	day := s.now().UTC().Format("2006-01-02")
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(userID)
		if u == nil {
			return nil, ErrNotFound
		}
		if u.MailDay != day {
			u.MailDay, u.MailSent = day, 0
		}
		if u.MailSent >= perDay {
			return nil, ErrLetterLimit
		}
		u.MailSent++
		return nil, nil
	})
}

// ReleaseLetter gives back a reserved letter that did not go.
func (s *Service) ReleaseLetter(userID string) {
	day := s.now().UTC().Format("2006-01-02")
	_ = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		if u := st.UserByID(userID); u != nil && u.MailDay == day && u.MailSent > 0 {
			u.MailSent--
		}
		return nil, nil
	})
}

// LetterSent journals a letter (S-26.2): that it went, to whom, and what
// was attached by kind — never the subject, the text or the files.
func (s *Service) LetterSent(userID string, to []string, attached string) {
	_ = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(userID)
		if u == nil {
			return nil, nil
		}
		detail := "to " + strings.Join(to, ", ")
		if attached != "" {
			detail += "; " + attached
		}
		return []store.AuditEntry{{When: s.now(), Who: u.Login, What: "mail.send", Whom: u.Login, Detail: detail}}, nil
	})
}

// LinkID is the ID a link's owner sees for a link made with this token.
func LinkID(token string) string {
	hash, err := hashToken(token)
	if err != nil {
		return ""
	}
	return publish.ID(hash)
}
