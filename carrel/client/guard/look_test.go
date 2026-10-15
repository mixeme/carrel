package guard_test

import (
	"path/filepath"
	"strings"
	"testing"

	"carrel.dev/carrel/client/screens"
	"carrel.dev/carrel/server/view"
)

// Appearance (appearance-1, the server's Shell.Look): a browser that does
// not show the source column gets the combined lists without it, as the
// list of one collection is drawn; the default shows it.
func TestTheSourceColumnFollowsTheLook(t *testing.T) {
	for _, c := range []struct{ fixture, model string }{
		{"app-tasks-all/normal.json", "TasksPage"},
		{"app-notes-all/normal.json", "NotesPage"},
		{"app-agenda-all/normal.json", "AgendaPage"},
	} {
		path := filepath.Join("..", "..", "..", "docs", "spec", "fixtures", filepath.FromSlash(c.fixture))
		var scr screens.Screen
		for _, s := range screens.All {
			if modelName(s) == c.model {
				scr = s
			}
		}
		draw := func(source bool) string {
			model, err := decode(scr, path)
			if err != nil {
				t.Fatal(err)
			}
			switch m := model.(type) {
			case *view.TasksPage:
				m.Look.Source = source
			case *view.NotesPage:
				m.Look.Source = source
			case *view.AgendaPage:
				m.Look.Source = source
			}
			return scr.Draw(model).Render()
		}
		if !strings.Contains(draw(true), "tc-row__source") {
			t.Errorf("%s: the default draws no source column", c.fixture)
		}
		if strings.Contains(draw(false), "tc-row__source") {
			t.Errorf("%s: the source column is drawn though the look hides it", c.fixture)
		}
	}
}
