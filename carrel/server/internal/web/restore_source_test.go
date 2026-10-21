package web_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// S-24.3, the owner's decision of 19.10.2026: an archive is read where it
// lies, a piece at a time — every GET of it a range —, and nothing of it is
// on Carrel's disk.
func TestRestoreReadsTheArchiveWhereItLies(t *testing.T) {
	ownTemp(t)
	before := len(restoreTemps(t))
	f := backupFixture(t)
	job := saveJob(t, f, jobForm(f, sealed.BackupToWebDAV, "8", "archive-pw"))
	run := runJob(t, f, job)
	asked := len(f.srv.Ranges())
	if rec := f.h.do(postForm("/app/settings/backup/restore", f.sess, url.Values{
		"source": {"run"}, "run": {job.ID + "|" + run.ID}, "password": {"archive-pw"},
	})); rec.Code != http.StatusSeeOther {
		t.Fatalf("open: %d %s", rec.Code, rec.Body.String())
	}
	page := decode[view.BackupPage](t, f.h.do(get("/app/settings/backup/restore", f.sess)))
	if page.Restore.Match == nil || len(page.Restore.Match.Rows) != 3 {
		t.Fatalf("matching = %+v", page.Restore.Match)
	}
	got := f.srv.Ranges()[asked:]
	if len(got) == 0 {
		t.Fatal("the archive was not read from its server")
	}
	for _, r := range got {
		if !strings.HasPrefix(r, "bytes=") {
			t.Errorf("a GET of the archive without a range: %q", got)
			break
		}
	}
	if n := len(restoreTemps(t)); n != before {
		t.Errorf("the archive is on Carrel's disk (%d files)", n-before)
	}
}

// A server that answers a range with the whole file is named in the
// refusal: the archive cannot be opened on it without fetching all of it.
func TestRestoreRefusesAServerWithoutRanges(t *testing.T) {
	f := backupFixture(t)
	job := saveJob(t, f, jobForm(f, sealed.BackupToWebDAV, "8", "archive-pw"))
	run := runJob(t, f, job)
	f.srv.FilesIgnoreRange = true
	page := decode[view.BackupPage](t, f.h.do(postForm("/app/settings/backup/restore", f.sess, url.Values{
		"source": {"run"}, "run": {job.ID + "|" + run.ID}, "password": {"archive-pw"},
	})))
	if !strings.Contains(plainRuns(page.Restore.Refusal), "ignores HTTP Range") || page.Restore.Match != nil {
		t.Errorf("refusal = %+v", page.Restore.Refusal)
	}
}

// uploadArchiveWith sends an archive from this computer without a script, the
// form's fields in front of the file.
func uploadArchiveWith(t *testing.T, f davFixture, archive []byte, fields url.Values) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("_csrf", f.sess.CSRFToken())
	for k, vs := range fields {
		for _, v := range vs {
			_ = mw.WriteField(k, v)
		}
	}
	fw, _ := mw.CreateFormFile("file", "carrel.zip")
	_, _ = fw.Write(archive)
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/app/settings/backup/restore?from=file", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: f.sess.ID()})
	return f.h.do(req)
}

// A browser without a script sends the whole archive, once: over the
// ceiling of an import it is refused, saying why, unless the step names a
// folder of a storage — then the same stream goes there and the archive is
// read from it.
func TestRestoreOfALargeUploadGoesIntoAStorage(t *testing.T) {
	ownTemp(t)
	before := len(restoreTemps(t))
	f := backupFixture(t)
	job := saveJob(t, f, jobForm(f, sealed.BackupToDownload, "8", ""))
	archive := f.h.do(get("/app/settings/backup?op=download&job="+job.ID, f.sess)).Body.Bytes()
	f.h.cfg.ImportMaxBytes = int64(len(archive) / 2)

	refused := decode[view.BackupPage](t, uploadArchiveWith(t, f, archive, nil))
	// The step again, as chosen, why at the top of its card and the way
	// named (s-backup-16).
	if why := refused.Restore.Refusal; !strings.Contains(plainRuns(why), "larger than") || !hasRun(why, "From a server", view.RunStrong) ||
		refused.Restore.Match != nil || !refused.Restore.From[2].On {
		t.Fatalf("over the ceiling, no folder: %+v", refused.Restore)
	}
	if folder := formRow(refused.Restore.Source, "folder"); folder == nil || !folder.ReadOnly {
		t.Errorf("the folder without a storage = %+v", folder)
	}

	if rec := uploadArchiveWith(t, f, archive, url.Values{"store": {sel(f, filesDocs)}, "folder": {"share"}}); rec.Code != http.StatusSeeOther {
		t.Fatalf("into the storage: %d %s", rec.Code, rec.Body.String())
	}
	stored, ok := f.srv.File(filesDocs + "share/carrel.zip")
	if !ok || stored != string(archive) {
		t.Fatalf("the archive in the storage: %v, %d of %d bytes", ok, len(stored), len(archive))
	}
	page := decode[view.BackupPage](t, f.h.do(get("/app/settings/backup/restore", f.sess)))
	if page.Restore.Match == nil || len(page.Restore.Match.Rows) != 3 {
		t.Fatalf("matching = %+v", page.Restore.Match)
	}
	// Put there for this restore, it goes after it (the owner, 19.10.2026).
	restoreRun(t, f, "/app/settings/backup/restore/match?op=restore", matchForm(*page.Restore.Match))
	if _, ok := f.srv.File(filesDocs + "share/carrel.zip"); ok {
		t.Error("the archive put into the storage outlived the restore")
	}
	if n := len(restoreTemps(t)); n != before {
		t.Errorf("the archive is on Carrel's disk (%d files)", n-before)
	}
}
