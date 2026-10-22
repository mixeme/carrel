package web_test

import (
	"mime"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// «Send this link» (s-mail-4; the owner's decision of 21.10.2026): a link
// given out goes by a letter of its own, never into the letter being
// written. Send by letter of a link just made opens its sheet — over
// Published links —; the letter carries the line typed and the same link,
// not a second one; the letter being written waits as it was.
func TestALinkGoesByALetterOfItsOwn(t *testing.T) {
	f, relay := letterFixture(t)
	typed := url.Values{"to": {"ann@example.org"}, "subject": {"Minutes"}, "text": {"Dear Ann,"}}
	if rec := f.h.do(postForm("/app/mail/send?op=preview", f.sess, typed)); rec.Code >= 400 {
		t.Fatalf("typing: %d", rec.Code)
	}
	made := decode[view.PublishSheet](t, f.h.do(postForm("/app/publish?"+url.Values{"k": {"contacts"}, "a": {f.id}, "c": {searchBook}, "uid": {"ada"}}.Encode(), f.sess, url.Values{})))
	mail := actionByID(made.Actions, "publish.mail")
	if mail == nil || mail.Method != "GET" || !strings.HasPrefix(mail.Href, "/app/mail/link?id=") {
		t.Fatalf("Send by letter = %+v", mail)
	}
	sheet := decode[view.MailLinkSheet](t, f.h.do(get(mail.Href, f.sess)))
	if sheet.Subject != "Ada Lovelace" || len(sheet.Carries) != 3 || !strings.Contains(string(sheet.Carries[0]), "nothing of the contact itself") ||
		!strings.Contains(string(sheet.Carries[2]), "as the link does") || sheet.From == "" {
		t.Errorf("sheet = %+v", sheet)
	}
	if sheet.Under == nil || sheet.Under.Published == nil || len(sheet.Under.Published.Rows) != 1 {
		t.Errorf("under = %+v, want Published links", sheet.Under)
	}
	if cancel := actionByID(sheet.Actions, "mail.cancel"); cancel == nil || cancel.Href != "/app/files/published" {
		t.Errorf("Cancel = %+v", cancel)
	}

	// Nobody to send to: the sheet again, with why at To.
	rec := f.h.do(postForm(sheet.Form.Action, f.sess, url.Values{"to": {""}, "line": {"Here it is"}}))
	if rec.Code != http.StatusOK {
		t.Fatalf("no recipient: %d", rec.Code)
	}
	again := decode[view.MailLinkSheet](t, rec)
	if to := formRow(again.Form, "to"); to == nil || to.Error != "Add a recipient." || formRow(again.Form, "line").Value != "Here it is" {
		t.Errorf("to = %+v", formRow(again.Form, "to"))
	}

	rec = f.h.do(postForm(sheet.Form.Action, f.sess, url.Values{"to": {"bob@example.org"}, "line": {"Here it is"}}))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/app/files/published?sent=1" {
		t.Fatalf("send: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	letters := relay.Letters()
	if len(letters) != 1 {
		t.Fatalf("letters = %d", len(letters))
	}
	head, body := letterBody(t, letters[0])
	if head.Get("Subject") != "Ada Lovelace" || !strings.Contains(body, "Here it is") || !strings.Contains(body, "http://example.com"+string(made.Link)) {
		t.Errorf("letter %q:\n%s", head.Get("Subject"), body)
	}
	if n := len(f.h.users.PublishedLinks(f.sess.AccountID())); n != 1 {
		t.Errorf("links = %d, want the same one", n)
	}
	page := decode[view.PublishedLinksPage](t, f.h.do(get("/app/files/published?sent=1", f.sess)))
	if len(page.Notices) != 1 || page.Notices[0].Title != "The letter went to 1 recipient." {
		t.Errorf("notices = %+v", page.Notices)
	}
	letter := decode[view.MailSheet](t, f.h.do(get("/app/mail/compose", f.sess)))
	if len(letterTo(letter)) != 1 || letterValue(letter, "subject") != "Minutes" || letterValue(letter, "text") != "Dear Ann," || len(letter.Attach) != 0 {
		t.Errorf("the letter being written changed: %+v", letter)
	}

	// Copy link instead sends nothing: the link's ready sheet, its address
	// whole to copy (s-publish-13; Typecase's note 110 p. 5), closing back
	// to Published links.
	rec = f.h.do(postForm(actionHref(t, sheet.Actions, "mail.link.copy"), f.sess, url.Values{"to": {"bob@example.org"}}))
	ready := decode[view.PublishSheet](t, rec)
	if rec.Code != http.StatusOK || len(relay.Letters()) != 1 {
		t.Errorf("copy instead: %d, letters %d", rec.Code, len(relay.Letters()))
	}
	whole := formRow(ready.Form, "link")
	if whole == nil || string(whole.Value) != "http://example.com"+string(made.Link) || !whole.ReadOnly || ready.Title != "Publish a contact" {
		t.Errorf("copy instead = %q, link %+v", ready.Title, whole)
	}
	if c := actionHref(t, ready.Actions, "publish.close"); c != "/app/files/published" {
		t.Errorf("its cross leads to %q", c)
	}
}

// A link is kept whole in the person's blob (the owner's decision of
// 19.09.2026), so Send of a row of Published links writes it from another
// session too, named as the list names it, with its days (Пб16; s-mail-4:
// «Личный · 10–31 August»). The instance keeps only its hash.
func TestALinkGoesByLetterFromPublishedLinks(t *testing.T) {
	f := calendarFixture(t, func(h *harness) { h.cfg.MailEnabled = true })
	relay := f.h.relayed()
	q := url.Values{"k": {"calendar"}, "a": {f.id}, "c": {f.col}}
	if rec := f.h.do(postForm("/app/publish?"+q.Encode(), f.sess, url.Values{
		"how": {"range"}, "from": {"2026-10-01"}, "to": {"2026-10-10"}, "download": {"on"},
	})); rec.Code != 200 {
		t.Fatalf("publish: %d %s", rec.Code, rec.Body)
	}
	later := f.h.login("owner", "password123")
	page := decode[view.PublishedLinksPage](t, f.h.do(get("/app/files/published", later)))
	if len(page.Rows) != 1 || !strings.HasSuffix(string(page.Rows[0].Title), " · 1–10 October") {
		t.Fatalf("rows = %+v", page.Rows)
	}
	sheet := decode[view.MailLinkSheet](t, f.h.do(get(actionHref(t, page.Rows[0].Actions, "publish.mail"), later)))
	if sheet.Subject != page.Rows[0].Title || !strings.Contains(string(sheet.Carries[0]), "nothing of the calendar itself") {
		t.Errorf("sheet = %+v", sheet)
	}
	if rec := f.h.do(postForm(sheet.Form.Action, later, url.Values{"to": {"ann@example.org"}})); rec.Code != http.StatusSeeOther {
		t.Fatalf("send: %d %s", rec.Code, rec.Body)
	}
	head, body := letterBody(t, relay.Letters()[0])
	subject, _ := new(mime.WordDecoder).DecodeHeader(head.Get("Subject"))
	if subject != string(page.Rows[0].Title) || !strings.Contains(body, "http://example.com"+string(page.Rows[0].Link)) {
		t.Errorf("letter %q:\n%s", subject, body)
	}
	if snap := f.h.st.Snapshot(); len(snap.Published) != 1 || strings.Contains(string(snap.Published[0].Hash), strings.TrimPrefix(string(page.Rows[0].Link), "/p/")) {
		t.Errorf("published = %+v", snap.Published)
	}
}

// Б36 of the audit of 11.10.2026 is gone with its cause: what a person took
// back has no sheet and no letter — a revoked link is not there to send.
func TestARevokedLinkHasNoLetter(t *testing.T) {
	f, relay := letterFixture(t)
	made := decode[view.PublishSheet](t, f.h.do(postForm("/app/publish?"+url.Values{"k": {"contacts"}, "a": {f.id}, "c": {searchBook}, "uid": {"ada"}}.Encode(), f.sess, url.Values{})))
	href := actionHref(t, made.Actions, "publish.mail")
	sheet := decode[view.MailLinkSheet](t, f.h.do(get(href, f.sess)))
	for _, l := range f.h.users.PublishedLinks(f.sess.AccountID()) {
		if err := f.h.users.RevokePublished(f.sess.AccountID(), l.ID); err != nil {
			t.Fatal(err)
		}
	}
	if rec := f.h.do(get(href, f.sess)); rec.Code != http.StatusNotFound {
		t.Errorf("sheet of a revoked link: %d", rec.Code)
	}
	if rec := f.h.do(postForm(sheet.Form.Action, f.sess, url.Values{"to": {"ann@example.org"}})); rec.Code != http.StatusNotFound {
		t.Errorf("letter of a revoked link: %d", rec.Code)
	}
	if n := len(relay.Letters()); n != 0 {
		t.Errorf("letters = %d", n)
	}
}
