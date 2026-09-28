package web

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/conf"
	"carrel.dev/carrel/server/internal/provider/file"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// Settings → Attachments (S-20, U-6.12): the folder large attachments go to —
// a file collection from a list, the folder in it typed or chosen with
// Browse in the one tree of the product (C-6.7), a subfolder per month or
// not (attachments-1, attachments-2) — and the boundary between the two ways
// to keep a file, shown and not editable.

type attachForm struct {
	account, col string   // the file collection chosen
	folder       []string // the folder in it; made on first use
	folderText   string   // the folder as typed, shown back when refused
	monthly      bool     // a subfolder per month
	names        string   // how files are named: sealed.AttachNames…
	errors       map[string]string
}

// attachFormOf is the form as the catalog holds it, or at the folder a href
// of the tree names (Browse comes back with ?to=).
func (s *Server) attachFormOf(sess *session.Session, blob sealed.Blob, to string) attachForm {
	f := attachForm{names: attachNamesOf(blob)}
	if af := blob.Attachments; af != nil {
		f.account, f.col, f.folder, f.monthly = af.AccountID, af.Path, af.Folder, af.Monthly
	}
	if to != "" {
		if fc, ok := s.folderAddress(sess, to); ok && !fc.col.Feed {
			f.account, f.col, f.folder = fc.conn.ID, fc.col.Path, fc.segs
		}
	}
	return f
}

// attachNamesOf is how the person names attachments; by date until chosen.
func attachNamesOf(blob sealed.Blob) string {
	if blob.AttachNames == "" {
		return sealed.AttachNamesDated
	}
	return blob.AttachNames
}

// attachNames is the list of «Written as» (attachments-3), with the name each
// choice makes of sync-board.png attached to «Sync-up» of 10 August
// (attachments-1 draws the first).
var attachNames = []struct{ value, label, example string }{
	{sealed.AttachNamesDated, "Date, then the original name", "2026-08-10 sync-board.png"},
	{sealed.AttachNamesOriginal, "The original name", "sync-board.png"},
	{sealed.AttachNamesTitled, "Record title, then the original name", "Sync-up - sync-board.png"},
}

func (s *Server) getSettingsAttachments(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	blob := s.catalogOf(sess)
	s.writePage(w, r, http.StatusOK, s.attachmentsPage(r.Context(), sess, blob, s.attachFormOf(sess, blob, r.URL.Query().Get("to"))))
}

// postSettingsAttachments stores the folder: a file collection that can be
// written to, the folder in it, made when the first attachment goes there, a
// subfolder per month or not; and how the files are named. Nothing is stored
// unless all of it is right.
func (s *Server) postSettingsAttachments(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	blob := s.catalogOf(sess)
	f := attachForm{names: r.FormValue("name"), errors: map[string]string{}, folderText: r.FormValue("folder")}
	v := r.FormValue("monthly")
	f.monthly = v == "on" || v == "true" || v == "1"
	if f.names == "" {
		f.names = sealed.AttachNamesDated // a form sent without the field
	}
	if !slices.ContainsFunc(attachNames, func(n struct{ value, label, example string }) bool { return n.value == f.names }) {
		f.errors["name"] = "Choose one of the ways shown."
	}
	f.account, f.col, _ = strings.Cut(r.FormValue("collection"), "|")
	col, known := blob.Collection(f.account, f.col)
	switch {
	case !known || col.Gone || col.Feed || !col.Has(view.KindFile):
		f.errors["collection"] = "Choose a file collection you can write to."
	case col.ReadOnly:
		f.errors["collection"] = readOnlyReason + " Choose a collection you can write to."
	}
	segs, ok := relSegs(f.folderText)
	if !ok {
		f.errors["folder"] = "A folder is names separated by /, without . or .., nor a backslash."
	}
	f.folder = segs
	if len(f.errors) == 0 {
		key, _ := sess.DataKey()
		err := s.users.SetAttachmentFolder(sess.AccountID(), key, sealed.AttachmentFolder{AccountID: f.account, Path: f.col, Folder: segs, Monthly: f.monthly})
		if err == nil {
			err = s.users.SetAttachmentNames(sess.AccountID(), key, f.names)
		}
		if err == nil {
			s.redirect(w, r, "/app/settings/attachments")
			return
		}
		f.errors["collection"] = "The folder could not be stored. Open this page again and choose it once more."
	}
	s.writePage(w, r, http.StatusOK, s.attachmentsPage(r.Context(), sess, blob, f))
}

// attachFolderHref is the folder of the form in the files section — where
// Browse opens the tree — or "" when no collection is chosen.
func (s *Server) attachFolderHref(blob sealed.Blob, f attachForm) string {
	if _, ok := blob.Collection(f.account, f.col); !ok {
		return ""
	}
	return s.route(fileHref(f.account, f.col, "", f.folder, true))
}

// monthFolder is a subfolder «A subfolder per month» makes: «2026-08».
var monthFolder = regexp.MustCompile(`^\d{4}-\d{2}$`)

// folderNow is «In this folder now» (attachments-1): how many files the
// attachment folder holds and how large, the oldest of them, and its whole
// address — the one to mount without Carrel. The month subfolders count as
// the folder: the setting may have been on once. A folder not made yet holds
// nothing; one that cannot be read says so with a dash.
func (s *Server) folderNow(ctx context.Context, sess *session.Session, fc *fileCtx) view.FieldSet {
	p, _ := fc.path(fc.segs, true)
	files, oldest := "—", "—"
	entries, err := fc.files.List(ctx, p)
	if file.NotFound(err) {
		entries, err = nil, nil
	}
	if err == nil {
		var n int
		var size int64
		var first time.Time
		add := func(e file.Entry) {
			n, size = n+1, size+e.Size
			if !e.Modified.IsZero() && (first.IsZero() || e.Modified.Before(first)) {
				first = e.Modified
			}
		}
		for _, e := range entries {
			if !e.Dir {
				add(e)
				continue
			}
			if !monthFolder.MatchString(e.Name) {
				continue
			}
			inside, err := fc.files.List(ctx, e.Path)
			if err != nil {
				continue
			}
			for _, in := range inside {
				if !in.Dir {
					add(in)
				}
			}
		}
		files = fmt.Sprintf("%d · %s", n, formatSize(size))
		if n == 0 {
			files = "0"
		}
		if !first.IsZero() {
			oldest = first.In(s.timeFor(sess).tz).Format("2006-01-02")
		}
	}
	return view.FieldSet{Legend: "In this folder now", Rows: []view.FormRow{
		{Name: "now.files", Label: "Files", Kind: view.InputText, Value: text(files), ReadOnly: true},
		{Name: "now.oldest", Label: "Oldest", Kind: view.InputText, Value: text(oldest), ReadOnly: true},
		{Name: "now.path", Label: "Path", Kind: view.InputText, Value: text(fileURI(fc.conn.Server, p)), ReadOnly: true,
			Hint: "Carrel keeps no index of what points at these files. Nothing here deletes them: an attachment is removed from the record it hangs on, in that record."},
	}}
}

func (s *Server) attachmentsPage(ctx context.Context, sess *session.Session, blob sealed.Blob, f attachForm) view.AttachmentsPage {
	page := view.AttachmentsPage{Shell: s.buildShell(sess, "Attachments")}
	page.Rail = s.settingsRail(sess, "/app/settings/attachments")

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

	// Collection is a list of the file collections (attachments-2): a
	// read-only one is shown with its mark and not taken, so it is plain why.
	collection := view.FormRow{Name: "collection", Label: "Collection", Kind: view.InputSelect, Value: text(selValue(f.account, f.col)),
		Required: true, Error: text(f.errors["collection"]), Hint: "Any file collection you can write to."}
	for _, conn := range blob.Connections {
		for _, col := range blob.CollectionsOf(conn.ID) {
			if col.Gone || col.Feed || !col.Has(view.KindFile) {
				continue
			}
			o := view.Option{Value: selValue(conn.ID, col.Path), Label: text(string(stamp(connLabel(conn), collectionLabel(col)))),
				Colour: catalogColour(col), Selected: conn.ID == f.account && col.Path == f.col && !col.ReadOnly}
			if col.ReadOnly {
				o.Disabled, o.Reason = true, "Read-only"
			}
			collection.Options = append(collection.Options, o)
		}
	}
	shown := f.folderText
	if f.errors["folder"] == "" {
		shown = "/" + strings.Join(f.folder, "/")
	}
	browse := view.Action{ID: "attachments.browse", Label: "Browse", Method: "GET", Enabled: true,
		Href: s.postQuery("/app/files/picker", url.Values{"for": {"attachments"}, "to": {s.attachFolderHref(blob, f)}})}
	now := s.timeFor(sess).now
	thisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	folder := view.FieldSet{Legend: "Folder", Rows: []view.FormRow{
		csrfField(sess),
		collection,
		{Name: "folder", Label: "Folder in it", Kind: view.InputText, Value: text(shown), Error: text(f.errors["folder"]), Extra: &browse,
			Hint: "Created on first use. It stays an ordinary folder: openable in Files, mountable as a network drive, readable without Carrel."},
		{Name: "monthly", Label: "A subfolder per month", Kind: view.InputCheck, Value: text(boolStr(f.monthly)),
			Hint: text(thisMonth.AddDate(0, -1, 0).Format("2006-01") + ", " + thisMonth.Format("2006-01") +
				". Off means one flat folder — fine for a hundred files, not for ten thousand.")},
	}}
	written := view.FormRow{Name: "name", Label: "Written as", Kind: view.InputSelect, Value: text(f.names), Error: text(f.errors["name"])}
	for _, n := range attachNames {
		written.Options = append(written.Options, view.Option{Value: n.value, Label: text(n.label), Selected: n.value == f.names})
		if n.value == f.names {
			written.Hint = text(n.example + ". A name already taken is never overwritten: a counter is added.")
		}
	}
	name := view.FieldSet{Legend: "File name", Rows: []view.FormRow{written}}
	page.Form = view.Form{
		Action: s.route("/app/settings/attachments"), Method: "POST", Sets: []view.FieldSet{small, folder, name},
		Actions: []view.Action{{ID: "attachments.save", Label: "Save", Method: "POST", Enabled: true}},
	}
	if saved := s.savedFolder(sess, blob); saved != nil {
		page.Form.Sets = append(page.Form.Sets, s.folderNow(ctx, sess, saved))
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
	page.Note = "Changing the folder moves nothing. Records already written carry the old path and keep working; only new attachments go to the new place."
	return page
}
