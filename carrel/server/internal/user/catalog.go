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

// Rediscovery is one confirmed re-discovery of a connection (S-6.1): what
// discovery finds now, and the paths the person was shown as appeared and as
// gone.
type Rediscovery struct {
	Found []sealed.Collection
	Add   []string
	Gone  []string
}

// ApplyRediscovery writes a re-discovery the person confirmed. Only what they
// were shown and the server still confirms changes: a path offered as new joins
// the catalog if discovery still finds it, a path offered as gone is marked
// gone if discovery still misses it. A gone entry keeps its place and its
// ticks, so the selection names it gone instead of dropping it; if its path
// comes back and is applied, it is the same entry again. Every entry discovery
// found takes the name, colour, rights and change tag the server gives now.
// The journal names the connection and the counts, not the paths.
func (s *Service) ApplyRediscovery(viewerID string, dataKey [crypt.KeyLen]byte, accountID string, rd Rediscovery) error {
	offeredAdd, offeredGone := pathSet(rd.Add), pathSet(rd.Gone)
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
		if _, ok := blob.Connection(accountID); !ok {
			return nil, ErrNotFound
		}
		found := make(map[string]sealed.Collection, len(rd.Found))
		for _, c := range rd.Found {
			c.AccountID, c.Gone = accountID, false
			found[c.Path] = c
		}
		added, gone := 0, 0
		inCatalog := map[string]bool{}
		for i := range blob.Collections {
			c := &blob.Collections[i]
			if c.AccountID != accountID {
				continue
			}
			inCatalog[c.Path] = true
			fresh, ok := found[c.Path]
			switch {
			case ok && !c.Gone:
				*c = fresh
			case ok && offeredAdd[c.Path]:
				*c = fresh
				added++
			case !ok && !c.Gone && offeredGone[c.Path]:
				c.Gone = true
				gone++
			}
		}
		for _, c := range rd.Found {
			if !inCatalog[c.Path] && offeredAdd[c.Path] {
				blob.Collections = append(blob.Collections, found[c.Path])
				added++
			}
		}
		sealedBlob, err := sealed.Seal(dataKey, blob, u.Login)
		if err != nil {
			return nil, err
		}
		u.SealedBlob = sealedBlob
		return []store.AuditEntry{{
			When: now, Who: u.Login, What: "connection.rediscover", Whom: u.Login,
			Detail: fmt.Sprintf("%s +%d -%d", accountID, added, gone),
		}}, nil
	})
}

// RevealConnectionPassword returns the stored password of one of the viewer's
// connections and journals the release (S-10.7). The unlock window lives in
// the session, so the caller checks it. When the journal line cannot be
// written, nothing is returned.
func (s *Service) RevealConnectionPassword(viewerID string, dataKey [crypt.KeyLen]byte, accountID string) (string, error) {
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
		conn, ok := blob.Connection(accountID)
		if !ok {
			return nil, ErrNotFound
		}
		password = conn.Password
		return []store.AuditEntry{{
			When: now, Who: u.Login, What: "connection.password.reveal", Whom: u.Login, Detail: accountID,
		}}, nil
	})
	if err != nil {
		return "", err
	}
	return password, nil
}

func pathSet(paths []string) map[string]bool {
	out := make(map[string]bool, len(paths))
	for _, p := range paths {
		out[p] = true
	}
	return out
}

// OwnsCollection reports whether {col} is a live catalog entry of the viewer's
// connection {account}. A collection the catalog does not hold, or holds as
// gone, answers like a foreign one (S-9.5): the catalog, not the far server,
// is the list of what this person may open.
func (s *Service) OwnsCollection(viewerID string, dataKey [crypt.KeyLen]byte, accountParam, col string) bool {
	if accountParam == "" || col == "" {
		return false
	}
	blob, ok := s.Catalog(viewerID, dataKey)
	if !ok {
		return false
	}
	c, ok := blob.Collection(accountParam, col)
	return ok && !c.Gone
}

// OwnsRecord checks {uid} under an owned collection. Whether the record
// exists is the DAV server's answer, and missing looks like foreign.
func (s *Service) OwnsRecord(viewerID string, dataKey [crypt.KeyLen]byte, accountParam, col, uid string) bool {
	return uid != "" && s.OwnsCollection(viewerID, dataKey, accountParam, col)
}
