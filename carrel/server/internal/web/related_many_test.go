package web_test

import (
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// A record has as many relations as it has (the owner's decision of
// 09.10.2026; RFC 5545 gives RELATED-TO any number of times). The field
// Related to of an edit holds every one of them — each chosen option is a
// value under its name —, and the line a link is typed into, «related.add»,
// stands whether there are relations or not and adds one more (Typecase's
// task 142: «Title or UID», «Add a link»). A relation kept keeps its
// parameters; one whose tick is gone goes.
func TestRelatedToHoldsEveryRelation(t *testing.T) {
	f := calendarFixture(t, nil,
		"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Slides\nRELATED-TO;RELTYPE=SIBLING:t2\nRELATED-TO:elsewhere\nEND:VTODO"),
		"t2.ics", ics("BEGIN:VTODO\nUID:t2\nDTSTAMP:20260101T000000Z\nSUMMARY:Rehearse\nEND:VTODO"),
		"t3.ics", ics("BEGIN:VTODO\nUID:t3\nDTSTAMP:20260101T000000Z\nSUMMARY:Book the room\nEND:VTODO"))
	list := f.path("tasks")
	rec := list + "/t1"
	_ = decode[view.TasksPage](t, f.h.do(get(list+"?show=all", f.sess))) // the session loads the list
	edit := func() view.Form {
		t.Helper()
		return decode[view.TaskEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form
	}
	chosen := func(row view.FormRow) []string {
		var out []string
		for _, o := range row.Options {
			if o.Selected {
				out = append(out, o.Value)
			}
		}
		return out
	}
	save := func(related []string, typed string) string {
		t.Helper()
		form := formFields(edit())
		form.Del("related")
		for _, v := range related {
			form.Add("related", v)
		}
		form.Set("related.add", typed)
		if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != 303 {
			t.Fatalf("POST %v + %q: %d %s", related, typed, resp.Code, resp.Body.String())
		}
		return body(t, f, "t1.ics")
	}

	form := edit()
	row := rowNamed(t, form, "related")
	// The one outside the loaded lists stands first among the options; the
	// form's values are the two all the same.
	if got := chosen(row); !row.Multiple || !slices.Equal(got, []string{"elsewhere", "t2"}) {
		t.Fatalf("related: multiple %v, chosen %q, want both relations of the task", row.Multiple, got)
	}
	if add := row.Add; add == nil || add.Name != "related.add" || add.Kind != view.InputText || add.Placeholder != "Title or UID" ||
		add.Label != "Add a link" || add.Suggest == "" || add.Value != "" {
		t.Errorf("the line a link is typed into = %+v", add)
	}
	// The form as it came changes nothing: the same relations in the
	// options' order are no change, and the record keeps its own.
	before := body(t, f, "t1.ics")
	if got := save([]string{"elsewhere", "t2"}, ""); got != before {
		t.Errorf("the form as it came rewrote the task:\n%s", got)
	}

	// One more is typed by its UID: the two stay, the kept one with its
	// parameter.
	got := save([]string{"t2", "elsewhere"}, "t3")
	mustContain(t, got, "RELATED-TO;RELTYPE=SIBLING:t2", "RELATED-TO:elsewhere", "RELATED-TO:t3")

	// The first is unticked: the others keep what they had — the parameter
	// of the first does not slide onto the next.
	got = save([]string{"elsewhere", "t3"}, "")
	mustContain(t, got, "RELATED-TO:elsewhere", "RELATED-TO:t3")
	if strings.Contains(got, "t2") || strings.Contains(got, "RELTYPE") {
		t.Errorf("the unticked relation or its parameter stayed:\n%s", got)
	}

	// A title of a record the session has loaded is that record; one already
	// related is not written twice.
	got = save([]string{"elsewhere", "t3"}, "Rehearse")
	mustContain(t, got, "RELATED-TO:elsewhere", "RELATED-TO:t3", "RELATED-TO:t2")
	got = save([]string{"elsewhere", "t3", "t2"}, "t3")
	if strings.Count(got, "RELATED-TO") != 3 {
		t.Errorf("a relation typed again was written twice:\n%s", got)
	}

	// Nothing ticked and nothing typed: no relation.
	got = save(nil, "")
	if strings.Contains(got, "RELATED-TO") {
		t.Errorf("unticked relations stayed:\n%s", got)
	}
	// And the line stands in an empty field too.
	row = rowNamed(t, edit(), "related")
	if got := chosen(row); len(got) != 0 || row.Add == nil {
		t.Errorf("chosen = %q, the line %+v after all were unticked", got, row.Add)
	}
}
