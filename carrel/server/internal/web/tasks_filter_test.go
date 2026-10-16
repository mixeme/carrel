package web_test

import (
	"testing"

	"carrel.dev/carrel/server/view"
)

// Note 69 of Typecase (task 167): a task list has «Filter by word» first in
// its bar, as the notes' has — without scripts a GET to the list, which
// keeps the tasks whose title or text holds the word (s-tasks-1, -15).
func TestTasksFilterByWord(t *testing.T) {
	f := calendarFixture(t, nil,
		"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Buy milk\nEND:VTODO"),
		"t2.ics", ics("BEGIN:VTODO\nUID:t2\nDTSTAMP:20260101T000000Z\nSUMMARY:Call Anna\nDESCRIPTION:about the milk run\nEND:VTODO"),
		"t3.ics", ics("BEGIN:VTODO\nUID:t3\nDTSTAMP:20260101T000000Z\nSUMMARY:Water the plants\nEND:VTODO"))
	titles := func(p view.TasksPage) []string {
		var out []string
		for _, g := range p.Groups {
			for _, r := range g.Rows {
				out = append(out, string(r.Summary))
			}
		}
		return out
	}
	one := decode[view.TasksPage](t, f.h.do(get(f.path("tasks")+"?filter=Milk", f.sess)))
	if one.Bar.FilterForm == nil || one.Bar.Filter != "Milk" || !equalStrings(titles(one), []string{"Buy milk", "Call Anna"}) {
		t.Errorf("one list filtered = %q, form %v, rows %v", one.Bar.Filter, one.Bar.FilterForm != nil, titles(one))
	}
	tickRail(t, f.h, f.sess, "tasks", f.path("tasks"))
	all := combined(t, f.h, f.sess, "/app/tasks?filter=plants", func(p view.TasksPage) *view.PollBar { return p.Poll })
	if all.Bar.FilterForm == nil || !equalStrings(titles(all), []string{"Water the plants"}) {
		t.Errorf("combined filtered = form %v, rows %v", all.Bar.FilterForm != nil, titles(all))
	}
}
