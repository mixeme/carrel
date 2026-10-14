package web_test

import (
	"net/http"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Р26 of scans/kinds-parity.md: the card showed a website and an anniversary
// (s-contacts-1) that no field of the form could change, while its phones,
// e-mails and birthday each had one. Now the websites are a rubric of rows,
// labelled as the phones are, and the anniversary is a day typed as the
// birthday is; each change writes its own line and leaves the rest (S-7).
func TestContactWebsiteAndAnniversaryAreFields(t *testing.T) {
	const card = "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\n" +
		"URL;TYPE=work:https://analytical.example\nANNIVERSARY:18350708\nX-CUSTOM:keep\nEND:VCARD\n"
	f := contactsFixture(t, false, card)
	ada := f.path("contacts") + "/ada"

	edit := decode[view.ContactEditPage](t, f.h.do(get(ada+"/edit", f.sess)))
	form := formFields(edit.Form)
	if form.Get("url.0") != "https://analytical.example" || form.Get("url.0.type") != "work" || form.Get("anniversary") != "08.07.1835" {
		t.Fatalf("form = %v", form)
	}
	var legends []string
	for _, s := range edit.Form.Sets {
		legends = append(legends, string(s.Legend))
	}
	if !strings.Contains(strings.Join(legends, ","), "Websites") {
		t.Errorf("sets = %q, want Websites", legends)
	}

	save := func(set map[string]string) string {
		t.Helper()
		form := formFields(decode[view.ContactEditPage](t, f.h.do(get(ada+"/edit", f.sess))).Form)
		for k, v := range set {
			form.Set(k, v)
		}
		resp := f.h.do(postForm(ada+"/edit", f.sess, form))
		if resp.Code != http.StatusSeeOther {
			t.Fatalf("POST %v: %d %s", set, resp.Code, resp.Body.String())
		}
		return strings.ReplaceAll(body(t, f, "ada.vcf"), "\r\n", "\n")
	}
	has := func(got string, lines ...string) {
		t.Helper()
		for _, l := range lines {
			if !strings.Contains("\n"+got, "\n"+l+"\n") {
				t.Errorf("no line %q in:\n%s", l, got)
			}
		}
	}

	got := save(map[string]string{"anniversary": "09.07.1835"})
	has(got, "ANNIVERSARY:18350709", "URL;TYPE=work:https://analytical.example", "X-CUSTOM:keep")

	got = save(map[string]string{"url.0": "https://ada.example", "anniversary": "08.07"})
	has(got, "URL;TYPE=work:https://ada.example", "ANNIVERSARY:--0708", "X-CUSTOM:keep")

	// The card shows the day as it shows the birthday: as a date, set in
	// mono (s-contacts-1).
	page := decode[view.ContactPage](t, f.h.do(get(ada, f.sess)))
	found := false
	for _, fl := range page.Fields {
		if fl.Label == "Anniversary" {
			found = true
			if fl.Value != "--07-08" || !fl.Mono {
				t.Errorf("Anniversary = %q mono %v, want --07-08 in mono", fl.Value, fl.Mono)
			}
		}
	}
	if !found {
		t.Errorf("the card shows no Anniversary: %+v", page.Fields)
	}

	// A new contact offers both, and a website added is written.
	fresh := decode[view.ContactEditPage](t, f.h.do(get(f.path("contacts")+"/new", f.sess)))
	if !hasRow(fresh.Form, "anniversary") {
		t.Error("a new contact has no Anniversary")
	}
	var add bool
	for _, s := range fresh.Form.Sets {
		for _, r := range s.Rows {
			if r.Kind == view.InputAdd && r.Value == "url" {
				add = true
			}
		}
	}
	if !add {
		t.Error("a new contact has no Add website")
	}
}
