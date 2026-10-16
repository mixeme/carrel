package web_test

import (
	"net/http"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Р26 of scans/kinds-parity.md: the card showed a website and an anniversary
// (s-contacts-1) that no field of the form could change, while its phones,
// e-mails and birthday each had one. The websites are a rubric of rows,
// labelled as the phones are. The dates are a rubric too, by the owner's
// decision of 16.10.2026 (Typecase's note 71, task 171; s-contacts-34): each
// row what it marks and the day, typed as the birthday is, written as Apple
// writes them — item1.X-ABDATE and item1.X-ABLabel; the card's ANNIVERSARY
// is a row marked «Anniversary» and stays that line while it is so marked.
// Each change writes its own lines and leaves the rest (S-7).
func TestContactWebsitesAndDatesAreRows(t *testing.T) {
	const card = "BEGIN:VCARD\nVERSION:3.0\nUID:ada\nFN:Ada Lovelace\n" +
		"URL;TYPE=work:https://analytical.example\nANNIVERSARY:1835-07-08\n" +
		"item1.TEL:+44 20 7946 0000\nitem1.X-ABLabel:Lab\n" +
		"item2.X-ABDATE:1843-09-01\nitem2.X-ABLabel:_$!<Other>!$_\nX-CUSTOM:keep\nEND:VCARD\n"
	f := contactsFixture(t, false, card)
	ada := f.path("contacts") + "/ada"

	edit := decode[view.ContactEditPage](t, f.h.do(get(ada+"/edit", f.sess)))
	form := formFields(edit.Form)
	if form.Get("url.0") != "https://analytical.example" || form.Get("url.0.type") != "work" {
		t.Fatalf("websites = %v", form)
	}
	if form.Get("date.0.mark") != "Anniversary" || form.Get("date.0") != "08.07.1835" ||
		form.Get("date.1.mark") != "Other" || form.Get("date.1") != "01.09.1843" {
		t.Fatalf("dates = %v", form)
	}
	var legends []string
	for _, s := range edit.Form.Sets {
		legends = append(legends, string(s.Legend))
	}
	if got := strings.Join(legends, ","); !strings.Contains(got, "Websites,Dates,") {
		t.Errorf("sets = %q, want Websites, then Dates", got)
	}
	// A date's words are its row's; a phone's label stays foreign.
	var foreign []string
	for _, fl := range decode[view.ContactPage](t, f.h.do(get(ada, f.sess))).Foreign {
		foreign = append(foreign, string(fl.Label)+"="+string(fl.Value))
	}
	if got := strings.Join(foreign, " "); got != "X-ABLABEL=Lab X-CUSTOM=keep" {
		t.Errorf("foreign = %q, want only the phone's label and X-CUSTOM", got)
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
	hasNo := func(got string, lines ...string) {
		t.Helper()
		for _, l := range lines {
			if strings.Contains(got, l) {
				t.Errorf("a line %q in:\n%s", l, got)
			}
		}
	}

	// The anniversary's day: its own line, the rest as it was.
	got := save(map[string]string{"date.0": "09.07.1835"})
	has(got, "ANNIVERSARY:1835-07-09", "item2.X-ABDATE:1843-09-01", "item2.X-ABLABEL:_$!<Other>!$_",
		"item1.X-ABLABEL:Lab", "URL;TYPE=work:https://analytical.example", "X-CUSTOM:keep")

	// A date added is Apple's pair in the next group; a day without a year
	// in vCard 3.0 is Apple's year it names.
	got = save(map[string]string{"date.2": "12.06", "date.2.mark": "Свадьба"})
	has(got, "ANNIVERSARY:1835-07-09", "item2.X-ABDATE:1843-09-01", "item3.X-ABDATE;X-APPLE-OMIT-YEAR=1604:1604-06-12",
		"item3.X-ABLABEL:Свадьба", "item1.X-ABLABEL:Lab")

	// The anniversary marked otherwise is no longer ANNIVERSARY; a row
	// removed takes its words with it.
	form = formFields(decode[view.ContactEditPage](t, f.h.do(get(ada+"/edit", f.sess))).Form)
	form.Set("date.0.mark", "Помолвка")
	form.Set("date.1", "")
	resp := f.h.do(postForm(ada+"/edit", f.sess, form))
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("POST: %d %s", resp.Code, resp.Body.String())
	}
	got = strings.ReplaceAll(body(t, f, "ada.vcf"), "\r\n", "\n")
	has(got, "item4.X-ABDATE:1835-07-09", "item4.X-ABLABEL:Помолвка", "item3.X-ABLABEL:Свадьба", "item1.X-ABLABEL:Lab")
	hasNo(got, "ANNIVERSARY", "1843-09-01", "_$!<Other>!$_")

	// The card shows each date with its words, the day as the birthday's,
	// in mono (s-contacts-34).
	page := decode[view.ContactPage](t, f.h.do(get(ada, f.sess)))
	dates := map[string]view.Field{}
	for _, fl := range page.Fields {
		dates[string(fl.Label)] = fl
	}
	if d := dates["Помолвка"]; d.Value != "1835-07-09" || !d.Mono {
		t.Errorf("Помолвка = %+v", d)
	}
	if d := dates["Свадьба"]; d.Value != "--06-12" || !d.Mono {
		t.Errorf("Свадьба = %+v", d)
	}

	// A wrong day is refused at its row, nothing written.
	puts := f.srv.CountRequests("PUT", f.col)
	form = formFields(decode[view.ContactEditPage](t, f.h.do(get(ada+"/edit", f.sess))).Form)
	form.Set("date.0", "31.31.2020")
	refused := decode[view.ContactEditPage](t, f.h.do(postForm(ada+"/edit", f.sess, form)))
	if len(refused.Form.Errors) == 0 || refused.Form.Errors[0].Name != "date" || f.srv.CountRequests("PUT", f.col) != puts {
		t.Errorf("a wrong day: errors %+v", refused.Form.Errors)
	}

	// A new contact shows both rubrics with their buttons (s-contacts-27).
	fresh := decode[view.ContactEditPage](t, f.h.do(get(f.path("contacts")+"/new", f.sess)))
	adds := map[string]bool{}
	for _, s := range fresh.Form.Sets {
		for _, r := range s.Rows {
			if r.Kind == view.InputAdd {
				adds[string(s.Legend)+"/"+string(r.Value)] = true
			}
		}
	}
	if !adds["Websites/url"] || !adds["Dates/date"] {
		t.Errorf("a new contact's buttons = %v, want Add website in Websites and Add date in Dates", adds)
	}
}
