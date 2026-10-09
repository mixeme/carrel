package web_test

import (
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

func tasksPoll(p view.TasksPage) *view.PollBar { return p.Poll }

// frameGroup is a rubric of a list frame with its count's alarm.
type frameGroup struct {
	rubric string
	alert  bool
}

func frameGroups(t *testing.T, frame string) []frameGroup {
	t.Helper()
	var out []frameGroup
	for _, m := range regexp.MustCompile(`<div class="tc-list__group"><span class="tc-rubric">([^<]*)</span><span class="tc-num( is-alert)?">`).FindAllStringSubmatch(frame, -1) {
		out = append(out, frameGroup{m[1], m[2] != ""})
	}
	return out
}

// The combined list of tasks as s-tasks-1 draws it (the client's scan of
// the tasks, ЗБ1–ЗБ6, ЗБ9, ЗВ1–ЗВ3): the line under the title, Print in the
// head, each group with its count — the overdue one's an alarm —, the row
// of a linked task with how many sources it has and both in its hint, and
// the task open in the panel without scripts (?open=), its row on: the
// panel's fields in the frame's order, monospaced where the frame sets
// them, and the line under its attachments. One list says the same of
// itself; nothing ticked says what to do.
func TestTaskListIsTheFrame(t *testing.T) {
	frame := mockupFrame(t, "s-tasks-1")
	shape := regexp.MustCompile(`^\d+ of \d+ lists · nextcloud · \d+ tasks$`)
	if sub := frameSubtitle(t, "s-tasks-1"); !shape.MatchString(sub) {
		t.Fatalf("s-tasks-1 writes %q under its title: read the frame again", sub)
	}
	groups := frameGroups(t, frame)
	if len(groups) != 3 || !groups[0].alert || groups[1].alert {
		t.Fatalf("s-tasks-1 groups = %+v: read the frame again", groups)
	}
	panel := framePanel(t, "s-tasks-1")
	var terms []string
	for _, m := range regexp.MustCompile(`<dt>([^<]+)</dt><dd( class="tc-mono")?>`).FindAllStringSubmatch(panel, -1) {
		terms = append(terms, m[1]+map[bool]string{true: " mono"}[m[2] != ""])
	}

	app := newMockupApp(t)
	linkMockupTask(t, app)
	seedMockupTrip(t, app)
	app.lastUsed(t, "dela", view.KindTask)
	// The calendar was read: the panel's Related names the trip.
	openList(t, app, "/app/calendar?from=2026-08-10")
	first := combined(t, app.h, app.sess, "/app/tasks", tasksPoll)
	app.h.srv.SetClock(mockupNow(t).Add(mockupSince))
	open := app.listPath(t, "tasks", "dela") + "/aa11-2d09-5f84"
	page := decode[view.TasksPage](t, app.h.do(get("/app/tasks?task="+first.Poll.TaskID+"&open="+url.QueryEscape(open), app.sess)))

	if !shape.MatchString(string(page.Subtitle)) {
		t.Errorf("subtitle = %q, want the shape of %q", page.Subtitle, frameSubtitle(t, "s-tasks-1"))
	}
	// The table counts rows, the rail records (the owner's decision of
	// 09.10.2026): the linked task is one of «6 tasks» — the done one
	// counts too —, and one of each of its lists in the rail.
	if sub := frameSubtitle(t, "s-tasks-1"); string(page.Subtitle) != sub || !strings.HasSuffix(runsText(page.Poll.Summary), " · 6 tasks") {
		t.Errorf("subtitle = %q, poll %q; the frame %q", page.Subtitle, runsText(page.Poll.Summary), sub)
	}
	if all := page.Rail.Sections[0].Rows[0]; all.Count == nil || *all.Count != 7 {
		t.Errorf("the rail's combined row = %+v, want the 7 tasks of the ticked lists", all.Count)
	}
	// The section Tasks counts the overdue ones (Typecase's note 39, Р-47:
	// «Tasks 2» of every frame — the two under Overdue).
	nav := regexp.MustCompile(`<a[^>]*class="[^"]*tc-nav__item[^"]*"[^>]*>(?:<svg.*?</svg>)?Tasks<span class="tc-num">(\d+)</span>`).FindStringSubmatch(frame)
	if nav == nil || nav[1] != "2" || !groups[0].alert {
		t.Fatalf("s-tasks-1 nav = %v: read the frame again", nav)
	}
	for _, n := range page.Rail.Nav {
		if n.ID == "nav.tasks" && (n.Count == nil || *n.Count != 2) {
			t.Errorf("the section Tasks counts %v, want the 2 overdue", n.Count)
		}
	}
	if actionByID(page.Bar.Actions, "list.print") == nil {
		t.Errorf("no Print in the head: %+v", page.Bar.Actions)
	}
	if n := actionByID(page.Bar.Actions, "record.new"); n == nil || len(n.Arrow) != 2 || n.Arrow[0].Aside != "last used" {
		t.Errorf("New task = %+v, want its arrow with «last used» (s-tasks-5)", n)
	}
	var got []frameGroup
	for _, g := range page.Groups {
		got = append(got, frameGroup{string(g.Rubric), g.Overdue})
		if g.Count != len(g.Rows) {
			t.Errorf("%s counts %d of %d rows", g.Rubric, g.Count, len(g.Rows))
		}
	}
	if !slices.Equal(got, groups) {
		t.Errorf("groups = %+v, want %+v", got, groups)
	}
	var linked *view.TaskRow
	for gi := range page.Groups {
		for ri := range page.Groups[gi].Rows {
			if r := &page.Groups[gi].Rows[ri]; r.Summary == "Собрать чемодан" {
				linked = r
			}
		}
	}
	if linked == nil || linked.Stamp != "2 sources" || len(linked.Sources) != 2 || !linked.On {
		t.Fatalf("the linked row = %+v, want «2 sources», both stamps, open in the panel", linked)
	}
	d := page.Details
	if d == nil || d.Close == "" || strings.Contains(d.Close, "open=") {
		t.Fatalf("panel = %+v, want the task with the list without it behind the cross", d)
	}
	var fields []string
	for _, f := range d.Fields {
		// A value in pieces is monospaced by its first piece: the due date
		// before how late it is.
		mono := f.Mono || (len(f.Runs) > 0 && f.Runs[0].Mark == view.RunMono)
		fields = append(fields, string(f.Label)+map[bool]string{true: " mono"}[mono])
	}
	if !slices.Equal(fields, terms) {
		t.Errorf("panel fields = %q, want %q", fields, terms)
	}
	// The panel says where the task lies and how it compares with the
	// server, as its card does (Typecase's note 37: the block Source).
	if !strings.Contains(panel, `<div class="tc-rubric">Source</div>`) {
		t.Fatal("the panel of s-tasks-1 draws no Source: read the frame again")
	}
	if d.Source.Path == "" || d.Source.Compared == "" {
		t.Errorf("panel source = %+v, want its path and what the server says of it", d.Source)
	}
	if !strings.Contains(string(d.AttachHint), "2 MB") || d.Body == "" || len(d.Attach) != 1 || !d.Attach[0].Inline {
		t.Errorf("panel = %+v, want the description, the file in the task and the line under it", d)
	}
	if len(d.Related) != 1 || d.Related[0].Title != "Поездка в Казань" {
		t.Errorf("related = %+v", d.Related)
	}

	// One list: where and how many, Print, the panel.
	one := decode[view.TasksPage](t, app.h.do(get(app.listPath(t, "tasks", "dela")+"?open="+url.QueryEscape(open), app.sess)))
	// Four: the done one tagged «read» counts too (C24 of the client).
	if one.Subtitle != "nextcloud · 4 tasks" || actionByID(one.Bar.Actions, "list.print") == nil || one.Details == nil {
		t.Errorf("one list = %q %+v %+v", one.Subtitle, one.Bar.Actions, one.Details)
	}

	// Nothing ticked: what to do, and no poll (as states-14 of the agenda).
	if rec := app.h.do(postForm("/app/sources/tasks", app.sess, url.Values{})); rec.Code != 303 {
		t.Fatalf("untick everything: %d", rec.Code)
	}
	none := decode[view.TasksPage](t, app.h.do(get("/app/tasks", app.sess)))
	if none.Poll != nil || none.Empty == nil || len(none.Empty.Actions) != 1 || none.Empty.Actions[0].ID != "rail.tick.all" {
		t.Errorf("nothing ticked = poll %+v, empty %+v", none.Poll, none.Empty)
	}
}
