package web_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
)

// s-backup-2…5: the card of the external trigger hands over what to paste
// where the clock already is, by tab — crontab, GoSentry, Gitea, curl — and
// the password on its own with what it opens. What depends on the instance
// and the job is the server's (the address, the header of the secret, the
// job, the secret); the words of the card are the screen's. A job that
// writes to WebDAV is started by a line; Download and Both need a script,
// and GoSentry then runs the script.
func TestTriggerTabsAreWhatTheFramesHandOver(t *testing.T) {
	f := backupFixture(t)
	job := saveJob(t, f, jobForm(f, sealed.BackupToWebDAV, "8", "pw"))
	secret, tb := issueTrigger(t, f, job)
	run := "/api/backup/run?job=" + job.ID
	bearer := "Authorization: Bearer " + secret
	if !tb.Shown || tb.Script || tb.ScriptName != "" || tb.CronLine != "" || tb.At != "03:00" || tb.Header != "Authorization" {
		t.Errorf("line: shown %v, script %v %q %q, at %q, header %q", tb.Shown, tb.Script, tb.ScriptName, tb.CronLine, tb.At, tb.Header)
	}
	g := tb.GoSentry
	args := make([]string, len(g.Arguments))
	for i, a := range g.Arguments {
		args[i] = string(a)
	}
	if g.Name == "" || g.Folder != "Backup" || g.Schedule != "0 3 * * *" || g.Command != "curl" || len(args) != 8 ||
		strings.Join(args[:7], "\n") != "-fsS\n-m\n30\n-X\nPOST\n-H\n"+bearer || !strings.HasPrefix(args[7], "http") || !strings.HasSuffix(args[7], run) {
		t.Errorf("GoSentry = %+v (curl %q)", g, tb.Curl)
	}
	if tb.Status != text("GET /api/backup/status?job="+job.ID) {
		t.Errorf("the second job's request = %q", tb.Status)
	}
	if !strings.HasPrefix(string(tb.Curl), "curl -fsS -m 30 -X POST -H '"+bearer+"' '") || !strings.HasSuffix(string(tb.Curl), run+"'") || strings.Contains(string(tb.Curl), "0 3 * * *") {
		t.Errorf("curl = %q", tb.Curl)
	}
	if !strings.Contains(string(tb.Gitea), "cron: '0 3 * * *'") || !strings.Contains(string(tb.Gitea), "secrets."+string(tb.GiteaSecret)) || strings.Contains(string(tb.Gitea), secret) ||
		!strings.HasPrefix(string(tb.GiteaFile), ".gitea/workflows/") || tb.GiteaSecret == "" {
		t.Errorf("Gitea %q in %q under %q:\n%s", tb.GiteaSecret, tb.GiteaFile, tb.GiteaSecret, tb.Gitea)
	}
	ends := strings.Join(texts(tb.Endpoints), "\n")
	if ends != "POST "+run+"\nGET /api/backup/status?job="+job.ID+"\nGET /api/backup/{id}/download" {
		t.Errorf("what it opens:\n%s", ends)
	}

	both := saveJob(t, f, jobForm(f, sealed.BackupToBoth, "8", "pw"))
	_, sb := issueTrigger(t, f, both)
	name := "carrel-" + both.ID + ".sh"
	if !sb.Script || sb.ScriptName != text(name) || sb.CronLine != text("0 3 * * * /usr/local/bin/"+name) ||
		sb.GoSentry.Command != text("/usr/local/bin/"+name) || len(sb.GoSentry.Arguments) != 0 || !strings.HasPrefix(string(sb.Command), "#!/bin/sh\nset -eu\n") {
		t.Errorf("script: %v %q %q %+v\n%s", sb.Script, sb.ScriptName, sb.CronLine, sb.GoSentry, sb.Command)
	}
}
