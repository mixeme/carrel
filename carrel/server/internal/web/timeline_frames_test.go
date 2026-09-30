package web_test

import (
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// A person's timeline as the frame draws it (s-contacts-7; the client's scan
// of the contacts section, 30.09.2026, group Г): the head with the avatar,
// the line of where and as what, «in 2 books»; the buttons with icons and
// Print; the range without a hint; the run bar in pieces — the addresses in
// mono, a source that did not answer as an alarm; the count of each month;
// the card in the panel with its avatar, without the head's rows, with the
// other books and its UID.

var (
	frameTLHead   = regexp.MustCompile(`(?s)<div class="tc-main">\s*<div class="tc-head[^"]*">\s*<div class="tc-head__titles[^"]*">\s*<span class="tc-avatar[^"]*">([^<]*)</span>.*?<div class="tc-head__sub">([^<]*)<span class="tc-badge is-linked">([^<]*)</span>.*?<div class="tc-acts">(.*?)</div>`)
	frameRunbar   = regexp.MustCompile(`(?s)<div class="tc-runbar">(.*?)</div>`)
	frameRunMono  = regexp.MustCompile(`<span class="tc-mono">([^<]*)</span>`)
	frameRunAlarm = regexp.MustCompile(`<span class="tc-dim--danger">([^<]*)</span>`)
	frameTLRange  = regexp.MustCompile(`(?s)<span class="tc-range">(.*?)</span>`)
	frameSideDL   = regexp.MustCompile(`(?s)<div class="tc-side">.*?<dl class="tc-fields">(.*?)</dl>`)
)

func TestTimelineIsTheFrame(t *testing.T) {
	app := newMockupApp(t)
	combined(t, app.h, app.sess, "/app/contacts", contactsPoll)
	card := app.listPath(t, "contacts", "kontakty") + "/4f2b-9c31-a0e7"
	first := decode[view.ContactTimelinePage](t, app.h.do(get(card+"/timeline", app.sess)))
	waitPoll(t, app.h, app.sess, first.Poll.TaskID, func(b view.PollBar) bool { return !b.Running })
	page := decode[view.ContactTimelinePage](t, app.h.do(get(card+"/timeline", app.sess)))
	frame, _, _ := strings.Cut(mockupFrame(t, "s-contacts-7"), "<figcaption")

	head := frameTLHead.FindStringSubmatch(frame)
	if head == nil {
		t.Fatal("s-contacts-7 draws no head: read the frame again")
	}
	if page.Card.Avatar == nil || string(page.Card.Avatar.Initials) != frameText(head[1]) {
		t.Errorf("avatar = %+v, the frame %q", page.Card.Avatar, head[1])
	}
	if string(page.Subtitle) != frameText(head[2]) {
		t.Errorf("subtitle = %q, the frame %q", page.Subtitle, frameText(head[2]))
	}
	var want, got []string
	for _, b := range frameHeadButton.FindAllStringSubmatch(head[4], -1) {
		want = append(want, frameText(b[2])+" ["+frameIcon.FindStringSubmatch(b[2])[1]+"]")
	}
	for _, a := range page.Actions {
		got = append(got, string(a.Label)+" ["+a.Icon+"]")
	}
	if strings.Join(got, " | ") != strings.Join(want, " | ") {
		t.Errorf("head = %q, the frame %q", got, want)
	}
	if p := actionByID(page.Actions, "timeline.print"); p == nil || !strings.HasSuffix(p.Href, "/timeline?print=1") {
		t.Errorf("Print = %+v", p)
	}

	// The range: two days and Show, nothing under them.
	if r := frameTLRange.FindStringSubmatch(frame); r == nil || strings.Contains(r[1], "tc-field__hint") {
		t.Fatal("s-contacts-7 draws no range, or a hint in it: read the frame again")
	}
	for _, s := range page.Range.Form.Sets {
		for _, r := range s.Rows {
			if r.Hint != "" {
				t.Errorf("%s: hint %q, the frame draws none", r.Name, r.Hint)
			}
		}
	}

	// The run bar in pieces.
	bar := frameRunbar.FindStringSubmatch(frame)
	if bar == nil || len(frameRunMono.FindAllString(bar[1], -1)) == 0 || frameRunAlarm.FindString(bar[1]) == "" {
		t.Fatal("s-contacts-7 draws no run bar with addresses and an alarm: read the frame again")
	}
	var mono, alarm []string
	var words strings.Builder
	for _, r := range page.Poll.Summary {
		words.WriteString(string(r.Text))
		switch r.Mark {
		case view.RunMono:
			mono = append(mono, string(r.Text))
		case view.RunDanger:
			alarm = append(alarm, string(r.Text))
		}
	}
	said := words.String()
	if strings.Join(mono, ",") != "a.kovaleva@example.org" || !strings.Contains(said, " · matched by ") || !strings.Contains(said, " and the name") {
		t.Errorf("summary %q (mono %q), the frame %q", said, mono, frameText(bar[1]))
	}
	if len(alarm) != 1 || alarm[0] != "baikal · Общий unavailable" || !strings.HasPrefix(said, "Polled ") {
		t.Errorf("summary %q: alarm %q, the frame %q", said, alarm, frameRunAlarm.FindStringSubmatch(bar[1])[1])
	}

	// Each month says how many.
	for _, g := range page.Groups {
		if g.Count != len(g.Rows) || g.Count == 0 {
			t.Errorf("month %q: count %d, rows %d", g.Rubric, g.Count, len(g.Rows))
		}
	}

	// The card in the panel: no Organisation or Title — the head has them —,
	// its UID in the source.
	dl := frameSideDL.FindStringSubmatch(frame)
	if dl == nil {
		t.Fatal("s-contacts-7 draws no card in its panel: read the frame again")
	}
	drawn := map[string]bool{}
	for _, m := range framePanelTerm.FindAllStringSubmatch(dl[1], -1) {
		drawn[m[1]] = true
	}
	for _, f := range page.Card.Fields {
		if !drawn[string(f.Label)] && (f.Label == "Organisation" || f.Label == "Title") {
			t.Errorf("the panel draws no %q: %+v", f.Label, f)
		}
	}
	if page.Card.Source.UID != "4f2b-9c31-a0e7" {
		t.Errorf("card source = %+v", page.Card.Source)
	}

	// A person in two books: «in 2 books», and the other in the panel.
	artem := app.listPath(t, "contacts", "komanda") + "/d41a-6f30-92be/timeline"
	ap := decode[view.ContactTimelinePage](t, app.h.do(get(artem, app.sess)))
	shape := regexp.MustCompile("^" + frameDigits.ReplaceAllString(regexp.QuoteMeta(frameText(head[3])), `\d+`) + "$")
	if ap.Linked == nil || !shape.MatchString(string(ap.Linked.Label)) || ap.Linked.Role != view.BadgeLinked || len(ap.Card.Books) == 0 {
		t.Errorf("Артём: linked %+v, books %+v; the frame %q", ap.Linked, ap.Card.Books, head[3])
	}
	if page.Linked != nil {
		t.Errorf("Анна is in one book: %+v", page.Linked)
	}
}
