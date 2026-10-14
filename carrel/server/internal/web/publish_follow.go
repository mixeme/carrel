package web

import (
	"net/http"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
)

// A published record goes where Carrel takes it (Б41 of the audit of
// 11.10.2026): a link to a record moved, merged into another or given
// another UID by its server opens the record in its new place; a link to a
// collection moved opens the new one. Where Carrel cannot follow — a record
// deleted, a record moved under a UID the server did not say, a collection
// removed — the link is marked gone, and the owner's list says it is dead.
// A guest who finds what a link opens no longer there marks it so too.

// followLinks points the session's links at the record (uid; empty — the
// collection) of c to its new place in to. A link that could not be pointed
// is left as it was: the guest finds it gone, and it is marked then.
func (s *Server) followLinks(sess *session.Session, from user.Place, to *collectionCtx, uid string) {
	_, _ = s.users.FollowPublished(sess.AccountID(), from, user.Place{AccountID: to.conn.ID, Col: to.col.Path, UID: uid}, to.conn)
}

// linksGone marks the session's links at the place gone.
func (s *Server) linksGone(sess *session.Session, at user.Place) {
	_ = s.users.MarkPublishedGone(sess.AccountID(), at)
}

// placeOf is where a record of c is, for its links.
func placeOf(c *collectionCtx, uid string) user.Place {
	return user.Place{AccountID: c.conn.ID, Col: c.col.Path, UID: uid}
}

// goneErr says that what a read asked for is not on its server: a record
// (notFoundErr), or the collection itself — 404 or 410 to its listing.
func goneErr(err error) bool {
	return notFoundErr(err) || dav.IsStatus(err, http.StatusNotFound) || dav.IsStatus(err, http.StatusGone)
}
