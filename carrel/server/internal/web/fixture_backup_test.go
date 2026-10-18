package web_test

import (
	"net/url"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/store"
)

// C-14.2: the list of backup jobs as s-backup-1 draws it (Нс54, Нс66) —
// three jobs, each its own sources, destination and trigger, and the runs
// of the history: a whole copy read back, one with a collection the server
// refused, a copy made at sign-in. The jobs are made and the runs written
// through the service on the clock of the frames: a real run is a stand's
// work and its sizes the machine's.
func TestFixtureBackupStates(t *testing.T) {
	app := newMockupApp(t)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	key, _ := app.sess.DataKey()
	sel := func(slugs ...string) []sealed.Selection {
		var out []sealed.Selection
		for _, slug := range slugs {
			out = append(out, sealed.Selection{AccountID: app.realAccount(t, slug), Path: app.realPath(t, slug)})
		}
		return out
	}
	save := func(job sealed.BackupJob) string {
		id, err := app.h.users.SaveBackupJob(app.sess.AccountID(), key, job)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	work := save(sealed.BackupJob{Name: "Рабочее, ежедневно", Sources: sel("rabota", "proekty", "lichnyj"), Mode: sealed.BackupToWebDAV,
		DestAccount: app.realAccount(t, "Documents"), DestPath: app.realPath(t, "Documents"), DestFolder: []string{"backup", "work"}, Keep: 14, Encrypted: true})
	home := save(sealed.BackupJob{Name: "Личное на домашний диск", Sources: sel("lichnyj", "arhiv", "Files"), Mode: sealed.BackupToWebDAV,
		DestAccount: app.realAccount(t, "Files"), DestPath: app.realPath(t, "Files"), DestFolder: []string{"carrel-backup"}, Keep: 8, Encrypted: true, OnSignInDays: 7})
	save(sealed.BackupJob{Name: "Contacts archive, by hand", Sources: sel("arhiv"), Mode: sealed.BackupToDownload, Off: true})
	if _, err := app.h.users.EnableBackupTrigger(app.sess.AccountID(), key, work); err != nil {
		t.Fatal(err)
	}
	const mb = 1 << 20
	for _, r := range []struct {
		job, at, trigger string
		size             int64
		cols, whole      int
		err              string
	}{
		{home, "2026-08-14 09:12", "sign-in", 2100 * mb, 2, 2, ""},
		{work, "2026-08-15 03:10", "external", 182 * mb / 10, 3, 2, "baikal · Общий: the server answered 401, so this collection is not in the archive. The other two were read back."},
		{work, "2026-08-16 03:10", "external", 184 * mb / 10, 3, 3, ""},
	} {
		app.h.users.SetClock(mockupDayAt(t, r.at))
		id, err := app.h.users.StartBackupRun(app.sess.AccountID(), r.job, r.trigger, sealed.BackupToWebDAV, func(string) bool { return false })
		if err != nil {
			t.Fatal(err)
		}
		state := "done"
		if r.whole < r.cols {
			state = "partial"
		}
		if err := app.h.users.FinishBackupRun(app.sess.AccountID(), r.job, id, func(run *store.BackupRun) {
			run.State, run.Size, run.Collections, run.Whole, run.Verified, run.Error = state, r.size, r.cols, r.whole, true, r.err
			run.File = "carrel-" + r.job + ".zip.age"
		}); err != nil {
			t.Fatal(err)
		}
	}
	app.h.users.SetClock(mockupNow(t))
	app.compare(t, "app-settings-backup/jobs.json", app.h.do(get("/app/settings/backup", app.sess)))

	// A job's page the moment its line was issued (s-backup-2, Нс61, Нс62):
	// the form in its rubrics, the tick on, the card with the line once.
	now := mockupNow(t).Add(mockupSince)
	app.sess.SetUnlockUntil(now.Add(4*time.Minute + 12*time.Second))
	app.compare(t, "app-settings-backup/job.json", app.h.do(postForm("/app/settings/backup?op=trigger&job="+home, app.sess, url.Values{})))
}
