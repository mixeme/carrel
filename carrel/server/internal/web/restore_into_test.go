package web_test

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// openArchive runs a job of the backup fixture and opens its archive for
// restoring; what changes on the server before it is opened is changed.
func openArchive(t *testing.T, f davFixture, change func()) view.RestoreMatch {
	t.Helper()
	job := saveJob(t, f, jobForm(f, sealed.BackupToWebDAV, "8", "archive-pw"))
	run := runJob(t, f, job)
	if run.State != "done" {
		t.Fatalf("run = %+v", run)
	}
	if change != nil {
		change()
	}
	if rec := f.h.do(postForm("/app/settings/backup/restore", f.sess, url.Values{
		"source": {"run"}, "run": {job.ID + "|" + run.ID}, "password": {"archive-pw"},
	})); rec.Code != http.StatusSeeOther {
		t.Fatalf("open: %d %s", rec.Code, rec.Body.String())
	}
	page := decode[view.BackupPage](t, f.h.do(get("/app/settings/backup", f.sess)))
	if page.Restore.Match == nil {
		t.Fatalf("no matching after the archive was opened: %+v", page.Restore)
	}
	return *page.Restore.Match
}

// matchForm is what the matching form sends as it stands: every row's
// choice, the rows ticked.
func matchForm(m view.RestoreMatch) url.Values {
	v := url.Values{}
	for _, r := range m.Rows {
		v.Set(r.Into.Name, firstSelected(r.Into))
		if r.Pick.Value == "true" {
			v.Set(r.Pick.Name, "on")
		}
	}
	return v
}

func firstSelected(r view.FormRow) string {
	for _, o := range r.Options {
		if o.Selected {
			return o.Value
		}
	}
	return ""
}

func davDelete(t *testing.T, f davFixture, p string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodDelete, f.srv.URL+p, nil)
	req.SetBasicAuth(f.srv.Login, f.srv.Password)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode >= 300 {
		t.Fatalf("DELETE %s: %v %v", p, err, resp)
	}
	resp.Body.Close()
}

// The lead's decision on s-backup-8: the frame rules, restoring is an
// import, not a rollback. Each collection of the archive gets its target in
// its row, preset to the collection matched by path; what is missing there
// is created, the same record (same UID, same content) is not made twice,
// a UID held by a record changed since the copy gets a fresh UID and a line
// in the report; nothing on the server is deleted or overwritten. The rows
// say what will happen before anything is written.
func TestRestoreIntoTheCollectionsMatchedByPath(t *testing.T) {
	f := backupFixture(t)
	m := openArchive(t, f, func() {
		davDelete(t, f, searchBook+"ada.vcf")
		f.srv.Seed(searchWork+"e1.ics", ics("BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260801T000000Z\nSUMMARY:Standup moved\nDTSTART:20260810T100000Z\nEND:VEVENT"))
	})
	if len(m.Rows) != 3 || m.Selected != 3 {
		t.Fatalf("rows %d, selected %d", len(m.Rows), m.Selected)
	}
	want := []struct{ into, will string }{
		{sel(f, searchBook), "matched by path — 1 record missing, will be created"},
		{sel(f, searchWork), "matched by path — 1 changed since the copy, created again with a fresh UID"},
		{sel(f, filesDocs), "matched by path — missing files are uploaded, none is overwritten"},
	}
	for i, r := range m.Rows {
		if got := firstSelected(r.Into); got != want[i].into || string(r.Will) != want[i].will || r.Danger || r.Pick.Value != "true" ||
			r.Into.Name != "into."+strconv.Itoa(i) || r.Pick.Name != "pick."+strconv.Itoa(i) {
			t.Errorf("row %d (%s): into %q, will %q, danger %v, pick %+v; want %q, %q", i, r.Name, got, r.Will, r.Danger, r.Pick, want[i].into, want[i].will)
		}
	}
	last := m.Rows[0].Into.Options
	if n := len(last); n < 3 || last[n-2].Value != "new" || !last[n-2].Separator || last[n-1].Value != "skip" || last[n-1].Label != "Skip this one" {
		t.Errorf("the choices of a row end %+v, want New collection after a line, then Skip this one", last)
	}
	run := actionByID(m.Form.Actions, "restore.run")
	if run == nil || run.Label != "Restore 3 collections" || !run.Primary {
		t.Fatalf("run = %+v", run)
	}

	before := len(f.srv.Paths(searchWork))
	rep := restoreRun(t, f, run.Href, matchForm(m))
	if _, _, ok := f.srv.Object(searchBook + "ada.vcf"); !ok {
		t.Errorf("the missing card was not created again under its name: %v", f.srv.Paths(searchBook))
	}
	if body, _, _ := f.srv.Object(searchWork + "e1.ics"); !strings.Contains(body, "Standup moved") {
		t.Errorf("the changed event was overwritten: %s", body)
	}
	if n := len(f.srv.Paths(searchWork)); n != before+1 {
		t.Errorf("the calendar holds %d, want %d: the changed event once more, the task not twice", n, before+1)
	}
	// The report (s-backup-18): the fresh UID counted, the same records
	// not repeated.
	if lines := reportLines(rep); !strings.Contains(lines, "1 record with a new UID") || !strings.Contains(string(rep.Subtitle), "1 with a new UID") {
		t.Errorf("the report does not name the fresh UID:\n%s\n%s", rep.Subtitle, lines)
	}
	if body, ok := f.srv.File(filesDocs + "share/readme.txt"); !ok || body != "hello" {
		t.Errorf("the file there = %q %v", body, ok)
	}
	if names := f.srv.Names(filesDocs + "share/"); len(names) != 1 {
		t.Errorf("the same file was made twice: %v", names)
	}
	key, _ := f.sess.DataKey()
	if blob, _ := f.h.users.Catalog(f.sess.AccountID(), key); len(blob.Collections) != 3 {
		t.Errorf("a collection was made: %+v", blob.Collections)
	}
}

// The bar of the selection (s-backup-8): «Restore into» gives every ticked
// row the target chosen there, Skip skips the ticked ones, Select all ticks
// every row; without scripts the same form goes to its own address and the
// page comes back with the choice made.
func TestRestoreBarChoosesForTheTickedRows(t *testing.T) {
	f := backupFixture(t)
	m := openArchive(t, f, nil)
	bar := func(id string, form url.Values) view.RestoreMatch {
		t.Helper()
		a := actionByID(m.Form.Actions, id)
		if a == nil {
			t.Fatalf("no %s in %+v", id, m.Form.Actions)
		}
		rec := f.h.do(postForm(a.Href, f.sess, form))
		if rec.Code == http.StatusSeeOther {
			rec = f.h.do(get(rec.Header().Get("Location"), f.sess))
		}
		return *decode[view.BackupPage](t, rec).Restore.Match
	}
	form := matchForm(m)
	form.Del("pick.0")
	form.Del("pick.2")
	got := bar("restore.skip", form)
	if firstSelected(got.Rows[1].Into) != "skip" || got.Rows[1].Pick.Value == "true" || got.Rows[1].Will != "skip" || firstSelected(got.Rows[0].Into) != sel(f, searchBook) || got.Selected != 0 {
		t.Errorf("after Skip of the second: %+v / %+v, selected %d", got.Rows[1], got.Rows[0].Into, got.Selected)
	}
	got = bar("restore.all", matchForm(got))
	if got.Selected != 3 {
		t.Errorf("Select all: %d selected", got.Selected)
	}
	form = matchForm(got)
	form.Set("into", "new")
	got = bar("restore.into", form)
	for i, r := range got.Rows {
		if firstSelected(r.Into) != "new" || !strings.HasPrefix(string(r.Will), "a new ") {
			t.Errorf("row %d after Restore into New collection: %q, %q", i, firstSelected(r.Into), r.Will)
		}
	}
	if run := actionByID(got.Form.Actions, "restore.run"); run == nil || run.Label != "Restore 3 collections" {
		t.Errorf("run = %+v", run)
	}
}
