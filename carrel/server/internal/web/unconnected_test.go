package web_test

import (
	"encoding/json"
	"testing"
)

// An instance with nothing connected (states-1; the owner, from a tablet,
// 21.11.2026): every list says that no server is connected, explains in one
// sentence where Carrel's records come from, and offers Connect account —
// not «No books are ticked» with «Tick every book», which ticks nothing. The
// head is the section's name with nothing to make or carry, the rail says
// «Nothing connected yet» under «Sources». Every kind alike.
func TestAnInstanceWithNothingConnectedSaysSo(t *testing.T) {
	h := newHarness(t)
	h.admin("mix", "password123")
	mix := h.login("mix", "password123")
	for path, title := range map[string]string{
		"/app/contacts": "Contacts", "/app/calendar?from=2026-08-10": "Calendar",
		"/app/tasks": "Tasks", "/app/notes": "Notes", "/app/files": "Files",
	} {
		rec := h.do(get(path, mix))
		var p struct {
			Title    string `json:"title"`
			Subtitle any    `json:"subtitle"`
			Empty    *struct {
				Title   string `json:"title"`
				Body    string `json:"body"`
				Actions []struct {
					ID    string `json:"id"`
					Label string `json:"label"`
					Href  string `json:"href"`
				} `json:"actions"`
			} `json:"empty"`
			Bar struct {
				Actions []any `json:"actions"`
			} `json:"bar"`
			Rail struct {
				Sections []struct {
					Account string `json:"account"`
					Note    string `json:"note"`
					Rows    []any  `json:"rows"`
				} `json:"sections"`
			} `json:"rail"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
			t.Fatalf("%s: %d %v", path, rec.Code, err)
		}
		sub, _ := json.Marshal(p.Subtitle)
		if p.Title != title || (string(sub) != `"No server connected yet"` && string(sub) != `[{"text":"No server connected yet"}]`) {
			t.Errorf("%s: head %q / %s", path, p.Title, sub)
		}
		if p.Empty == nil || p.Empty.Title != "" ||
			p.Empty.Body != "Carrel stores nothing of its own: contacts, calendars, tasks and notes come from your DAV servers. Connect the first one — you will need an address, a user name and a password." ||
			len(p.Empty.Actions) != 1 || p.Empty.Actions[0].Label != "Connect account" {
			t.Errorf("%s: empty %+v", path, p.Empty)
		}
		if len(p.Bar.Actions) != 0 {
			t.Errorf("%s: the head offers %d actions", path, len(p.Bar.Actions))
		}
		if len(p.Rail.Sections) != 1 || p.Rail.Sections[0].Account != "Sources" || p.Rail.Sections[0].Note != "Nothing connected yet" {
			t.Errorf("%s: rail sections %+v", path, p.Rail.Sections)
		}
	}
}
