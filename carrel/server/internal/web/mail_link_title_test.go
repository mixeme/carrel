package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// A link sent from the list of links is named in the letter as the list
// names it, with its days (Пб16; s-mail-4: «Личный · 10–31 August»): two
// links to one calendar are told apart.
func TestALinkGoesByLetterWithItsDays(t *testing.T) {
	f := calendarFixture(t, func(h *harness) { h.cfg.MailEnabled = true })
	f.h.relayed()
	q := url.Values{"k": {"calendar"}, "a": {f.id}, "c": {f.col}}
	if rec := f.h.do(postForm("/app/publish?"+q.Encode(), f.sess, url.Values{
		"how": {"range"}, "from": {"2026-10-01"}, "to": {"2026-10-10"}, "download": {"on"},
	})); rec.Code != 200 {
		t.Fatalf("publish: %d %s", rec.Code, rec.Body)
	}
	page := decode[view.PublishedLinksPage](t, f.h.do(get("/app/files/published", f.sess)))
	if len(page.Rows) != 1 || !strings.HasSuffix(string(page.Rows[0].Title), " · 1–10 October") {
		t.Fatalf("rows = %+v", page.Rows)
	}
	if rec := f.h.do(postForm(actionHref(t, page.Rows[0].Actions, "publish.mail"), f.sess, url.Values{})); rec.Code != 303 {
		t.Fatalf("send this link: %d", rec.Code)
	}
	sheet := decode[view.MailSheet](t, f.h.do(get("/app/mail/compose", f.sess)))
	if len(sheet.Attach) != 1 || sheet.Attach[0].Title != page.Rows[0].Title {
		t.Errorf("the letter names the link %+v, the list %q", sheet.Attach, page.Rows[0].Title)
	}
}
