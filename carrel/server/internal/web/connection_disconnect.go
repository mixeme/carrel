package web

import (
	"fmt"
	"net/http"
	"net/url"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// Disconnect (review Р-9): a connection leaves Carrel, and on its
// server nothing is touched. The sheet is the brother of the collection's
// delete sheet (s-profile-9) — what exactly goes, named before it goes —
// without an export or an archive: there is nothing to save from deletion.

func disconnectQuery(accountID string) url.Values {
	return url.Values{"account": {accountID}}
}

// openDisconnect finds the connection the sheet is about. A subscription is
// unsubscribed from, not disconnected, and is not found here.
func (s *Server) openDisconnect(w http.ResponseWriter, r *http.Request) (*session.Session, sealed.Blob, sealed.Connection, bool) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return nil, sealed.Blob{}, sealed.Connection{}, false
	}
	key, live := sess.DataKey()
	if !live {
		s.notFound(w, r)
		return nil, sealed.Blob{}, sealed.Connection{}, false
	}
	blob, opened := s.users.Catalog(sess.AccountID(), key)
	conn, known := blob.Connection(r.FormValue("account"))
	if !opened || !known || conn.Feed {
		s.notFound(w, r)
		return nil, sealed.Blob{}, sealed.Connection{}, false
	}
	return sess, blob, conn, true
}

func (s *Server) getConnectionDisconnect(w http.ResponseWriter, r *http.Request) {
	sess, blob, conn, ok := s.openDisconnect(w, r)
	if !ok {
		return
	}
	sheet := s.disconnectSheet(sess, blob, conn)
	if render.WantsFragment(r) && render.FragmentTarget(r) == render.TargetSheet {
		_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, sheet, nil)
		return
	}
	sheet.Under = s.underOf(r, "", "/app/settings/connections") // Лс29
	s.writePage(w, r, http.StatusOK, sheet)
}

// postConnectionDisconnect takes the connection out and goes back to the
// connections. Nothing is asked of the server: what leaves is Carrel's own
// knowledge of it (user.RemoveConnection).
func (s *Server) postConnectionDisconnect(w http.ResponseWriter, r *http.Request) {
	sess, _, conn, ok := s.openDisconnect(w, r)
	if !ok {
		return
	}
	key, _ := sess.DataKey()
	if _, err := s.users.RemoveConnection(sess.AccountID(), key, conn.ID); err != nil {
		http.Error(w, "could not disconnect", http.StatusInternalServerError)
		return
	}
	sess.Cache().InvalidateConnection(conn.ID)
	s.redirect(w, r, "/app/settings/connections")
}

func (s *Server) disconnectSheet(sess *session.Session, blob sealed.Blob, conn sealed.Connection) view.ConnectionRemoveSheet {
	action := "/app/settings/connections/disconnect?" + disconnectQuery(conn.ID).Encode()
	label, host := connLabel(conn), serverHost(conn.Server)
	subject := label
	if label != host {
		subject += " · " + host
	}
	return view.ConnectionRemoveSheet{
		Subject: text(subject),
		// What stays, in alarm, and what goes after it in numbers, without
		// names (s-profile-26; the owner, 19.10.2026).
		Warning:    text("Nothing is deleted on " + host + ". Carrel removes:"),
		References: s.disconnectReferences(sess, blob, conn),
		Form:       s.plainForm(action, "POST", csrfField(sess)),
		Actions: []view.Action{
			{ID: "connection.disconnect.confirm", Label: "Disconnect", Href: s.route(action), Method: "POST", Danger: true, Enabled: true},
			{ID: "connection.disconnect.cancel", Label: "Cancel", Href: s.route("/app/settings/connections"), Method: "GET", Enabled: true},
		},
	}
}

// disconnectReferences are what goes or loses its ground with the
// connection, counted (s-profile-26; the owner, 19.10.2026: numbers, no
// names): its collections, the links that stop opening, the sources and
// destinations of copy jobs, the davloom devices that sync them, the
// linked groups of duplicates it is in.
func (s *Server) disconnectReferences(sess *session.Session, blob sealed.Blob, conn sealed.Connection) []view.Text {
	var cols, links, sources, dests, devices, groups int
	for _, col := range blob.CollectionsOf(conn.ID) {
		if !col.Gone {
			cols++
		}
	}
	for _, l := range s.users.PublishedLinks(sess.AccountID()) {
		if l.Target.AccountID == conn.ID {
			links++
		}
	}
	for _, j := range blob.BackupJobs {
		for _, src := range j.Sources {
			if src.AccountID == conn.ID {
				sources++
				break
			}
		}
		if j.DestAccount == conn.ID {
			dests++
		}
	}
	for _, d := range s.users.Devices(sess.AccountID()) {
		for _, c := range d.ScopeCols {
			if c.AccountID == conn.ID {
				devices++
				break
			}
		}
	}
	for _, g := range blob.Duplicates.Linked {
		for _, m := range g.Members {
			if m.AccountID == conn.ID {
				groups++
				break
			}
		}
	}
	refs := []view.Text{}
	add := func(n int, one, many string) {
		switch {
		case n == 1:
			refs = append(refs, text("1 "+one))
		case n > 1:
			refs = append(refs, text(fmt.Sprintf("%d %s", n, many)))
		}
	}
	add(cols, "collection", "collections")
	add(links, "published link", "published links")
	add(sources, "source of a backup job", "sources of backup jobs")
	add(dests, "destination of a backup job", "destinations of backup jobs")
	add(devices, "davloom device’s sync of them", "davloom devices’ sync of them")
	add(groups, "linked group of duplicates", "linked groups of duplicates")
	return refs
}
