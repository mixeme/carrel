package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// review Р-12 (s-admin-7): the journal is read with filters — All ·
// Security · Failures — and handed over as CSV. The filter of the screen is
// the filter of the file.
func TestAuditFiltersAndExport(t *testing.T) {
	h := newHarness(t)
	h.admin("owner", "password123")
	sess := h.login("owner", "password123")
	// A wrong password writes login.fail (v0.0.138); the sign-ins above
	// wrote login.ok, and the setup wrote setup.first_admin.
	h.do(h.anonPostForm("/login", "/login", url.Values{"login": {"owner"}, "password": {"nope"}}))

	page := decode[view.AdminAuditPage](t, h.do(get("/admin/audit", sess)))
	seg := func(p view.AdminAuditPage, id string) *view.Action { return actionByID(p.Bar.Segments, id) }
	all, security, failures := seg(page, "audit.all"), seg(page, "audit.security"), seg(page, "audit.failures")
	if all == nil || security == nil || failures == nil {
		t.Fatalf("segments = %+v", page.Bar.Segments)
	}
	if !all.On || all.Enabled {
		t.Errorf("All = %+v, want the one shown", all)
	}
	if len(page.Rows) == 0 {
		t.Fatal("the journal is empty")
	}

	u, _ := url.Parse(failures.Href)
	only := decode[view.AdminAuditPage](t, h.do(get(u.RequestURI(), sess)))
	if len(only.Rows) == 0 {
		t.Fatalf("Failures shows nothing: %+v", only.Rows)
	}
	for _, row := range only.Rows {
		if !strings.HasSuffix(string(row.What), ".fail") {
			t.Errorf("Failures holds %q", row.What)
		}
	}
	if s := seg(only, "audit.failures"); s == nil || !s.On {
		t.Errorf("the chosen segment does not say it is on: %+v", only.Bar.Segments)
	}

	u, _ = url.Parse(seg(only, "audit.security").Href)
	sec := decode[view.AdminAuditPage](t, h.do(get(u.RequestURI(), sess)))
	var whats []string
	for _, row := range sec.Rows {
		whats = append(whats, string(row.What))
	}
	line := strings.Join(whats, ",")
	if !strings.Contains(line, "login.ok") || !strings.Contains(line, "login.fail") || !strings.Contains(line, "setup.first_admin") {
		t.Errorf("Security = %q", line)
	}

	// The file: the same filter, oldest first, no zone of the reader's.
	export := actionByID(only.Bar.Actions, "audit.export")
	if export == nil {
		t.Fatalf("no Export CSV: %+v", only.Bar.Actions)
	}
	u, _ = url.Parse(export.Href)
	rec := h.do(get(u.RequestURI(), sess))
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/csv") ||
		!strings.Contains(rec.Header().Get("Content-Disposition"), "carrel-audit.csv") {
		t.Fatalf("export: %d %v", rec.Code, rec.Header())
	}
	body := rec.Body.String()
	lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(body, "\r\n", "\n")), "\n")
	if lines[0] != "when,who,what,whom,detail" {
		t.Errorf("header = %q", lines[0])
	}
	for _, l := range lines[1:] {
		if !strings.Contains(l, ".fail") {
			t.Errorf("the file does not keep the filter: %q", l)
		}
	}
	if !strings.Contains(body, "T") || !strings.Contains(body, "Z") {
		t.Errorf("times are not RFC 3339:\n%s", body)
	}
}
