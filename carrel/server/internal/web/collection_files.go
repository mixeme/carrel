package web

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/provider/file"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// S-12.1 for the folders of a file storage (s-profile-12): a folder is renamed
// by MOVE, deleted by DELETE, and goes to another server as a tree, the way
// files go (S-17.1). What a storage offers as its collections are the folders
// at its address; the address itself, when it is the collection, is the
// whole storage, and that one is neither renamed nor deleted from here.

const storageRootReason = "This is the whole storage, not a folder in it."

// storageRoot reports whether the collection is the storage itself rather
// than a folder in it: the address the connection was made with, or the
// server's root.
func storageRoot(conn sealed.Connection, col sealed.Collection) bool {
	p := strings.TrimSuffix(col.Path, "/")
	if p == "" {
		return true
	}
	if conn.FileRoot == "" {
		return false
	}
	u, err := url.Parse(conn.FileRoot)
	return err == nil && strings.TrimSuffix(u.Path, "/") == p
}

// fileHome is where a new folder of a file storage is made: beside the
// folders it holds, else at the address of a plain storage. Empty for a
// connection that holds no files — a folder has no place to go there.
func fileHome(blob sealed.Blob, conn sealed.Connection) string {
	for _, c := range blob.CollectionsOf(conn.ID) {
		if c.Gone || !c.Has(view.KindFile) || storageRoot(conn, c) {
			continue
		}
		return strings.TrimSuffix(path.Dir(strings.TrimSuffix(c.Path, "/")), "/") + "/"
	}
	if conn.FileRoot == "" {
		return ""
	}
	u, err := url.Parse(conn.FileRoot)
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(u.Path, "/") + "/"
}

// collectionHome is where a collection like col is made on conn.
func collectionHome(blob sealed.Blob, conn sealed.Connection, col sealed.Collection) string {
	if col.Has(view.KindFile) {
		return fileHome(blob, conn)
	}
	return homeFor(conn, col.Kinds[0])
}

// fileProvider is the file provider over a guarded client of the connection.
func (s *Server) fileProvider(conn sealed.Connection) (*file.Provider, error) {
	client, err := dav.NewClient(s.guard, conn.Server, conn.Login, conn.Password, conn.CertFingerprint)
	if err != nil {
		return nil, err
	}
	return file.New(client)
}

// runFiles moves or copies a folder of a storage to another one (S-12.1:
// «as a tree, like files»): a folder of the same name, free there, is made
// and the tree is copied into it a file at a time through Carrel; a copy that
// stops part way is taken back. The original goes only once the copy is
// whole, and the catalog holds both before anything is deleted.
func (m *collectionMove) runFiles(ctx context.Context) (string, *view.Notice) {
	label := collectionLabel(m.col)
	src, err := m.s.fileProvider(m.conn)
	if err != nil {
		return "", davRefusal(moveCollectionRefusal(m.title(), label), m.conn, "PROPFIND", m.col.Path, err)
	}
	dst, err := m.s.fileProvider(m.dest)
	if err != nil {
		return "", davRefusal(moveCollectionRefusal(m.title(), label), m.dest, "PROPFIND", m.home, err)
	}
	client, err := dav.NewClient(m.s.guard, m.dest.Server, m.dest.Login, m.dest.Password, m.dest.CertFingerprint)
	var taken map[string]bool
	if err == nil {
		taken, err = childSegments(ctx, client, m.home)
	}
	if err != nil {
		return "", davRefusal(moveCollectionRefusal(m.title(), label), m.dest, "PROPFIND", m.home, err)
	}
	base := path.Base(strings.TrimSuffix(m.col.Path, "/"))
	if file.CheckName(base) != nil {
		base = label
	}
	name := base
	for n := 2; taken[strings.ToLower(name)]; n++ {
		name = fmt.Sprintf("%s-%d", base, n)
	}
	newPath := strings.TrimSuffix(m.home, "/") + "/" + name + "/"
	m.placed = sealed.Collection{Path: newPath, Name: name, Kinds: []view.Kind{view.KindFile}}
	if n, err := file.CopyTree(ctx, src, m.col.Path, dst, newPath, m.s.cfg.FilesMaxEntries); err != nil {
		// The name was free when it was looked up: what stands there now is
		// what this copy made, and a half copy is not left behind (S-12.2).
		_ = dst.Delete(context.WithoutCancel(ctx), newPath, "")
		notice := davRefusal(moveCollectionRefusal(m.title(), label), m.dest, "PUT", newPath, err)
		if n > 0 {
			notice.Body = text(fmt.Sprintf("%s stopped after %s, so the new folder there was taken back. %s is as it was.",
				connLabel(m.dest), plural(n, "entry"), label))
		}
		return "", notice
	}
	if err := m.place(false); err != nil {
		_ = dst.Delete(context.WithoutCancel(ctx), newPath, "")
		return "", m.catalogFailure()
	}
	if m.copying {
		return m.newAddress(), nil
	}
	if err := src.Delete(ctx, m.col.Path, ""); err != nil {
		n := m.kept("Copied, but the original could not be deleted",
			fmt.Sprintf("Both are there now. Delete %s on %s yourself once the server lets you.", label, connLabel(m.conn)), nil)
		n.Details = davRefusal(moveCollectionRefusal("", label), m.conn, "DELETE", m.col.Path, err).Details
		return "", n
	}
	if err := m.place(true); err != nil {
		return "", m.kept("Moved, but the list of collections still names the original",
			"The folder was moved and the original deleted; run re-discovery on its connection to take it out.", nil)
	}
	return m.newAddress(), nil
}

// createFolder makes a folder of a plain file storage (s-profile-1, New
// folder): beside its folders, under the name typed — for a folder the name
// is the address — never over something already there. A refusal creates
// nothing and keeps the form (S-12.2).
func (s *Server) createFolder(w http.ResponseWriter, r *http.Request, sess *session.Session, key [32]byte, blob sealed.Blob, f collectionForm, conn sealed.Connection, home string) {
	ctx := r.Context()
	files, err := s.fileProvider(conn)
	var client *dav.Client
	if err == nil {
		client, err = dav.NewClient(s.guard, conn.Server, conn.Login, conn.Password, conn.CertFingerprint)
	}
	var taken map[string]bool
	if err == nil {
		taken, err = childSegments(ctx, client, home)
	}
	if err != nil {
		s.writePage(w, r, http.StatusOK, s.collectionNewSheet(sess, blob, f, davRefusal(createRefusal, conn, "PROPFIND", home, err)))
		return
	}
	if taken[strings.ToLower(f.name)] {
		f.errs["name"] = "Something already lives at this address on the server."
		s.writePage(w, r, http.StatusOK, s.collectionNewSheet(sess, blob, f, nil))
		return
	}
	p := strings.TrimSuffix(home, "/") + "/" + f.name + "/"
	if err := files.MkCol(ctx, p); err != nil {
		s.writePage(w, r, http.StatusOK, s.collectionNewSheet(sess, blob, f, davRefusal(createRefusal, conn, "MKCOL", p, err)))
		return
	}
	if err := s.users.AddCollection(sess.AccountID(), key, conn.ID, sealed.Collection{Path: p, Name: f.name, Kinds: []view.Kind{view.KindFile}}); err != nil {
		_ = files.Delete(context.WithoutCancel(ctx), p, "")
		http.Error(w, "could not save the collection", http.StatusInternalServerError)
		return
	}
	s.redirect(w, r, fileHref(conn.ID, p, "", nil, true))
}

// renameFileCollection is the POST of the rename sheet for a folder: the
// name must be a name, the whole storage has none to change, and a refusal
// keeps the form and changes nothing in the catalog.
func (s *Server) renameFileCollection(w http.ResponseWriter, r *http.Request, sess *session.Session, key [32]byte, conn sealed.Connection, col sealed.Collection, name string) {
	switch {
	case storageRoot(conn, col):
		s.writePage(w, r, http.StatusOK, s.collectionRenameSheet(sess, conn, col, name, "", map[string]view.Text{"name": storageRootReason}, nil, sheetBack(r.URL.Query().Get("back"))))
		return
	case file.CheckName(name) != nil:
		s.writePage(w, r, http.StatusOK, s.collectionRenameSheet(sess, conn, col, name, "",
			map[string]view.Text{"name": "A name cannot be “.” or “..”, and cannot hold “/” or “\\”."}, nil, sheetBack(r.URL.Query().Get("back"))))
		return
	case name == collectionLabel(col):
		s.redirect(w, r, "/app/settings/connections")
		return
	}
	renamed, err := s.renameFolder(r.Context(), conn, col, name)
	if err != nil {
		s.writePage(w, r, http.StatusOK, s.collectionRenameSheet(sess, conn, col, name, "", nil,
			davRefusal(renameRefusal, conn, "MOVE", col.Path, err), sheetBack(r.URL.Query().Get("back"))))
		return
	}
	if err := s.users.PlaceMovedCollection(sess.AccountID(), key, conn.ID, col.Path, conn.ID, renamed, true); err != nil {
		s.writePage(w, r, http.StatusOK, s.connectionsPage(sess, connectionForm{}, &view.Notice{
			Role: view.NoticeAlert, Title: "Renamed, but the list of collections still names the old folder",
			Body: text(col.Path + " is " + renamed.Path + " on " + connLabel(conn) + " now; run Look for changes on its connection to take the new one in."),
		}))
		return
	}
	s.redirect(w, r, "/app/settings/connections")
}

// renameFolder renames a folder of a storage: MOVE to the new name beside it,
// never over something already there, then the catalog follows with its
// ticks and the attachment folder (S-12.1). The address changes with the
// name — for a folder the name is its address.
func (s *Server) renameFolder(ctx context.Context, conn sealed.Connection, col sealed.Collection, name string) (sealed.Collection, error) {
	files, err := s.fileProvider(conn)
	if err != nil {
		return sealed.Collection{}, err
	}
	newPath := path.Dir(strings.TrimSuffix(col.Path, "/")) + "/" + name + "/"
	if err := files.Move(ctx, col.Path, newPath, false); err != nil {
		return sealed.Collection{}, err
	}
	renamed := col
	renamed.Path, renamed.Name, renamed.ChangeTag = newPath, name, ""
	return renamed, nil
}
