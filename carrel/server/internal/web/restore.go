package web

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"slices"
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
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// Restore (S-24.3) is an import, not a rollback: each collection of the
// archive goes where its row says (s-backup-8) — the collection it came from
// by default, another, a new one, or nowhere —, what is missing there is
// created and nothing there is deleted, so nothing made after the copy is
// lost; a record there that differs from the archive's is skipped, replaced
// or copied beside, as the step's question is answered — the import's
// (the owner, 26.10.2026). It is checked on the same screen before anything is
// written: the archive is opened, its manifest read and each collection's
// outcome said in its row. Three sources: a run of a job, a file on a WebDAV storage, a
// file from this computer. The archive waits for the next step in a temporary
// file of this session, never in memory and never on the volume's data: it is
// deleted when the restore ends or the session does.

// restoreTask is a restore in progress: the archive on disk, its password
// and its manifest. As a session task it is cancelled — the file deleted —
// when the session ends.
type restoreTask struct {
	id string
	// src is the archive where it lies, read a piece at a time (S-24.3):
	// nothing of it is on Carrel's disk
	src      archive
	password string
	file     string // where the archive lay: the run's file, the path on the storage, the uploaded name
	manifest backup.Manifest
	mu       sync.Mutex
	gone     bool // cancelled: the archive let go of
	// choice is the matching (s-backup-8), preset on the first look; counts
	// are what each row's target was found to hold, read once.
	choice restoreChoice
	counts map[string]restoreCount
	// put is where an archive from this computer was put to be read from
	// a storage; deleted after the restore (the owner, 19.10.2026)
	put *storedArchive
	// here — the archive came from this computer; layout — where the
	// members of one sent in parts lie, read once (restore_parts.go)
	here   bool
	layout *backup.Layout
}

// storedArchive is an archive put into a storage for its restore.
type storedArchive struct {
	files *file.Provider
	path  string
}

// restoreDone is what the restore did to one collection of the archive, a
// row of the report (s-backup-18).
type restoreDone struct {
	Name, Colour, Into   string
	Created, Fresh, Same int
	// Here, Replaced — records already there that differed: skipped, or
	// written over; copied beside, they are Fresh
	Here, Replaced int
	Refused        int
	Why            string // why the refused were refused, the server's words
	Files          bool
	Matched        bool // into the collection the archive was taken of
}

func (t *restoreTask) ID() string    { return t.id }
func (t *restoreTask) Running() bool { return !t.ended() }

func (t *restoreTask) Cancel() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.gone, t.src = true, nil
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

// open reads the archive again where it lies: its directory and manifest,
// the members as they are read.
func (t *restoreTask) open() (*zip.Reader, io.Closer, error) {
	t.mu.Lock()
	src := t.src
	t.mu.Unlock()
	if src == nil {
		return nil, nil, errors.New("the archive is no longer open")
	}
	zr, _, err := backup.OpenAt(src, src.Size(), t.password)
	if err != nil {
		return nil, nil, err
	}
	return zr, io.NopCloser(nil), nil
}

// restoreBlock is the restore part of the Backup screen: which archive, then
// each of its collections and where it goes (s-backup-8).
// run names the copy a row of the history opened it with (Нс56): that one
// is the job's row, chosen.
func (s *Server) restoreBlock(sess *session.Session, blob sealed.Blob, from, run string, typed url.Values) view.RestoreBlock {
	if typed == nil {
		typed = url.Values{}
	}
	b := view.RestoreBlock{Note: line("Restoring is an import: records made since the archive stay, nothing is deleted.")}
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
			// Its day, its size, how many collections (s-backup-7): a
			// column each.
			runs = append(runs, view.Option{Value: job.ID + "|" + newest.ID, Label: text(job.Name),
				Columns: []view.Text{text(string(tf.numStamp(newest.Started))), text(string(formatSize(newest.Size))), text(plural(newest.Collections, "collection"))}})
		}
	}
	for _, conn := range blob.Connections {
		for _, col := range blob.CollectionsOf(conn.ID) {
			if col.Has(view.KindFile) && !col.Gone {
				v := selValue(conn.ID, col.Path)
				stores = append(stores, view.Option{Value: v, Label: text(string(stamp(connLabel(conn), collectionLabel(col)))), Selected: v == typed.Get("store")})
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
			Href: s.route("/app/settings/backup/restore?" + url.Values{"from": {f.id}}.Encode())})
	}
	source := map[string]string{"job": "run", "server": "webdav", "file": "upload"}[from]
	rows := []view.FormRow{csrfField(sess), {Name: "source", Kind: view.InputHidden, Value: text(source)}}
	// The source's own card, its rubric the source's name (s-backup-7, -15,
	// -17), and the archive's password in a card of its own.
	set := view.FieldSet{Legend: map[string]view.Text{"job": "From a job", "server": "From a server", "file": "From this computer"}[from]}
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
		set.Rows = append(set.Rows, view.FormRow{Name: "run", Label: "From a job", Kind: view.InputRadio, Options: runs})
	case "server":
		// The collection and the archive's path in it, Browse choosing it
		// in the tree (s-backup-17, Typecase's note 99).
		if !slices.ContainsFunc(stores, func(o view.Option) bool { return o.Selected }) && len(stores) > 0 {
			stores[0].Selected = true
		}
		browse := view.Action{ID: "restore.browse", Label: "Browse", Method: "POST", Enabled: true, Submits: true,
			Href: s.postQuery("/app/settings/backup/restore", url.Values{"from": {"server"}, "op": {"browse"}})}
		set.Rows = append(set.Rows,
			view.FormRow{Name: "store", Label: "Collection", Kind: view.InputSelect, Options: stores},
			view.FormRow{Name: "path", Label: "Archive in it", Kind: view.InputText, Mono: true, Value: text(typed.Get("path")), Extra: &browse,
				Placeholder: "/backups/carrel-personal-2026-08-14.zip"})
	case "file":
		// Over the ceiling of an import the archive goes into a folder of
		// a storage, in the same stream, and is read from there and deleted
		// after the restore (S-24.3, the owner's decisions of 19.10.2026):
		// the fields stand before the file, as a browser sends them; the
		// folder is off while there is no storage (s-backup-15, -16).
		store := typed.Get("store")
		// «None» switches the folder off while it is chosen (s-backup-16,
		// Typecase's note 106): a folder of no storage means nothing.
		stores := append([]view.Option{{Value: "", Label: text("None — up to " + string(formatSize(s.cfg.ImportMaxBytes))), Selected: store == "", Disables: "folder"}}, stores...)
		folder := typed.Get("folder")
		if folder == "" {
			folder = "/carrel/restore"
		}
		set.Rows = append(set.Rows,
			view.FormRow{Name: "store", Label: "Storage for a large archive", Kind: view.InputSelect, Options: stores},
			view.FormRow{Name: "folder", Label: "Folder in it", Kind: view.InputText, Mono: true, Value: text(folder), ReadOnly: store == "",
				Hint: "A larger archive is put here, read from there and deleted after the restore."},
			// The file last: a browser sends the fields before it.
			view.FormRow{Name: "file", Label: "Archive", Kind: view.InputFile, Accept: ".zip"})
	}
	b.Source = s.plainForm("/app/settings/backup/restore", "POST", rows...)
	if from == "file" {
		// With the live layer the archive goes in parts, the ones this
		// address names (Typecase's notes 98, 109).
		b.Ranges = s.route("/app/settings/backup/restore?op=ranges")
	}
	b.Source.Sets = append(b.Source.Sets, set, view.FieldSet{Legend: "Archive password", Rows: []view.FormRow{
		{Name: "password", Label: "Archive password", Kind: view.InputPassword, Hint: "The one set in the job, not your sign-in password."}}})
	open := view.Action{ID: "restore.open", Label: "Open the manifest", Primary: true, Href: s.route("/app/settings/backup/restore"), Method: "POST", Enabled: true}
	if from == "file" {
		// The archive comes with the form, from this computer.
		b.Source.Action = s.postQuery("/app/settings/backup/restore", url.Values{"from": {"file"}})
		open.Href = b.Source.Action
	}
	if from == "server" {
		b.Source.Action = s.postQuery("/app/settings/backup/restore", url.Values{"from": {"server"}})
		open.Href = b.Source.Action
	}
	b.Source.Actions = []view.Action{open,
		{ID: "restore.close", Label: "Cancel", Href: s.route("/app/settings/backup"), Method: "GET", Enabled: true}}
	b.Title, b.Subtitle = "Restore", line("Nothing is written until you have seen what goes where")
	if t := restoreOf(sess); t != nil {
		b.Subtitle = archiveLine(t)
		if !t.manifest.Taken.IsZero() {
			b.Title = text("Restore from " + string(tf.dayKey(t.manifest.Taken)))
		}
		b.Match = s.restoreMatch(sess, blob, t)
		b.Actions = []view.Action{{ID: "restore.cancel", Label: "Cancel", Href: s.postQuery("/app/settings/backup/restore/match", url.Values{"op": {"cancel"}}), Method: "POST", Enabled: true}}
	}
	return b
}

// The choices of a row's «Restore into» besides the collections.
const (
	restoreNew  = "new"
	restoreSkip = "skip"
)

// restoreChoice is what the matching says of every collection of the
// archive: where it goes and whether its row is ticked, and what becomes of
// the records already there that differ — an answer of the import
// (existingChoice), skipped without one. The task keeps it between the
// steps; a form of the step sends the rows again.
type restoreChoice struct {
	into     []string
	picked   []bool
	existing string
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
func postedChoice(fields url.Values, blob sealed.Blob, t *restoreTask, kept restoreChoice) restoreChoice {
	c := restoreChoice{into: append([]string{}, kept.into...), picked: make([]bool, len(kept.picked)), existing: kept.existing}
	for i, e := range t.manifest.Collections {
		if v := fields.Get("into." + strconv.Itoa(i)); validInto(blob, e, v) {
			c.into[i] = v
		}
		v := fields.Get("pick." + strconv.Itoa(i))
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
	return restoreChoice{into: append([]string{}, t.choice.into...), picked: append([]bool{}, t.choice.picked...), existing: t.choice.existing}
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
	going, here, unread := 0, 0, false
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
		would := s.restoreWill(sess, blob, t, zr, i, e, choice.into[i], matched && choice.into[i] == selValue(col.AccountID, col.Path))
		row.Will, row.Danger, row.Here = would.will, would.danger, would.here
		if choice.picked[i] {
			m.Selected++
			if choice.into[i] != restoreSkip {
				going++
				here += would.here
				unread = unread || would.unread
			}
		}
		m.Rows = append(m.Rows, row)
	}
	m.Existing = s.restoreExisting(here, unread, choice.existing)
	bar.Options = append(bar.Options,
		view.Option{Value: restoreNew, Label: "New collection", Separator: true},
		view.Option{Value: restoreSkip, Label: "Skip this one"})
	// The bar's choice shows the target of the first ticked row: Restore
	// into gives it to every ticked one (s-backup-8, Typecase's note 102).
	for i := range t.manifest.Collections {
		if !choice.picked[i] || choice.into[i] == restoreSkip {
			continue
		}
		for k := range bar.Options {
			bar.Options[k].Selected = bar.Options[k].Value == choice.into[i]
		}
		break
	}
	m.Into = bar
	// One form (s-backup-8): each button sends it with its op.
	q := func(op string) string {
		return s.postQuery("/app/settings/backup/restore/match", url.Values{"op": {op}})
	}
	m.Form = s.plainForm("/app/settings/backup/restore/match", "POST", csrfField(sess))
	s.partsMatch(t, m)
	// «Restore 3 collections» (s-backup-8): what the press restores.
	run := view.Action{ID: "restore.run", Label: text("Restore " + plural(going, "collection")), Href: q("restore"), Method: "POST", Primary: true, Enabled: going > 0}
	if going == 0 {
		run.Reason = "Tick a collection and choose where it goes."
	}
	m.Form.Actions = []view.Action{run,
		{ID: "restore.into", Label: "Restore into", Href: q("into"), Method: "POST", Enabled: true},
		{ID: "restore.skip", Label: "Skip", Href: q("skip"), Method: "POST", Enabled: true},
		{ID: "restore.all", Label: "Select all", Icon: "i-selectall", Href: q("all"), Method: "POST", Enabled: true},
		{ID: "restore.none", Label: "Clear selection", Icon: "i-close", Href: q("none"), Method: "POST", Enabled: true},
	}
	return m
}

// restoreWould is «What will happen» of a row: the words, whether they are
// an alarm, how many records of the archive the target holds otherwise, and
// whether the target is compared only when the restore runs.
type restoreWould struct {
	will   view.Text
	danger bool
	here   int
	unread bool
}

// restoreWill is «What will happen» of a row (s-backup-8): what goes where,
// then how many are already there and differ — their fate is the step's
// question.
func (s *Server) restoreWill(sess *session.Session, blob sealed.Blob, t *restoreTask, zr *zip.Reader, i int, e backup.Entry, into string, matched bool) restoreWould {
	if e.Error != "" {
		return restoreWould{will: text("not whole in this archive: " + e.Error), danger: true}
	}
	if into == restoreSkip {
		return restoreWould{will: "skip"}
	}
	// What the archive cannot give is said first: nothing of it is written.
	// An archive from this computer sent in parts holds no records until
	// Restore sends them (Typecase's note 109, s-backup-20).
	read := t.holds(e)
	if e.Kind != view.KindFile && read {
		if _, err := s.archiveRecords(zr, e); err != nil {
			return restoreWould{will: text("cannot be restored: " + err.Error()), danger: true}
		}
	}
	switch {
	case e.Kind == view.KindFile && into == restoreNew:
		col, _ := newRestoreFolder(blob, e)
		conn, _ := blob.Connection(col.AccountID)
		return restoreWould{will: text("a new folder in " + string(stamp(connLabel(conn), collectionLabel(col))) + " — " + plural(e.Count, "file") + " will be uploaded")}
	case into == restoreNew:
		conn, _ := newRestoreConn(blob, e)
		return restoreWould{will: text("a new collection on " + connLabel(conn) + " — " + plural(e.Count, "record") + " will be created")}
	}
	prefix := ""
	if matched {
		prefix = "matched by path — "
	}
	var w restoreWould
	missing := 0
	if e.Kind != view.KindFile {
		if !read {
			w.unread = true
		} else {
			n := s.countFor(sess, blob, t, zr, i, e, into)
			if n.err != nil {
				return restoreWould{will: text(prefix + "cannot be read now: " + n.err.Error()), danger: true}
			}
			missing, w.here = n.missing, n.changed
		}
	}
	// A server that is gone says so in alarm; what the collection chosen
	// instead holds follows, plain (s-backup-8).
	if _, there := blob.Connection(e.AccountID); !there {
		w.will, w.danger = "that server is gone — another collection chosen", true
		return w
	}
	switch {
	case e.Kind == view.KindFile:
		w.will = text(prefix + "missing files are uploaded, none is overwritten")
	case w.unread:
		w.will = text(prefix + "compared when you restore")
	case missing > 0:
		w.will = text(prefix + plural(missing, "record") + " missing, will be created")
	case w.here > 0:
		w.will = text(strings.TrimSuffix(prefix, " — "))
	default:
		w.will = text(prefix + "no differences")
	}
	return w
}

// restoreExisting is the step's question about the records of the archive
// already there that differ (s-backup-8, -20; the owner, 26.10.2026): the
// import's three answers, Skip them without one. How many is said when every
// target was read; an archive sent in parts is compared when it is restored.
func (s *Server) restoreExisting(here int, unread bool, answer string) *view.ImportExisting {
	if here == 0 && !unread {
		return nil
	}
	options := []struct{ id, word, label, hint string }{
		{"skip", existingSkip, "Skip them", "They stay as they are here; the archive's version is not written."},
		{"replace", existingReplace, "Replace them", "The ones here are overwritten with the archive's version."},
		{"copy", existingCopies, "Import as copies", "Each gets a new UID and stands beside the one here."},
	}
	out := &view.ImportExisting{Note: text(plural(here, "record") + " of the archive are already here and differ")}
	switch {
	case unread:
		out.Note = "Records of the archive that are already here and differ"
	case here == 1:
		out.Note = "1 record of the archive is already here and differs"
		options = []struct{ id, word, label, hint string }{
			{"skip", existingSkip, "Skip it", "It stays as it is here; the archive's version is not written."},
			{"replace", existingReplace, "Replace it", "The one here is overwritten with the archive's version."},
			{"copy", existingCopies, "Import as a copy", "It gets a new UID and stands beside the one here."},
		}
	}
	for _, o := range options {
		if answer == o.word {
			out.Hint = text(o.hint)
		}
		out.Options = append(out.Options, view.Action{ID: "restore.existing." + o.id, Label: text(o.label), Method: "GET", Enabled: true,
			Href: s.route("/app/settings/backup/restore?" + url.Values{"existing": {o.id}}.Encode()), On: answer == o.word})
	}
	return out
}

// matchRestore is a press on the bar of the matching (s-backup-8): the form
// as it stands, and do= — into gives the ticked rows the bar's target (a row
// whose kind it does not take keeps its own), skip skips them and unticks
// them, all ticks every row, none unticks every row. Nothing is written;
// the page comes back.
func (s *Server) matchRestore(w http.ResponseWriter, r *http.Request, sess *session.Session, blob sealed.Blob) {
	t := restoreOf(sess)
	if t == nil {
		s.writePage(w, r, http.StatusOK, s.restorePage(sess, "", "", restoreFailure("There is no archive open — the session may have ended. Open it again.")))
		return
	}
	fields := restoreFields(r)
	c := postedChoice(fields, blob, t, t.choiceOf(blob))
	switch op := restoreOp(r, fields); op {
	case "into":
		v := fields.Get("into")
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
	case "all", "none":
		for i := range c.picked {
			c.picked[i] = op == "all"
		}
	}
	t.keep(c)
	// Asked for the page's column (the zone of an archive from this
	// computer, Typecase's note 109): answered in place, the file the page
	// keeps stays with it.
	if render.WantsFragment(r) {
		s.writePage(w, r, http.StatusOK, s.restorePage(sess, "", ""))
		return
	}
	s.redirect(w, r, "/app/settings/backup/restore")
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

// openRestore is step one: the archive opened where it lies, its manifest
// read with its password, and the matching of the next step (S-24.3, the
// owner's decision of 19.10.2026). A job's copy and a file on a storage are
// read by ranges; one sent by a browser without a script arrives whole,
// once — up to the ceiling of an import it is held in the session's
// memory, larger it goes on, in the same stream, into the folder of a
// storage the step names and is read from there.
func (s *Server) openRestore(w http.ResponseWriter, r *http.Request, sess *session.Session, blob sealed.Blob) {
	endRestore(sess)
	sess.SetDraft(session.DraftRestoreReport, nil) // another archive: the last report goes
	fields := r.PostForm
	var src archive
	var put *storedArchive
	var name string
	source := r.URL.Query().Get("source")
	if source == "" {
		source = fields.Get("source")
	}
	// The archive from this computer comes in a form of files, whose
	// fields are read with the file (s-backup-15).
	if source == "" && r.URL.Query().Get("from") == "file" {
		source = "upload"
	}
	fail := func(msg string) {
		why := line(msg)
		// Too large without a storage: what to do instead, its way named
		// (s-backup-16).
		if before, ok := strings.CutSuffix(msg, " From a server."); ok {
			why = join(before+" ", strongRun("From a server"), ".")
		}
		s.writePage(w, r, http.StatusOK, s.refusedStep(sess, source, fields, why))
	}
	var parts *partsArchive
	if source == "upload" {
		up, err := uploadOf(r)
		if err != nil || up.next == nil {
			fail("No file came with the form.")
			return
		}
		fields = up.fields
		if sent, size, ok := sentFile(fields); ok {
			// The live layer sent the parts it was told (?op=ranges): the
			// archive's start and end.
			parts = &partsArchive{name: sent, size: size}
			if err := readParts(up, parts, s.partsLimit()); err != nil {
				fail(err.Error())
				return
			}
			src, name = parts, sent
		} else {
			name = up.next.FileName()
			if src, put, err = s.uploadedArchive(r.Context(), blob, up, fields); err != nil {
				fail(err.Error())
				return
			}
		}
	} else {
		var err error
		if src, name, err = s.restoreSource(r.Context(), sess, blob, source, fields); err != nil {
			fail(err.Error())
			return
		}
	}
	t := &restoreTask{src: src, password: fields.Get("password"), file: name, put: put, here: source == "upload"}
	_, m, err := backup.OpenAt(src, src.Size(), t.password)
	if err == nil && parts != nil {
		t.layout, err = backup.ReadLayout(src, src.Size(), t.password)
	}
	if err != nil {
		msg := "This is not a Carrel archive, or it is damaged: " + err.Error()
		switch {
		case errors.Is(err, backup.ErrWrongPassword):
			msg = "The password does not open this archive."
		case errors.Is(err, errNoRange), errors.Is(err, errArchiveChanged):
			msg = err.Error()
		case errors.Is(err, errPartMissing):
			msg = "The archive's directory is not where Carrel looked for it. Put the archive into a folder of your storage and choose From a server."
		}
		fail(msg)
		return
	}
	t.manifest = m
	id, err := user.NewTaskID()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	t.id = "restore-" + id
	sess.KeepTask(t)
	// Asked for the page's column (Typecase's note 109): step 2 comes in
	// place, and the page keeps the file for Restore.
	if render.WantsFragment(r) {
		s.writePage(w, r, http.StatusOK, s.restorePage(sess, "", ""))
		return
	}
	s.redirect(w, r, "/app/settings/backup/restore")
}

// uploadedArchive is the archive a browser sent without a script (S-24.3):
// up to CARREL_IMPORT_MAX_BYTES held in memory; larger, the rest of the
// same stream goes after what was read into the folder of the storage the
// form names (store, folder), under the file's name, and is read from there
// by ranges; with no folder named, a refusal that says why and how.
func (s *Server) uploadedArchive(ctx context.Context, blob sealed.Blob, up *uploadBody, fields url.Values) (archive, *storedArchive, error) {
	max := s.cfg.ImportMaxBytes
	head, err := io.ReadAll(io.LimitReader(up.next, max+1))
	if err != nil {
		return nil, nil, errors.New("The archive did not arrive whole: " + err.Error())
	}
	if int64(len(head)) <= max {
		return memArchive{bytes.NewReader(head)}, nil, nil
	}
	if fields.Get("store") == "" {
		return nil, nil, fmt.Errorf("The archive is larger than %s. Choose a folder of your storage for it, or put it there and choose From a server.", formatSize(max))
	}
	name := up.next.FileName()
	rel := strings.Trim(fields.Get("folder"), "/")
	if rel != "" {
		rel += "/"
	}
	files, p, _, err := s.restoreStore(blob, fields.Get("store"), rel+name)
	if err != nil {
		return nil, nil, err
	}
	if _, err := files.UploadCreate(ctx, p, io.MultiReader(bytes.NewReader(head), up.next)); err != nil {
		return nil, nil, errors.New("The archive could not be put into the storage: " + err.Error())
	}
	a, err := openRange(ctx, files, p)
	return a, &storedArchive{files: files, path: p}, err
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
	switch {
	case t.here:
		head = "From this computer · "
	case t.manifest.Job != "":
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

// restoreSource is the archive the first step names where it lies — a job's
// copy at its destination, a file on a storage —, read by ranges (S-24.3),
// and where it lay.
func (s *Server) restoreSource(ctx context.Context, sess *session.Session, blob sealed.Blob, source string, f url.Values) (archive, string, error) {
	switch source {
	case "run":
		jobID, runID, _ := strings.Cut(f.Get("run"), "|")
		job, ok := blob.BackupJob(jobID)
		if !ok {
			return nil, "", errors.New("choose a copy")
		}
		for _, r := range s.users.BackupRuns(sess.AccountID())[jobID] {
			if r.ID == runID && r.File != "" && r.Verified {
				dest, err := s.backupDest(blob, job)
				if err != nil {
					return nil, "", err
				}
				a, err := openRange(ctx, dest.files, dest.folder+r.File)
				return a, r.File, err
			}
		}
		return nil, "", errors.New("choose a copy")
	case "webdav":
		files, p, segs, err := s.restoreStore(blob, f.Get("store"), f.Get("path"))
		if err != nil {
			return nil, "", err
		}
		a, err := openRange(ctx, files, p)
		return a, strings.Join(segs, "/"), err
	}
	return nil, "", errors.New("choose where the archive is")
}

// restoreStore is a path on a storage of the catalog the restore names:
// store — the storage, «account|path» —, rel — the path under its root.
func (s *Server) restoreStore(blob sealed.Blob, store, rel string) (*file.Provider, string, []string, error) {
	acc, colPath, _ := strings.Cut(store, "|")
	col, ok := blob.Collection(acc, colPath)
	conn, hasConn := blob.Connection(acc)
	segs, okPath := relSegs(rel)
	if !ok || !hasConn || !col.Has(view.KindFile) || !okPath || len(segs) == 0 {
		return nil, "", nil, errors.New("choose a storage and the archive's path in it")
	}
	files, err := s.filesOf(conn)
	if err != nil {
		return nil, "", nil, err
	}
	p, err := file.Join(fileRoot(conn, col), segs...)
	if err != nil {
		return nil, "", nil, err
	}
	return files, p, segs, nil
}

// runRestore is step three: every ticked collection goes where its row says
// — records created in an existing collection or a new one, files uploaded
// —, the way an import does: nothing on the server is deleted or
// overwritten (S-24.3). The page says what went where.
func (s *Server) runRestore(w http.ResponseWriter, r *http.Request, sess *session.Session, key [32]byte, blob sealed.Blob) {
	t := restoreOf(sess)
	if t == nil {
		s.writePage(w, r, http.StatusOK, s.restorePage(sess, "", "", restoreFailure("There is no archive open — the session may have ended. Open it again.")))
		return
	}
	fields := restoreFields(r)
	choice := postedChoice(fields, blob, t, t.choiceOf(blob))
	t.keep(choice)
	if a := t.parts(); a != nil {
		// An archive from this computer in parts: the members of the
		// collections chosen came with the form (?op=ranges). None came —
		// the page that kept the file is gone — and step 1 asks for it
		// again (Typecase's note 109).
		if up, err := uploadOf(r); err == nil && isFileUpload(r) {
			if name, size, ok := sentFile(up.fields); ok && name == a.name && size == a.size {
				if err := readParts(up, a, s.partsLimit()); err != nil {
					endRestore(sess)
					s.writePage(w, r, http.StatusOK, s.refusedStep(sess, "upload", nil, line(err.Error())))
					return
				}
			}
		}
		if _, fits := s.restoreSpans(sess, t, a, fields); !fits {
			endRestore(sess)
			s.writePage(w, r, http.StatusOK, s.refusedStep(sess, "upload", nil, line(errPartsTooLarge{s.cfg.ImportMaxBytes}.Error())))
			return
		}
		for i, e := range t.manifest.Collections {
			if choice.picked[i] && choice.into[i] != restoreSkip && !t.holds(e) {
				endRestore(sess)
				s.writePage(w, r, http.StatusOK, s.refusedStep(sess, "upload", nil, line(chooseAgain)))
				return
			}
		}
	}
	zr, f, err := t.open()
	if err != nil {
		s.writePage(w, r, http.StatusOK, s.restorePage(sess, "", "", restoreFailure(err.Error())))
		return
	}
	// Closed before the restore ends: an open file cannot be deleted on every
	// system, and the archive must not outlive the restore.
	closeOnce := sync.OnceFunc(func() { f.Close() })
	defer closeOnce()
	ctx := r.Context()
	day := time.Now().In(s.tz()).Format("2006-01-02")
	var rows []restoreDone
	for i, e := range t.manifest.Collections {
		into := choice.into[i]
		if !choice.picked[i] || into == restoreSkip {
			continue
		}
		var done restoreDone
		var err error
		if e.Kind == view.KindFile {
			done, err = s.restoreFiles(ctx, blob, zr, e, into, day)
		} else {
			done, err = s.restoreRecords(ctx, sess, key, blob, zr, e, into, day, choice.existing)
		}
		if err != nil && done.Why == "" {
			done.Why, done.Refused = err.Error(), max(done.Refused, 1)
		}
		done.Name = e.Connection + " / " + e.Name
		done.Colour = string(catalogColour(sealed.Collection{AccountID: e.AccountID, Path: e.Path}))
		if c, ok := blob.Collection(e.AccountID, e.Path); ok {
			done.Colour = string(catalogColour(c))
		}
		col, matched := matchedByPath(blob, e)
		done.Matched = matched && into == selValue(col.AccountID, col.Path)
		rows = append(rows, done)
	}
	if len(rows) == 0 {
		s.writePage(w, r, http.StatusOK, s.restorePage(sess, "", "", view.Notice{Role: view.NoticeInfo, Title: "Nothing was restored", Body: "Tick at least one collection and choose where it goes."}))
		return
	}
	closeOnce()
	// An archive put into a storage for this restore goes (the owner,
	// 19.10.2026): the folder's hint said so.
	if p := t.put; p != nil {
		_ = p.files.Delete(context.WithoutCancel(ctx), p.path, "")
	}
	report := restoreReportOf{Day: string(s.timeFor(sess).dayKey(t.manifest.Taken)), Rows: rows}
	if raw, err := json.Marshal(report); err == nil {
		sess.SetDraft(session.DraftRestoreReport, raw)
	}
	endRestore(sess)
	s.redirect(w, r, "/app/settings/backup/restore/report")
}

// restoreReportOf is the last restore as the session holds it for its
// report (s-backup-18).
type restoreReportOf struct {
	Day  string
	Rows []restoreDone
}

// getBackupRestoreReport is the restore's third step (s-backup-18): what
// the last restore did, a row a collection; with none, the first step.
func (s *Server) getBackupRestoreReport(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	var held restoreReportOf
	if raw := sess.Draft(session.DraftRestoreReport); raw == nil || json.Unmarshal(raw, &held) != nil {
		s.redirect(w, r, "/app/settings/backup/restore")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	page := s.restorePage(sess, "", "")
	page.Restore.Report = s.restoreReport(held)
	s.writePage(w, r, http.StatusOK, page)
}

// restoreReport is the report's words (s-backup-18): «Restored from …», the
// sums in a line, a row a collection — what was created, what was already
// there and what became of it as the step's question was answered, what was
// refused and why, in alarm —, Done.
func (s *Server) restoreReport(held restoreReportOf) *view.RestoreReport {
	rep := &view.RestoreReport{Title: "Restored",
		Done: view.Action{ID: "restore.done", Label: "Done", Href: s.route("/app/settings/backup"), Method: "GET", Primary: true, Enabled: true}}
	if held.Day != "" {
		rep.Title = text("Restored from " + held.Day)
	}
	created, here, replaced, copies, refused := 0, 0, 0, 0, 0
	for _, d := range held.Rows {
		created, here, replaced, refused = created+d.Created, here+d.Here, replaced+d.Replaced, refused+d.Refused
		if !d.Files {
			copies += d.Fresh
		}
		row := view.RestoreReportRow{Name: text(d.Name), Colour: view.Colour(d.Colour), Into: text(d.Into), Created: d.Created}
		var parts []any
		add := func(p ...any) {
			if len(parts) > 0 {
				parts = append(parts, " · ")
			}
			parts = append(parts, p...)
		}
		noun := "record"
		if d.Files {
			noun = "file"
		}
		if d.Created > 0 && d.Matched {
			add(plural(d.Created, "missing "+noun) + " created")
		}
		switch {
		case d.Here > 0:
			add(fmt.Sprintf("%d already here — skipped", d.Here))
		case d.Replaced > 0:
			add(fmt.Sprintf("%d already here — replaced", d.Replaced))
		case d.Fresh > 0 && d.Files:
			add(plural(d.Fresh, "file") + " put beside the one there")
		case d.Fresh > 0:
			add(fmt.Sprintf("%d already here — imported as copies", d.Fresh))
		}
		if d.Refused > 0 {
			add(view.Run{Text: text(fmt.Sprintf("%d not restored: %s", d.Refused, d.Why)), Mark: view.RunDanger})
		}
		switch {
		case len(parts) > 0:
		case d.Created > 0:
			add(plural(d.Created, noun) + " created")
		default:
			add("no differences")
			row.Same = true
		}
		row.Happened = join(parts...)
		rep.Rows = append(rep.Rows, row)
	}
	sums := []string{plural(len(held.Rows), "collection"), plural(created, "record") + " created"}
	for _, n := range []struct {
		count int
		then  string
	}{{here, "skipped"}, {replaced, "replaced"}, {copies, "imported as copies"}} {
		if n.count > 0 {
			sums = append(sums, fmt.Sprintf("%d already here, %s", n.count, n.then))
		}
	}
	if refused > 0 {
		sums = append(sums, fmt.Sprintf("%d not restored", refused))
	}
	rep.Subtitle = text(strings.Join(sums, " · "))
	return rep
}

// restoreRecords puts the records of one archived collection into the
// target — an existing collection, or a new one of its kinds on the server
// newRestoreConn names: a record missing there is created under its UID,
// the same record is left as it is, and a UID held there by different
// content goes as existing says — skipped, written over the version read a
// moment ago (one changed since is refused), or created again under a fresh
// UID.
func (s *Server) restoreRecords(ctx context.Context, sess *session.Session, key [32]byte, blob sealed.Blob, zr *zip.Reader, e backup.Entry, into, day, existing string) (restoreDone, error) {
	done := restoreDone{}
	objs, err := s.archiveRecords(zr, e)
	if err != nil {
		return done, err
	}
	var conn sealed.Connection
	var col sealed.Collection
	there := map[string]*model.Object{}
	if into == restoreNew {
		var ok bool
		if conn, ok = newRestoreConn(blob, e); !ok {
			return done, errors.New("no server here makes collections of this kind")
		}
		kinds := e.Kinds
		if len(kinds) == 0 {
			kinds = []view.Kind{e.Kind}
		}
		if col, err = s.newCollectionOn(ctx, conn, kinds, e.Name+" (restored "+day+")"); err != nil {
			return done, err
		}
		if err := s.users.AddCollection(sess.AccountID(), key, conn.ID, col); err != nil {
			return done, err
		}
	} else {
		acc, path, _ := strings.Cut(into, "|")
		var ok, hasConn bool
		col, ok = blob.Collection(acc, path)
		conn, hasConn = blob.Connection(acc)
		if !ok || !hasConn || conn.Feed || col.ReadOnly {
			return done, errors.New("the chosen collection does not take writes")
		}
		if there, err = s.recordsThere(ctx, sess, blob, into, e.Kind); err != nil {
			return done, err
		}
	}
	done.Into = string(stamp(connLabel(conn), collectionLabel(col)))
	dests := map[view.Kind]*collectionCtx{}
	made, same, refused, here, replaced := 0, 0, 0, 0, 0
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
			switch existing {
			case existingSkip:
				here++
				continue
			case existingReplace:
				o.Path, o.ETag = ex.Path, ex.ETag
				if err := dest.update(ctx, o); err != nil {
					refused++
					if done.Why == "" {
						done.Why = refusedWhy(err)
					}
					continue
				}
				replaced++
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
			if done.Why == "" {
				done.Why = refusedWhy(err)
			}
			continue
		}
		made++
	}
	done.Created, done.Same, done.Fresh, done.Refused = made, same, len(renamed), refused
	done.Here, done.Replaced = here, replaced
	return done, nil
}

// refusedWhy is a record's refusal as the report says it (s-backup-18):
// «the server refused it (413, too large)».
func refusedWhy(err error) string {
	code := dav.StatusCode(err)
	if code == 0 {
		return err.Error()
	}
	words := map[int]string{http.StatusRequestEntityTooLarge: "too large", http.StatusForbidden: "not allowed", http.StatusInsufficientStorage: "no room"}[code]
	if words == "" {
		words = strings.ToLower(http.StatusText(code))
	}
	return fmt.Sprintf("the server refused it (%d, %s)", code, words)
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
func (s *Server) restoreFiles(ctx context.Context, blob sealed.Blob, zr *zip.Reader, e backup.Entry, into, day string) (restoreDone, error) {
	done := restoreDone{Files: true}
	var col sealed.Collection
	if into == restoreNew {
		var ok bool
		if col, ok = newRestoreFolder(blob, e); !ok {
			return done, errors.New("no storage here takes writes")
		}
	} else {
		acc, colPath, _ := strings.Cut(into, "|")
		var ok bool
		col, ok = blob.Collection(acc, colPath)
		if !ok || col.ReadOnly || col.Feed || !col.Has(view.KindFile) {
			return done, errors.New("the chosen storage does not take writes")
		}
	}
	conn, hasConn := blob.Connection(col.AccountID)
	if !hasConn {
		return done, errors.New("the chosen storage does not take writes")
	}
	done.Into = string(stamp(connLabel(conn), collectionLabel(col)))
	files, err := s.filesOf(conn)
	if err != nil {
		return done, err
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
			return done, errors.New(base + " is already there")
		}
		if err := files.MkCol(ctx, base); err != nil {
			return done, err
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
			return done, err
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
				return done, fmt.Errorf("%s: every name beside it is taken", rel)
			}
			renamed = append(renamed, strings.Join(segs, "/")+" → "+path.Base(at)+": changed there since the copy")
			p = at
		}
		if err := uploadMember(ctx, files, p, zf); err != nil {
			done.Refused++
			if done.Why == "" {
				done.Why = rel + ": " + refusedWhy(err)
			}
			continue
		}
		uploaded++
	}
	if into == restoreNew {
		done.Into += " / " + strings.TrimSuffix(strings.TrimPrefix(base, strings.TrimSuffix(fileRoot(conn, col), "/")+"/"), "/")
	}
	done.Created, done.Same, done.Fresh = uploaded, same, len(renamed)
	return done, nil
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

// restorePage is the restore on its own page, /app/settings/backup/restore
// (s-backup-7, -8): the step of the source the address names, or the
// matching of the archive open; notices over it.
func (s *Server) restorePage(sess *session.Session, from, run string, notices ...view.Notice) view.BackupPage {
	page := s.backupPage(sess, nil, nil, notices...)
	page.Restore = s.restoreBlock(sess, s.catalogOf(sess), from, run, nil)
	page.Restore.Open = true
	return page
}

// refusedStep is the first step again after a refusal: the source as
// chosen, what was typed in it, why in alarm at the top of its card
// (s-backup-16).
func (s *Server) refusedStep(sess *session.Session, source string, typed url.Values, why view.Runs) view.BackupPage {
	page := s.backupPage(sess, nil, nil)
	from := map[string]string{"run": "job", "webdav": "server", "upload": "file"}[source]
	page.Restore = s.restoreBlock(sess, s.catalogOf(sess), from, typed.Get("run"), typed)
	page.Restore.Open, page.Restore.Refusal = true, why
	return page
}

// getBackupRestore is the restore's page (s-backup-7, Нс74): ?from= — where
// the archive is, ?run= — a job's copy chosen in the history.
func (s *Server) getBackupRestore(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	q := r.URL.Query()
	// The answer about the records already there (s-backup-8): kept with
	// the open archive until Restore.
	if v, ok := existingChoice(q.Get("existing")); ok {
		if t := restoreOf(sess); t != nil {
			c := t.choiceOf(s.catalogOf(sess))
			c.existing = v
			t.keep(c)
		}
	}
	page := s.restorePage(sess, q.Get("from"), q.Get("run"))
	if q.Get("store") != "" || q.Get("path") != "" {
		// Back from Browse with the archive chosen (s-backup-17).
		page.Restore = s.restoreBlock(sess, s.catalogOf(sess), q.Get("from"), q.Get("run"), q)
		page.Restore.Open = true
	}
	s.writePage(w, r, http.StatusOK, page)
}

// postBackupRestore opens the archive the first step names (s-backup-7):
// its manifest read, the matching follows.
func (s *Server) postBackupRestore(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	if _, live := sess.DataKey(); !live {
		s.notFound(w, r)
		return
	}
	switch r.URL.Query().Get("op") {
	case "browse":
		s.browseRestore(w, r, sess)
		return
	case "ranges":
		s.restoreRanges(w, r, sess)
		return
	}
	s.openRestore(w, r, sess, s.catalogOf(sess))
}

// restoreOp is the op a press of the matching sends: in the address
// (formaction) or as the button's value.
func restoreOp(r *http.Request, fields url.Values) string {
	if op := r.URL.Query().Get("op"); op != "" {
		return op
	}
	return fields.Get("op")
}

// postBackupRestoreMatch is the one form of the matching (s-backup-8), by
// the op of the button pressed: restore — the run; into, skip, all, none —
// the bar on the ticked rows; cancel — the archive closed, back to the jobs.
func (s *Server) postBackupRestoreMatch(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	key, live := sess.DataKey()
	if !live {
		s.notFound(w, r)
		return
	}
	blob := s.catalogOf(sess)
	switch restoreOp(r, restoreFields(r)) {
	case "restore":
		s.runRestore(w, r, sess, key, blob)
	case "into", "skip", "all", "none":
		s.matchRestore(w, r, sess, blob)
	case "cancel":
		endRestore(sess)
		s.redirect(w, r, "/app/settings/backup")
	default:
		s.notFound(w, r)
	}
}
