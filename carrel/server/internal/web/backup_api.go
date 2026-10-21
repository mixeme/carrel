package web

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/internal/user"
)

// The external trigger (S-24.4, C-5.2) is an interface for somebody else's
// script, not for a browser: the job's secret in the Authorization header,
// a short plain-text answer, no cookie, no markup, no redirect.
//
//	POST /api/backup/run            starts a copy: 202 and the run's id
//	GET  /api/backup/status         ?id= one word; ?job= the last run and when
//	GET  /api/backup/{id}/download  the archive a run wrote; 409 while it runs
//	GET  /api/backup/download       a fresh archive, streamed in this request
//
// A wrong secret is the one "no" of every secret route, the same body in the
// same time as a token that never existed. A right one reads with the data
// key its envelope holds.

// triggerJob is the job a request's secret opens, with its person and their
// catalog; false once the answer has been written.
type triggerJob struct {
	userID string
	blob   sealed.Blob
	job    sealed.BackupJob
	// via is the secret's hint and the address the request came from: what
	// the job's history names a run by (note 91)
	via []string
}

func (s *Server) openTrigger(w http.ResponseWriter, r *http.Request) (triggerJob, bool) {
	start := time.Now()
	ip := s.clientIP(r)
	if !s.limit.anonRoom(ip, start) {
		plain(w, http.StatusTooManyRequests, "too many requests")
		return triggerJob{}, false
	}
	token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	userID, jobID, key, ok := s.users.OpenBackupTrigger(strings.TrimSpace(token))
	if !ok {
		// A refusal is written down in the history of the job it named
		// (note 91): the secret tried by its hint, and where from.
		if job := r.URL.Query().Get("job"); job != "" && token != "" {
			s.users.NoteRefusedTrigger(job, strings.TrimSpace(token), ip)
		}
		// The one "no" of every secret route (audit №3): the same answer, in
		// the same time, as a token that never existed.
		s.limit.allowAnon(ip, start)
		s.tokenNotFound(w, r, start)
		return triggerJob{}, false
	}
	if !s.limit.allowLink("backup:"+jobID, start) {
		plain(w, http.StatusTooManyRequests, "too many requests")
		return triggerJob{}, false
	}
	blob, ok := s.users.Catalog(userID, key)
	job, has := blob.BackupJob(jobID)
	if !ok || !has || (r.URL.Query().Get("job") != "" && r.URL.Query().Get("job") != jobID) {
		s.tokenNotFound(w, r, start)
		return triggerJob{}, false
	}
	return triggerJob{userID: userID, blob: blob, job: job, via: []string{user.TriggerHint(strings.TrimSpace(token)), ip}}, true
}

// jobIsOff answers a start of a job whose «Job is on» is unticked: it runs
// from the screen only (s-backup-2).
func jobIsOff(w http.ResponseWriter, t triggerJob) bool {
	if t.job.Off {
		plain(w, http.StatusConflict, "this job is off: tick «Job is on» on its page")
	}
	return t.job.Off
}

func plain(w http.ResponseWriter, code int, body string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_, _ = io.WriteString(w, body+"\n")
}

// postBackupRun starts a copy to the job's WebDAV destination and answers at
// once: accepted, and the run's id — not whether the copy went through.
func (s *Server) postBackupRun(w http.ResponseWriter, r *http.Request) {
	t, ok := s.openTrigger(w, r)
	if !ok {
		return
	}
	if jobIsOff(w, t) {
		return
	}
	if t.job.Mode == sealed.BackupToDownload {
		plain(w, http.StatusConflict, "this job writes nowhere: GET /api/backup/download takes its archive")
		return
	}
	id, err := s.startBackup(t.userID, t.blob, t.job, "external", t.via...)
	switch {
	case errors.Is(err, user.ErrBackupRunning):
		plain(w, http.StatusConflict, "running")
	case err != nil:
		plain(w, http.StatusConflict, err.Error())
	default:
		plain(w, http.StatusAccepted, id)
	}
}

// getBackupStatus is one word for a run (?id=): running, done, partial,
// failed. For the job (?job=, or nothing) it is the last run's word and when
// it finished, and the last whole copy — what a watchdog asks.
func (s *Server) getBackupStatus(w http.ResponseWriter, r *http.Request) {
	t, ok := s.openTrigger(w, r)
	if !ok {
		return
	}
	runs := s.users.BackupRuns(t.userID)[t.job.ID]
	if id := r.URL.Query().Get("id"); id != "" {
		for _, run := range runs {
			if run.ID == id {
				plain(w, http.StatusOK, run.State)
				return
			}
		}
		plain(w, http.StatusNotFound, "no such run")
		return
	}
	if len(runs) == 0 {
		plain(w, http.StatusOK, "never")
		return
	}
	last := runs[0]
	line := last.State
	if last.Finished != nil {
		line += " " + last.Finished.UTC().Format(time.RFC3339)
	}
	if whole := user.LastWholeRun(runs); !whole.IsZero() {
		line += "\nlast whole copy " + whole.UTC().Format(time.RFC3339)
	}
	plain(w, http.StatusOK, line)
}

// getBackupDownload hands over the archive a run wrote, streamed from its
// destination; 409 while it runs, so a script waits instead of failing.
func (s *Server) getBackupDownload(w http.ResponseWriter, r *http.Request) {
	t, ok := s.openTrigger(w, r)
	if !ok {
		return
	}
	var run *store.BackupRun
	for _, rn := range s.users.BackupRuns(t.userID)[t.job.ID] {
		if rn.ID == r.PathValue("id") {
			run = &rn
			break
		}
	}
	switch {
	case run == nil:
		plain(w, http.StatusNotFound, "no such run")
		return
	case run.State == user.RunRunning:
		plain(w, http.StatusConflict, "running")
		return
	case run.File == "" || !run.Verified:
		plain(w, http.StatusNotFound, fmt.Sprintf("this run left no archive to take: %s", run.State))
		return
	}
	dest, err := s.backupDest(t.blob, t.job)
	if err != nil {
		plain(w, http.StatusConflict, err.Error())
		return
	}
	body, err := dest.Open(r.Context(), run.File)
	if err != nil {
		plain(w, http.StatusBadGateway, "the destination did not hand the archive back")
		return
	}
	defer body.Close()
	if v := mime.FormatMediaType("attachment", map[string]string{"filename": run.File}); v != "" {
		w.Header().Set("Content-Disposition", v)
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = io.Copy(w, body)
}

// getBackupDownloadLegacy builds a fresh archive of a job that downloads and
// streams it in this same request: nothing to wait for, and the bytes keep a
// proxy's idle timer from firing.
func (s *Server) getBackupDownloadLegacy(w http.ResponseWriter, r *http.Request) {
	t, ok := s.openTrigger(w, r)
	if !ok {
		return
	}
	if jobIsOff(w, t) {
		return
	}
	if t.job.Mode == sealed.BackupToWebDAV {
		plain(w, http.StatusConflict, "this job writes to WebDAV: POST /api/backup/run, then GET /api/backup/{id}/download")
		return
	}
	if err := s.streamBackup(r.Context(), w, t.userID, t.blob, t.job, "external", t.via...); errors.Is(err, user.ErrBackupRunning) {
		plain(w, http.StatusConflict, "running")
	}
}
