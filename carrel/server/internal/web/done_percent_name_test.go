package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Б17 of the audit of 11.10.2026 (the owner's decision of 12.10.2026): a
// task marked done is 100 per cent done; with the mark taken off nothing is
// brought back, because nothing was kept to bring back — Needs action, no
// percent, no date of completion.
func TestDoneIsAHundredPerCent(t *testing.T) {
	f := calendarFixture(t, nil, "t1.ics",
		ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Check the proof\nSTATUS:IN-PROCESS\nPERCENT-COMPLETE:60\nEND:VTODO"))
	mark := func(done string) string {
		t.Helper()
		_, etag, _ := f.srv.Object(f.col + "t1.ics")
		href := f.path("tasks") + "/t1/edit?" + url.Values{"op": {"done"}, "etag": {etag}, "done": {done}}.Encode()
		if rec := f.h.do(postForm(href, f.sess, url.Values{})); rec.Code != http.StatusSeeOther {
			t.Fatalf("mark %s: %d %s", done, rec.Code, rec.Body.String())
		}
		body, _, _ := f.srv.Object(f.col + "t1.ics")
		return unfolded(body)
	}
	got := mark("on")
	mustContain(t, got, "STATUS:COMPLETED", "PERCENT-COMPLETE:100", "COMPLETED:")
	got = mark("off")
	mustContain(t, got, "STATUS:NEEDS-ACTION")
	if strings.Contains(got, "PERCENT-COMPLETE") || strings.Contains(got, "COMPLETED:") {
		t.Errorf("the mark taken off left\n%s\nwant no percent and no date of completion", got)
	}
}

// Б30 of the audit of 11.10.2026 (the owner's decision of 12.10.2026): the
// display name follows the given and family names while the person has not
// made it their own — it was what the two parts make, so it becomes what
// they make now. A display name that was written by hand is left alone.
func TestTheDisplayNameFollowsTheParts(t *testing.T) {
	f := bookFixture(t, nil,
		"BEGIN:VCARD\nVERSION:4.0\nUID:anna\nFN:Anna Kovaleva\nN:Kovaleva;Anna;;;\nEND:VCARD\n",
		"BEGIN:VCARD\nVERSION:4.0\nUID:own\nFN:Anya (the printer's)\nN:Kovaleva;Anna;;;\nEND:VCARD\n",
		"BEGIN:VCARD\nVERSION:4.0\nUID:turned\nFN:Kovaleva Anna\nN:Kovaleva;Anna;;;\nEND:VCARD\n")
	save := func(uid, family string) string {
		t.Helper()
		page := decode[view.ContactEditPage](t, f.h.do(get(f.path("contacts")+"/"+uid+"/edit", f.sess)))
		fields := formFields(page.Form)
		if fields.Get("n.given") != "Anna" || fields.Get("n.family") != "Kovaleva" {
			t.Fatalf("%s: the form's name parts = %q, %q", uid, fields.Get("n.given"), fields.Get("n.family"))
		}
		fields.Set("n.family", family)
		u, _ := url.Parse(page.Form.Action)
		if rec := f.h.do(postForm(u.EscapedPath()+"?"+u.RawQuery, f.sess, fields)); rec.Code != http.StatusSeeOther {
			t.Fatalf("save %s: %d %s", uid, rec.Code, rec.Body.String())
		}
		return body(t, f, uid+".vcf")
	}
	mustContain(t, save("anna", "Belova"), "FN:Anna Belova", "N:Belova;Anna")
	mustContain(t, save("turned", "Belova"), "FN:Belova Anna", "N:Belova;Anna")
	mustContain(t, save("own", "Belova"), "FN:Anya (the printer's)", "N:Belova;Anna")
}
