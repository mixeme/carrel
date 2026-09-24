package web_test

import (
	"net/http"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// A collection that did not answer the poll says so in the rail, in a few
// words and in full, with a Retry of it alone — as every frame with a rail
// has drawn it («password · 401»). Until 24.09.2026 the rail marked only an
// unreachable server, and said nothing of why.
func TestTheRailSaysWhatDidNotAnswer(t *testing.T) {
	f := contactsFixture(t, false, vcardOf("ada", "Ada Lovelace"))
	tickRail(t, f.h, f.sess, "contacts", f.path("contacts"))
	f.srv.Refuse("REPORT", http.StatusUnauthorized, "")
	page := combined(t, f.h, f.sess, "/app/contacts", contactsPoll)
	_, row := railRow(t, page.Rail, "People")
	if row.State != view.CollectionUnavailable || row.Message != "password · 401" ||
		!strings.Contains(string(row.Detail), "saved password was refused") {
		t.Errorf("row = %+v", row)
	}
	if row.Retry == nil || row.Retry.ID != "source.retry" || !strings.Contains(row.Retry.Href, "/retry?source=") {
		t.Errorf("retry = %+v", row.Retry)
	}

	f.srv.Accept("REPORT")
	page = combined(t, f.h, f.sess, "/app/contacts", contactsPoll)
	if _, row := railRow(t, page.Rail, "People"); row.State == view.CollectionUnavailable || row.Message != "" || row.Retry != nil {
		t.Errorf("after the server answered: %+v", row)
	}
}
