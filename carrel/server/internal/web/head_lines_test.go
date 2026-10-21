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

// headSub is the line under a frame's title, its marked pieces kept.
func headSub(t *testing.T, id string) string {
	t.Helper()
	m := regexp.MustCompile(`(?s)<div class="tc-head__sub[^"]*">(.*?)</div>`).FindStringSubmatch(mockupFrame(t, id))
	if m == nil {
		t.Fatalf("%s draws no line under the title: read the frame again", id)
	}
	return framePieces(m[1])
}

// Lines under titles in pieces (C-6.2 Runs; plan «Выделенные куски в других
// фразах сервера»): the destination of a job in bold (drawn by s-backup-3
// until it became a fragment of the trigger's sheet, Typecase's note 91;
// s-backup-2 draws a job writing to WebDAV), the archive monospaced
// (s-backup-8), the line of a public file and the day of
// a public note monospaced (s-publish-8…11), the warning of davloom on a
// local address (s-davloom-1).
func TestHeadLinesInPieces(t *testing.T) {
	if got := headSub(t, "s-backup-2"); !strings.HasPrefix(got, "Job · last copy ") {
		t.Fatalf("s-backup-2 = %q: read the frame again", got)
	}
	f := backupFixture(t)
	both := saveJob(t, f, jobForm(f, sealed.BackupToBoth, "8", "archive-pw"))
	page := decode[view.BackupPage](t, f.h.do(get("/app/settings/backup?job="+both.ID, f.sess)))
	if got := saidRuns(page.Subtitle); got != "Job · destination ⟨strong:WebDAV and download⟩ · no copy yet" {
		t.Errorf("job writing to both = %q", got)
	}
	plain := saveJob(t, f, jobForm(f, sealed.BackupToWebDAV, "8", "archive-pw"))
	run := runJob(t, f, plain)
	page = decode[view.BackupPage](t, f.h.do(get("/app/settings/backup?job="+plain.ID, f.sess)))
	if got := saidRuns(page.Subtitle); got != "Job · last copy just now" {
		t.Errorf("job writing to WebDAV = %q", got)
	}

	// The archive opened for a restore (s-backup-8): its job, what the
	// manifest holds, and where it lies, monospaced.
	if got := headSub(t, "s-backup-8"); !regexp.MustCompile(`^Job «.+» · manifest read · \d+ collections · [\d ]+ records · ⟨mono:.+⟩$`).MatchString(got) {
		t.Fatalf("s-backup-8 = %q: read the frame again", got)
	}
	if rec := f.h.do(postForm("/app/settings/backup/restore", f.sess, url.Values{
		"source": {"run"}, "run": {plain.ID + "|" + run.ID}, "password": {"archive-pw"},
	})); rec.Code != http.StatusSeeOther {
		t.Fatalf("open: %d %s", rec.Code, rec.Body.String())
	}
	page = decode[view.BackupPage](t, f.h.do(get("/app/settings/backup", f.sess)))
	want := "Job «Рабочее» · manifest read · 3 collections · 3 records · ⟨mono:" + run.File + "⟩"
	if got := saidRuns(page.Restore.Subtitle); got != want {
		t.Errorf("restore subtitle = %q, want %q", got, want)
	}

	// davloom on a local address warns of it; on a public one it does not.
	h := newHarness(t)
	h.cfg.DAVloomPort = 8081
	h.rebuild()
	h.admin("mix", "password123")
	mix := h.login("mix", "password123")
	if got, frame := saidRuns(decode[view.DavloomPage](t, h.do(get("/app/settings/davloom", mix))).Subtitle), headSub(t, "s-davloom-1"); got != frame {
		t.Errorf("davloom on 127.0.0.1 = %q, the frame %q", got, frame)
	}
	h.cfg.PublicURL = "https://carrel.example.org"
	if got := saidRuns(decode[view.DavloomPage](t, h.do(get("/app/settings/davloom", mix))).Subtitle); got != "One address for every device instead of an account for every server" {
		t.Errorf("davloom on a public address = %q", got)
	}
}

// s-publish-9…11: the line under the name of a public file is one
// monospaced piece — type, size, «changed» and the day; s-publish-8: the
// day of a public note, monospaced.
func TestPublicLinesAreMonospaced(t *testing.T) {
	if got := headSub(t, "s-publish-9"); !regexp.MustCompile(`^⟨mono:[^⟩]+ · [^⟩]+ · changed \d{4}-\d{2}-\d{2}⟩$`).MatchString(got) {
		t.Fatalf("s-publish-9 = %q: read the frame again", got)
	}
	f := pubDocs(t)
	page := publicPage(t, f, publishFile(t, f, "share/readme.txt", false))
	want := "⟨mono:" + string(page.File.Type) + " · " + string(page.File.Size) + " · changed " + string(page.File.Changed) + "⟩"
	if got := saidRuns(page.Lede); got != want {
		t.Errorf("file lede = %q, want %q", got, want)
	}
	g := publishFixture(t)
	note := publicPage(t, g, publishLink(t, g, g.publishHref("notes", searchWork, "n1"), true))
	if got := saidRuns(note.Lede); got != "⟨mono:8 August 2026⟩" {
		t.Errorf("note lede = %q", got)
	}
}

// Messages in pieces: the panel of a subscription says what Check does
// (s-profile-10), the restore that it creates (s-backup-8).
func TestMessagesInPieces(t *testing.T) {
	f := feedHarness(t)
	page := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections?op=feed", f.sess)))
	if got := saidRuns(page.Note); !msgDrawn(frameMsgs(t, "s-profile-10"), got) {
		t.Errorf("subscribe panel note = %q, the frame %q", got, frameMsgs(t, "s-profile-10"))
	}
	if got := saidRuns(decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess))).Note); got != "" {
		t.Errorf("the list without the panel says %q", got)
	}
	b := backupFixture(t)
	restore := decode[view.BackupPage](t, b.h.do(get("/app/settings/backup", b.sess))).Restore
	if got := saidRuns(restore.Note); !strings.HasPrefix(got, "Restoring ⟨strong:creates⟩") {
		t.Errorf("restore note = %q", got)
	}
}
