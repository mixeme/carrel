package web_test

import (
	"html"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// from-typecase note 32, tasks 78 and 94 (the owner's decisions of
// 02.10.2026). The Gitea tab of the external trigger is a workflow of Gitea
// Actions with the password in the repository's secrets under the name the
// file gives, saved as the file the repository keeps (s-backup-10); the curl
// tab is the bare request (s-backup-11). «Regenerate» is «New password», its
// hint saying the old one stops working (s-backup-2, -3, -10, -11). With no
// job, Backup in the rail counts nothing and the rail says nothing of copies
// (s-backup-6).

var (
	frameDiagByID   = regexp.MustCompile(`(?s)<div class="tc-diag[^"]*" id="([^"]+)">(.*?)</div>`)
	frameDiagPlain  = regexp.MustCompile(`(?s)<div class="tc-diag[^"]*">(.*?)</div>`)
	frameSaveAs     = regexp.MustCompile(`data-tc-save="([^"]+)" data-tc-from="([^"]+)"`)
	frameNewPass    = regexp.MustCompile(`<button class="tc-btn" type="button" title="([^"]+)">New password</button>`)
	frameSecretName = regexp.MustCompile(`aria-label="Secret" value="([^"]+)"`)
	frameGiteaFile  = regexp.MustCompile(`as <span class="tc-mono">([^<]+\.yaml)</span>`)
)

func TestGiteaAndCurlTabsAreTheFrames(t *testing.T) {
	gitea, curl := mockupFrame(t, "s-backup-10"), mockupFrame(t, "s-backup-11")
	workflow := frameDiagByID.FindStringSubmatch(gitea)
	save := frameSaveAs.FindStringSubmatch(gitea)
	secret := frameSecretName.FindStringSubmatch(gitea)
	file := frameGiteaFile.FindStringSubmatch(gitea)
	line := frameDiagPlain.FindStringSubmatch(curl)
	if workflow == nil || save == nil || save[2] != workflow[1] || secret == nil || file == nil || line == nil ||
		!strings.HasSuffix(file[1], "/"+save[1]) {
		t.Fatalf("s-backup-10, -11: read the frames again")
	}
	var hints []string
	// The card left s-backup-2 with the form's tick (note 85 of Typecase).
	for _, id := range []string{"s-backup-3", "s-backup-10", "s-backup-11"} {
		m := frameNewPass.FindStringSubmatch(mockupFrame(t, id))
		if m == nil || strings.Contains(mockupFrame(t, id), ">Regenerate<") {
			t.Fatalf("%s draws no «New password» with its hint: read the frame again", id)
		}
		hints = append(hints, html.UnescapeString(m[1]))
	}
	if hints[0] != hints[1] || hints[1] != hints[2] {
		t.Fatalf("the hints of New password differ: %q", hints)
	}

	f := backupFixture(t)
	job := saveJob(t, f, jobForm(f, sealed.BackupToWebDAV, "8", "pw"))
	pass, tb := issueTrigger(t, f, job)
	base := strings.TrimSuffix(string(tb.Curl[strings.LastIndex(string(tb.Curl), " '")+2:]), "/api/backup/run?job="+job.ID+"'")
	// The frame's job is «personal» at carrel.example with its password;
	// here they are this job's.
	ours := func(s string) string {
		s = strings.ReplaceAll(s, "cbk_7f3a-91de-4402-lamp", pass)
		s = strings.ReplaceAll(s, "https://carrel.example", base)
		s = strings.ReplaceAll(s, "job=personal", "job="+job.ID)
		s = strings.ReplaceAll(s, "Carrel backup · personal", "Carrel backup · "+job.Name)
		return strings.ReplaceAll(s, "carrel-personal.yaml", "carrel-"+job.ID+".yaml")
	}
	if want := ours(html.UnescapeString(workflow[2])); string(tb.Gitea) != want {
		t.Errorf("the workflow:\n%s\nthe frame (s-backup-10):\n%s", tb.Gitea, want)
	}
	if string(tb.GiteaSecret) != secret[1] || string(tb.GiteaFile) != ours(file[1]) {
		t.Errorf("Gitea secret %q in %q, the frame %q in %q", tb.GiteaSecret, tb.GiteaFile, secret[1], ours(file[1]))
	}
	if want := ours(html.UnescapeString(line[1])); string(tb.Curl) != want {
		t.Errorf("curl = %q, the frame (s-backup-11) %q", tb.Curl, want)
	}
	if tb.Password.Reveal.Label != "New password" || string(tb.Password.Reveal.Hint) != hints[0] {
		t.Errorf("the password's button = %q, hint %q; the frames «New password», %q", tb.Password.Reveal.Label, tb.Password.Reveal.Hint, hints[0])
	}
	if strings.Contains(runsText(tb.Note), "Regenerate") {
		t.Errorf("the note still says Regenerate: %q", tb.Note)
	}
}

func TestBackupRailWithoutJobs(t *testing.T) {
	frame := mockupFrame(t, "s-backup-6")
	if !strings.Contains(frame, `<use href="#i-archive"/></svg>Backup</a>`) || strings.Contains(frame, "Backups are running") {
		t.Fatal("s-backup-6 counts its jobs or says the copies run: read the frame again")
	}
	f := backupFixture(t)
	rail := decode[view.BackupPage](t, f.h.do(get("/app/settings/backup", f.sess))).Rail
	for _, n := range rail.Nav {
		if n.ID == "settings.backup" && n.Count != nil {
			t.Errorf("no job, and Backup counts %d", *n.Count)
		}
	}
	for _, st := range rail.Status {
		if strings.Contains(strings.ToLower(string(st.Title)), "backup") {
			t.Errorf("no job, and the rail says %q", st.Title)
		}
	}
	saveJob(t, f, jobForm(f, sealed.BackupToWebDAV, "8", "pw"))
	rail = decode[view.BackupPage](t, f.h.do(get("/app/settings/backup", f.sess))).Rail
	for _, n := range rail.Nav {
		if n.ID == "settings.backup" && (n.Count == nil || *n.Count != 1) {
			t.Errorf("one job, and Backup counts %v", n.Count)
		}
	}
}
