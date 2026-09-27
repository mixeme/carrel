package web

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/text/unicode/norm"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// S-12.4: a calendar or an address book is deleted on the server with
// everything in it. The sheet says before anything is sent what points at it,
// that Carrel keeps no copy, and what a restore would not do; it offers the
// export first; and it takes the name typed as the confirmation, since what is
// destroyed cannot be listed. Files are not deleted here: a file collection is
// the root of a storage, and its folders go through op (S-17).

func collectionDeleteQuery(conn sealed.Connection, col sealed.Collection) url.Values {
	return url.Values{"a": {conn.ID}, "c": {col.Path}}
}

// collectionDeleteItem is the last item of a collection row's menu, or none
// for a file collection and one that is gone. It is not hidden on a read-only
// row: the right to delete lives on the home, not on the collection, and a
// refusal is shown when it happens (S-12.2).
func (s *Server) collectionDeleteItem(conn sealed.Connection, col sealed.Collection) (view.MenuItem, bool) {
	if col.Gone || len(col.Kinds) == 0 || col.Has(view.KindFile) {
		return view.MenuItem{}, false
	}
	return view.MenuItem{Separator: true, Action: view.Action{
		ID: "collection.delete", Label: "Delete on the server…",
		Href:   s.postQuery("/app/collections/delete", collectionDeleteQuery(conn, col)),
		Method: "GET", Danger: true, Enabled: true,
	}}, true
}

// recordNoun names what the collection holds: its one kind, or records.
func recordNoun(col sealed.Collection) string {
	if len(col.Kinds) == 1 {
		if spec, ok := recordKinds[segmentOf(col.Kinds[0])]; ok {
			return spec.noun
		}
	}
	return "record"
}

// collectionDeleteSheet is the sheet; count is the resources the server holds
// in it, or -1 when it did not say.
func (s *Server) collectionDeleteSheet(sess *session.Session, conn sealed.Connection, col sealed.Collection, count int, typed string, errText view.Text) view.CollectionDeleteSheet {
	noun, label, host := collectionNoun(col), collectionLabel(col), connLabel(conn)
	what := "everything in it"
	if count > 0 {
		what = "all " + plural(count, recordNoun(col)) + " in it"
	}
	action := "/app/collections/delete?" + collectionDeleteQuery(conn, col).Encode()
	sheet := view.CollectionDeleteSheet{
		Kind:    col.Kinds[0],
		Subject: text(label + " · " + host),
		Warning: text(fmt.Sprintf("The %s and %s are deleted on %s. Carrel holds no copy — it reads and writes, it does not store — so there is nothing here to undo it with.",
			noun, what, host)),
		// What else points at it and stops working with it (S-12.4): the
		// person's published links (S-25), copy jobs (S-24) and davloom
		// devices whose scope names it (S-27).
		References: s.collectionReferences(sess, conn, col),
		Archive: text("No backup job of Carrel copies it, so there is no archive of it here. A restore from any archive brings the records back into a new " +
			noun + " at a new address — not into this one, and not to this address."),
		Form: s.plainForm(action, "POST",
			csrfField(sess),
			view.FormRow{Name: "name", Label: "Type the name to confirm", Kind: view.InputText, Value: text(typed),
				Hint: text(label), Error: errText, Required: true},
		),
		Actions: []view.Action{{ID: "collection.delete.confirm", Label: "Delete on the server", Href: s.route(action), Method: "POST", Danger: true, Enabled: true}},
	}
	if count == 0 {
		sheet.Warning = text(fmt.Sprintf("The %s is deleted on %s. It holds nothing now; Carrel keeps no copy of it either.", noun, host))
		sheet.ExportNote = "There is nothing in it to download."
		return sheet
	}
	// The same stream as S-19. A calendar goes with every component it holds.
	list := origin{conn: conn, col: col}.listPath(segmentOf(col.Kinds[0])) + "/export"
	ext, q := ".vcf", url.Values{}
	if !col.Has(view.KindContact) {
		ext = ".ics"
		q.Set("whole", "1")
	}
	sheet.Export = &view.Action{ID: "collection.export", Label: text("Download " + ext + " first"), Icon: "i-down",
		Href: s.postQuery(list, q), Method: "GET", Enabled: true}
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
	count := s.countMembers(r.Context(), conn, col)
	s.writePage(w, r, http.StatusOK, s.collectionDeleteSheet(sess, conn, col, count, "", ""))
}

// postCollectionDelete deletes a collection whose name was typed. A name that
// does not match sends nothing. A refusal leaves the collection and says why
// on the connections page; a server that no longer has it is taken at its word.
func (s *Server) postCollectionDelete(w http.ResponseWriter, r *http.Request) {
	sess, key, conn, col, ok := s.openCollectionSheet(w, r)
	if !ok {
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
		s.writePage(w, r, http.StatusOK, s.collectionDeleteSheet(sess, conn, col, count, typed, errText))
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
	if err := s.users.RemoveCollection(sess.AccountID(), key, conn.ID, col.Path); err != nil {
		s.writePage(w, r, http.StatusOK, s.connectionsPage(sess, connectionForm{}, &view.Notice{
			Role: view.NoticeAlert, Title: text("Deleted, but the list of collections still names it"),
			Body: text(label + " was deleted on " + connLabel(conn) + "; run re-discovery on its connection to take it out."),
		}))
		return
	}
	s.redirect(w, r, "/app/settings/connections")
}

// collectionReferences names every published link, copy job and davloom
// device of the person that names the collection.
func (s *Server) collectionReferences(sess *session.Session, conn sealed.Connection, col sealed.Collection) []view.Text {
	refs := []view.Text{}
	for _, l := range s.users.PublishedLinks(sess.AccountID()) {
		if l.Target.AccountID == conn.ID && l.Target.Col == col.Path {
			refs = append(refs, text("Published link «"+l.Target.Title+"» — it stops opening"))
		}
	}
	blob := s.catalogOf(sess)
	for _, j := range blob.BackupJobs {
		for _, src := range j.Sources {
			if src.AccountID == conn.ID && src.Path == col.Path {
				refs = append(refs, text("Backup job «"+j.Name+"» — it copies the others and says this one is gone"))
				break
			}
		}
		if j.DestAccount == conn.ID && j.DestPath == col.Path {
			refs = append(refs, text("Backup job «"+j.Name+"» — its copies are written here"))
		}
	}
	for _, d := range s.users.Devices(sess.AccountID()) {
		for _, c := range d.ScopeCols {
			if c.AccountID == conn.ID && c.Path == col.Path {
				refs = append(refs, text("davloom device «"+d.Name+"» — the collection leaves it at its next sync"))
				break
			}
		}
	}
	return refs
}
