package web

import (
	"archive/zip"
	"context"
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
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// Restore (S-24.3) is an import, not a rollback: what an archive holds goes
// into a NEW collection — a new folder for files — so nothing made after the
// copy is overwritten. It is checked on the same screen before anything is
// written: the archive is opened, its manifest read and each collection
// previewed. Three sources: a run of a job, a file on a WebDAV storage, a
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
	manifest backup.Manifest
	mu       sync.Mutex
	gone     bool // cancelled: the file is deleted
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
// what it holds and where each collection goes.
func (s *Server) restoreBlock(sess *session.Session, blob sealed.Blob) view.RestoreBlock {
	b := view.RestoreBlock{Note: "Restoring creates: the records go into a new collection, never over what is there now. The archive is opened and shown before anything is written."}
	var runs, stores []view.Option
	byJob := s.users.BackupRuns(sess.AccountID())
	tf, now := s.timeFor(sess), time.Now()
	for _, job := range blob.BackupJobs {
		for _, r := range byJob[job.ID] {
			if r.File != "" && r.Verified {
				runs = append(runs, view.Option{Value: job.ID + "|" + r.ID, Label: text(job.Name + " · " + string(tf.formatWhen(r.Started, true, now)))})
			}
		}
	}
	for _, conn := range blob.Connections {
		for _, col := range blob.CollectionsOf(conn.ID) {
			if col.Has(view.KindFile) && !col.Gone {
				stores = append(stores, view.Option{Value: selValue(conn.ID, col.Path), Label: text(string(stamp(serverHost(conn.Server), collectionLabel(col))))})
			}
		}
	}
	b.Source = s.plainForm("/app/settings/backup?op=restore-open", "POST",
		csrfField(sess),
		view.FormRow{Name: "source", Label: "Which archive", Kind: view.InputRadio, Options: []view.Option{
			{Value: "run", Label: "From a job's history", Selected: len(runs) > 0},
			{Value: "webdav", Label: "From a storage (WebDAV)", Selected: len(runs) == 0},
			{Value: "upload", Label: "From this computer"},
		}},
		view.FormRow{Name: "run", Label: "Copy", Kind: view.InputSelect, Options: runs},
		view.FormRow{Name: "store", Label: "Storage", Kind: view.InputSelect, Options: stores},
		view.FormRow{Name: "file", Label: "File in it", Kind: view.InputText, Hint: "The path of the archive under the storage's root, such as backups/carrel-work-2026-08-16T031000Z.zip.age."},
		view.FormRow{Name: "password", Label: "Archive password", Kind: view.InputPassword, Hint: "The job's own password. An archive that is not encrypted needs none."},
	)
	b.Source.Actions = []view.Action{
		{ID: "restore.open", Label: "Open the archive", Href: s.postQuery("/app/settings/backup", url.Values{"op": {"restore-open"}}), Method: "POST", Enabled: true},
		{ID: "restore.upload", Label: "Open a file from this computer", Icon: "i-up", Href: s.postQuery("/app/settings/backup", url.Values{"op": {"restore-open"}, "source": {"upload"}}), Method: "POST", Enabled: true},
	}
	if t := restoreOf(sess); t != nil {
		s.fillRestore(sess, blob, t, &b)
	}
	return b
}

// fillRestore is step two: every collection of the archive with what it
// holds, and where it can go — a server that makes collections of its kind,
// or a file collection that takes writes.
func (s *Server) fillRestore(sess *session.Session, blob sealed.Blob, t *restoreTask, b *view.RestoreBlock) {
	zr, f, err := t.open()
	if err != nil {
		return
	}
	defer f.Close()
	for _, e := range t.manifest.Collections {
		table := view.ImportTable{Rubric: text(fmt.Sprintf("%s · %s · from %s", e.Name, e.Kind, e.Connection)), Count: e.Count}
		switch e.Kind {
		case view.KindFile:
			table.Columns = []view.Text{"File"}
			for _, zf := range zr.File {
				if name, ok := strings.CutPrefix(zf.Name, e.Dir+"files/"); ok && len(table.Rows) < 10 {
					table.Rows = append(table.Rows, []view.Text{text(name)})
				}
			}
		default:
			table.Columns = []view.Text{"Record", "Kind"}
			objs, err := s.archiveRecords(zr, e)
			if err != nil {
				table.Rubric += text(" · cannot be restored: " + err.Error())
				objs = nil
			}
			for _, o := range objs {
				if len(table.Rows) >= 10 {
					break
				}
				k := view.KindContact
				if o.IsCalendar() {
					k = cal.Kind(o)
				}
				table.Rows = append(table.Rows, []view.Text{text(recordTitle(k, o)), text(string(k))})
			}
		}
		if e.Error != "" {
			table.Rubric += text(" · not whole in this archive: " + e.Error)
		}
		b.Preview = append(b.Preview, table)
		picker := view.TargetPicker{}
		for _, conn := range blob.Connections {
			if e.Kind == view.KindFile {
				for _, col := range blob.CollectionsOf(conn.ID) {
					if col.Has(view.KindFile) && !col.Gone && !col.ReadOnly {
						picker.Options = append(picker.Options, view.TargetOption{ID: selValue(conn.ID, col.Path), Label: text(collectionLabel(col)), Stamp: stamp(serverHost(conn.Server), collectionLabel(col))})
					}
				}
				continue
			}
			opt := view.TargetOption{ID: conn.ID, Label: text(serverHost(conn.Server)), Stamp: stamp(serverHost(conn.Server), "")}
			if homeFor(conn, e.Kind) == "" {
				opt.ReadOnly, opt.Reason = true, "Makes no collections of this kind"
			}
			picker.Options = append(picker.Options, opt)
		}
		b.Targets = append(b.Targets, picker)
	}
	b.Actions = append(b.Actions,
		view.Action{ID: "restore.run", Label: "Restore into new collections", Href: s.postQuery("/app/settings/backup", url.Values{"op": {"restore-run"}}), Method: "POST", Enabled: true},
		view.Action{ID: "restore.cancel", Label: "Close the archive", Href: s.postQuery("/app/settings/backup", url.Values{"op": {"restore-cancel"}}), Method: "POST", Enabled: true},
	)
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
			objs, err = card.Split(body)
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
		fields, src = up.fields, up.next
	} else {
		var err error
		src, closer, err = s.restoreSource(r.Context(), sess, blob, source, fields)
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
	t := &restoreTask{path: f.Name(), password: fields.Get("password")}
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

func restoreFailure(body string) view.Notice {
	return view.Notice{Role: view.NoticeAlert, Title: "The archive could not be opened", Body: text(body)}
}

// restoreSource opens the archive a form names: a run of a job, or a file on
// a storage of the catalog.
func (s *Server) restoreSource(ctx context.Context, sess *session.Session, blob sealed.Blob, source string, f url.Values) (io.Reader, io.Closer, error) {
	switch source {
	case "run":
		jobID, runID, _ := strings.Cut(f.Get("run"), "|")
		job, ok := blob.BackupJob(jobID)
		if !ok {
			return nil, nil, errors.New("choose a copy")
		}
		for _, r := range s.users.BackupRuns(sess.AccountID())[jobID] {
			if r.ID == runID && r.File != "" && r.Verified {
				dest, err := s.backupDest(blob, job)
				if err != nil {
					return nil, nil, err
				}
				body, err := dest.Open(ctx, r.File)
				return body, body, err
			}
		}
		return nil, nil, errors.New("choose a copy")
	case "webdav":
		acc, colPath, _ := strings.Cut(f.Get("store"), "|")
		col, ok := blob.Collection(acc, colPath)
		conn, hasConn := blob.Connection(acc)
		segs, okPath := relSegs(f.Get("file"))
		if !ok || !hasConn || !col.Has(view.KindFile) || !okPath || len(segs) == 0 {
			return nil, nil, errors.New("choose a storage and the archive's path in it")
		}
		files, err := s.filesOf(conn)
		if err != nil {
			return nil, nil, err
		}
		p, err := file.Join(fileRoot(conn, col), segs...)
		if err != nil {
			return nil, nil, err
		}
		d, err := files.Open(ctx, p, nil)
		if err != nil {
			return nil, nil, err
		}
		return d.Body, d.Body, nil
	}
	return nil, nil, errors.New("choose where the archive is")
}

// runRestore is step three: every collection with a target goes into a new
// collection there — records created one by one, files uploaded under a new
// folder — and the page says what went where.
func (s *Server) runRestore(w http.ResponseWriter, r *http.Request, sess *session.Session, key [32]byte, blob sealed.Blob) {
	t := restoreOf(sess)
	if t == nil {
		s.writePage(w, r, http.StatusOK, s.backupPage(sess, nil, nil, restoreFailure("There is no archive open — the session may have ended. Open it again.")))
		return
	}
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
		target := r.PostFormValue("target." + strconv.Itoa(i))
		if target == "" {
			continue
		}
		var line string
		var err error
		if e.Kind == view.KindFile {
			line, err = s.restoreFiles(ctx, blob, zr, e, target, day)
		} else {
			line, err = s.restoreRecords(ctx, sess, key, blob, zr, e, target, day)
		}
		if err != nil {
			failed = true
			line = e.Name + ": " + err.Error()
		}
		lines = append(lines, text(line))
	}
	if len(lines) == 0 {
		s.writePage(w, r, http.StatusOK, s.backupPage(sess, nil, nil, view.Notice{Role: view.NoticeInfo, Title: "Nothing was restored", Body: "Choose where at least one collection goes."}))
		return
	}
	closeOnce()
	endRestore(sess)
	n := view.Notice{Role: view.NoticeInfo, Title: "Restored into new collections", Body: "Nothing that was there before was touched.",
		Details: &view.Diagnostic{Summary: "What went where", Lines: lines}}
	if failed {
		n.Role, n.Title = view.NoticeAlert, "Restored in part"
	}
	s.writePage(w, r, http.StatusOK, s.backupPage(sess, nil, nil, n))
}

// restoreRecords makes a new collection of the archived one's kinds on the
// connection and creates its records there.
func (s *Server) restoreRecords(ctx context.Context, sess *session.Session, key [32]byte, blob sealed.Blob, zr *zip.Reader, e backup.Entry, accountID, day string) (string, error) {
	conn, ok := blob.Connection(accountID)
	if !ok || conn.Feed {
		return "", errors.New("the chosen server is not in the catalog")
	}
	objs, err := s.archiveRecords(zr, e)
	if err != nil {
		return "", err
	}
	kinds := e.Kinds
	if len(kinds) == 0 {
		kinds = []view.Kind{e.Kind}
	}
	col, err := s.newCollectionOn(ctx, conn, kinds, e.Name+" (restored "+day+")")
	if err != nil {
		return "", err
	}
	if err := s.users.AddCollection(sess.AccountID(), key, conn.ID, col); err != nil {
		return "", err
	}
	made, refused := 0, 0
	for _, o := range objs {
		k := view.KindContact
		uid := ""
		if o.IsCalendar() {
			k, uid = cal.Kind(o), cal.UID(o)
		} else {
			uid = card.UID(o)
		}
		if !col.Has(k) {
			refused++
			continue
		}
		dest, err := s.newCollectionCtx(sess, recordKinds[segmentOf(k)], conn, col)
		if err == nil {
			err = s.createImported(ctx, dest, o, uid)
		}
		if err != nil {
			refused++
			continue
		}
		made++
	}
	line := fmt.Sprintf("%s → %s · %s: %s created", e.Name, serverHost(conn.Server), col.Name, plural(made, "record"))
	if refused > 0 {
		line += fmt.Sprintf(", %d refused", refused)
		return line, errors.New(line)
	}
	return line, nil
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

// restoreFiles uploads the archived files under a new folder of a file
// collection; a name there is never overwritten.
func (s *Server) restoreFiles(ctx context.Context, blob sealed.Blob, zr *zip.Reader, e backup.Entry, target, day string) (string, error) {
	acc, colPath, _ := strings.Cut(target, "|")
	col, ok := blob.Collection(acc, colPath)
	conn, hasConn := blob.Connection(acc)
	if !ok || !hasConn || col.ReadOnly || col.Feed || !col.Has(view.KindFile) {
		return "", errors.New("the chosen storage does not take writes")
	}
	files, err := s.filesOf(conn)
	if err != nil {
		return "", err
	}
	// The folder's name comes from the manifest, and a manifest is part of
	// the archive: a name that is not one segment is made one (S-29.4).
	name := e.Name
	if file.CheckName(name) != nil {
		name = backup.Slug(name)
	}
	base := strings.TrimSuffix(fileRoot(conn, col), "/") + "/" + name + " (restored " + day + ")/"
	if _, err := files.Stat(ctx, base); err == nil {
		return "", errors.New(base + " is already there")
	}
	made := map[string]bool{}
	mkdirs := func(p string) error {
		var chain []string
		for dir := path.Dir(strings.TrimSuffix(p, "/")) + "/"; strings.HasPrefix(dir, base) && !made[dir]; dir = path.Dir(strings.TrimSuffix(dir, "/")) + "/" {
			chain = append([]string{dir}, chain...)
		}
		if !made[base] {
			chain = append([]string{base}, chain...)
		}
		for _, d := range chain {
			if made[d] {
				continue
			}
			if err := files.MkCol(ctx, d); err != nil {
				return err
			}
			made[d] = true
		}
		return nil
	}
	count := 0
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
			return "", err
		}
		rc, err := zf.Open()
		if err != nil {
			return "", err
		}
		_, err = files.UploadCreate(ctx, p, rc)
		rc.Close()
		if err != nil {
			return "", fmt.Errorf("%s: %w", rel, err)
		}
		count++
	}
	return fmt.Sprintf("%s → %s · %s: %s", e.Name, serverHost(conn.Server), strings.TrimPrefix(base, fileRoot(conn, col)), plural(count, "file")), nil
}
