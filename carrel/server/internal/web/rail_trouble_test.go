package web_test

import (
	"net/http"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// A collection that did not answer the poll says so in the rail — its mark
// of an error, the whole sentence in its hint (from-typecase note 32: no
// words beside the name, no Retry there) — and the poll bar carries the
// refusal in full with a Retry of it alone. Until 24.09.2026 the rail marked
// only an unreachable server, and said nothing of why.
func TestTheRailSaysWhatDidNotAnswer(t *testing.T) {
	f := contactsFixture(t, false, vcardOf("ada", "Ada Lovelace"))
	tickRail(t, f.h, f.sess, "contacts", f.path("contacts"))
	f.srv.Refuse("REPORT", http.StatusUnauthorized, "")
	page := combined(t, f.h, f.sess, "/app/contacts", contactsPoll)
	_, row := railRow(t, page.Rail, "People")
	if row.State != view.CollectionUnavailable || !strings.Contains(string(row.Detail), "saved password was refused") {
		t.Errorf("row = %+v", row)
	}
	people := sourceNamed(t, *page.Poll, "People")
	if people.Retry == nil || people.Retry.ID != "source.retry" || !strings.Contains(people.Retry.Href, "/retry?source=") ||
		!strings.HasPrefix(string(people.Answer), "REPORT ") || !strings.HasSuffix(string(people.Answer), " → 401 Unauthorized: the saved password was refused. Open Connections to update it.") ||
		page.Poll.Failed != "1 failed" {
		t.Errorf("the bar: %q, %+v", page.Poll.Failed, people)
	}

	f.srv.Accept("REPORT")
	page = combined(t, f.h, f.sess, "/app/contacts", contactsPoll)
	if _, row := railRow(t, page.Rail, "People"); row.State == view.CollectionUnavailable || row.Detail != "" {
		t.Errorf("after the server answered: %+v", row)
	}
	if page.Poll.Failed != "" || sourceNamed(t, *page.Poll, "People").Answer != "" {
		t.Errorf("after the server answered, the bar: %+v", page.Poll)
	}
}
