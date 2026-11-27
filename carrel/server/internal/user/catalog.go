package user

import (
	"errors"
	"fmt"
	"slices"

	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/internal/publish"
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
	if conn.NoPassword {
		conn.Password = ""
	}
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
			// What a connection brings is ticked in its sections (the
			// owner's decision of 24.11.2026): Connect no longer leaves
			// every rail empty. A selection the person empties stays empty.
			for _, k := range c.Kinds {
				blob.Selected = append(blob.Selected, sealed.Selection{Kind: k, AccountID: id, Path: c.Path})
			}
		}
		sealedBlob, err := sealed.Seal(dataKey, blob, u.Login)
		if err != nil {
			return nil, err
		}
		u.SealedBlob = sealedBlob
		u.ConnCount = blob.ConnectionCount()
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
	// Scheduling and Addresses are what the principal said this time
	// (S-15.4); they replace what the connection held.
	Scheduling string
	Addresses  []string
}

// ApplyRediscovery writes a re-discovery the person confirmed. Only what they
// were shown and the server still confirms changes: a path offered as new joins
// the catalog if discovery still finds it, a path offered as gone is marked
// gone if discovery still misses it. A gone entry keeps its place and its
// ticks, so the selection names it gone instead of dropping it; if its path
// comes back and is applied, it is the same entry again. Every entry discovery
// found takes the name, colour, rights and change tag the server gives now,
// and the connection what its principal says of scheduling. The journal names the connection and the counts, not the paths.
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
		for i := range blob.Connections {
			if blob.Connections[i].ID == accountID {
				blob.Connections[i].Scheduling = rd.Scheduling
				blob.Connections[i].Addresses = rd.Addresses
			}
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

// Disconnected counts what went with a connection: the numbers of its
// journal line and of the page that follows.
type Disconnected struct {
	Collections int // catalog entries
	Links       int // published links revoked
	Jobs        int // backup jobs that lost a source or their destination
	Devices     int // davloom devices that lost a collection
}

// RemoveConnection takes a connection out of Carrel whole, in one edit of the
// blob and the state (review Р-9): its catalog, ticks, last-used
// places, linked duplicates, device exclusion, attachment and upload folders,
// the sources of copy jobs and its published links with their envelopes;
// davloom devices stop seeing its collections. Nothing on the server is
// touched. A copy job is never deleted silently: one whose destination was
// here keeps naming it, and its screen says the destination is gone. The
// "not duplicates" signatures stay — a group without these members is another
// group. The journal names the connection and the counts, not the address.
func (s *Service) RemoveConnection(viewerID string, dataKey [crypt.KeyLen]byte, accountID string) (Disconnected, error) {
	var res Disconnected
	now := s.now()
	err := s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		res = Disconnected{}
		u := st.UserByID(viewerID)
		if u == nil || u.Disabled {
			return nil, ErrNotFound
		}
		blob, err := sealed.Open(dataKey, u.SealedBlob, u.Login)
		if err != nil {
			return nil, fmt.Errorf("user: open blob: %w", err)
		}
		at := -1
		for i, c := range blob.Connections {
			if c.ID == accountID {
				at = i
			}
		}
		if at < 0 {
			return nil, ErrNotFound
		}
		blob.Connections = append(blob.Connections[:at], blob.Connections[at+1:]...)

		var cols []sealed.Collection
		for _, c := range blob.Collections {
			if c.AccountID == accountID {
				res.Collections++
				continue
			}
			cols = append(cols, c)
		}
		blob.Collections = cols
		blob.Selected = selectionsNotOf(blob.Selected, accountID)
		blob.LastUsed = selectionsNotOf(blob.LastUsed, accountID)
		if blob.Attachments != nil && blob.Attachments.AccountID == accountID {
			blob.Attachments = nil
		}
		if blob.UploadTo != nil && blob.UploadTo.AccountID == accountID {
			blob.UploadTo = nil
		}
		var excluded []string
		for _, id := range blob.DeviceExcluded {
			if id != accountID {
				excluded = append(excluded, id)
			}
		}
		blob.DeviceExcluded = excluded

		var groups []sealed.LinkedGroup
		for _, g := range blob.Duplicates.Linked {
			var members []sealed.RecordRef
			for _, m := range g.Members {
				if m.AccountID != accountID {
					members = append(members, m)
				}
			}
			if len(members) < 2 {
				continue // a group of one is no group (S-16)
			}
			g.Members = members
			for prop, m := range g.Prefs {
				if m.AccountID == accountID {
					delete(g.Prefs, prop)
				}
			}
			groups = append(groups, g)
		}
		blob.Duplicates.Linked = groups
		// A "not duplicates" with a member here is a group that is no more:
		// its signature names a member that cannot be loaded again.
		var ignored []sealed.IgnoredGroup
		for _, g := range blob.Duplicates.Ignored {
			if !slices.ContainsFunc(g.Members, func(m sealed.RecordRef) bool { return m.AccountID == accountID }) {
				ignored = append(ignored, g)
			}
		}
		blob.Duplicates.Ignored = ignored

		for i := range blob.BackupJobs {
			j := &blob.BackupJobs[i]
			sources := selectionsNotOf(j.Sources, accountID)
			if len(sources) != len(j.Sources) || j.DestAccount == accountID {
				res.Jobs++
			}
			j.Sources = sources
		}

		published := st.Published[:0]
		for _, l := range st.Published {
			if l.UserID == viewerID && l.AccountID == accountID {
				delete(blob.Links, publish.ID(l.Hash))
				crypt.Wipe(l.Envelope)
				res.Links++
				continue
			}
			published = append(published, l)
		}
		st.Published = published

		for i := range u.ProxyDevices {
			d := &u.ProxyDevices[i]
			var scope []store.DeviceScope
			for _, c := range d.ScopeCols {
				if c.AccountID != accountID {
					scope = append(scope, c)
				}
			}
			if len(scope) != len(d.ScopeCols) {
				res.Devices++
			}
			d.ScopeCols = scope
		}

		sealedBlob, err := sealed.Seal(dataKey, blob, u.Login)
		if err != nil {
			return nil, err
		}
		u.SealedBlob = sealedBlob
		u.ConnCount = blob.ConnectionCount()
		return []store.AuditEntry{{
			When: now, Who: u.Login, What: "connection.remove", Whom: u.Login,
			Detail: fmt.Sprintf("%s collections %d links %d jobs %d devices %d", accountID, res.Collections, res.Links, res.Jobs, res.Devices),
		}}, nil
	})
	if err != nil {
		return Disconnected{}, err
	}
	if res.Devices > 0 {
		s.devicesChanged(viewerID)
	}
	return res, nil
}

// UpdateConnection writes what discovery found at the connection's new
// address or with its new credentials: the server, login, password, the
// accepted fingerprint and the homes. The ID stays (C-4.1) and with it the
// catalog, the ticks and everything bound to them; the catalog is reconciled
// by ApplyRediscovery, as the person confirms it (S-6.1). NoPassword clears
// the stored password — the server asks none (s-profile-14); without it an
// empty password keeps the stored one. Every published link of the connection gets its
// envelope sealed again with the new credentials in the same edit, or the
// public page would go on reading with the old ones. The journal names the
// connection and the number of links, not the address.
func (s *Service) UpdateConnection(viewerID string, dataKey [crypt.KeyLen]byte, conn sealed.Connection) error {
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
		var stored *sealed.Connection
		for i := range blob.Connections {
			if blob.Connections[i].ID == conn.ID {
				stored = &blob.Connections[i]
			}
		}
		if stored == nil {
			return nil, ErrNotFound
		}
		switch {
		case conn.NoPassword:
			conn.Password = ""
		case conn.Password == "":
			conn.Password = stored.Password
		}
		conn.Feed = false
		*stored = conn

		plain, err := publish.Envelope{Server: conn.Server, Login: conn.Login, Password: conn.Password, CertFingerprint: conn.CertFingerprint}.Marshal()
		if err != nil {
			return nil, err
		}
		defer crypt.Wipe(plain)
		links := 0
		for i := range st.Published {
			l := &st.Published[i]
			if l.UserID != viewerID || l.AccountID != conn.ID {
				continue
			}
			env, err := s.st.SealLink(l.Hash, plain)
			if err != nil {
				return nil, err
			}
			crypt.Wipe(l.Envelope)
			l.Envelope = env
			links++
		}

		sealedBlob, err := sealed.Seal(dataKey, blob, u.Login)
		if err != nil {
			return nil, err
		}
		u.SealedBlob = sealedBlob
		return []store.AuditEntry{{
			When: now, Who: u.Login, What: "connection.edit", Whom: u.Login,
			Detail: fmt.Sprintf("%s links %d", conn.ID, links),
		}}, nil
	})
}

func selectionsNotOf(sels []sealed.Selection, accountID string) []sealed.Selection {
	var out []sealed.Selection
	for _, sel := range sels {
		if sel.AccountID != accountID {
			out = append(out, sel)
		}
	}
	return out
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
// server no longer holds it, and it is not gone by re-discovery. A folder that
// moved — to another storage, or renamed in its own (S-12.1) — takes the
// attachment folder and the upload folder along when they were in it. A copy
// is a new collection and gets no tick (S-6.1). The journal names the
// connections, not the addresses.
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
		for _, af := range []*sealed.AttachmentFolder{blob.Attachments, blob.UploadTo} {
			if af != nil && af.AccountID == fromAccount && af.Path == fromPath {
				af.AccountID, af.Path = toAccount, col.Path
			}
		}
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
// Nothing goes into the journal: a tick says what this person looks at,
// not what happened to the account, and every tick of the rail made an
// entry (the owner, 27.11.2026: noise in the audit log).
func (s *Service) SetSelection(viewerID string, dataKey [crypt.KeyLen]byte, kind view.Kind, picks []sealed.Selection) error {
	type at struct{ account, path string }
	want := make(map[at]bool, len(picks))
	for _, p := range picks {
		want[at{p.AccountID, p.Path}] = true
	}
	return s.editBlob(viewerID, dataKey, nil, func(blob *sealed.Blob) error {
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

// SetAttachmentNames stores how large attachments are named (S-20). No
// journal: a way of naming files is a preference of a form, like the upload
// folder, and moves nothing already written.
func (s *Service) SetAttachmentNames(viewerID string, dataKey [crypt.KeyLen]byte, names string) error {
	switch names {
	case "", sealed.AttachNamesDated, sealed.AttachNamesOriginal, sealed.AttachNamesTitled:
	default:
		return ErrNotFound
	}
	return s.editBlob(viewerID, dataKey, nil, func(blob *sealed.Blob) error {
		blob.AttachNames = names
		return nil
	})
}

// SetUploadFolder remembers where "Upload file" puts files. No journal: a
// folder chosen for the next upload is a hint of a form, not an event.
func (s *Service) SetUploadFolder(viewerID string, dataKey [crypt.KeyLen]byte, folder sealed.AttachmentFolder) error {
	return s.editBlob(viewerID, dataKey, nil, func(blob *sealed.Blob) error {
		col, ok := blob.Collection(folder.AccountID, folder.Path)
		if _, hasConn := blob.Connection(folder.AccountID); !hasConn || !ok || col.Gone || !col.Has(view.KindFile) {
			return ErrNotFound
		}
		blob.UploadTo = &folder
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
