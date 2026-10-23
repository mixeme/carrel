package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// The primary buttons no fixture carries and no other test reads off its
// frame (plan/server.md, «Primary и Mono там, где их не видит ни фикстура,
// ни проверка кадра»): the bar of a folder, the public file, the sheet of a
// link just made, the device just made, a merge that stopped, the steps of
// an import and of a restore, and the field of Related to typed by hand.

// s-files-1, -2, -4, -15: Upload leads the bar of a folder — read-only too.
func TestFolderBarMarksAreTheFrames(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"a.txt", "alpha")
	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	for _, id := range []string{"s-files-1", "s-files-2", "s-files-4"} {
		sameActionMarks(t, id, "folder bar", page.Bar.Actions, nil)
	}
	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		for i := range b.Collections {
			if b.Collections[i].Path == filesDocs {
				b.Collections[i].ReadOnly = true
			}
		}
	})
	page = decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	sameActionMarks(t, "s-files-15", "read-only folder bar", page.Bar.Actions, nil)
}

// s-publish-9, -11: the one button of a public file is primary; a file
// that cannot be shown carries its size on it — «Download 84.2 MB».
func TestPublicFileDownloadIsTheFrames(t *testing.T) {
	f := pubDocs(t)
	f.srv.SeedFile(filesDocs+"share/cover.psd", strings.Repeat("x", 3000))
	f.srv.SeedFile(filesDocs+"share/board.png", "\x89PNG\r\n\x1a\n")
	shown := publicPage(t, f, publishFile(t, f, "share/board.png", false))
	sameActionMarks(t, "s-publish-9", "a file shown", shown.Actions, nil)

	m := regexp.MustCompile(`<button class="tc-btn[^"]*" type="button">(?:<svg.*?</svg>)?(Download) ([0-9.]+ [KMG]?B)</button>`).FindStringSubmatch(mockupFrame(t, "s-publish-11"))
	if m == nil {
		t.Fatal("s-publish-11 draws no «Download <size>»: read the frame again")
	}
	page := publicPage(t, f, publishFile(t, f, "share/cover.psd", false))
	dl := actionByID(page.Actions, "published.download")
	if page.File == nil || page.File.Preview != view.PreviewNone || dl == nil || string(dl.Label) != m[1]+" "+string(page.File.Size) || !dl.Primary {
		t.Errorf("a file not shown: %+v, download %+v; the frame «%s %s», primary", page.File, dl, m[1], m[2])
	}
}

// s-publish-13: the sheet of a link just made leads with Send by letter.
func TestLinkSheetMarksAreTheFrame(t *testing.T) {
	f, _ := letterFixture(t)
	made := decode[view.PublishSheet](t, f.h.do(postForm("/app/publish?"+url.Values{"k": {"contacts"}, "a": {f.id}, "c": {searchBook}, "uid": {"ada"}}.Encode(), f.sess, url.Values{})))
	sameActionMarks(t, "s-publish-13", "link sheet", made.Actions, nil)
	if len(made.Actions) == 0 || made.Actions[0].ID != "publish.mail" {
		t.Errorf("actions = %+v: the frame leads with Send by letter", made.Actions)
	}
}

// s-davloom-3: Done under the password of a device just made.
func TestDeviceMadeMarksAreTheFrame(t *testing.T) {
	h := newHarness(t)
	h.cfg.DAVloomPort = 8081
	h.rebuild()
	h.admin("alice", "password123")
	alice := h.login("alice", "password123")
	key, _ := alice.DataKey()
	h.putConnections("alice", key, "a1")
	h.do(fragment(postForm("/app/settings/unlock", alice, url.Values{"password": {"password123"}}), "sheet"))
	form := url.Values{"op": {"issue"}, "name": {"Телефон · DAVx5"}, "kind.event": {"on"}, "kind.contact": {"on"}, "col.a1|" + seedBook: {"on"}}
	if rec := h.do(postForm("/app/settings/davloom", alice, form)); rec.Code != http.StatusSeeOther {
		t.Fatalf("issue: %d", rec.Code)
	}
	page := decode[view.DavloomPage](t, h.do(get("/app/settings/davloom", alice)))
	if page.Dialog == nil {
		t.Fatal("no dialog of the password issued")
	}
	sameActionMarks(t, "s-davloom-3", "device made", page.Dialog.Form.Actions, nil)
}

// s-dup-5: a merge that stopped leads to the kept record.
func TestMergeStoppedMarksAreTheFrame(t *testing.T) {
	f := newDupFixture(t)
	f.load(t)
	p := openMerge(t, f.h, f.sess, onlyGroup(t, f.page(t)), nil)
	f.srv.Refuse(http.MethodDelete, http.StatusForbidden, "no")
	f.other.Refuse(http.MethodDelete, http.StatusForbidden, "no")
	got := decode[view.DuplicatesMergePage](t, f.h.do(postForm(mergeAction(t, p, "duplicates.merge.confirm").Href, f.sess, url.Values{})))
	sameActionMarks(t, "s-dup-5", "merge stopped", got.Actions, nil)
}

// s-io-3, -5, -6, -9: the import's steps — Preview file, Import N <kind>,
// Back to list — each its step's primary button, in the frame's words.
func TestImportStepsAreTheFrames(t *testing.T) {
	f := importFixture(t)
	page := decode[view.ImportPage](t, f.h.do(get(f.path("contacts")+"/import", f.sess)))
	sameActionMarks(t, "s-io-3", "source", page.Actions, nil)
	if len(page.Actions) != 1 {
		t.Errorf("source: %+v — the frame draws one button, the rail is the way out", page.Actions)
	}
	page = decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, "cards.vcf", []byte(importedCards))))
	run := actionByID(page.Actions, "import.run")
	shape := regexp.MustCompile(`^Import \d+ contacts?$`)
	if run == nil || !shape.MatchString(string(run.Label)) || !run.Primary {
		t.Fatalf("preview: run = %+v, the frame «Import 122 contacts», primary (s-io-5)", run)
	}
	buttons, _ := frameMarks(t, "s-io-5")
	if on, ok := oneWay(buttons["Cancel"]); !ok || on {
		t.Fatal("s-io-5 draws no plain Cancel: read the frame again")
	}
	if c := actionByID(page.Actions, "import.cancel"); c == nil || c.Primary {
		t.Errorf("preview: cancel = %+v", c)
	}
	rec := f.h.do(postForm(run.Href, f.sess, url.Values{}))
	if rec.Code == http.StatusSeeOther {
		rec = f.h.do(get(rec.Header().Get("Location"), f.sess))
	}
	report := decode[view.ImportPage](t, rec)
	if report.Step != view.ImportStepReport {
		t.Fatalf("report step = %q", report.Step)
	}
	sameActionMarks(t, "s-io-9", "report", report.Actions, nil)
	if len(report.Actions) != 1 {
		t.Errorf("report: %+v — the frame draws Back to list alone", report.Actions)
	}
}

// s-backup-7, -8: Open the manifest, then Restore N collections.
func TestRestoreStepsAreTheFrames(t *testing.T) {
	f := backupFixture(t)
	job := saveJob(t, f, jobForm(f, sealed.BackupToWebDAV, "8", "archive-pw"))
	run := runJob(t, f, job)
	page := decode[view.BackupPage](t, f.h.do(get("/app/settings/backup", f.sess)))
	sameActionMarks(t, "s-backup-7", "restore source", page.Restore.Source.Actions, nil)
	if rec := f.h.do(postForm("/app/settings/backup/restore", f.sess, url.Values{
		"source": {"run"}, "run": {job.ID + "|" + run.ID}, "password": {"archive-pw"},
	})); rec.Code != http.StatusSeeOther {
		t.Fatalf("open: %d", rec.Code)
	}
	page = decode[view.BackupPage](t, f.h.do(get("/app/settings/backup", f.sess)))
	actions := append(append([]view.Action{}, page.Restore.Actions...), page.Restore.Match.Form.Actions...)
	r := actionByID(actions, "restore.run")
	if r == nil || string(r.Label) != "Restore 3 collections" || !r.Primary {
		t.Errorf("restore = %+v, the frame «Restore 3 collections», primary (s-backup-8)", r)
	}
	sameActionMarks(t, "s-backup-8", "restore", actions, nil)
}

// s-tasks-11: Related to typed by hand is a UID, monospaced.
func TestRelatedToTypedIsMonospaced(t *testing.T) {
	f := calendarFixture(t, nil,
		"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:One\nEND:VTODO"))
	form := decode[view.TaskEditPage](t, f.h.do(get(f.path("tasks")+"/t1/edit", f.sess))).Form
	related := rowNamed(t, form, "related")
	sameActionMarks(t, "s-tasks-11", "related", nil, []view.FormRow{related})
}
