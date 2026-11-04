package web_test

import (
	"net/http"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// SV-56, the owner's decision 04.11.2026 (variant А): a birthday without a
// year is kept on Google as Apple writes it — Google Contacts show «13
// April» in the browser and «--04-13» on Android, no year 1604 (checked
// live that day). Google keeps every card as vCard 3.0, drops --MMDD, and
// keeps Apple's form with the marker inside VALUE. Found live: a new
// contact went as vCard 4.0 with --0413 and its birthday was lost; an edit
// of the 3.0 card went as Apple writes it, was kept, and was reported lost;
// and the card then showed the placeholder year.
func TestABirthdayWithoutAYearOnGoogle(t *testing.T) {
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:3.0\nUID:ada\nFN:Ada\nN:Lovelace;Ada;;;\nEND:VCARD\n")
	f.srv.StoresVCard3 = true
	list := f.path("contacts")
	const google = "BDAY;VALUE=DATE,X-APPLE-OMIT-YEAR=1604:1604-04-"

	// A new contact: the server keeps it as 3.0, and the day goes again as
	// 3.0 holds it, once — not reported lost.
	form := formFields(decode[view.ContactEditPage](t, f.h.do(get(list+"/new", f.sess))).Form)
	form.Set("n.given", "Vera")
	form.Set("n.family", "Probe")
	form.Set("bday", "13.04")
	resp := f.h.do(postForm(list+"/new", f.sess, form))
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("new contact with a birthday without a year: %d %s", resp.Code, resp.Body.String())
	}
	card := resp.Header().Get("Location")
	var made string
	for _, p := range f.srv.Paths(f.col) {
		if b, _, _ := f.srv.Object(p); strings.Contains(string(b), "Probe") {
			made = unfolded(b)
		}
	}
	mustContain(t, made, "VERSION:3.0", google+"13")

	// Shown without the placeholder year, on the card and in its form.
	shown := ""
	for _, fl := range decode[view.ContactPage](t, f.h.do(get(card, f.sess))).Fields {
		if fl.Label == "Birthday" {
			shown = string(fl.Value)
		}
	}
	if shown != "--04-13" {
		t.Errorf("the card shows %q, want --04-13", shown)
	}
	edit := formFields(decode[view.ContactEditPage](t, f.h.do(get(card+"/edit", f.sess))).Form)
	if edit.Get("bday") != "13.04" {
		t.Errorf("the form holds %q, want 13.04", edit.Get("bday"))
	}

	// Another day typed: written in the form the card holds, not lost.
	edit.Set("bday", "14.04")
	if resp := f.h.do(postForm(card+"/edit", f.sess, edit)); resp.Code != http.StatusSeeOther {
		t.Fatalf("another day: %d %s", resp.Code, resp.Body.String())
	}
	// A year typed: the marker goes, or the year would be hidden.
	edit = formFields(decode[view.ContactEditPage](t, f.h.do(get(card+"/edit", f.sess))).Form)
	edit.Set("bday", "14.04.1990")
	if resp := f.h.do(postForm(card+"/edit", f.sess, edit)); resp.Code != http.StatusSeeOther {
		t.Fatalf("a year typed: %d %s", resp.Code, resp.Body.String())
	}
	for _, p := range f.srv.Paths(f.col) {
		if b, _, _ := f.srv.Object(p); strings.Contains(string(b), "Probe") {
			made = unfolded(b)
		}
	}
	if !strings.Contains(made, "BDAY") || strings.Contains(made, "OMIT-YEAR") || !strings.Contains(made, "1990-04-14") {
		t.Errorf("a full date:\n%s", made)
	}

	// An edit of the card that was there: as Apple writes it, kept, not lost.
	ed := formFields(decode[view.ContactEditPage](t, f.h.do(get(list+"/ada/edit", f.sess))).Form)
	ed.Set("bday", "12.04")
	if resp := f.h.do(postForm(list+"/ada/edit", f.sess, ed)); resp.Code != http.StatusSeeOther {
		t.Fatalf("an edit: %d %s", resp.Code, resp.Body.String())
	}
	mustContain(t, body(t, f, "ada.vcf"), google+"12")
}
