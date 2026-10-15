package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// review Р-10: a phone or an address row showed its label ("work",
// "cell") and its address, but neither could be changed (s-contacts-3). The
// label is a choice of the usual ones plus what the row already has; an
// address is five parts, and the two parts no field offers (PO box,
// extended) stay as they were. Changing one row writes that row.
func TestContactRowLabelsAndAddresses(t *testing.T) {
	const card = "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\n" +
		"TEL;TYPE=work:+44 20 1\nTEL;TYPE=x-lab:+44 20 2\nEMAIL:ada@example.com\n" +
		"ADR;TYPE=home:PO 7;Flat 2;12 St James's Square;London;;SW1Y 4JH;UK\nX-CUSTOM:keep\nEND:VCARD\n"
	f := contactsFixture(t, false, card)
	ada := f.path("contacts") + "/ada"

	edit := decode[view.ContactEditPage](t, f.h.do(get(ada+"/edit", f.sess)))
	form := formFields(edit.Form)
	if form.Get("tel.0.type") != "work" || form.Get("tel.1.type") != "x-lab" || form.Get("email.0.type") != "" {
		t.Errorf("labels = %v", form)
	}
	lab := rowNamed(t, edit.Form, "tel.1.type")
	found := false
	for _, o := range lab.Options {
		found = found || (o.Value == "x-lab" && o.Selected)
	}
	if !found {
		t.Errorf("a label another client wrote is not kept as a choice: %+v", lab.Options)
	}
	if form.Get("adr.0.street") != "12 St James's Square" || form.Get("adr.0.city") != "London" ||
		form.Get("adr.0.code") != "SW1Y 4JH" || form.Get("adr.0.country") != "UK" || form.Get("adr.0.region") != "" {
		t.Errorf("address = %v", form)
	}
	if form.Get("adr.0.type") != "home" {
		t.Errorf("the address's label = %q", form.Get("adr.0.type"))
	}
	if add := addRow(t, edit.Form, "adr"); len(add.Template) != 7 || add.Template[0].Name != "adr.{n}.type" ||
		add.Template[1].Name != "adr.{n}.street" || add.Template[6].Kind != view.InputRemove {
		t.Errorf("Add address = %+v", add)
	}

	save := func(set map[string]string) string {
		t.Helper()
		form := formFields(decode[view.ContactEditPage](t, f.h.do(get(ada+"/edit", f.sess))).Form)
		for k, v := range set {
			form.Set(k, v)
		}
		if resp := f.h.do(postForm(ada+"/edit", f.sess, form)); resp.Code != 303 {
			t.Fatalf("POST %v: %d %s", set, resp.Code, resp.Body.String())
		}
		return body(t, f, "ada.vcf")
	}

	got := save(map[string]string{"tel.0.type": "cell"})
	mustContain(t, got, "TEL;TYPE=cell:+44 20 1", "TEL;TYPE=x-lab:+44 20 2", "EMAIL:ada@example.com",
		"ADR;TYPE=home:PO 7;Flat 2;12 St James's Square;London;;SW1Y 4JH;UK", "X-CUSTOM:keep")

	got = save(map[string]string{"email.0.type": "work", "tel.1.type": ""})
	mustContain(t, got, "EMAIL;TYPE=work:ada@example.com", "TEL:+44 20 2", "TEL;TYPE=cell:+44 20 1")

	got = save(map[string]string{"adr.0.region": "Westminster"})
	mustContain(t, got, "ADR;TYPE=home:PO 7;Flat 2;12 St James's Square;London;Westminster;SW1Y 4JH;UK")

	// The label is picked as a phone's is (s-contacts-3), and only it moves.
	got = save(map[string]string{"adr.0.type": "work"})
	mustContain(t, got, "ADR;TYPE=work:PO 7;Flat 2;12 St James's Square;London;Westminster;SW1Y 4JH;UK")

	// A new address comes without PO box and extended parts; a semicolon
	// typed in a part is dropped, or it would shift every part after it.
	got = save(map[string]string{"adr.1.street": "1 Lab; Lane", "adr.1.city": "Oxford"})
	mustContain(t, got, "ADR:;;1 Lab Lane;Oxford;;;")

	// An address emptied of every part goes.
	got = save(map[string]string{"adr.1.street": "", "adr.1.city": ""})
	if strings.Count(got, "ADR") != 1 {
		t.Errorf("the emptied address stayed:\n%s", got)
	}
}

// A label changed alone is a change too: when the card moved on the server
// meanwhile (S-8), the conflict screen names the phone under each label, and
// "Use only my edit" writes the label.
func TestRowLabelSurvivesTheConflictScreen(t *testing.T) {
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nTEL;TYPE=work:+44 20 1\nEND:VCARD\n")
	rec := f.path("contacts") + "/ada"

	form := formFields(decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form)
	f.srv.Seed(f.col+"ada.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Byron\nTEL;TYPE=work:+44 20 1\nEND:VCARD\n")
	form.Set("tel.0.type", "cell")
	if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != 303 || resp.Header().Get("Location") != rec+"/conflict" {
		t.Fatalf("POST over a moved version: %d %q", resp.Code, resp.Header().Get("Location"))
	}
	page := decode[view.ConflictPage](t, f.h.do(get(rec+"/conflict", f.sess)))
	// FN, which only the server changed, is a card of its own, the server's
	// chosen (Рв2, Рв4; until v0.2.271 it was not shown). The phone is a
	// card per label (Рв6, s-conflict-1: TEL;work): the number came under
	// cell and left work; until v0.2.273 one card «TEL» with the label after
	// each number.
	want := []view.ConflictRow{
		{Property: "FN", Mine: "Ada Lovelace", Theirs: "Ada Byron", Choice: view.ChoiceTheirs, Name: "choice.0", Diff: view.DiffTheirs},
		{Property: "TEL;cell", Mine: "+44 20 1", Theirs: "", Choice: view.ChoiceMine, Name: "choice.1", Diff: view.DiffMine},
		{Property: "TEL;work", Mine: "", Theirs: "+44 20 1", Choice: view.ChoiceMine, Name: "choice.2", Diff: view.DiffMine},
	}
	if len(page.Rows) != len(want) || page.Rows[0] != want[0] || page.Rows[1] != want[1] || page.Rows[2] != want[2] {
		t.Fatalf("rows = %+v", page.Rows)
	}
	if resp := f.h.do(postForm(actionHref(t, page.Actions, "conflict.mine"), f.sess, url.Values{})); resp.Code != 303 {
		t.Fatalf("mine: %d %s", resp.Code, resp.Body.String())
	}
	mustContain(t, body(t, f, "ada.vcf"), "TEL;TYPE=cell:+44 20 1", "FN:Ada Byron")
}
