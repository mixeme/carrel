package web_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// review Р-10: a task showed a start, a status, a priority, a place,
// categories and what it relates to (s-tasks-3), and the form changed none
// of them. Status moves COMPLETED with it, as the Done box does; priority is
// a choice that keeps another client's number; Related to is picked from
// the records this session has loaded, or typed as a UID.
func TestTaskFields(t *testing.T) {
	f := calendarFixture(t, nil,
		"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Slides\nPRIORITY:3\nSTATUS:NEEDS-ACTION\nX-KEEP:yes\nEND:VTODO"),
		"t2.ics", ics("BEGIN:VTODO\nUID:t2\nDTSTAMP:20260101T000000Z\nSUMMARY:Rehearse\nEND:VTODO"))
	list := f.path("tasks")
	rec := list + "/t1"
	_ = decode[view.TasksPage](t, f.h.do(get(list+"?show=all", f.sess))) // the session loads the list
	save := func(set map[string]string) string {
		t.Helper()
		form := formFields(decode[view.TaskEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form)
		for k, v := range set {
			form.Set(k, v)
		}
		if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != 303 {
			t.Fatalf("POST %v: %d %s", set, resp.Code, resp.Body.String())
		}
		return body(t, f, "t1.ics")
	}

	page := decode[view.TaskEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
	form := formFields(page.Form)
	if form.Get("priority") != "3" || form.Get("status") != "NEEDS-ACTION" || form.Get("start.date") != "" || form.Get("related") != "" {
		t.Fatalf("form = %v", form)
	}
	prio := rowNamed(t, page.Form, "priority")
	if last := prio.Options[len(prio.Options)-1]; last.Value != "3" || !last.Selected {
		t.Errorf("priority options = %+v, want another client's 3 kept as a choice", prio.Options)
	}
	related := rowNamed(t, page.Form, "related")
	offered := false
	for _, o := range related.Options {
		if o.Value == "t1" {
			t.Error("a task is offered as related to itself")
		}
		offered = offered || (o.Value == "t2" && strings.Contains(string(o.Label), "Rehearse"))
	}
	if !offered {
		t.Errorf("related options = %+v, want the loaded Rehearse", related.Options)
	}

	got := save(map[string]string{"start.date": "2026-02-01", "start.allday": "on", "location": "Room 4", "categories": "work, talk"})
	mustContain(t, got, "DTSTART;VALUE=DATE:20260201", "LOCATION:Room 4", "CATEGORIES:work,talk", "PRIORITY:3", "X-KEEP:yes")

	got = save(map[string]string{"priority": "1", "status": "IN-PROCESS"})
	mustContain(t, got, "PRIORITY:1", "STATUS:IN-PROCESS")

	got = save(map[string]string{"status": "COMPLETED"})
	mustContain(t, got, "STATUS:COMPLETED", "COMPLETED:")
	got = save(map[string]string{"status": "NEEDS-ACTION"})
	if strings.Contains(got, "\nCOMPLETED:") {
		t.Errorf("COMPLETED stayed on a task no longer done:\n%s", got)
	}

	got = save(map[string]string{"related": "t2"})
	mustContain(t, got, "RELATED-TO:t2")
	got = save(map[string]string{"related.uid": "elsewhere-uid"})
	mustContain(t, got, "RELATED-TO:elsewhere-uid")
	if strings.Contains(got, "RELATED-TO:t2") {
		t.Errorf("the old relation stayed:\n%s", got)
	}
	got = save(map[string]string{"related": "", "priority": ""})
	if strings.Contains(got, "RELATED-TO") || strings.Contains(got, "PRIORITY") {
		t.Errorf("emptied fields left their lines:\n%s", got)
	}
}
