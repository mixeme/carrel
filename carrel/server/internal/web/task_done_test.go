package web_test

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

func menuAction(items []view.MenuItem, id string) *view.Action {
	for i := range items {
		if items[i].Action.ID == id {
			return &items[i].Action
		}
	}
	return nil
}

// review Р-10, Mark done: beside Edit on the task card (s-tasks-2) and on
// a row of the list (s-tasks-1), the three properties the Done box writes.
// No address of its own: a POST to the record's edit with done and the
// version, the rest of the record as it is; 412 is the conflict screen.
func TestMarkDoneFromTheCardAndTheRow(t *testing.T) {
	f := calendarFixture(t, nil, "t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Slides\n"+
		"PERCENT-COMPLETE:40\nDESCRIPTION:Keep me\nX-KEEP:yes\nEND:VTODO"))
	list := f.path("tasks")
	rec := list + "/t1"

	page := decode[view.TaskPage](t, f.h.do(get(rec, f.sess)))
	done := actionByID(page.Actions, "task.done")
	if done == nil || done.Label != "Mark done" || done.Method != "POST" {
		t.Fatalf("card actions = %+v, want Mark done", page.Actions)
	}
	resp := f.h.do(postHref(f, done.Href))
	if resp.Code != 303 || resp.Header().Get("Location") != rec {
		t.Fatalf("Mark done: %d %q %s", resp.Code, resp.Header().Get("Location"), resp.Body.String())
	}
	got := body(t, f, "t1.ics")
	mustContain(t, got, "STATUS:COMPLETED", "COMPLETED:", "SUMMARY:Slides", "DESCRIPTION:Keep me", "X-KEEP:yes")

	// From the list row: the same action, back to the list.
	tasks := decode[view.TasksPage](t, f.h.do(get(list+"?show=all", f.sess)))
	if len(tasks.Groups) == 0 || len(tasks.Groups[0].Rows) != 1 {
		t.Fatalf("groups = %+v", tasks.Groups)
	}
	undo := tasks.Groups[0].Rows[0].Tick
	if undo == nil || undo.ID != "task.done" || undo.Label != "Mark not done" {
		t.Fatalf("row tick = %+v, want Mark not done", undo)
	}
	resp = f.h.do(postHref(f, undo.Href))
	if resp.Code != 303 || !strings.HasPrefix(resp.Header().Get("Location"), list) || resp.Header().Get("Location") == rec {
		t.Fatalf("from the row: %d %q", resp.Code, resp.Header().Get("Location"))
	}
	got = body(t, f, "t1.ics")
	if strings.Contains(got, "STATUS:COMPLETED") || strings.Contains(got, "\nCOMPLETED:") {
		t.Errorf("still done:\n%s", got)
	}
	// Done was a hundred per cent, and the mark taken off brings nothing
	// back (the owner's decision of 12.10.2026, Б17): the percent goes.
	mustContain(t, got, "SUMMARY:Slides", "X-KEEP:yes")
	if strings.Contains(got, "PERCENT-COMPLETE") {
		t.Errorf("the mark taken off left a percent:\n%s", got)
	}

	// The version moved meanwhile: the conflict screen, nothing written.
	page = decode[view.TaskPage](t, f.h.do(get(rec, f.sess)))
	stale := actionByID(page.Actions, "task.done")
	f.srv.Seed(f.col+"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Slides, renamed\nEND:VTODO"))
	resp = f.h.do(postHref(f, stale.Href))
	if resp.Code != 303 || resp.Header().Get("Location") != rec+"/conflict" {
		t.Fatalf("over a moved version: %d %q", resp.Code, resp.Header().Get("Location"))
	}
	if strings.Contains(body(t, f, "t1.ics"), "COMPLETED") {
		t.Error("a stale Mark done was written")
	}

	// Only a task has it.
	u, _ := url.Parse(stale.Href)
	if resp := f.h.do(postForm(strings.Replace(u.EscapedPath(), "/tasks/", "/calendar/", 1), f.sess, u.Query())); resp.Code != 404 {
		t.Errorf("done on an event: %d, want 404", resp.Code)
	}
}

// Р-42 (the owner's word of 09.10.2026): a task marked done from the
// combined list stays where it stood in the list answered from the poll —
// crossed, its tick full, counted as it was read —, and the tick takes the
// mark off again; a list read anew from the server has it among the done.
func TestATaskJustMarkedDoneStaysWhereItStood(t *testing.T) {
	f := calendarFixture(t, nil,
		"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Slides\nEND:VTODO"),
		"t2.ics", ics("BEGIN:VTODO\nUID:t2\nDTSTAMP:20260101T000000Z\nSUMMARY:Notes\nEND:VTODO"))
	tickAll(t, f.h, f.sess)
	page := combined(t, f.h, f.sess, "/app/tasks", tasksPoll)
	row := func(p view.TasksPage, summary string) (view.TaskRow, string, bool) {
		for _, g := range p.Groups {
			for _, r := range g.Rows {
				if string(r.Summary) == summary {
					return r, string(g.Rubric), true
				}
			}
		}
		return view.TaskRow{}, "", false
	}
	before, group, ok := row(page, "Slides")
	if !ok || before.Done || before.Tick == nil {
		t.Fatalf("Slides before = %+v", before)
	}
	if resp := f.h.do(postHref(f, before.Tick.Href)); resp.Code != 303 {
		t.Fatalf("Mark done: %d %s", resp.Code, resp.Body.String())
	}
	again := decode[view.TasksPage](t, f.h.do(get("/app/tasks?task="+page.Poll.TaskID, f.sess)))
	after, where, ok := row(again, "Slides")
	if !ok || !after.Done || where != group || after.Tick == nil || after.Tick.Label != "Mark not done" {
		t.Errorf("Slides after the tick = %+v in %q, want it done where it stood (%q), its tick taking it back", after, where, group)
	}
	if fmt.Sprint(again.Bar.Segments) != fmt.Sprint(page.Bar.Segments) {
		t.Errorf("the counts moved with the tick: %+v, were %+v", again.Bar.Segments, page.Bar.Segments)
	}
	// Read anew: among the done, not the open.
	fresh := decode[view.TasksPage](t, f.h.do(get(f.path("tasks"), f.sess)))
	if _, _, ok := row(fresh, "Slides"); ok {
		t.Errorf("a list read anew still shows Slides among the open: %+v", fresh.Groups)
	}
}

// Р-42: the full tick of a task just marked done takes the mark off again —
// it carries the version the mark wrote, not the one the poll read, so it is
// written and not sent to the conflict —, and the row is open once more.
func TestTheTickTakesTheMarkOffAgain(t *testing.T) {
	f := calendarFixture(t, nil,
		"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Slides\nEND:VTODO"))
	tickAll(t, f.h, f.sess)
	page := combined(t, f.h, f.sess, "/app/tasks", tasksPoll)
	slides := func(p view.TasksPage) view.TaskRow {
		t.Helper()
		for _, g := range p.Groups {
			for _, r := range g.Rows {
				if string(r.Summary) == "Slides" && r.Tick != nil {
					return r
				}
			}
		}
		t.Fatalf("no Slides with a tick: %+v", p.Groups)
		return view.TaskRow{}
	}
	if resp := f.h.do(postHref(f, slides(page).Tick.Href)); resp.Code != 303 {
		t.Fatalf("Mark done: %d %s", resp.Code, resp.Body.String())
	}
	again := decode[view.TasksPage](t, f.h.do(get("/app/tasks?task="+page.Poll.TaskID, f.sess)))
	resp := f.h.do(postHref(f, slides(again).Tick.Href))
	if resp.Code != 303 || strings.HasSuffix(resp.Header().Get("Location"), "/conflict") {
		t.Fatalf("Mark not done: %d → %q, want it written", resp.Code, resp.Header().Get("Location"))
	}
	back := slides(decode[view.TasksPage](t, f.h.do(get("/app/tasks?task="+page.Poll.TaskID, f.sess))))
	if back.Done || back.Tick.Label != "Mark done" {
		t.Errorf("Slides after the mark taken off = %+v, want it open, its tick marking it done", back)
	}
	if got := body(t, f, "t1.ics"); strings.Contains(got, "STATUS:COMPLETED") {
		t.Errorf("the server still holds Slides done:\n%s", got)
	}
}
