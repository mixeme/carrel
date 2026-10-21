package web_test

import (
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// Нс74 of the scan of 15.10.2026 (s-backup-7; Typecase's note 54): «Where
// the archive is» is three addresses of the same step — From a job, From a
// server, From this computer —, and the step holds the fields of the one
// chosen alone: a job's copies one row a job (its name; its day, size and
// collections), a server's storage and path, this computer's file; then the
// archive's password, with the frame's words.
func TestRestoreSourceIsTheStep(t *testing.T) {
	frame := mockupFrame(t, "s-backup-7")
	hint := regexp.MustCompile(`(?s)From this computer</a></span>\s*<span class="tc-field__hint">(.*?)</span>`).FindStringSubmatch(frame)
	pw := regexp.MustCompile(`(?s)name="password"[^>]*>(?:</div>)?\s*<span class="[^"]*">(.*?)</span>`).FindStringSubmatch(frame)
	if hint == nil || pw == nil {
		t.Fatal("s-backup-7 reads otherwise: read the frame again")
	}
	f := backupFixture(t)
	job := saveJob(t, f, jobForm(f, sealed.BackupToWebDAV, "8", "archive-pw"))
	if run := runJob(t, f, job); run.State != "done" {
		t.Fatalf("run = %+v", run)
	}
	step := func(q string) view.RestoreBlock {
		t.Helper()
		return decode[view.BackupPage](t, f.h.do(get("/app/settings/backup/restore?restore=1"+q, f.sess))).Restore
	}

	// A job's copy is there: the step opens on it.
	b := step("")
	if len(b.From) != 3 || !b.From[0].On || b.From[0].Label != "From a job" || !strings.Contains(b.From[1].Href, "from=server") ||
		string(b.FromHint) != frameText(hint[1]) {
		t.Fatalf("from = %+v, hint %q", b.From, b.FromHint)
	}
	run := formRow(b.Source, "run")
	if run == nil || run.Kind != view.InputRadio || len(run.Options) != 1 || string(run.Options[0].Label) != job.Name ||
		len(run.Options[0].Columns) != 3 || !strings.HasSuffix(string(run.Options[0].Columns[2]), " collections") && !strings.HasSuffix(string(run.Options[0].Columns[2]), " collection") {
		t.Errorf("a job's copies = %+v", run)
	}
	if formRow(b.Source, "store") != nil || formRow(b.Source, "path") != nil {
		t.Error("the job's step holds a server's fields")
	}
	if p := formRow(b.Source, "password"); p == nil || string(p.Hint) != frameText(pw[1]) {
		t.Errorf("password = %+v, the frame %q", p, frameText(pw[1]))
	}

	// From a server: its storage and path, no copies.
	b = step("&from=server")
	if !b.From[1].On || formRow(b.Source, "store") == nil || formRow(b.Source, "path") == nil || formRow(b.Source, "run") != nil {
		t.Errorf("server step = %+v", b.Source)
	}

	// From this computer: the file goes with the form, and a storage and a
	// folder for an archive over the ceiling of an import (S-24.3).
	b = step("&from=file")
	if !b.From[2].On || formRow(b.Source, "run") != nil || formRow(b.Source, "store") == nil || formRow(b.Source, "folder") == nil {
		t.Errorf("file step = %+v", b.Source)
	}
	if file := formRow(b.Source, "file"); file == nil || file.Kind != view.InputFile || file.Accept != ".zip" {
		t.Errorf("the archive's field = %+v", file)
	}
}
