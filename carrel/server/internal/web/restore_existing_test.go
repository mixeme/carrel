package web_test

import (
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

var (
	frameRestoreQuestion = regexp.MustCompile(`(?s)<div class="tc-field is-own-line"><label>(.*?)</label>(.*?)<span class="tc-field__hint">(.*?)</span>`)
	frameSegLink         = regexp.MustCompile(`<a class="(is-on )?tc-seg__opt" href="([^"]+)">([^<]+)</a>`)
)

// Note 135, the owner's decision of 26.10.2026: what of the archive is
// already here and differs is the import's question — Skip them without an
// answer, Replace them, Import as copies — asked over the bar of the
// matching in the frame's words (s-backup-8); the row says how many after
// what it does, and the report what became of them (s-backup-18).
func TestRestoreAsksAboutWhatIsAlreadyHere(t *testing.T) {
	frame := mockupFrame(t, "s-backup-8")
	q := frameRestoreQuestion.FindStringSubmatch(frame)
	if q == nil {
		t.Fatal("s-backup-8 asks nothing about what is already here: read the frame again")
	}
	label := regexp.MustCompile(`^\d+ `).ReplaceAllString(frameText(q[1]), "2 ")
	var frameOpts [][]string
	for _, m := range frameSegLink.FindAllStringSubmatch(q[2], -1) {
		frameOpts = append(frameOpts, m)
	}
	if len(frameOpts) != 3 || frameOpts[0][1] == "" || !strings.HasPrefix(frameText(q[1]), "5 records of the archive") {
		t.Fatalf("s-backup-8 asks %q with %q: read the frame again", q[1], frameOpts)
	}
	report := mockupFrame(t, "s-backup-18")
	if !strings.Contains(report, "already here — skipped") || !strings.Contains(report, "already here, skipped") {
		t.Fatal("s-backup-18 does not say what was skipped: read the frame again")
	}

	open := func(t *testing.T) (davFixture, view.RestoreMatch) {
		f := backupFixture(t)
		return f, openArchive(t, f, func() {
			f.srv.Seed(searchWork+"e1.ics", ics("BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260801T000000Z\nSUMMARY:Standup moved\nDTSTART:20260810T100000Z\nEND:VEVENT"))
			f.srv.Seed(searchWork+"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260801T000000Z\nSUMMARY:Pack twice\nEND:VTODO"))
		})
	}
	answer := func(t *testing.T, f davFixture, m view.RestoreMatch, id string) view.RestoreMatch {
		t.Helper()
		a := actionByID(m.Existing.Options, "restore.existing."+id)
		if a == nil || a.Method != "GET" {
			t.Fatalf("no answer %s in %+v", id, m.Existing.Options)
		}
		return *decode[view.BackupPage](t, f.h.do(get(a.Href, f.sess))).Restore.Match
	}

	f, m := open(t)
	e := m.Existing
	if e == nil || string(e.Note) != label {
		t.Fatalf("the question = %+v, the frame %q", e, label)
	}
	for i, o := range e.Options {
		if string(o.Label) != frameOpts[i][3] || o.Href != frameOpts[i][2] || o.On != (frameOpts[i][1] != "") {
			t.Errorf("answer %d = %q %q on %v, the frame %q", i, o.Label, o.Href, o.On, frameOpts[i][1:])
		}
	}
	if string(e.Hint) != frameText(q[3]) {
		t.Errorf("hint = %q, the frame %q", e.Hint, frameText(q[3]))
	}
	if r := m.Rows[1]; r.Here != 2 || r.Will != "matched by path" {
		t.Errorf("the calendar's row: %q · %d already here", r.Will, r.Here)
	}
	if m.Rows[0].Here != 0 {
		t.Errorf("the book holds nothing changed, its row says %d already here", m.Rows[0].Here)
	}
	before := len(f.srv.Paths(searchWork))

	// Replace them: the archive's version over the one here.
	m = answer(t, f, m, "replace")
	if !m.Existing.Options[1].On || string(m.Existing.Hint) != "The ones here are overwritten with the archive's version." {
		t.Errorf("after Replace them: %+v", m.Existing)
	}
	rep := restoreRun(t, f, actionByID(m.Form.Actions, "restore.run").Href, matchForm(m))
	if body, _, _ := f.srv.Object(searchWork + "e1.ics"); strings.Contains(body, "Standup moved") {
		t.Errorf("replaced, the event is still the one here: %s", body)
	}
	if n := len(f.srv.Paths(searchWork)); n != before {
		t.Errorf("replaced, the calendar holds %d, want %d", n, before)
	}
	if lines := reportLines(rep); !strings.Contains(lines, "2 already here — replaced") || !strings.Contains(string(rep.Subtitle), "2 already here, replaced") {
		t.Errorf("the report of Replace them:\n%s\n%s", rep.Subtitle, lines)
	}

	// Import as copies: beside the ones here, under new UIDs.
	f, m = open(t)
	m = answer(t, f, m, "copy")
	rep = restoreRun(t, f, actionByID(m.Form.Actions, "restore.run").Href, matchForm(m))
	if body, _, _ := f.srv.Object(searchWork + "e1.ics"); !strings.Contains(body, "Standup moved") {
		t.Errorf("copied, the one here was overwritten: %s", body)
	}
	if n := len(f.srv.Paths(searchWork)); n != before+2 {
		t.Errorf("copied, the calendar holds %d, want %d", n, before+2)
	}
	if lines := reportLines(rep); !strings.Contains(lines, "2 already here — imported as copies") {
		t.Errorf("the report of Import as copies:\n%s\n%s", rep.Subtitle, lines)
	}
}
