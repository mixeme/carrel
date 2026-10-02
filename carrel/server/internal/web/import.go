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
	"slices"
	"strconv"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/card"
	"carrel.dev/carrel/server/internal/provider/file"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/render"
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
	// What the head of the preview says of the source (s-io-5…8): its
	// size, the format it was read as, and for a folder how many files it
	// held.
	Size   int64
	Format string
	Files  int
}

// importRecord is one record of the file as its own document, with what its
// kind shows of it before it is written.
type importRecord struct {
	UID    string
	Title  string
	Detail string
	Doc    []byte
	// File is the file of a folder or a zip the record was read from; empty
	// for one file (the draft's Source)
	File string
	// Ready is what its row says after «Ready» (s-io-6, s-io-7): «the series
	// as one record»; empty — Ready alone
	Ready string
}

// importSkip is what the file held and this import does not write.
type importSkip struct {
	What   string // the component, or the kind of record it is
	Title  string
	Where  string // the section it belongs in; empty when Carrel keeps no such record
	Reason string
	Failed bool // could not be read at all: counted «with errors», not «skipped»
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
		return []view.Text{".ics", ".md", ".zip"}
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
	return view.Action{ID: "collection.import", Label: "Import", Icon: "i-up",
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

// newCollectionLabels are the last lines of «Into» (s-io-10…13).
var newCollectionLabels = map[view.Kind]view.Text{
	view.KindContact: "New address book", view.KindEvent: "New calendar",
	view.KindTask: "New task list", view.KindNote: "New notebook",
}

// importTargets are every collection of the kind, the read-only ones shown
// and closed with the reason (S-19: the destination is chosen, and the rail
// only says where the wizard was opened); under a line, a new collection of
// the kind, made on its sheet, which brings the wizard back with it chosen.
func (s *Server) importTargets(c *collectionCtx, dest *collectionCtx) view.TargetPicker {
	picker := view.TargetPicker{New: &view.Action{
		ID: "import.collection.new", Label: newCollectionLabels[c.spec.kind], Icon: "i-plus", Method: "GET", Enabled: true,
		Href: s.postQuery("/app/collections/new", url.Values{"kind": {string(c.spec.kind)}, "back": {c.listPath() + "/import"}}),
	}}
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
			// A choice is a link to the wizard with it chosen (U-8,
			// s-io-10…13); a read-only one is shown and leads nowhere.
			opt.Href = opt.ID
			if col.ReadOnly {
				opt.ReadOnly, opt.Reason, opt.Href = true, "Read-only", ""
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
// The folder is chosen in a tree (s-io-4) — the storages, and the folders of
// the one opened, level by level, down to the folder chosen: each node a GET
// that comes back with it chosen —, and «Preview folder» under it takes the
// chosen one. No file collection, no second source.
func (s *Server) noteFolders(c *collectionCtx, st importState) (*view.FileTree, *view.Action) {
	tree := &view.FileTree{}
	to, into := targetOf(st.dest)
	blob := s.catalogOf(c.sess)
	preview := &view.Action{ID: "import.folder", Label: "Preview folder", Method: "POST", Primary: true,
		Reason: "Choose a folder in the tree first."}
	link := func(acc, colPath string, segs []string, op string) string {
		q := importQuery(op, to, into)
		q.Set("folder", acc)
		q.Set("path", colPath)
		if len(segs) > 0 {
			q.Set("in", strings.Join(segs, "/"))
		}
		return s.postQuery(c.listPath()+"/import", q)
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.ProgressSourceTimeout)
	defer cancel()
	for _, conn := range blob.Connections {
		for _, col := range blob.CollectionsOf(conn.ID) {
			if col.Gone || !col.Has(view.KindFile) {
				continue
			}
			node := view.FileTreeNode{ID: link(conn.ID, col.Path, nil, ""), Label: collectionLabel(col), Stamp: origin{conn: conn, col: col}.stamp()}
			node.Href = node.ID // a node chooses by a link, as the trees of Files (U-8)
			if conn.ID == st.folderAcc && col.Path == st.folderPath {
				if prov, err := s.filesOf(conn); err == nil {
					node.Expanded = true
					node.Children = s.folderLevel(ctx, prov, col.Path, nil, st.folderIn, func(segs []string) string { return link(conn.ID, col.Path, segs, "") })
				}
				tree.Chosen = link(conn.ID, col.Path, st.folderIn, "")
				preview.Href, preview.Enabled, preview.Reason = link(conn.ID, col.Path, st.folderIn, "folder"), true, ""
			}
			tree.Nodes = append(tree.Nodes, node)
		}
	}
	if len(tree.Nodes) == 0 {
		return nil, nil
	}
	return tree, preview
}

// folderLevel is the folders of base at at, the one on the way to chosen
// opened with its own: one PROPFIND a level, as the tree of Move.
func (s *Server) folderLevel(ctx context.Context, prov *file.Provider, base string, at, chosen []string, link func([]string) string) []view.FileTreeNode {
	entries, err := prov.List(ctx, folderUnder(base, at))
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if e.Dir {
			dirs = append(dirs, e.Name)
		}
	}
	slices.SortFunc(dirs, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	if len(dirs) > s.cfg.FilesMaxEntries {
		dirs = dirs[:s.cfg.FilesMaxEntries]
	}
	var out []view.FileTreeNode
	for _, name := range dirs {
		segs := append(slices.Clone(at), name)
		node := view.FileTreeNode{ID: link(segs), Label: name}
		node.Href = node.ID
		if len(chosen) > len(at) && chosen[len(at)] == name {
			node.Expanded, node.Children = true, s.folderLevel(ctx, prov, base, segs, chosen, link)
		}
		out = append(out, node)
	}
	return out
}

// folderUnder is the folder segs below the collection's base.
func folderUnder(base string, segs []string) string {
	if len(segs) == 0 {
		return base
	}
	return strings.TrimSuffix(base, "/") + "/" + strings.Join(segs, "/") + "/"
}

// folderChoice reads the folder chosen in the tree of notes from a query:
// the storage, its path, and ?in= — the folder below its root, segments
// that stay inside it. One that would leave it chooses nothing.
func folderChoice(q url.Values) (acc, colPath string, in []string) {
	in, ok := relSegs(q.Get("in"))
	if !ok {
		return "", "", nil
	}
	return q.Get("folder"), q.Get("path"), in
}

// importState is what one request of the wizard shows.
type importState struct {
	draft   *importDraft
	dest    *collectionCtx
	preview []view.ImportTable
	taken   map[string]bool // the identities the destination holds, for the counts
	report  *view.ImportReport
	notices []view.Notice
	// entry: the wizard was opened from a combined view, and nothing is
	// chosen under «Into» until the person chooses it (C-5, U-6.9).
	entry bool
	// folderAcc, folderPath and folderIn are the folder chosen in the tree
	// of notes (s-io-4) — the storage and the folder below its root —,
	// before «Preview folder» takes it.
	folderAcc, folderPath string
	folderIn              []string
}

// importSteps are the words of the wizard's steps line (s-io-3…9), in the
// order of view.ImportStep.
var importSteps = []view.Text{"1 · Source", "2 · Preview", "3 · Report"}

// importHints are the line under the file field (s-io-3, s-io-4): what the
// file may be, and where the other kind's extension goes instead.
var importHints = map[view.Kind]view.Runs{
	view.KindContact: join("vCard 2.1, 3.0 and 4.0. One file holds any number of cards; the calendar and the list of tasks take ", monoRun(".ics"), " here instead."),
	view.KindEvent:   join("iCalendar 2.0. One file holds any number of events; tasks and notes in it are named and left out. An address book takes ", monoRun(".vcf"), " here instead."),
	view.KindTask:    join("iCalendar 2.0. One file holds any number of tasks; events and notes in it are named and left out. An address book takes ", monoRun(".vcf"), " here instead."),
}

// importFolderHint is the line under the tree of folders (s-io-4).
var importFolderHint = join("Every ", monoRun(".md"), " directly in that folder. Subfolders are not walked, nothing on the server is moved or deleted — the files stay where they were.")

// importNotes are the message of the first step and of the preview, by kind
// (s-io-3…8).
var (
	importFirstNote = line("Nothing is written until the preview has been read: the next step counts what was parsed and names what failed.")
	importNoteFirst = join("Front matter is read when present: title, date, tags, UID and ", monoRun("X-"), "properties exported earlier. A file without it becomes a note dated by its own timestamp.")
	importNotes     = map[view.Kind]view.Runs{
		view.KindContact: line("Import always creates. A UID that already exists gets a new one and is listed in the report."),
		view.KindEvent:   line("Import always creates. A repeating event keeps its rule: the series is one record, not fifty-two."),
		view.KindTask:    line("STATUS, PERCENT-COMPLETE and PRIORITY come across as they are: a task that was half done arrives half done."),
		view.KindNote:    line("Markdown is stored as it was written — nothing is reformatted — and front matter Carrel exported earlier comes back: UID, tags, X-properties."),
	}
)

func (s *Server) importPage(c *collectionCtx, st importState) view.ImportPage {
	page := view.ImportPage{
		Shell: s.buildShell(c.sess, "Import"), Kind: c.spec.kind,
		Step: view.ImportStepSource, Steps: importSteps, Preview: st.preview, Report: st.report,
	}
	page.Rail = s.recordRail(c, s.listBack(c))
	page.Notices = append(page.Notices, st.notices...)
	page.Into = s.importTargets(c, st.dest)
	page.Source = view.ImportSource{Kinds: importFormats(c.spec.kind), Hint: importHints[c.spec.kind], Note: importFirstNote}
	if c.spec.kind == view.KindNote {
		page.Source.Note = importNoteFirst
		page.Source.Folder, page.Source.Preview = s.noteFolders(c, st)
		if page.Source.Folder != nil {
			page.Source.FolderHint = importFolderHint
		}
	}
	switch {
	case st.report != nil:
		// The report's one door, to the list the records went into (s-io-9).
		page.Step = view.ImportStepReport
		done := view.Action{ID: "import.done", Label: "Back to list", Method: "GET", Primary: true, Enabled: true,
			Href: s.route(c.listPath())}
		if st.dest != nil {
			done.Href = s.route(st.dest.listPath())
			page.Subtitle = line(string(st.dest.stamp()))
		}
		page.Actions = []view.Action{done}
	case st.draft != nil:
		page.Step = view.ImportStepPreview
		page.Subtitle = importSubtitle(st.draft)
		page.Counts = importCounts(st.draft, st.taken)
		page.Source.Note = importNotes[c.spec.kind]
		if st.draft.Note != "" {
			page.Source.Note = join(page.Source.Note, " "+st.draft.Note)
		}
		// «Import 122 contacts» (s-io-5…8): what the press creates.
		run := view.Action{ID: "import.run", Label: text("Import " + plural(len(st.draft.Records), c.spec.noun)), Method: "POST", Primary: true, Enabled: true}
		switch {
		case st.dest == nil:
			run.Enabled, run.Reason = false, "Choose a collection that can be written."
		case len(st.draft.Records) == 0:
			run.Enabled, run.Reason = false, "This file holds no record of this kind."
		default:
			run.Href = s.importHref(c, "run", st.dest.conn.ID, st.dest.col.Path)
		}
		// The file waits in the session: leaving drops it, so Cancel is a
		// POST here and not the way back of the first step.
		page.Actions = []view.Action{run, {ID: "import.cancel", Label: "Cancel",
			Href: s.importHref(c, "cancel", "", ""), Method: "POST", Enabled: true}}
	default:
		to, into := targetOf(st.dest)
		// The first step has one button; the rail is the way out (s-io-3,
		// s-io-4).
		// Without scripts the file goes in a form of its own, its field
		// named «file» (U-8; importPart reads that part).
		choose := view.Action{ID: "import.choose", Label: "Preview file", Method: "POST", Primary: true, Enabled: true,
			Href: s.importHref(c, "", to, into), File: "file"}
		if st.dest == nil {
			choose.Enabled, choose.Reason, choose.Href = false, "Choose a collection that can be written.", ""
			if st.entry {
				choose.Reason = "Choose under «Into» where the records go."
			}
		}
		page.Actions = []view.Action{choose}
	}
	return page
}

// importSubtitle is the head of the preview (s-io-5…8): the file
// monospaced, its size and the format it was read as; a folder's name, the
// size of what was read and how many files it held.
func importSubtitle(d *importDraft) view.Runs {
	parts := []any{monoRun(d.Source), " · " + roughSize(d.Size)}
	if d.Files > 0 {
		parts = append(parts, " · "+plural(d.Files, "file"))
	} else if d.Format != "" {
		parts = append(parts, " · "+d.Format)
	}
	return join(parts...)
}

// importCounts are the counts over the preview (s-io-5…8): what is ready,
// what cannot be read, what will take a new UID here, what is left out.
// Ready is always said; the others when there are any.
func importCounts(d *importDraft, taken map[string]bool) []view.ImportCount {
	failed, skipped, collisions := 0, 0, 0
	for _, sk := range d.Skipped {
		if sk.Failed {
			failed++
		} else {
			skipped++
		}
	}
	for _, rec := range d.Records {
		if taken[rec.UID] {
			collisions++
		}
	}
	out := []view.ImportCount{{Count: len(d.Records), Label: "ready"}}
	if failed > 0 {
		out = append(out, view.ImportCount{Count: failed, Label: "with errors", Error: true})
	}
	if collisions > 0 {
		label := "UID collisions"
		if collisions == 1 {
			label = "UID collision"
		}
		out = append(out, view.ImportCount{Count: collisions, Label: text(label)})
	}
	if skipped > 0 {
		out = append(out, view.ImportCount{Count: skipped, Label: "skipped"})
	}
	return out
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
// no collection is chosen yet (s-contacts-1): the wizard's first step, with
// nothing chosen under «Into» — it asks, and a choice there is the wizard of
// the first writable collection with that one chosen. It used to open on
// that first collection chosen in silence. Nothing of the kind takes
// records — 404, and the bar says so before the click.
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
				page := s.importPage(c, importState{entry: true})
				page.Rail = s.combinedBackRail(c)
				s.writePage(w, r, http.StatusOK, page)
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
			href := s.postQuery(o.listPath(spec.segment)+"/export", q)
			sheet.Target.Options = append(sheet.Target.Options, view.TargetOption{
				ID: href, Href: href, Label: text(collectionLabel(col)),
				Stamp: o.stamp(), Colour: catalogColour(col),
			})
		}
	}
	if len(sheet.Target.Options) == 0 {
		s.notFound(w, r)
		return
	}
	if render.WantsFragment(r) && render.FragmentTarget(r) == render.TargetSheet {
		_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, sheet, nil)
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
	st.folderAcc, st.folderPath, st.folderIn = folderChoice(q)
	if d, has := loadImportDraft(c); has {
		st.draft = d
		if q.Get("to") == "" {
			st.dest = s.importTarget(c, d.To, d.Into)
		}
		st.draft.To, st.draft.Into = targetOf(st.dest)
		saveImportDraft(c, st.draft)
		st.preview, st.taken, st.notices = s.importPreview(r.Context(), c, st.draft, st.dest)
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
		s.redirect(w, r, c.listPath())
	case "run":
		s.runImport(w, r, c, dest)
	case "folder":
		acc, colPath, in := folderChoice(q)
		s.takeFolder(w, r, c, dest, acc, colPath, in)
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
	st.preview, st.taken, st.notices = s.importPreview(r.Context(), c, d, dest)
	s.writePage(w, r, http.StatusOK, s.importPage(c, st))
}

// readImport takes a file apart by its kind. A calendar file holds records of
// three kinds at once, and the two this collection does not keep are named,
// not dropped (S-19).
func (s *Server) readImport(c *collectionCtx, name string, body []byte) (*importDraft, error) {
	if c.spec.kind == view.KindNote && isZip(body) {
		return s.readZipNotes(c, name, body)
	}
	d := &importDraft{Kind: c.spec.kind, Account: c.conn.ID, Col: c.col.Path, Source: name, Size: int64(len(body))}
	if c.spec.kind == view.KindNote && isMarkdown(name) {
		d.Format = "Markdown"
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
		d.Format = vcardFormat(objs)
		for _, obj := range objs {
			if err := d.add(c, obj); err != nil {
				return nil, err
			}
		}
		return d, nil
	}
	d.Format = icalFormat(body)
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
				What: obj.MainComponent(), Title: titleOf(k, obj), Where: kindHolders[k][0],
				Reason: kindHolders[c.spec.kind][0] + " takes " + kindHolders[c.spec.kind][1],
			})
			continue
		}
		if err := d.add(c, obj); err != nil {
			return nil, err
		}
	}
	return d, nil
}

// vcardFormat is the versions of the cards of a file, as the head of the
// preview names them: «vCard 3.0», «vCard 3.0 and 4.0».
func vcardFormat(objs []*model.Object) string {
	var seen []string
	for _, o := range objs {
		v := "3.0"
		if vs := o.PropText("VERSION"); len(vs) > 0 && strings.TrimSpace(vs[0]) != "" {
			v = strings.TrimSpace(vs[0])
		}
		if !slices.Contains(seen, v) {
			seen = append(seen, v)
		}
	}
	slices.Sort(seen)
	switch len(seen) {
	case 0:
		return "vCard"
	case 1:
		return "vCard " + seen[0]
	}
	return "vCard " + strings.Join(seen[:len(seen)-1], ", ") + " and " + seen[len(seen)-1]
}

// icalFormat is the version a calendar file declares: «iCalendar 2.0».
func icalFormat(body []byte) string {
	for _, l := range strings.SplitN(string(body), "\n", 16) {
		if v, ok := strings.CutPrefix(strings.TrimSpace(l), "VERSION:"); ok {
			return "iCalendar " + strings.TrimSpace(v)
		}
	}
	return "iCalendar"
}

// add keeps one record of the file as its own document.
func (d *importDraft) add(c *collectionCtx, obj *model.Object) error {
	return d.addFrom(c, obj, "")
}

// addFrom keeps one record read from file, a file of a folder or a zip.
func (d *importDraft) addFrom(c *collectionCtx, obj *model.Object, file string) error {
	doc, err := c.body(obj)
	if err != nil {
		return err
	}
	d.Records = append(d.Records, importRecord{
		UID: uidOf(c, obj), Title: titleOf(c.spec.kind, obj), Detail: c.importDetail(obj), Doc: doc,
		File: file, Ready: readyNote(c.spec.kind, obj),
	})
	return nil
}

// readyNote is what a row says after «Ready» when the record is not plain
// (s-io-6, s-io-7): a repeating event comes as its series, a task keeps how
// far it got and its priority.
func readyNote(k view.Kind, obj *model.Object) string {
	switch k {
	case view.KindEvent:
		if len(obj.PropText("RRULE")) > 0 {
			return "the series as one record"
		}
	case view.KindTask:
		var parts []string
		if p := percentOf(obj); p != nil && *p > 0 && *p < 100 {
			parts = append(parts, strconv.Itoa(*p)+" % done")
		}
		if v := obj.PropText("PRIORITY"); len(v) > 0 && strings.TrimSpace(v[0]) != "" && strings.TrimSpace(v[0]) != "0" {
			parts = append(parts, "priority "+strings.TrimSpace(v[0]))
		}
		return strings.Join(parts, ", ")
	}
	return ""
}

// kindHolders are what holds a kind, as the preview says where a stray
// record belongs (s-io-6, s-io-7).
var kindHolders = map[view.Kind][2]string{
	view.KindEvent: {"a calendar", "events"},
	view.KindTask:  {"a list of tasks", "tasks"},
	view.KindNote:  {"a notebook", "notes"},
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
// notes have (S-19) — a storage's root, or the folder in below it chosen in
// the tree (s-io-4). The files are read through the guard, and the folder is
// read one level deep — a notebook is a folder, not a tree.
func (s *Server) takeFolder(w http.ResponseWriter, r *http.Request, c *collectionCtx, dest *collectionCtx, account, folder string, in []string) {
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
		entries, err = prov.List(r.Context(), folderUnder(col.Path, in))
	}
	if err != nil {
		fail(view.Notice{
			Role: view.NoticeAlert, Title: text(collectionLabel(col) + " is unavailable"),
			Body:    text("The folder could not be read, so nothing was taken from it."),
			Details: &view.Diagnostic{Summary: text("PROPFIND " + folderUnder(col.Path, in)), Lines: []view.Text{text(connLabel(conn)), text(err.Error())}},
		})
		return
	}
	// «Documents › notes» (s-io-4): the storage, and the way to the folder.
	d := &importDraft{Kind: c.spec.kind, Account: c.conn.ID, Col: c.col.Path, Source: strings.Join(append([]string{collectionLabel(col)}, in...), " › ")}
	left := s.cfg.ImportMaxBytes
	markdown := 0
	for _, e := range entries {
		if e.Dir {
			continue
		}
		d.Files++
		if !isMarkdown(e.Name) {
			// Named, not dropped (s-io-8): the folder held it.
			d.Skipped = append(d.Skipped, importSkip{What: e.Name, Reason: "Not Markdown."})
			continue
		}
		markdown++
		if len(d.Records) >= s.cfg.ImportMaxRecords {
			fail(s.importRefusal(errTooManyRecords{len(entries), s.cfg.ImportMaxRecords}))
			return
		}
		body, err := readAtMost(r.Context(), prov, e.Path, left)
		if err != nil {
			d.Skipped = append(d.Skipped, importSkip{What: e.Name, Reason: err.Error(), Failed: true})
			continue
		}
		left -= int64(len(body))
		d.Size += int64(len(body))
		// A file without front matter is dated by its own timestamp
		// (s-io-4): the time the server says it was last changed.
		stamp := e.Modified
		if stamp.IsZero() {
			stamp = time.Now()
		}
		obj, err := cal.ParseNote(e.Name, body, stamp)
		if err != nil {
			d.Skipped = append(d.Skipped, importSkip{What: e.Name, Reason: err.Error(), Failed: true})
			continue
		}
		if err := d.addFrom(c, obj, e.Name); err != nil {
			d.Skipped = append(d.Skipped, importSkip{What: e.Name, Reason: err.Error(), Failed: true})
		}
	}
	if markdown == 0 {
		d.Note = "The folder holds no Markdown file."
	}
	d.To, d.Into = targetOf(dest)
	saveImportDraft(c, d)
	st := importState{draft: d, dest: dest}
	st.preview, st.taken, st.notices = s.importPreview(r.Context(), c, d, dest)
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
func (s *Server) importPreview(ctx context.Context, c *collectionCtx, d *importDraft, dest *collectionCtx) ([]view.ImportTable, map[string]bool, []view.Notice) {
	var notices []view.Notice
	taken := map[string]bool{}
	if dest != nil {
		var err error
		if taken, err = destUIDs(ctx, dest); err != nil {
			notices = append(notices, s.unavailableNotice(dest, s.importHref(c, "", dest.conn.ID, dest.col.Path), err))
		}
	}
	return []view.ImportTable{previewTable(c.spec.kind, d, taken)}, taken, notices
}

// previewTable is the preview as the frames draw it (s-io-5…8): one table,
// the columns of the kind and Result last; the records first, what the file
// holds and this import does not write after them, each with why.
func previewTable(k view.Kind, d *importDraft, taken map[string]bool) view.ImportTable {
	t := view.ImportTable{Columns: previewColumns[k]}
	source := func(file string) view.Text {
		if file != "" {
			return text(file)
		}
		return text(d.Source)
	}
	for _, rec := range d.Records {
		result := "Ready"
		switch {
		case taken[rec.UID]:
			result = "New UID will be assigned"
		case rec.Ready != "":
			result += " — " + rec.Ready
		}
		third := rec.Detail
		if k == view.KindContact {
			third = rec.UID
		}
		t.Rows = append(t.Rows, []view.Text{source(rec.File), text(rec.Title), text(third), text(result)})
	}
	for _, sk := range d.Skipped {
		result := sk.Reason
		if !sk.Failed {
			result = "Skipped: " + lowerFirst(strings.TrimSuffix(sk.Reason, "."))
			if sk.Where != "" {
				result += " — import it into " + sk.Where
			}
		}
		file, what := "", sk.What
		if k == view.KindNote {
			file, what = sk.What, ""
		}
		t.Rows = append(t.Rows, []view.Text{source(file), text(sk.Title), text(what), text(result)})
	}
	t.Count = len(t.Rows)
	return t
}

// previewColumns are the columns of the preview by kind (s-io-5…8).
var previewColumns = map[view.Kind][]view.Text{
	view.KindContact: {"Source", "Name", "UID", "Result"},
	view.KindEvent:   {"Source", "Summary", "When", "Result"},
	view.KindTask:    {"Source", "Summary", "Due", "Result"},
	view.KindNote:    {"File", "Title", "Date", "Result"},
}

// lowerFirst writes a sentence's first letter small, as it reads after
// «Skipped: ».
func lowerFirst(s string) string {
	for i, r := range s {
		return strings.ToLower(string(r)) + s[i+len(string(r)):]
	}
	return s
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
		st.preview, st.taken, st.notices = s.importPreview(r.Context(), c, d, nil)
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

	report := &view.ImportReport{}
	for _, sk := range d.Skipped {
		if sk.Failed {
			report.Failed++
			report.Failures = append(report.Failures, text(sk.What+": "+sk.Reason))
		} else {
			report.Skipped++
		}
	}
	written := view.ImportTable{Rubric: text("Created in " + collectionLabel(dest.col)), Columns: importColumns(c.spec.kind)}
	refused := view.ImportTable{Rubric: text("Not created"), Columns: []view.Text{text("What"), text("What the server answered")}}
	// Every identity is settled before the first write, so that a record
	// linking to another of the file names the identity that one will have
	// (S-19): links inside the file travel with it.
	uids, errs, moved := planImportUIDs(d.Records, taken)
	props := linkProps(dest)
	var made []importMade
	for i, rec := range d.Records {
		obj, err := dest.parse("", "", rec.Doc)
		if err == nil {
			err = errs[i]
		}
		uid, renamed, note := uids[i], uids[i] != rec.UID, "Created"
		if err == nil && renamed {
			if obj, err = setUID(dest, obj, uid); err == nil {
				note = "Created under " + uid + ": the collection already held the identity of this record"
			}
		}
		if err == nil {
			if p, ok := linkPatch(obj, props, moved); ok {
				if err = p.Apply(obj); err == nil {
					note += ". Its links to records of this file follow their new identities"
				}
			}
		}
		if err == nil {
			err = s.createImported(ctx, dest, obj, uid)
		}
		was := ""
		if err != nil && dav.IsStatus(err, http.StatusPreconditionFailed) && nameIsFree(ctx, dest, importName(dest, uid)) {
			// 412 on a name nothing holds is not about the name: the account
			// holds this UID in another of its collections (SV-55, iCloud).
			// The owner's rule for a copy holds here: a UID of its own, and
			// what of this file points at the record follows it.
			dest.sess.NoteRefusal(dest.conn.ID, "", refusedSameUID)
			var fresh string
			if obj, fresh, err = withOwnUID(dest, obj); err == nil {
				if err = s.createImported(ctx, dest, obj, fresh); err == nil {
					was, moved[uid] = uid, fresh
					uid, renamed = fresh, true
					note = "Created under " + fresh + ": this server holds one record per UID across the account, and another of its collections holds this one"
				}
			}
		}
		if err != nil {
			refused.Rows = append(refused.Rows, []view.Text{text(rec.Title), text(err.Error())})
			report.Failed++
			report.Failures = append(report.Failures, text(rec.Title+": "+err.Error()))
			continue
		}
		report.Created++
		if renamed {
			report.Renamed++
		}
		made = append(made, importMade{row: len(written.Rows), uid: uid, from: rec.UID, was: was, path: obj.Path, renamed: renamed, obj: obj})
		written.Rows = append(written.Rows, []view.Text{text(rec.Title), text(rec.Detail), text(note)})
	}
	s.importLanded(ctx, dest, report, &written, made)
	// The identities that changed, as the report's card lists them (s-io-9).
	for _, m := range made {
		if m.held != m.from {
			report.Renames = append(report.Renames, text(m.from+" → "+m.held))
		}
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

// importMade is a record an import wrote: its row in the report, the
// identity it was written under, and where.
type importMade struct {
	row int
	uid string
	// from is the identity the record had in the file; held the one the
	// server keeps it under, once importLanded has looked
	from, held string
	// was is the identity the record came with when its server refused it
	// for the UID alone (SV-55), and it was written under uid instead
	was     string
	path    string
	renamed bool
	obj     *model.Object // as it was sent: its links name the file's identities
}

// importLanded looks for what an import wrote, once, after the last record:
// a server may keep a new record under an identity of its own — Google does
// with every card (docs/servers.md, SV-25) — and the report must name the
// identity the record has, not the one it was sent with. One whole read of
// the destination; a read at the written address only for a record not
// found under its identity. The row says what became of it. Links inside the
// import follow the identities the server gave (followTheirs); what already
// pointed at the identity a record came with is left alone: an import is a
// copy, and a record of that identity may well live on elsewhere (S-19).
func (s *Server) importLanded(ctx context.Context, dest *collectionCtx, report *view.ImportReport, written *view.ImportTable, made []importMade) {
	for i := range made {
		made[i].held = made[i].uid
	}
	if len(made) == 0 {
		return
	}
	held, err := destUIDs(ctx, dest)
	if err != nil {
		return // the rows stand as the writes were answered
	}
	gave := map[string]string{} // written under → held under
	for i := range made {
		m := &made[i]
		if m.was != "" {
			gave[m.was] = m.uid // what was written before it names it still
		}
		if held[m.uid] {
			continue
		}
		at, err := dest.fetch(ctx, m.path)
		if err != nil {
			written.Rows[m.row][2] = text("Created, but the server does not show it under this identity or at the address it was written to")
			continue
		}
		theirs := uidOf(dest, at)
		if theirs == "" || theirs == m.uid {
			continue // there under its own identity; the listing missed it
		}
		written.Rows[m.row][2] = text("Created under " + theirs + ": the server gave it an identity of its own")
		gave[m.uid] = theirs
		m.held = theirs
		dest.noteWrite(m.path, at)
		if !m.renamed {
			report.Renamed++
		}
	}
	followTheirs(ctx, dest, written.Rows, made, gave)
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
	second := strings.TrimSuffix(dest.col.Path, "/") + "/" + fileStem(uid) + "-2" + dest.spec.ext
	for _, dst := range []string{importName(dest, uid), second} {
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

// importName is where a record of this identity is written first.
func importName(dest *collectionCtx, uid string) string {
	return strings.TrimSuffix(dest.col.Path, "/") + "/" + fileStem(uid) + dest.spec.ext
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

// combinedBackRail is the rail of the wizard opened from a combined view:
// the way back is that view, and no collection is open on it.
func (s *Server) combinedBackRail(c *collectionCtx) view.Rail {
	return s.backRail(c.sess, s.catalogOf(c.sess), c.spec, c.conn.ID, s.backAction(c.spec, "", "/app/"+c.spec.segment))
}
