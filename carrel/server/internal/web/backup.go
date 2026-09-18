package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
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
		entry.Connection, entry.Name, entry.Kinds = serverHost(conn.Server), collectionLabel(col), col.Kinds
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
				objs, err := c.allRecords(ctx)
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
					return len(objs), nil
				}
				w, err := add("calendar.ics")
				if err != nil {
					return 0, err
				}
				writeCalendar(w, objs)
				return len(objs), nil
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
		r.Collections = len(res.Manifest.Collections)
		for _, e := range res.Manifest.Collections {
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
func (s *Server) startBackup(userID string, blob sealed.Blob, job sealed.BackupJob, trigger string) (string, error) {
	dest, err := s.backupDest(blob, job)
	if err != nil {
		return "", err
	}
	runID, err := s.users.StartBackupRun(userID, job.ID, trigger, job.Mode, runners.isRunning)
	if err != nil {
		return "", err
	}
	runners.set(runID, true)
	go func() {
		defer runners.set(runID, false)
		ctx := context.Background()
		at := time.Now()
		res, err := backup.Run(ctx, dest, backup.FileName(job.Name, at, job.Encrypted), job.Name, job.Keep,
			s.backupManifest(job, at), s.backupSources(ctx, blob, job), jobPassword(job))
		_ = s.users.FinishBackupRun(userID, job.ID, runID, finishWith(res, err))
	}()
	return runID, nil
}

// streamBackup writes a fresh archive of the job to w and records the run:
// nothing is written anywhere else, so nothing is read back either.
func (s *Server) streamBackup(ctx context.Context, w http.ResponseWriter, userID string, blob sealed.Blob, job sealed.BackupJob, trigger string) error {
	runID, err := s.users.StartBackupRun(userID, job.ID, trigger, sealed.BackupToDownload, runners.isRunning)
	if err != nil {
		return err
	}
	runners.set(runID, true)
	defer runners.set(runID, false)
	at := time.Now()
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
		if job.OnSignInDays <= 0 || job.Mode == sealed.BackupToDownload {
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
	label := string(stamp(serverHost(conn.Server), collectionLabel(col)))
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

// backupPageIssued is the page right after a trigger was issued: its command
// and secret, shown this once.
func (s *Server) backupPageIssued(sess *session.Session, commands, secrets map[string]string) view.BackupPage {
	return s.backupPageWith(sess, nil, nil, commands, secrets)
}

func (s *Server) backupPageWith(sess *session.Session, editing *sealed.BackupJob, errs map[string]view.Text, commands, secrets map[string]string, notices ...view.Notice) view.BackupPage {
	blob := s.catalogOf(sess)
	tf, now := s.timeFor(sess), time.Now()
	page := view.BackupPage{Shell: s.buildShell(sess, "Backup")}
	page.Notices = append(page.Notices, notices...)
	runs := s.users.BackupRuns(sess.AccountID())
	for _, job := range blob.BackupJobs {
		row := view.BackupJob{
			Name: text(job.Name), Destination: text(s.destLabel(blob, job)), Keep: job.Keep, Encrypted: job.Encrypted,
		}
		for _, sel := range job.Sources {
			if col, ok := blob.Collection(sel.AccountID, sel.Path); ok {
				conn, _ := blob.Connection(sel.AccountID)
				row.Sources = append(row.Sources, stamp(serverHost(conn.Server), collectionLabel(col)))
			}
		}
		for i, r := range runs[job.ID] {
			if i == 0 {
				row.LastRun, row.LastState = tf.formatWhen(r.Started, true, now), runStates[r.State]
				if r.State == user.RunPartial || r.State == user.RunFailed {
					page.Notices = append(page.Notices, view.Notice{
						Role: view.NoticeAlert, Title: text("The last copy of «" + job.Name + "» is not whole"),
						Body: text(fmt.Sprintf("%d of %d collections went in. %s", r.Whole, r.Collections, r.Error)),
					})
				}
			}
			run := view.BackupRun{ID: text(r.ID), Started: tf.formatWhen(r.Started, false, now), State: runStates[r.State], Size: formatSize(r.Size)}
			if r.File != "" && r.Mode != sealed.BackupToDownload && r.Verified {
				run.Download = &view.Action{ID: "backup.get", Label: "Download", Icon: "i-down", Method: "GET", Enabled: true,
					Href: s.postQuery("/app/settings/backup", url.Values{"op": {"get"}, "job": {job.ID}, "run": {r.ID}})}
			}
			row.Runs = append(row.Runs, run)
		}
		row.Trigger = s.triggerBlock(sess, job, commands)
		if secrets[job.ID] != "" {
			row.Trigger.Password.Value = text(secrets[job.ID])
		}
		q := func(op string) url.Values { return url.Values{"op": {op}, "job": {job.ID}} }
		if s.users.BackupTriggerOn(sess.AccountID(), job.ID) {
			row.Actions = append(row.Actions, view.Action{ID: "backup.trigger.off", Label: "Turn the external trigger off", Href: s.postQuery("/app/settings/backup", q("trigger-off")), Method: "POST", Enabled: true})
		}
		if job.Mode != sealed.BackupToDownload {
			row.Actions = append(row.Actions, view.Action{ID: "backup.run", Label: "Run", Href: s.postQuery("/app/settings/backup", q("run")), Method: "POST", Enabled: true})
		}
		if job.Mode != sealed.BackupToWebDAV {
			row.Actions = append(row.Actions, view.Action{ID: "backup.download", Label: "Download a copy", Icon: "i-down", Href: s.postQuery("/app/settings/backup", q("download")), Method: "GET", Enabled: true})
		}
		row.Actions = append(row.Actions,
			view.Action{ID: "backup.edit", Label: "Edit", Href: s.postQuery("/app/settings/backup", url.Values{"job": {job.ID}}), Method: "GET", Enabled: true},
			view.Action{ID: "backup.delete", Label: "Delete job", Href: s.postQuery("/app/settings/backup", q("delete")), Method: "POST", Danger: true, Enabled: true,
				Reason: "Archives already made stay where they are: the job is deleted, not the copies."},
		)
		page.Jobs = append(page.Jobs, row)
	}
	job := sealed.BackupJob{Mode: sealed.BackupToWebDAV, Keep: 8, Encrypted: true}
	if editing != nil {
		job = *editing
	}
	page.Add = s.backupForm(sess, blob, job, errs)
	page.Restore.Note = "Restoring creates: the records go into a new collection, never over what is there now."
	return page
}

func selValue(accountID, path string) string { return accountID + "|" + path }

func (s *Server) backupForm(sess *session.Session, blob sealed.Blob, job sealed.BackupJob, errs map[string]view.Text) view.Form {
	chosen := map[string]bool{}
	for _, sel := range job.Sources {
		chosen[selValue(sel.AccountID, sel.Path)] = true
	}
	var srcOpts, destOpts []view.Option
	for _, conn := range blob.Connections {
		for _, col := range blob.CollectionsOf(conn.ID) {
			if col.Gone {
				continue
			}
			v := selValue(conn.ID, col.Path)
			label := text(string(stamp(serverHost(conn.Server), collectionLabel(col))))
			srcOpts = append(srcOpts, view.Option{Value: v, Label: label, Selected: chosen[v]})
			if col.Has(view.KindFile) && !col.ReadOnly {
				destOpts = append(destOpts, view.Option{Value: v, Label: label, Selected: v == selValue(job.DestAccount, job.DestPath)})
			}
		}
	}
	modes := []view.Option{
		{Value: sealed.BackupToWebDAV, Label: "To WebDAV", Selected: job.Mode == sealed.BackupToWebDAV},
		{Value: sealed.BackupToDownload, Label: "Download", Selected: job.Mode == sealed.BackupToDownload},
		{Value: sealed.BackupToBoth, Label: "Both", Selected: job.Mode == sealed.BackupToBoth},
	}
	encrypted := ""
	if job.Encrypted {
		encrypted = "1"
	}
	passwordHint := view.Text("Separate from the sign-in password: the destination may be somebody else's hosting. Forget it and this job's archives open for nobody, including you.")
	if job.Password != "" {
		passwordHint = "A password is set. Leave the field empty to keep it."
	}
	action := "/app/settings/backup?op=save"
	if job.ID != "" {
		action += "&job=" + url.QueryEscape(job.ID)
	}
	return s.plainForm(action, "POST",
		csrfField(sess),
		view.FormRow{Name: "name", Label: "Name", Kind: view.InputText, Value: text(job.Name), Required: true, Error: errs["name"]},
		view.FormRow{Name: "src", Label: "What to copy", Kind: view.InputCheck, Options: srcOpts, Error: errs["src"],
			Hint: "A file collection is copied as files. Tasks and notes live inside calendar collections and are copied with them."},
		view.FormRow{Name: "mode", Label: "Where to", Kind: view.InputRadio, Options: modes},
		view.FormRow{Name: "dest", Label: "Collection", Kind: view.InputSelect, Options: destOpts, Error: errs["dest"]},
		view.FormRow{Name: "folder", Label: "Folder", Kind: view.InputText, Value: text(strings.Join(job.DestFolder, "/")), Error: errs["folder"],
			Hint: "Created on first use."},
		view.FormRow{Name: "keep", Label: "Keep copies", Kind: view.InputNumber, Value: text(strconv.Itoa(job.Keep)), Error: errs["keep"],
			Hint: "Copies past this number are deleted after the new one has been read back and checked, not before. An archive is dated and never overwritten."},
		view.FormRow{Name: "encrypted", Label: "Encrypt the archive", Kind: view.InputCheck, Value: text(encrypted),
			Options: []view.Option{{Value: "1", Label: "Encrypt the archive", Selected: job.Encrypted}},
			Hint:    "On by default. Turning it off makes sense in one case only: the copy goes where nobody but you gets in anyway — and then any archiver opens it, without Carrel and without a password."},
		view.FormRow{Name: "password", Label: "Archive password", Kind: view.InputPassword, Error: errs["password"], Hint: passwordHint},
		view.FormRow{Name: "signin", Label: "On sign-in, if the last copy is older than (days)", Kind: view.InputNumber,
			Value: text(strconv.Itoa(job.OnSignInDays)), Hint: "0 is never. Carrel keeps no clock of its own."},
	)
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
	if pw := r.PostFormValue("password"); pw != "" {
		job.Password = pw
	}
	if job.Encrypted && job.Password == "" {
		errs["password"] = "An encrypted archive needs its own password."
	}
	job.OnSignInDays, _ = strconv.Atoi(strings.TrimSpace(r.PostFormValue("signin")))
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
	w.Header().Set("Cache-Control", "no-store")
	s.writePage(w, r, http.StatusOK, s.backupPage(sess, editing, nil))
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
	job, has := blob.BackupJob(id)
	if id != "" && !has {
		s.notFound(w, r)
		return
	}
	switch r.URL.Query().Get("op") {
	case "save":
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
	case "trigger":
		if !has {
			s.notFound(w, r)
			return
		}
		s.enableTrigger(w, r, sess, key, job)
	case "trigger-off":
		if !has {
			s.notFound(w, r)
			return
		}
		if err := s.users.DisableBackupTrigger(sess.AccountID(), job.ID); err != nil {
			http.Error(w, "could not turn the trigger off", http.StatusInternalServerError)
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
