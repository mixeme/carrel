package web_test

import (
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// The head of Backup (s-backup-1, s-backup-6): New job and Restore, both
// there with no job at all — an archive from another instance is restored
// without one. With no job the page says so under the title and in its
// empty state, in the frame's words; with jobs it counts them.
func TestBackupHeadAndEmptyState(t *testing.T) {
	frame := mockupFrame(t, "s-backup-6")
	empty := regexp.MustCompile(`(?s)<div class="tc-empty">\s*(.*?)\s*<span class="tc-dim--t12-5">(.*?)</span>`).FindStringSubmatch(frame)
	if empty == nil {
		t.Fatal("s-backup-6 draws no empty state: read the frame again")
	}
	f := backupFixture(t)
	page := decode[view.BackupPage](t, f.h.do(get("/app/settings/backup", f.sess)))
	if saidRuns(page.Subtitle) != "No jobs yet" {
		t.Errorf("subtitle = %q", saidRuns(page.Subtitle))
	}
	if got := saidRuns(page.Empty); got != framePieces(empty[1]) {
		t.Errorf("empty = %q, the frame %q", got, framePieces(empty[1]))
	}
	if got := saidRuns(page.EmptyNote); got != framePieces(empty[2]) {
		t.Errorf("empty note = %q, the frame %q", got, framePieces(empty[2]))
	}
	if len(page.Actions) != 2 || page.Actions[0].Label != "New job" || !page.Actions[0].Primary ||
		page.Actions[1].Label != "Restore" || page.Actions[1].Primary {
		t.Fatalf("head = %+v", page.Actions)
	}
	for _, a := range page.Actions {
		if a.Method != "GET" || !a.Enabled || a.Href == "" {
			t.Errorf("%s: %+v", a.Label, a)
		}
	}
	if page.Restore.Open {
		t.Error("the restore is open on the plain page")
	}

	job := decode[view.BackupPage](t, f.h.do(get(page.Actions[0].Href, f.sess)))
	if job.Title != "New job" || !hasAction(job.Add.Actions, "backup.create") || !hasAction(job.Add.Actions, "backup.cancel") {
		t.Errorf("New job: title %q, actions %+v", job.Title, job.Add.Actions)
	}
	restore := decode[view.BackupPage](t, f.h.do(get(page.Actions[1].Href, f.sess)))
	if !restore.Restore.Open || !hasAction(restore.Restore.Source.Actions, "restore.open") {
		t.Errorf("Restore: open %v, %+v", restore.Restore.Open, restore.Restore.Source.Actions)
	}

	saveJob(t, f, jobForm(f, sealed.BackupToWebDAV, "8", "secret-pass"))
	page = decode[view.BackupPage](t, f.h.do(get("/app/settings/backup", f.sess)))
	if !strings.HasPrefix(saidRuns(page.Subtitle), "One job · ") || len(page.Empty) != 0 || len(page.Actions) != 2 {
		t.Errorf("with a job: subtitle %q, empty %q, head %+v", saidRuns(page.Subtitle), saidRuns(page.Empty), page.Actions)
	}
}
