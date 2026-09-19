package web

import (
	"fmt"
	"net/http"
	"net/url"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// Disconnect (connections-tail.md): a connection leaves Carrel, and on its
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
	if WantsFragment(r) && FragmentTarget(r) == TargetSheet {
		_ = s.render.Fragment(w, r, http.StatusOK, TargetSheet, sheet, nil)
		return
	}
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
		Warning: text(fmt.Sprintf(
			"%s leaves Carrel: its collections, the ticks and the decisions taken about them go with it, and nothing is deleted on %s. Connecting it again finds the collections anew, without the ticks and without what is listed here.",
			label, host)),
		References: s.disconnectReferences(sess, blob, conn),
		Form:       s.plainForm(action, "POST", csrfField(sess)),
		Actions: []view.Action{
			{ID: "connection.disconnect.confirm", Label: "Disconnect", Href: s.route(action), Method: "POST", Danger: true, Enabled: true},
			{ID: "connection.disconnect.cancel", Label: "Cancel", Href: s.route("/app/settings/connections"), Method: "GET", Enabled: true},
		},
	}
}

// disconnectReferences name everything of the person that goes or loses its
// ground with the connection: its collections, the links that stop opening,
// the copy jobs that lose a source or their destination, and the davloom
// devices that stop seeing it.
func (s *Server) disconnectReferences(sess *session.Session, blob sealed.Blob, conn sealed.Connection) []view.Text {
	refs := []view.Text{}
	var live, ticked int
	for _, col := range blob.CollectionsOf(conn.ID) {
		if col.Gone {
			continue
		}
		live++
	}
	for _, sel := range blob.Selected {
		if sel.AccountID == conn.ID {
			ticked++
		}
	}
	if live > 0 {
		line := plural(live, "collection") + " leave the rail"
		if ticked > 0 {
			line += fmt.Sprintf("; %d of the ticks in the combined views go with them", ticked)
		}
		refs = append(refs, text(line))
	}
	for _, l := range s.users.PublishedLinks(sess.AccountID()) {
		if l.Target.AccountID == conn.ID {
			refs = append(refs, text("Published link «"+l.Target.Title+"» — it stops opening, and its password goes with it"))
		}
	}
	for _, j := range blob.BackupJobs {
		for _, src := range j.Sources {
			if src.AccountID == conn.ID {
				refs = append(refs, text("Backup job «"+j.Name+"» — it loses a source and copies the rest"))
				break
			}
		}
		if j.DestAccount == conn.ID {
			refs = append(refs, text("Backup job «"+j.Name+"» — its copies were written here; it will say the destination is gone"))
		}
	}
	for _, d := range s.users.Devices(sess.AccountID()) {
		for _, c := range d.ScopeCols {
			if c.AccountID == conn.ID {
				refs = append(refs, text("davloom device «"+d.Name+"» — these collections leave it at its next sync"))
				break
			}
		}
	}
	for _, g := range blob.Duplicates.Linked {
		for _, m := range g.Members {
			if m.AccountID == conn.ID {
				refs = append(refs, text("A linked group of duplicates — the records of this connection leave it"))
				break
			}
		}
	}
	return refs
}
