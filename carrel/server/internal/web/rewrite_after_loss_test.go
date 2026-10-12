package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Б44 of the audit of 11.10.2026: an address replaced on a card is asked
// about — the records that named the old one — unless the save of the card
// came back with a loss report: then the question was dropped and never
// asked, and those records fell out of the person's timeline without a
// word. The loss is still said first; the question stands beside it.
func TestTheAddressQuestionSurvivesALossReport(t *testing.T) {
	f := newRewriteStand(t)
	f.srv.Drop = []string{"NICKNAME"}
	rec := f.path("contacts") + "/ada"
	form := formFields(decode[editedForm](t, f.h.do(get(rec+"/edit", f.sess))).Form)
	form.Set("email.0", "ada@new.example")
	form.Set("nickname", "Countess")
	resp := f.h.do(postForm(rec+"/edit", f.sess, form))
	if resp.Code != http.StatusOK {
		t.Fatalf("save: %d %s, want the loss report", resp.Code, resp.Header().Get("Location"))
	}
	page := decode[view.ContactEditPage](t, resp)
	if page.Loss == nil {
		t.Fatalf("no loss report: the stand kept the nickname")
	}
	var sheet string
	for _, n := range page.Notices {
		for _, a := range n.Actions {
			if a.ID == "contact.rewrite" {
				sheet = a.Href
			}
		}
	}
	if sheet == "" {
		t.Fatalf("the question about the replaced address is gone: notices = %+v", page.Notices)
	}
	asked := decode[view.RewriteSheet](t, f.h.do(get(sheet, f.sess)))
	if len(asked.Pairs) != 1 || !strings.Contains(string(asked.Pairs[0].From), "ada@old.example") || !strings.Contains(string(asked.Pairs[0].To), "ada@new.example") {
		t.Errorf("the sheet behind the notice = %+v", asked.Pairs)
	}
}

// The same question on the way through the conflict screen: the card
// changed elsewhere, the person's edit is written from that screen, and the
// replaced address is asked about as after any other save.
func TestTheAddressQuestionIsAskedAfterAConflict(t *testing.T) {
	f := newRewriteStand(t)
	rec := f.path("contacts") + "/ada"
	form := formFields(decode[editedForm](t, f.h.do(get(rec+"/edit", f.sess))).Form)
	f.srv.Seed(peopleBook+"ada.vcf", strings.Replace(rewriteCard, "FN:Ada Lovelace", "FN:Ada Lovelace\r\nNOTE:written elsewhere", 1))
	form.Set("email.0", "ada@new.example")
	if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != rec+"/conflict" {
		t.Fatalf("save: %d %s, want the conflict screen", resp.Code, resp.Header().Get("Location"))
	}
	page := decode[view.ConflictPage](t, f.h.do(get(rec+"/conflict", f.sess)))
	resp := f.h.do(postForm(actionHref(t, page.Actions, "conflict.mine"), f.sess, url.Values{}))
	if resp.Code != http.StatusSeeOther || !strings.Contains(resp.Header().Get("Location"), "rewrite") {
		t.Fatalf("use my edit: %d %s, want the question about the replaced address", resp.Code, resp.Header().Get("Location"))
	}
}

// Б43 of the audit of 11.10.2026: a refused edit is measured against the
// version the form was opened with. When Carrel no longer holds that
// version, every field that differs from the server's is listed as the
// person's change — also what only the other client changed, and «Use only
// my edit» would put the old value back. The screen says so.
func TestTheConflictScreenSaysWhenItCannotTellWhoseChange(t *testing.T) {
	f := newRewriteStand(t)
	rec := f.path("contacts") + "/ada"
	form := formFields(decode[editedForm](t, f.h.do(get(rec+"/edit", f.sess))).Form)
	form.Set("etag", "a-version-nobody-holds")
	form.Set("nickname", "Countess")
	if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != rec+"/conflict" {
		t.Fatalf("save: %d %s, want the conflict screen", resp.Code, resp.Header().Get("Location"))
	}
	page := decode[view.ConflictPage](t, f.h.do(get(rec+"/conflict", f.sess)))
	if !hasNotice(page.Notices, "Some of these may not be your changes") {
		t.Errorf("notices = %+v", page.Notices)
	}

	// With the version held, nothing of the kind is said.
	g := newRewriteStand(t)
	form = formFields(decode[editedForm](t, g.h.do(get(g.path("contacts")+"/ada/edit", g.sess))).Form)
	g.srv.Seed(peopleBook+"ada.vcf", strings.Replace(rewriteCard, "FN:Ada Lovelace", "FN:Ada Lovelace\r\nNOTE:written elsewhere", 1))
	form.Set("nickname", "Countess")
	if resp := g.h.do(postForm(g.path("contacts")+"/ada/edit", g.sess, form)); resp.Code != http.StatusSeeOther {
		t.Fatalf("save: %d", resp.Code)
	}
	if page := decode[view.ConflictPage](t, g.h.do(get(g.path("contacts")+"/ada/conflict", g.sess))); hasNotice(page.Notices, "Some of these may not be your changes") {
		t.Errorf("the version was held, and the screen doubts all the same: %+v", page.Notices)
	}
}
