package web

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/card"
	"carrel.dev/carrel/server/internal/provider/file"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/view"
)

// S-19: one wizard, three steps — source, preview, report. The preview is
// counted against the collection that was chosen, and choosing another one
// counts it again. An import always creates: a record whose identity the
// collection already holds is created under a new one and named in the
// report. A record of another kind in the same file is skipped and named,
// with where it belongs: half a file cannot go missing in silence.

// importDraft is a file taken apart, waiting for the word to write it
// (DraftPendingImport). There is one slot: a file chosen for another
// collection replaces it.
type importDraft struct {
	Kind    view.Kind
	Account string // where the wizard was opened
	Col     string
	Source  string // the name of the file, or of the folder it was read from
	To      string // the connection chosen to write into
	Into    string // and the collection there
	Records []importRecord
	Skipped []importSkip
	Note    string
}

// importRecord is one record of the file as its own document, with what its
// kind shows of it before it is written.
type importRecord struct {
	UID    string
	Title  string
	Detail string
	Doc    []byte
}

// importSkip is what the file held and this import does not write.
type importSkip struct {
	What   string // the component, or the kind of record it is
	Title  string
	Where  string // the section it belongs in; empty when Carrel keeps no such record
	Reason string
}

func loadImportDraft(c *collectionCtx) (*importDraft, bool) {
	data := c.sess.Draft(session.DraftPendingImport)
	if data == nil {
		return nil, false
	}
	var d importDraft
	if gob.NewDecoder(bytes.NewReader(data)).Decode(&d) != nil {
		return nil, false
	}
	if d.Kind != c.spec.kind || d.Account != c.conn.ID || d.Col != c.col.Path {
		return nil, false
	}
	return &d, true
}

func saveImportDraft(c *collectionCtx, d *importDraft) {
	var buf bytes.Buffer
	if gob.NewEncoder(&buf).Encode(d) == nil {
		c.sess.SetDraft(session.DraftPendingImport, buf.Bytes())
	}
}

func clearImportDraft(c *collectionCtx) {
	if _, ok := loadImportDraft(c); ok {
		c.sess.SetDraft(session.DraftPendingImport, nil)
	}
}

// importFormats are the file kinds one record kind is taken from.
func importFormats(k view.Kind) []view.Text {
	if k == view.KindContact {
		return []view.Text{".vcf"}
	}
	if k == view.KindNote {
		return []view.Text{".ics", ".md"}
	}
	return []view.Text{".ics"}
}

func importQuery(op, to, into string) url.Values {
	q := url.Values{}
	if op != "" {
		q.Set("op", op)
	}
	if to != "" {
		q.Set("to", to)
		q.Set("into", into)
	}
	return q
}

func (s *Server) importHref(c *collectionCtx, op, to, into string) string {
	return s.postQuery(c.listPath()+"/import", importQuery(op, to, into))
}

func (s *Server) importAction(c *collectionCtx) view.Action {
	return view.Action{ID: "collection.import", Label: "Import…", Icon: "i-up",
		Href: s.importHref(c, "", "", ""), Method: "GET", Enabled: true}
}

// importTarget is the collection this import writes into: the one the
// address names, or the one the wizard was opened on when it can be written.
func (s *Server) importTarget(c *collectionCtx, to, into string) *collectionCtx {
	if to != "" && into != "" {
		if dest, ok := s.collectionOf(c.sess, c.spec, to, into); ok && !dest.col.Gone && !dest.col.ReadOnly {
			return dest
		}
		return nil
	}
	if c.col.ReadOnly {
		return nil
	}
	return c
}

// importTargets are every collection of the kind, the read-only ones shown
// and closed with the reason (S-19: the destination is chosen, and the rail
// only says where the wizard was opened).
func (s *Server) importTargets(c *collectionCtx, dest *collectionCtx) view.TargetPicker {
	picker := view.TargetPicker{}
	for _, conn := range s.catalogOf(c.sess).Connections {
		for _, col := range s.catalogOf(c.sess).CollectionsOf(conn.ID) {
			if col.Gone || !col.Has(c.spec.kind) {
				continue
			}
			o := origin{conn: conn, col: col}
			opt := view.TargetOption{
				ID:    s.importHref(c, "", conn.ID, col.Path),
				Label: text(collectionLabel(col)), Stamp: o.stamp(), Colour: catalogColour(col),
			}
			if col.ReadOnly {
				opt.ReadOnly, opt.Reason = true, "Read-only"
			}
			if dest != nil && conn.ID == dest.conn.ID && col.Path == dest.col.Path {
				picker.Chosen = opt.ID
			}
			picker.Options = append(picker.Options, opt)
		}
	}
	return picker
}

// noteFolders is the second source of notes (S-19): a folder on a WebDAV
// server of this person, read through the guard like every outbound request.
// No file collection, no second source.
func (s *Server) noteFolders(c *collectionCtx) *view.TargetPicker {
	picker := view.TargetPicker{}
	blob := s.catalogOf(c.sess)
	for _, conn := range blob.Connections {
		for _, col := range blob.CollectionsOf(conn.ID) {
			if col.Gone || !col.Has(view.KindFile) {
				continue
			}
			picker.Options = append(picker.Options, view.TargetOption{
				ID:    s.postQuery(c.listPath()+"/import", url.Values{"op": {"folder"}, "folder": {conn.ID}, "path": {col.Path}}),
				Label: text(collectionLabel(col)), Stamp: origin{conn: conn, col: col}.stamp(),
			})
		}
	}
	if len(picker.Options) == 0 {
		return nil
	}
	return &picker
}

// importState is what one request of the wizard shows.
type importState struct {
	draft   *importDraft
	dest    *collectionCtx
	preview []view.ImportTable
	report  *view.ImportReport
	notices []view.Notice
}

func (s *Server) importPage(c *collectionCtx, st importState) view.ImportPage {
	page := view.ImportPage{
		Shell: s.buildShell(c.sess, "Import"), Kind: c.spec.kind,
		Step: view.ImportStepSource, Preview: st.preview, Report: st.report,
	}
	page.Rail = s.backRail(c.sess, s.catalogOf(c.sess), c.spec, s.listBack(c))
	page.Notices = append(page.Notices, st.notices...)
	page.Into = s.importTargets(c, st.dest)
	page.Source = view.ImportSource{
		Kinds: importFormats(c.spec.kind),
		Note:  text("Nothing here is overwritten. Every record in the file is created, and one whose identity the collection already holds is created under a new one."),
	}
	if c.spec.kind == view.KindNote {
		page.Source.Folder = s.noteFolders(c)
	}
	back := view.Action{ID: "import.cancel", Label: "Cancel", Href: s.route(c.listPath()), Method: "GET", Enabled: true}
	switch {
	case st.report != nil:
		page.Step = view.ImportStepReport
		done := view.Action{ID: "import.done", Label: "Open the collection", Method: "GET", Enabled: true,
			Href: s.route(c.listPath())}
		if st.dest != nil {
			done.Href = s.route(st.dest.listPath())
		}
		page.Actions = []view.Action{done, {ID: "import.again", Label: "Import another file",
			Href: s.importHref(c, "", "", ""), Method: "GET", Enabled: true}}
	case st.draft != nil:
		page.Step = view.ImportStepPreview
		page.Source.Note = text("From " + st.draft.Source + ". " + string(page.Source.Note))
		if st.draft.Note != "" {
			page.Source.Note = text(string(page.Source.Note) + " " + st.draft.Note)
		}
		run := view.Action{ID: "import.run", Label: "Import", Method: "POST", Enabled: true}
		switch {
		case st.dest == nil:
			run.Enabled, run.Reason = false, "Choose a collection that can be written."
		case len(st.draft.Records) == 0:
			run.Enabled, run.Reason = false, "This file holds no record of this kind."
		default:
			run.Href = s.importHref(c, "run", st.dest.conn.ID, st.dest.col.Path)
		}
		page.Actions = []view.Action{run, {ID: "import.cancel", Label: "Choose another file",
			Href: s.importHref(c, "cancel", "", ""), Method: "POST", Enabled: true}}
	default:
		to, into := targetOf(st.dest)
		choose := view.Action{ID: "import.choose", Label: "Read the file", Method: "POST", Enabled: true,
			Href: s.importHref(c, "", to, into)}
		if st.dest == nil {
			choose.Enabled, choose.Reason = false, "Choose a collection that can be written."
		}
		page.Actions = []view.Action{choose, back}
	}
	return page
}

// targetOf is the connection and collection of a destination, or two empty
// strings when none is chosen.
func targetOf(dest *collectionCtx) (string, string) {
	if dest == nil {
		return "", ""
	}
	return dest.conn.ID, dest.col.Path
}

// getKindImport is the wizard: the source step, or the preview of the file
// that is already in hand, counted against the collection now chosen.
// getKindImportEntry is the way into the import from a combined view, where
// no collection is chosen yet (s-contacts-1): the wizard opens on the first
// writable collection of the kind and asks "Into" for the rest. Nothing of
// the kind takes records — 404, and the bar says so before the click.
func (s *Server) getKindImportEntry(w http.ResponseWriter, r *http.Request, kind string) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	spec, known := recordKinds[kind]
	if !known {
		s.notFound(w, r)
		return
	}
	blob := s.catalogOf(sess)
	for _, conn := range blob.Connections {
		for _, col := range blob.CollectionsOf(conn.ID) {
			if col.Gone || col.ReadOnly || conn.Feed || !col.Has(spec.kind) {
				continue
			}
			if c, made := s.collectionOf(sess, spec, conn.ID, col.Path); made {
				s.redirect(w, r, c.listPath()+"/import")
				return
			}
		}
	}
	s.notFound(w, r)
}

// getKindExportEntry is the way into the export from a combined view: which
// collection to hand over. One is not guessed — the file would be somebody
// else's records (S-14).
func (s *Server) getKindExportEntry(w http.ResponseWriter, r *http.Request, kind string) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	spec, known := recordKinds[kind]
	if !known {
		s.notFound(w, r)
		return
	}
	blob := s.catalogOf(sess)
	sheet := view.ExportSheet{Kind: spec.kind, Actions: []view.Action{
		{ID: "export.cancel", Label: "Cancel", Href: s.route("/app/" + spec.segment), Method: "GET", Enabled: true},
	}}
	for _, conn := range blob.Connections {
		for _, col := range blob.CollectionsOf(conn.ID) {
			if col.Gone || !col.Has(spec.kind) {
				continue
			}
			o := origin{conn: conn, col: col}
			q := url.Values{}
			if spec.kind == view.KindEvent {
				q.Set("whole", "1") // a calendar goes whole from here: there is no week on this screen
			}
			sheet.Target.Options = append(sheet.Target.Options, view.TargetOption{
				ID: s.postQuery(o.listPath(spec.segment)+"/export", q), Label: text(collectionLabel(col)),
				Stamp: o.stamp(), Colour: catalogColour(col),
			})
		}
	}
	if len(sheet.Target.Options) == 0 {
		s.notFound(w, r)
		return
	}
	if WantsFragment(r) && FragmentTarget(r) == TargetSheet {
		_ = s.render.Fragment(w, r, http.StatusOK, TargetSheet, sheet, nil)
		return
	}
	s.writePage(w, r, http.StatusOK, sheet)
}

func (s *Server) getKindImport(w http.ResponseWriter, r *http.Request, kind string) {
	c, ok := s.openCollection(w, r, kind, false)
	if !ok {
		return
	}
	q := r.URL.Query()
	st := importState{dest: s.importTarget(c, q.Get("to"), q.Get("into"))}
	if d, has := loadImportDraft(c); has {
		st.draft = d
		if q.Get("to") == "" {
			st.dest = s.importTarget(c, d.To, d.Into)
		}
		st.draft.To, st.draft.Into = targetOf(st.dest)
		saveImportDraft(c, st.draft)
		st.preview, st.notices = s.importPreview(r.Context(), c, st.draft, st.dest)
	}
	s.writePage(w, r, http.StatusOK, s.importPage(c, st))
}

// postKindImport takes a file, drops one, or writes one. A body the CSRF
// check had to parse leaves temporary files behind; they go with the request.
func (s *Server) postKindImport(w http.ResponseWriter, r *http.Request, kind string) {
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	c, ok := s.openCollection(w, r, kind, false)
	if !ok {
		return
	}
	q := r.URL.Query()
	dest := s.importTarget(c, q.Get("to"), q.Get("into"))
	switch q.Get("op") {
	case "cancel":
		clearImportDraft(c)
		s.writePage(w, r, http.StatusOK, s.importPage(c, importState{dest: dest}))
	case "run":
		s.runImport(w, r, c, dest)
	case "folder":
		s.takeFolder(w, r, c, dest, q.Get("folder"), q.Get("path"))
	default:
		s.takeFile(w, r, c, dest)
	}
}

// takeFile reads the uploaded file into a draft. Nothing is written yet: the
// preview says what the import would do, and against which collection.
func (s *Server) takeFile(w http.ResponseWriter, r *http.Request, c *collectionCtx, dest *collectionCtx) {
	body, name, err := uploadedImport(r, s.cfg.ImportMaxBytes)
	if err != nil {
		s.writePage(w, r, http.StatusOK, s.importPage(c, importState{dest: dest, notices: []view.Notice{s.importRefusal(err)}}))
		return
	}
	d, err := s.readImport(c, name, body)
	if err != nil {
		s.writePage(w, r, http.StatusOK, s.importPage(c, importState{dest: dest, notices: []view.Notice{s.importRefusal(err)}}))
		return
	}
	d.To, d.Into = targetOf(dest)
	saveImportDraft(c, d)
	st := importState{draft: d, dest: dest}
	st.preview, st.notices = s.importPreview(r.Context(), c, d, dest)
	s.writePage(w, r, http.StatusOK, s.importPage(c, st))
}

// readImport takes a file apart by its kind. A calendar file holds records of
// three kinds at once, and the two this collection does not keep are named,
// not dropped (S-19).
func (s *Server) readImport(c *collectionCtx, name string, body []byte) (*importDraft, error) {
	d := &importDraft{Kind: c.spec.kind, Account: c.conn.ID, Col: c.col.Path, Source: name}
	if c.spec.kind == view.KindNote && isMarkdown(name) {
		obj, err := cal.ParseNote(name, body, time.Now())
		if err != nil {
			return nil, err
		}
		return d, d.add(c, obj)
	}
	if c.card != nil {
		objs, err := card.Split(body)
		if err != nil {
			return nil, err
		}
		if len(objs) > s.cfg.ImportMaxRecords {
			return nil, errTooManyRecords{len(objs), s.cfg.ImportMaxRecords}
		}
		for _, obj := range objs {
			if err := d.add(c, obj); err != nil {
				return nil, err
			}
		}
		return d, nil
	}
	objs, strays, err := cal.Split(body, time.Now())
	if err != nil {
		return nil, err
	}
	if len(objs) > s.cfg.ImportMaxRecords {
		return nil, errTooManyRecords{len(objs), s.cfg.ImportMaxRecords}
	}
	for _, stray := range strays {
		d.Skipped = append(d.Skipped, importSkip{
			What: stray.Component, Title: stray.Summary, Reason: stray.Reason,
		})
	}
	for _, obj := range objs {
		if k := cal.Kind(obj); k != c.spec.kind {
			d.Skipped = append(d.Skipped, importSkip{
				What: obj.MainComponent(), Title: titleOf(k, obj), Where: kindTitle(segmentOf(k)),
				Reason: "Not a " + c.spec.noun + ".",
			})
			continue
		}
		if err := d.add(c, obj); err != nil {
			return nil, err
		}
	}
	return d, nil
}

// add keeps one record of the file as its own document.
func (d *importDraft) add(c *collectionCtx, obj *model.Object) error {
	doc, err := c.body(obj)
	if err != nil {
		return err
	}
	d.Records = append(d.Records, importRecord{
		UID: uidOf(c, obj), Title: titleOf(c.spec.kind, obj), Detail: c.importDetail(obj), Doc: doc,
	})
	return nil
}

// importColumns are the columns one kind shows in the preview and the report:
// the wizard is one, and the preview of each kind is its own (S-19).
func importColumns(k view.Kind) []view.Text {
	switch k {
	case view.KindContact:
		return []view.Text{"Name", "Email", "Identity"}
	case view.KindEvent:
		return []view.Text{"Summary", "When", "Identity"}
	case view.KindTask:
		return []view.Text{"Summary", "Due", "Identity"}
	default:
		return []view.Text{"Title", "Date", "Identity"}
	}
}

// importDetail is the middle column: what tells one record of this kind from
// another before either is written.
func (c *collectionCtx) importDetail(obj *model.Object) string {
	switch c.spec.kind {
	case view.KindContact:
		return firstValue(obj, "EMAIL")
	case view.KindEvent:
		if t, err := cal.TimingOf(obj, c.tf.tz); err == nil {
			return c.whenOf(t)
		}
	case view.KindTask:
		if t, err := cal.DueOf(obj, c.tf.tz); err == nil {
			return c.whenOf(t)
		}
	default:
		if t := noteTime(obj, c.tf.tz); !t.IsZero() {
			return c.tf.date(t)
		}
	}
	return ""
}

func (c *collectionCtx) whenOf(t cal.Timing) string {
	switch {
	case t.Start.IsZero():
		return ""
	case t.AllDay:
		return c.tf.date(t.Start)
	}
	return c.tf.date(t.Start) + ", " + c.tf.clock(t.Start)
}

// titleOf is what one record is called in the preview and the report.
func titleOf(k view.Kind, obj *model.Object) string {
	switch k {
	case view.KindContact:
		return titleOr(contactName(obj), "Unnamed contact")
	case view.KindNote:
		return noteTitle(cal.Text(obj, "SUMMARY"), cal.Description(obj))
	case view.KindTask:
		return titleOr(cal.Text(obj, "SUMMARY"), "Untitled task")
	default:
		return titleOr(cal.Text(obj, "SUMMARY"), "Untitled event")
	}
}

// takeFolder reads a folder of Markdown files as notes: the second source
// notes have (S-19). The files are read through the guard, and the folder is
// read one level deep — a notebook is a folder, not a tree.
func (s *Server) takeFolder(w http.ResponseWriter, r *http.Request, c *collectionCtx, dest *collectionCtx, account, folder string) {
	fail := func(n view.Notice) {
		s.writePage(w, r, http.StatusOK, s.importPage(c, importState{dest: dest, notices: []view.Notice{n}}))
	}
	if c.spec.kind != view.KindNote {
		s.notFound(w, r)
		return
	}
	blob := s.catalogOf(c.sess)
	conn, hasConn := blob.Connection(account)
	col, hasCol := blob.Collection(account, folder)
	if !hasConn || !hasCol || col.Gone || !col.Has(view.KindFile) {
		s.notFound(w, r)
		return
	}
	prov, err := s.filesOf(conn)
	var entries []file.Entry
	if err == nil {
		entries, err = prov.List(r.Context(), col.Path)
	}
	if err != nil {
		fail(view.Notice{
			Role: view.NoticeAlert, Title: text(collectionLabel(col) + " is unavailable"),
			Body:    text("The folder could not be read, so nothing was taken from it."),
			Details: &view.Diagnostic{Summary: text("PROPFIND " + col.Path), Lines: []view.Text{text(connLabel(conn)), text(err.Error())}},
		})
		return
	}
	d := &importDraft{Kind: c.spec.kind, Account: c.conn.ID, Col: c.col.Path, Source: collectionLabel(col)}
	left := s.cfg.ImportMaxBytes
	for _, e := range entries {
		if e.Dir || !isMarkdown(e.Name) {
			continue
		}
		if len(d.Records) >= s.cfg.ImportMaxRecords {
			fail(s.importRefusal(errTooManyRecords{len(entries), s.cfg.ImportMaxRecords}))
			return
		}
		body, err := readAtMost(r.Context(), prov, e.Path, left)
		if err != nil {
			d.Skipped = append(d.Skipped, importSkip{What: e.Name, Reason: err.Error()})
			continue
		}
		left -= int64(len(body))
		obj, err := cal.ParseNote(e.Name, body, time.Now())
		if err != nil {
			d.Skipped = append(d.Skipped, importSkip{What: e.Name, Reason: err.Error()})
			continue
		}
		if err := d.add(c, obj); err != nil {
			d.Skipped = append(d.Skipped, importSkip{What: e.Name, Reason: err.Error()})
		}
	}
	if len(d.Records) == 0 && len(d.Skipped) == 0 {
		d.Note = "The folder holds no Markdown file."
	}
	d.To, d.Into = targetOf(dest)
	saveImportDraft(c, d)
	st := importState{draft: d, dest: dest}
	st.preview, st.notices = s.importPreview(r.Context(), c, d, dest)
	s.writePage(w, r, http.StatusOK, s.importPage(c, st))
}

func isMarkdown(name string) bool {
	ext := strings.ToLower(path.Ext(name))
	return ext == ".md" || ext == ".markdown"
}

// readAtMost reads one file of the folder within what is left of the import
// ceiling (C-12).
func readAtMost(ctx context.Context, prov *file.Provider, p string, left int64) ([]byte, error) {
	if left <= 0 {
		return nil, fmt.Errorf("the import is already at its ceiling")
	}
	rc, _, err := prov.Get(ctx, p, nil)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	body, err := io.ReadAll(io.LimitReader(rc, left+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > left {
		return nil, fmt.Errorf("the import is at its ceiling")
	}
	return body, nil
}

func (s *Server) filesOf(conn sealed.Connection) (*file.Provider, error) {
	client, err := dav.NewClient(s.guard, conn.Server, conn.Login, conn.Password, conn.CertFingerprint)
	if err != nil {
		return nil, err
	}
	return file.New(client)
}

// importPreview counts the file against the collection that was chosen
// (S-19): what will be created, what will be created under a new identity
// because this one is taken here, and what this import does not write.
func (s *Server) importPreview(ctx context.Context, c *collectionCtx, d *importDraft, dest *collectionCtx) ([]view.ImportTable, []view.Notice) {
	var notices []view.Notice
	taken := map[string]bool{}
	if dest != nil {
		var err error
		if taken, err = destUIDs(ctx, dest); err != nil {
			notices = append(notices, s.unavailableNotice(dest, s.importHref(c, "", dest.conn.ID, dest.col.Path), err))
		}
	}
	create := view.ImportTable{
		Rubric:  text("To create in " + destLabel(dest)),
		Columns: importColumns(c.spec.kind),
		Count:   len(d.Records),
	}
	for _, rec := range d.Records {
		state := "New"
		if taken[rec.UID] {
			state = "Taken here: this record is created under a new identity"
		}
		create.Rows = append(create.Rows, []view.Text{text(rec.Title), text(rec.Detail), text(state)})
	}
	tables := []view.ImportTable{create}
	if t, ok := skippedTable(d.Skipped); ok {
		tables = append(tables, t)
	}
	return tables, notices
}

func destLabel(dest *collectionCtx) string {
	if dest == nil {
		return "the collection"
	}
	return collectionLabel(dest.col)
}

// skippedTable names what the file held and the import does not write, and
// where it belongs (S-19).
func skippedTable(skipped []importSkip) (view.ImportTable, bool) {
	if len(skipped) == 0 {
		return view.ImportTable{}, false
	}
	t := view.ImportTable{
		Rubric:  text("Skipped"),
		Columns: []view.Text{text("What"), text("Why"), text("Where it belongs")},
		Count:   len(skipped),
	}
	for _, sk := range skipped {
		what := sk.What
		if sk.Title != "" {
			what += " · " + sk.Title
		}
		t.Rows = append(t.Rows, []view.Text{text(what), text(sk.Reason), text(sk.Where)})
	}
	return t, true
}

// destUIDs are the identities the destination already holds. A collection
// holds one resource per UID whatever its component (RFC 4791 §5.3.2, RFC
// 6352 §6.3.2), so a calendar is read whole, not by the kind alone.
func destUIDs(ctx context.Context, dest *collectionCtx) (map[string]bool, error) {
	out := map[string]bool{}
	if dest.card != nil {
		objs, err := dest.card.Query(ctx, dest.col.Path)
		if err != nil {
			return nil, err
		}
		for _, obj := range objs {
			out[card.UID(obj)] = true
		}
		return out, nil
	}
	for _, p := range calendarParts {
		objs, _, err := dest.cal.Query(ctx, dest.col.Path, p.component, nil, nil)
		if err != nil {
			return nil, err
		}
		for _, obj := range objs {
			out[cal.UID(obj)] = true
		}
	}
	return out, nil
}

// runImport writes the draft. Every record is created: an identity the
// collection already holds is replaced by a fresh one before the write, and
// the row says so (S-19). Nothing is overwritten, and nothing is deleted.
func (s *Server) runImport(w http.ResponseWriter, r *http.Request, c *collectionCtx, dest *collectionCtx) {
	d, has := loadImportDraft(c)
	if !has {
		s.writePage(w, r, http.StatusOK, s.importPage(c, importState{dest: dest, notices: []view.Notice{{
			Role: view.NoticeAlert, Title: "There is no file to import",
			Body: "The file is no longer in hand — the session may have ended. Choose it again.",
		}}}))
		return
	}
	if dest == nil {
		st := importState{draft: d, dest: nil}
		st.preview, st.notices = s.importPreview(r.Context(), c, d, nil)
		s.writePage(w, r, http.StatusOK, s.importPage(c, st))
		return
	}
	ctx := r.Context()
	taken, err := destUIDs(ctx, dest)
	if err != nil {
		st := importState{draft: d, dest: dest}
		st.preview = []view.ImportTable{}
		st.notices = []view.Notice{s.unavailableNotice(dest, s.importHref(c, "", dest.conn.ID, dest.col.Path), err)}
		s.writePage(w, r, http.StatusOK, s.importPage(c, st))
		return
	}

	report := &view.ImportReport{Skipped: len(d.Skipped)}
	written := view.ImportTable{Rubric: text("Created in " + collectionLabel(dest.col)), Columns: importColumns(c.spec.kind)}
	refused := view.ImportTable{Rubric: text("Not created"), Columns: []view.Text{text("What"), text("What the server answered")}}
	for _, rec := range d.Records {
		obj, err := dest.parse("", "", rec.Doc)
		uid, renamed, note := rec.UID, false, "Created"
		if err == nil && (taken[uid] || uid == "") {
			var fresh string
			if fresh, err = model.NewUID(); err == nil {
				if obj, err = setUID(dest, obj, fresh); err == nil {
					uid, renamed = fresh, true
					note = "Created under " + fresh + ": the collection already held the identity of this record"
				}
			}
		}
		if err == nil {
			err = s.createImported(ctx, dest, obj, uid)
		}
		if err != nil {
			refused.Rows = append(refused.Rows, []view.Text{text(rec.Title), text(err.Error())})
			continue
		}
		taken[uid] = true
		report.Created++
		if renamed {
			report.Renamed++
		}
		written.Rows = append(written.Rows, []view.Text{text(rec.Title), text(rec.Detail), text(note)})
	}
	written.Count = report.Created
	report.Rows = append(report.Rows, written)
	if len(refused.Rows) > 0 {
		refused.Count = len(refused.Rows)
		report.Rows = append(report.Rows, refused)
	}
	if t, ok := skippedTable(d.Skipped); ok {
		report.Rows = append(report.Rows, t)
	}
	report.Note = text(importNote(report, len(refused.Rows), d.Source))

	clearImportDraft(c)
	s.writePage(w, r, http.StatusOK, s.importPage(c, importState{dest: dest, report: report}))
}

func importNote(report *view.ImportReport, refused int, source string) string {
	parts := []string{fmt.Sprintf("%s created from %s", plural(report.Created, "record"), source)}
	if report.Renamed > 0 {
		parts = append(parts, fmt.Sprintf("%s under a new identity", plural(report.Renamed, "record")))
	}
	if report.Skipped > 0 {
		parts = append(parts, fmt.Sprintf("%s skipped", plural(report.Skipped, "record")))
	}
	if refused > 0 {
		parts = append(parts, fmt.Sprintf("%s refused by the server", plural(refused, "record")))
	}
	return strings.Join(parts, " · ") + ". Nothing there was overwritten."
}

// setUID gives an imported record a fresh identity, by the rules of its kind.
func setUID(c *collectionCtx, obj *model.Object, uid string) (*model.Object, error) {
	if c.card != nil {
		return card.SetUID(obj, uid)
	}
	return cal.SetUID(obj, uid)
}

// createImported writes one record with If-None-Match (S-19: an import always
// creates). A name taken there — another record under a name of this shape —
// becomes {uid}-2 once, as a move does.
func (s *Server) createImported(ctx context.Context, dest *collectionCtx, obj *model.Object, uid string) error {
	var err error
	for _, name := range []string{fileStem(uid) + dest.spec.ext, fileStem(uid) + "-2" + dest.spec.ext} {
		dst := strings.TrimSuffix(dest.col.Path, "/") + "/" + name
		obj.Path = dst
		if err = dest.create(ctx, dst, obj); err == nil {
			dest.noteWrite(dst, obj)
			return nil
		}
		if !dav.IsStatus(err, http.StatusPreconditionFailed) {
			break
		}
	}
	return err
}

// errTooManyRecords is a file over the ceiling of one import (C-12). Half a
// file is not taken: the whole of it is refused, and it can be split.
type errTooManyRecords struct{ have, max int }

func (e errTooManyRecords) Error() string {
	return fmt.Sprintf("the file holds %d records; an import takes %d", e.have, e.max)
}

var errNoImportFile = errors.New("no file in the form")

// uploadedImport is the file part of the form. A body the middleware parsed
// for its _csrf is read from that form; one sent with X-CSRF-Token is read as
// it arrives. Either way no more than the ceiling is kept.
func uploadedImport(r *http.Request, max int64) ([]byte, string, error) {
	part, name, err := importPart(r)
	if err != nil {
		return nil, "", err
	}
	defer part.Close()
	body, err := io.ReadAll(io.LimitReader(part, max+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(body)) > max {
		return nil, "", &http.MaxBytesError{Limit: max}
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, "", errNoImportFile
	}
	return body, name, nil
}

func importPart(r *http.Request) (io.ReadCloser, string, error) {
	if r.MultipartForm != nil {
		files := r.MultipartForm.File["file"]
		if len(files) == 0 {
			return nil, "", errNoImportFile
		}
		f, err := files[0].Open()
		return f, path.Base(files[0].Filename), err
	}
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, "", errNoImportFile
	}
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			return nil, "", errNoImportFile
		}
		if err != nil {
			return nil, "", err
		}
		if part.FormName() == "file" && part.FileName() != "" {
			return part, path.Base(part.FileName()), nil
		}
	}
}

// importRefusal says why a file was not taken. Nothing was written, and no
// draft is left.
func (s *Server) importRefusal(err error) view.Notice {
	n := view.Notice{Role: view.NoticeAlert, Title: "Nothing was imported"}
	var many errTooManyRecords
	switch {
	case errors.Is(err, errNoImportFile):
		n.Title, n.Body = "No file was chosen", "Choose the file to import and send the form again."
	case tooLarge(err):
		n.Body = text("A file of up to " + string(formatSize(s.cfg.ImportMaxBytes)) + " can be imported.")
	case errors.As(err, &many):
		n.Body = text(strings.ToUpper(err.Error()[:1]) + err.Error()[1:] + ". Split it and import the parts.")
	default:
		n.Body = "Carrel could not read this file."
		n.Details = &view.Diagnostic{Summary: "Import", Lines: []view.Text{text(err.Error())}}
	}
	return n
}
