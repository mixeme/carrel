package web_test

import (
	"net/url"
	"regexp"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Typecase's notes 166 and 167 (Р-77, the owner 04.11.2026): the hints say
// only what the screen does not. Of the frames they change, Carrel draws the
// Apple profile (s-plan-1, -6) and the backup job (s-backup-2, -19); every
// string below is cut out of the frame.

var (
	frameFieldHint = regexp.MustCompile(`(?s)<span class="tc-field__hint[^"]*">(.*?)</span>`)
	frameAside     = regexp.MustCompile(`(?s)<span class="tc-msg__aside">(.*?)</span>\s*</div>`)
	frameModeMsg   = regexp.MustCompile(`(?s)<div class="tc-msg tc-grow tc-msg--flush-top4[^"]*">(.*?)</div>`)
	// frameText puts a space where a tag closed: «<b>two steps</b>.».
	frameBeforeMark = regexp.MustCompile(` ([.,:;])`)
)

// frameSaid is frameAll as the words read: no space before a mark.
func frameSaid(re *regexp.Regexp, s string) []string {
	out := frameAll(re, s, 1)
	for i := range out {
		out[i] = frameBeforeMark.ReplaceAllString(out[i], "$1")
	}
	return out
}

func noticeSays(n view.Notice) string {
	out := string(n.Title)
	if n.Body != "" {
		out += " " + string(n.Body)
	}
	return out
}

func runsSay(r view.Runs) string {
	var out string
	for _, p := range r {
		out += string(p.Text)
	}
	return out
}

func TestTheProfileSaysWhatTheFrameSays(t *testing.T) {
	f, _ := deviceFixture(t)
	locked := devicesPage(t, f, "apple")
	frame6 := mockupFrame(t, "s-plan-6")
	if want := frameSaid(frameAside, frame6); len(want) != 1 || noticeSays(locked.Warning) != want[0] {
		t.Errorf("s-plan-6 alert = %q, want %q", noticeSays(locked.Warning), want)
	}
	if want := frameSaid(frameFieldHint, frame6); len(want) != 1 || locked.Note != text(want[0]) {
		t.Errorf("s-plan-6 note = %q, want %q", locked.Note, want)
	}

	unlock(t, f)
	issued := decode[view.DevicesPage](t, f.h.do(postForm(deviceAction(t, devicesPage(t, f, "apple").Actions, "device.profile").Href, f.sess, url.Values{})))
	frame1 := mockupFrame(t, "s-plan-1")
	if want := frameSaid(frameAside, frame1); len(want) != 1 || noticeSays(issued.Warning) != want[0] {
		t.Errorf("s-plan-1 alert = %q, want %q", noticeSays(issued.Warning), want)
	}
	// Inside the window Generate profile has no line (the badge in the
	// head says it); the one hint of s-plan-1 is the link's.
	if issued.Note != "" {
		t.Errorf("s-plan-1 note = %q, want none", issued.Note)
	}
	if issued.Link == nil {
		t.Fatal("no link card after Generate profile")
	}
	if want := frameSaid(frameFieldHint, frame1); len(want) != 1 || issued.Link.Hint != text(want[0]) {
		t.Errorf("s-plan-1 link hint = %q, want %q", issued.Link.Hint, want)
	}
}

func TestTheBackupJobSaysWhatTheFrameSays(t *testing.T) {
	f := backupFixture(t)
	page := decode[view.BackupPage](t, f.h.do(get("/app/settings/backup/new", f.sess)))
	if len(page.Add.Sets) == 0 {
		t.Fatal("no job form")
	}
	said := map[string]string{}
	for _, set := range page.Add.Sets {
		for _, r := range set.Rows {
			switch r.Name {
			case "mode", "keep":
				said[r.Name] = runsSay(r.HintRuns)
			case "encrypted":
				said[r.Name] = string(r.Hint)
			}
		}
	}
	for _, id := range []string{"s-backup-2", "s-backup-19"} {
		frame := mockupFrame(t, id)
		hints := frameSaid(frameFieldHint, frame)
		if len(hints) < 2 || said["keep"] != hints[0] || said["encrypted"] != hints[1] {
			t.Errorf("%s: keep %q, encrypt %q; the frame's %q", id, said["keep"], said["encrypted"], hints)
		}
		msgs := frameSaid(frameModeMsg, frame)
		if len(msgs) < 2 || said["mode"] != msgs[1] {
			t.Errorf("%s: where to %q; the frame's %q", id, said["mode"], msgs)
		}
	}
}
