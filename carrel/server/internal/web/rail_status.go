package web

import (
	"strconv"
	"sync"
	"time"

	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/user"
	"carrel.dev/carrel/server/view"
)

// The state block at the foot of the settings and administration rails
// (components-35): davloom and the copies under the settings, mail and the
// installation check under the administration. Each line is what this
// server knows, not a guess: a time it did not see since the start is said
// as such.

// staleBackup is how old the newest copy may be before the block raises it:
// the week of the «if older than 7 days» trigger (s-backup-1).
const staleBackup = 7 * 24 * time.Hour

// railMarks are the moments the block names that nothing else keeps: the
// last letter that left and the last installation check, since the start.
type railMarks struct {
	mu        sync.Mutex
	sent      time.Time
	checked   time.Time
	checkBad  int
	checkRows int
}

func (s *Server) markSent() {
	s.marks.mu.Lock()
	s.marks.sent = s.now()
	s.marks.mu.Unlock()
}

func (s *Server) markChecked(rows []view.InstallRow) {
	bad := 0
	for _, r := range rows {
		if r.State == view.CheckBad {
			bad++
		}
	}
	s.marks.mu.Lock()
	s.marks.checked, s.marks.checkBad, s.marks.checkRows = s.now(), bad, len(rows)
	s.marks.mu.Unlock()
}

// stampRun is a time of the block, monospaced as the frames write it.
func (s *Server) stampRun(sess *session.Session, t time.Time) view.Run {
	return monoRun(t.In(s.timeFor(sess).tz).Format("2006-01-02 15:04"))
}

// settingsStatus is davloom and, when there are jobs, the copies.
func (s *Server) settingsStatus(sess *session.Session) []view.RailStatus {
	out := []view.RailStatus{s.davloomStatus(sess)}
	if st, ok := s.backupStatus(sess); ok {
		out = append(out, st)
	}
	return out
}

func (s *Server) davloomStatus(sess *session.Session) view.RailStatus {
	if s.cfg.DAVloomPort == 0 || sess == nil {
		return view.RailStatus{Tone: view.StatusOff, Title: "davloom is off", Line: line("No device connects until it is started.")}
	}
	devices := s.users.Devices(sess.AccountID())
	var last time.Time
	for _, d := range devices {
		if at := s.users.DeviceLastSeen(d.ID); at.After(last) {
			last = at
		}
	}
	st := view.RailStatus{Tone: view.StatusOn, Title: text("davloom · running · " + plural(len(devices), "device"))}
	if last.IsZero() {
		st.Line = line("No device has connected since the start.")
	} else {
		st.Line = join("Last response ", s.stampRun(sess, last))
	}
	return st
}

// backupStatus is the newest copy of any job; nothing when there is no job.
func (s *Server) backupStatus(sess *session.Session) (view.RailStatus, bool) {
	if sess == nil || len(s.catalogOf(sess).BackupJobs) == 0 {
		return view.RailStatus{}, false
	}
	var newest *struct {
		at       time.Time
		state    string
		verified bool
	}
	for _, runs := range s.users.BackupRuns(sess.AccountID()) {
		for _, r := range runs {
			if r.State == user.RunRunning {
				continue
			}
			if newest == nil || r.Started.After(newest.at) {
				newest = &struct {
					at       time.Time
					state    string
					verified bool
				}{r.Started, r.State, r.Verified}
			}
		}
	}
	if newest == nil {
		return view.RailStatus{Tone: view.StatusOff, Title: "No backup yet", Line: line("No job has run.")}, true
	}
	last := join("Last backup ", s.stampRun(sess, newest.at))
	switch age := s.now().Sub(newest.at); {
	case newest.state == user.RunPartial || newest.state == user.RunFailed:
		return view.RailStatus{Tone: view.StatusAlert, Title: "The last backup is not whole", Line: last}, true
	case age > staleBackup:
		return view.RailStatus{Tone: view.StatusAlert, Title: text("No backup for " + plural(int(age/(24*time.Hour)), "day")), Line: last}, true
	case newest.verified:
		// «verified» says what it means beside it (account-*, Нс5).
		verified := view.Run{Text: "verified", Hint: "The archive was read back from the destination and the checksum matched"}
		return view.RailStatus{Tone: view.StatusOn, Title: "Backups are running", Line: join(last, " · ", verified)}, true
	}
	return view.RailStatus{Tone: view.StatusOn, Title: "Backups are running", Line: last}, true
}

// adminStatus is mail and the installation check.
func (s *Server) adminStatus(sess *session.Session) []view.RailStatus {
	s.marks.mu.Lock()
	sent, checked, bad := s.marks.sent, s.marks.checked, s.marks.checkBad
	s.marks.mu.Unlock()
	mail := view.RailStatus{Tone: view.StatusAlert, Title: "SMTP not configured", Line: line("No mail is going out.")}
	if s.mailConfigured() {
		mail = view.RailStatus{Tone: view.StatusOn, Title: "Mail is going out", Line: line("No letter since the start.")}
		if !sent.IsZero() {
			mail.Line = join("Last sent ", s.stampRun(sess, sent))
		}
	}
	check := view.RailStatus{Tone: view.StatusOff, Title: "Not checked yet", Line: line("Run the check once Carrel is behind its proxy.")}
	if !checked.IsZero() {
		check = view.RailStatus{Tone: view.StatusOn, Title: "Checks passed", Line: join("Last check ", s.stampRun(sess, checked))}
		if bad > 0 {
			word := " checks failed"
			if bad == 1 {
				word = " check failed"
			}
			check.Tone, check.Title = view.StatusAlert, text(strconv.Itoa(bad)+word)
		}
	}
	return []view.RailStatus{mail, check}
}
