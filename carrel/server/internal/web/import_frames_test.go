package web_test

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// importSteps are the words of the steps line every frame of the wizard
// draws (s-io-3…9), in their order.
func importSteps(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, m := range regexp.MustCompile(`tc-steps__item">([^<]+)<`).FindAllStringSubmatch(mockupFrame(t, "s-io-3"), -1) {
		out = append(out, m[1])
	}
	if len(out) != 3 {
		t.Fatalf("s-io-3 draws steps %q: read the frame again", out)
	}
	return out
}

func importCounts(p view.ImportPage) string {
	var parts []string
	for _, c := range p.Counts {
		s := strings.TrimSpace(string(c.Label))
		s = strconv.Itoa(c.Count) + " " + s
		if c.Error {
			s += "!"
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}

// The wizard by its frames (s-io-3…9): the steps line, the file in the head
// of the preview — name monospaced, size, format —, the counts over the
// table, the report in numbers with the identities that changed, and the
// lines under the source and the first step's message in the frames' words.
func TestImportWizardByFrames(t *testing.T) {
	steps := importSteps(t)
	f := importFixture(t)
	page := decode[view.ImportPage](t, f.h.do(get(f.path("contacts")+"/import", f.sess)))
	if got := texts(page.Steps); strings.Join(got, "|") != strings.Join(steps, "|") {
		t.Errorf("steps = %q, the frame %q", got, steps)
	}
	if len(page.Subtitle) != 0 {
		t.Errorf("the first step has a line under the title: %q", saidRuns(page.Subtitle))
	}
	hint := regexp.MustCompile(`(?s)<div class="tc-field__hint tc-field__hint--top8">(.*?)</div>`).FindStringSubmatch(mockupFrame(t, "s-io-3"))
	if hint == nil {
		t.Fatal("s-io-3 draws no line under the file: read the frame again")
	}
	if got := saidRuns(page.Source.Hint); got != framePieces(hint[1]) {
		t.Errorf("hint = %q, the frame %q", got, framePieces(hint[1]))
	}
	if got := saidRuns(page.Source.Note); !msgDrawn(frameMsgs(t, "s-io-3"), got) {
		t.Errorf("note = %q, the frame %q", got, frameMsgs(t, "s-io-3"))
	}

	page = decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, "Разное (mix).vcf", []byte(importedCards))))
	page = asCopies(t, f, page)
	if got := saidRuns(page.Subtitle); got != "⟨mono:Разное (mix).vcf⟩ · "+strconv.Itoa(len(importedCards))+" B · vCard 3.0" {
		t.Errorf("preview subtitle = %q", got)
	}
	if got := importCounts(page); got != "2 ready, 1 already here" {
		t.Errorf("counts = %q", got)
	}

	page = decode[view.ImportPage](t, f.h.do(postForm(actionHref(t, page.Actions, "import.run"), f.sess, url.Values{})))
	if page.Step != view.ImportStepReport || page.Report == nil {
		t.Fatalf("step = %q", page.Step)
	}
	if got := saidRuns(page.Subtitle); !strings.HasSuffix(got, " · People") {
		t.Errorf("report subtitle = %q, want where it went", got)
	}
	r := page.Report
	if r.Created != 2 || r.Failed != 0 || len(r.NewUIDs) != 1 || !strings.Contains(string(r.NewUIDs[0]), " · ada → ") ||
		!strings.HasSuffix(string(r.NewUIDs[0]), " · imported as a copy") || len(r.Failures) != 0 {
		t.Errorf("report = created %d, failed %d, new UIDs %q, failures %q", r.Created, r.Failed, r.NewUIDs, r.Failures)
	}
}

// A calendar file: the format is iCalendar's, and what the calendar does not
// take is counted as skipped (s-io-6).
func TestImportCountsWhatItSkips(t *testing.T) {
	f := calendarFixture(t, nil)
	page := decode[view.ImportPage](t, f.h.do(get(f.path("calendar")+"/import", f.sess)))
	page = decode[view.ImportPage](t, f.h.do(importRequest(actionHref(t, page.Actions, "import.choose"), f.sess, false, "Личный 2026.ics", []byte(importedCalendar))))
	if got := saidRuns(page.Subtitle); !strings.HasSuffix(got, " · iCalendar 2.0") || !strings.HasPrefix(got, "⟨mono:Личный 2026.ics⟩ · ") {
		t.Errorf("subtitle = %q", got)
	}
	if got := importCounts(page); got != "1 ready, 1 skipped" {
		t.Errorf("counts = %q", got)
	}
}

// Notes: the folder is chosen in the tree, and «Preview folder» under it
// takes it (s-io-4); the first step's message marks X- (s-io-4). What the
// folder holds that is no Markdown is counted as skipped, a file that cannot
// be read as one with errors (s-io-8).
func TestImportNotesFolderByFrames(t *testing.T) {
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	notebook := davtest.CalendarHome + "notes/"
	srv.AddCalendar(caldav.Calendar{Path: notebook, Name: "Notebook", SupportedComponentSet: []string{"VJOURNAL"}})
	srv.SeedFolder(davtest.FileRoot + "docs/")
	srv.SeedFile(davtest.FileRoot+"docs/plain.md", "# Уголь\n\nЗаказать мешок.\n")
	srv.SeedFile(davtest.FileRoot+"docs/skan.pdf", "not a note")
	f := connectFixture(t, srv, notebook, nil)
	addFileCollection(t, f, davtest.FileRoot+"docs/", "Docs")

	page := decode[view.ImportPage](t, f.h.do(get(f.path("notes")+"/import", f.sess)))
	if got := saidRuns(page.Source.Note); !msgDrawn(frameMsgs(t, "s-io-4"), got) {
		t.Errorf("note = %q, the frame %q", got, frameMsgs(t, "s-io-4"))
	}
	if page.Source.Folder == nil || len(page.Source.Folder.Nodes) != 1 || page.Source.Folder.Chosen != "" {
		t.Fatalf("folder = %+v", page.Source.Folder)
	}
	preview := page.Source.Preview
	if preview == nil || preview.Label != "Preview folder" || preview.Enabled || preview.Reason == "" {
		t.Fatalf("Preview folder before a folder is chosen: %+v", preview)
	}
	page = decode[view.ImportPage](t, f.h.do(get(page.Source.Folder.Nodes[0].ID, f.sess)))
	if page.Step != view.ImportStepSource || page.Source.Folder.Chosen != page.Source.Folder.Nodes[0].ID {
		t.Fatalf("choosing a folder: step %q, chosen %q", page.Step, page.Source.Folder.Chosen)
	}
	preview = page.Source.Preview
	if preview == nil || !preview.Enabled || !preview.Primary || preview.Method != "POST" {
		t.Fatalf("Preview folder with a folder chosen: %+v", preview)
	}
	page = decode[view.ImportPage](t, f.h.do(postForm(preview.Href, f.sess, url.Values{})))
	if page.Step != view.ImportStepPreview {
		t.Fatalf("step = %q notices = %+v", page.Step, page.Notices)
	}
	if got := saidRuns(page.Subtitle); !strings.HasPrefix(got, "⟨mono:Docs⟩ · ") || !strings.HasSuffix(got, " · 2 files") {
		t.Errorf("subtitle = %q", got)
	}
	if got := importCounts(page); got != "1 ready, 1 skipped" {
		t.Errorf("counts = %q", got)
	}
}

// A folder with no Markdown says so after the preview's own message: the
// two are one sentence in pieces, neither lost.
func TestImportFolderWithoutMarkdownSaysSo(t *testing.T) {
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	notebook := davtest.CalendarHome + "notes/"
	srv.AddCalendar(caldav.Calendar{Path: notebook, Name: "Notebook", SupportedComponentSet: []string{"VJOURNAL"}})
	srv.SeedFolder(davtest.FileRoot + "docs/")
	srv.SeedFile(davtest.FileRoot+"docs/skan.pdf", "not a note")
	f := connectFixture(t, srv, notebook, nil)
	addFileCollection(t, f, davtest.FileRoot+"docs/", "Docs")
	page := decode[view.ImportPage](t, f.h.do(get(f.path("notes")+"/import", f.sess)))
	page = decode[view.ImportPage](t, f.h.do(get(page.Source.Folder.Nodes[0].ID, f.sess)))
	page = decode[view.ImportPage](t, f.h.do(postForm(page.Source.Preview.Href, f.sess, url.Values{})))
	got := saidRuns(page.Source.Note)
	if !strings.HasPrefix(got, "Markdown is stored as it was written") || !strings.HasSuffix(got, "The folder holds no Markdown file.") {
		t.Errorf("note = %q", got)
	}
}
