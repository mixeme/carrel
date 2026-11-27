package user

import (
	"errors"
	"fmt"

	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/store"
)

// NewMailboxID is the {mailbox} of a person's own mailbox (carrel-mailbox.md
// M-3): issued here, like a connection's, never derived from its address.
func NewMailboxID() (string, error) {
	return newEntityID("m-")
}

// ErrNoMailbox is a {mailbox} the blob does not hold.
var ErrNoMailbox = errors.New("user: no such mailbox")

// AddMailbox puts a mailbox that signed in into the blob and returns its ID.
// It counts as a connection (S-3). The journal names the ID only: the
// address lives in the sealed blob.
func (s *Service) AddMailbox(viewerID string, dataKey [crypt.KeyLen]byte, m sealed.Mailbox) (string, error) {
	id, err := NewMailboxID()
	if err != nil {
		return "", err
	}
	m.ID = id
	now := s.now()
	err = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(viewerID)
		if u == nil || u.Disabled {
			return nil, ErrNotFound
		}
		blob, err := sealed.Open(dataKey, u.SealedBlob, u.Login)
		if err != nil {
			return nil, fmt.Errorf("user: open blob: %w", err)
		}
		if _, taken := blob.Mailbox(id); taken {
			return nil, errors.New("user: mailbox ID collision")
		}
		blob.Mailboxes = append(blob.Mailboxes, m)
		sealedBlob, err := sealed.Seal(dataKey, blob, u.Login)
		if err != nil {
			return nil, err
		}
		u.SealedBlob = sealedBlob
		u.ConnCount = blob.ConnectionCount()
		return []store.AuditEntry{{When: now, Who: u.Login, What: "mailbox.add", Whom: u.Login, Detail: id}}, nil
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// EditMailbox changes the mailbox id in the blob: its label, colour, name and
// signature, password and servers, the folders' roles, the result of a
// check. The journal names what was done (what) and the ID.
func (s *Service) EditMailbox(viewerID string, dataKey [crypt.KeyLen]byte, id, what string, edit func(*sealed.Mailbox)) error {
	return s.editCatalog(viewerID, dataKey, what, id, func(b *sealed.Blob) error {
		for i := range b.Mailboxes {
			if b.Mailboxes[i].ID == id {
				edit(&b.Mailboxes[i])
				return nil
			}
		}
		return ErrNoMailbox
	})
}

// RemoveMailbox takes the mailbox out of the blob: its password and servers
// go, the letters stay on the mail server (M-2). The person does it, also
// while mail is off on the instance: the administrator cannot open the blob.
func (s *Service) RemoveMailbox(viewerID string, dataKey [crypt.KeyLen]byte, id string) error {
	now := s.now()
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(viewerID)
		if u == nil || u.Disabled {
			return nil, ErrNotFound
		}
		blob, err := sealed.Open(dataKey, u.SealedBlob, u.Login)
		if err != nil {
			return nil, fmt.Errorf("user: open blob: %w", err)
		}
		kept := blob.Mailboxes[:0]
		found := false
		for _, m := range blob.Mailboxes {
			if m.ID == id {
				found = true
				continue
			}
			kept = append(kept, m)
		}
		if !found {
			return nil, ErrNoMailbox
		}
		blob.Mailboxes = kept
		sealedBlob, err := sealed.Seal(dataKey, blob, u.Login)
		if err != nil {
			return nil, err
		}
		u.SealedBlob = sealedBlob
		u.ConnCount = blob.ConnectionCount()
		return []store.AuditEntry{{When: now, Who: u.Login, What: "mailbox.remove", Whom: u.Login, Detail: id}}, nil
	})
}

// RevealMailboxPassword lets a mailbox's password out behind the unlock
// window, as a connection's (S-10.7); the journal says it happened.
func (s *Service) RevealMailboxPassword(viewerID string, dataKey [crypt.KeyLen]byte, id string) (string, error) {
	var password string
	now := s.now()
	err := s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(viewerID)
		if u == nil || u.Disabled {
			return nil, ErrNotFound
		}
		blob, err := sealed.Open(dataKey, u.SealedBlob, u.Login)
		if err != nil {
			return nil, fmt.Errorf("user: open blob: %w", err)
		}
		m, ok := blob.Mailbox(id)
		if !ok {
			return nil, ErrNoMailbox
		}
		password = m.Credentials.Password
		return []store.AuditEntry{{When: now, Who: u.Login, What: "mailbox.password.reveal", Whom: u.Login, Detail: id}}, nil
	})
	return password, err
}
