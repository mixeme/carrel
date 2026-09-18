package user

import (
	"strings"

	"carrel.dev/carrel/server/internal/crypt"
	"carrel.dev/carrel/server/internal/store"
)

// TriggerPrefix marks a backup trigger's secret, so a person who finds one in
// a crontab knows what it is.
const TriggerPrefix = "cbk_"

// EnableBackupTrigger turns a job's external trigger on, or issues it anew
// (S-24.4): a secret of the job — not of the account, so the machine that
// runs the work copy cannot run the personal one — and an envelope of the
// person's data key under server.key, bound to the job. That envelope is
// the named loss of protection: a stolen volume with server.key reads what
// this job reads. The secret is returned once. The caller checks the unlock
// window.
func (s *Service) EnableBackupTrigger(viewerID string, dataKey [crypt.KeyLen]byte, jobID string) (string, error) {
	blob, ok := s.Catalog(viewerID, dataKey)
	if !ok {
		return "", ErrNotFound
	}
	if _, ok := blob.BackupJob(jobID); !ok {
		return "", ErrNotFound
	}
	raw, hash, err := newSecret()
	if err != nil {
		return "", err
	}
	env, err := s.st.SealBackupKey(jobID, dataKey)
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
		crypt.Wipe(b.KeyEnvelope)
		b.TriggerHash, b.KeyEnvelope = hash, env
		return []store.AuditEntry{{When: now, Who: u.Login, What: "backup.trigger.on", Whom: u.Login, Detail: jobID}}, nil
	})
	if err != nil {
		return "", err
	}
	return TriggerPrefix + raw, nil
}

// DisableBackupTrigger turns the trigger off: the secret and the envelope
// go, and every line handed out stops working.
func (s *Service) DisableBackupTrigger(viewerID, jobID string) error {
	now := s.now()
	return s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		u := st.UserByID(viewerID)
		if u == nil {
			return nil, ErrNotFound
		}
		b := backupStateOf(st, viewerID, jobID)
		crypt.Wipe(b.KeyEnvelope)
		b.TriggerHash, b.KeyEnvelope = nil, nil
		return []store.AuditEntry{{When: now, Who: u.Login, What: "backup.trigger.off", Whom: u.Login, Detail: jobID}}, nil
	})
}

// BackupTriggerOn reports whether a job's external trigger is on.
func (s *Service) BackupTriggerOn(viewerID, jobID string) bool {
	snap := s.st.Snapshot()
	for _, b := range snap.Backups {
		if b.UserID == viewerID && b.JobID == jobID {
			return len(b.TriggerHash) > 0
		}
	}
	return false
}

// OpenBackupTrigger checks a trigger's secret and opens the job it belongs
// to and the person's data key from its envelope. Every job with a trigger is
// compared, in constant time each; a wrong secret, a disabled account and a
// job without a trigger are one false. Every use is journaled, naming the
// job.
func (s *Service) OpenBackupTrigger(token string) (userID, jobID string, dataKey [crypt.KeyLen]byte, ok bool) {
	raw, found := strings.CutPrefix(token, TriggerPrefix)
	if !found {
		return "", "", dataKey, false
	}
	hash, err := hashToken(raw)
	if err != nil {
		return "", "", dataKey, false
	}
	snap := s.st.Snapshot()
	var match *store.BackupState
	for i := range snap.Backups {
		b := &snap.Backups[i]
		if len(b.TriggerHash) > 0 && verifyHash(b.TriggerHash, hash) && match == nil {
			match = b
		}
	}
	if match == nil {
		return "", "", dataKey, false
	}
	jobID = match.JobID
	u := snap.UserByID(match.UserID)
	if u == nil || u.Disabled {
		return "", "", dataKey, false
	}
	key, err := s.st.OpenBackupKey(jobID, match.KeyEnvelope)
	if err != nil {
		return "", "", dataKey, false
	}
	now := s.now()
	_ = s.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		return []store.AuditEntry{{When: now, Who: u.Login, What: "backup.trigger.use", Whom: u.Login, Detail: jobID}}, nil
	})
	return u.ID, jobID, key, true
}
