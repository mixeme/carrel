package web_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/backup"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// script is a request from somebody else's machine: no cookie, the secret in
// the header.
func script(method, path, secret string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	if secret != "" {
		r.Header.Set("Authorization", "Bearer "+secret)
	}
	return r
}

var secretIn = regexp.MustCompile(`Bearer (cbk_[A-Za-z0-9_-]+)`)

func issueTrigger(t *testing.T, f davFixture, job sealed.BackupJob) (string, view.TriggerBlock) {
	t.Helper()
	unlock(t, f)
	page := decode[view.BackupPage](t, f.h.do(postForm("/app/settings/backup?op=trigger&job="+job.ID, f.sess, url.Values{})))
	tb := page.Jobs[0].Trigger
	if tb == nil {
		t.Fatal("no trigger card after the trigger was issued")
	}
	m := secretIn.FindStringSubmatch(string(tb.Command))
	if m == nil || !tb.Password.Shown || string(tb.Password.Value) != m[1] {
		t.Fatalf("trigger = %+v", tb)
	}
	return m[1], *tb
}

// S-24.4: the trigger is issued only inside the unlock window and shown once;
// the line starts a copy with nobody signed in; the status is one word; the
// archive is taken from where it lies; a wrong secret is the one "no".
func TestExternalTriggerRunsWithoutASession(t *testing.T) {
	f := backupFixture(t)
	job := saveJob(t, f, jobForm(f, sealed.BackupToWebDAV, "8", "pw"))
	locked := decode[view.BackupPage](t, f.h.do(postForm("/app/settings/backup?op=trigger&job="+job.ID, f.sess, url.Values{})))
	if !hasNotice(locked.Notices, "Confirm your password first") {
		t.Fatalf("locked = %+v", locked.Notices)
	}
	secret, tb := issueTrigger(t, f, job)
	if !strings.Contains(string(tb.Command), "0 3 * * * curl -fsS -m 30 -X POST") || !strings.Contains(string(tb.Command), "/api/backup/run?job="+job.ID) {
		t.Errorf("command = %q", tb.Command)
	}
	state, _ := json.Marshal(f.h.st.Snapshot())
	if strings.Contains(string(state), strings.TrimPrefix(secret, "cbk_")) {
		t.Error("the state holds the secret")
	}
	again := decode[view.BackupPage](t, f.h.do(get("/app/settings/backup", f.sess)))
	if again.Jobs[0].Trigger == nil || again.Jobs[0].Trigger.Command != "" || again.Jobs[0].Trigger.Password.Shown {
		t.Error("the secret is shown again")
	}

	started := f.h.do(script(http.MethodPost, "/api/backup/run?job="+job.ID, secret))
	if started.Code != http.StatusAccepted || len(started.Result().Cookies()) != 0 {
		t.Fatalf("run: %d %q", started.Code, started.Body.String())
	}
	id := strings.TrimSpace(started.Body.String())
	deadline := time.Now().Add(30 * time.Second)
	status := ""
	for time.Now().Before(deadline) {
		status = strings.TrimSpace(f.h.do(script(http.MethodGet, "/api/backup/status?id="+id, secret)).Body.String())
		if status != "running" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if status != "done" {
		t.Fatalf("status = %q", status)
	}
	if st := f.h.do(script(http.MethodGet, "/api/backup/status", secret)).Body.String(); !strings.HasPrefix(st, "done ") || !strings.Contains(st, "last whole copy") {
		t.Errorf("job status = %q", st)
	}
	dl := f.h.do(script(http.MethodGet, "/api/backup/"+id+"/download", secret))
	if dl.Code != http.StatusOK {
		t.Fatalf("download: %d %s", dl.Code, dl.Body.String())
	}
	if _, _, err := backup.Open(dl.Body.Bytes(), "pw"); err != nil {
		t.Errorf("the archive taken does not open: %v", err)
	}

	never := f.h.do(get("/forgot/"+strings.Repeat("A", 43), nil)).Body.String()
	for name, req := range map[string]*http.Request{
		"no secret":    script(http.MethodPost, "/api/backup/run?job="+job.ID, ""),
		"wrong secret": script(http.MethodPost, "/api/backup/run?job="+job.ID, "cbk_"+strings.Repeat("A", 43)),
		"other job":    script(http.MethodPost, "/api/backup/run?job=j-other", secret),
	} {
		if rec := f.h.do(req); rec.Code != http.StatusNotFound || rec.Body.String() != never {
			t.Errorf("%s: %d %q", name, rec.Code, rec.Body.String())
		}
	}

	if rec := f.h.do(postForm("/app/settings/backup?op=trigger-off&job="+job.ID, f.sess, url.Values{})); rec.Code != http.StatusSeeOther {
		t.Fatalf("off: %d", rec.Code)
	}
	if rec := f.h.do(script(http.MethodGet, "/api/backup/status", secret)); rec.Code != http.StatusNotFound {
		t.Errorf("a secret turned off still works: %d", rec.Code)
	}
}

// A job that downloads is taken in the one request that builds it; Both
// gets a script, not a line; an unencrypted job is warned twice.
func TestExternalTriggerDownloadAndBoth(t *testing.T) {
	f := backupFixture(t)
	job := saveJob(t, f, jobForm(f, sealed.BackupToDownload, "8", ""))
	secret, tb := issueTrigger(t, f, job)
	if !strings.Contains(string(tb.Command), "-o /var/backups/") || !strings.Contains(string(tb.Command), `\%F`) {
		t.Errorf("download command = %q", tb.Command)
	}
	if !strings.Contains(string(tb.Note), "not encrypted") {
		t.Errorf("no warning for an open archive with a trigger: %q", tb.Note)
	}
	rec := f.h.do(script(http.MethodGet, "/api/backup/download?job="+job.ID, secret))
	if rec.Code != http.StatusOK || rec.Body.Len() == 0 || backup.Encrypted(rec.Body.Bytes()) {
		t.Fatalf("download: %d", rec.Code)
	}
	if rec := f.h.do(script(http.MethodPost, "/api/backup/run?job="+job.ID, secret)); rec.Code != http.StatusConflict {
		t.Errorf("run of a download job: %d", rec.Code)
	}

	both := saveJob(t, f, jobForm(f, sealed.BackupToBoth, "8", "pw"))
	page := decode[view.BackupPage](t, f.h.do(postForm("/app/settings/backup?op=trigger&job="+both.ID, f.sess, url.Values{})))
	for _, j := range page.Jobs {
		if j.Name == "Рабочее" && j.Trigger != nil && strings.Contains(string(j.Trigger.Command), "#!/bin/sh") {
			if !strings.Contains(string(j.Trigger.Command), "until [") {
				t.Errorf("script = %q", j.Trigger.Command)
			}
			return
		}
	}
	t.Error("Both got no script")
}
