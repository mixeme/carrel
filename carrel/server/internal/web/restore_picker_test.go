package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// Typecase's note 99 (s-backup-17): «From a server» — the collection and
// the archive's path in it; Browse opens the tree with files, Choose takes
// the archive back to the step, and the archive is opened where it lies.
func TestRestoreBrowsesTheArchiveOnAServer(t *testing.T) {
	f := backupFixture(t)
	job := saveJob(t, f, jobForm(f, sealed.BackupToWebDAV, "8", "archive-pw"))
	run := runJob(t, f, job)
	if run.State != "done" || run.File == "" {
		t.Fatalf("run = %+v", run)
	}
	step := decode[view.BackupPage](t, f.h.do(get("/app/settings/backup/restore?from=server", f.sess))).Restore
	path := formRow(step.Source, "path")
	if path == nil || path.Extra == nil || path.Extra.ID != "restore.browse" || formRow(step.Source, "store") == nil {
		t.Fatalf("server step = %+v", step.Source)
	}
	if step.Source.Sets[1].Legend != "From a server" || step.Source.Sets[2].Legend != "Archive password" {
		t.Errorf("sets = %+v", step.Source.Sets)
	}

	rec := f.h.do(postForm(path.Extra.Href, f.sess, url.Values{"store": {sel(f, filesDocs)}, "path": {"/backups/work/"}}))
	if rec.Code != http.StatusSeeOther || !strings.HasPrefix(rec.Header().Get("Location"), "/app/files/picker?") {
		t.Fatalf("browse: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	loc, _ := url.Parse(rec.Header().Get("Location"))
	to := loc.Query().Get("to")
	sheet := decode[view.FilePickerSheet](t, f.h.do(get(rec.Header().Get("Location"), f.sess)))
	if choose := actionByID(sheet.Actions, "files.picker.restore"); choose == nil || choose.Enabled {
		t.Fatalf("choose before a file = %+v", choose)
	}
	q := loc.Query()
	q.Set("to", to+url.PathEscape(run.File))
	sheet = decode[view.FilePickerSheet](t, f.h.do(get("/app/files/picker?"+q.Encode(), f.sess)))
	choose := actionByID(sheet.Actions, "files.picker.restore")
	if choose == nil || !choose.Enabled {
		t.Fatalf("choose = %+v", choose)
	}
	back := decode[view.BackupPage](t, f.h.do(get(choose.Href, f.sess))).Restore
	if p := formRow(back.Source, "path"); p == nil || !strings.HasSuffix(string(p.Value), run.File) {
		t.Fatalf("path after Choose = %+v", p)
	}
	chosen := ""
	for _, o := range formRow(back.Source, "store").Options {
		if o.Selected {
			chosen = o.Value
		}
	}
	if chosen != sel(f, filesDocs) {
		t.Errorf("store after Choose = %q", chosen)
	}
	open := f.h.do(postForm(back.Source.Action, f.sess, url.Values{"source": {"webdav"}, "store": {chosen},
		"path": {string(formRow(back.Source, "path").Value)}, "password": {"archive-pw"}}))
	if open.Code != http.StatusSeeOther {
		t.Fatalf("open: %d %s", open.Code, open.Body.String())
	}
	if m := decode[view.BackupPage](t, f.h.do(get("/app/settings/backup/restore", f.sess))).Restore.Match; m == nil || len(m.Rows) == 0 {
		t.Fatalf("matching = %+v", m)
	}
}
