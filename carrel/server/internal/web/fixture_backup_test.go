package web_test

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/backup"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

// C-14.2: the list of backup jobs as s-backup-1 draws it (Нс54, Нс66) —
// three jobs, each its own sources, destination and trigger, and the runs
// of the history: a whole copy read back, one with a collection the server
// refused, a copy made at sign-in. The jobs are made and the runs written
// through the service on the clock of the frames: a real run is a stand's
// work and its sizes the machine's.
func TestFixtureBackupStates(t *testing.T) {
	// No job yet (s-backup-6): the settings' instance, its devices.
	empty := newMockupApp(t)
	settingsDevices(t, empty)
	empty.compare(t, "app-settings-backup/normal.json", empty.h.do(get("/app/settings/backup", empty.sess)))

	app := newMockupApp(t)
	work, home := settingsState(t, app)
	app.compare(t, "app-settings-backup/jobs.json", app.h.do(get("/app/settings/backup", app.sess)))

	// The restore on its own page (s-backup-7): where the archive is — a
	// job's copies, a row each —, the archive's password.
	app.compare(t, "app-settings-backup-restore/normal.json", app.h.do(get("/app/settings/backup/restore", app.sess)))

	// The archive opened (s-backup-8): a copy of the home job made for
	// real — the runs above are the frames' history and have no archive —,
	// its manifest read, matched by path.
	real := newMockupApp(t)
	_, own := settingsState(t, real)
	// The archive of s-backup-8: Личный, Работа, radicale's Проекты and Дни
	// рождения; radicale is disconnected after the copy, so its row says
	// the server is gone and another collection is chosen, in alarm.
	editBlob(t, real.h, real.sess, func(b *sealed.Blob) {
		for i := range b.BackupJobs {
			if b.BackupJobs[i].ID != own {
				continue
			}
			b.BackupJobs[i].Sources = nil
			for _, slug := range []string{"lichnyj", "rabota", "proekty", "dni-rozhdeniya"} {
				b.BackupJobs[i].Sources = append(b.BackupJobs[i].Sources,
					sealed.Selection{Kind: view.KindEvent, AccountID: real.realAccount(t, slug), Path: real.realPath(t, slug)})
			}
		}
	})
	if rec := real.h.do(postForm("/app/settings/backup?op=run&job="+own, real.sess, url.Values{})); rec.Code != http.StatusSeeOther {
		t.Fatalf("run: %d %s", rec.Code, rec.Body.String())
	}
	var made store.BackupRun
	for deadline := time.Now().Add(60 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if runs := real.h.users.BackupRuns(real.sess.AccountID())[own]; len(runs) > 0 && runs[0].State != "running" {
			made = runs[0]
			break
		}
	}
	if made.File == "" || !made.Verified {
		t.Fatalf("the home job's copy = %+v", made)
	}
	real.archive = strings.TrimSuffix(fmt.Sprintf("%.1f", float64(made.Size)/1024), ".0") + " KB"
	key, _ := real.sess.DataKey()
	if _, err := real.h.users.RemoveConnection(real.sess.AccountID(), key, real.realAccount(t, "proekty")); err != nil {
		t.Fatal(err)
	}
	// Since the copy Личный lost four events on its server and three were
	// changed there (s-backup-8: «4 records missing, will be created · 3
	// already here»; s-backup-18: created, and skipped without an answer).
	nc := real.servers["nextcloud"]
	lichnyj := real.realPath(t, "lichnyj")
	held := nc.Paths(lichnyj)
	if len(held) < 7 {
		t.Fatalf("Личный holds %d events, the frames lose four and change three", len(held))
	}
	for _, p := range held[:4] {
		nc.Unseed(p)
	}
	for _, p := range held[4:7] {
		body, _, _ := nc.Object(p)
		nc.Seed(p, strings.Replace(body, "SUMMARY:", "SUMMARY:Moved · ", 1))
	}
	if rec := real.h.do(postForm("/app/settings/backup/restore", real.sess, url.Values{
		"source": {"run"}, "run": {own + "|" + made.ID}, "password": {"archive-pw"},
	})); rec.Code != http.StatusSeeOther {
		t.Fatalf("open: %d %s", rec.Code, rec.Body.String())
	}
	// The last skipped, the others ticked (s-backup-8): Skip on it, then the
	// rest ticked again.
	match := "/app/settings/backup/restore/match?op="
	if rec := real.h.do(postForm(match+"skip", real.sess, url.Values{"pick.3": {"on"}})); rec.Code != http.StatusSeeOther {
		t.Fatalf("skip: %d %s", rec.Code, rec.Body.String())
	}
	if rec := real.h.do(postForm(match+"into", real.sess, url.Values{"pick.0": {"on"}, "pick.1": {"on"}, "pick.2": {"on"}})); rec.Code != http.StatusSeeOther {
		t.Fatalf("tick: %d %s", rec.Code, rec.Body.String())
	}
	real.compare(t, "app-settings-backup-restore/match.json", real.h.do(get("/app/settings/backup/restore", real.sess)))
	// Restore pressed (s-backup-18, Typecase's note 99): the report, a row
	// a collection restored, and the server refuses Проекты's one record as
	// too large.
	nc.RefuseAt("PUT", lichnyj+"9c27-0e5d-4412", http.StatusRequestEntityTooLarge, "")
	m := decode[view.BackupPage](t, real.h.do(get("/app/settings/backup/restore", real.sess))).Restore.Match
	if rec := real.h.do(postForm(match+"restore", real.sess, matchForm(*m))); rec.Code != http.StatusSeeOther {
		t.Fatalf("restore: %d %s", rec.Code, rec.Body.String())
	}
	real.compare(t, "app-settings-backup-report/normal.json", real.h.do(get("/app/settings/backup/restore/report", real.sess)))

	// The same archive brought from this computer by the live layer
	// (s-backup-20, Typecase's note 109): opened by its start and end, the
	// records not read yet — compared when restored —, the last skipped as
	// on s-backup-8.
	var archive []byte
	for _, srv := range real.servers {
		if body, ok := srv.File(real.realPath(t, "Files") + "carrel-backup/" + made.File); ok {
			archive = []byte(body)
		}
	}
	if archive == nil {
		t.Fatal("the home job's archive is not on its storage")
	}
	step1 := url.Values{"source": {"upload"}, "store": {""}, "password": {"archive-pw"}}
	if rec := real.h.do(restoreParts(t, real, "/app/settings/backup/restore?from=file", step1, archive, backup.OpenSpans(int64(len(archive))))); rec.Code != http.StatusSeeOther {
		t.Fatalf("open in parts: %d %s", rec.Code, rec.Body.String())
	}
	if rec := real.h.do(postForm(match+"skip", real.sess, url.Values{"pick.3": {"on"}})); rec.Code != http.StatusSeeOther {
		t.Fatalf("skip: %d %s", rec.Code, rec.Body.String())
	}
	if rec := real.h.do(postForm(match+"into", real.sess, url.Values{"pick.0": {"on"}, "pick.1": {"on"}, "pick.2": {"on"}})); rec.Code != http.StatusSeeOther {
		t.Fatalf("tick: %d %s", rec.Code, rec.Body.String())
	}
	real.compare(t, "app-settings-backup-restore/file-match.json", real.h.do(get("/app/settings/backup/restore", real.sess)))

	// The first step's other sources (Typecase's notes 97, 99): this
	// computer with a storage for a large archive (s-backup-15), the same
	// refused over the ceiling without one (s-backup-16), a server with the
	// archive's path (s-backup-17).
	docs := app.realAccount(t, "Documents") + "|" + app.realPath(t, "Documents")
	app.compare(t, "app-settings-backup-restore/file.json", app.h.do(get("/app/settings/backup/restore?"+url.Values{"from": {"file"}, "store": {docs}}.Encode(), app.sess)))
	app.compare(t, "app-settings-backup-restore/too-large.json", app.h.do(restoreUpload(t, app, int(app.h.cfg.ImportMaxBytes)+1)))
	app.compare(t, "app-settings-backup-restore/server.json", app.h.do(get("/app/settings/backup/restore?"+url.Values{"from": {"server"}, "store": {docs},
		"path": {"/old-carrel/carrel-personal-2026-05-30.zip"}}.Encode(), app.sess)))

	// A job's page (s-backup-2, Нс61): the form in its rubrics.
	app.compare(t, "app-settings-backup/job.json", app.h.do(get("/app/settings/backup?job="+home, app.sess)))
	// New job (s-backup-19, Typecase's note 99): the same form, nothing
	// chosen, Create.
	app.compare(t, "app-settings-backup/new.json", app.h.do(get("/app/settings/backup/new", app.sess)))

	// The sheet of a job's external trigger over the list (Typecase's note
	// 91): on — the password as a row, the line with it hidden (s-backup-12);
	// off — the price and Turn on (s-backup-14); just issued — the password
	// and the line whole, once (s-backup-13). The frames draw no unlock
	// mark: the window is open only for New password.
	trigger := "/app/settings/backup/trigger?job="
	app.compare(t, "app-settings-backup-trigger/normal.json", app.h.do(get(trigger+work, app.sess)))
	app.compare(t, "app-settings-backup-trigger/off.json", app.h.do(get(trigger+home, app.sess)))
	now := mockupNow(t).Add(mockupSince)
	app.sess.SetUnlockUntil(now.Add(4 * time.Minute))
	if rec := app.h.do(postForm(trigger+work+"&op=new", app.sess, url.Values{})); rec.Code != http.StatusSeeOther {
		t.Fatalf("New password: %d", rec.Code)
	}
	app.sess.SetUnlockUntil(now.Add(-time.Minute))
	app.compare(t, "app-settings-backup-trigger/issued.json", app.h.do(get(trigger+work, app.sess)))
}

// mockupBackups are the jobs of s-backup-1 and their runs, made through the
// service on the clock of the frames: every frame that draws «Backups are
// running» in the rail's foot (s-publish-1) stands on them. It answers the
// IDs of the work job and the home one.
func mockupBackups(t *testing.T, app *mockupApp) (string, string) {
	t.Helper()
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
		DestAccount: app.realAccount(t, "Documents"), DestPath: app.realPath(t, "Documents"), DestFolder: []string{"backup", "work"}, Keep: 14, Encrypted: true, Password: "archive-pw"})
	home := save(sealed.BackupJob{Name: "Личное на домашний диск", Sources: sel("lichnyj", "kontakty", "arhiv", "zapisi", "Documents"), Mode: sealed.BackupToWebDAV,
		DestAccount: app.realAccount(t, "Files"), DestPath: app.realPath(t, "Files"), DestFolder: []string{"carrel-backup"}, Keep: 8, Encrypted: true, Password: "archive-pw", OnSignInDays: 7})
	save(sealed.BackupJob{Name: "Contacts archive, by hand", Sources: sel("arhiv"), Mode: sealed.BackupToDownload, Off: true})
	// The work job's trigger, issued 2 August; its password is named by the
	// frames' hint (s-backup-1, -12), the secret itself being random.
	app.h.users.SetClock(mockupDayAt(t, "2026-08-02 10:00"))
	if _, err := app.h.users.EnableBackupTrigger(app.sess.AccountID(), key, work); err != nil {
		t.Fatal(err)
	}
	const hint = "cbk_…lamp"
	if err := app.h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		for i := range st.Backups {
			if st.Backups[i].JobID == work {
				st.Backups[i].TriggerHint = hint
			}
		}
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	const mb = 1 << 20
	for _, r := range []struct {
		job, at, trigger string
		size             int64
		cols, whole      int
		err              string
		files            int
	}{
		{home, "2026-08-14 09:12", "sign-in", 2100 * mb, 2, 2, "", 1240},
		{work, "2026-08-15 03:10", "external", 182 * mb / 10, 3, 2, "baikal · Общий: the server answered 401, so this collection is not in the archive. The other two were read back.", 0},
		{work, "2026-08-16 03:10", "external", 184 * mb / 10, 3, 3, "", 0},
	} {
		app.h.users.SetClock(mockupDayAt(t, r.at))
		var via []string
		if r.trigger == "external" {
			via = []string{hint, "10.0.0.5"}
		}
		id, err := app.h.users.StartBackupRun(app.sess.AccountID(), r.job, r.trigger, sealed.BackupToWebDAV, func(string) bool { return false }, via...)
		if err != nil {
			t.Fatal(err)
		}
		state := "done"
		if r.whole < r.cols {
			state = "partial"
		}
		if err := app.h.users.FinishBackupRun(app.sess.AccountID(), r.job, id, func(run *store.BackupRun) {
			run.State, run.Size, run.Collections, run.Whole, run.Verified, run.Error = state, r.size, r.cols, r.whole, true, r.err
			run.Files = r.files
			run.File = "carrel-" + r.job + ".zip.age"
		}); err != nil {
			t.Fatal(err)
		}
	}
	// A start the trigger refused, with somebody else's password (s-backup-1).
	app.h.users.SetClock(mockupDayAt(t, "2026-08-15 21:40"))
	app.h.users.NoteRefusedTrigger(work, "cbk_7f3a-91de", "203.0.113.7")
	app.h.users.SetClock(mockupNow(t))
	return work, home
}

// restoreParts is a form of the restore with the parts of an archive from
// this computer, as the library's live layer sends it (Typecase's notes 98,
// 109): the fields, the file's name and size, the parts.
func restoreParts(t *testing.T, app *mockupApp, href string, fields url.Values, archive []byte, parts []backup.Span) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("_csrf", app.sess.CSRFToken())
	for k, vs := range fields {
		for _, v := range vs {
			_ = mw.WriteField(k, v)
		}
	}
	_ = mw.WriteField("file.name", "carrel-backup-2026-08-14.zip")
	_ = mw.WriteField("file.size", strconv.Itoa(len(archive)))
	for _, p := range parts {
		fw, _ := mw.CreateFormFile("file", strconv.FormatInt(p.At, 10)+"-"+strconv.FormatInt(p.Len, 10))
		_, _ = fw.Write(archive[p.At:p.End()])
	}
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, href, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: app.sess.ID()})
	return req
}

// restoreUpload is an archive of n bytes sent from this computer without a
// script and without a storage (s-backup-16).
func restoreUpload(t *testing.T, app *mockupApp, n int) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("_csrf", app.sess.CSRFToken())
	_ = mw.WriteField("store", "")
	fw, _ := mw.CreateFormFile("file", "carrel-personal-2026-08-14.zip")
	_, _ = fw.Write(bytes.Repeat([]byte{'.'}, n))
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/app/settings/backup/restore?from=file", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: app.sess.ID()})
	return req
}
