package user

import (
	"errors"
	"time"

	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/store"
)

// ErrBackupRunning is a job asked to run while it runs.
var ErrBackupRunning = errors.New("user: this job is already running")

// Run states (S-24.3): a copy that went in whole and was read back is done;
// one that left a collection out is partial; one that was not written or not
// read back is failed. A start the external trigger refused is a row of
// the history and no run (note 91).
const (
	RunRunning = "running"
	RunDone    = "done"
	RunPartial = "partial"
	RunFailed  = "failed"
	RunRefused = "refused"
)

// SaveBackupJob writes a job into the blob, new when its ID is empty, and
// returns its ID. The journal names the job.
func (s *Service) SaveBackupJob(viewerID string, dataKey [crypt.KeyLen]byte, job sealed.BackupJob) (string, error) {
	if job.ID == "" {
		id, err := newEntityID("j-")
		if err != nil {
			return "", err
		}
		job.ID = id
	}
	err := s.editCatalog(viewerID, dataKey, "backup.job.save", job.ID, func(blob *sealed.Blob) error {
		for i := range blob.BackupJobs {
			if blob.BackupJobs[i].ID == job.ID {
				blob.BackupJobs[i] = job
				return nil
			}
		}
		blob.BackupJobs = append(blob.BackupJobs, job)
		return nil
	})
	return job.ID, err
}

// DeleteBackupJob removes a job and its state. Archives already written stay
// where they are: the job goes, not the copies.
func (s *Service) DeleteBackupJob(viewerID string, dataKey [crypt.KeyLen]byte, id string) error {
	err := s.editCatalog(viewerID, dataKey, "backup.job.delete", id, func(blob *sealed.Blob) error {
		kept := blob.BackupJobs[:0]
		found := false
		for _, j := range blob.BackupJobs {
			if j.ID == id {
				found = true
				continue
			}
			kept = append(kept, j)
		}
		if !found {
			return ErrNotFound
		}
		blob.BackupJobs = kept
		return nil
	})
	if err != nil {
		return err
	}
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		kept := st.Backups[:0]
		for _, b := range st.Backups {
			if b.JobID == id && b.UserID == viewerID {
				crypt.Wipe(b.KeyEnvelope)
				continue
			}
			kept = append(kept, b)
		}
		st.Backups = kept
		return nil, nil
	})
}

// BackupRuns are the viewer's runs by job, newest first.
func (s *Service) BackupRuns(viewerID string) map[string][]store.BackupRun {
	return s.backupRuns(viewerID, false)
}

// BackupRefusals are the starts the external trigger refused, by job,
// newest first (note 91): rows of the history, and no run of a job — not
// its last copy, not anything to restore.
func (s *Service) BackupRefusals(viewerID string) map[string][]store.BackupRun {
	return s.backupRuns(viewerID, true)
}

func (s *Service) backupRuns(viewerID string, refused bool) map[string][]store.BackupRun {
	snap := s.st.Snapshot()
	out := map[string][]store.BackupRun{}
	for _, b := range snap.Backups {
		if b.UserID != viewerID {
			continue
		}
		for i := len(b.Runs) - 1; i >= 0; i-- {
			if (b.Runs[i].State == RunRefused) == refused {
				out[b.JobID] = append(out[b.JobID], b.Runs[i])
			}
		}
	}
	return out
}

func backupStateOf(st *store.State, viewerID, jobID string) *store.BackupState {
	for i := range st.Backups {
		if st.Backups[i].JobID == jobID && st.Backups[i].UserID == viewerID {
			return &st.Backups[i]
		}
	}
	st.Backups = append(st.Backups, store.BackupState{JobID: jobID, UserID: viewerID})
	return &st.Backups[len(st.Backups)-1]
}

// StartBackupRun records a run as started and returns its ID; a job runs
// once at a time. A run left "running" by a process that stopped is closed as
// failed here, so it never blocks the job for good.
//
// via is an external run's secret hint and the address it came from.
func (s *Service) StartBackupRun(viewerID, jobID, trigger, mode string, running func(runID string) bool, via ...string) (string, error) {
	id, err := newEntityID("r-")
	if err != nil {
		return "", err
	}
	now := s.now()
	err = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(viewerID)
		if u == nil || u.Disabled {
			return nil, ErrNotFound
		}
		b := backupStateOf(st, viewerID, jobID)
		for i := range b.Runs {
			r := &b.Runs[i]
			if r.State != RunRunning {
				continue
			}
			if running(r.ID) {
				return nil, ErrBackupRunning
			}
			r.State, r.Error, r.Finished = RunFailed, "the process stopped during the run", &now
		}
		run := store.BackupRun{ID: id, Started: now, State: RunRunning, Trigger: trigger, Mode: mode}
		if len(via) == 2 {
			run.Password, run.From = via[0], via[1]
		}
		b.Runs = append(b.Runs, run)
		if len(b.Runs) > store.BackupRunsKept {
			b.Runs = b.Runs[len(b.Runs)-store.BackupRunsKept:]
		}
		return []store.AuditEntry{{When: now, Who: u.Login, What: "backup.run", Whom: u.Login, Detail: jobID + " " + trigger}}, nil
	})
	return id, err
}

// FinishBackupRun writes how a run ended.
func (s *Service) FinishBackupRun(viewerID, jobID, runID string, finish func(r *store.BackupRun)) error {
	now := s.now()
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		b := backupStateOf(st, viewerID, jobID)
		for i := range b.Runs {
			if b.Runs[i].ID == runID {
				finish(&b.Runs[i])
				b.Runs[i].Finished = &now
				u := st.UserByID(viewerID)
				if u == nil {
					return nil, nil
				}
				return []store.AuditEntry{{When: now, Who: u.Login, What: "backup.run." + b.Runs[i].State, Whom: u.Login, Detail: jobID}}, nil
			}
		}
		return nil, ErrNotFound
	})
}

// LastWholeRun is when a job last finished whole, or zero.
func LastWholeRun(runs []store.BackupRun) time.Time {
	for _, r := range runs {
		if r.State == RunDone && r.Finished != nil {
			return *r.Finished
		}
	}
	return time.Time{}
}
