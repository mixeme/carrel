package web

import (
	"context"
	"net/http"
	"slices"
	"sort"
	"strings"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/provider/file"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// Settings → Attachments (S-20, U-6.12): the folder large attachments go to,
// chosen once in the one tree of the product (C-6.7), and the boundary between
// the two ways to keep a file, shown and not editable.

type attachForm struct {
	chosen *fileCtx // the folder the tree shows chosen
	sub    string   // a folder to make inside it on first use
	errors map[string]string
}

func (s *Server) getSettingsAttachments(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	blob := s.catalogOf(sess)
	f := attachForm{chosen: s.savedFolder(sess, blob)}
	if to := r.URL.Query().Get("to"); to != "" {
		f.chosen, _ = s.folderAddress(sess, to)
	}
	s.writePage(w, r, http.StatusOK, s.attachmentsPage(r.Context(), sess, blob, f))
}

// postSettingsAttachments stores the folder: the one chosen in the tree, and
// inside it the folders typed, made when the first attachment goes there.
func (s *Server) postSettingsAttachments(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	blob := s.catalogOf(sess)
	f := attachForm{sub: strings.TrimSpace(r.FormValue("sub")), errors: map[string]string{}}
	dest, ok := s.folderAddress(sess, r.FormValue("to"))
	switch {
	case !ok:
		f.errors["to"] = "Choose a folder of a file collection in the tree."
	case dest.col.ReadOnly:
		f.chosen, f.errors["to"] = dest, readOnlyReason+" Choose a folder you can write to."
	default:
		f.chosen = dest
	}
	var segs []string
	if dest != nil {
		segs = slices.Clone(dest.segs)
	}
	for _, part := range strings.Split(f.sub, "/") {
		if part = strings.TrimSpace(part); part == "" {
			continue
		}
		if file.CheckName(part) != nil {
			f.errors["sub"] = "A folder name cannot be . or .., nor hold a backslash."
			break
		}
		segs = append(segs, part)
	}
	if len(f.errors) == 0 {
		key, _ := sess.DataKey()
		err := s.users.SetAttachmentFolder(sess.AccountID(), key, sealed.AttachmentFolder{AccountID: dest.conn.ID, Path: dest.col.Path, Folder: segs})
		if err == nil {
			s.redirect(w, r, "/app/settings/attachments")
			return
		}
		f.errors["to"] = "The folder could not be stored. Open this page again and choose it once more."
	}
	s.writePage(w, r, http.StatusOK, s.attachmentsPage(r.Context(), sess, blob, f))
}

func (s *Server) attachmentsPage(ctx context.Context, sess *session.Session, blob sealed.Blob, f attachForm) view.AttachmentsPage {
	page := view.AttachmentsPage{Shell: s.buildShell(sess, "Attachments")}

	small := view.FieldSet{Legend: "Small files stay in the record", Rows: []view.FormRow{
		{Name: "inline", Label: text("Up to " + sizeText(conf.AttachInlineMax)), Kind: view.InputText, ReadOnly: true,
			Value: "Written into the event, the task or the note itself, and travels with it."},
		{Name: "larger", Label: "Larger", Kind: view.InputText, ReadOnly: true,
			Value: "Put in the folder below; the record keeps a link to it.",
			Hint:  "A collection that says it takes less (max-resource-size) lowers the boundary for itself. Nothing here raises it."},
	}}
	for _, col := range blob.Collections {
		conn, ok := blob.Connection(col.AccountID)
		if !ok || col.Gone || !(col.Has(view.KindEvent) || col.Has(view.KindTask) || col.Has(view.KindNote)) {
			continue
		}
		small.Rows = append(small.Rows, view.FormRow{
			Name: "limit." + conn.ID + "." + col.Path, Label: view.Text(stamp(connLabel(conn), collectionLabel(col))),
			Kind: view.InputText, Value: text(sizeText(attachLimit(col))), ReadOnly: true,
		})
	}

	tree := s.fileTree(ctx, &fileCtx{blob: blob}, f.chosen)
	folder := view.FieldSet{Legend: "Folder", Rows: []view.FormRow{
		csrfField(sess),
		{Name: "to", Label: "Folder", Kind: view.InputText, Value: text(tree.Chosen), Required: true, Error: text(f.errors["to"]),
			Hint: "Any folder of a file collection you can write to, chosen in the tree."},
		{Name: "sub", Label: "New folder inside", Kind: view.InputText, Value: text(f.sub), Error: text(f.errors["sub"]),
			Hint: "Optional, such as carrel/attachments. Made when the first attachment goes there; it stays an ordinary folder, readable without Carrel."},
	}}
	name := view.FieldSet{Legend: "File name", Rows: []view.FormRow{
		{Name: "name", Label: "Written as", Kind: view.InputText, ReadOnly: true,
			Value: "The date of the record, its title, then the file's own name",
			Hint:  "2026-08-10 Sync-up - report.pdf. A name already taken is never overwritten: a number is added."},
	}}
	page.Folder = *tree
	page.Form = view.Form{
		Action: s.route("/app/settings/attachments"), Method: "POST", Sets: []view.FieldSet{small, folder, name},
		Actions: []view.Action{{ID: "attachments.save", Label: "Save", Method: "POST", Enabled: true}},
	}
	if saved := s.savedFolder(sess, blob); saved != nil {
		page.Form.Actions = append(page.Form.Actions, view.Action{
			ID: "attachments.open", Label: "Open in Files", Icon: "i-file", Href: s.route(saved.href("", saved.segs, true)), Method: "GET", Enabled: true,
		})
	} else if blob.Attachments != nil {
		page.Notices = append(page.Notices, view.Notice{
			Role: view.NoticeAlert, Title: "The folder for attachments is not available",
			Body: "Its storage is no longer among your collections. Large attachments cannot be written until another folder is chosen.",
		})
	}
	names := make([]string, 0, len(f.errors))
	for n := range f.errors {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		page.Form.Errors = append(page.Form.Errors, view.FieldError{Name: n, Message: text(f.errors[n])})
	}
	page.Note = "Removing an attachment from a record does not delete its file: another record may point at it, and Carrel keeps no index of what does. Changing the folder moves nothing: records already written keep their links, and only new attachments go to the new place."
	return page
}
