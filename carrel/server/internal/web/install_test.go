package web_test

import (
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// S-28.1: the check calls this Carrel at the declared address and gives
// eleven rows in their order; the address goes to settings.env; the run is
// journalled. On a bare http listener without a proxy: reachable, the
// stream unbuffered, the body let through, no certificate and no
// X-Forwarded-For — said as such.
func TestInstallationCheck(t *testing.T) {
	h := newHarness(t)
	h.admin("alice", "password123")
	alice := h.login("alice", "password123")
	live := httptest.NewServer(h.srv.Handler())
	defer live.Close()

	page := decode[view.AdminInstallPage](t, h.do(postForm("/admin/install", alice, url.Values{"url": {live.URL}})))
	if len(page.Rows) != 11 || page.PublicURL != view.Text(live.URL) {
		t.Fatalf("rows = %d, url %q", len(page.Rows), page.PublicURL)
	}
	state := map[string]view.CheckState{}
	var order []string
	for _, r := range page.Rows {
		state[string(r.Check)] = r.State
		order = append(order, string(r.Check))
		if r.State == view.CheckBad && r.Advice == "" {
			t.Errorf("%s: broken without advice", r.Check)
		}
	}
	want := "Reachable from outside|Certificate|Base address|Scheme behind the proxy|Client address|Event stream (SSE)|Upload size|Long request timeout|Cookie|Clock|Data folder"
	if strings.Join(order, "|") != want {
		t.Errorf("order = %v", order)
	}
	for check, st := range map[string]view.CheckState{
		"Reachable from outside": view.CheckOK, "Certificate": view.CheckBad, "Base address": view.CheckOK,
		"Scheme behind the proxy": view.CheckOK, "Client address": view.CheckBad, "Event stream (SSE)": view.CheckOK,
		"Upload size": view.CheckOK, "Long request timeout": view.CheckUnknown, "Cookie": view.CheckOK,
		"Clock": view.CheckOK, "Data folder": view.CheckOK,
	} {
		if state[check] != st {
			t.Errorf("%s = %s, want %s", check, state[check], st)
		}
	}
	data, _ := os.ReadFile(filepath.Join(h.cfg.DataDir, "settings.env"))
	if !strings.Contains(string(data), "CARREL_PUBLIC_URL="+live.URL) {
		t.Errorf("settings.env = %q", data)
	}
	snap := h.st.Snapshot()
	if e := snap.Audit[len(snap.Audit)-1]; e.What != "install.check" {
		t.Errorf("journal = %+v", e)
	}
}
