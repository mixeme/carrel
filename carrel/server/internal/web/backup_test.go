package web_test

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/backup"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

// backupFixture: a book, a calendar and a file collection on one server —
// the three shapes a copy holds.
func backupFixture(t *testing.T) davFixture {
	t.Helper()
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work"})
	srv.Seed(searchBook+"ada.vcf", adaCard())
	srv.Seed(searchWork+"e1.ics", ics("BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260801T000000Z\nSUMMARY:Standup\nDTSTART:20260810T090000Z\nEND:VEVENT"))
	srv.Seed(searchWork+"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260801T000000Z\nSUMMARY:Pack\nEND:VTODO"))
	srv.SeedFolder(filesDocs + "share/")
	srv.SeedFile(filesDocs+"share/readme.txt", "hello")
	f := connectFixture(t, srv, searchBook, nil)
	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		b.Collections = append(b.Collections, sealed.Collection{AccountID: f.id, Path: filesDocs, Name: "Docs", Kinds: []view.Kind{view.KindFile}})
	})
	return f
}

func sel(f davFixture, path string) string { return f.id + "|" + path }

func saveJob(t *testing.T, f davFixture, form url.Values) sealed.BackupJob {
	t.Helper()
	rec := f.h.do(postForm("/app/settings/backup?op=save", f.sess, form))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	key, _ := f.sess.DataKey()
	blob, _ := f.h.users.Catalog(f.sess.AccountID(), key)
	return blob.BackupJobs[len(blob.BackupJobs)-1]
}

func jobForm(f davFixture, mode, keep, password string) url.Values {
	v := url.Values{
		"name": {"Рабочее"}, "src": {sel(f, searchBook), sel(f, searchWork), sel(f, filesDocs)},
		"mode": {mode}, "dest": {sel(f, filesDocs)}, "folder": {"backups/work"}, "keep": {keep},
	}
	if password != "" {
		v.Set("encrypted", "1")
		v.Set("password", password)
	}
	return v
}

// finished waits for the job's newest run to end.
func finished(t *testing.T, f davFixture, jobID string) store.BackupRun {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		runs := f.h.users.BackupRuns(f.sess.AccountID())[jobID]
		if len(runs) > 0 && runs[0].State != "running" {
			return runs[0]
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("the run did not finish")
	return store.BackupRun{}
}

func runJob(t *testing.T, f davFixture, job sealed.BackupJob) store.BackupRun {
	t.Helper()
	rec := f.h.do(postForm("/app/settings/backup?op=run&job="+job.ID, f.sess, url.Values{}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("run: %d %s", rec.Code, rec.Body.String())
	}
	return finished(t, f, job.ID)
}

// The editor refuses a job that cannot work: an encrypted archive needs its
// own password, a job needs a source and somewhere to write.
func TestBackupJobIsChecked(t *testing.T) {
	f := backupFixture(t)
	v := jobForm(f, sealed.BackupToWebDAV, "8", "")
	v.Set("encrypted", "1")
	v.Del("src")
	page := decode[view.BackupPage](t, f.h.do(postForm("/app/settings/backup?op=save", f.sess, v)))
	errs := map[string]bool{}
	for _, set := range page.Add.Sets {
		for _, r := range set.Rows {
			if r.Error != "" {
				errs[r.Name] = true
			}
		}
	}
	if !errs["password"] || !errs["src"] {
		t.Errorf("errors = %v", errs)
	}
	job := saveJob(t, f, jobForm(f, sealed.BackupToWebDAV, "8", "archive-pw"))
	if !job.Encrypted || job.Password != "archive-pw" || len(job.Sources) != 3 || strings.Join(job.DestFolder, "/") != "backups/work" {
		t.Errorf("job = %+v", job)
	}
}

// S-24: a run writes one sealed archive to the folder — made on first use —
// reads it back and compares; the archive holds every collection with its
// manifest; the history offers it for download, streamed from where it lies.
func TestBackupRunWritesVerifiesAndHandsBack(t *testing.T) {
	f := backupFixture(t)
	job := saveJob(t, f, jobForm(f, sealed.BackupToWebDAV, "8", "archive-pw"))
	run := runJob(t, f, job)
	if run.State != "done" || !run.Verified || run.Collections != 3 || run.Whole != 3 || !strings.HasSuffix(run.File, ".zip.age") {
		t.Fatalf("run = %+v", run)
	}
	stored, ok := f.srv.File(filesDocs + "backups/work/" + run.File)
	if !ok {
		t.Fatalf("no archive in the folder: %v", f.srv.Paths(filesDocs+"backups/work/"))
	}
	zr, m, err := backup.Open([]byte(stored), "archive-pw")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Collections) != 3 || m.Collections[1].Count != 2 || m.Collections[2].Count != 1 {
		t.Errorf("manifest = %+v", m)
	}
	names := map[string]bool{}
	for _, zf := range zr.File {
		names[zf.Name] = true
	}
	for _, want := range []string{"01-people/contacts.vcf", "02-work/calendar.ics", "03-docs/files/share/readme.txt", "manifest.json"} {
		if !names[want] {
			t.Errorf("archive lacks %s: %v", want, names)
		}
	}

	page := decode[view.BackupPage](t, f.h.do(get("/app/settings/backup", f.sess)))
	if len(page.Jobs) != 1 || page.Jobs[0].LastState != view.OpDone || len(page.Jobs[0].Runs) != 1 || page.Jobs[0].Runs[0].Download == nil {
		t.Fatalf("page = %+v", page.Jobs)
	}
	got := f.h.do(get(page.Jobs[0].Runs[0].Download.Href, f.sess))
	if got.Code != http.StatusOK || got.Body.String() != stored {
		t.Errorf("history download: %d, %d bytes", got.Code, got.Body.Len())
	}
}

// Rotation after the check: past keep, the oldest go — only once the new
// one is read back.
func TestBackupRotation(t *testing.T) {
	f := backupFixture(t)
	job := saveJob(t, f, jobForm(f, sealed.BackupToWebDAV, "1", ""))
	first := runJob(t, f, job)
	time.Sleep(1100 * time.Millisecond) // archives are named to the second
	second := runJob(t, f, job)
	if first.File == second.File || second.State != "done" {
		t.Fatalf("runs %+v %+v", first, second)
	}
	if _, ok := f.srv.File(filesDocs + "backups/work/" + first.File); ok {
		t.Error("the old copy is still there past keep=1")
	}
	stored, ok := f.srv.File(filesDocs + "backups/work/" + second.File)
	if !ok {
		t.Fatal("the new copy is gone")
	}
	if _, err := zip.NewReader(bytes.NewReader([]byte(stored)), int64(len(stored))); err != nil {
		t.Errorf("an unencrypted archive is a plain zip: %v", err)
	}
}

// A copy that leaves a collection out is partial: red on the screen, and it
// never pushes out a whole one.
func TestPartialCopyIsRedAndKeepsTheOld(t *testing.T) {
	f := backupFixture(t)
	job := saveJob(t, f, jobForm(f, sealed.BackupToWebDAV, "1", ""))
	whole := runJob(t, f, job)
	time.Sleep(1100 * time.Millisecond)
	f.srv.Refuse("REPORT", http.StatusServiceUnavailable, "down")
	part := runJob(t, f, job)
	if part.State != "partial" || part.Whole != 1 || part.Error == "" {
		t.Fatalf("partial run = %+v", part)
	}
	if _, ok := f.srv.File(filesDocs + "backups/work/" + whole.File); !ok {
		t.Error("a partial copy pushed out the whole one")
	}
	page := decode[view.BackupPage](t, f.h.do(get("/app/settings/backup", f.sess)))
	if page.Jobs[0].LastState != view.OpPartial || !hasNotice(page.Notices, "The last copy of «Рабочее» is not whole") {
		t.Errorf("page = %+v %+v", page.Jobs[0], page.Notices)
	}
}

// Download writes nothing to a server: the archive streams to the browser,
// the run is recorded, and nothing is left on the storage.
func TestBackupDownloadWritesNothing(t *testing.T) {
	f := backupFixture(t)
	job := saveJob(t, f, jobForm(f, sealed.BackupToDownload, "8", "pw"))
	before := f.srv.CountRequests(http.MethodPut, "")
	rec := f.h.do(get("/app/settings/backup?op=download&job="+job.ID, f.sess))
	if rec.Code != http.StatusOK || !backup.Encrypted(rec.Body.Bytes()) || !strings.Contains(rec.Header().Get("Content-Disposition"), ".zip.age") {
		t.Fatalf("download: %d %v", rec.Code, rec.Header())
	}
	if _, _, err := backup.Open(rec.Body.Bytes(), "pw"); err != nil {
		t.Errorf("the streamed archive does not open: %v", err)
	}
	if n := f.srv.CountRequests(http.MethodPut, "") - before; n != 0 {
		t.Errorf("a download wrote %d times", n)
	}
	runs := f.h.users.BackupRuns(f.sess.AccountID())[job.ID]
	if len(runs) != 1 || runs[0].Mode != sealed.BackupToDownload || runs[0].Verified || runs[0].State != "done" {
		t.Errorf("runs = %+v", runs)
	}
	if rec := f.h.do(postForm("/app/settings/backup?op=run&job="+job.ID, f.sess, url.Values{})); rec.Code != http.StatusNotFound {
		t.Errorf("a download job has no background run: %d", rec.Code)
	}
}

// At sign-in a job whose last whole copy is older than it allows runs.
func TestBackupRunsAtSignIn(t *testing.T) {
	f := backupFixture(t)
	v := jobForm(f, sealed.BackupToWebDAV, "8", "")
	v.Set("signin", "7")
	job := saveJob(t, f, v)
	rec := f.h.do(f.h.anonPostForm("/login", "/login", url.Values{"login": {"owner"}, "password": {"password123"}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("login: %d", rec.Code)
	}
	if run := finished(t, f, job.ID); run.Trigger != "sign-in" || run.State != "done" {
		t.Errorf("sign-in run = %+v", run)
	}
	rec = f.h.do(f.h.anonPostForm("/login", "/login", url.Values{"login": {"owner"}, "password": {"password123"}}))
	if runs := f.h.users.BackupRuns(f.sess.AccountID())[job.ID]; len(runs) != 1 {
		t.Errorf("a fresh copy ran again at the next sign-in: %d runs", len(runs))
	}
}
