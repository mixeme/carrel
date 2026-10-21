package web

import (
	"net/http"
	"net/url"
	"strings"

	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// Browse of the restore's «From a server» (s-backup-17, Typecase's note 99):
// the tree of the person's storages, the archive chosen in it; Choose goes
// back to the step with its collection and path, where the archive is read
// where it lies.

// browseRestore keeps what was typed in the step and opens the tree at the
// folder of the path typed, or at the collection chosen.
func (s *Server) browseRestore(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	_ = r.ParseForm()
	q := url.Values{"for": {"restore"}}
	blob := s.catalogOf(sess)
	acc, col, _ := strings.Cut(r.PostFormValue("store"), "|")
	if c, ok := blob.Collection(acc, col); ok && c.Has(view.KindFile) {
		var segs []string
		for _, seg := range strings.Split(strings.Trim(r.PostFormValue("path"), "/"), "/") {
			if seg != "" {
				segs = append(segs, seg)
			}
		}
		dir := len(segs) == 0 || !strings.HasSuffix(strings.ToLower(segs[len(segs)-1]), ".zip")
		q.Set("to", s.route(fileHref(acc, col, "", segs, dir)))
	}
	s.redirect(w, r, "/app/files/picker?"+q.Encode())
}

// restorePicker is the sheet of Browse (for=restore): the tree with files,
// ?to= opening a folder or choosing a file; Choose takes the file to the
// step, Cancel goes back to it.
func (s *Server) restorePicker(r *http.Request, sess *session.Session) view.FilePickerSheet {
	blob := s.catalogOf(sess)
	q := r.URL.Query()
	var at *fileCtx
	name := ""
	if to := q.Get("to"); to != "" {
		if fc, dir, ok := s.fileAddress(sess, to); ok && dir {
			at = fc
		} else if ok && len(fc.segs) > 0 {
			at, name = fc.at(fc.segs[:len(fc.segs)-1]), fc.segs[len(fc.segs)-1]
		}
	}
	if at != nil && at.col.Feed {
		at, name = nil, ""
	}
	server := s.treeServer(blob, at, q.Get("server"), true)
	if at != nil && at.conn.ID != server {
		at, name = nil, ""
	}
	sheet := view.FilePickerSheet{
		Title:   "Archive to restore",
		Purpose: "A Carrel archive on one of your storages: it is read where it lies, a piece at a time, and nothing of it is kept on Carrel.",
		Tree:    *s.sourceTree(r.Context(), blob, at, name, 0, server),
		Servers: s.fileServers(sess, blob, server, true, func(id string) string {
			v := cloneValues(q)
			v.Del("to")
			v.Set("server", id)
			return s.postQuery("/app/files/picker", v)
		}),
	}
	clearLands(sheet.Tree.Nodes)
	linkTree(&sheet.Tree, func(id string) string {
		v := cloneValues(q)
		v.Del("server")
		v.Set("to", id)
		return s.postQuery("/app/files/picker", v)
	})
	choose := view.Action{ID: "files.picker.restore", Label: "Choose", Method: "GET", Primary: true, Reason: "Choose an archive in the tree."}
	if name != "" && at != nil {
		file := at.at(append(append([]string{}, at.segs...), name))
		choose.Enabled, choose.Reason = true, ""
		choose.Href = s.route("/app/settings/backup/restore?" + url.Values{"from": {"server"},
			"store": {selValue(file.conn.ID, file.col.Path)}, "path": {"/" + strings.Join(file.segs, "/")}}.Encode())
	}
	sheet.Actions = []view.Action{choose,
		{ID: "files.picker.cancel", Label: "Cancel", Method: "GET", Enabled: true, Href: s.route("/app/settings/backup/restore?from=server")}}
	return sheet
}

// clearLands takes off what Attach would do with a file: a restore reads it
// where it lies.
func clearLands(nodes []view.FileTreeNode) {
	for i := range nodes {
		nodes[i].Lands = ""
		clearLands(nodes[i].Children)
	}
}
