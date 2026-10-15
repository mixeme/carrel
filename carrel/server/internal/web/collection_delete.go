package web

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"golang.org/x/text/unicode/norm"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/discover"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// S-12.4: a calendar or an address book is deleted on the server with
// everything in it. The sheet says before anything is sent what points at it,
// that Carrel keeps no copy, and what a restore would not do; it offers the
// export first; and it takes the name typed as the confirmation, since what is
// destroyed cannot be listed. A folder of a file storage is deleted the same
// way (s-profile-12); the storage itself is not.

func collectionDeleteQuery(conn sealed.Connection, col sealed.Collection) url.Values {
	return url.Values{"a": {conn.ID}, "c": {col.Path}}
}

// collectionDeleteItem is the last item of a collection row's menu, or none
// for a file collection and one that is gone. It is not hidden on a read-only
// row: the right to delete lives on the home, not on the collection, and a
// refusal is shown when it happens (S-12.2).
func (s *Server) collectionDeleteItem(conn sealed.Connection, col sealed.Collection, back string) (view.MenuItem, bool) {
	if col.Gone || col.Feed || len(col.Kinds) == 0 {
		return view.MenuItem{}, false
	}
	q := collectionDeleteQuery(conn, col)
	if back != "" {
		q.Set("back", back)
	}
	item := view.MenuItem{Separator: true, Action: view.Action{
		ID: "collection.delete", Label: "Delete on the server…", Icon: "i-trash",
		Href:   s.postQuery("/app/collections/delete", q),
		Method: "GET", Danger: true, Enabled: true,
	}}
	if col.Has(view.KindFile) && storageRoot(conn, col) {
		item.Enabled, item.Href, item.Reason = false, "", storageRootReason
	}
	return item, true
}

// collectionExportExt is what a collection is handed over as whole.
func collectionExportExt(col sealed.Collection) string {
	if col.Has(view.KindContact) {
		return ".vcf"
	}
	return ".ics"
}

// collectionExport hands the collection over whole (S-19): an address book
// as .vcf, a calendar as .ics with every component it holds, so a task in a
// calendar of events is not left out.
func (s *Server) collectionExport(conn sealed.Connection, col sealed.Collection) view.Action {
	list := origin{conn: conn, col: col}.listPath(segmentOf(col.Kinds[0])) + "/export"
	q := url.Values{}
	if !col.Has(view.KindContact) {
		q.Set("whole", "1")
	}
	return view.Action{ID: "collection.export", Label: text("Export " + collectionExportExt(col)), Icon: "i-down",
		Href: s.postQuery(list, q), Method: "GET", Enabled: true}
}

// recordNoun names what the collection holds: its one kind, or records; a
// folder holds files (s-profile-19).
func recordNoun(col sealed.Collection) string {
	if col.Has(view.KindFile) {
		return "file"
	}
	if len(col.Kinds) == 1 {
		if spec, ok := recordKinds[segmentOf(col.Kinds[0])]; ok {
			return spec.noun
		}
	}
	return "record"
}

// collectionDeleteSheet is the sheet; count is the resources the server holds
// in it, or -1 when it did not say.
func (s *Server) collectionDeleteSheet(sess *session.Session, conn sealed.Connection, col sealed.Collection, count int, typed string, errText view.Text, back string) view.CollectionDeleteSheet {
	noun, label, host := collectionNoun(col), collectionLabel(col), connLabel(conn)
	what := []view.Run{plainRun("everything in it")}
	if count > 0 {
		what = append(allOf(count, recordNoun(col)), plainRun(" in it"))
	}
	q := collectionDeleteQuery(conn, col)
	if back != "" {
		q.Set("back", back)
	}
	action := "/app/collections/delete?" + q.Encode()
	sheet := view.CollectionDeleteSheet{
		Kind:  col.Kinds[0],
		Title: "Delete on the server",
		// The server, then the collection (s-profile-9, s-profile-19).
		Subject: text(host + " · " + label),
		Warning: join("The "+noun+" and ", what, " are deleted on "+host+". Carrel holds no copy — it reads and writes, it does not store — so there is nothing here to undo it with."),
		// What else points at it and stops working with it (S-12.4): the
		// person's published links (S-25), copy jobs (S-24) and davloom
		// devices whose scope names it (S-27).
		References: s.collectionReferences(sess, conn, col),
		Archive:    s.collectionArchive(sess, conn, col),
		Form: focused(s.plainForm(action, "POST",
			csrfField(sess),
			// The name is the field's sample (s-profile-9, s-profile-19).
			view.FormRow{Name: "name", Label: "Type the name to confirm", Kind: view.InputText, Value: text(typed),
				Placeholder: text(label), Error: errText, Required: true},
		), "name"),
		Actions: []view.Action{
			{ID: "collection.delete.confirm", Label: "Delete on the server", Href: s.route(action), Method: "POST", Danger: true, Enabled: true},
			s.sheetCancel(back, collectionList(conn, col)),
		},
	}
	// A calendar deleted on a server which schedules takes its meetings
	// off it, as a move does, and that server may tell their attendees of
	// a cancellation (the owner's decision of 12.10.2026, А12; Кл18).
	if col.Has(view.KindEvent) && conn.Scheduling == discover.SchedulingYes && count != 0 {
		sheet.Cancels = text(host + " may tell the attendees of its meetings that they are cancelled.")
	}
	if count == 0 {
		sheet.Warning = line(fmt.Sprintf("The %s is deleted on %s. It holds nothing now; Carrel keeps no copy of it either.", noun, host))
		sheet.ExportNote = "There is nothing in it to download."
		return sheet
	}
	if col.Has(view.KindFile) {
		// A folder is not exported: it is downloaded in Files (s-profile-12).
		sheet.Export = &view.Action{ID: "collection.open", Label: "Open in Files", Icon: "i-file",
			Href: s.route(fileHref(conn.ID, col.Path, "", nil, true)), Method: "GET", Enabled: true}
		sheet.ExportNote = "Download what you need from it there first."
		return sheet
	}
	export := s.collectionExport(conn, col)
	export.Label = text("Download " + collectionExportExt(col) + " first")
	sheet.Export = &export
	if count > 0 {
		sheet.ExportNote = text(plural(count, recordNoun(col)) + ", one file, straight to you")
	} else {
		sheet.ExportNote = "Everything in it, one file, straight to you"
	}
	return sheet
}

// countMembers is how many resources the collection holds, or -1.
func (s *Server) countMembers(ctx context.Context, conn sealed.Connection, col sealed.Collection) int {
	client, err := dav.NewClient(s.guard, conn.Server, conn.Login, conn.Password, conn.CertFingerprint)
	if err != nil {
		return -1
	}
	ms, err := members(ctx, client, col.Path)
	if err != nil {
		return -1
	}
	return len(ms)
}

// sameName is the confirmation: the name as shown, in the same letters and
// case; spaces around it and the Unicode form it was typed in do not count.
func sameName(typed, label string) bool {
	return norm.NFC.String(strings.TrimSpace(typed)) == norm.NFC.String(label)
}

func (s *Server) getCollectionDelete(w http.ResponseWriter, r *http.Request) {
	sess, _, conn, col, ok := s.openCollectionSheet(w, r)
	if !ok {
		return
	}
	if item, ok := s.collectionDeleteItem(conn, col, ""); !ok || !item.Enabled {
		s.notFound(w, r)
		return
	}
	count := s.countMembers(r.Context(), conn, col)
	sheet := s.collectionDeleteSheet(sess, conn, col, count, "", "", sheetBack(r.URL.Query().Get("back")))
	sheet.Under = s.collectionUnder(r)
	s.writePage(w, r, http.StatusOK, sheet)
}

// postCollectionDelete deletes a collection whose name was typed. A name that
// does not match sends nothing. A refusal leaves the collection and says why
// on the connections page; a server that no longer has it is taken at its word.
func (s *Server) postCollectionDelete(w http.ResponseWriter, r *http.Request) {
	sess, key, conn, col, ok := s.openCollectionSheet(w, r)
	if !ok {
		return
	}
	if item, ok := s.collectionDeleteItem(conn, col, ""); !ok || !item.Enabled {
		s.notFound(w, r)
		return
	}
	label := collectionLabel(col)
	typed := r.PostFormValue("name")
	if !sameName(typed, label) {
		errText := text("Type the name exactly as shown: " + label)
		if strings.TrimSpace(typed) == "" {
			errText = text("Type the name to confirm: " + label)
		}
		count := s.countMembers(r.Context(), conn, col)
		s.writePage(w, r, http.StatusOK, s.collectionDeleteSheet(sess, conn, col, count, typed, errText, sheetBack(r.URL.Query().Get("back"))))
		return
	}
	client, err := dav.NewClient(s.guard, conn.Server, conn.Login, conn.Password, conn.CertFingerprint)
	if err == nil {
		err = client.Delete(r.Context(), col.Path, "")
	}
	if err != nil && !dav.IsStatus(err, http.StatusNotFound) {
		s.writePage(w, r, http.StatusOK, s.connectionsPage(sess, connectionForm{}, davRefusal(refusalText{
			title:     "The server would not delete it",
			forbidden: "This account can read here but not delete",
			nothing:   label + " is as it was.",
		}, conn, "DELETE", col.Path, err)))
		return
	}
	// Its links, and the links of its records, open nothing now (Б41).
	s.linksGone(sess, user.Place{AccountID: conn.ID, Col: col.Path})
	if err := s.users.RemoveCollection(sess.AccountID(), key, conn.ID, col.Path); err != nil {
		s.writePage(w, r, http.StatusOK, s.connectionsPage(sess, connectionForm{}, &view.Notice{
			Role: view.NoticeAlert, Title: text("Deleted, but the list of collections still names it"),
			Body: text(label + " was deleted on " + connLabel(conn) + "; run re-discovery on its connection to take it out."),
		}))
		return
	}
	s.redirect(w, r, "/app/settings/connections")
}

// collectionReferences is what of the person points at the collection: the
// attachment folder, the published links and the davloom devices counted, the
// copy jobs by name (s-profile-9, s-profile-19).
func (s *Server) collectionReferences(sess *session.Session, conn sealed.Connection, col sealed.Collection) []view.Reference {
	refs := []view.Reference{}
	blob := s.catalogOf(sess)
	// The attachment folder in a folder of a storage (s-profile-19).
	if a := blob.Attachments; a != nil && a.AccountID == conn.ID && a.Path == col.Path {
		refs = append(refs, view.Reference{What: "Attachments", Then: "the attachment folder is in it; new attachments will have nowhere to go"})
	}
	// Links and devices are counted, a job is named (s-profile-9): a line
	// for each link and device is longer than it helps.
	links := 0
	for _, l := range s.users.PublishedLinks(sess.AccountID()) {
		if l.Target.AccountID == conn.ID && l.Target.Col == col.Path {
			links++
		}
	}
	if links > 0 {
		refs = append(refs, view.Reference{What: text(plural(links, "published link")), Then: "answers “not found” from that moment"})
	}
	what := "collection"
	if col.Has(view.KindFile) {
		what = "folder"
	}
	for _, j := range blob.BackupJobs {
		for _, src := range j.Sources {
			if src.AccountID == conn.ID && src.Path == col.Path {
				refs = append(refs, view.Reference{What: text("Backup job «" + j.Name + "»"), Then: text("copies this " + what + "; it will run one short and say so")})
				break
			}
		}
		if j.DestAccount == conn.ID && j.DestPath == col.Path {
			refs = append(refs, view.Reference{What: text("Backup job «" + j.Name + "»"), Then: "its copies are written here"})
		}
	}
	devices := 0
	for _, d := range s.users.Devices(sess.AccountID()) {
		if slices.ContainsFunc(d.ScopeCols, func(c store.DeviceScope) bool { return c.AccountID == conn.ID && c.Path == col.Path }) {
			devices++
		}
	}
	switch {
	case devices == 1:
		refs = append(refs, view.Reference{What: "1 device through davloom", Then: "it disappears for it at the next sync"})
	case devices > 1:
		refs = append(refs, view.Reference{What: text(fmt.Sprintf("%d devices through davloom", devices)), Then: "it disappears for them at the next sync"})
	}
	return refs
}

// collectionArchive is the archive line (s-profile-9, s-profile-19): when a
// backup job of Carrel last made a whole archive of the collection, and that
// a restore brings it back into a new one, not to this address.
//
// The day is monospaced and «new» bold, as the frames mark them.
func (s *Server) collectionArchive(sess *session.Session, conn sealed.Connection, col sealed.Collection) view.Runs {
	what, into := "records", "collection"
	if col.Has(view.KindFile) {
		what, into = "files", "folder"
	}
	var last time.Time
	copied := false
	runs := s.users.BackupRuns(sess.AccountID())
	for _, j := range s.catalogOf(sess).BackupJobs {
		if !slices.ContainsFunc(j.Sources, func(src sealed.Selection) bool { return src.AccountID == conn.ID && src.Path == col.Path }) {
			continue
		}
		copied = true
		if at := user.LastWholeRun(runs[j.ID]); at.After(last) {
			last = at
		}
	}
	restore := []view.Run{plainRun("Restoring brings the " + what + " back into a "), strongRun("new"), plainRun(" " + into + " — not into this one, and not to this address.")}
	switch {
	case !last.IsZero():
		return join("Last time it was in an archive: ", monoRun(last.In(s.timeFor(sess).tz).Format("2006-01-02")), ". ", restore)
	case copied:
		return join("A backup job copies it, but none has finished whole yet, so there is no archive of it here. ", restore)
	}
	if col.Has(view.KindFile) {
		return line("No backup job of Carrel copies it, so there is no archive of it here. A restore from any archive brings the files back into a new folder at a new address — not into this one.")
	}
	return line("No backup job of Carrel copies it, so there is no archive of it here. A restore from any archive brings the records back into a new " +
		collectionNoun(col) + " at a new address — not into this one, and not to this address.")
}
