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
	// Existing is what the person chose to do with the records of the file
	// the destination already holds (existingSkip, …Replace, …Copies)
	Existing string
	// Picked — several files were picked at once (Files of them): the head
	// counts them, it names none
	Picked bool
}

// What an import does with a record whose identity the destination already
// holds (А7 of the audit of 11.10.2026; the owner's decision of 12.10.2026):
// skips it, which is what happens without an answer; writes the file's
// version over the one there; or creates a copy under a fresh identity, as
// every import did before.
const (
	existingSkip    = ""
	existingReplace = "replace"
	existingCopies  = "copies"
)

// existingChoice reads the answer of an address — the frames' words
// (s-io-5…-8: existing=skip|replace|copy; Им7), «copies» as before: a word
// that is none of them is no answer.
func existingChoice(v string) (string, bool) {
	switch v {
	case "skip":
		return existingSkip, true
	case existingReplace:
		return v, true
	case "copy", existingCopies:
		return existingCopies, true
	}
	return "", false
}

// collides counts the records of the draft whose identity taken holds.
func collides(d *importDraft, taken map[string]bool) int {
	n := 0
	for _, rec := range d.Records {
		if taken[rec.UID] {
			n++
		}
	}
	return n
}

// importExisting is the question of the preview about the records already
// there: how many, and the three answers as links to the same preview.
func (s *Server) importExisting(c *collectionCtx, st importState) *view.ImportExisting {
	n := collides(st.draft, st.taken)
	if n == 0 || st.dest == nil {
		return nil
	}
	// The question is the label of the field Into (from-typecase note 52):
	// what is already here is counted against the collection chosen.
	noun := c.spec.noun
	if c.spec.kind == view.KindContact {
		noun = "record" // s-io-5
	}
	out := &view.ImportExisting{Note: text(fmt.Sprintf("%s of this file are already in %s", plural(n, noun), collectionLabel(st.dest.col)))}
	options := []struct{ id, word, label, hint string }{
		{"skip", existingSkip, "Skip them", "They stay as they are here; the file's version is not written."},
		{"replace", existingReplace, "Replace them", "The ones here are overwritten with the file's version."},
		{"copy", existingCopies, "Import as copies", "Each gets a new UID and stands beside the one here."},
	}
	if n == 1 {
		out.Note = text(fmt.Sprintf("1 %s of this file is already in %s", noun, collectionLabel(st.dest.col)))
		options = []struct{ id, word, label, hint string }{
			{"skip", existingSkip, "Skip it", "It stays as it is here; the file's version is not written."},
			{"replace", existingReplace, "Replace it", "The one here is overwritten with the file's version."},
			{"copy", existingCopies, "Import as a copy", "It gets a new UID and stands beside the one here."},
		}
	}
	for _, o := range options {
		if st.draft.Existing == o.word {
			out.Hint = text(o.hint)
		}
		q := importQuery("", st.dest.conn.ID, st.dest.col.Path)
		q.Set("existing", o.id)
		out.Options = append(out.Options, view.Action{ID: "import.existing." + o.id, Label: text(o.label), Method: "GET", Enabled: true,
			Href: s.postQuery(c.listPath()+"/import", q), On: st.draft.Existing == o.word})
	}
	return out
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
	// Untitled: the record has no title, and Title is the frame's
	// placeholder (Им10)
	Untitled bool
}

// importSkip is what the file held and this import does not write.
type importSkip struct {
	What  string // the component, or the kind of record it is
	Title string
	// Detail is the record's own date, in the third column of the
	// preview (Им9; s-io-7: «2026-08-17» of an event in a list of tasks);
	// empty — «—»
	Detail string
	Where  string // the section it belongs in; empty when Carrel keeps no such record
	Reason string
	Failed bool // could not be read at all: counted «with errors», not «skipped»
}

// named is the name of a record left out (Им16; s-io-5, -9, -14): its
// title; a record without one — «—» (Typecase's note 111 p. 6); a file of a
// set — its name.
func (sk importSkip) named() string {
	switch {
	case sk.Title != "":
		return sk.Title
	case strings.HasPrefix(sk.What, "V"):
		return "—"
	}
	return sk.What
}

// line is a record left out as the report names it (Им15; s-io-14): «Купить
// билеты · VTODO: a calendar takes events — import it into a list of
// tasks»; a file of a set — «skan.pdf: not Markdown».
func (sk importSkip) line() string {
	what := sk.What
	if sk.Title != "" {
		what = sk.Title + " · " + sk.What
	}
	out := what + ": " + lowerFirst(strings.TrimSuffix(sk.Reason, "."))
	if sk.Where != "" {
		out += " — import it into " + sk.Where
	}
	return out
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
		return []view.Text{".md", ".zip", ".ics"} // the order of accept (s-io-4)
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
	return view.Action{ID: "collection.import", Label: "Import", Icon: "i-import",
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
			// The root says its connection only (Им6; s-io-4: «Documents»,
			// dimmed «sftpgo»).
			node := view.FileTreeNode{ID: link(conn.ID, col.Path, nil, ""), Label: collectionLabel(col), Stamp: view.Stamp(connLabel(conn))}
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
	// unanswered: the collection chosen did not answer the preview, and
	// Import is off for it (s-io-19)
	unanswered string
	// entry: the wizard was opened from a combined view, and nothing is
	// chosen under «Into» until the person chooses it (C-5, U-6.9).
	entry bool
	// held is the file dropped on a combined view (Им4; s-io-1): taken,
	// waiting for «Into»; the first step says which.
	held *importDraft
	// folderAcc, folderPath and folderIn are the folder chosen in the tree
	// of notes (s-io-4) — the storage and the folder below its root —,
	// before «Preview folder» takes it.
	folderAcc, folderPath string
	folderIn              []string
}

// importSteps are the words of the wizard's steps line (s-io-3…9), in the
// order of view.ImportStep.
var importSteps = []view.Text{"1 · Source", "2 · Preview", "3 · Report"}

// importHints are the line under the file field (s-io-3, -4, -15, -16;
// Typecase's note 111 pp. 1, 2): each extension the kind takes and what it
// is, how many one file holds, what in it is skipped.
var importHints = map[view.Kind]view.Runs{
	view.KindContact: join(monoRun(".vcf"), " — vCard 2.1, 3.0 and 4.0, any number of cards in one file."),
	view.KindEvent:   join(monoRun(".ics"), " — iCalendar, any number of events in one file; tasks and journal entries in it are skipped and named in the report."),
	view.KindTask:    join(monoRun(".ics"), " — iCalendar, any number of tasks in one file; events and journal entries in it are skipped and named in the report."),
	view.KindNote: join(monoRun(".md"), " — Markdown, ", monoRun(".zip"), " — any number of them, ", monoRun(".ics"), " — iCalendar journal entries. Several files at once; events and tasks in an ",
		monoRun(".ics"), " are skipped and named in the report."),
}

// importFolderHint is the line under the tree of folders (s-io-4).
var importFolderHint = join("Every ", monoRun(".md"), " directly in that folder, without subfolders. Nothing on the server is moved or deleted.")

// importNoteFirst is the message of the first step of notes (s-io-4); the
// other first steps and the previews have none (note 135).
var importNoteFirst = line("A file without front matter is dated by its own timestamp.")

func (s *Server) importPage(c *collectionCtx, st importState) view.ImportPage {
	page := view.ImportPage{
		Shell: s.buildShell(c.sess, "Import "+c.spec.noun+"s"), Kind: c.spec.kind,
		Step: view.ImportStepSource, Steps: importSteps, Preview: st.preview, Report: st.report,
	}
	page.Rail = s.recordRail(c, s.listBack(c))
	page.Notices = append(page.Notices, st.notices...)
	// «2 min ago» in the head (Им1; s-io-3…9): when the collection the
	// records go into was last read, and the wizard read again — the
	// preview counted anew against it; after the report, its list.
	{
		read, here := c, c.listPath()+"/import"
		if st.dest != nil {
			read = st.dest
			to, into := targetOf(st.dest)
			here += "?" + importQuery("", to, into).Encode()
		}
		if st.report != nil && st.dest != nil {
			here = st.dest.listPath()
		}
		page.Top.Refresh = s.recordRefresh(read, here)
	}
	page.Into = s.importTargets(c, st.dest)
	// Notes take several files at once (Typecase's note 111 p. 3).
	page.Source = view.ImportSource{Kinds: importFormats(c.spec.kind), Hint: importHints[c.spec.kind], Many: c.spec.kind == view.KindNote}
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
		page.Title = "Import report" // s-io-9, -14 (Им2)
		done := view.Action{ID: "import.done", Label: "Back to list", Method: "GET", Primary: true, Enabled: true,
			Href: s.route(c.listPath())}
		if st.dest != nil {
			done.Href = s.route(st.dest.listPath())
			// The collection by its mark (the owner, 22.10.2026).
			page.Subtitle = view.Runs{{Text: text(string(st.dest.stamp())), Colour: catalogColour(st.dest.col)}}
		}
		page.Actions = []view.Action{done}
	case st.draft != nil:
		st.draft = fitDraft(st.draft, st.dest)
		page.Step = view.ImportStepPreview
		page.Subtitle = importSubtitle(st.draft)
		page.Counts = importCounts(st.draft, st.taken)
		page.Existing = s.importExisting(c, st)
		// The preview has no message of its own (note 135): only what
		// the file lacked.
		page.Source.Note = nil
		if st.draft.Note != "" {
			page.Source.Note = line(st.draft.Note)
		}
		// «Import 122 contacts» (s-io-5…8): what the press writes — the
		// records already there are not among them while they are skipped.
		writes := len(st.draft.Records)
		if st.draft.Existing == existingSkip {
			writes -= collides(st.draft, st.taken)
		}
		run := view.Action{ID: "import.run", Label: text("Import " + plural(writes, c.spec.noun)), Method: "POST", Primary: true, Enabled: true}
		switch {
		case st.dest == nil:
			run.Enabled, run.Reason = false, "Choose a collection that can be written."
		case st.unanswered != "":
			run.Enabled, run.Reason = false, text(st.unanswered)
		case len(st.draft.Records) == 0:
			run.Enabled, run.Reason = false, "This file holds no record of this kind."
		case writes == 0:
			run.Enabled, run.Reason = false, "Nothing to import: every record is already here."
		default:
			run.Href = s.importHref(c, "run", st.dest.conn.ID, st.dest.col.Path)
		}
		// The file waits in the session: leaving drops it, so Cancel is a
		// POST here and not the way back of the first step.
		page.Actions = []view.Action{run, {ID: "import.cancel", Label: "Cancel",
			Href: s.importHref(c, "cancel", "", ""), Method: "POST", Enabled: true}}
	default:
		if st.held != nil {
			// The file's name alone, in the file field (s-io-17).
			page.Source.Taken = join(monoRun(st.held.Source))
		}
		to, into := targetOf(st.dest)
		// The first step has one button; the rail is the way out (s-io-3,
		// s-io-4).
		// Without scripts the file goes in a form of its own, its field
		// named «file» (U-8; importPart reads that part).
		choose := view.Action{ID: "import.choose", Label: "Preview file", Method: "POST", Primary: true, Enabled: true,
			Href: s.importHref(c, "", to, into), File: "file"}
		if st.dest == nil {
			// Off with why; its address stays the form's, which holds the
			// file taken by a drop (s-io-17).
			choose.Enabled, choose.Reason = false, "Choose a collection that can be written."
			if st.entry {
				choose.Reason = "Choose under «Into» where the records go."
			}
		}
		page.Actions = []view.Action{choose}
		// A file dropped on the window stands in the step's file field, as
		// one picked there (Typecase's note 209, task 265): the drop sheet's
		// words, what the field takes and what it refuses, as a list's.
		page.Source.Drop = dropOf(c.spec.kind, choose.Href, "", "")
	}
	return page
}

// importSubtitle is the head of the preview (s-io-5…8): the file
// monospaced, its size and the format it was read as; a folder's name, the
// size of what was read and how many files it held; several files picked at
// once — how many and their size, «3 files · 12 KB» (Typecase's note 111
// p. 3).
func importSubtitle(d *importDraft) view.Runs {
	if d.Picked {
		return line(plural(d.Files, "file") + " · " + roughSize(d.Size))
	}
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
	// «ready» is what the press writes: the records already here are not
	// among them while they are skipped; «already here» counts them whatever
	// the answer (from-typecase note 52).
	ready := len(d.Records)
	if d.Existing == existingSkip {
		ready -= collisions
	}
	out := []view.ImportCount{{Count: ready, Label: "ready"}}
	if failed > 0 {
		out = append(out, view.ImportCount{Count: failed, Label: "with errors", Error: true})
	}
	if collisions > 0 {
		out = append(out, view.ImportCount{Count: collisions, Label: "already here"})
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
	c := s.importHost(sess, spec)
	if c == nil {
		s.notFound(w, r)
		return
	}
	st := importState{entry: true}
	// A file dropped here and not yet sent anywhere (Им4) is still in hand.
	if d, has := loadImportDraft(c); has && d.To == "" {
		st.held = d
	}
	page := s.importPage(c, st)
	page.Rail = s.combinedBackRail(c)
	s.writePage(w, r, http.StatusOK, page)
}

// importHost is the collection the wizard of a combined view stands on: the
// first that takes records of the kind. Its address carries the wizard
// only; «Into» is the person's to choose. nil — none takes any.
func (s *Server) importHost(sess *session.Session, spec kindSpec) *collectionCtx {
	blob := s.catalogOf(sess)
	for _, conn := range blob.Connections {
		for _, col := range blob.CollectionsOf(conn.ID) {
			if col.Gone || col.ReadOnly || conn.Feed || !col.Has(spec.kind) {
				continue
			}
			if c, made := s.collectionOf(sess, spec, conn.ID, col.Path); made {
				return c
			}
		}
	}
	return nil
}

// postKindImportEntry takes a file dropped on a combined view (Им4; s-io-1):
// nothing is chosen there, so the first step opens with the file in hand and
// «Into» empty — the cards are not laid into the first book that comes, a
// server of somebody's. Choosing «Into» opens the preview of this file.
func (s *Server) postKindImportEntry(w http.ResponseWriter, r *http.Request, kind string) {
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	spec, known := recordKinds[kind]
	if !known {
		s.notFound(w, r)
		return
	}
	c := s.importHost(sess, spec)
	if c == nil {
		s.notFound(w, r)
		return
	}
	st := importState{entry: true}
	d, err := s.readUpload(r, c)
	if err != nil {
		st.notices = []view.Notice{s.importRefusal(err)}
	} else {
		d.To, d.Into = "", ""
		saveImportDraft(c, d)
		st.held = d
	}
	page := s.importPage(c, st)
	page.Rail = s.combinedBackRail(c)
	s.writePage(w, r, http.StatusOK, page)
}

// getKindExportEntry is the way into the export from a combined view: which
// collection to hand over. One is not guessed — the file would be somebody
// else's records (S-14). The sheet is a GET form (s-agenda-30): the
// collection a radio «c», a calendar's span the whole of it or days; sent,
// it is the collection's own export.
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
	q := r.URL.Query()
	if v := q.Get("c"); v != "" {
		account, col, _ := strings.Cut(v, "/")
		path, err := url.PathUnescape(col)
		c, ok := s.collectionOf(sess, spec, account, path)
		if err != nil || !ok || c.col.Gone {
			s.notFound(w, r)
			return
		}
		out := url.Values{}
		switch {
		case spec.kind != view.KindEvent:
		case q.Get("span") == "whole":
			out.Set("whole", "1")
		default:
			out.Set("from", q.Get("from"))
			out.Set("to", q.Get("to"))
		}
		s.redirect(w, r, s.postQuery(c.listPath()+"/export", out))
		return
	}
	blob := s.catalogOf(sess)
	ext := exportFormats[spec.kind].ext
	download := view.Action{ID: "export.download", Label: text("Download " + ext), Icon: "i-download", Method: "GET", Primary: true, Enabled: true, Submits: true}
	sheet := view.ExportSheet{Kind: spec.kind, Title: "Export", Subject: text(combinedTitles[spec.kind]), Rubric: exportRubrics[spec.kind], Actions: []view.Action{download,
		{ID: "export.cancel", Label: "Cancel", Href: s.route("/app/" + spec.segment), Method: "GET", Enabled: true},
	}}
	sheet.Target.Field = "c"
	for _, conn := range blob.Connections {
		for _, col := range blob.CollectionsOf(conn.ID) {
			if col.Gone || !col.Has(spec.kind) {
				continue
			}
			opt := view.TargetOption{
				ID:    s.postQuery("/app/"+spec.segment+"/export", url.Values{"c": {moveValue(conn.ID, col.Path)}}),
				Label: text(collectionLabel(col)), Stamp: view.Stamp(connLabel(conn)), Colour: catalogColour(col),
				Value: moveValue(conn.ID, col.Path),
			}
			// A server that does not answer has no file to give (s-agenda-30:
			// «password · 401»).
			if why := silentWhy(sess, conn); why != "" {
				opt.ReadOnly, opt.Reason = true, text(why)
			} else if sheet.Target.Chosen == "" {
				sheet.Target.Chosen = opt.ID
			}
			sheet.Target.Options = append(sheet.Target.Options, opt)
		}
	}
	if len(sheet.Target.Options) == 0 {
		s.notFound(w, r)
		return
	}
	if sheet.Target.Chosen == "" {
		sheet.Actions[0].Enabled, sheet.Actions[0].Reason = false, "No collection here answers now."
	}
	var rows []view.FormRow
	if spec.kind == view.KindEvent {
		rows = s.exportSpan(sess)
	}
	sheet.Form = s.plainForm("/app/"+spec.segment+"/export", "GET", rows...)
	if render.WantsFragment(r) && render.FragmentTarget(r) == render.TargetSheet {
		_ = s.render.Fragment(w, r, http.StatusOK, render.TargetSheet, sheet, nil)
		return
	}
	// Over the list it was opened from, else the kind's combined one (Лс29).
	sheet.Under = s.underOf(r, s.sheetBack(r, q.Get("back"), true), "/app/"+spec.segment)
	s.writePage(w, r, http.StatusOK, sheet)
}

// exportRubrics are the words over the collections of the export sheet
// (s-agenda-30).
var exportRubrics = map[view.Kind]view.Text{
	view.KindContact: "Which address book",
	view.KindEvent:   "Which calendar",
	view.KindTask:    "Which list",
	view.KindNote:    "Which notebook",
}

// exportSpan is «What goes in» of a calendar's export (s-agenda-30): the
// whole calendar, which turns the days off, or days — from today to the end
// of the month, chosen as the sheet opens.
func (s *Server) exportSpan(sess *session.Session) []view.FormRow {
	tf := s.timeFor(sess)
	today := tf.now.In(tf.tz)
	return []view.FormRow{
		{Name: "span", Label: "What goes in", Kind: view.InputRadio, Value: "dates", Options: []view.Option{
			{Value: "whole", Label: "Whole calendar", Disables: "from to"},
			{Value: "dates", Label: "Dates", Selected: true},
		}},
		{Name: "from", Label: "From", Kind: view.InputDate, Mono: true, Value: text(today.Format("2006-01-02"))},
		{Name: "to", Label: "To", Kind: view.InputDate, Mono: true, Value: text(today.AddDate(0, 1, -today.Day()).Format("2006-01-02"))},
	}
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
		if choice, ok := existingChoice(q.Get("existing")); ok {
			st.draft.Existing = choice
		}
		saveImportDraft(c, st.draft)
		st.preview, st.taken, st.notices, st.unanswered = s.importPreview(r.Context(), c, st.draft, st.dest)
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
	d, err := s.readUpload(r, c)
	if err != nil {
		s.writePage(w, r, http.StatusOK, s.importPage(c, importState{dest: dest, notices: []view.Notice{s.importRefusal(err)}}))
		return
	}
	d.To, d.Into = targetOf(dest)
	saveImportDraft(c, d)
	st := importState{draft: d, dest: dest}
	st.preview, st.taken, st.notices, st.unanswered = s.importPreview(r.Context(), c, d, dest)
	s.writePage(w, r, http.StatusOK, s.importPage(c, st))
}

// readUpload reads the files of the request as the wizard of c takes them.
func (s *Server) readUpload(r *http.Request, c *collectionCtx) (*importDraft, error) {
	files, err := uploadedImports(r)
	var d *importDraft
	switch {
	case err != nil:
	case len(files) > 1 && c.spec.kind != view.KindNote:
		// Only notes are taken several files at once (s-io-1); the others
		// are refused whole rather than read by their first file (Им5).
		err = errOneFileOnly
	case len(files) > 1:
		d = s.readNoteFiles(c, files)
	case len(bytes.TrimSpace(files[0].body)) == 0:
		// An empty file is no file to read (Им14): said so, not «no file
		// was chosen» (s-io-18: «‹name›» is empty.).
		err = errRead{"«" + files[0].name + "» is empty."}
	default:
		d, err = s.readImport(c, files[0].name, files[0].body)
	}
	return d, err
}

// readImport takes a file apart by its kind. A calendar file holds records of
// three kinds at once, and the two this collection does not keep are named,
// not dropped (S-19).
func (s *Server) readImport(c *collectionCtx, name string, body []byte) (*importDraft, error) {
	if c.spec.kind == view.KindNote && (isZip(body) || strings.HasSuffix(strings.ToLower(name), ".zip")) {
		return s.readZipNotes(c, name, body)
	}
	d := &importDraft{Kind: c.spec.kind, Account: c.conn.ID, Col: c.col.Path, Source: name, Size: int64(len(body))}
	if c.spec.kind == view.KindNote && isMarkdown(name) {
		d.Format = "Markdown"
		// A browser does not send the file's time: dated the day of the
		// import (Б35).
		obj, err := cal.ParseNote(name, body, c.now())
		if err != nil {
			return nil, err
		}
		if err := d.add(c, obj); err != nil {
			return nil, err
		}
		// Where its title and date come from, as a file of a set says it
		// (Им12; s-io-8).
		d.Records[len(d.Records)-1].Ready = noteReady(body, false)
		return d, nil
	}
	if c.card != nil {
		if !strings.EqualFold(firstLine(body), "BEGIN:VCARD") {
			return nil, errNotThisFile{name, "a vCard", "BEGIN:VCARD"}
		}
		objs, broken, err := card.Split(body)
		if err != nil {
			return nil, err
		}
		if len(objs) == 0 && len(broken) == 0 {
			return nil, errRead{"This file holds no records."}
		}
		d.Format = vcardFormat(objs)
		// A card that cannot be read is counted and named; the cards after
		// it are the file's all the same (S-19).
		for _, b := range broken {
			d.Skipped = append(d.Skipped, importSkip{What: "VCARD", Title: b.Name, Reason: b.Reason, Failed: true})
		}
		for _, obj := range objs {
			if err := d.add(c, obj); err != nil {
				return nil, err
			}
		}
		return d, nil
	}
	if !strings.EqualFold(firstLine(body), "BEGIN:VCALENDAR") {
		return nil, errNotThisFile{name, "an iCalendar", "BEGIN:VCALENDAR"}
	}
	d.Format = icalFormat(body)
	objs, strays, err := cal.Split(body, time.Now())
	if err != nil {
		return nil, err
	}
	// Records of another kind are no refusal: the preview names them and
	// where they go (Typecase's note 113 p. 6). A file of none is.
	if len(objs) == 0 && len(strays) == 0 {
		return nil, errRead{"This file holds no records."}
	}
	for _, stray := range strays {
		// A record of this file that could not be read is counted «with
		// errors», its line said (s-io-7); a component Carrel keeps no
		// record of is skipped.
		d.Skipped = append(d.Skipped, importSkip{
			What: stray.Component, Title: stray.Summary, Reason: stray.Reason, Failed: stray.Broken,
		})
	}
	for _, obj := range objs {
		if k := cal.Kind(obj); k != c.spec.kind {
			title, _ := importTitle(k, obj)
			d.Skipped = append(d.Skipped, importSkip{
				What: obj.MainComponent(), Title: title, Detail: c.detailOf(k, obj), Where: kindHolders[k][0],
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
// takeMarkdown adds a Markdown file of a set — a zip, a folder, several
// files dropped at once — as a note, or names it among the skipped with
// why. One rule for the three ways in (Им14; s-io-8): an empty file is
// skipped, «nothing to store and nothing to name it by». A file without
// front matter is dated by stamp; none — now.
func (d *importDraft) takeMarkdown(c *collectionCtx, label string, body []byte, stamp time.Time) {
	if len(bytes.TrimSpace(body)) == 0 {
		d.Skipped = append(d.Skipped, importSkip{What: label, Reason: emptyMarkdown, Failed: true})
		return
	}
	// A file that carries no time is dated the day of the import, and the
	// preview says so (Б35 of the audit of 11.10.2026).
	dated := !stamp.IsZero()
	if !dated {
		stamp = c.now()
	}
	obj, err := cal.ParseNote(path.Base(label), body, stamp)
	if err != nil {
		d.Skipped = append(d.Skipped, importSkip{What: label, Reason: err.Error(), Failed: true})
		return
	}
	if err := d.addFrom(c, obj, label); err != nil {
		d.Skipped = append(d.Skipped, importSkip{What: label, Reason: err.Error(), Failed: true})
		return
	}
	// Where the note's title and date come from (Им12; s-io-8).
	d.Records[len(d.Records)-1].Ready = noteReady(body, dated)
}

// noteReady says where a Markdown note's title and date come from (s-io-8);
// dated is whether its file carried a time.
func noteReady(body []byte, dated bool) string {
	switch {
	case cal.HasNoteHeader(body):
		return "title, date and tags from front matter"
	case !dated:
		// The owner's words of 03.11.2026: the frame's row with the date's
		// own source; the date itself stands in its column.
		return "no front matter: title from the first heading, date of the import (the file has none)"
	}
	return "no front matter: title from the first heading, date from the file"
}

// emptyMarkdown is why an empty Markdown file is not a note (s-io-8).
const emptyMarkdown = "Empty file: nothing to store and nothing to name it by"

func (d *importDraft) addFrom(c *collectionCtx, obj *model.Object, file string) error {
	doc, err := c.body(obj)
	if err != nil {
		return err
	}
	title, untitled := importTitle(c.spec.kind, obj)
	d.Records = append(d.Records, importRecord{
		UID: uidOf(c, obj), Title: title, Untitled: untitled, Detail: c.detailOf(c.spec.kind, obj), Doc: doc,
		File: file, Ready: readyNote(c, obj, untitled),
	})
	return nil
}

// readyNote is what a row says after «Ready» when the record is not plain
// (s-io-6, s-io-7; Им12): a repeating event comes as its series, an event
// written in another zone keeps it; a task keeps how far it got and its
// priority, an overdue one stays overdue, and one without a date is a task
// all the same. A note's is said where it is read (takeMarkdown). What the
// file left out is written all the same, as the standard reads it (Р-72;
// Typecase's note 111 p. 5): an event without an end — a day lasts the day,
// a moment is a moment —, a record without a title under the default one.
func readyNote(c *collectionCtx, obj *model.Object, untitled bool) string {
	var parts []string
	switch c.spec.kind {
	case view.KindEvent:
		if len(obj.PropText("RRULE")) > 0 {
			parts = append(parts, "the series as one record")
		}
		// A day without an end lasts the day, a moment without one is a
		// moment, as the standard reads them (s-io-6; Typecase's note 113
		// p. 4).
		if props := obj.Props(); len(props["DTEND"]) == 0 && len(props["DURATION"]) == 0 {
			if tm, err := cal.TimingOf(obj, c.tf.tz); err == nil && !tm.Start.IsZero() {
				if tm.AllDay {
					parts = append(parts, "a whole day (no end)")
				} else {
					parts = append(parts, "a moment (no end)")
				}
			}
		}
		// The row's When is the person's clock; the event's own, with its
		// zone, is what was kept (s-io-6; Typecase's note 142 p. 4).
		if tm, err := cal.TimingOf(obj, c.tf.tz); err == nil {
			if at, _ := c.tf.keptAt(tm); at != "" {
				parts = append(parts, "kept as sent: "+at)
			}
		}
	case view.KindTask:
		if p := percentOf(obj); p != nil && *p > 0 && *p < 100 {
			parts = append(parts, strconv.Itoa(*p)+" % done")
		}
		if v := obj.PropText("PRIORITY"); len(v) > 0 && strings.TrimSpace(v[0]) != "" && strings.TrimSpace(v[0]) != "0" {
			parts = append(parts, "priority "+strings.TrimSpace(v[0]))
		}
		due, derr := cal.DueOf(obj, c.tf.tz)
		start, serr := cal.TimingOf(obj, c.tf.tz)
		switch {
		case derr == nil && !due.Start.IsZero() && due.Start.Before(c.now()) && !taskDone(obj):
			parts = append(parts, "overdue, and stays overdue")
		case (derr != nil || due.Start.IsZero()) && (serr != nil || start.Start.IsZero()):
			parts = append(parts, "a task without a date is a task")
		}
	}
	if untitled && (c.spec.kind == view.KindEvent || c.spec.kind == view.KindTask) {
		parts = append(parts, "no title")
	}
	// A card is named by its FN; without one the import writes the name of
	// its N, and «Unnamed contact» when there is none (Р-76, the owner's
	// decision of 22.10.2026; s-io-5).
	if c.spec.kind == view.KindContact && strings.TrimSpace(firstValue(obj, "FN")) == "" {
		if nameFromN(obj) != "" {
			parts = append(parts, "name from N")
		} else {
			parts = append(parts, "no name")
		}
	}
	// One note a property, with commas (Typecase's note 113 p. 4; s-io-6:
	// «a whole day (no end), no title»; s-io-7: «40 % done, priority 1»).
	return strings.Join(parts, ", ")
}

// nameFromN is a card's name as its N gives it, given name first: what an
// import writes as FN where the file wrote none (Р-76); "" — no N.
func nameFromN(obj *model.Object) string {
	n := strings.Split(firstValue(obj, "N"), ";")
	if len(n) < 2 {
		return strings.TrimSpace(n[0])
	}
	return strings.TrimSpace(strings.TrimSpace(n[1]) + " " + strings.TrimSpace(n[0]))
}

// withFN is a card that has an FN: its own, else its N's name, else
// «Unnamed contact» (Р-76). Organisation and address do not name a card.
func withFN(obj *model.Object) *model.Object {
	if strings.TrimSpace(firstValue(obj, "FN")) != "" {
		return obj
	}
	name := nameFromN(obj)
	if name == "" {
		name = "Unnamed contact"
	}
	props := obj.Props()
	props["FN"] = []model.RawValue{{Text: name}}
	return model.NewObject(obj.Path, obj.ETag, props)
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
// detailOf is the third column of a record of kind k: a contact's address,
// an event's time, a task's due date, a note's date.
func (c *collectionCtx) detailOf(k view.Kind, obj *model.Object) string {
	switch k {
	case view.KindContact:
		return firstValue(obj, "EMAIL")
	case view.KindEvent:
		if t, err := cal.TimingOf(obj, c.tf.tz); err == nil {
			// An event a list of tasks skips is named by its day alone, as
			// that list's frame draws it (s-io-7).
			if c.spec.kind != view.KindEvent {
				return c.whenOf(t)
			}
			return c.eventWhen(t, cal.RawRRULE(obj))
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

// eventWhen is the When of an event in the preview, in the card's full form
// (Typecase's note 142): «17 August 2026, 08:00» by the person's clock; a
// day and the days of one — «4 May 2026 · all day», «13 – 26 July 2026 · all
// day», the last day before DTEND —; a series from its first start, the
// weekday before it, and its rule in the words of the series question —
// «Monday, 10 August 2026, 09:00 · weekly until 28 December».
func (c *collectionCtx) eventWhen(t cal.Timing, rule string) string {
	if t.Start.IsZero() {
		return ""
	}
	start := t.Start.In(c.tf.tz)
	var when string
	switch {
	case t.AllDay:
		when = c.tf.date(start)
		if !t.End.IsZero() {
			if last := t.End.In(c.tf.tz).AddDate(0, 0, -1); last.After(start) {
				when = allDayRange(start, last)
			}
		}
		when += " · all day"
	default:
		when = c.tf.date(start) + ", " + c.tf.clock(start)
	}
	if rule == "" {
		return when
	}
	return start.Format("Monday") + ", " + when + " · " + seriesPace(rule) + seriesEnd(c.tf, rule, t.Start)
}

// allDayRange is two days of one event, the month and the year once where they
// are the same: «13 – 26 July 2026», «30 June – 2 July 2026», «28 December
// 2026 – 2 January 2027».
func allDayRange(first, last time.Time) string {
	switch {
	case first.Year() != last.Year():
		return first.Format("2 January 2006") + " – " + last.Format("2 January 2006")
	case first.Month() != last.Month():
		return first.Format("2 January") + " – " + last.Format("2 January 2006")
	}
	return first.Format("2") + " – " + last.Format("2 January 2006")
}

// titleOf is what one record is called in the preview and the report.
// importTitle is a record's title as the preview names it, and whether it
// has none: then the frame's placeholder, «— (no FN)», «— (no SUMMARY)»,
// «—» of a note (Им10; s-io-5…8).
func importTitle(k view.Kind, obj *model.Object) (string, bool) {
	switch k {
	case view.KindContact:
		// FN, else the name N gives (Р-76: the import writes it so); an
		// organisation or an address does not name a card.
		if name := strings.TrimSpace(firstValue(obj, "FN")); name != "" {
			return name, false
		}
		if name := nameFromN(obj); name != "" {
			return name, false
		}
		return "Unnamed contact", true
	case view.KindNote:
		if t := noteTitle(cal.Text(obj, "SUMMARY"), cal.Description(obj)); t != "Untitled note" {
			return t, false
		}
		return "—", true
	}
	if t := strings.TrimSpace(cal.Text(obj, "SUMMARY")); t != "" {
		return t, false
	}
	// The default title a record without one is written under (Р-72;
	// s-io-6, -7).
	return titleOf(k, obj), true
}

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
		// The server's refusal of the folder (s-io-18; Typecase's note 113
		// p. 5): its words, and under them what was asked and the code.
		fail(view.Notice{
			Role: view.NoticeAlert, Title: "The folder could not be read.",
			Body:    text(sentence(err.Error())),
			Details: refusalDetails("read", "PROPFIND", folderUnder(col.Path, in), err),
		})
		return
	}
	// «Documents › notes» (s-io-4): the storage, and the way to the folder.
	d := &importDraft{Kind: c.spec.kind, Account: c.conn.ID, Col: c.col.Path, Source: strings.Join(append([]string{collectionLabel(col)}, in...), " › ")}
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
		body, err := readWhole(r.Context(), prov, e.Path)
		if err != nil {
			d.Skipped = append(d.Skipped, importSkip{What: e.Name, Reason: err.Error(), Failed: true})
			continue
		}
		d.Size += int64(len(body))
		// A file without front matter is dated by its own timestamp
		// (s-io-4): the time the server says it was last changed.
		d.takeMarkdown(c, e.Name, body, e.Modified)
	}
	if markdown == 0 {
		// Read, and nothing in it to take (s-io-18).
		fail(view.Notice{Role: view.NoticeAlert, Title: nothingRead, Body: "The folder holds no Markdown file."})
		return
	}
	d.To, d.Into = targetOf(dest)
	saveImportDraft(c, d)
	st := importState{draft: d, dest: dest}
	st.preview, st.taken, st.notices, st.unanswered = s.importPreview(r.Context(), c, d, dest)
	s.writePage(w, r, http.StatusOK, s.importPage(c, st))
}

func isMarkdown(name string) bool {
	ext := strings.ToLower(path.Ext(name))
	return ext == ".md" || ext == ".markdown"
}

// readWhole reads one file of the folder: an import has no ceiling of its
// own (Р-73, the owner's decision of 22.10.2026).
func readWhole(ctx context.Context, prov *file.Provider, p string) ([]byte, error) {
	rc, _, err := prov.Get(ctx, p, nil)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
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
//
// A collection that does not answer leaves what is already there untold
// (s-io-19; Typecase's note 111 p. 9): one notice, the preview stays, and
// unanswered is what Import says it is off for.
func (s *Server) importPreview(ctx context.Context, c *collectionCtx, d *importDraft, dest *collectionCtx) (tables []view.ImportTable, taken map[string]bool, notices []view.Notice, unanswered string) {
	taken = map[string]bool{}
	if dest != nil {
		var err error
		if taken, err = destUIDs(ctx, dest); err != nil {
			taken = map[string]bool{}
			unanswered = string(dest.stamp()) + " did not answer"
			// Try again asks the same preview once more, the file kept; no
			// technical block — there was no answer (Typecase's note 113
			// p. 8).
			notices = append(notices, view.Notice{Role: view.NoticeAlert, Runs: importSilent(unanswered, c.spec.kind),
				Actions: []view.Action{{ID: "import.again", Label: "Try again", Href: s.importHref(c, "", dest.conn.ID, dest.col.Path), Method: "GET", Enabled: true}}})
		}
	}
	return []view.ImportTable{previewTable(c.spec.kind, fitDraft(d, dest), taken)}, taken, notices, unanswered
}

// fitDraft is the draft as the collection chosen can take it: a record
// larger than its max-resource-size (RFC 4791) is not sent — it stands among
// what is not written, with the sizes (S-19). Stalwart takes records up to
// 512 KB and answers a larger one with 412 (acceptance on our test servers,
// 18.11.2026). The draft kept in the session is not changed: another
// collection may take it.
func fitDraft(d *importDraft, dest *collectionCtx) *importDraft {
	if d == nil || dest == nil || dest.col.MaxResourceSize <= 0 {
		return d
	}
	limit := dest.col.MaxResourceSize
	var fit []importRecord
	var over []importSkip
	for _, rec := range d.Records {
		if n := int64(len(rec.Doc)); n > limit {
			over = append(over, importSkip{What: rec.File, Title: rec.Title, Detail: rec.Detail, Failed: true,
				Reason: fmt.Sprintf("Too large: %s — %s takes records up to %s", formatSize(n), dest.stamp(), formatSize(limit))})
			continue
		}
		fit = append(fit, rec)
	}
	if len(over) == 0 {
		return d
	}
	out := *d
	out.Records = fit
	out.Skipped = append(over, d.Skipped...)
	return &out
}

// intoNoun is what «Into» chooses, by kind (s-io-17, -19).
// importSilent is the alert of a preview whose collection did not answer
// (s-io-19).
func importSilent(unanswered string, kind view.Kind) view.Runs {
	return join(strongRun(unanswered+"."),
		" Nothing is written; try again, or choose another "+intoNoun[kind]+" under «Into».")
}

var intoNoun = map[view.Kind]string{
	view.KindContact: "address book", view.KindEvent: "calendar", view.KindTask: "list of tasks", view.KindNote: "notebook",
}

// previewTable is the preview as the frames draw it (s-io-5…8): one table,
// the columns of the kind and Result last; the records first, what the file
// holds and this import does not write after them, each with why. Only the
// notes, read from many files, have the file first; one file is named by
// the head (Typecase's note 157).
func previewTable(k view.Kind, d *importDraft, taken map[string]bool) view.ImportTable {
	t := view.ImportTable{Columns: previewColumns[k], Lines: []view.ImportLine{}}
	row := func(file string, cells ...view.Text) []view.Text {
		if k != view.KindNote {
			return cells
		}
		if file == "" {
			file = d.Source
		}
		return append([]view.Text{text(file)}, cells...)
	}
	for _, rec := range d.Records {
		result := "Ready"
		switch {
		case taken[rec.UID] && d.Existing == existingSkip:
			result = "Already here — skipped"
		case taken[rec.UID] && d.Existing == existingReplace:
			result = "Replaces the one here"
		case taken[rec.UID]:
			result = "New UID will be assigned"
		case rec.Ready != "":
			result += " — " + rec.Ready
		}
		// A contact's third cell is its first address (the owner's decision
		// of 02.11.2026): a person tells two cards apart by it.
		res := view.ImportReady
		if taken[rec.UID] && d.Existing == existingSkip {
			res = view.ImportSkipped
		}
		t.Lines = append(t.Lines, view.ImportLine{Cells: row(rec.File, text(rec.Title), text(rec.Detail), text(result)),
			Result: res, Untitled: rec.Untitled})
	}
	for _, sk := range d.Skipped {
		result := sk.Reason
		if !sk.Failed {
			result = "Skipped: " + lowerFirst(strings.TrimSuffix(sk.Reason, "."))
			if sk.Where != "" {
				result += " — import it into " + sk.Where
			}
		}
		// The third column is the record's own date, its component a badge
		// at its title (Им8, Им9); a file of a set is in the first column.
		line := view.ImportLine{Result: view.ImportSkipped}
		if sk.Failed {
			line.Result = view.ImportFailed
		}
		file, title := "", sk.Title
		switch {
		case k == view.KindNote:
			file = sk.What
		case !sk.Failed:
			line.Component = text(sk.What)
		}
		if title == "" {
			title, line.Untitled = sk.named(), true
			if k == view.KindNote {
				title = "—"
			}
		}
		line.Cells = row(file, text(title), text(sk.Detail), text(result))
		t.Lines = append(t.Lines, line)
	}
	t.Count = len(t.Lines)
	return t
}

// previewColumns are the columns of the preview by kind (s-io-5…8).
var previewColumns = map[view.Kind][]view.Text{
	view.KindContact: {"Name", "Email", "Result"},
	view.KindEvent:   {"Summary", "When", "Result"},
	view.KindTask:    {"Summary", "Due", "Result"},
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

// destUIDs are the identities the destination already holds. A collection
// holds one resource per UID whatever its component (RFC 4791 §5.3.2, RFC
// 6352 §6.3.2), so a calendar is read whole, not by the kind alone.
func destUIDs(ctx context.Context, dest *collectionCtx) (map[string]bool, error) {
	recs, err := destRecords(ctx, dest)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for uid := range recs {
		out[uid] = true
	}
	return out, nil
}

// destRecords are the records the destination holds, by identity: one whole
// read.
func destRecords(ctx context.Context, dest *collectionCtx) (map[string]*model.Object, error) {
	out := map[string]*model.Object{}
	if dest.card != nil {
		objs, err := dest.card.Query(ctx, dest.col.Path)
		if err != nil {
			return nil, err
		}
		for _, obj := range objs {
			out[card.UID(obj)] = obj
		}
		return out, nil
	}
	for _, p := range calendarParts {
		// Asked about what it holds alone: Mail.ru answers a question about
		// tasks in a calendar of events with 400 (docs/servers.md, SV-71).
		if !dest.col.Has(p.kind) {
			continue
		}
		objs, _, err := dest.cal.Query(ctx, dest.col.Path, p.component, nil, nil)
		if err != nil {
			return nil, err
		}
		for _, obj := range objs {
			out[cal.UID(obj)] = obj
		}
	}
	return out, nil
}

// runImport writes the draft. A record whose identity the collection
// already holds is skipped, written over the one there, or created under a
// fresh identity, as the person chose on the preview (importDraft.Existing;
// S-19) — and the row says which. Every other record is created. Nothing is
// overwritten that the person did not ask to replace, and nothing is deleted.
func (s *Server) runImport(w http.ResponseWriter, r *http.Request, c *collectionCtx, dest *collectionCtx) {
	d, has := loadImportDraft(c)
	if !has {
		s.writePage(w, r, http.StatusOK, s.importPage(c, importState{dest: dest, notices: []view.Notice{{
			Role: view.NoticeAlert, Title: nothingRead,
			Body: "There is no file to import: the session ended. Choose it again.",
		}}}))
		return
	}
	if dest == nil {
		st := importState{draft: d, dest: nil}
		st.preview, st.taken, st.notices, st.unanswered = s.importPreview(r.Context(), c, d, nil)
		s.writePage(w, r, http.StatusOK, s.importPage(c, st))
		return
	}
	ctx := r.Context()
	d = fitDraft(d, dest)
	taken, err := destUIDs(ctx, dest)
	if err != nil {
		// The same screen as the preview's: the table stays (s-io-19).
		st := importState{draft: d, dest: dest}
		st.preview, st.taken, st.notices, st.unanswered = s.importPreview(ctx, c, d, dest)
		s.writePage(w, r, http.StatusOK, s.importPage(c, st))
		return
	}

	// The records already there, by what was chosen for them: skipped ones
	// leave the list before any identity is settled; replaced ones keep
	// theirs, and are written at the address and over the version there.
	var held map[string]*model.Object
	var here []importRecord
	records := d.Records
	skipped := d.Skipped
	switch d.Existing {
	case existingSkip:
		records = nil
		for _, rec := range d.Records {
			if taken[rec.UID] {
				// «Already here, skipped» of the report (s-io-9), apart from
				// what the file held of another kind.
				here = append(here, rec)
				continue
			}
			records = append(records, rec)
		}
	case existingReplace:
		if held, err = destRecords(ctx, dest); err != nil {
			st := importState{draft: d, dest: dest}
			st.preview = []view.ImportTable{}
			st.notices = []view.Notice{s.unavailableNotice(dest, s.importHref(c, "", dest.conn.ID, dest.col.Path), err)}
			s.writePage(w, r, http.StatusOK, s.importPage(c, st))
			return
		}
		// An identity about to be replaced is not taken: its record keeps it.
		free := make(map[string]bool, len(taken))
		for uid := range taken {
			if held[uid] == nil {
				free[uid] = true
			}
		}
		taken = free
	}

	report := &view.ImportReport{Here: len(here)}
	for _, rec := range here {
		report.Heres = append(report.Heres, text(rec.Title+" · "+rec.UID))
	}
	for _, sk := range skipped {
		if sk.Failed {
			report.Failed++
			// Its name, then the phrase the preview gave it; a record
			// without a name, the phrase alone (Typecase's notes 111 p. 6,
			// 113 p. 9).
			line := sk.Reason
			if n := sk.named(); n != "—" {
				line = n + " · " + line
			}
			report.Failures = append(report.Failures, text(line))
		} else {
			report.Skipped++
			report.Skips = append(report.Skips, text(sk.line()))
		}
	}
	written := view.ImportTable{Rubric: text("Created in " + collectionLabel(dest.col)), Columns: importColumns(c.spec.kind)}
	// Every identity is settled before the first write, so that a record
	// linking to another of the file names the identity that one will have
	// (S-19): links inside the file travel with it.
	uids, errs, moved := planImportUIDs(records, taken)
	props := linkProps(dest)
	var made []importMade
	for i, rec := range records {
		obj, err := dest.parse("", "", rec.Doc)
		if err == nil {
			err = errs[i]
		}
		if err == nil && dest.card != nil {
			obj = withFN(obj)
		}
		uid, renamed, note := uids[i], uids[i] != rec.UID, "Created"
		// Why the record has another identity here (s-io-9; Typecase's note
		// 113 pp. 2–3): a copy beside the one here, as the answer asked —
		// «Imported as copies» —, or what was not the answer — a second
		// record of the file under an identity the first took, one the file
		// gave none.
		why, asked := "", false
		switch {
		case !renamed:
		case rec.UID != "" && taken[rec.UID]:
			asked = true
		case rec.UID != "":
			why = "the file holds this UID twice"
		default:
			why = "the file gave it none"
		}
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
		// The file's version over the one there, with the version read a
		// moment ago: a record changed since is refused, not overwritten.
		over := held[uid]
		if over != nil && renamed {
			over = nil
		}
		if err == nil && over != nil {
			obj.Path, obj.ETag = over.Path, over.ETag
			if err = dest.update(ctx, obj); err == nil {
				note = "Replaced the one here with the file's version"
				held[uid] = nil // a second record of the file under this identity is a copy
			}
		} else if err == nil {
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
					why = "the account holds this UID in another collection"
					note = "Created under " + fresh + ": this server holds one record per UID across the account, and another of its collections holds this one"
				}
			}
		}
		if err != nil {
			// The request and the answer, as the menu of refusals writes a
			// refusal (states-16): «PUT …/5c81.ics → 400 Bad Request».
			why := err.Error()
			var se *dav.StatusError
			if errors.As(err, &se) && se.Method != "" {
				why = refusalAnswer(view.SourceError, err, "")
			}
			report.Failed++
			report.Failures = append(report.Failures, text(rec.Title+" · "+why))
			continue
		}
		if over != nil {
			report.Replaced++
			report.Replaces = append(report.Replaces, text(rec.Title+" · "+uid))
		} else {
			report.Created++
		}
		if renamed {
			report.Renamed++
		}
		made = append(made, importMade{row: len(written.Lines), uid: uid, from: rec.UID, was: was, path: obj.Path, renamed: renamed, obj: obj, title: rec.Title, why: why, asked: asked})
		written.Lines = append(written.Lines, view.ImportLine{Cells: []view.Text{text(rec.Title), text(rec.Detail), text(note)}})
	}
	s.importLanded(ctx, dest, report, &written, made)
	// The records under another identity than the file's, and why (s-io-9,
	// «With a new UID»; Typecase's note 111 p. 7).
	for _, m := range made {
		switch {
		case m.held == m.from:
		case m.asked && m.why == "":
			report.Renames = append(report.Renames, text(m.title+" · "+m.from+" → "+m.held))
		case m.from == "":
			report.NewUIDs = append(report.NewUIDs, text(m.title+" · "+m.held+" · "+m.why))
		default:
			report.NewUIDs = append(report.NewUIDs, text(m.title+" · "+m.from+" → "+m.held+" · "+m.why))
		}
	}

	clearImportDraft(c)
	s.writePage(w, r, http.StatusOK, s.importPage(c, importState{dest: dest, report: report}))
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
	// title and why are what the report says of a record under another
	// identity than the file's (s-io-9); asked — the answer «Import as
	// copies» gave it, and it stands under «Imported as copies»
	title, why string
	asked      bool
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
//
// The same read says what the server kept of each record (S-7.1; Б35 of the
// audit of 11.10.2026: an import was the one write nobody read back). A
// record that lost a part of itself says so in its row, and lossy counts
// them for the note of the report.
func (s *Server) importLanded(ctx context.Context, dest *collectionCtx, report *view.ImportReport, written *view.ImportTable, made []importMade) (lossy int) {
	for i := range made {
		made[i].held = made[i].uid
	}
	if len(made) == 0 {
		return 0
	}
	held, err := destRecords(ctx, dest)
	if err != nil {
		return 0 // the rows stand as the writes were answered
	}
	kept := func(m *importMade, got *model.Object) {
		if got == nil || m.obj == nil {
			return
		}
		if loss := writeLoss(dest.conn.ID, m.obj, got, nil, true); loss != nil && len(loss.Properties) > 0 {
			written.Lines[m.row].Cells[2] = text(string(written.Lines[m.row].Cells[2]) + ". The server did not keep " + lossSaid(loss))
			// «Not kept by the server» of the report (s-io-9; Typecase's
			// note 111 p. 8): the record and what it lost.
			report.Lost++
			report.Losts = append(report.Losts, text(m.title+" · "+lossSaid(loss)))
			lossy++
		}
	}
	gave := map[string]string{} // written under → held under
	for i := range made {
		m := &made[i]
		if m.was != "" {
			gave[m.was] = m.uid // what was written before it names it still
		}
		if got := held[m.uid]; got != nil {
			kept(m, got)
			continue
		}
		at, err := dest.fetch(ctx, m.path)
		if err != nil {
			written.Lines[m.row].Cells[2] = text("Created, but the server does not show it under this identity or at the address it was written to")
			// Written and not shown at all: kept by the server — nothing of
			// it (s-io-9; the frames do not draw it, task 222 of Typecase
			// p. 9).
			report.Lost++
			report.Losts = append(report.Losts, text(m.title+" · not there after it was written"))
			continue
		}
		theirs := uidOf(dest, at)
		if theirs == "" || theirs == m.uid {
			kept(m, at)
			continue // there under its own identity; the listing missed it
		}
		written.Lines[m.row].Cells[2] = text("Created under " + theirs + ": the server gave it an identity of its own")
		kept(m, at)
		gave[m.uid] = theirs
		m.held, m.why = theirs, "the server gave its own"
		dest.noteWrite(m.path, at)
		if !m.renamed {
			report.Renamed++
		}
	}
	followTheirs(ctx, dest, written.Lines, made, gave)
	return lossy
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
		if err = dest.place(ctx, dst, obj); err == nil {
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

// uploadedImports are the files the import form sent, in their order (the
// field «file»; several when notes are dropped at once). An import has no
// ceiling of its own (Р-73, the owner's decision of 22.10.2026): the body
// comes as a stream (isFileUpload), read part by part, nothing of it on
// Carrel's disk.
func uploadedImports(r *http.Request) ([]importFile, error) {
	up, err := uploadOf(r)
	if err != nil {
		return nil, errNoImportFile
	}
	var out []importFile
	for part := up.next; ; part = nil {
		if part == nil {
			if part, err = up.mr.NextPart(); errors.Is(err, io.EOF) {
				break
			} else if err != nil {
				return nil, err
			}
		}
		if part.FormName() != "file" || part.FileName() == "" {
			if err := up.keep(part); err != nil {
				return nil, err
			}
			continue
		}
		body, err := io.ReadAll(part)
		if err != nil {
			return nil, err
		}
		out = append(out, importFile{name: path.Base(partName(part)), body: body})
	}
	if len(out) == 0 {
		return nil, errNoImportFile
	}
	return out, nil
}

// importFile is a file the import form sent.
type importFile struct {
	name string
	body []byte
}

// errOneFileOnly refuses several files where one is taken: contacts, events
// and tasks come in one file each (s-io-1).
var errOneFileOnly = errors.New("several files at once")

// errRead is a file that was read and found no good (s-io-18): the
// refusal begins with «Nothing was read.» and says why.
type errRead struct{ why string }

func (e errRead) Error() string { return e.why }

// readNoteFiles are several files picked at once for a notebook (Typecase's
// note 111 p. 3): each read as one file is — a Markdown file, a zip of
// them, an .ics of journal entries —, a file that is none of these skipped
// and named, one that cannot be read named as failed; the head counts them.
func (s *Server) readNoteFiles(c *collectionCtx, files []importFile) *importDraft {
	d := &importDraft{Kind: c.spec.kind, Account: c.conn.ID, Col: c.col.Path, Source: plural(len(files), "file"), Picked: true}
	taken := 0
	for _, f := range files {
		d.Files++
		d.Size += int64(len(f.body))
		if !isMarkdown(f.name) && !isZip(f.body) && !strings.HasSuffix(strings.ToLower(f.name), ".ics") {
			d.Skipped = append(d.Skipped, importSkip{What: f.name, Reason: "Not Markdown."})
			continue
		}
		taken++
		if isMarkdown(f.name) && !isZip(f.body) {
			// A Markdown file of the set as before: an empty one named and
			// left out, the rest each a note of its own.
			d.takeMarkdown(c, f.name, f.body, time.Time{})
			continue
		}
		one, err := s.readImport(c, f.name, f.body)
		if err != nil {
			d.Skipped = append(d.Skipped, importSkip{What: f.name, Reason: sentence(err.Error()), Failed: true})
			continue
		}
		// A zip of the set is one file of it; its members are named in it
		// (Typecase's note 113 p. 12: «‹архив›.zip › ‹имя›.md»).
		within := func(member string) string {
			if member == "" {
				return f.name
			}
			if isZip(f.body) {
				return f.name + " › " + member
			}
			return member
		}
		for _, rec := range one.Records {
			rec.File = within(rec.File)
			d.Records = append(d.Records, rec)
		}
		for _, sk := range one.Skipped {
			if !strings.HasPrefix(sk.What, "V") {
				sk.What = within(sk.What)
			}
			d.Skipped = append(d.Skipped, sk)
		}
	}
	if taken == 0 {
		d.Note = "None of these files is Markdown."
	}
	return d
}

// importRefusal says why a file was not taken. Nothing was written, and no
// draft is left.
func (s *Server) importRefusal(err error) view.Notice {
	// One notice over the step, the frames' words (s-io-18; Typecase's note
	// 111 p. 9). There is no ceiling of an import to be over (Р-73).
	// A refusal of a file read and found no good begins with «Nothing was
	// read.»; one where no file was read does not (Typecase's note 113
	// p. 5).
	n := view.Notice{Role: view.NoticeAlert, Title: nothingRead}
	var wrong errNotThisFile
	var bad errRead
	switch {
	case errors.Is(err, errNoImportFile):
		n.Title, n.Body = "", "No file was chosen."
	case errors.Is(err, errOneFileOnly):
		n.Title, n.Body = "", oneFileOnly+"."
	case errors.As(err, &bad):
		n.Body = text(bad.why)
	case errors.As(err, &wrong):
		n.Runs = join("«"+wrong.name+"» is not "+wrong.what+" file: its first line is not ", monoRun(wrong.begin), ". Choose the file again.")
	default:
		n.Body = "Carrel could not read this file."
		n.Details = &view.Diagnostic{Summary: "Import", Lines: []view.Text{text(err.Error())}}
	}
	return n
}

// nothingRead is the title of a refusal of the first step (s-io-18).
const nothingRead = "Nothing was read."

// errNotThisFile is a file whose first line is not what its kind begins
// with (s-io-18): a vCard file begins with BEGIN:VCARD, an iCalendar one
// with BEGIN:VCALENDAR.
type errNotThisFile struct{ name, what, begin string }

func (e errNotThisFile) Error() string {
	return e.name + " is not " + e.what + " file"
}

// firstLine is the first line of a file that holds something, without a
// byte order mark and the spaces around it.
func firstLine(body []byte) string {
	body = bytes.TrimPrefix(body, []byte("\xef\xbb\xbf"))
	for _, l := range strings.Split(string(body), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return ""
}

// combinedBackRail is the rail of the wizard opened from a combined view:
// the way back is that view, and no collection is open on it.
func (s *Server) combinedBackRail(c *collectionCtx) view.Rail {
	return s.backRail(c.sess, s.catalogOf(c.sess), c.spec, c.conn.ID, s.backAction(c.spec, "", "/app/"+c.spec.segment))
}
