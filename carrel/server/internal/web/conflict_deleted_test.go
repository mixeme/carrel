package web_test

import (
	"net/http"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Р-67 (Typecase's note 77, s-conflict-7): a record deleted on the server
// while it was edited is the conflict screen too — what happened under the
// title, one card by the record's name, the edit against nothing, no side
// chosen and «Write this choice» waiting for one. «Your edit · write it
// back» writes the record anew with the edit; «Server · leave it deleted»
// drops the edit and leads to the list.
func TestARecordDeletedWhileEditedIsAChoice(t *testing.T) {
	deleted := func(t *testing.T) (rewriteStand, view.ConflictPage) {
		f := newRewriteStand(t)
		rec := f.path("contacts") + "/ada"
		form := formFields(decode[editedForm](t, f.h.do(get(rec+"/edit", f.sess))).Form)
		f.srv.Unseed(peopleBook + "ada.vcf")
		form.Set("nickname", "Countess")
		if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != rec+"/conflict" {
			t.Fatalf("save over a deleted card: %d %s, want the conflict screen", resp.Code, resp.Header().Get("Location"))
		}
		return f, decode[view.ConflictPage](t, f.h.do(get(rec+"/conflict", f.sess)))
	}
	f, page := deleted(t)
	if !page.Deleted || !strings.Contains(string(page.Subtitle), "deleted this contact") || page.Ask != "Choose a side." {
		t.Errorf("deleted %v, subtitle %q, ask %q", page.Deleted, page.Subtitle, page.Ask)
	}
	if len(page.Rows) != 1 {
		t.Fatalf("rows = %+v, want the one card", page.Rows)
	}
	if r := page.Rows[0]; r.Property != "Ada Lovelace" || r.Mine != "Ada Lovelace · Countess" || r.Theirs != "" || r.Choice != view.ChoiceUnset || r.Name != "choice.0" {
		t.Errorf("card = %+v", r)
	}
	if page.Form == nil || len(page.Form.Actions) != 1 {
		t.Fatalf("form = %+v", page.Form)
	}
	if c := page.Form.Actions[0]; c.Label != "Write this choice" || !c.Enabled || !c.Waits || c.Reason != "Choose a side." {
		t.Errorf("write = %+v", c)
	}
	if len(page.Actions) != 1 || page.Actions[0].ID != "conflict.cancel" {
		t.Errorf("actions = %+v, want Cancel alone", page.Actions)
	}

	// No side: nothing is written, the choice is shown again.
	again := decode[view.ConflictPage](t, f.h.do(postForm(page.Form.Action, f.sess, formFields(*page.Form))))
	if !again.Deleted || !hasNotice(again.Notices, "Nothing was written") {
		t.Errorf("no side chosen: %+v", again.Notices)
	}
	if _, _, ok := f.srv.Object(peopleBook + "ada.vcf"); ok {
		t.Fatal("a choice with no side wrote the card")
	}

	// Write it back: the card is there again, with the edit.
	mine := formFields(*page.Form)
	mine.Set("choice.0", string(view.ChoiceMine))
	if resp := f.h.do(postForm(page.Form.Action, f.sess, mine)); resp.Code != http.StatusSeeOther {
		t.Fatalf("write it back: %d %s", resp.Code, resp.Body.String())
	}
	body, _, ok := f.srv.Object(peopleBook + "ada.vcf")
	if !ok {
		t.Fatal("the card was not written back")
	}
	for _, line := range []string{"UID:ada", "FN:Ada Lovelace", "NICKNAME:Countess", "ada@old.example"} {
		mustContain(t, unfolded(body), line)
	}

	// Leave it deleted: nothing is written, and the person is at the list.
	g, page := deleted(t)
	theirs := formFields(*page.Form)
	theirs.Set("choice.0", string(view.ChoiceTheirs))
	resp := g.h.do(postForm(page.Form.Action, g.sess, theirs))
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != g.path("contacts") {
		t.Fatalf("leave it deleted: %d %s", resp.Code, resp.Header().Get("Location"))
	}
	if _, _, ok := g.srv.Object(peopleBook + "ada.vcf"); ok {
		t.Error("leaving it deleted wrote the card")
	}
	if resp := g.h.do(get(g.path("contacts")+"/ada/conflict", g.sess)); resp.Code != http.StatusSeeOther {
		t.Errorf("the edit outlived its answer: GET conflict %d", resp.Code)
	}
}

// Р-69 (Typecase's note 77): when the server already holds everything the
// edit changed there is no screen — nothing is written, the draft goes,
// and the person is at the card, as after a save. «Nothing differs» and
// its two buttons switched off are gone.
func TestNothingToChooseIsNoScreen(t *testing.T) {
	f := newRewriteStand(t)
	rec := f.path("contacts") + "/ada"
	form := formFields(decode[editedForm](t, f.h.do(get(rec+"/edit", f.sess))).Form)
	f.srv.Seed(peopleBook+"ada.vcf", strings.Replace(rewriteCard, "FN:Ada Lovelace\r\n", "FN:Ada Lovelace\r\nNICKNAME:Countess\r\n", 1))
	form.Set("nickname", "Countess")
	resp := f.h.do(postForm(rec+"/edit", f.sess, form))
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("save: %d", resp.Code)
	}
	if at := resp.Header().Get("Location"); at == rec+"/conflict" {
		resp = f.h.do(get(at, f.sess))
		if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != rec {
			t.Fatalf("GET conflict with nothing to choose: %d %s, want the card", resp.Code, resp.Header().Get("Location"))
		}
	}
	if resp := f.h.do(get(rec+"/conflict", f.sess)); resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != rec {
		t.Errorf("the draft outlived it: %d %s", resp.Code, resp.Header().Get("Location"))
	}
}
