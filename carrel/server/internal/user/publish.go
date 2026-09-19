package user

import (
	"context"
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
}

// PublishedLink is a link as its owner's screens show it: no secret.
type PublishedLink struct {
	ID        string
	Target    PublishTarget
	CreatedAt time.Time
	ExpiresAt *time.Time
	LastSeen  *time.Time
}

// OpenLink is a live link opened for a reader without a session.
type OpenLink struct {
	ID     string // publish.ID of the link: its rate bucket, never its secret
	Target PublishTarget
	Conn   sealed.Connection
}

// lastSeenEvery keeps a busy link from rewriting the state on every view.
const lastSeenEvery = time.Hour

// Publish makes a link to target, living until expires (nil: no expiry,
// chosen explicitly). conn is the target's connection as the owner's blob
// holds it: its address and credentials go into the link's envelope, sealed
// under the server key. The secret is returned once and never kept. The
// journal names the kind and the connection.
func (s *Service) Publish(viewerID string, dataKey [crypt.KeyLen]byte, conn sealed.Connection, target PublishTarget, expires *time.Time) (string, error) {
	raw, hash, err := newSecret()
	if err != nil {
		return "", err
	}
	plain, err := publish.Envelope{Server: conn.Server, Login: conn.Login, Password: conn.Password, CertFingerprint: conn.CertFingerprint}.Marshal()
	if err != nil {
		return "", err
	}
	env, err := s.st.SealLink(hash, plain)
	crypt.Wipe(plain)
	if err != nil {
		return "", err
	}
	now := s.now()
	err = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(viewerID)
		if u == nil || u.Disabled {
			return nil, ErrNotFound
		}
		prunePublished(st, now)
		st.Published = append(st.Published, store.PublishedLink{
			Hash: hash, UserID: u.ID, AccountID: target.AccountID, Kind: target.Kind, Col: target.Col,
			UID: target.UID, Path: target.Path, Folder: target.Folder, Title: target.Title,
			From: target.From, To: target.To, NoDownload: target.NoDownload, HideDescriptions: target.HideDescriptions,
			CreatedAt: now, ExpiresAt: expires, Envelope: env,
		})
		return []store.AuditEntry{{
			When: now, Who: u.Login, What: "publish.create", Whom: u.Login, Detail: target.Kind + " " + target.AccountID,
		}}, nil
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
	if found.LastSeen == nil || now.Sub(*found.LastSeen) >= lastSeenEvery {
		_ = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
			for i := range st.Published {
				if verifyHash(st.Published[i].Hash, hash) {
					st.Published[i].LastSeen = &now
				}
			}
			return nil, nil
		})
	}
	return OpenLink{
		ID:     publish.ID(found.Hash),
		Target: targetOf(*found),
		Conn:   sealed.Connection{ID: found.AccountID, Server: env.Server, Login: env.Login, Password: env.Password, CertFingerprint: env.CertFingerprint},
	}, true
}

// PublicLinkValid reports whether a token opens a live link.
func (s *Service) PublicLinkValid(_ context.Context, token string) bool {
	_, ok := s.OpenPublished(token)
	return ok
}

// PublishedLinks are the viewer's live links, newest first.
func (s *Service) PublishedLinks(viewerID string) []PublishedLink {
	snap := s.st.Snapshot()
	now := s.now()
	var out []PublishedLink
	for i := len(snap.Published) - 1; i >= 0; i-- {
		l := snap.Published[i]
		if l.UserID != viewerID || publish.Expired(l.ExpiresAt, now) {
			continue
		}
		out = append(out, PublishedLink{ID: publish.ID(l.Hash), Target: targetOf(l), CreatedAt: l.CreatedAt, ExpiresAt: l.ExpiresAt, LastSeen: l.LastSeen})
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
		crypt.Wipe(st.Published[at].Envelope)
		st.Published = append(st.Published[:at], st.Published[at+1:]...)
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
		Title: l.Title, From: l.From, To: l.To, NoDownload: l.NoDownload, HideDescriptions: l.HideDescriptions}
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
