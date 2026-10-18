package web

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"carrel.dev/carrel/server/internal/backup"
	"carrel.dev/carrel/server/internal/dav"
	"carrel.dev/carrel/server/internal/model"
	"carrel.dev/carrel/server/internal/provider/cal"
	"carrel.dev/carrel/server/internal/provider/card"
	"carrel.dev/carrel/server/internal/provider/file"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// Restore (S-24.3) is an import, not a rollback: each collection of the
// archive goes where its row says (s-backup-8) — the collection it came from
// by default, another, a new one, or nowhere —, what is missing there is
// created and nothing there is deleted or overwritten, so nothing made after
// the copy is lost. It is checked on the same screen before anything is
// written: the archive is opened, its manifest read and each collection's
// outcome said in its row. Three sources: a run of a job, a file on a WebDAV storage, a
// file from this computer. The archive waits for the next step in a temporary
// file of this session, never in memory and never on the volume's data: it is
// deleted when the restore ends or the session does.

// restoreTask is a restore in progress: the archive on disk, its password
// and its manifest. As a session task it is cancelled — the file deleted —
// when the session ends.
type restoreTask struct {
	id       string
	path     string
	password string
	file     string // where the archive lay: the run's file, the path on the storage, the uploaded name
	manifest backup.Manifest
	mu       sync.Mutex
	gone     bool // cancelled: the file is deleted
	// choice is the matching (s-backup-8), preset on the first look; counts
	// are what each row's target was found to hold, read once.
	choice restoreChoice
	counts map[string]restoreCount
}

func (t *restoreTask) ID() string    { return t.id }
func (t *restoreTask) Running() bool { return !t.ended() }

func (t *restoreTask) Cancel() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.gone {
		t.gone = true
		_ = os.Remove(t.path)
	}
}

func (t *restoreTask) ended() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.gone
}

// restoreOf is the session's open restore, if any.
func restoreOf(sess *session.Session) *restoreTask {
	for _, t := range sess.Tasks() {
		if rt, ok := t.(*restoreTask); ok && !rt.ended() {
			return rt
		}
	}
	return nil
}

// endRestore lets go of every restore of the session.
func endRestore(sess *session.Session) {
	for _, t := range sess.Tasks() {
		if rt, ok := t.(*restoreTask); ok {
			rt.Cancel()
		}
	}
}

// open reads the archive again from its file.
func (t *restoreTask) open() (*zip.Reader, *os.File, error) {
	f, err := os.Open(t.path)
	if err != nil {
		return nil, nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	zr, _, err := backup.OpenAt(f, st.Size(), t.password)
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	return zr, f, nil
}

// restoreBlock is the restore part of the Backup screen: which archive, then
// each of its collections and where it goes (s-backup-8).
// run names the copy a row of the history opened it with (Нс56): that one
// is the job's row, chosen.
func (s *Server) restoreBlock(sess *session.Session, blob sealed.Blob, from, run string) view.RestoreBlock {
	b := view.RestoreBlock{Note: join("Restoring ", strongRun("creates"), " records, the way import does: a colliding UID gets a fresh one and a line in the report. "+
		"Nothing on the server is deleted or overwritten — Carrel does not promise “exactly as it was”, because that would mean erasing somebody else’s changes.")}
	// A job's copies: one row a job, its newest verified copy — its day, its
	// size, how many collections (s-backup-7, Нс74).
	var runs, stores []view.Option
	byJob := s.users.BackupRuns(sess.AccountID())
	tf := s.timeFor(sess)
	for _, job := range blob.BackupJobs {
		var newest *store.BackupRun
		for i, r := range byJob[job.ID] {
			if r.File != "" && r.Verified && (newest == nil || r.Started.After(newest.Started)) {
				newest = &byJob[job.ID][i]
			}
		}
		for i, r := range byJob[job.ID] {
			if r.File != "" && r.Verified && job.ID+"|"+r.ID == run {
				newest = &byJob[job.ID][i]
			}
		}
		if newest != nil {
			runs = append(runs, view.Option{Value: job.ID + "|" + newest.ID, Label: text(job.Name),
				Meta: text(string(tf.numStamp(newest.Started)) + " · " + string(formatSize(newest.Size)) + " · " + plural(newest.Collections, "collection"))})
		}
	}
	for _, conn := range blob.Connections {
		for _, col := range blob.CollectionsOf(conn.ID) {
			if col.Has(view.KindFile) && !col.Gone {
				stores = append(stores, view.Option{Value: selValue(conn.ID, col.Path), Label: text(string(stamp(connLabel(conn), collectionLabel(col))))})
			}
		}
	}
	// Where the archive is: one of three, an address each (s-backup-7,
	// Нс74) — a job's copy when there is one, else a server.
	switch from {
	case "job", "server", "file":
	default:
		from = "server"
		if len(runs) > 0 {
			from = "job"
		}
	}
	for _, f := range []struct{ id, label string }{{"job", "From a job"}, {"server", "From a server"}, {"file", "From this computer"}} {
		b.From = append(b.From, view.Action{ID: "restore.from." + f.id, Label: text(f.label), Method: "GET", Enabled: true, On: f.id == from,
			Href: s.route("/app/settings/backup?" + url.Values{"restore": {"1"}, "from": {f.id}}.Encode())})
	}
	b.FromHint = "One archive, one of three. The other two stay empty until you switch."
	source := map[string]string{"job": "run", "server": "webdav", "file": "upload"}[from]
	rows := []view.FormRow{csrfField(sess), {Name: "source", Kind: view.InputHidden, Value: text(source)}}
	switch from {
	case "job":
		chosen := 0
		for i := range runs {
			if runs[i].Value == run {
				chosen = i
			}
		}
		for i := range runs {
			runs[i].Selected = i == chosen
		}
		rows = append(rows, view.FormRow{Name: "run", Label: "From a job", Kind: view.InputRadio, Options: runs})
	case "server":
		rows = append(rows,
			view.FormRow{Name: "store", Label: "Storage", Kind: view.InputSelect, Options: stores},
			view.FormRow{Name: "file", Label: "File in it", Kind: view.InputText, Hint: "The path of the archive under the storage's root, such as backups/carrel-work-2026-08-16T031000Z.zip.age."})
	}
	rows = append(rows, view.FormRow{Name: "password", Label: "Archive password", Kind: view.InputPassword,
		Hint: "The one set in the job, not your sign-in password. Without it the manifest cannot be read — Carrel will not even see the list of collections."})
	b.Source = s.plainForm("/app/settings/backup?op=restore-open", "POST", rows...)
	open := view.Action{ID: "restore.open", Label: "Open the manifest", Primary: true, Href: s.postQuery("/app/settings/backup", url.Values{"op": {"restore-open"}}), Method: "POST", Enabled: true}
	if from == "file" {
		// The archive comes with the form, from this computer.
		open.ID, open.Icon, open.File = "restore.upload", "i-upload", "archive"
		open.Href = s.postQuery("/app/settings/backup", url.Values{"op": {"restore-open"}, "source": {"upload"}})
	}
	b.Source.Actions = []view.Action{open,
		{ID: "restore.close", Label: "Cancel", Href: s.route("/app/settings/backup"), Method: "GET", Enabled: true}}
	b.Title, b.Subtitle = "Restore", line("Nothing is written until you have seen what goes where")
	if t := restoreOf(sess); t != nil {
		b.Open = true
		b.Subtitle = archiveLine(t)
		if !t.manifest.Taken.IsZero() {
			b.Title = text("Restore from " + string(tf.dayKey(t.manifest.Taken)))
		}
		b.Match = s.restoreMatch(sess, blob, t)
		b.Actions = []view.Action{{ID: "restore.cancel", Label: "Cancel", Href: s.postQuery("/app/settings/backup", url.Values{"op": {"restore-cancel"}}), Method: "POST", Enabled: true}}
	}
	return b
}

// The choices of a row's «Restore into» besides the collections.
const (
	restoreNew  = "new"
	restoreSkip = "skip"
)

// restoreChoice is what the matching says of every collection of the
// archive: where it goes and whether its row is ticked. The task keeps it
// between the steps; a form of the step sends it again.
type restoreChoice struct {
	into   []string
	picked []bool
}

// restoreTarget is where one collection of the archive can go: a collection
// that takes its kind and can be written, the new one, or nowhere.
func restoreTargets(blob sealed.Blob, k view.Kind) []sealed.Collection {
	var out []sealed.Collection
	for _, conn := range blob.Connections {
		if conn.Feed {
			continue
		}
		for _, col := range blob.CollectionsOf(conn.ID) {
			if col.Has(k) && !col.Gone && !col.ReadOnly && !col.Feed {
				out = append(out, col)
			}
		}
	}
	return out
}

// newRestoreConn is the server a «New collection» of an archived one is made
// on: its own when it is still there and makes collections of the kind, else
// the first that does. A file collection gets a new folder instead, in the
// collection it matched or the first that takes writes.
func newRestoreConn(blob sealed.Blob, e backup.Entry) (sealed.Connection, bool) {
	if e.Kind == view.KindFile {
		return sealed.Connection{}, false
	}
	if conn, ok := blob.Connection(e.AccountID); ok && !conn.Feed && homeFor(conn, e.Kind) != "" {
		return conn, true
	}
	for _, conn := range blob.Connections {
		if !conn.Feed && homeFor(conn, e.Kind) != "" {
			return conn, true
		}
	}
	return sealed.Connection{}, false
}

// newRestoreFolder is the file collection a «New collection» of archived
// files makes its folder in.
func newRestoreFolder(blob sealed.Blob, e backup.Entry) (sealed.Collection, bool) {
	if col, ok := matchedByPath(blob, e); ok {
		return col, true
	}
	if all := restoreTargets(blob, view.KindFile); len(all) > 0 {
		return all[0], true
	}
	return sealed.Collection{}, false
}

// matchedByPath is the collection of the catalog an archived one came from:
// the same connection, the same path, there still and taking writes.
func matchedByPath(blob sealed.Blob, e backup.Entry) (sealed.Collection, bool) {
	col, ok := blob.Collection(e.AccountID, e.Path)
	if !ok || col.Gone || col.ReadOnly || col.Feed || !col.Has(e.Kind) {
		return sealed.Collection{}, false
	}
	return col, true
}

// presetChoice is the matching before anyone chose: the collection matched
// by path; on a server that is gone, another collection of the kind; on a
// server that lost it, a new one. A collection the archive does not hold
// whole, or that has nowhere to go, is skipped.
func presetChoice(blob sealed.Blob, t *restoreTask) restoreChoice {
	var c restoreChoice
	for _, e := range t.manifest.Collections {
		into := restoreSkip
		_, newOK := newRestoreConn(blob, e)
		if _, folder := newRestoreFolder(blob, e); e.Kind == view.KindFile && folder {
			newOK = true
		}
		col, matched := matchedByPath(blob, e)
		_, connThere := blob.Connection(e.AccountID)
		others := restoreTargets(blob, e.Kind)
		switch {
		case e.Error != "":
		case matched:
			into = selValue(col.AccountID, col.Path)
		case !connThere && len(others) > 0:
			into = selValue(others[0].AccountID, others[0].Path)
		case newOK:
			into = restoreNew
		}
		c.into = append(c.into, into)
		c.picked = append(c.picked, into != restoreSkip)
	}
	return c
}

// validInto says whether v is a choice of «Restore into» for e: one of its
// targets, the new one where there is one, or skip. Nothing the form sends
// is taken otherwise.
func validInto(blob sealed.Blob, e backup.Entry, v string) bool {
	switch v {
	case restoreSkip:
		return true
	case restoreNew:
		if e.Kind == view.KindFile {
			_, ok := newRestoreFolder(blob, e)
			return ok
		}
		_, ok := newRestoreConn(blob, e)
		return ok
	}
	for _, col := range restoreTargets(blob, e.Kind) {
		if selValue(col.AccountID, col.Path) == v {
			return true
		}
	}
	return false
}

// postedChoice reads the matching form: into.N and pick.N of every row;
// a choice that is not one of the row's is the one kept before.
func postedChoice(r *http.Request, blob sealed.Blob, t *restoreTask, kept restoreChoice) restoreChoice {
	c := restoreChoice{into: append([]string{}, kept.into...), picked: make([]bool, len(kept.picked))}
	for i, e := range t.manifest.Collections {
		if v := r.PostFormValue("into." + strconv.Itoa(i)); validInto(blob, e, v) {
			c.into[i] = v
		}
		v := r.PostFormValue("pick." + strconv.Itoa(i))
		c.picked[i] = v == "on" || v == "true"
	}
	return c
}

// choiceOf is the task's matching, preset on the first look.
func (t *restoreTask) choiceOf(blob sealed.Blob) restoreChoice {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.choice.into) != len(t.manifest.Collections) {
		t.choice = presetChoice(blob, t)
	}
	return restoreChoice{into: append([]string{}, t.choice.into...), picked: append([]bool{}, t.choice.picked...)}
}

func (t *restoreTask) keep(c restoreChoice) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.choice = c
}

// restoreCount is what a collection of the archive would leave in an
// existing one: records missing there, and records whose UID is held there
// by different content.
type restoreCount struct {
	missing, changed int
	err              error
}

// countFor reads the target and compares; one read per row and target for
// the life of the archive.
func (s *Server) countFor(sess *session.Session, blob sealed.Blob, t *restoreTask, zr *zip.Reader, i int, e backup.Entry, into string) restoreCount {
	key := strconv.Itoa(i) + "\x00" + into
	t.mu.Lock()
	if n, ok := t.counts[key]; ok {
		t.mu.Unlock()
		return n
	}
	t.mu.Unlock()
	var n restoreCount
	objs, err := s.archiveRecords(zr, e)
	if err == nil {
		var there map[string]*model.Object
		ctx, cancel := context.WithTimeout(context.Background(), s.cfg.ProgressTotalTimeout)
		there, err = s.recordsThere(ctx, sess, blob, into, e.Kind)
		cancel()
		for _, o := range objs {
			switch ex, held := there[recordUID(o)]; {
			case !held:
				n.missing++
			case !sameRecord(o, ex):
				n.changed++
			}
		}
	}
	n.err = err
	t.mu.Lock()
	if t.counts == nil {
		t.counts = map[string]restoreCount{}
	}
	t.counts[key] = n
	t.mu.Unlock()
	return n
}

// recordUID is a record's UID, card or calendar.
func recordUID(o *model.Object) string {
	if o.IsCalendar() {
		return cal.UID(o)
	}
	return card.UID(o)
}

// sameRecord says whether two records hold the same: every property of
// either, in the other, with the same values; what a server stamps of its
// own accord (DTSTAMP, REV, …) is not the record's (model.CompareAfterPut).
func sameRecord(a, b *model.Object) bool {
	return model.CompareAfterPut("", a, b, nil) == nil && model.CompareAfterPut("", b, a, nil) == nil
}

// recordsThere are the records an existing collection holds, by UID.
func (s *Server) recordsThere(ctx context.Context, sess *session.Session, blob sealed.Blob, into string, k view.Kind) (map[string]*model.Object, error) {
	acc, path, _ := strings.Cut(into, "|")
	col, ok := blob.Collection(acc, path)
	conn, hasConn := blob.Connection(acc)
	if !ok || !hasConn {
		return nil, errors.New("the collection is not in the catalog")
	}
	dest, err := s.newCollectionCtx(sess, recordKinds[segmentOf(k)], conn, col)
	if err != nil {
		return nil, err
	}
	objs, err := dest.allRecords(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]*model.Object{}
	for _, o := range objs {
		out[recordUID(o)] = o
	}
	return out, nil
}

// restoreMatch is step two (s-backup-8): a row for every collection of the
// archive — what it is, how much, where it goes and what that will do —,
// and the bar that chooses for the ticked rows.
func (s *Server) restoreMatch(sess *session.Session, blob sealed.Blob, t *restoreTask) *view.RestoreMatch {
	zr, f, err := t.open()
	if err != nil {
		return nil
	}
	defer f.Close()
	choice := t.choiceOf(blob)
	m := &view.RestoreMatch{}
	bar := view.FormRow{Name: "into", Label: "Restore into", Kind: view.InputSelect}
	seen := map[string]bool{}
	going := 0
	for i, e := range t.manifest.Collections {
		col, matched := matchedByPath(blob, e)
		colour := catalogColour(sealed.Collection{AccountID: e.AccountID, Path: e.Path})
		if c, ok := blob.Collection(e.AccountID, e.Path); ok {
			colour = catalogColour(c)
		}
		row := view.RestoreRow{Name: text(e.Connection + " / " + e.Name), Colour: colour, Kind: e.Kind, Count: e.Count,
			Pick: view.FormRow{Name: "pick." + strconv.Itoa(i), Label: text(e.Connection + " / " + e.Name), Kind: view.InputCheck, Value: text(boolStr(choice.picked[i]))},
			Into: view.FormRow{Name: "into." + strconv.Itoa(i), Label: "Restore into", Kind: view.InputSelect},
		}
		for _, c := range restoreTargets(blob, e.Kind) {
			conn, _ := blob.Connection(c.AccountID)
			v := selValue(c.AccountID, c.Path)
			opt := view.Option{Value: v, Label: text(string(stamp(connLabel(conn), collectionLabel(c)))), Colour: catalogColour(c), Selected: choice.into[i] == v}
			row.Into.Options = append(row.Into.Options, opt)
			if !seen[v] {
				seen[v] = true
				opt.Selected = false
				bar.Options = append(bar.Options, opt)
			}
		}
		if validInto(blob, e, restoreNew) {
			row.Into.Options = append(row.Into.Options, view.Option{Value: restoreNew, Label: "New collection", Separator: true, Selected: choice.into[i] == restoreNew})
		}
		row.Into.Options = append(row.Into.Options, view.Option{Value: restoreSkip, Label: "Skip this one", Separator: !validInto(blob, e, restoreNew), Selected: choice.into[i] == restoreSkip})
		row.Will, row.Danger = s.restoreWill(sess, blob, t, zr, i, e, choice.into[i], matched && choice.into[i] == selValue(col.AccountID, col.Path))
		if choice.picked[i] {
			m.Selected++
			if choice.into[i] != restoreSkip {
				going++
			}
		}
		m.Rows = append(m.Rows, row)
	}
	bar.Options = append(bar.Options,
		view.Option{Value: restoreNew, Label: "New collection", Separator: true},
		view.Option{Value: restoreSkip, Label: "Skip this one"})
	m.Into = bar
	q := func(op, do string) string {
		v := url.Values{"op": {op}}
		if do != "" {
			v.Set("do", do)
		}
		return s.postQuery("/app/settings/backup", v)
	}
	m.Form = s.plainForm("/app/settings/backup?op=restore-run", "POST", csrfField(sess))
	// «Restore 3 collections» (s-backup-8): what the press restores.
	run := view.Action{ID: "restore.run", Label: text("Restore " + plural(going, "collection")), Href: q("restore-run", ""), Method: "POST", Primary: true, Enabled: going > 0}
	if going == 0 {
		run.Reason = "Tick a collection and choose where it goes."
	}
	m.Form.Actions = []view.Action{run,
		{ID: "restore.into", Label: "Restore into", Href: q("restore-match", "into"), Method: "POST", Enabled: true},
		{ID: "restore.skip", Label: "Skip", Href: q("restore-match", "skip"), Method: "POST", Enabled: true},
		{ID: "restore.all", Label: "Select all", Href: q("restore-match", "all"), Method: "POST", Enabled: true},
	}
	return m
}

// restoreWill is «What will happen» of a row, and whether it is an alarm.
func (s *Server) restoreWill(sess *session.Session, blob sealed.Blob, t *restoreTask, zr *zip.Reader, i int, e backup.Entry, into string, matched bool) (view.Text, bool) {
	if e.Error != "" {
		return text("not whole in this archive: " + e.Error), true
	}
	if into == restoreSkip {
		return "skip", false
	}
	// What the archive cannot give is said first: nothing of it is written.
	if e.Kind != view.KindFile {
		if _, err := s.archiveRecords(zr, e); err != nil {
			return text("cannot be restored: " + err.Error()), true
		}
	}
	_, connThere := blob.Connection(e.AccountID)
	switch {
	case !connThere && into != restoreNew:
		return "that server is gone — another collection chosen", true
	case e.Kind == view.KindFile && into == restoreNew:
		col, _ := newRestoreFolder(blob, e)
		conn, _ := blob.Connection(col.AccountID)
		return text("a new folder in " + string(stamp(connLabel(conn), collectionLabel(col))) + " — " + plural(e.Count, "file") + " will be uploaded"), false
	case into == restoreNew:
		conn, _ := newRestoreConn(blob, e)
		return text("a new collection on " + connLabel(conn) + " — " + plural(e.Count, "record") + " will be created"), false
	}
	prefix := ""
	if matched {
		prefix = "matched by path — "
	}
	if e.Kind == view.KindFile {
		return text(prefix + "missing files are uploaded, none is overwritten"), false
	}
	n := s.countFor(sess, blob, t, zr, i, e, into)
	if n.err != nil {
		return text(prefix + "cannot be read now: " + n.err.Error()), true
	}
	var parts []string
	if n.missing > 0 {
		parts = append(parts, plural(n.missing, "record")+" missing, will be created")
	}
	switch {
	case n.changed == 1:
		parts = append(parts, "1 changed since the copy, created again with a fresh UID")
	case n.changed > 1:
		parts = append(parts, strconv.Itoa(n.changed)+" changed since the copy, created again with fresh UIDs")
	}
	if len(parts) == 0 {
		return text(prefix + "no differences"), false
	}
	return text(prefix + strings.Join(parts, " · ")), false
}

// matchRestore is a press on the bar of the matching (s-backup-8): the form
// as it stands, and do= — into gives the ticked rows the bar's target (a row
// whose kind it does not take keeps its own), skip skips them and unticks
// them, all ticks every row. Nothing is written; the page comes back.
func (s *Server) matchRestore(w http.ResponseWriter, r *http.Request, sess *session.Session, blob sealed.Blob) {
	t := restoreOf(sess)
	if t == nil {
		s.writePage(w, r, http.StatusOK, s.backupPage(sess, nil, nil, restoreFailure("There is no archive open — the session may have ended. Open it again.")))
		return
	}
	c := postedChoice(r, blob, t, t.choiceOf(blob))
	switch r.URL.Query().Get("do") {
	case "into":
		v := r.PostFormValue("into")
		for i, e := range t.manifest.Collections {
			if c.picked[i] && validInto(blob, e, v) {
				c.into[i] = v
			}
		}
	case "skip":
		for i := range c.picked {
			if c.picked[i] {
				c.into[i], c.picked[i] = restoreSkip, false
			}
		}
	case "all":
		for i := range c.picked {
			c.picked[i] = true
		}
	}
	t.keep(c)
	s.redirect(w, r, "/app/settings/backup")
}

// archiveRecords are the records of one collection of the archive, read
// under the ceilings of an import (S-29.4, S-24.3: restoring is an import):
// a member is not held past CARREL_IMPORT_MAX_BYTES, and a collection of
// more than CARREL_IMPORT_MAX_RECORDS is refused whole, as the wizard
// refuses such a file. The archive is somebody else's file; what it says
// of itself is not what it is.
func (s *Server) archiveRecords(zr *zip.Reader, e backup.Entry) ([]*model.Object, error) {
	var out []*model.Object
	max := s.cfg.ImportMaxBytes
	for _, zf := range zr.File {
		if !strings.HasPrefix(zf.Name, e.Dir) || strings.HasPrefix(zf.Name, e.Dir+"files/") {
			continue
		}
		ext := path.Ext(zf.Name)
		if ext != ".vcf" && ext != ".ics" {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(io.LimitReader(rc, max+1))
		rc.Close()
		if err != nil {
			return nil, err
		}
		if int64(len(body)) > max {
			return nil, fmt.Errorf("%s is larger than an import takes (%s)", path.Base(zf.Name), formatSize(max))
		}
		var objs []*model.Object
		if ext == ".vcf" {
			objs, _, err = card.Split(body)
		} else {
			objs, _, err = cal.Split(body, time.Now())
		}
		if err != nil {
			return nil, err
		}
		out = append(out, objs...)
		if len(out) > s.cfg.ImportMaxRecords {
			return nil, errTooManyRecords{len(out), s.cfg.ImportMaxRecords}
		}
	}
	return out, nil
}

// openRestore is step one: the archive, from where it lies, into a
// temporary file of the session; then opened and its manifest read. A wrong
// password or a file that is no Carrel archive says so and keeps nothing.
func (s *Server) openRestore(w http.ResponseWriter, r *http.Request, sess *session.Session, blob sealed.Blob) {
	endRestore(sess)
	fields := r.PostForm
	var src io.Reader
	var closer io.Closer
	var name string
	source := r.URL.Query().Get("source")
	if source == "" {
		source = fields.Get("source")
	}
	if source == "upload" {
		up, err := uploadOf(r)
		if err != nil || up.next == nil {
			s.writePage(w, r, http.StatusOK, s.backupPage(sess, nil, nil, restoreFailure("No file came with the form.")))
			return
		}
		fields, src, name = up.fields, up.next, up.next.FileName()
	} else {
		var err error
		src, closer, name, err = s.restoreSource(r.Context(), sess, blob, source, fields)
		if err != nil {
			s.writePage(w, r, http.StatusOK, s.backupPage(sess, nil, nil, restoreFailure(err.Error())))
			return
		}
		defer closer.Close()
	}
	f, err := os.CreateTemp("", "carrel-restore-*")
	if err != nil {
		http.Error(w, "could not keep the archive", http.StatusInternalServerError)
		return
	}
	size, err := io.Copy(f, src)
	f.Close()
	t := &restoreTask{path: f.Name(), password: fields.Get("password"), file: name}
	if err != nil {
		t.Cancel()
		s.writePage(w, r, http.StatusOK, s.backupPage(sess, nil, nil, restoreFailure("The archive did not arrive whole: "+err.Error())))
		return
	}
	rf, err := os.Open(t.path)
	if err == nil {
		_, t.manifest, err = backup.OpenAt(rf, size, t.password)
		rf.Close()
	}
	if err != nil {
		t.Cancel()
		msg := "This is not a Carrel archive, or it is damaged: " + err.Error()
		if errors.Is(err, backup.ErrWrongPassword) {
			msg = "The password does not open this archive."
		}
		s.writePage(w, r, http.StatusOK, s.backupPage(sess, nil, nil, restoreFailure(msg)))
		return
	}
	id, err := user.NewTaskID()
	if err != nil {
		t.Cancel()
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	t.id = "restore-" + id
	sess.KeepTask(t)
	s.redirect(w, r, "/app/settings/backup")
}

// archiveLine is the line under the title of an open archive (s-backup-8):
// its job, what the manifest holds, and where it lay, monospaced.
func archiveLine(t *restoreTask) view.Runs {
	records := 0
	for _, e := range t.manifest.Collections {
		if e.Kind != view.KindFile {
			records += e.Count
		}
	}
	head := "Archive · "
	if t.manifest.Job != "" {
		head = "Job «" + t.manifest.Job + "» · "
	}
	parts := []any{head + "manifest read · " + plural(len(t.manifest.Collections), "collection") + " · " + grouped(records) + " " + pluralWord(records, "record")}
	if t.file != "" {
		parts = append(parts, " · ", monoRun(t.file))
	}
	return join(parts...)
}

// grouped is a count with its thousands apart, as the frames write it:
// «1 214».
func grouped(n int) string {
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + " " + s[i:]
	}
	return s
}

func pluralWord(n int, unit string) string {
	if n == 1 {
		return unit
	}
	return unit + "s"
}

func restoreFailure(body string) view.Notice {
	return view.Notice{Role: view.NoticeAlert, Title: "The archive could not be opened", Body: text(body)}
}

// restoreSource opens the archive a form names: a run of a job, or a file on
// a storage of the catalog.
func (s *Server) restoreSource(ctx context.Context, sess *session.Session, blob sealed.Blob, source string, f url.Values) (io.Reader, io.Closer, string, error) {
	switch source {
	case "run":
		jobID, runID, _ := strings.Cut(f.Get("run"), "|")
		job, ok := blob.BackupJob(jobID)
		if !ok {
			return nil, nil, "", errors.New("choose a copy")
		}
		for _, r := range s.users.BackupRuns(sess.AccountID())[jobID] {
			if r.ID == runID && r.File != "" && r.Verified {
				dest, err := s.backupDest(blob, job)
				if err != nil {
					return nil, nil, "", err
				}
				body, err := dest.Open(ctx, r.File)
				return body, body, r.File, err
			}
		}
		return nil, nil, "", errors.New("choose a copy")
	case "webdav":
		acc, colPath, _ := strings.Cut(f.Get("store"), "|")
		col, ok := blob.Collection(acc, colPath)
		conn, hasConn := blob.Connection(acc)
		segs, okPath := relSegs(f.Get("file"))
		if !ok || !hasConn || !col.Has(view.KindFile) || !okPath || len(segs) == 0 {
			return nil, nil, "", errors.New("choose a storage and the archive's path in it")
		}
		files, err := s.filesOf(conn)
		if err != nil {
			return nil, nil, "", err
		}
		p, err := file.Join(fileRoot(conn, col), segs...)
		if err != nil {
			return nil, nil, "", err
		}
		d, err := files.Open(ctx, p, nil)
		if err != nil {
			return nil, nil, "", err
		}
		return d.Body, d.Body, strings.Join(segs, "/"), nil
	}
	return nil, nil, "", errors.New("choose where the archive is")
}

// runRestore is step three: every ticked collection goes where its row says
// — records created in an existing collection or a new one, files uploaded
// —, the way an import does: nothing on the server is deleted or
// overwritten (S-24.3). The page says what went where.
func (s *Server) runRestore(w http.ResponseWriter, r *http.Request, sess *session.Session, key [32]byte, blob sealed.Blob) {
	t := restoreOf(sess)
	if t == nil {
		s.writePage(w, r, http.StatusOK, s.backupPage(sess, nil, nil, restoreFailure("There is no archive open — the session may have ended. Open it again.")))
		return
	}
	choice := postedChoice(r, blob, t, t.choiceOf(blob))
	t.keep(choice)
	zr, f, err := t.open()
	if err != nil {
		s.writePage(w, r, http.StatusOK, s.backupPage(sess, nil, nil, restoreFailure(err.Error())))
		return
	}
	// Closed before the restore ends: an open file cannot be deleted on every
	// system, and the archive must not outlive the restore.
	closeOnce := sync.OnceFunc(func() { f.Close() })
	defer closeOnce()
	ctx := r.Context()
	day := time.Now().In(s.tz()).Format("2006-01-02")
	var lines []view.Text
	failed := false
	for i, e := range t.manifest.Collections {
		into := choice.into[i]
		if !choice.picked[i] || into == restoreSkip {
			continue
		}
		var said []string
		var err error
		if e.Kind == view.KindFile {
			said, err = s.restoreFiles(ctx, blob, zr, e, into, day)
		} else {
			said, err = s.restoreRecords(ctx, sess, key, blob, zr, e, into, day)
		}
		if err != nil {
			failed = true
			said = append(said, e.Connection+" / "+e.Name+": "+err.Error())
		}
		for _, l := range said {
			lines = append(lines, text(l))
		}
	}
	if len(lines) == 0 {
		s.writePage(w, r, http.StatusOK, s.backupPage(sess, nil, nil, view.Notice{Role: view.NoticeInfo, Title: "Nothing was restored", Body: "Tick at least one collection and choose where it goes."}))
		return
	}
	closeOnce()
	endRestore(sess)
	n := view.Notice{Role: view.NoticeInfo, Title: "Restored", Body: "Nothing on the server was deleted or overwritten.",
		Details: &view.Diagnostic{Summary: "What went where", Lines: lines}}
	if failed {
		n.Role, n.Title = view.NoticeAlert, "Restored in part"
	}
	s.writePage(w, r, http.StatusOK, s.backupPage(sess, nil, nil, n))
}

// restoreRecords puts the records of one archived collection into the
// target — an existing collection, or a new one of its kinds on the server
// newRestoreConn names: a record missing there is created under its UID,
// the same record is left as it is, and a UID held there by different
// content gets a fresh UID and a line. The first line says the counts.
func (s *Server) restoreRecords(ctx context.Context, sess *session.Session, key [32]byte, blob sealed.Blob, zr *zip.Reader, e backup.Entry, into, day string) ([]string, error) {
	objs, err := s.archiveRecords(zr, e)
	if err != nil {
		return nil, err
	}
	var conn sealed.Connection
	var col sealed.Collection
	there := map[string]*model.Object{}
	if into == restoreNew {
		var ok bool
		if conn, ok = newRestoreConn(blob, e); !ok {
			return nil, errors.New("no server here makes collections of this kind")
		}
		kinds := e.Kinds
		if len(kinds) == 0 {
			kinds = []view.Kind{e.Kind}
		}
		if col, err = s.newCollectionOn(ctx, conn, kinds, e.Name+" (restored "+day+")"); err != nil {
			return nil, err
		}
		if err := s.users.AddCollection(sess.AccountID(), key, conn.ID, col); err != nil {
			return nil, err
		}
	} else {
		acc, path, _ := strings.Cut(into, "|")
		var ok, hasConn bool
		col, ok = blob.Collection(acc, path)
		conn, hasConn = blob.Connection(acc)
		if !ok || !hasConn || conn.Feed || col.ReadOnly {
			return nil, errors.New("the chosen collection does not take writes")
		}
		if there, err = s.recordsThere(ctx, sess, blob, into, e.Kind); err != nil {
			return nil, err
		}
	}
	dests := map[view.Kind]*collectionCtx{}
	made, same, refused := 0, 0, 0
	var renamed []string
	for _, o := range objs {
		k := view.KindContact
		if o.IsCalendar() {
			k = cal.Kind(o)
		}
		uid := recordUID(o)
		if !col.Has(k) {
			refused++
			continue
		}
		dest := dests[k]
		if dest == nil {
			if dest, err = s.newCollectionCtx(sess, recordKinds[segmentOf(k)], conn, col); err != nil {
				refused++
				continue
			}
			dests[k] = dest
		}
		if ex, held := there[uid]; held {
			if sameRecord(o, ex) {
				same++
				continue
			}
			fresh, err := model.NewUID()
			if err == nil {
				o, err = setUID(dest, o, fresh)
			}
			if err != nil {
				refused++
				continue
			}
			renamed = append(renamed, uid+" → "+fresh+": changed there since the copy")
			uid = fresh
		}
		if err := s.createImported(ctx, dest, o, uid); err != nil {
			refused++
			continue
		}
		made++
	}
	line := fmt.Sprintf("%s / %s → %s · %s: %s created", e.Connection, e.Name, connLabel(conn), collectionLabel(col), plural(made, "record"))
	if same > 0 {
		line += fmt.Sprintf(", %d already there", same)
	}
	if len(renamed) > 0 {
		line += fmt.Sprintf(", %d with a fresh UID", len(renamed))
	}
	out := append([]string{line}, renamed...)
	if refused > 0 {
		return out, fmt.Errorf("%d refused", refused)
	}
	return out, nil
}

// newCollectionOn makes a collection on a connection's home for kinds, under
// a free address from the name, and returns it as the catalog will hold it.
func (s *Server) newCollectionOn(ctx context.Context, conn sealed.Connection, kinds []view.Kind, name string) (sealed.Collection, error) {
	home := homeFor(conn, kinds[0])
	if home == "" {
		return sealed.Collection{}, errors.New("this server makes no collections of this kind")
	}
	client, err := dav.NewClient(s.guard, conn.Server, conn.Login, conn.Password, conn.CertFingerprint)
	if err != nil {
		return sealed.Collection{}, err
	}
	taken, err := childSegments(ctx, client, home)
	if err != nil {
		return sealed.Collection{}, err
	}
	f := collectionForm{kind: kinds[0], name: name}
	for _, k := range kinds {
		if c := componentOf(k); c != "" {
			f.components = append(f.components, c)
		}
	}
	p := strings.TrimSuffix(home, "/") + "/" + uniqueSlug(collectionSlug(name), taken) + "/"
	method, props, made := createRequest(f)
	if err := client.MkColProps(ctx, method, p, props); err != nil {
		return sealed.Collection{}, fmt.Errorf("%s %s: %w", method, p, err)
	}
	settled, err := settlePath(ctx, client, home, p, taken)
	if err != nil {
		return sealed.Collection{}, err
	}
	return sealed.Collection{Path: settled, Name: s.settleName(ctx, client, settled, name), Kinds: made, Components: f.components, AccountID: conn.ID}, nil
}

// restoreFiles puts the archived files of one collection into the target:
// at their places in an existing file collection, or under a new folder
// («Name (restored day)») in the collection newRestoreFolder names. A file
// missing there is uploaded; the same file there (the same bytes) is left as
// it is; a name taken by other bytes gets a fresh one beside it — nothing is
// overwritten. A path from the archive that would climb out is written
// nowhere, and the folder's name from the manifest is one segment (S-29.4).
func (s *Server) restoreFiles(ctx context.Context, blob sealed.Blob, zr *zip.Reader, e backup.Entry, into, day string) ([]string, error) {
	var col sealed.Collection
	if into == restoreNew {
		var ok bool
		if col, ok = newRestoreFolder(blob, e); !ok {
			return nil, errors.New("no storage here takes writes")
		}
	} else {
		acc, colPath, _ := strings.Cut(into, "|")
		var ok bool
		col, ok = blob.Collection(acc, colPath)
		if !ok || col.ReadOnly || col.Feed || !col.Has(view.KindFile) {
			return nil, errors.New("the chosen storage does not take writes")
		}
	}
	conn, hasConn := blob.Connection(col.AccountID)
	if !hasConn {
		return nil, errors.New("the chosen storage does not take writes")
	}
	files, err := s.filesOf(conn)
	if err != nil {
		return nil, err
	}
	base := strings.TrimSuffix(fileRoot(conn, col), "/") + "/"
	made := map[string]bool{base: true}
	if into == restoreNew {
		name := e.Name
		if file.CheckName(name) != nil {
			name = backup.Slug(name)
		}
		base += name + " (restored " + day + ")/"
		if _, err := files.Stat(ctx, base); err == nil {
			return nil, errors.New(base + " is already there")
		}
		if err := files.MkCol(ctx, base); err != nil {
			return nil, err
		}
		made[base] = true
	}
	// mkdirs makes the folders above p that are not there; one that is
	// there answers 405 and stays as it is.
	mkdirs := func(p string) error {
		var chain []string
		for dir := path.Dir(strings.TrimSuffix(p, "/")) + "/"; strings.HasPrefix(dir, base) && !made[dir]; dir = path.Dir(strings.TrimSuffix(dir, "/")) + "/" {
			chain = append([]string{dir}, chain...)
		}
		for _, d := range chain {
			if err := files.MkCol(ctx, d); err != nil && !dav.IsStatus(err, http.StatusMethodNotAllowed) {
				return err
			}
			made[d] = true
		}
		return nil
	}
	uploaded, same := 0, 0
	var renamed []string
	for _, zf := range zr.File {
		rel, ok := strings.CutPrefix(zf.Name, e.Dir+"files/")
		if !ok || rel == "" || strings.HasSuffix(rel, "/") {
			continue
		}
		segs, okSegs := relSegs(rel)
		if !okSegs {
			continue // a name that would climb out is not written anywhere
		}
		p := base + strings.Join(segs, "/")
		if err := mkdirs(p); err != nil {
			return nil, err
		}
		// A server that ignores If-None-Match would overwrite: what is there
		// is asked first (file.Provider.UploadCreate).
		if _, err := files.Stat(ctx, p); err == nil {
			if sameFile(ctx, files, p, zf) {
				same++
				continue
			}
			// Other bytes under the name: beside it, never over it.
			ext, at := path.Ext(p), ""
			for n := 1; n <= 9 && at == ""; n++ {
				fresh := strings.TrimSuffix(p, ext) + " (restored " + day + ")" + ext
				if n > 1 {
					fresh = strings.TrimSuffix(p, ext) + " (restored " + day + " " + strconv.Itoa(n) + ")" + ext
				}
				if _, err := files.Stat(ctx, fresh); err != nil {
					at = fresh
				}
			}
			if at == "" {
				return nil, fmt.Errorf("%s: every name beside it is taken", rel)
			}
			renamed = append(renamed, strings.Join(segs, "/")+" → "+path.Base(at)+": changed there since the copy")
			p = at
		}
		err := uploadMember(ctx, files, p, zf)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", rel, err)
		}
		uploaded++
	}
	line := fmt.Sprintf("%s / %s → %s: %s uploaded", e.Connection, e.Name, strings.TrimPrefix(base, strings.TrimSuffix(fileRoot(conn, col), "/")+"/"), plural(uploaded, "file"))
	if into != restoreNew {
		line = fmt.Sprintf("%s / %s → %s · %s: %s uploaded", e.Connection, e.Name, connLabel(conn), collectionLabel(col), plural(uploaded, "file"))
	}
	if same > 0 {
		line += fmt.Sprintf(", %d already there", same)
	}
	return append([]string{line}, renamed...), nil
}

// uploadMember writes one member of the archive where nothing is (If-None-
// Match), as it is read from the archive.
func uploadMember(ctx context.Context, files *file.Provider, p string, zf *zip.File) error {
	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	_, err = files.UploadCreate(ctx, p, rc)
	return err
}

// sameFile says whether the file at p holds the bytes of the member: the
// sizes first, then both read through a hash — neither is held whole.
func sameFile(ctx context.Context, files *file.Provider, p string, zf *zip.File) bool {
	st, err := files.Stat(ctx, p)
	if err != nil || st.Size != int64(zf.UncompressedSize64) {
		return false
	}
	there, _, err := files.Get(ctx, p, nil)
	if err != nil {
		return false
	}
	defer there.Close()
	ours, err := zf.Open()
	if err != nil {
		return false
	}
	defer ours.Close()
	a, b := sha256.New(), sha256.New()
	if _, err := io.Copy(a, there); err != nil {
		return false
	}
	if _, err := io.Copy(b, ours); err != nil {
		return false
	}
	return bytes.Equal(a.Sum(nil), b.Sum(nil))
}
