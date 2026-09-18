package user

import (
	"errors"
	"fmt"

	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
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

// AddCollection puts a collection the person has just created on the server
// into the catalog of its connection (S-12). An entry already at that path —
// one marked gone — becomes the new one in its place. Nothing is selected, the
// same as for any collection that joins the catalog (S-6.1). The journal
// names the connection, not the address.
func (s *Service) AddCollection(viewerID string, dataKey [crypt.KeyLen]byte, accountID string, col sealed.Collection) error {
	col.AccountID, col.Gone = accountID, false
	return s.editCatalog(viewerID, dataKey, "collection.create", accountID, func(blob *sealed.Blob) error {
		if _, ok := blob.Connection(accountID); !ok {
			return ErrNotFound
		}
		for i := range blob.Collections {
			if c := &blob.Collections[i]; c.AccountID == accountID && c.Path == col.Path {
				*c = col
				return nil
			}
		}
		blob.Collections = append(blob.Collections, col)
		return nil
	})
}

// PlaceMovedCollection puts a collection made on another connection by a move
// or a copy into the catalog in one edit (S-12.3). A move carries the ticks of
// the old entry to the new one and takes the old entry out with its ticks: the
// server no longer holds it, and it is not gone by re-discovery. A copy is a
// new collection and gets no tick (S-6.1). The journal names the connections,
// not the addresses.
func (s *Service) PlaceMovedCollection(viewerID string, dataKey [crypt.KeyLen]byte, fromAccount, fromPath, toAccount string, col sealed.Collection, move bool) error {
	col.AccountID, col.Gone = toAccount, false
	what := "collection.copy"
	if move {
		what = "collection.move"
	}
	return s.editCatalog(viewerID, dataKey, what, fromAccount+" → "+toAccount, func(blob *sealed.Blob) error {
		if _, ok := blob.Connection(toAccount); !ok {
			return ErrNotFound
		}
		src := -1
		for i, c := range blob.Collections {
			if c.AccountID == fromAccount && c.Path == fromPath && !c.Gone {
				src = i
			}
		}
		if src < 0 {
			return ErrNotFound
		}
		placed := false
		for i, c := range blob.Collections {
			if c.AccountID == toAccount && c.Path == col.Path {
				blob.Collections[i], placed = col, true
			}
		}
		if !placed {
			blob.Collections = append(blob.Collections, col)
		}
		if !move {
			return nil
		}
		var ticks []sealed.Selection
		for _, t := range blob.Selected {
			if t.AccountID == fromAccount && t.Path == fromPath {
				t.AccountID, t.Path = toAccount, col.Path
			}
			ticks = append(ticks, t)
		}
		blob.Selected = ticks
		blob.Collections = append(blob.Collections[:src], blob.Collections[src+1:]...)
		return nil
	})
}

// RemoveCollection takes a collection the server has deleted out of the catalog
// with its ticks (S-12.4): it is not gone by re-discovery, it is no more. The
// journal names the connection, not the address.
func (s *Service) RemoveCollection(viewerID string, dataKey [crypt.KeyLen]byte, accountID, path string) error {
	return s.editCatalog(viewerID, dataKey, "collection.delete", accountID, func(blob *sealed.Blob) error {
		at := -1
		for i, c := range blob.Collections {
			if c.AccountID == accountID && c.Path == path {
				at = i
			}
		}
		if at < 0 {
			return ErrNotFound
		}
		blob.Collections = append(blob.Collections[:at], blob.Collections[at+1:]...)
		var ticks []sealed.Selection
		for _, t := range blob.Selected {
			if t.AccountID != accountID || t.Path != path {
				ticks = append(ticks, t)
			}
		}
		blob.Selected = ticks
		return nil
	})
}

// RenameCollection writes the label the server has accepted (S-12.2): the
// name and, when one was chosen, the colour. The path never changes here.
func (s *Service) RenameCollection(viewerID string, dataKey [crypt.KeyLen]byte, accountID, path, name, colour string) error {
	return s.editCatalog(viewerID, dataKey, "collection.rename", accountID, func(blob *sealed.Blob) error {
		for i := range blob.Collections {
			c := &blob.Collections[i]
			if c.AccountID != accountID || c.Path != path || c.Gone {
				continue
			}
			c.Name = name
			if colour != "" {
				c.Colour = colour
			}
			return nil
		}
		return ErrNotFound
	})
}

// SetSelection replaces the ticks of one kind with picks (S-14); the ticks of
// other kinds stay as they are. A pick the catalog does not hold for that kind
// is dropped. A gone entry stays ticked only if it was ticked already: a tick
// is taken off by the person, never put on what is no longer there (S-6.1).
// The journal names the kind, not the addresses.
func (s *Service) SetSelection(viewerID string, dataKey [crypt.KeyLen]byte, kind view.Kind, picks []sealed.Selection) error {
	type at struct{ account, path string }
	want := make(map[at]bool, len(picks))
	for _, p := range picks {
		want[at{p.AccountID, p.Path}] = true
	}
	return s.editCatalog(viewerID, dataKey, "sources.select", string(kind), func(blob *sealed.Blob) error {
		was := map[at]bool{}
		var out []sealed.Selection
		for _, sel := range blob.Selected {
			if sel.Kind == kind {
				was[at{sel.AccountID, sel.Path}] = true
				continue
			}
			out = append(out, sel)
		}
		cols := append([]sealed.Collection(nil), blob.Collections...)
		for _, f := range blob.Subscriptions {
			cols = append(cols, f.FeedCollection())
		}
		for _, c := range cols {
			key := at{c.AccountID, c.Path}
			if !want[key] || !c.Has(kind) || (c.Gone && !was[key]) {
				continue
			}
			if _, ok := blob.Connection(c.AccountID); !ok {
				continue
			}
			out = append(out, sealed.Selection{Kind: kind, AccountID: c.AccountID, Path: c.Path})
		}
		blob.Selected = out
		return nil
	})
}

// SetAttachmentFolder stores the folder large attachments go to (S-20). It
// must be a live file collection of the catalog; the folder under it need not
// exist yet. Nothing already written moves. The journal names the connection,
// not the address.
func (s *Service) SetAttachmentFolder(viewerID string, dataKey [crypt.KeyLen]byte, folder sealed.AttachmentFolder) error {
	return s.editCatalog(viewerID, dataKey, "attachments.folder", folder.AccountID, func(blob *sealed.Blob) error {
		col, ok := blob.Collection(folder.AccountID, folder.Path)
		if _, hasConn := blob.Connection(folder.AccountID); !hasConn || !ok || col.Gone || !col.Has(view.KindFile) {
			return ErrNotFound
		}
		blob.Attachments = &folder
		return nil
	})
}

// SetLastUsed remembers the collection a record of this kind was just created
// in, so the quick sheet opened from the shell starts there (U-6.16). It
// writes nothing to the journal: this happens on every record made, and a hint
// for a form is not an event of the account.
func (s *Service) SetLastUsed(viewerID string, dataKey [crypt.KeyLen]byte, sel sealed.Selection) error {
	return s.editBlob(viewerID, dataKey, nil, func(blob *sealed.Blob) error {
		kept := make([]sealed.Selection, 0, len(blob.LastUsed)+1)
		for _, was := range blob.LastUsed {
			if was.Kind != sel.Kind {
				kept = append(kept, was)
			}
		}
		blob.LastUsed = append(kept, sel)
		return nil
	})
}

// editCatalog opens the viewer's blob, applies edit, seals it back and
// journals what with detail. A failed edit writes nothing.
func (s *Service) editCatalog(viewerID string, dataKey [crypt.KeyLen]byte, what, detail string, edit func(*sealed.Blob) error) error {
	now := s.now()
	return s.editBlob(viewerID, dataKey, func(login string) []store.AuditEntry {
		return []store.AuditEntry{{When: now, Who: login, What: what, Whom: login, Detail: detail}}
	}, edit)
}

// editBlob opens the viewer's blob, applies edit and seals it back. journal
// may be nil, for a change that is not an event of the account.
func (s *Service) editBlob(viewerID string, dataKey [crypt.KeyLen]byte, journal func(login string) []store.AuditEntry, edit func(*sealed.Blob) error) error {
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(viewerID)
		if u == nil || u.Disabled {
			return nil, ErrNotFound
		}
		blob, err := sealed.Open(dataKey, u.SealedBlob, u.Login)
		if err != nil {
			return nil, fmt.Errorf("user: open blob: %w", err)
		}
		if err := edit(&blob); err != nil {
			return nil, err
		}
		sealedBlob, err := sealed.Seal(dataKey, blob, u.Login)
		if err != nil {
			return nil, err
		}
		u.SealedBlob = sealedBlob
		if journal == nil {
			return nil, nil
		}
		return journal(u.Login), nil
	})
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
