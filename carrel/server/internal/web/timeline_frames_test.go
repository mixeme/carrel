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
// mono, a source that did not answer as the red «1 failed» of the menu of
// refusals (from-typecase note 32); the count of each month;
// the card in the panel with its avatar, without the head's rows, with the
// other books and its UID.

var (
	frameTLHead  = regexp.MustCompile(`(?s)<div class="tc-main">\s*<div class="tc-head[^"]*">\s*<div class="tc-head__titles[^"]*">\s*<span class="tc-avatar[^"]*">([^<]*)</span>.*?<div class="tc-head__sub">([^<]*)<span class="tc-badge is-linked">([^<]*)</span>.*?<div class="tc-acts">(.*?)</div>`)
	frameRunbar  = regexp.MustCompile(`(?s)<div class="tc-runbar">(.*?)<details class="tc-drop"><summary class="tc-btn is-sm is-danger">(\d+ failed)`)
	frameRunMono = regexp.MustCompile(`<span class="tc-mono">([^<]*)</span>`)
	frameTLRange = regexp.MustCompile(`(?s)<form class="tc-range" action="([^"]*)" method="(get)">(.*?)</form>`)
	frameTLDay   = regexp.MustCompile(`<input class="tc-in[^"]*" type="date" value="[^"]*" name="([^"]*)" aria-label="([^"]*)">`)
	frameSideDL  = regexp.MustCompile(`(?s)<div class="tc-side">.*?<dl class="tc-fields">(.*?)</dl>`)
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

	// The range: a GET form to the timeline (task 51) — two days by their
	// names and Show sending it, nothing under them.
	r := frameTLRange.FindStringSubmatch(frame)
	if r == nil || strings.Contains(r[3], "tc-field__hint") || !strings.Contains(r[3], `type="submit">Show</button>`) {
		t.Fatal("s-contacts-7 draws no range form, or a hint in it: read the frame again")
	}
	form := page.Range.Form
	if form == nil || form.Method != "GET" || screenPlace(form.Action) != screenPlace(r[1]) || form.Action != card+"/timeline" {
		t.Fatalf("range form = %+v, the frame %s %q", form, r[2], r[1])
	}
	var days, frameDays []string
	for _, s := range form.Sets {
		for _, row := range s.Rows {
			if row.Kind != view.InputHidden {
				days = append(days, row.Name+" "+string(row.Label)+" "+string(row.Kind))
			}
			if row.Hint != "" {
				t.Errorf("%s: hint %q, the frame draws none", row.Name, row.Hint)
			}
		}
	}
	for _, d := range frameTLDay.FindAllStringSubmatch(r[3], -1) {
		frameDays = append(frameDays, d[1]+" "+d[2]+" date")
	}
	if strings.Join(days, " | ") != strings.Join(frameDays, " | ") || len(frameDays) != 2 {
		t.Errorf("range fields = %q, the frame %q", days, frameDays)
	}
	if show := actionByID(form.Actions, "range.show"); show == nil || show.Label != "Show" || show.Method != "GET" {
		t.Errorf("range actions = %+v, want Show sending the form", form.Actions)
	}

	// The card's «⋯» in the panel: a button with its menu, drawn closed
	// (task 49), the menu the card's own — the panel's, not the bar's «⋯»
	// with the twin of Columns before it (Typecase's task 112, ac658b0).
	_, panel, ok := strings.Cut(frame, `class="tc-side`)
	if !ok {
		t.Fatal("s-contacts-7 draws no panel: read the frame again")
	}
	cardMenu := frameMenuIn(t, "s-contacts-7", panel)
	if g := serverMenu(page.Card.Menu); !equalStrings(g, cardMenu) {
		t.Errorf("timeline card «⋯» = %q,\nwant (s-contacts-7) %q", g, cardMenu)
	}

	// The run bar in pieces.
	bar := frameRunbar.FindStringSubmatch(frame)
	if bar == nil || len(frameRunMono.FindAllString(bar[1], -1)) == 0 || strings.Contains(bar[1], "tc-dim--danger") {
		t.Fatal("s-contacts-7 draws no run bar with addresses and «N failed»: read the frame again")
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
	// Both her addresses, as the frame says them (from-typecase note 117:
	// the second is her card's in «Контакты»).
	if strings.Join(mono, ",") != "a.kovaleva@example.org,anna.kovaleva@slovo.example" || !strings.Contains(said, " · matched by ") || !strings.Contains(said, " and the name") {
		t.Errorf("summary %q (mono %q), the frame %q", said, mono, frameText(bar[1]))
	}
	if len(alarm) != 0 || !strings.HasPrefix(said, "Polled ") || !strings.HasSuffix(said, " and the name") || string(page.Poll.Failed) != bar[2] {
		t.Errorf("summary %q: alarm %q, failed %q; the frame «%s» and %q", said, alarm, page.Poll.Failed, frameText(bar[1]), bar[2])
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
	// Анна is in three books (from-typecase note 117 p. 2).
	if page.Linked == nil || page.Linked.Label != "in 3 books" {
		t.Errorf("Анна: linked %+v, want «in 3 books»", page.Linked)
	}
}
