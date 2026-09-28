package web_test

import (
	"net/http"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// review Р-10: the card showed a nickname, a birthday and categories that
// the form did not let anyone change (s-contacts-3). Each is a field now, and
// changing one writes that one line (S-7): the rest of the card, its
// parameters and what no field offers stay byte for byte. The card starts
// with one category: davtest serves a card through go-vcard, which escapes
// the commas of a list — Baikal and Radicale serve the bytes they keep — so
// an untouched list of two would come back escaped by the stand, not by
// Carrel.
func TestContactNicknameBirthdayCategories(t *testing.T) {
	const card = "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nNICKNAME:Ada\n" +
		"BDAY;VALUE=date:18151210\nCATEGORIES:work\nTEL:+44 20 1\nX-CUSTOM:keep\nEND:VCARD\n"
	f := contactsFixture(t, false, card,
		"BEGIN:VCARD\nVERSION:3.0\nUID:bob\nFN:Bob\nBDAY:--0412\nEND:VCARD\n")
	ada := f.path("contacts") + "/ada"

	edit := decode[view.ContactEditPage](t, f.h.do(get(ada+"/edit", f.sess)))
	form := formFields(edit.Form)
	if form.Get("nickname") != "Ada" || form.Get("bday") != "10.12.1815" || form.Get("categories") != "work" {
		t.Fatalf("form = %v", form)
	}
	save := func(rec string, set map[string]string) {
		t.Helper()
		form := formFields(decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form)
		for k, v := range set {
			form.Set(k, v)
		}
		resp := f.h.do(postForm(rec+"/edit", f.sess, form))
		if resp.Code != http.StatusSeeOther {
			t.Fatalf("POST %v: %d %s", set, resp.Code, resp.Body.String())
		}
	}
	lines := func(file string) map[string]bool {
		out := map[string]bool{}
		for _, l := range strings.Split(strings.ReplaceAll(body(t, f, file), "\r\n", "\n"), "\n") {
			out[l] = true
		}
		return out
	}
	unchanged := func(file string, keep ...string) {
		t.Helper()
		got := lines(file)
		for _, l := range keep {
			if !got[l] {
				t.Errorf("%s lost the line %q:\n%s", file, l, body(t, f, file))
			}
		}
	}

	save(ada, map[string]string{"nickname": "The Enchantress"})
	unchanged("ada.vcf", "NICKNAME:The Enchantress", "BDAY;VALUE=date:18151210", "CATEGORIES:work", "TEL:+44 20 1", "X-CUSTOM:keep", "FN:Ada Lovelace")

	save(ada, map[string]string{"bday": "11.12.1815"})
	unchanged("ada.vcf", "BDAY;VALUE=date:18151211", "NICKNAME:The Enchantress", "CATEGORIES:work")

	save(ada, map[string]string{"categories": "work, maths , poetry,"})
	unchanged("ada.vcf", "CATEGORIES:work,maths,poetry", "BDAY;VALUE=date:18151211", "X-CUSTOM:keep")

	// Emptied, a field takes its line away: empty is not removal, but a field
	// the person emptied is (S-7).
	save(ada, map[string]string{"categories": "", "nickname": ""})
	if got := body(t, f, "ada.vcf"); strings.Contains(got, "CATEGORIES") || strings.Contains(got, "NICKNAME") {
		t.Errorf("emptied fields left their lines:\n%s", got)
	}

	// A birthday without a year is read as DD.MM (s-contacts-3) and written
	// the way it came: untouched when another field changes, basic when
	// changed.
	bob := f.path("contacts") + "/bob"
	form = formFields(decode[view.ContactEditPage](t, f.h.do(get(bob+"/edit", f.sess))).Form)
	if form.Get("bday") != "12.04" {
		t.Errorf("a birthday without a year reads %q, want 12.04", form.Get("bday"))
	}
	save(bob, map[string]string{"nickname": "B"})
	unchanged("bob.vcf", "BDAY:--0412")
	save(bob, map[string]string{"bday": "13.04"})
	unchanged("bob.vcf", "BDAY:--0413")

	// What is not a date is refused and nothing is written.
	puts := f.srv.CountRequests("PUT", f.col)
	form = formFields(decode[view.ContactEditPage](t, f.h.do(get(bob+"/edit", f.sess))).Form)
	form.Set("bday", "13 April")
	page := decode[view.ContactEditPage](t, f.h.do(postForm(bob+"/edit", f.sess, form)))
	if len(page.Form.Errors) == 0 || page.Form.Errors[0].Name != "bday" {
		t.Errorf("errors = %+v, want bday", page.Form.Errors)
	}
	if n := f.srv.CountRequests("PUT", f.col) - puts; n != 0 {
		t.Errorf("a refused birthday sent %d PUT", n)
	}
}

// Baikal and Nextcloud (sabre) keep a birthday without a year as Apple
// writes it: BDAY;X-APPLE-OMIT-YEAR=1604:1604-04-12 (found live, 19.09.2026).
// That is --04-12 to a person, not the year 1604; a day without a year is
// written back in the same form, and a full date takes the marker off, or
// the year typed would be hidden.
func TestAppleBirthdayWithoutAYear(t *testing.T) {
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:3.0\nUID:vera\nFN:Vera\nBDAY;X-APPLE-OMIT-YEAR=1604:1604-04-12\nEND:VCARD\n")
	rec := f.path("contacts") + "/vera"
	form := formFields(decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form)
	if form.Get("bday") != "12.04" {
		t.Fatalf("bday shown %q, want 12.04", form.Get("bday"))
	}
	shown := false
	for _, fl := range decode[view.ContactPage](t, f.h.do(get(rec, f.sess))).Fields {
		shown = shown || fl.Label == "Birthday" && fl.Value == "--04-12"
	}
	if !shown {
		t.Error("the card shows the placeholder year")
	}
	save := func(v string) string {
		t.Helper()
		form := formFields(decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form)
		form.Set("bday", v)
		if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != http.StatusSeeOther {
			t.Fatalf("POST %s: %d %s", v, resp.Code, resp.Body.String())
		}
		return body(t, f, "vera.vcf")
	}
	mustContain(t, save("13.04"), "BDAY;X-APPLE-OMIT-YEAR=1604:1604-04-13")
	got := save("13.04.1990")
	mustContain(t, got, "BDAY:1990-04-13")
	if strings.Contains(got, "X-APPLE-OMIT-YEAR") {
		t.Errorf("a full date kept the marker that hides its year:\n%s", got)
	}
}
