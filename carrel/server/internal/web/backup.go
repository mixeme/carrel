package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"carrel.dev/carrel/server/internal/backup"
	"carrel.dev/carrel/server/internal/provider/card"
	"carrel.dev/carrel/server/internal/provider/file"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// S-24: copy jobs. A run reads what the screens read, through the same
// providers (S-24.3), writes one archive, reads it back and compares, and
// only then rotates. It is recorded — when, what went in, how it ended, how
// big — and a copy that is not whole is red here, not only in the journal.
// Carrel keeps no clock of its own: a job runs from the screen, at sign-in
// when its last whole copy is old enough, or from outside (S-24.4).

// backupRunner knows which runs this process is running, so a run the state
// says is running but no process runs does not block its job for good.
type backupRunner struct {
	mu      sync.Mutex
	running map[string]bool
}

var runners = &backupRunner{running: map[string]bool{}}

func (b *backupRunner) set(id string, on bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if on {
		b.running[id] = true
	} else {
		delete(b.running, id)
	}
}

func (b *backupRunner) isRunning(id string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running[id]
}

// backupSources are the job's collections as the archive's sources. A
// collection the catalog no longer holds is still a source: its entry in the
// manifest says why it is not there.
func (s *Server) backupSources(ctx context.Context, blob sealed.Blob, job sealed.BackupJob) []backup.Source {
	var out []backup.Source
	for _, sel := range job.Sources {
		col, hasCol := blob.Collection(sel.AccountID, sel.Path)
		conn, hasConn := blob.Connection(sel.AccountID)
		entry := backup.Entry{AccountID: sel.AccountID, Path: sel.Path, Name: sel.Path}
		if !hasCol || !hasConn || col.Gone || col.Feed {
			out = append(out, backup.Source{Entry: entry, Write: func(func(string) (io.Writer, error)) (int, error) {
				return 0, errors.New("no longer in the catalog")
			}})
			continue
		}
		entry.Connection, entry.Name, entry.Kinds = connLabel(conn), collectionLabel(col), col.Kinds
		if len(col.Kinds) > 0 {
			entry.Kind = col.Kinds[0]
		}
		switch {
		case col.Has(view.KindFile):
			out = append(out, backup.Source{Entry: entry, Write: func(add func(string) (io.Writer, error)) (int, error) {
				return s.backupFiles(ctx, conn, col, add)
			}})
		default:
			spec := recordKinds["calendar"]
			if col.Has(view.KindContact) {
				spec = recordKinds["contacts"]
			}
			out = append(out, backup.Source{Entry: entry, Write: func(add func(string) (io.Writer, error)) (int, error) {
				c, err := s.newCollectionCtx(nil, spec, conn, col)
				if err != nil {
					return 0, err
				}
				objs, bad, err := c.allRecordsAndUnread(ctx)
				if err != nil {
					return 0, err
				}
				if c.card != nil {
					w, err := add("contacts.vcf")
					if err != nil {
						return 0, err
					}
					for _, o := range objs {
						if err := card.Encode(w, o); err != nil {
							return 0, err
						}
					}
					// A card Carrel could not read goes in as the server
					// holds it, as a calendar's record does.
					raw, lacks := c.unreadKept(ctx, bad)
					for _, b := range raw {
						if _, err := w.Write(b); err != nil {
							return 0, err
						}
					}
					return len(objs) + len(raw), lacks
				}
				w, err := add("calendar.ics")
				if err != nil {
					return 0, err
				}
				// A record Carrel could not read goes in as the server
				// holds it; one that cannot is why the collection is not
				// in the archive whole (Entry.Error), never left out
				// without a word.
				raw, lacks := c.unreadKept(ctx, bad)
				writeCalendar(w, objs, raw)
				return len(objs) + len(raw), lacks
			}})
		}
	}
	return out
}

// backupFiles copies a file collection as files, the tree as it is, up to
// CARREL_FILES_MAX_ENTRIES; a larger tree is said to be larger, not cut
// silently.
func (s *Server) backupFiles(ctx context.Context, conn sealed.Connection, col sealed.Collection, add func(string) (io.Writer, error)) (int, error) {
	files, err := s.filesOf(conn)
	if err != nil {
		return 0, err
	}
	count, seen := 0, 0
	var walk func(folder, prefix string) error
	walk = func(folder, prefix string) error {
		entries, err := files.List(ctx, folder)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if seen++; seen > s.cfg.FilesMaxEntries {
				return fmt.Errorf("more than %d entries (CARREL_FILES_MAX_ENTRIES); the rest is not in the archive", s.cfg.FilesMaxEntries)
			}
			if e.Dir {
				if err := walk(e.Path, prefix+e.Name+"/"); err != nil {
					return err
				}
				continue
			}
			d, err := files.Open(ctx, e.Path, nil)
			if err != nil {
				return err
			}
			w, err := add("files/" + prefix + e.Name)
			if err == nil {
				_, err = io.Copy(w, d.Body)
			}
			d.Body.Close()
			if err != nil {
				return err
			}
			count++
		}
		return nil
	}
	err = walk(strings.TrimSuffix(col.Path, "/")+"/", "")
	return count, err
}

// webDest is a folder of a file collection as a backup destination.
type webDest struct {
	files  *file.Provider
	folder string // server path, ends in "/"
	etags  map[string]string
}

func (s *Server) backupDest(blob sealed.Blob, job sealed.BackupJob) (*webDest, error) {
	col, ok := blob.Collection(job.DestAccount, job.DestPath)
	conn, hasConn := blob.Connection(job.DestAccount)
	if !ok || !hasConn || col.Gone || col.ReadOnly || !col.Has(view.KindFile) {
		return nil, errors.New("the destination is not a writable file collection of the catalog")
	}
	files, err := s.filesOf(conn)
	if err != nil {
		return nil, err
	}
	folder := strings.TrimSuffix(col.Path, "/") + "/"
	for _, seg := range job.DestFolder {
		folder += seg + "/"
	}
	return &webDest{files: files, folder: folder, etags: map[string]string{}}, nil
}

func (d *webDest) Create(ctx context.Context, name string, body io.Reader) error {
	// The folder is made on first use, a segment at a time.
	root := d.folder
	var made []string
	for p := root; ; {
		if _, err := d.files.Stat(ctx, p); err == nil {
			break
		} else if !file.NotFound(err) {
			return err
		}
		made = append([]string{p}, made...)
		p = p[:strings.LastIndex(strings.TrimSuffix(p, "/"), "/")+1]
		if p == "" || p == "/" {
			break
		}
	}
	for _, p := range made {
		if err := d.files.MkCol(ctx, p); err != nil {
			return err
		}
	}
	p := d.folder + name
	if _, err := d.files.Stat(ctx, p); err == nil {
		return errors.New(name + " is already there; an archive is never overwritten")
	}
	_, err := d.files.UploadCreate(ctx, p, body)
	return err
}

func (d *webDest) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	dl, err := d.files.Open(ctx, d.folder+name, nil)
	if err != nil {
		return nil, err
	}
	return dl.Body, nil
}

func (d *webDest) List(ctx context.Context) ([]string, error) {
	entries, err := d.files.List(ctx, d.folder)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.Dir {
			out = append(out, e.Name)
			d.etags[e.Name] = e.ETag
		}
	}
	return out, nil
}

func (d *webDest) Delete(ctx context.Context, name string) error {
	return d.files.Delete(ctx, d.folder+name, d.etags[name])
}

func (s *Server) backupManifest(job sealed.BackupJob, at time.Time) backup.Manifest {
	return backup.Manifest{Carrel: s.version, Job: job.Name, Taken: at}
}

func jobPassword(job sealed.BackupJob) string {
	if job.Encrypted {
		return job.Password
	}
	return ""
}

// runState is how a run ended.
func runState(res backup.Result, err error) string {
	switch {
	case err != nil:
		return user.RunFailed
	case !res.Manifest.Complete():
		return user.RunPartial
	default:
		return user.RunDone
	}
}

func finishWith(res backup.Result, err error) func(r *store.BackupRun) {
	return func(r *store.BackupRun) {
		r.State, r.File, r.Size, r.Verified = runState(res, err), res.File, res.Size, res.Verified
		for _, e := range res.Manifest.Collections {
			// Files taken whole are counted as files (s-backup-1); a file
			// collection that failed is a collection not copied.
			if e.Kind == view.KindFile && e.Error == "" {
				r.Files += e.Count
				continue
			}
			r.Collections++
			if e.Error == "" {
				r.Whole++
			} else if r.Error == "" {
				r.Error = e.Name + ": " + e.Error
			}
		}
		if err != nil {
			r.Error = err.Error()
		}
	}
}

// startBackup runs a job to its WebDAV destination in the background, with
// the credentials of blob, and returns the run's ID at once.
func (s *Server) startBackup(userID string, blob sealed.Blob, job sealed.BackupJob, trigger string, via ...string) (string, error) {
	dest, err := s.backupDest(blob, job)
	if err != nil {
		return "", err
	}
	runID, err := s.users.StartBackupRun(userID, job.ID, trigger, job.Mode, runners.isRunning, via...)
	if err != nil {
		return "", err
	}
	runners.set(runID, true)
	go func() {
		defer runners.set(runID, false)
		// The run parses every record of every source; a panic in it is a
		// failed run on the screen (S-24.3), not the end of the process.
		defer func() {
			if r := recover(); r != nil {
				_ = s.users.FinishBackupRun(userID, job.ID, runID, finishWith(backup.Result{}, fmt.Errorf("backup: %v", r)))
			}
		}()
		ctx := context.Background()
		at := s.now()
		res, err := backup.Run(ctx, dest, backup.FileName(job.Name, at, job.Encrypted), job.Name, job.Keep,
			s.backupManifest(job, at), s.backupSources(ctx, blob, job), jobPassword(job))
		_ = s.users.FinishBackupRun(userID, job.ID, runID, finishWith(res, err))
	}()
	return runID, nil
}

// streamBackup writes a fresh archive of the job to w and records the run:
// nothing is written anywhere else, so nothing is read back either.
func (s *Server) streamBackup(ctx context.Context, w http.ResponseWriter, userID string, blob sealed.Blob, job sealed.BackupJob, trigger string, via ...string) error {
	runID, err := s.users.StartBackupRun(userID, job.ID, trigger, sealed.BackupToDownload, runners.isRunning, via...)
	if err != nil {
		return err
	}
	runners.set(runID, true)
	defer runners.set(runID, false)
	at := s.now()
	name := backup.FileName(job.Name, at, job.Encrypted)
	h := w.Header()
	h.Set("Content-Type", "application/zip")
	if job.Encrypted {
		h.Set("Content-Type", "application/octet-stream")
	}
	if v := mime.FormatMediaType("attachment", map[string]string{"filename": name}); v != "" {
		h.Set("Content-Disposition", v)
	}
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	res, err := backup.Stream(w, s.backupManifest(job, at), s.backupSources(ctx, blob, job), jobPassword(job))
	res.File = name
	return s.users.FinishBackupRun(userID, job.ID, runID, finishWith(res, err))
}

// backupOnSignIn runs the jobs whose last whole copy is older than they
// allow, now that a session holds the data key.
func (s *Server) backupOnSignIn(sess *session.Session) {
	blob := s.catalogOf(sess)
	runs := s.users.BackupRuns(sess.AccountID())
	for _, job := range blob.BackupJobs {
		if job.Off || job.OnSignInDays <= 0 || job.Mode == sealed.BackupToDownload {
			continue
		}
		last := user.LastWholeRun(runs[job.ID])
		if !last.IsZero() && time.Since(last) < time.Duration(job.OnSignInDays)*24*time.Hour {
			continue
		}
		_, _ = s.startBackup(sess.AccountID(), blob, job, "sign-in")
	}
}

// The screen.

var runStates = map[string]view.OpState{
	user.RunRunning: view.OpRunning, user.RunDone: view.OpDone, user.RunPartial: view.OpPartial, user.RunFailed: view.OpFailed,
}

func (s *Server) destLabel(blob sealed.Blob, job sealed.BackupJob) string {
	if job.Mode == sealed.BackupToDownload {
		return "Download in the browser — nothing is written to a server"
	}
	col, ok := blob.Collection(job.DestAccount, job.DestPath)
	conn, _ := blob.Connection(job.DestAccount)
	if !ok {
		return "A destination that is no longer in the catalog"
	}
	label := string(stamp(connLabel(conn), collectionLabel(col)))
	if len(job.DestFolder) > 0 {
		label += " /" + strings.Join(job.DestFolder, "/")
	}
	if job.Mode == sealed.BackupToBoth {
		label += " and a download"
	}
	return label
}

func (s *Server) backupPage(sess *session.Session, editing *sealed.BackupJob, errs map[string]view.Text, notices ...view.Notice) view.BackupPage {
	return s.backupPageWith(sess, editing, errs, nil, nil, notices...)
}

func (s *Server) backupPageWith(sess *session.Session, editing *sealed.BackupJob, errs map[string]view.Text, commands, secrets map[string]string, notices ...view.Notice) view.BackupPage {
	blob := s.catalogOf(sess)
	tf, now := s.timeFor(sess), s.now()
	page := view.BackupPage{Shell: s.settingsShell(sess, "Backup", "/app/settings/backup", "/app/settings/backup")}
	page.Notices = append(page.Notices, notices...)
	runs, refusals := s.users.BackupRuns(sess.AccountID()), s.users.BackupRefusals(sess.AccountID())
	var history []historyRun
	for _, job := range blob.BackupJobs {
		row := s.jobRow(sess, blob, job)
		if r := runs[job.ID]; len(r) > 0 {
			row.LastRun, row.LastState, row.LastNote = tf.numStamp(r[0].Started), runStates[r[0].State], lastNote(r[0])
		}
		for _, r := range runs[job.ID] {
			history = append(history, historyRun{job, r})
		}
		// The trigger's refusals are rows of the history only (note 91).
		for _, r := range refusals[job.ID] {
			history = append(history, historyRun{job, r})
		}
		if editing != nil && editing.ID == job.ID {
			// The job's own page (s-backup-2, Нс61): its form; the external
			// trigger has a sheet of its own (Typecase's note 91).
			page.Job = &row
			continue
		}
		// The row is what a job is opened for, Run and Edit (s-backup-1),
		// and in its «⋯» after their twins the external trigger's sheet
		// (note 91); the job is deleted on its page (s-backup-2).
		row.External = view.Action{ID: "backup.trigger", Label: "External trigger", Method: "GET", Enabled: true,
			Href: s.postQuery("/app/settings/backup/trigger", url.Values{"job": {job.ID}})}
		row.Actions = []view.Action{s.backupRunAction(job, "Run"),
			{ID: "backup.edit", Label: "Edit", Href: s.postQuery("/app/settings/backup", url.Values{"job": {job.ID}}), Method: "GET", Enabled: true}}
		// A job's page and a new job's are the job alone (s-backup-2).
		if editing == nil {
			page.Jobs = append(page.Jobs, row)
			page.JobsHint = verifiedHint
		}
	}
	// A new job (s-backup-19): to WebDAV, into the first file collection
	// that can be written, eight kept, encrypted, at sign-in when the last
	// copy is a week old.
	job := sealed.BackupJob{Mode: sealed.BackupToWebDAV, Keep: 8, Encrypted: true, OnSignInDays: 7}
	for _, col := range blob.Collections {
		if !col.Gone && !col.Feed && !col.ReadOnly && col.Has(view.KindFile) {
			job.DestAccount, job.DestPath = col.AccountID, col.Path
			break
		}
	}
	if editing != nil {
		job = *editing
	}
	page.Add = s.backupForm(sess, blob, job, errs)
	if saved, ok := blob.BackupJob(job.ID); ok && job.ID != "" {
		// The job's page is headed by its name and its last copy
		// (s-backup-2: «Job · last copy 2 days ago»).
		// A destination other than WebDAV alone is named, in bold
		// (s-backup-3: «Job · destination **WebDAV and download** · …»).
		last := "no copy yet"
		if r := runs[job.ID]; len(r) > 0 {
			last = "last copy " + string(tf.ago(r[0].Started, now))
		}
		page.Title, page.Subtitle = text(saved.Name), line("Job · "+last)
		if where := jobDestinationWords[saved.Mode]; where != "" {
			page.Subtitle = join("Job · destination ", strongRun(where), " · "+last)
		}
	}
	// The job's own page (s-backup-2): Cancel beside Save, the deletion at
	// its foot with what it does not delete.
	if _, ok := blob.BackupJob(job.ID); ok && job.ID != "" {
		// Run now stands in the list only (note 85): here it would run what
		// was saved, not what is typed.
		page.Add.Actions = append(page.Add.Actions,
			view.Action{ID: "backup.cancel", Label: "Cancel", Href: s.route("/app/settings/backup"), Method: "GET", Enabled: true})
		page.Delete = &view.Action{ID: "backup.delete", Label: "Delete job", Method: "POST", Danger: true, Enabled: true,
			Href: s.postQuery("/app/settings/backup", url.Values{"op": {"delete"}, "job": {job.ID}})}
		page.DeleteNote = "Archives already made stay where they are: the schedule is deleted, not the copies."
	}
	page.Restore = s.restoreBlock(sess, blob, "", "", nil)
	if editing == nil {
		page.History = s.backupHistory(tf, history)
		backupHead(&page, len(blob.BackupJobs))
		page.Actions = []view.Action{
			{ID: "backup.new", Label: "New job", Icon: "i-plus", Href: s.route("/app/settings/backup/new"), Method: "GET", Primary: true, Enabled: true},
			{ID: "restore.start", Label: "Restore", Icon: "i-restore", Href: s.route("/app/settings/backup/restore"), Method: "GET", Enabled: true},
		}
	}
	return page
}

// jobDestinationWords are the destinations a job's page names under its
// title (s-backup-3); WebDAV alone is the usual one and goes unsaid
// (s-backup-2).
var jobDestinationWords = map[string]string{
	sealed.BackupToDownload: "download",
	sealed.BackupToBoth:     "WebDAV and download",
}

// backupHead is the line under the title of the list and its empty state
// (s-backup-1, s-backup-6): how many jobs there are, or that there are none
// and that a restore needs none.
func backupHead(page *view.BackupPage, jobs int) {
	switch jobs {
	case 0:
		page.Subtitle = line("No jobs yet")
		page.Empty = join("Carrel copies raw ", monoRun(".vcf"), ", ", monoRun(".ics"), " and files to storage of your own, with a manifest and a read-back check. Nothing is kept here.")
		page.EmptyNote = join("Already have an archive from another instance? ", strongRun("Restore"), " takes it from a file — a job is not needed for that.")
	case 1:
		page.Subtitle = line("One job · its own sources, destination and trigger")
	default:
		n := countWord(jobs)
		page.Subtitle = line(strings.ToUpper(n[:1]) + n[1:] + " jobs · their own sources, destinations and triggers")
	}
}

// backupRunAction starts the job from the screen. A job that writes to
// WebDAV runs in the background; one that downloads streams its archive in
// this request, so its Run is the download (s-backup-1 draws Run on every
// job).
func (s *Server) backupRunAction(job sealed.BackupJob, label view.Text) view.Action {
	if job.Mode == sealed.BackupToDownload {
		return view.Action{ID: "backup.run", Label: label, Method: "GET", Enabled: true,
			Href: s.postQuery("/app/settings/backup", url.Values{"op": {"download"}, "job": {job.ID}})}
	}
	return view.Action{ID: "backup.run", Label: label, Method: "POST", Enabled: true,
		Href: s.postQuery("/app/settings/backup", url.Values{"op": {"run"}, "job": {job.ID}})}
}

func selValue(accountID, path string) string { return accountID + "|" + path }

func (s *Server) backupForm(sess *session.Session, blob sealed.Blob, job sealed.BackupJob, errs map[string]view.Text) view.Form {
	chosen := map[string]bool{}
	for _, sel := range job.Sources {
		chosen[selValue(sel.AccountID, sel.Path)] = true
	}
	var srcOpts, destOpts []view.Option
	for _, conn := range blob.Connections {
		first := true
		for _, col := range blob.CollectionsOf(conn.ID) {
			if col.Gone {
				continue
			}
			v := selValue(conn.ID, col.Path)
			// What to copy goes by connection (s-backup-2, Typecase's note
			// 85): its collections under it, each its mark and its kind.
			o := view.Option{Value: v, Label: text(collectionLabel(col)), Selected: chosen[v], Colour: catalogColour(col), Meta: sourceKind(col)}
			if first {
				o.Group, first = text(connLabel(conn)), false
			}
			srcOpts = append(srcOpts, o)
			// A read-only file collection is shown and cannot be taken
			// (s-backup-9).
			if col.Has(view.KindFile) {
				d := view.Option{Value: v, Label: text(string(stamp(connLabel(conn), collectionLabel(col)))), Colour: catalogColour(col),
					Selected: v == selValue(job.DestAccount, job.DestPath) && !col.ReadOnly}
				if col.ReadOnly {
					d.Disabled, d.Reason = true, "Read-only: this account cannot write here"
				}
				destOpts = append(destOpts, d)
			}
		}
	}
	modes := []view.Option{
		{Value: sealed.BackupToWebDAV, Label: "To WebDAV", Selected: job.Mode == sealed.BackupToWebDAV},
		{Value: sealed.BackupToDownload, Label: "Download", Selected: job.Mode == sealed.BackupToDownload},
		{Value: sealed.BackupToBoth, Label: "Both", Selected: job.Mode == sealed.BackupToBoth},
	}
	encrypted, on := "", ""
	if job.Encrypted {
		encrypted = "1"
	}
	if !job.Off {
		on = "1"
	}
	// A password set is not sent back; the field says there is one (s-backup-2).
	var passwordSet view.Text
	if job.Password != "" {
		passwordSet = "••••••••••••"
	}
	// A job not made yet is created (s-backup-19, Typecase's note 99).
	form := withPrimary(s.plainForm("/app/settings/backup?op=create", "POST"), "backup.create", "Create")
	if job.ID != "" {
		form = withPrimary(s.plainForm("/app/settings/backup?op=save&job="+url.QueryEscape(job.ID), "POST"), "backup.save", "Save")
	}
	// Browse sends the form as typed and answers with the folder chooser
	// over the job (s-backup-2, note 85).
	browseQ := url.Values{"op": {"browse"}, "job": {job.ID}}
	if job.ID == "" {
		browseQ.Set("job", "new")
	}
	browse := view.Action{ID: "backup.browse", Label: "Browse", Method: "POST", Enabled: true, Submits: true, Href: s.postQuery("/app/settings/backup", browseQ)}
	signin := ""
	if job.OnSignInDays > 0 {
		signin = "on"
	}
	days := job.OnSignInDays
	if days <= 0 {
		days = 7
	}
	// Name and the switch, then a rubric each (s-backup-2, Нс57).
	form.Sets = []view.FieldSet{
		{Rows: []view.FormRow{
			csrfField(sess),
			{Name: "name", Label: "Name", Kind: view.InputText, Value: text(job.Name), Required: true, Error: errs["name"]},
			{Name: "on", Label: "Job is on", Kind: view.InputCheck, Value: text(on),
				Options: []view.Option{{Value: "1", Label: "Job is on", Selected: !job.Off}}},
		}},
		{Legend: "What to copy", LegendAside: s.chosenAside(sess.AccountID(), blob, job),
			HintRuns: join("Address books go as ", monoRun(".vcf"), "; calendars, tasks and notes as ", monoRun(".ics"), "; files as files."),
			Rows:     []view.FormRow{{Name: "src", Label: "What to copy", Kind: view.InputCheck, Options: srcOpts, Error: errs["src"]}}},
		{Legend: "Where to", Rows: []view.FormRow{
			{Name: "mode", Label: "Where to", Kind: view.InputRadio, Options: modes,
				// Р-77 (Typecase's note 166): only what the screen does not say.
				HintRuns: join(strongRun("Download"), " writes nothing to the server. On a schedule it runs only from an external trigger: someone has to take the file. ",
					strongRun("Both"), " does the two, and its trigger has ", strongRun("two steps"), ".")},
			{Name: "dest", Label: "Collection", Kind: view.InputSelect, Options: destOpts, Error: errs["dest"]},
			{Name: "folder", Label: "Folder in it", Kind: view.InputText, Value: text(strings.Join(job.DestFolder, "/")), Error: errs["folder"], Mono: true, Extra: &browse},
			{Name: "keep", Label: "Keep copies", Kind: view.InputNumber, Value: text(strconv.Itoa(job.Keep)), Error: errs["keep"], Mono: true,
				HintRuns: join("Older copies are deleted only ", strongRun("after"), " the new one is checked.")},
		}},
		{Legend: "How the archive is closed", Rows: []view.FormRow{
			{Name: "encrypted", Label: "Encrypt the archive", Kind: view.InputCheck, Value: text(encrypted),
				Options: []view.Option{{Value: "1", Label: "Encrypt the archive", Selected: job.Encrypted}},
				Hint:    "Turn it off only where nobody but you can reach the copy."},
			// The owner's words (19.10.2026, Typecase's note 95).
			{Name: "password", Label: "Archive password", Kind: view.InputPassword, Error: errs["password"], Placeholder: passwordSet,
				Required: job.ID == "", Hint: "Separate from the sign-in password."},
		}},
		// One tick (the owner, 19.10.2026; note 85): the copy at sign-in when
		// the last is older than so many days; what else starts it is said
		// under it — Run now of the list, the line of the job's page.
		{Legend: "When",
			HintRuns: view.Runs{strongRun("Also started by"), {Mark: view.RunBreak}, {Text: "• Run now — in the job list"}, {Mark: view.RunBreak},
				{Text: "• Your own clock — "}, monoRun("cron"), {Text: ", GoSentry, a job in Gitea, "}, monoRun("curl"), {Text: ": External trigger in the job’s menu"}},
			Rows: []view.FormRow{
				{Name: "signin.on", Label: "On sign-in, if the last copy is older than", Kind: view.InputCheck, Value: text(signin),
					Options: []view.Option{{Value: "on", Label: "On sign-in, if the last copy is older than", Selected: signin != ""}}},
				{Name: "signin", Label: "Days", Kind: view.InputNumber, Mono: true, Value: text(strconv.Itoa(days))},
			}},
	}
	return focused(form, "name")
}

// chosenAside is the sum after «What to copy» (s-backup-2): what is chosen
// and how big the last copy was — «2 collections and 1 folder chosen · 2.1
// GB last time».
func (s *Server) chosenAside(viewerID string, blob sealed.Blob, job sealed.BackupJob) view.Text {
	cols, folders := 0, 0
	for _, sel := range job.Sources {
		col, ok := blob.Collection(sel.AccountID, sel.Path)
		switch {
		case !ok:
		case col.Has(view.KindFile):
			folders++
		default:
			cols++
		}
	}
	if cols+folders == 0 {
		return "nothing chosen" // s-backup-19
	}
	var parts []string
	if cols > 0 {
		parts = append(parts, plural(cols, "collection"))
	}
	if folders > 0 {
		parts = append(parts, plural(folders, "folder"))
	}
	aside := strings.Join(parts, " and ") + " chosen"
	if job.ID == "" {
		return text(aside)
	}
	for _, r := range s.users.BackupRuns(viewerID)[job.ID] {
		if r.State != user.RunRunning && r.Size > 0 {
			return text(aside + " · " + string(formatSize(r.Size)) + " last time")
		}
	}
	return text(aside)
}

// jobFromForm reads the editor into job, and what is wrong with it.
func (s *Server) jobFromForm(r *http.Request, blob sealed.Blob, job sealed.BackupJob) (sealed.BackupJob, map[string]view.Text) {
	errs := map[string]view.Text{}
	job.Name = strings.TrimSpace(r.PostFormValue("name"))
	if job.Name == "" {
		errs["name"] = "Give the job a name."
	}
	job.Sources = nil
	for _, v := range r.PostForm["src"] {
		acc, path, ok := strings.Cut(v, "|")
		if col, has := blob.Collection(acc, path); ok && has && !col.Gone && !col.Feed {
			job.Sources = append(job.Sources, sealed.Selection{Kind: col.Kinds[0], AccountID: acc, Path: path})
		}
	}
	if len(job.Sources) == 0 {
		errs["src"] = "Choose at least one collection."
	}
	job.Mode = r.PostFormValue("mode")
	switch job.Mode {
	case sealed.BackupToWebDAV, sealed.BackupToDownload, sealed.BackupToBoth:
	default:
		job.Mode = sealed.BackupToWebDAV
	}
	job.DestAccount, job.DestPath, job.DestFolder = "", "", nil
	if job.Mode != sealed.BackupToDownload {
		acc, path, _ := strings.Cut(r.PostFormValue("dest"), "|")
		col, ok := blob.Collection(acc, path)
		if !ok || col.Gone || col.ReadOnly || !col.Has(view.KindFile) {
			errs["dest"] = "Choose a file collection you can write to."
		}
		job.DestAccount, job.DestPath = acc, path
		segs, ok := relSegs(r.PostFormValue("folder"))
		if !ok {
			errs["folder"] = "A folder is names separated by /, without . or .."
		}
		job.DestFolder = segs
	}
	keep, err := strconv.Atoi(strings.TrimSpace(r.PostFormValue("keep")))
	if err != nil || keep < 1 || keep > 1000 {
		errs["keep"] = "Keep at least one copy."
		keep = 1
	}
	job.Keep = keep
	job.Encrypted = r.PostFormValue("encrypted") != ""
	job.Off = r.PostFormValue("on") == ""
	if pw := r.PostFormValue("password"); pw != "" {
		job.Password = pw
	}
	if job.Encrypted && job.Password == "" {
		errs["password"] = "An encrypted archive needs its own password."
	}
	job.OnSignInDays, _ = strconv.Atoi(strings.TrimSpace(r.PostFormValue("signin")))
	// The tick off is never; the days stay a field of it (note 85).
	if v := r.PostFormValue("signin.on"); v != "on" && v != "true" {
		job.OnSignInDays = 0
	}
	if job.OnSignInDays < 0 {
		job.OnSignInDays = 0
	}
	return job, errs
}

func (s *Server) getSettingsBackup(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireLogin(w, r)
	if !ok {
		return
	}
	blob := s.catalogOf(sess)
	q := r.URL.Query()
	// New job has an address of its own (s-backup-19); ?job=new is the
	// same, as Browse comes back to it.
	if strings.HasSuffix(r.URL.Path, "/backup/new") {
		q.Set("job", "new")
	}
	job, has := blob.BackupJob(q.Get("job"))
	switch q.Get("op") {
	case "download":
		if !has || job.Mode == sealed.BackupToWebDAV {
			s.notFound(w, r)
			return
		}
		if err := s.streamBackup(r.Context(), w, sess.AccountID(), blob, job, "screen"); errors.Is(err, user.ErrBackupRunning) {
			s.writePage(w, r, http.StatusOK, s.backupPage(sess, nil, nil, runningNotice(job)))
		}
		return
	case "get":
		s.getBackupArchive(w, r, sess.AccountID(), blob, job, has, q.Get("run"))
		return
	}
	var editing *sealed.BackupJob
	if has {
		editing = &job
	}
	// Back from the folder chooser of Browse (note 85): the job as it was
	// typed, with the folder chosen — or as it was, Cancel.
	if held, ok := heldJob(sess, q.Get("job")); ok && (q.Get("held") != "" || q.Get("op") == "folder") {
		if dest, ok := s.folderAddress(sess, q.Get("to")); ok && q.Get("op") == "folder" && !dest.col.ReadOnly {
			held.DestAccount, held.DestPath, held.DestFolder = dest.conn.ID, dest.col.Path, slices.Clone(dest.segs)
		}
		editing = &held
	}
	w.Header().Set("Cache-Control", "no-store")
	page := s.backupPage(sess, editing, nil)
	switch {
	case q.Get("job") == "new":
		// New job (s-backup-19): the page of a job not made yet — the form,
		// Create and Cancel, no Delete job.
		page.Title, page.Subtitle, page.Actions, page.Empty, page.EmptyNote = "New job", line("Nothing is copied until it is created"), nil, nil, nil
		page.Jobs, page.History = nil, nil
		page.Add.Actions = append(page.Add.Actions,
			view.Action{ID: "backup.cancel", Label: "Cancel", Href: s.route("/app/settings/backup"), Method: "GET", Enabled: true})
	}
	s.writePage(w, r, http.StatusOK, page)
}

func runningNotice(job sealed.BackupJob) view.Notice {
	return view.Notice{Role: view.NoticeInfo, Title: text("«" + job.Name + "» is running"), Body: "A job runs once at a time. Its result will be in the history."}
}

// getBackupArchive hands over an archive a run wrote, streamed from where it
// lies — never from Carrel's volume, which holds none (S-1).
func (s *Server) getBackupArchive(w http.ResponseWriter, r *http.Request, userID string, blob sealed.Blob, job sealed.BackupJob, has bool, runID string) {
	if !has {
		s.notFound(w, r)
		return
	}
	var run *store.BackupRun
	for _, r := range s.users.BackupRuns(userID)[job.ID] {
		if r.ID == runID && r.File != "" && r.Verified {
			run = &r
			break
		}
	}
	dest, err := s.backupDest(blob, job)
	if run == nil || err != nil {
		s.notFound(w, r)
		return
	}
	body, err := dest.Open(r.Context(), run.File)
	if err != nil {
		s.notFound(w, r)
		return
	}
	defer body.Close()
	if v := mime.FormatMediaType("attachment", map[string]string{"filename": run.File}); v != "" {
		w.Header().Set("Content-Disposition", v)
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.Copy(w, body)
}

// postSettingsBackup saves (op=save), deletes (op=delete) or runs (op=run)
// a job.
func (s *Server) postSettingsBackup(w http.ResponseWriter, r *http.Request) {
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
	id := r.URL.Query().Get("job")
	if id == "new" {
		id = "" // Browse of a job not made yet (note 85)
	}
	job, has := blob.BackupJob(id)
	if id != "" && !has {
		s.notFound(w, r)
		return
	}
	switch op := r.URL.Query().Get("op"); op {
	case "browse":
		s.browseJob(w, r, sess, blob, job)
	case "save", "create":
		if (op == "create") != (id == "") {
			s.notFound(w, r)
			return
		}
		next, errs := s.jobFromForm(r, blob, job)
		if len(errs) > 0 {
			s.writePage(w, r, http.StatusOK, s.backupPage(sess, &next, errs))
			return
		}
		if _, err := s.users.SaveBackupJob(sess.AccountID(), key, next); err != nil {
			http.Error(w, "could not save the job", http.StatusInternalServerError)
			return
		}
		s.redirect(w, r, "/app/settings/backup")
	case "delete":
		if !has {
			s.notFound(w, r)
			return
		}
		if err := s.users.DeleteBackupJob(sess.AccountID(), key, id); err != nil {
			http.Error(w, "could not delete the job", http.StatusInternalServerError)
			return
		}
		s.redirect(w, r, "/app/settings/backup")
	case "switch":
		// The tick of the list's row (s-backup-1): «Job is on» alone.
		if !has {
			s.notFound(w, r)
			return
		}
		job.Off = !job.Off
		if _, err := s.users.SaveBackupJob(sess.AccountID(), key, job); err != nil {
			http.Error(w, "could not save the job", http.StatusInternalServerError)
			return
		}
		s.redirect(w, r, "/app/settings/backup")
	case "run":
		if !has || job.Mode == sealed.BackupToDownload {
			s.notFound(w, r)
			return
		}
		_, err := s.startBackup(sess.AccountID(), blob, job, "screen")
		switch {
		case errors.Is(err, user.ErrBackupRunning):
			s.writePage(w, r, http.StatusOK, s.backupPage(sess, nil, nil, runningNotice(job)))
		case err != nil:
			s.writePage(w, r, http.StatusOK, s.backupPage(sess, nil, nil, view.Notice{Role: view.NoticeAlert, Title: "The job cannot start", Body: text(err.Error())}))
		default:
			s.redirect(w, r, "/app/settings/backup")
		}
	default:
		s.notFound(w, r)
	}
}

// jobRow is a job as the list's row says it (s-backup-1, Нс55): on or off,
// the marks of what it copies and the sum, where to and when, each in two
// lines, how many copies it keeps.
func (s *Server) jobRow(sess *session.Session, blob sealed.Blob, job sealed.BackupJob) view.BackupJob {
	row := view.BackupJob{Name: text(job.Name), On: !job.Off, Keep: job.Keep, Encrypted: job.Encrypted}
	label := "Turn the job off"
	if job.Off {
		label = "Turn the job on"
	}
	row.Toggle = &view.Action{ID: "backup.switch", Label: text(label), Method: "POST", Enabled: true,
		Href: s.postQuery("/app/settings/backup", url.Values{"op": {"switch"}, "job": {job.ID}})}
	cols, books, files := 0, 0, 0
	for _, sel := range job.Sources {
		col, ok := blob.Collection(sel.AccountID, sel.Path)
		if !ok {
			continue
		}
		if c := catalogColour(col); c != "" {
			row.Marks = append(row.Marks, c)
		}
		switch {
		case col.Has(view.KindFile):
			files++
		case len(col.Kinds) == 1 && col.Kinds[0] == view.KindContact:
			books++
			cols++
		default:
			cols++
		}
	}
	switch {
	case cols > 0 && cols == books:
		row.Copies = text(plural(books, "book"))
	case cols > 0:
		row.Copies = text(plural(cols, "collection"))
	}
	if files > 0 {
		if row.Copies == "" {
			row.Copies = "files"
		} else {
			row.Copies += " + files"
		}
	}
	switch job.Mode {
	case sealed.BackupToDownload:
		row.Where, row.WhereNote, row.Keep = "download in browser", "nothing is written", 0
	default:
		col, ok := blob.Collection(job.DestAccount, job.DestPath)
		conn, _ := blob.Connection(job.DestAccount)
		if !ok {
			row.Where = "a destination that is no longer in the catalog"
			break
		}
		row.Where = text(string(stamp(connLabel(conn), collectionLabel(col))))
		row.WhereNote = text("/" + strings.Join(job.DestFolder, "/"))
		if job.Mode == sealed.BackupToBoth {
			row.WhereNote += " · and a download"
		}
	}
	// When is only what is set (owner's decision 19.10, Typecase's note 95):
	// the sign-in rule in words, an issued trigger password a badge; the
	// schedule of an external trigger Carrel does not know.
	if job.OnSignInDays > 0 {
		row.When, row.WhenNote = "on sign-in", text(fmt.Sprintf("if older than %d days", job.OnSignInDays))
	}
	row.Triggered = s.users.BackupTriggerOn(sess.AccountID(), job.ID)
	return row
}

// verifiedHint stands under the jobs (s-backup-1, -12, -13, -14).
const verifiedHint = "verified — the archive was read back from where it was written and the checksum matched; otherwise it is marked red."

// verifiedRun is the word of a copy read back from where it was written
// (s-backup-1); one written and not read back is marked red, and one that
// was only downloaded was written nowhere.
func verifiedRun(r store.BackupRun) view.Run {
	switch {
	case r.State == user.RunRunning:
		return view.Run{Text: "running"}
	case r.Mode == sealed.BackupToDownload:
		return view.Run{Text: "downloaded"}
	case r.Verified:
		return view.Run{Text: "verified", Hint: "The archive was read back from the destination and the checksum matched"}
	}
	return view.Run{Text: "not verified", Mark: view.RunDanger}
}

// lastNote is the line under a job's last copy: its size and whether it
// was read back (s-backup-1: «18.4 MB · verified»).
func lastNote(r store.BackupRun) view.Runs {
	if r.State == user.RunRunning {
		return line("running")
	}
	return join(string(formatSize(r.Size))+" · ", verifiedRun(r))
}

// historyRun is a run of a job, for the history of all jobs.
type historyRun struct {
	job sealed.BackupJob
	run store.BackupRun
}

// historyTriggers are what started a run, as the history says it.
var historyTriggers = map[string]view.Text{"external": "external", "sign-in": "on sign-in", "screen": "by hand"}

// backupHistory is «History, all jobs» (s-backup-1, Нс56): every run of
// every job, newest first — when, which job, how big, how many collections
// went in whole, whether it was read back and what was not copied, what
// started it; Restore and Download of a copy that can be had.
func (s *Server) backupHistory(tf timeFmt, runs []historyRun) []view.BackupHistoryRow {
	sort.SliceStable(runs, func(i, j int) bool { return runs[i].run.Started.After(runs[j].run.Started) })
	var out []view.BackupHistoryRow
	for _, h := range runs {
		r := h.run
		row := view.BackupHistoryRow{
			ID: text(r.ID), At: tf.numStamp(r.Started), Job: text(h.job.Name), Size: formatSize(r.Size),
			Collections: text(fmt.Sprintf("%d / %d", r.Whole, r.Collections)), Whole: r.Whole >= r.Collections,
			Trigger: historyTriggers[r.Trigger],
		}
		if r.Files > 0 {
			files := " files"
			if r.Files == 1 {
				files = " file"
			}
			row.Collections += text(" + " + grouped(r.Files) + files)
		}
		// An external run names the password it came with and where from
		// (note 91).
		if r.Password != "" {
			row.Via = text(r.Password + " · " + r.From)
		}
		// A start the trigger refused: its own row, nothing to restore.
		if r.State == user.RunRefused {
			row.Size, row.Collections, row.Whole = "—", "—", true
			row.Verified = view.Runs{{Text: "refused: not this job’s password", Mark: view.RunDanger}}
			out = append(out, row)
			continue
		}
		if r.Verified {
			row.Verified = line("read back")
		} else {
			row.Verified = runs1(verifiedRun(r))
		}
		if r.State != user.RunRunning && r.Whole < r.Collections {
			missing := view.Run{Text: text(fmt.Sprintf("%d not copied", r.Collections-r.Whole)), Mark: view.RunDanger, Hint: text(r.Error)}
			row.Verified = append(row.Verified, view.Run{Text: " · "}, missing)
		}
		if r.File != "" && r.Mode != sealed.BackupToDownload && r.Verified {
			row.Actions = []view.Action{
				{ID: "backup.restore", Label: "Restore", Icon: "i-restore", Method: "GET", Enabled: true,
					Href: s.postQuery("/app/settings/backup/restore", url.Values{"from": {"job"}, "run": {h.job.ID + "|" + r.ID}})},
				{ID: "backup.get", Label: "Download", Icon: "i-download", Method: "GET", Enabled: true,
					Href: s.postQuery("/app/settings/backup", url.Values{"op": {"get"}, "job": {h.job.ID}, "run": {r.ID}})},
			}
		}
		out = append(out, row)
	}
	return out
}

// runs1 is one piece as a sentence.
func runs1(r view.Run) view.Runs { return view.Runs{r} }

// sourceKind is what a source of a job is, as the tree of «What to copy»
// says it beside its name (s-backup-2): cal., tasks, notes, book, files —
// a read-only storage «files · ro».
func sourceKind(col sealed.Collection) view.Text {
	switch {
	case col.Has(view.KindFile) && col.ReadOnly:
		return "files · ro"
	case col.Has(view.KindFile):
		return "files"
	case col.Has(view.KindContact):
		return "book"
	case col.Has(view.KindEvent):
		return "cal."
	case col.Has(view.KindTask):
		return "tasks"
	case col.Has(view.KindNote):
		return "notes"
	}
	return ""
}

// browseJob is Browse of a job's form (s-backup-2, note 85): the job as
// typed is held in the session, and the answer is the chooser of its
// folder over it — it opens at the folder typed, or at the collection.
func (s *Server) browseJob(w http.ResponseWriter, r *http.Request, sess *session.Session, blob sealed.Blob, job sealed.BackupJob) {
	next, _ := s.jobFromForm(r, blob, job)
	if next.Password == "" {
		next.Password = job.Password
	}
	raw, err := json.Marshal(next)
	if err != nil {
		http.Error(w, "could not hold the form", http.StatusInternalServerError)
		return
	}
	sess.SetDraft(session.DraftBackupJob, raw)
	q := url.Values{"for": {"backup"}, "job": {jobKey(next)}}
	if fc, ok := s.fileCollection(sess, next.DestAccount, next.DestPath); ok {
		if _, err := fc.path(next.DestFolder, true); err == nil {
			q.Set("to", s.route(fc.href("", next.DestFolder, true)))
		}
	}
	s.redirect(w, r, "/app/files/picker?"+q.Encode())
}

// jobKey is how the address names a job: its ID, «new» before it is made.
func jobKey(job sealed.BackupJob) string {
	if job.ID == "" {
		return "new"
	}
	return job.ID
}

// heldJob is the job Browse held for the job key, if it is that one.
func heldJob(sess *session.Session, key string) (sealed.BackupJob, bool) {
	var job sealed.BackupJob
	raw := sess.Draft(session.DraftBackupJob)
	if raw == nil || json.Unmarshal(raw, &job) != nil || jobKey(job) != key {
		return sealed.BackupJob{}, false
	}
	return job, true
}
