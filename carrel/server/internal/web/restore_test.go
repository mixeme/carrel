package web_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

func restoreTemps(t *testing.T) []string {
	t.Helper()
	// Any file, whatever its name: nothing of an archive is kept on disk
	// (S-24.3). The test's own directories (t.TempDir) are not files.
	entries, _ := os.ReadDir(os.TempDir())
	var found []string
	for _, e := range entries {
		if !e.IsDir() {
			found = append(found, filepath.Join(os.TempDir(), e.Name()))
		}
	}
	return found
}

// ownTemp points os.TempDir at a directory of the test's own (TMPDIR on
// Linux, TMP and TEMP on Windows): a test that counts the temporary files
// the server leaves counts its own, not another run's at the same time.
func ownTemp(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	for _, k := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(k, dir)
	}
	if os.TempDir() != dir {
		t.Fatalf("os.TempDir() = %q, not the directory set", os.TempDir())
	}
}

// S-24.3: restoring is an import, and New collection in a row's «Restore
// into» makes one for it — a new folder for files. The archive is opened
// with its password and matched before anything is written; a wrong
// password says so; what was there before is not touched; the archive kept
// for the steps is deleted when the restore ends.
func TestRestoreFromHistoryIntoNewCollections(t *testing.T) {
	ownTemp(t)
	before := len(restoreTemps(t))
	f := backupFixture(t)
	job := saveJob(t, f, jobForm(f, sealed.BackupToWebDAV, "8", "archive-pw"))
	run := runJob(t, f, job)
	if run.State != "done" {
		t.Fatalf("run = %+v", run)
	}
	open := func(pw string) *httptest.ResponseRecorder {
		return f.h.do(postForm("/app/settings/backup/restore", f.sess, url.Values{
			"source": {"run"}, "run": {job.ID + "|" + run.ID}, "password": {pw},
		}))
	}
	wrong := decode[view.BackupPage](t, open("nope"))
	if plainRuns(wrong.Restore.Refusal) != "The password does not open this archive." || wrong.Restore.Match != nil {
		t.Fatalf("wrong password = %+v", wrong.Restore.Refusal)
	}
	made := f.srv.CountRequests("MKCOL", "") + f.srv.CountRequests("MKCALENDAR", "")
	if rec := open("archive-pw"); rec.Code != http.StatusSeeOther {
		t.Fatalf("open: %d %s", rec.Code, rec.Body.String())
	}
	page := decode[view.BackupPage](t, f.h.do(get("/app/settings/backup", f.sess)))
	if m := page.Restore.Match; m == nil || len(m.Rows) != 3 || m.Rows[0].Count != 1 {
		t.Fatalf("matching = %+v", page.Restore.Match)
	}
	if n := f.srv.CountRequests("MKCOL", "") + f.srv.CountRequests("MKCALENDAR", ""); n != made {
		t.Errorf("opening and previewing made %d collections", n-made)
	}
	rep := restoreRun(t, f, "/app/settings/backup/restore/match?op=restore", url.Values{
		"into.0": {"new"}, "into.1": {"new"}, "into.2": {"new"}, "pick.0": {"on"}, "pick.1": {"on"}, "pick.2": {"on"},
	})
	key, _ := f.sess.DataKey()
	blob, _ := f.h.users.Catalog(f.sess.AccountID(), key)
	var book, cal sealed.Collection
	for _, c := range blob.Collections {
		switch {
		case strings.HasPrefix(c.Name, "People (restored"):
			book = c
		case strings.HasPrefix(c.Name, "Work (restored"):
			cal = c
		}
	}
	if book.Path == "" || cal.Path == "" || book.Path == searchBook || cal.Path == searchWork {
		t.Fatalf("new collections = %+v", blob.Collections)
	}
	if n := len(f.srv.Paths(book.Path)); n != 1 {
		t.Errorf("restored book holds %d", n)
	}
	if n := len(f.srv.Paths(cal.Path)); n != 2 {
		t.Errorf("restored calendar holds %d", n)
	}
	if n := len(f.srv.Paths(searchBook)); n != 1 {
		t.Errorf("the original book changed: %d", n)
	}
	lines := reportLines(rep)
	day := time.Now().In(time.UTC).Format("2006-01-02")
	if body, ok := f.srv.File(filesDocs + "Docs (restored " + day + ")/share/readme.txt"); !ok || body != "hello" {
		if body, ok = f.srv.File(filesDocs + "Docs (restored " + time.Now().Format("2006-01-02") + ")/share/readme.txt"); !ok || body != "hello" {
			t.Errorf("restored file not found: %v", lines)
		}
	}
	if n := len(restoreTemps(t)); n != before {
		t.Errorf("the archive kept for the restore is still on disk (%d files)", n-before)
	}
}

// An archive brought from this computer: the file streams in with the form,
// and closing the archive deletes it.
func TestRestoreFromThisComputer(t *testing.T) {
	ownTemp(t)
	before := len(restoreTemps(t))
	f := backupFixture(t)
	job := saveJob(t, f, jobForm(f, sealed.BackupToDownload, "8", ""))
	archive := f.h.do(get("/app/settings/backup?op=download&job="+job.ID, f.sess)).Body.Bytes()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("_csrf", f.sess.CSRFToken())
	fw, _ := mw.CreateFormFile("archive", "carrel.zip")
	_, _ = fw.Write(archive)
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/app/settings/backup/restore?source=upload", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: f.sess.ID()})
	if rec := f.h.do(req); rec.Code != http.StatusSeeOther {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	page := decode[view.BackupPage](t, f.h.do(get("/app/settings/backup/restore", f.sess)))
	if page.Restore.Match == nil || len(page.Restore.Match.Rows) != 3 {
		t.Fatalf("matching = %+v", page.Restore.Match)
	}
	// Held in the session's memory, not on Carrel's disk (S-24.3, the
	// owner's decision of 19.10.2026).
	if n := len(restoreTemps(t)); n != before {
		t.Errorf("the uploaded archive is on disk (%d)", n-before)
	}
	if rec := f.h.do(postForm("/app/settings/backup/restore/match?op=cancel", f.sess, url.Values{})); rec.Code != http.StatusSeeOther {
		t.Fatalf("cancel: %d", rec.Code)
	}
	if n := len(restoreTemps(t)); n != before {
		t.Errorf("closing left the archive on disk (%d)", n-before)
	}
	if again := decode[view.BackupPage](t, f.h.do(get("/app/settings/backup/restore", f.sess))); again.Restore.Match != nil {
		t.Error("a closed archive is still previewed")
	}
}

// restoreRun presses Restore and follows it to the report (s-backup-18).
func restoreRun(t *testing.T, f davFixture, href string, form url.Values) *view.RestoreReport {
	t.Helper()
	rec := f.h.do(postForm(href, f.sess, form))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/app/settings/backup/restore/report" {
		t.Fatalf("restore: %d %s %s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	page := decode[view.BackupPage](t, f.h.do(get("/app/settings/backup/restore/report", f.sess)))
	if page.Restore.Report == nil || page.Restore.Report.Note != "Nothing on the server was deleted or overwritten." {
		t.Fatalf("report = %+v", page.Restore.Report)
	}
	return page.Restore.Report
}

// reportLines are the report's rows as lines: name, where to, what happened.
func reportLines(rep *view.RestoreReport) string {
	var out string
	for _, r := range rep.Rows {
		out += string(r.Name) + " → " + string(r.Into) + ": " + plainRuns(r.Happened) + "\n"
	}
	return out
}
