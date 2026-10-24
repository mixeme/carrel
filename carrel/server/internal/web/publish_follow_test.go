package web_test

import (
	"net/url"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Б41 of the audit of 11.10.2026: a published record did not follow a move —
// the guest read «No longer there» while the owner's list called the link
// alive —, and a removed collection told the guest to try later forever.
// Now a link goes where Carrel takes what it opens; where Carrel cannot
// follow, the owner's list says the link is dead, and the guest is told it
// is gone.
func TestAPublishedRecordFollowsItsMove(t *testing.T) {
	f := newMoveFixture(t)
	link := publishWith(t, f.davFixture, f.publishHref("contacts", peopleBook, "ada"))
	if page := publicPage(t, f.davFixture, link); page.Title != "Ada" {
		t.Fatalf("the link does not open the card: %+v", page)
	}
	row := func() view.PublishedRow {
		t.Helper()
		page := decode[view.PublishedLinksPage](t, f.h.do(get("/app/files/published", f.sess)))
		if len(page.Rows) != 1 {
			t.Fatalf("rows = %+v", page.Rows)
		}
		return page.Rows[0]
	}

	// Moved to another account: the link opens it there — read with that
	// account's own credentials.
	f.h.do(postConfirm(t, f, sheetFor(f, peopleBook, "ada", f.otherID, workBook, false)))
	page := publicPage(t, f.davFixture, link)
	if page.Title != "Ada" || len(page.Notices) != 0 {
		t.Errorf("after the move the link opens %q with %+v", page.Title, page.Notices)
	}
	if r := row(); r.Gone != "" {
		t.Errorf("a followed link is called dead: %q", r.Gone)
	}

	// Deleted: nothing to follow. The owner's list says so; the guest reads
	// that it is gone.
	_, etag, _ := f.other.Object(workBook + "ada.vcf")
	f.h.do(postForm(f.record(f.otherID, workBook, "ada")+"/delete", f.sess, url.Values{"etag": {etag}, "sheet": {"1"}, "copy": {f.otherID + "/" + url.PathEscape(workBook)}}))
	if r := row(); r.Gone == "" {
		t.Error("a link to a deleted record is called alive")
	}
	if n := noticeTitled(publicPage(t, f.davFixture, link).Notices, "No longer there."); n == nil {
		t.Error("the guest is not told it is gone")
	}
}

// A collection removed on its server: the guest is told it is gone, not to
// try later, and the owner's list says the link is dead.
func TestALinkToARemovedCollectionIsGone(t *testing.T) {
	f := newMoveFixture(t)
	link := publishWith(t, f.davFixture, f.publishHref("contacts", friendsBook, ""))
	if page := publicPage(t, f.davFixture, link); len(page.Notices) != 0 {
		t.Fatalf("the link does not open: %+v", page.Notices)
	}
	f.srv.RemoveCollection(friendsBook)
	page := publicPage(t, f.davFixture, link)
	if noticeTitled(page.Notices, "No longer there.") == nil {
		t.Errorf("notices = %+v, want it gone", page.Notices)
	}
	rows := decode[view.PublishedLinksPage](t, f.h.do(get("/app/files/published", f.sess))).Rows
	if len(rows) != 1 || rows[0].Gone == "" {
		t.Errorf("rows = %+v, want the link dead", rows)
	}
}
