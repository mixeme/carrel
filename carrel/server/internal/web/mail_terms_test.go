package web_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/view"
)

// Note 70 of Typecase (task 165; s-mail-1; the owner's decision Б38): a
// thing that goes by letter as a link says its terms in its row — how long
// the link lives — and «Change…» opens the publish sheet over the letter
// with them; its main button is Apply, the link is still made when the
// letter goes, now with the terms chosen. Under the list one line says why
// these go as links and where they are revoked.
func TestALetterLinkHasItsTerms(t *testing.T) {
	f, _ := letterFixture(t)
	attachOne(t, f, url.Values{"what": {"file"}, "a": {f.id}, "c": {filesDocs}, "file": {"small.txt"}})
	attachOne(t, f, url.Values{"what": {"file"}, "a": {f.id}, "c": {filesDocs}, "file": {"big.bin"}})
	attachOne(t, f, url.Values{"what": {"folder"}, "a": {f.id}, "c": {filesDocs}, "f": {"share"}})
	sheet := decode[view.MailSheet](t, f.h.do(get("/app/mail/compose", f.sess)))
	if len(sheet.Attach) != 3 {
		t.Fatalf("attached = %+v", sheet.Attach)
	}
	if r := sheet.Attach[0]; r.Terms != "" || r.Change != nil {
		t.Errorf("a file attached has terms: %+v", r)
	}
	for _, r := range sheet.Attach[1:] {
		if r.Terms != "30 days" || r.Change == nil || r.Change.Label != "Change…" || r.Change.Method != "GET" || r.Change.Target != "sheet" {
			t.Errorf("%s: terms %q, change %+v", r.Title, r.Terms, r.Change)
		}
	}
	if sheet.Attach[1].Change == nil {
		t.FailNow()
	}
	links := runsText(sheet.Links)
	if !strings.HasPrefix(links, "Two of these go as links, not as files: ") || !strings.Contains(links, "and a folder always goes as a link.") ||
		!strings.HasSuffix(links, " — including for the people in this letter.") {
		t.Errorf("the line under the list = %q", links)
	}

	// Change…: the publish sheet with the row's terms; Apply keeps them in
	// the letter and makes nothing.
	pub := decode[view.PublishSheet](t, f.h.do(get(sheet.Attach[1].Change.Href, f.sess)))
	apply := actionByID(pub.Actions, "publish.apply")
	if apply == nil || apply.Label != "Apply" || !apply.Primary || actionByID(pub.Actions, "publish.create") != nil {
		t.Fatalf("the sheet over the letter = %+v", pub.Actions)
	}
	if lives := formRow(pub.Form, "lives"); lives == nil || lives.Value != "30" {
		t.Errorf("lives = %+v, want the letter's 30 days", lives)
	}
	before := len(f.h.users.PublishedLinks(f.sess.AccountID()))
	form := formFields(pub.Form)
	form.Set("lives", "7")
	rec := f.h.do(postForm(pub.Form.Action, f.sess, form))
	if rec.Code != 303 || rec.Header().Get("Location") != "/app/mail/compose" {
		t.Fatalf("apply: %d %q", rec.Code, rec.Header().Get("Location"))
	}
	if n := len(f.h.users.PublishedLinks(f.sess.AccountID())); n != before {
		t.Errorf("Apply made a link: %d → %d", before, n)
	}
	sheet = decode[view.MailSheet](t, f.h.do(get("/app/mail/compose", f.sess)))
	if sheet.Attach[1].Terms != "7 days" || sheet.Attach[2].Terms != "30 days" {
		t.Errorf("terms after Apply = %q, %q", sheet.Attach[1].Terms, sheet.Attach[2].Terms)
	}
	again := decode[view.PublishSheet](t, f.h.do(get(sheet.Attach[1].Change.Href, f.sess)))
	if lives := formRow(again.Form, "lives"); lives == nil || lives.Value != "7" {
		t.Errorf("the sheet again = %+v, want the terms chosen", lives)
	}

	// Sent: each link lives as its row said.
	if rec := f.h.do(postForm("/app/mail/compose?op=send", f.sess, url.Values{"to": {"ann@example.org"}, "subject": {"Files"}, "body": {"See"}})); rec.Code != 303 {
		t.Fatalf("send: %d %s", rec.Code, rec.Body)
	}
	days := map[string]int{}
	for _, l := range f.h.users.PublishedLinks(f.sess.AccountID()) {
		if l.ExpiresAt != nil {
			days[l.Target.Title] = int(time.Until(*l.ExpiresAt).Hours()/24 + 0.5)
		}
	}
	if days["big.bin"] != 7 || days["share"] != 30 {
		t.Errorf("links live = %v", days)
	}
}
