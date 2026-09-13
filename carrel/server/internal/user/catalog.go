package user

import (
	"errors"
	"fmt"

	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/store"
)

// Catalog opens the viewer's sealed blob: connections, the collection catalog,
// and the selection. False for a missing or disabled account and for a key
// that does not open the blob.
func (s *Service) Catalog(viewerID string, dataKey [crypt.KeyLen]byte) (sealed.Blob, bool) {
	snap := s.st.Snapshot()
	u := snap.UserByID(viewerID)
	if u == nil || u.Disabled {
		return sealed.Blob{}, false
	}
	blob, err := sealed.Open(dataKey, u.SealedBlob, u.Login)
	if err != nil {
		return sealed.Blob{}, false
	}
	return blob, true
}

// AddConnection appends a discovered connection and its collections to the
// catalog and returns the new connection ID (C-4.1: issued here, never derived
// from the server or the login). Nothing is selected: a new tick is not set by
// itself (S-6.1). The journal names the connection ID only — the server
// address lives in the sealed blob, and the plain journal must not reveal it.
func (s *Service) AddConnection(viewerID string, dataKey [crypt.KeyLen]byte, conn sealed.Connection, cols []sealed.Collection) (string, error) {
	id, err := NewAccountID()
	if err != nil {
		return "", err
	}
	conn.ID = id
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
		if _, taken := blob.Connection(id); taken {
			return nil, errors.New("user: connection ID collision")
		}
		blob.Connections = append(blob.Connections, conn)
		for _, c := range cols {
			c.AccountID = id
			blob.Collections = append(blob.Collections, c)
		}
		sealedBlob, err := sealed.Seal(dataKey, blob, u.Login)
		if err != nil {
			return nil, err
		}
		u.SealedBlob = sealedBlob
		u.ConnCount = len(blob.Connections)
		return []store.AuditEntry{{
			When: now, Who: u.Login, What: "connection.add", Whom: u.Login, Detail: id,
		}}, nil
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// OwnsCollection reports whether {col} is a catalog entry of the viewer's
// connection {account}. A collection the catalog does not hold answers like a
// foreign one (S-9.5): the catalog, not the far server, is the list of what
// this person may open.
func (s *Service) OwnsCollection(viewerID string, dataKey [crypt.KeyLen]byte, accountParam, col string) bool {
	if accountParam == "" || col == "" {
		return false
	}
	blob, ok := s.Catalog(viewerID, dataKey)
	if !ok {
		return false
	}
	_, ok = blob.Collection(accountParam, col)
	return ok
}

// OwnsRecord checks {uid} under an owned collection. Whether the record
// exists is the DAV server's answer, and missing looks like foreign.
func (s *Service) OwnsRecord(viewerID string, dataKey [crypt.KeyLen]byte, accountParam, col, uid string) bool {
	return uid != "" && s.OwnsCollection(viewerID, dataKey, accountParam, col)
}
