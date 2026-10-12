package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Б29 of the audit of 11.10.2026 (the owner's decision of 12.10.2026: a
// relation made in Carrel is «see also», not subordination). A relation the
// form adds is written with its type, RELTYPE=SIBLING: a bare RELATED-TO is a
// PARENT by the standard, and other programs drew the task a subtask. A
// relation the record already had keeps the type it came with, a bare one
// included — that one is another program's subtask.
func TestANewRelationSaysItsType(t *testing.T) {
	task := func(uid, title string, more ...string) string {
		return ics("BEGIN:VTODO\nUID:" + uid + "\nDTSTAMP:20260101T000000Z\nSUMMARY:" + title + strings.Join(append([]string{""}, more...), "\n") + "\nEND:VTODO")
	}
	f := calendarFixture(t, nil,
		"fair.ics", task("fair", "Prepare the fair"),
		"call.ics", task("call", "Call the printer"),
		"sub.ics", task("sub", "Order the stand", "RELATED-TO:fair"),
	)
	save := func(uid string, related ...string) string {
		t.Helper()
		page := decode[view.TaskEditPage](t, f.h.do(get(f.path("tasks")+"/"+uid+"/edit", f.sess)))
		fields := formFields(page.Form)
		fields.Del("related")
		for _, r := range related {
			fields.Add("related", r)
		}
		u, _ := url.Parse(page.Form.Action)
		if rec := f.h.do(postForm(u.EscapedPath()+"?"+u.RawQuery, f.sess, fields)); rec.Code != http.StatusSeeOther {
			t.Fatalf("save %s: %d %s", uid, rec.Code, rec.Body.String())
		}
		body, _, _ := f.srv.Object(f.col + uid + ".ics")
		return unfolded(body)
	}

	if got := save("call", "fair"); !strings.Contains(got, "RELATED-TO;RELTYPE=SIBLING:fair") {
		t.Errorf("a relation added in the form is written\n%s\nwant it typed SIBLING", got)
	}
	// The subtask of another program keeps its bare line when a relation is
	// added beside it; the added one is typed.
	got := save("sub", "fair", "call")
	if !strings.Contains(got, "RELATED-TO:fair") || !strings.Contains(got, "RELATED-TO;RELTYPE=SIBLING:call") {
		t.Errorf("a relation added beside a subtask's:\n%s\nwant the bare one kept and the new one typed", got)
	}

	// So a task marked done closes the other program's subtask and not the
	// task Carrel only related to it.
	_, etag, _ := f.srv.Object(f.col + "fair.ics")
	href := f.path("tasks") + "/fair/edit?" + url.Values{"op": {"done"}, "etag": {etag}, "done": {"on"}}.Encode()
	if rec := f.h.do(postForm(href, f.sess, url.Values{})); rec.Code != http.StatusSeeOther {
		t.Fatalf("mark done: %d %s", rec.Code, rec.Body.String())
	}
	done := func(uid string) bool {
		body, _, _ := f.srv.Object(f.col + uid + ".ics")
		return strings.Contains(unfolded(body), "STATUS:COMPLETED")
	}
	if !done("fair") || !done("sub") || done("call") {
		t.Errorf("fair done %v, its subtask %v, the task related to it %v — want true, true, false", done("fair"), done("sub"), done("call"))
	}
}
