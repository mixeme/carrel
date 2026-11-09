package web_test

import (
	"net/http"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// A published book with a card Carrel could not read (the owner's decision
// of 04.11.2026, Б3, variant Б; Typecase's note 181 p. 3, task 255): the
// guest gets the page with the cards Carrel read, not the one it could not,
// and the owner's list of links says so by the link's name — «1 record not in
// the link», why in the badge's hint (s-publish-40).
func TestAPublishedBookLeavesOutWhatCarrelCouldNotRead(t *testing.T) {
	frame := mockupFrame(t, "s-publish-40")
	const badge, hint = "1 record not in the link", "Carrel could not read it: invalid vCard. Unchanged on the server."
	if !strings.Contains(frame, `title="`+hint+`">`+badge+`</span>`) {
		t.Fatal("s-publish-40 does not draw the badge and its hint: read the frame again")
	}
	f := unreadableBookFixture(t)
	link := publishLink(t, f, f.publishHref("contacts", davtest.AddressBookHome+"people/", ""), true)
	resp := f.h.do(anon(link))
	if resp.Code != http.StatusOK {
		t.Fatalf("the guest's page: %d", resp.Code)
	}
	page := decode[view.PublishedPage](t, resp)
	var names []string
	for _, r := range publicRows(page) {
		names = append(names, string(r.Name))
	}
	if got := strings.Join(names, ","); got != "ada,bob" || len(page.Notices) != 0 {
		t.Errorf("the guest sees %q, notices %+v; want ada and bob, nothing about the card", got, page.Notices)
	}
	list := decode[view.PublishedLinksPage](t, f.h.do(get("/app/files/published", f.sess)))
	if len(list.Rows) != 1 || list.Rows[0].Missing != badge || list.Rows[0].MissingHint != hint {
		t.Errorf("the row of the link = %+v, want %q %q", list.Rows, badge, hint)
	}
}
