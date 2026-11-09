package user

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"slices"
	"sync"
	"time"

	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/internal/publish"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/store"
)

// PublishTarget is the one thing a link opens (S-25).
type PublishTarget struct {
	AccountID string
	Kind      string
	Col       string
	UID       string
	Path      string
	Folder    bool
	Title     string
	// From and To are the days a published calendar shows (2006-01-02, both
	// in it); empty is the whole calendar. NoDownload takes the .ics away,
	// HideDescriptions the text of the events (S-25.1).
	From, To         string
	NoDownload       bool
	HideDescriptions bool
	// Shows is what of a contact the visitor sees beside its name and
	// organisation: "phones", "emails", "photo", "note" (S-25.1); of a
	// calendar, an event, a task list and a task — "time", always, then
	// "title", "location", "description", "attendees", "attachments",
	// "tags" (Б39). A link of a calendar kind without "time" was made before
	// that rule and shows what it did then.
	Shows []string
	// HideOrganisation leaves the name alone: the organisation and the
	// title of a contact are not shown.
	HideOrganisation bool
	// Colour is the collection's colour (#rrggbb) as it stood: the rows of
	// the page wear it (s-publish-6, -7).
	Colour string
	// Attach is the attachment of the record UID the link opens, its
	// position among the ATTACH values plus one (Пб3); 0 — the record.
	// AttachSize is its size as published.
	Attach     int
	AttachSize int64
	// Parent is the link this one was made with (an attachment of a
	// published note); Children is how many were made with this one.
	Parent   string
	Children int
}

// Child is a link made together with another, to a thing of another place
// maybe: an attachment of a note lying in a file collection is read with
// that collection's connection.
type Child struct {
	Conn   sealed.Connection
	Target PublishTarget
}

// ChildToken is the secret of the n-th link made with the link parent
// (Пб3): derived from it, so the page the parent opens can point at its
// attachments without any secret being kept. Who has not the parent's
// secret cannot work it out; who has it is given the attachments anyway.
func ChildToken(parent string, n int) string {
	raw, err := base64.RawURLEncoding.DecodeString(parent)
	if err != nil {
		return ""
	}
	m := hmac.New(sha256.New, raw)
	fmt.Fprintf(m, "carrel published attachment %d", n)
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// PublishedLink is a link as its owner's screens show it: no secret.
type PublishedLink struct {
	ID        string
	Target    PublishTarget
	CreatedAt time.Time
	ExpiresAt *time.Time
	LastSeen  *time.Time
	Opens     int // how many times it was opened: stored, and since in memory
	Gone      bool
	// Missing is how many records the last guest's read left out, not valid
	// MissingFormat (s-publish-40)
	Missing       int
	MissingFormat string
}

// OpenLink is a live link opened for a reader without a session.
type OpenLink struct {
	ID     string // publish.ID of the link: its rate bucket, never its secret
	Target PublishTarget
	Conn   sealed.Connection
	// Expires is when the link stops working; nil — never (the page's foot
	// says so, Пс2).
	Expires *time.Time
}

// lastSeenEvery keeps a busy link from rewriting the state on every view.
const lastSeenEvery = time.Hour

// linkOpens counts the opens of each link since its count was last written
// with LastSeen, by the link's ID. Written with LastSeen, not on every open:
// a busy link does not rewrite the state each time. What a restart loses is
// at most an hour of opens of a link.
type linkOpens struct {
	mu sync.Mutex
	n  map[string]int
}

func (o *linkOpens) add(id string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.n == nil {
		o.n = map[string]int{}
	}
	o.n[id]++
}

// take returns the opens not yet written and forgets them.
func (o *linkOpens) take(id string) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	n := o.n[id]
	delete(o.n, id)
	return n
}

func (o *linkOpens) peek(id string) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.n[id]
}

// Publish makes a link to target, living until expires (nil: no expiry,
// chosen explicitly). conn is the target's connection as the owner's blob
// holds it: its address and credentials go into the link's envelope, sealed
// under the server key. The secret is returned once and never kept. The
// journal names the kind and the connection.
func (s *Service) Publish(viewerID string, dataKey [crypt.KeyLen]byte, conn sealed.Connection, target PublishTarget, expires *time.Time) (string, error) {
	return s.PublishWith(viewerID, dataKey, conn, target, expires, nil)
}

// PublishWith is Publish of a link with the links made together with it
// (Пб3: the attachments of a note, «Publish them too»), all in one write:
// each child expires with the link, is revoked with it, and has its secret
// derived from the link's (ChildToken). The journal names each.
func (s *Service) PublishWith(viewerID string, dataKey [crypt.KeyLen]byte, conn sealed.Connection, target PublishTarget, expires *time.Time, children []Child) (string, error) {
	raw, hash, err := newSecret()
	if err != nil {
		return "", err
	}
	now := s.now()
	target.Parent, target.Children = "", len(children)
	links := []store.PublishedLink{}
	add := func(hash []byte, conn sealed.Connection, t PublishTarget) error {
		plain, err := publish.Envelope{Server: conn.Server, Login: conn.Login, Password: conn.Password, CertFingerprint: conn.CertFingerprint}.Marshal()
		if err != nil {
			return err
		}
		env, err := s.st.SealLink(hash, plain)
		crypt.Wipe(plain)
		if err != nil {
			return err
		}
		links = append(links, store.PublishedLink{
			Hash: hash, AccountID: t.AccountID, Kind: t.Kind, Col: t.Col,
			UID: t.UID, Path: t.Path, Folder: t.Folder, Title: t.Title,
			From: t.From, To: t.To, NoDownload: t.NoDownload, HideDescriptions: t.HideDescriptions,
			Shows: slices.Clone(t.Shows), HideOrganisation: t.HideOrganisation, Colour: t.Colour,
			Attach: t.Attach, AttachSize: t.AttachSize, Parent: t.Parent, Children: t.Children,
			CreatedAt: now, ExpiresAt: expires, Envelope: env,
		})
		return nil
	}
	if err := add(hash, conn, target); err != nil {
		return "", err
	}
	for n, c := range children {
		childHash, err := hashToken(ChildToken(raw, n))
		if err != nil {
			return "", err
		}
		c.Target.Parent, c.Target.Children = publish.ID(hash), 0
		if err := add(childHash, c.Conn, c.Target); err != nil {
			return "", err
		}
	}
	err = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(viewerID)
		if u == nil || u.Disabled {
			return nil, ErrNotFound
		}
		prunePublished(st, now)
		var audit []store.AuditEntry
		for _, l := range links {
			l.UserID = u.ID
			st.Published = append(st.Published, l)
			audit = append(audit, store.AuditEntry{When: now, Who: u.Login, What: "publish.create", Whom: u.Login, Detail: l.Kind + " " + l.AccountID})
		}
		return audit, nil
	})
	if err != nil {
		return "", err
	}
	// The link itself goes into the person's blob; links revoked or expired
	// since are dropped from it on the way.
	live := map[string]bool{}
	for _, l := range s.PublishedLinks(viewerID) {
		live[l.ID] = true
	}
	id := publish.ID(hash)
	if err := s.editBlob(viewerID, dataKey, nil, func(b *sealed.Blob) error {
		for k := range b.Links {
			if !live[k] {
				delete(b.Links, k)
			}
		}
		if b.Links == nil {
			b.Links = map[string]string{}
		}
		b.Links[id] = raw
		return nil
	}); err != nil {
		return "", err
	}
	return raw, nil
}

// OpenPublished opens a link for a reader: the target and the credentials
// to read it with. Unknown, revoked, expired and a disabled owner are one
// answer; every live link is compared, in constant time each, so a miss
// costs what a hit costs.
func (s *Service) OpenPublished(token string) (OpenLink, bool) {
	return s.openPublished(token, true)
}

// openPublished opens a link; count says whether this is an open of it —
// a reader's request — or only a check that it lives.
func (s *Service) openPublished(token string, count bool) (OpenLink, bool) {
	hash, err := hashToken(token)
	if err != nil {
		return OpenLink{}, false
	}
	snap := s.st.Snapshot()
	now := s.now()
	var found *store.PublishedLink
	for i := range snap.Published {
		l := &snap.Published[i]
		if verifyHash(l.Hash, hash) && found == nil {
			found = l
		}
	}
	if found == nil || publish.Expired(found.ExpiresAt, now) {
		return OpenLink{}, false
	}
	if u := snap.UserByID(found.UserID); u == nil || u.Disabled {
		return OpenLink{}, false
	}
	plain, err := s.st.OpenLink(found.Hash, found.Envelope)
	if err != nil {
		return OpenLink{}, false
	}
	env, err := publish.ParseEnvelope(plain)
	crypt.Wipe(plain)
	if err != nil {
		return OpenLink{}, false
	}
	if !count {
		return OpenLink{ID: publish.ID(found.Hash), Target: targetOf(*found), Expires: found.ExpiresAt}, true
	}
	id := publish.ID(found.Hash)
	s.opens.add(id)
	if found.LastSeen == nil || now.Sub(*found.LastSeen) >= lastSeenEvery {
		opens := s.opens.take(id)
		_ = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
			for i := range st.Published {
				if verifyHash(st.Published[i].Hash, hash) {
					st.Published[i].LastSeen = &now
					st.Published[i].Opens += opens
				}
			}
			return nil, nil
		})
	}
	return OpenLink{
		ID:      publish.ID(found.Hash),
		Target:  targetOf(*found),
		Expires: found.ExpiresAt,
		Conn:    sealed.Connection{ID: found.AccountID, Server: env.Server, Login: env.Login, Password: env.Password, CertFingerprint: env.CertFingerprint},
	}, true
}

// LookPublished is the target of a live link without opening it: no open
// is counted and no credentials come out (the note's page naming the
// attachments published with it, Пб3).
func (s *Service) LookPublished(token string) (OpenLink, bool) {
	return s.openPublished(token, false)
}

// PublicLinkValid reports whether a token opens a live link.
func (s *Service) PublicLinkValid(_ context.Context, token string) bool {
	_, ok := s.openPublished(token, false)
	return ok
}

// PublishedLinks are the viewer's live links, newest first.
func (s *Service) PublishedLinks(viewerID string) []PublishedLink {
	snap := s.st.Snapshot()
	now := s.now()
	var out []PublishedLink
	for i := len(snap.Published) - 1; i >= 0; i-- {
		l := snap.Published[i]
		// A link made with another is that one's (Пб3): not a row.
		if l.UserID != viewerID || publish.Expired(l.ExpiresAt, now) || l.Parent != "" {
			continue
		}
		id := publish.ID(l.Hash)
		out = append(out, PublishedLink{ID: id, Target: targetOf(l), CreatedAt: l.CreatedAt, ExpiresAt: l.ExpiresAt, LastSeen: l.LastSeen,
			Opens: l.Opens + s.opens.peek(id), Gone: l.Gone, Missing: l.Missing, MissingFormat: l.MissingFormat})
	}
	return out
}

// RevokePublished deletes one of the viewer's links, envelope and all: a
// revoked link is a link that never was.
func (s *Service) RevokePublished(viewerID, id string) error {
	now := s.now()
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(viewerID)
		if u == nil {
			return nil, ErrNotFound
		}
		at := -1
		for i, l := range st.Published {
			if l.UserID == viewerID && publish.ID(l.Hash) == id {
				at = i
			}
		}
		if at < 0 {
			return nil, ErrNotFound
		}
		gone := st.Published[at]
		// The links made with it go with it (Пб3).
		kept := st.Published[:0]
		for i, l := range st.Published {
			if i == at || l.UserID == viewerID && l.Parent == id {
				crypt.Wipe(l.Envelope)
				continue
			}
			kept = append(kept, l)
		}
		st.Published = kept
		prunePublished(st, now)
		return []store.AuditEntry{{
			When: now, Who: u.Login, What: "publish.revoke", Whom: u.Login, Detail: gone.Kind + " " + gone.AccountID,
		}}, nil
	})
}

// prunePublished drops expired links with their envelopes.
func prunePublished(st *store.State, now time.Time) {
	kept := st.Published[:0]
	for _, l := range st.Published {
		if publish.Expired(l.ExpiresAt, now) {
			crypt.Wipe(l.Envelope)
			continue
		}
		kept = append(kept, l)
	}
	st.Published = kept
}

func targetOf(l store.PublishedLink) PublishTarget {
	return PublishTarget{AccountID: l.AccountID, Kind: l.Kind, Col: l.Col, UID: l.UID, Path: l.Path, Folder: l.Folder,
		Title: l.Title, From: l.From, To: l.To, NoDownload: l.NoDownload, HideDescriptions: l.HideDescriptions,
		Shows: slices.Clone(l.Shows), HideOrganisation: l.HideOrganisation, Colour: l.Colour,
		Attach: l.Attach, AttachSize: l.AttachSize, Parent: l.Parent, Children: l.Children}
}

// LinkOf is the whole link of one of the viewer's live links, from the
// blob; "" for a link made before links were kept, or not the viewer's.
func (s *Service) LinkOf(viewerID string, dataKey [crypt.KeyLen]byte, id string) string {
	for _, l := range s.PublishedLinks(viewerID) {
		if l.ID == id {
			blob, ok := s.Catalog(viewerID, dataKey)
			if !ok {
				return ""
			}
			return blob.Links[id]
		}
	}
	return ""
}

// Place is where a published thing is: a collection of a connection, and a
// record of it (UID; empty — the collection itself).
type Place struct {
	AccountID, Col, UID string
}

// matches says whether a link points at the place: the record, or for a
// collection the collection and every record of it.
func (p Place) matches(l *store.PublishedLink) bool {
	return l.AccountID == p.AccountID && l.Col == p.Col && (p.UID == "" || l.UID == p.UID)
}

// FollowPublished points the viewer's links at from to to: a record moved,
// merged into another or given another UID, a collection moved (Б41 of the
// audit of 11.10.2026: the link kept its old place and the guest read «no
// longer there»). A link that goes to another connection gets its envelope
// sealed again with that connection's address and credentials (conn), as an
// edit of a connection does. It is no longer gone. The number of links
// followed is returned and journalled.
func (s *Service) FollowPublished(viewerID string, from, to Place, conn sealed.Connection) (int, error) {
	if from == to {
		return 0, nil
	}
	now := s.now()
	n := 0
	err := s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(viewerID)
		if u == nil {
			return nil, ErrNotFound
		}
		var plain []byte
		for i := range st.Published {
			l := &st.Published[i]
			if l.UserID != viewerID || !from.matches(l) {
				continue
			}
			if to.AccountID != l.AccountID {
				if plain == nil {
					var err error
					if plain, err = (publish.Envelope{Server: conn.Server, Login: conn.Login, Password: conn.Password, CertFingerprint: conn.CertFingerprint}).Marshal(); err != nil {
						return nil, err
					}
					defer crypt.Wipe(plain)
				}
				env, err := s.st.SealLink(l.Hash, plain)
				if err != nil {
					return nil, err
				}
				crypt.Wipe(l.Envelope)
				l.Envelope = env
			}
			l.AccountID, l.Col = to.AccountID, to.Col
			if from.UID != "" {
				l.UID = to.UID
			}
			l.Gone = false
			n++
		}
		if n == 0 {
			return nil, nil
		}
		return []store.AuditEntry{{When: now, Who: u.Login, What: "publish.follow", Whom: u.Login, Detail: fmt.Sprintf("%s links %d", to.AccountID, n)}}, nil
	})
	return n, err
}

// MarkPublishedGone says of the viewer's links at the place that what they
// open is no longer there (Б41): the owner's list shows them dead.
func (s *Service) MarkPublishedGone(viewerID string, at Place) error {
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		for i := range st.Published {
			l := &st.Published[i]
			if l.UserID == viewerID && at.matches(l) {
				l.Gone = true
			}
		}
		return nil, nil
	})
}

// MarkLinkGone says of one link, by its ID, that what it opens is no
// longer there — a guest found it so (Б41).
// NoteLinkMissing keeps how many records a guest's read of the link left
// out, Carrel unable to read them as format (Б3; s-publish-40): the owner's
// list says so. A read like the last writes nothing.
func (s *Service) NoteLinkMissing(id string, n int, format string) error {
	if n == 0 {
		format = ""
	}
	for _, l := range s.st.Snapshot().Published {
		if publish.ID(l.Hash) == id && l.Missing == n && l.MissingFormat == format {
			return nil
		}
	}
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		for i := range st.Published {
			if l := &st.Published[i]; publish.ID(l.Hash) == id {
				l.Missing, l.MissingFormat = n, format
			}
		}
		return nil, nil
	})
}

func (s *Service) MarkLinkGone(id string) error {
	// Read first: a guest opening a dead link again writes nothing.
	for _, l := range s.st.Snapshot().Published {
		if publish.ID(l.Hash) == id && l.Gone {
			return nil
		}
	}
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		for i := range st.Published {
			l := &st.Published[i]
			if publish.ID(l.Hash) == id && !l.Gone {
				l.Gone = true
			}
		}
		return nil, nil
	})
}
