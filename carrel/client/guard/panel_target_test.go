package guard_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"carrel.dev/carrel/client/document"
	"carrel.dev/carrel/client/screens"
)

// The owner, 24.11.2026: going from note to note reloaded the page. A row
// that opens in the details panel (its Target, «Keep the details panel
// open» on) asks the live layer for the panel alone (C-7.2): its list names
// that zone. Without the kept panel a row leads to its page, and the list
// names nothing.
func TestRowsThatOpenInThePanelAskThePanel(t *testing.T) {
	l := load(t)
	checked := 0
	lists := map[string]bool{"ContactsListPage": true, "AgendaPage": true, "TasksPage": true, "NotesPage": true}
	for _, scr := range screens.All {
		if !lists[modelName(scr)] {
			continue
		}
		for _, path := range l.fixtures(scr) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var probe struct {
				Look struct {
					Panel bool `json:"panel"`
				} `json:"look"`
			}
			_ = json.Unmarshal(raw, &probe)
			if !strings.Contains(string(raw), `"target": "details"`) {
				continue
			}
			model, err := decode(scr, path)
			if err != nil {
				t.Fatal(err)
			}
			page := parseHTML(document.Of(scr, model, "").Root.Render())
			found := findAll(page, func(n *html.Node) bool { return hasClass(n, "tc-list") })
			if len(found) == 0 {
				continue
			}
			named := 0
			for _, n := range found {
				if attrOf(n, "data-tc-target") == "details" {
					named++
				}
			}
			at := label(scr) + " on " + key(path)
			switch {
			case probe.Look.Panel && named == 0:
				t.Errorf("%s: the panel is kept and no list asks it", at)
			case !probe.Look.Panel && named > 0:
				t.Errorf("%s: no kept panel, and %d lists ask it", at, named)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no fixture of a list with rows for the panel")
	}
}
