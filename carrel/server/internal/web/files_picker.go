package web

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// The picker (U-6.8 "/app/files/picker"): one tree of folders for choosing
// where something goes. "Upload file" of the create menu opens it: from a
// calendar there is no folder open, and putting a file in the first
// collection at hand is not allowed — it is somebody's server. The folder
// is the one chosen here last, else the attachment folder; the tree shows
// it in its place, read-only storage visible and closed. From Files of
// Attach opens the same tree to choose a file (for=attach, below), and
// Browse of Settings → Attachments to choose the folder (for=attachments).

func (s *Server) getFilesPicker(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	switch r.FormValue("for") {
	case "upload":
	case "attach":
		s.getAttachPicker(w, r, sess)
		return
	case "attachments":
		s.writeMailSheet(w, r, s.attachmentsPicker(r, sess))
		return
	default:
		s.notFound(w, r)
		return
	}
	var chosen *fileCtx
	if to := r.FormValue("to"); to != "" {
		chosen, _ = s.folderAddress(sess, to)
	}
	s.writeMailSheet(w, r, s.uploadPicker(r, sess, chosen, nil, r.FormValue("new") != "", ""))
}

// getAttachPicker is From Files of Attach (s-agenda-6, s-tasks-7): the same
// tree, choosing a file for a record — k, a, c and uid name it, etag and
// from travel on to its edit op. ?to= opens a folder or chooses a file; the
// tree starts at the attachment folder. Attach posts ?op=attachfile to the
// record, which answers as an upload does. Only a record Attach is offered
// on opens it: a writable calendar collection of the person's.
func (s *Server) getAttachPicker(w http.ResponseWriter, r *http.Request, sess *session.Session) {
	q := r.URL.Query()
	spec, known := recordKinds[q.Get("k")]
	account, col, uid := q.Get("a"), q.Get("c"), q.Get("uid")
	// ?new= — the draft of a new form (attach_new.go): no record yet.
	fresh := issuedUID(q.Get("new"))
	if !known || spec.component == "" || account == "" || col == "" || (uid == "" && fresh == "") {
		s.notFound(w, r)
		return
	}
	if fresh == "" && !s.checkOwnership(w, r, sess, account, col, uid) {
		return
	}
	c, ok := s.collectionOf(sess, spec, account, col)
	if !ok || c.col.ReadOnly || c.feed != nil || c.cal == nil {
		s.notFound(w, r)
		return
	}
	if fresh != "" {
		if _, held := loadAttachDraft(c, fresh); !held {
			s.notFound(w, r)
			return
		}
	}
	blob := s.catalogOf(sess)
	var at *fileCtx
	name := ""
	switch to := q.Get("to"); {
	case to == "":
		at = s.savedFolder(sess, blob)
	default:
		if fc, dir, ok := s.fileAddress(sess, to); ok && dir {
			at = fc
		} else if ok && len(fc.segs) > 0 {
			at, name = fc.at(fc.segs[:len(fc.segs)-1]), fc.segs[len(fc.segs)-1]
		}
	}
	if at != nil && at.col.Feed {
		at, name = nil, ""
	}
	etag, from := q.Get("etag"), q.Get("from")
	limit := attachLimit(c.col)
	// The head chooses the server (s-files-16); another one asked for,
	// nothing of it is open yet.
	server := s.treeServer(blob, at, q.Get("server"), true)
	if at != nil && at.conn.ID != server {
		at, name = nil, ""
	}
	sheet := view.FilePickerSheet{
		Title:   "Attach from Files",
		Purpose: text(fmt.Sprintf("Attach a file that is in Files already. Up to %s it is written into the record, as a file from this computer is; a larger one stays where it lies, and the record keeps a link to it.", sizeText(limit))),
		Tree:    *s.sourceTree(r.Context(), blob, at, name, limit, server),
		Servers: s.fileServers(blob, server, true, func(id string) string {
			v := cloneValues(q)
			v.Del("to")
			v.Set("server", id)
			return s.postQuery("/app/files/picker", v)
		}),
	}
	linkTree(&sheet.Tree, func(id string) string {
		v := cloneValues(q)
		v.Del("server")
		v.Set("to", id)
		return s.postQuery("/app/files/picker", v)
	})
	// The head names the record the file goes to (s-files-16).
	if fresh != "" {
		sheet.Subject = text("to the new " + c.spec.noun)
	} else if obj, err := c.find(r.Context(), uid); err == nil {
		sheet.Subject = text("to «" + recordTitle(c.spec.kind, obj) + "»")
	}
	attach := view.Action{ID: "files.picker.attach", Label: "Attach", Method: "POST", Primary: true} // no icon: s-files-16
	cancel := s.route(attachBack(c, uid, from))
	switch {
	case name == "":
		attach.Reason = "Choose a file in the tree."
	case fresh != "":
		attach.Enabled, attach.Href = true, s.newAttachStep(c, "attachfile", fresh, url.Values{"file": {sheet.Tree.Chosen}})
	default:
		op := s.opQuery("attachfile", etag, from)
		op.Set("file", sheet.Tree.Chosen)
		attach.Enabled, attach.Href = true, s.postQuery(c.recordPath(uid)+"/edit", op)
	}
	if fresh != "" {
		cancel = s.route(newFormAddress(c, fresh))
	}
	sheet.Actions = []view.Action{attach,
		{ID: "files.picker.cancel", Label: "Cancel", Method: "GET", Enabled: true, Href: cancel}}
	s.writeMailSheet(w, r, sheet)
}

// attachmentsPicker is Browse of Settings → Attachments (attachments-1): the
// tree at the folder of the form, ?to= choosing another; «Choose this
// folder» goes back to the settings with it, where Save stores it. A
// read-only storage is shown and not chosen.
func (s *Server) attachmentsPicker(r *http.Request, sess *session.Session) view.FilePickerSheet {
	blob := s.catalogOf(sess)
	var chosen *fileCtx
	if to := r.FormValue("to"); to != "" {
		chosen, _ = s.folderAddress(sess, to)
	} else {
		chosen = s.savedFolder(sess, blob)
	}
	if chosen != nil && chosen.col.Feed {
		chosen = nil
	}
	server := s.treeServer(blob, chosen, r.FormValue("server"), false)
	if chosen != nil && chosen.conn.ID != server {
		chosen = nil
	}
	sheet := view.FilePickerSheet{
		Title:   "Attachment folder",
		Purpose: "Where large attachments go: a folder of a file collection you can write to. Carrel makes it on first use.",
		Tree:    *s.fileTree(r.Context(), &fileCtx{blob: blob}, chosen, server),
		Where:   wherePath(chosen),
		Servers: s.fileServers(blob, server, false, func(id string) string {
			return s.postQuery("/app/files/picker", url.Values{"for": {"attachments"}, "server": {id}})
		}),
	}
	s.markTree(&sheet.Tree, blob, "")
	linkTree(&sheet.Tree, func(id string) string {
		return s.postQuery("/app/files/picker", url.Values{"for": {"attachments"}, "to": {id}})
	})
	choose := view.Action{ID: "files.picker.choose", Label: "Choose this folder", Method: "GET", Primary: true}
	switch {
	case chosen == nil:
		choose.Reason = "Choose a folder in the tree." // attachments-4
	case chosen.col.ReadOnly:
		choose.Reason = text(readOnlyReason)
	default:
		choose.Enabled = true
		choose.Href = s.postQuery("/app/settings/attachments", url.Values{"to": {sheet.Tree.Chosen}})
	}
	sheet.Actions = []view.Action{choose,
		{ID: "files.picker.cancel", Label: "Cancel", Method: "GET", Enabled: true, Href: s.route("/app/settings/attachments")}}
	return sheet
}

// postFilesPicker takes the folder chosen in the tree and remembers it for
// the next upload from the menu.
func (s *Server) postFilesPicker(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	if r.FormValue("for") != "upload" {
		s.notFound(w, r)
		return
	}
	dest, ok := s.folderAddress(sess, r.FormValue("to"))
	switch {
	case !ok:
		s.writeMailSheet(w, r, s.uploadPicker(r, sess, nil, &view.Notice{Role: view.NoticeAlert, Body: "Choose a folder of a file collection in the tree."}, false, ""))
		return
	case dest.col.ReadOnly:
		s.writeMailSheet(w, r, s.uploadPicker(r, sess, dest, &view.Notice{Role: view.NoticeAlert, Body: readOnlyReason + " Choose a folder you can write to."}, false, ""))
		return
	}
	// New folder of the tree (s-notes-11): made in the chosen folder and
	// chosen in its turn; remembered, like any folder, by an upload into it.
	if r.FormValue("mkcol") != "" {
		// A refusal stands under the line of the name, which stays open
		// with what was typed, as in the move sheet; the sheet's own line
		// stays (s-files-19).
		name := strings.TrimSpace(r.PostFormValue("name"))
		if n := s.makeFolder(r.Context(), dest, name); n != nil {
			sheet := s.uploadPicker(r, sess, dest, n, true, "")
			keepTyped(&sheet.Tree, name)
			s.writeMailSheet(w, r, sheet)
			return
		}
		made := dest.at(append(slices.Clone(dest.segs), name))
		s.writeMailSheet(w, r, s.uploadPicker(r, sess, made, nil, false, s.route(made.href("", made.segs, true))))
		return
	}
	key, _ := sess.DataKey()
	_ = s.users.SetUploadFolder(sess.AccountID(), key, sealed.AttachmentFolder{AccountID: dest.conn.ID, Path: dest.col.Path, Folder: dest.segs})
	s.writeMailSheet(w, r, s.uploadPicker(r, sess, dest, nil, false, ""))
}

// uploadFolder is where the menu's upload goes: the folder chosen here
// before, else the attachment folder.
func (s *Server) uploadFolder(sess *session.Session, blob sealed.Blob) *fileCtx {
	if af := blob.UploadTo; af != nil {
		if fc, ok := s.fileCollection(sess, af.AccountID, af.Path); ok {
			if _, err := fc.path(af.Folder, true); err == nil {
				return fc.at(af.Folder)
			}
		}
	}
	return s.savedFolder(sess, blob)
}

// uploadPicker is «Upload files» (s-notes-11): the tree with New folder,
// the line saying where the folder comes from, and a refusal, when there is
// one, beside it — never in its place (s-files-19). open says the line of a
// new folder's name stands; fresh is the folder New folder just made.
func (s *Server) uploadPicker(r *http.Request, sess *session.Session, chosen *fileCtx, failure *view.Notice, open bool, fresh string) view.FilePickerSheet {
	blob := s.catalogOf(sess)
	const purpose = "The folder is the attachment folder from Settings → Attachments, named once and remembered — unless you make a new one here. " +
		"Change it here and the next upload from this menu goes to the new one."
	if chosen == nil {
		chosen = s.uploadFolder(sess, blob)
	}
	// The head chooses the server (s-notes-11); another one asked for,
	// nothing of it is chosen yet.
	server := s.treeServer(blob, chosen, r.FormValue("server"), false)
	if chosen != nil && chosen.conn.ID != server {
		chosen = nil
	}
	sheet := view.FilePickerSheet{Title: "Upload files", Purpose: purpose, Tree: *s.fileTree(r.Context(), &fileCtx{blob: blob}, chosen, server), Failure: failure, Where: wherePath(chosen),
		Servers: s.fileServers(blob, server, false, func(id string) string {
			return s.postQuery("/app/files/picker", url.Values{"for": {"upload"}, "server": {id}})
		})}
	s.markTree(&sheet.Tree, blob, fresh)
	linkTree(&sheet.Tree, func(id string) string {
		return s.postQuery("/app/files/picker", url.Values{"for": {"upload"}, "to": {id}})
	})
	// The sheet has no «Choose this folder» (s-notes-11): a folder is
	// chosen by its node, and the upload into it remembers it for the next
	// one from the menu — its address says it is the menu's (menu=1).
	upload := view.Action{ID: "files.picker.upload", Label: "Upload", Method: "POST", Primary: true, File: uploadField}
	switch {
	case chosen == nil:
		upload.Reason = "Choose a folder in the tree first."
	case chosen.col.ReadOnly:
		upload.Reason = text(readOnlyReason)
	default:
		upload.Enabled, upload.Href = true, s.route(chosen.href("upload", chosen.segs, true))+"?menu=1"
	}
	back := url.Values{"for": {"upload"}}
	if chosen != nil {
		back.Set("to", s.route(chosen.href("", chosen.segs, true)))
	} else {
		back.Set("server", server)
	}
	s.newFolder(&sheet.Tree, chosen, "/app/files/picker", url.Values{"for": {"upload"}, "mkcol": {"1"}}, "/app/files/picker", back, open)
	sheet.Actions = []view.Action{upload,
		{ID: "files.picker.cancel", Label: "Cancel", Method: "GET", Enabled: true, Href: s.route("/app/files")}}
	return sheet
}
