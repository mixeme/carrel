package web

import (
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
// it in its place, read-only storage visible and closed.

func (s *Server) getFilesPicker(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	if r.FormValue("for") != "upload" {
		s.notFound(w, r)
		return
	}
	var chosen *fileCtx
	if to := r.FormValue("to"); to != "" {
		chosen, _ = s.folderAddress(sess, to)
	}
	s.writeMailSheet(w, r, s.uploadPicker(r, sess, chosen, ""))
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
		s.writeMailSheet(w, r, s.uploadPicker(r, sess, nil, "Choose a folder of a file collection in the tree."))
		return
	case dest.col.ReadOnly:
		s.writeMailSheet(w, r, s.uploadPicker(r, sess, dest, readOnlyReason+" Choose a folder you can write to."))
		return
	}
	// New folder of the tree (s-notes-11): made in the chosen folder and
	// chosen in its turn; remembered, like any folder, by Choose this folder.
	if r.FormValue("mkcol") != "" {
		name := strings.TrimSpace(r.PostFormValue("name"))
		if n := s.makeFolder(r.Context(), dest, name); n != nil {
			s.writeMailSheet(w, r, s.uploadPicker(r, sess, dest, string(n.Title)+". "+string(n.Body)))
			return
		}
		s.writeMailSheet(w, r, s.uploadPicker(r, sess, dest.at(append(slices.Clone(dest.segs), name)), ""))
		return
	}
	key, _ := sess.DataKey()
	_ = s.users.SetUploadFolder(sess.AccountID(), key, sealed.AttachmentFolder{AccountID: dest.conn.ID, Path: dest.col.Path, Folder: dest.segs})
	s.writeMailSheet(w, r, s.uploadPicker(r, sess, dest, ""))
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

func (s *Server) uploadPicker(r *http.Request, sess *session.Session, chosen *fileCtx, errText string) view.FilePickerSheet {
	blob := s.catalogOf(sess)
	purpose := "Upload files: where to put them. The folder is the attachment folder from Settings → Attachments unless you choose another here; the next upload from this menu goes where you chose last. A name already taken is refused, not overwritten."
	if chosen == nil {
		chosen = s.uploadFolder(sess, blob)
	}
	sheet := view.FilePickerSheet{Purpose: text(purpose), Tree: *s.fileTree(r.Context(), &fileCtx{blob: blob}, chosen)}
	if errText != "" {
		sheet.Purpose = text(errText)
	}
	choose := view.Action{ID: "files.picker.choose", Label: "Choose this folder", Method: "POST", Enabled: true,
		Href: s.postQuery("/app/files/picker", url.Values{"for": {"upload"}})}
	upload := view.Action{ID: "files.picker.upload", Label: "Upload here", Method: "POST"}
	switch {
	case chosen == nil:
		upload.Reason = "Choose a folder in the tree first."
	case chosen.col.ReadOnly:
		upload.Reason = text(readOnlyReason)
	default:
		upload.Enabled, upload.Href = true, s.route(chosen.href("upload", chosen.segs, true))
	}
	sheet.Tree.New = s.newFolderAction(chosen, "/app/files/picker", url.Values{"for": {"upload"}, "mkcol": {"1"}})
	sheet.Actions = []view.Action{upload, choose,
		{ID: "files.picker.cancel", Label: "Cancel", Method: "GET", Enabled: true, Href: s.route("/app/files")}}
	return sheet
}
