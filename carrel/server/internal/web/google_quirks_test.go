package web_test

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// docs/servers.md, SV-56: Google keeps an event whose DTEND is its DTSTART
// without the DTEND. Since SV-53 Carrel writes that end whenever the person
// gave none, and the read-back then lacked it: the new event came back as
// its form with «The server did not keep DTEND» instead of opening (live,
// 01.10.2026). RFC 5545 §3.6.1 reads a timed event without an end as ending
// when it starts — the event the server kept is the event that was sent, and
// nothing was lost. The full form, the quick sheet and an edit that clears
// the end open the event.
func TestAnEndTheServerKeepsAsNoEndIsNotALoss(t *testing.T) {
	f := calendarFixture(t, nil,
		"meet.ics", ics("BEGIN:VEVENT\nUID:meet\nDTSTAMP:20260101T000000Z\nDTSTART:20260115T100000Z\nDTEND:20260115T110000Z\nSUMMARY:Review\nEND:VEVENT"))
	f.srv.DropsZeroLengthEnd = true
	made := func(what, route string, set map[string]string) string {
		t.Helper()
		before := f.srv.Paths(f.col)
		var form url.Values
		if strings.HasSuffix(route, "/quick") {
			form = formFields(decode[view.NewSheet](t, f.h.do(get(route, f.sess))).Form)
		} else {
			form = formFields(decode[view.EventEditPage](t, f.h.do(get(route, f.sess))).Form)
		}
		for k, v := range set {
			form.Set(k, v)
		}
		if resp := f.h.do(postForm(route, f.sess, form)); resp.Code != http.StatusSeeOther {
			t.Fatalf("%s: %d %s", what, resp.Code, resp.Body.String())
		}
		for _, p := range f.srv.Paths(f.col) {
			if !slices.Contains(before, p) {
				b, _, _ := f.srv.Object(p)
				return unfolded(b)
			}
		}
		t.Fatalf("%s made nothing", what)
		return ""
	}
	noEnd := func(what, got string) {
		t.Helper()
		if lineOf(got, "DTEND") != "" {
			t.Errorf("%s: the stand kept %q — it does not drop what it names\n%s", what, lineOf(got, "DTEND"), got)
		}
	}

	noEnd("timed, full form", made("timed, full form", f.path("calendar")+"/new",
		map[string]string{"summary": "Call", "start.date": "2031-03-11", "start.time": "09:30", "end.date": "", "end.time": ""}))
	noEnd("timed, quick sheet", made("timed, quick sheet", f.path("calendar")+"/new/quick",
		map[string]string{"summary": "Call", "start.date": "2031-03-12", "start.time": "09:30"}))

	rec := f.path("calendar") + "/meet"
	form := formFields(decode[view.EventEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form)
	form.Set("end.date", "")
	form.Set("end.time", "")
	if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != http.StatusSeeOther {
		t.Fatalf("clear the end: %d %s", resp.Code, resp.Body.String())
	}
	noEnd("end cleared", body(t, f, "meet.ics"))
}

// docs/servers.md, SV-56: Google marks the first phone, address and e-mail
// of a card PREF and writes the TYPE values in capitals: a contact made with
// a mobile number and a home e-mail is kept as TEL;TYPE=CELL,PREF and
// EMAIL;TYPE=HOME,PREF, and the page answered with its form and «The server
// did not keep EMAIL, TEL» (live, 01.10.2026). The labels are the ones the
// person chose; the mark is the server's, as it is for any client that
// shows one number first. The new contact opens, and a label changed later
// is no loss either.
func TestAPreferredMarkTheServerAddsIsNotALoss(t *testing.T) {
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:3.0\nUID:ada\nFN:Ada\nN:Lovelace;Ada;;;\nTEL;TYPE=WORK,PREF:+1 555 0100\nEND:VCARD\n")
	f.srv.MarksThePreferred = true
	list := f.path("contacts")

	form := formFields(decode[view.ContactEditPage](t, f.h.do(get(list+"/new", f.sess))).Form)
	form.Set("n.given", "Bob")
	form.Set("n.family", "Smith")
	for _, kind := range []string{"tel", "email"} {
		form.Set("add", kind)
		form = formFields(decode[view.ContactEditPage](t, f.h.do(postForm(list+"/new", f.sess, form))).Form)
	}
	form.Del("add")
	form.Set("tel.0", "+1 555 0101")
	form.Set("tel.0.type", "cell")
	form.Set("email.0", "bob@example.org")
	form.Set("email.0.type", "home")
	resp := f.h.do(postForm(list+"/new", f.sess, form))
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("new contact with a phone and an e-mail: %d %s", resp.Code, resp.Body.String())
	}
	var made string
	for _, p := range f.srv.Paths(f.col) {
		if b, _, _ := f.srv.Object(p); strings.Contains(string(b), "Smith") {
			made = unfolded(b)
		}
	}
	mustContain(t, made, "TEL;TYPE=CELL,PREF:+1 555 0101", "EMAIL;TYPE=HOME,PREF:bob@example.org")

	// A person's own PREF that the server drops is still a loss: that
	// direction is not the server's mark (model.TestAPreferredMark…).
	page := decode[view.ContactEditPage](t, f.h.do(get(list+"/ada/edit", f.sess)))
	form = formFields(page.Form)
	form.Set("tel.0.type", "cell")
	if resp := f.h.do(postForm(list+"/ada/edit", f.sess, form)); resp.Code != http.StatusSeeOther {
		t.Fatalf("label changed: %d %s", resp.Code, resp.Body.String())
	}
	mustContain(t, body(t, f, "ada.vcf"), "TEL;TYPE=CELL,PREF:+1 555 0100")
}
