package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// S-8, Рв6 (s-conflict-1): a phone is a card per label, and a property the
// form does not show — the file-as line — is one of its own when only the
// server changed it. «Your edit» of such a card puts back what was opened:
// the rows of that label, the property's line as it was. The rows under the
// other labels and every other property stay as the server holds them.
func TestConflictCardsByLabelAndOutsideTheForm(t *testing.T) {
	const opened = "BEGIN:VCARD\nVERSION:3.0\nUID:ada\nFN:Ada Lovelace\nTEL;TYPE=WORK:+1\nTEL;TYPE=CELL:+2\n" +
		"X-EVOLUTION-FILE-AS:Lovelace\\, Ada\nX-KEEP:same\nEND:VCARD\n"
	const moved = "BEGIN:VCARD\nVERSION:3.0\nUID:ada\nFN:Ada Lovelace\nTEL;TYPE=WORK:+9\nTEL;TYPE=CELL:+2\n" +
		"X-EVOLUTION-FILE-AS:Byron\\, Ada\nX-KEEP:same\nEND:VCARD\n"
	refuse := func(t *testing.T) (davFixture, string, view.ConflictPage) {
		t.Helper()
		f := contactsFixture(t, false, opened)
		rec := f.path("contacts") + "/ada"
		form := formFields(decode[view.ContactEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form)
		f.srv.Seed(f.col+"ada.vcf", moved)
		form.Set("tel.1", "+3")
		if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != rec+"/conflict" {
			t.Fatalf("POST over a moved version: %d %q", resp.Code, resp.Header().Get("Location"))
		}
		return f, rec, decode[view.ConflictPage](t, f.h.do(get(rec+"/conflict", f.sess)))
	}

	f, rec, page := refuse(t)
	want := []view.ConflictRow{
		{Property: "TEL;work", Mine: "+1", Theirs: "+9", Choice: view.ChoiceTheirs, Name: "choice.0", Diff: view.DiffTheirs},
		{Property: "TEL;cell", Mine: "+3", Theirs: "+2", Choice: view.ChoiceMine, Name: "choice.1", Diff: view.DiffMine},
		{Property: "X-EVOLUTION-FILE-AS", Mine: "Lovelace, Ada", Theirs: "Byron, Ada", Choice: view.ChoiceTheirs, Name: "choice.2", Diff: view.DiffTheirs},
	}
	if len(page.Rows) != len(want) || page.Rows[0] != want[0] || page.Rows[1] != want[1] || page.Rows[2] != want[2] {
		t.Fatalf("rows = %+v", page.Rows)
	}
	choice := formFields(*page.Form)
	choice.Set("choice.0", "mine")
	choice.Set("choice.2", "mine")
	if resp := f.h.do(postForm(page.Form.Action, f.sess, choice)); resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != rec {
		t.Fatalf("combine: %d %q %s", resp.Code, resp.Header().Get("Location"), resp.Body.String())
	}
	got := body(t, f, "ada.vcf")
	mustContain(t, got, "TEL;TYPE=WORK:+1", "TEL;TYPE=CELL:+3", "X-EVOLUTION-FILE-AS:Lovelace\\, Ada", "X-KEEP:same")
	if strings.Contains(got, "+9") || strings.Contains(got, "Byron") || strings.Contains(got, "+2") {
		t.Errorf("the server's rows outlived the choice:\n%s", got)
	}

	// «Use only my edit» writes what the person changed and nothing else.
	f, _, page = refuse(t)
	if resp := f.h.do(postForm(actionHref(t, page.Actions, "conflict.mine"), f.sess, url.Values{})); resp.Code != http.StatusSeeOther {
		t.Fatalf("mine: %d %s", resp.Code, resp.Body.String())
	}
	mustContain(t, body(t, f, "ada.vcf"), "TEL;TYPE=WORK:+9", "TEL;TYPE=CELL:+3", "X-EVOLUTION-FILE-AS:Byron\\, Ada")
}
