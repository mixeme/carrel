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

// openJob is the first job of the list opened on its page by its Edit
// (s-backup-2, Нс61): its When and the card of its line.
func openJob(t *testing.T, f davFixture) view.BackupJob {
	t.Helper()
	list := decode[view.BackupPage](t, f.h.do(get("/app/settings/backup", f.sess)))
	if len(list.Jobs) == 0 {
		t.Fatal("no job in the list")
	}
	page := decode[view.BackupPage](t, f.h.do(get(actionHref(t, list.Jobs[0].Actions, "backup.edit"), f.sess)))
	if page.Job == nil {
		t.Fatalf("the job's page has no job: %+v", page)
	}
	return *page.Job
}

var secretIn = regexp.MustCompile(`Bearer (cbk_[A-Za-z0-9_-]+)`)

// triggerSheetOf is the sheet of a job's external trigger, tab open
// (Typecase's note 91).
func triggerSheetOf(t *testing.T, f davFixture, jobID, tab string) view.BackupTriggerSheet {
	t.Helper()
	q := url.Values{"job": {jobID}}
	if tab != "" {
		q.Set("tab", tab)
	}
	return decode[view.BackupTriggerSheet](t, f.h.do(get("/app/settings/backup/trigger?"+q.Encode(), f.sess)))
}

// issueTrigger turns a job's trigger on inside the unlock window: the sheet
// answers with the password and the line whole, once (s-backup-13).
func issueTrigger(t *testing.T, f davFixture, job sealed.BackupJob) (string, view.TriggerBlock) {
	t.Helper()
	unlock(t, f)
	rec := f.h.do(postForm("/app/settings/backup/trigger?job="+job.ID+"&op=on", f.sess, url.Values{}))
	if rec.Code != http.StatusSeeOther || !strings.HasSuffix(rec.Header().Get("Location"), "/app/settings/backup/trigger?job="+job.ID) {
		t.Fatalf("Turn on: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	sheet := triggerSheetOf(t, f, job.ID, "")
	tb := sheet.Line
	if tb == nil || sheet.Password == nil {
		t.Fatalf("no password and no line after the trigger was issued: %+v", sheet)
	}
	m := secretIn.FindStringSubmatch(string(tb.Command))
	if m == nil || !tb.Shown || string(sheet.Password.Value) != m[1] {
		t.Fatalf("trigger = %+v, password %+v", tb, sheet.Password)
	}
	return m[1], *tb
}

// S-24.4: the trigger is issued only inside the unlock window and shown once;
// the line starts a copy with nobody signed in; the status is one word; the
// archive is taken from where it lies; a wrong secret is the one "no".
func TestExternalTriggerRunsWithoutASession(t *testing.T) {
	f := backupFixture(t)
	job := saveJob(t, f, jobForm(f, sealed.BackupToWebDAV, "8", "pw"))
	// Outside the window Turn on answers with the password's sheet.
	off := triggerSheetOf(t, f, job.ID, "")
	if off.Password != nil || off.Line != nil || len(off.Form.Actions) == 0 || off.Form.Actions[0].Method != http.MethodPost {
		t.Fatalf("off, locked = %+v", off)
	}
	if rec := f.h.do(postForm("/app/settings/backup/trigger?job="+job.ID+"&op=on", f.sess, url.Values{})); rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "for=trigger") {
		t.Fatalf("locked Turn on: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	secret, tb := issueTrigger(t, f, job)
	if !strings.Contains(string(tb.Command), "0 3 * * * curl -fsS -m 30 -X POST") || !strings.Contains(string(tb.Command), "/api/backup/run?job="+job.ID) {
		t.Errorf("command = %q", tb.Command)
	}
	state, _ := json.Marshal(f.h.st.Snapshot())
	if strings.Contains(string(state), strings.TrimPrefix(secret, "cbk_")) {
		t.Error("the state holds the secret")
	}
	again := triggerSheetOf(t, f, job.ID, "")
	if again.Password == nil || again.Password.Value != "" || again.Line == nil || again.Line.Shown || strings.Contains(string(again.Line.Command), secret) {
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

	if rec := f.h.do(postForm("/app/settings/backup/trigger?job="+job.ID+"&op=revoke", f.sess, url.Values{})); rec.Code != http.StatusSeeOther {
		t.Fatalf("revoke: %d", rec.Code)
	}
	if rec := f.h.do(script(http.MethodGet, "/api/backup/status", secret)); rec.Code != http.StatusNotFound {
		t.Errorf("a secret turned off still works: %d", rec.Code)
	}
}

// A job that downloads is taken in the one request that builds it, by a
// script; Both gets a script too, not a line; an unencrypted job is warned.
func TestExternalTriggerDownloadAndBoth(t *testing.T) {
	f := backupFixture(t)
	job := saveJob(t, f, jobForm(f, sealed.BackupToDownload, "8", ""))
	secret, tb := issueTrigger(t, f, job)
	// A stream needs somebody at its end: a script, the date in it (s-backup-2).
	if !tb.Script || !strings.Contains(string(tb.Command), "out=/var/backups") || !strings.Contains(string(tb.Command), "/download?job="+job.ID) || !strings.Contains(string(tb.Command), "$(date +%F)") {
		t.Errorf("download command = %q", tb.Command)
	}
	if !strings.Contains(runsText(tb.Note), "not encrypted") {
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
	_, sb := issueTrigger(t, f, both)
	if !strings.Contains(string(sb.Command), "#!/bin/sh") || !strings.Contains(string(sb.Command), "until [") {
		t.Errorf("Both: script = %q", sb.Command)
	}
	if !strings.Contains(runsText(sb.Destination), "two steps") {
		t.Errorf("why a script: %q", runsText(sb.Destination))
	}
}
