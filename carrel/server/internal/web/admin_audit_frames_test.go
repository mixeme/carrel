package web_test

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Ад24, Ад25 of the scan of 15.10.2026 (s-admin-7): the journal is read from
// a day — a date in the bar's form, kept by the segments and by Export CSV —
// and a line that says something did not happen has its action in alarm.
func TestTheJournalIsReadFromADay(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	alice := h.login("alice", "password123")
	if _, _, ok, _ := h.users.Authenticate(context.Background(), "alice", "wrong-password"); ok {
		t.Fatal("a wrong password signed in")
	}

	page := decode[view.AdminAuditPage](t, h.do(get("/admin/audit?since=2000-01-01", alice)))
	failed := 0
	for _, r := range page.Rows {
		if r.Failure != strings.HasSuffix(string(r.What), ".fail") {
			t.Errorf("%s: failure %v", r.What, r.Failure)
		}
		if r.Failure {
			failed++
		}
	}
	if failed == 0 {
		t.Errorf("no line in alarm among %+v", page.Rows)
	}
	since := formRow(page.Filter, "since")
	if since == nil || since.Kind != view.InputDate || since.Value != "2000-01-01" {
		t.Errorf("since = %+v", since)
	}
	for _, a := range append(page.Bar.Segments, page.Bar.Actions...) {
		if u, _ := url.Parse(a.Href); u.Query().Get("since") != "2000-01-01" {
			t.Errorf("%s does not keep the day: %s", a.ID, a.Href)
		}
	}
	if later := decode[view.AdminAuditPage](t, h.do(get("/admin/audit?since=2999-01-01", alice))); len(later.Rows) != 0 {
		t.Errorf("lines before the day: %+v", later.Rows)
	}
	if csv := h.do(get("/admin/audit?format=csv&since=2999-01-01", alice)).Body.String(); strings.Count(csv, "\n") != 1 {
		t.Errorf("the export holds lines before the day:\n%s", csv)
	}
}

// Ад35 of the scan (s-admin-6): «never» and «not deposited» are dimmed; the
// row says whether its last recovery is a moment.
func TestEscrowRowsSayWhetherRecovered(t *testing.T) {
	h := newHarness(t)
	const master = "sixteen-char-pass!"
	h.enableEscrow("alice", "password123", master)
	alice := h.login("alice", "password123")
	snap := h.st.Snapshot()
	h.invite(snap.UserByLogin("alice").ID, "bob", "bobpass123")
	h.do(postForm("/admin/escrow", alice, url.Values{"action": {"recover"}, "login": {"bob"}, "master": {master}}))
	page := decode[view.AdminEscrowPage](t, h.do(get("/admin/escrow", alice)))
	for _, r := range page.Covered {
		if r.Recovered != (r.Login == "bob") {
			t.Errorf("%s: recovered %v, last %q", r.Login, r.Recovered, r.LastRecovery)
		}
	}
}
