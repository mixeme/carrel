package web_test

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// from-typecase note 32, the refused sources (the owner's decision of
// 02.10.2026: the fight with the length of a line is lost). A source that did
// not answer is only an error mark in the rail, the whole sentence its hint,
// no words beside the name and no Retry there. The poll bar does not name
// it either: its red «N failed» opens the menu of refusals — each source
// with its mark, what was asked and what the server answered in full, its
// Retry, and Retry all under them when there are more than one (s-agenda-1,
// states-16).

var (
	frameRefusal   = regexp.MustCompile(`<div class="tc-menu__refusal"><span class="tc-mark[^"]*" data-tc-colour="([^"]+)"></span><span class="tc-menu__what"><span class="tc-menu__source">([^<]*)</span><span class="tc-menu__answer">([^<]*)</span></span><button[^>]*>Retry</button></div>`)
	frameRetryAll  = regexp.MustCompile(`<div class="tc-menu__foot"><button[^>]*>Retry all</button></div>`)
	frameFailedBar = regexp.MustCompile(`(?s)<div class="tc-runbar">(.*?)<details class="tc-drop"><summary class="tc-btn is-sm is-danger">(\d+ failed)`)
	frameErrorRow  = regexp.MustCompile(`<div class="tc-tickrow is-error">(.*?)</div>`)
	frameErrorMark = regexp.MustCompile(`<span class="tc-tickrow__label">([^<]*)</span><span class="tc-tickrow__err" role="img" aria-label="([^"]*)" title="([^"]*)">`)
)

type frameRefused struct{ colour, source, answer string }

func frameRefusals(t *testing.T, id string) ([]frameRefused, bool) {
	t.Helper()
	frame := mockupFrame(t, id)
	var out []frameRefused
	for _, m := range frameRefusal.FindAllStringSubmatch(frame, -1) {
		out = append(out, frameRefused{strings.ToLower(m[1]), frameText(m[2]), frameText(m[3])})
	}
	return out, frameRetryAll.MatchString(frame)
}

// answered is the part of a refusal after what was asked: what the server
// said.
func answered(s string) string {
	_, after, _ := strings.Cut(s, " → ")
	return after
}

func TestRefusalsAreTheMenuOfTheBar(t *testing.T) {
	agenda := mockupFrame(t, "s-agenda-1")
	bar := frameFailedBar.FindStringSubmatch(agenda)
	one, oneAll := frameRefusals(t, "s-agenda-1")
	if bar == nil || bar[2] != "1 failed" || len(one) != 1 || oneAll {
		t.Fatalf("s-agenda-1 draws no «1 failed» with one refusal and no Retry all: read the frame again")
	}
	said := strings.TrimSuffix(strings.TrimSpace(frameText(regexp.MustCompile(`<[^>]+>`).ReplaceAllString(bar[1], ""))), " ·")
	pollLine := regexp.MustCompile(`^Polled \d+ of \d+ sources · \d+ events$`)
	if !pollLine.MatchString(said) {
		t.Fatalf("s-agenda-1 says %q before «1 failed»: read the frame again", said)
	}
	var railRows [][]string
	for _, m := range frameErrorRow.FindAllStringSubmatch(agenda, -1) {
		if strings.Contains(m[1], "tc-tickrow__meta") || strings.Contains(m[1], ">Retry<") {
			t.Fatalf("s-agenda-1 still draws words or a Retry by a refused source: read the frame again")
		}
		if mark := frameErrorMark.FindStringSubmatch(m[1]); mark != nil && mark[2] == mark[3] {
			railRows = append(railRows, mark)
		}
	}
	three, threeAll := frameRefusals(t, "states-16")
	if len(railRows) != 1 || len(three) != 3 || !threeAll {
		t.Fatalf("s-agenda-1 rail %q, states-16 %q (Retry all %v): read the frames again", railRows, three, threeAll)
	}

	app := newMockupApp(t)
	poll := func(p view.AgendaPage) *view.PollBar { return p.Poll }
	asked := regexp.MustCompile(`^REPORT /\S+ → `)
	check := func(what string, page view.AgendaPage, want []frameRefused, retryAll bool) {
		t.Helper()
		got := page.Poll
		if got == nil {
			t.Fatalf("%s: no poll bar", what)
		}
		if s := saidRuns(got.Summary); !pollLine.MatchString(s) {
			t.Errorf("%s: the bar says %q, the frame only %q and the button", what, s, said)
		}
		for _, r := range got.Summary {
			if r.Mark == view.RunDanger {
				t.Errorf("%s: the bar still names a refusal: %+v", what, got.Summary)
			}
		}
		if string(got.Failed) != strconv.Itoa(len(want))+strings.TrimPrefix(bar[2], "1") {
			t.Errorf("%s: failed %q, want %d failed", what, got.Failed, len(want))
		}
		if (got.RetryAll != nil) != retryAll || retryAll && (got.RetryAll.Label != "Retry all" || got.RetryAll.Method != "POST" ||
			!strings.HasSuffix(got.RetryAll.Href, "/app/search/progress/"+got.TaskID+"/retry")) {
			t.Errorf("%s: retry all = %+v, want one: %v", what, got.RetryAll, retryAll)
		}
		n := 0
		for _, sp := range got.Sources {
			if sp.Answer == "" {
				continue
			}
			n++
			i := -1
			for j, w := range want {
				if string(sp.Stamp) == w.source {
					i = j
				}
			}
			if i < 0 {
				t.Errorf("%s: %q refused, the frame does not draw it: %+v", what, sp.Stamp, sp)
				continue
			}
			if !asked.MatchString(string(sp.Answer)) || answered(string(sp.Answer)) != answered(want[i].answer) ||
				strings.ToLower(string(sp.Colour)) != want[i].colour || sp.Retry == nil || sp.Retry.Label != "Retry" {
				t.Errorf("%s: %q = %q %s, retry %+v; the frame %+v", what, sp.Stamp, sp.Answer, sp.Colour, sp.Retry, want[i])
			}
		}
		if n != len(want) {
			t.Errorf("%s: %d refusals, the frame %d", what, n, len(want))
		}
		// The rail marks the refused source, its reason in the hint.
		_, general := railRow(t, page.Rail, "Общий")
		if general.State != view.CollectionUnavailable || string(general.Detail) != frameText(railRows[0][2]) {
			t.Errorf("%s: Общий in the rail = %+v, the frame's hint %q", what, general, railRows[0][2])
		}
	}

	// baikal refuses the password (s-agenda-1).
	page := combined(t, app.h, app.sess, "/app/calendar?from=2026-08-10", poll)
	page = decode[view.AgendaPage](t, app.h.do(get("/app/calendar?from=2026-08-10&task="+page.Poll.TaskID, app.sess)))
	check("baikal", page, one, false)

	// And radicale is busy: three calendars (states-16).
	app.servers["radicale · vps"].Refuse("REPORT", 503, "the server is busy")
	page = combined(t, app.h, app.sess, "/app/calendar?from=2026-08-10&again=1", poll)
	task := page.Poll.TaskID
	page = decode[view.AgendaPage](t, app.h.do(get("/app/calendar?from=2026-08-10&task="+task, app.sess)))
	check("baikal and radicale", page, three, true)

	// Retry all asks every refused source again, and nobody else.
	app.servers["radicale · vps"].Accept("REPORT")
	app.servers["baikal"].Accept("REPORT")
	if rec := app.h.do(postForm(page.Poll.RetryAll.Href, app.sess, url.Values{})); rec.Code >= 400 {
		t.Fatalf("retry all: %d %s", rec.Code, rec.Body)
	}
	after := waitPoll(t, app.h, app.sess, task, nil)
	if after.Failed != "" || after.RetryAll != nil {
		t.Errorf("after Retry all: failed %q, %+v", after.Failed, after.RetryAll)
	}
	for _, sp := range after.Sources {
		if sp.Answer != "" || sp.State != view.SourceDone && sp.State != view.SourceEmpty {
			t.Errorf("after Retry all: %q = %s %q", sp.Stamp, sp.State, sp.Answer)
		}
	}
}
