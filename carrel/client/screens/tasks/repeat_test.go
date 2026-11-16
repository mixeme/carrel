package tasks_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/client/screens/tasks"
	"carrel.dev/carrel/server/view"
)

// s-tasks-21 (Typecase's note 206, task 264): the row of a repeating task
// carries «repeats weekly» after its name, and its tick says in its hint
// what pressing does (TaskRow.Tick.Hint, server v0.2.412) — it does not take
// the task to the done ones. A plain tick has no hint.
func TestARepeatingTaskRowSaysWhatItsTickDoes(t *testing.T) {
	hint := "Repeating task: Done completes this occurrence and moves the task to the next one, 24 Aug."
	page := &view.TasksPage{Groups: []view.TaskGroup{{Rows: []view.TaskRow{
		{UID: "water", Summary: "Полить цветы", Due: "17 Aug", Href: "/t/water",
			Badges: []view.Badge{{Label: "repeats weekly", Role: view.BadgeNeutral}},
			Tick:   &view.Action{ID: "task.done", Href: "/t/water/done", Method: "POST", Enabled: true, Hint: view.Text(hint)}},
		{UID: "once", Summary: "Once", Href: "/t/once",
			Tick: &view.Action{ID: "task.done", Href: "/t/once/done", Method: "POST", Enabled: true}},
	}}}}
	html := tasks.Screen(page).Render()
	if !strings.Contains(html, `title="`+hint+`"`) {
		t.Errorf("the tick of the repeating task says nothing:\n%s", html)
	}
	if strings.Count(html, `class="tc-tick"`) < 2 || strings.Count(html, "title=\"Repeating task") != 1 {
		t.Errorf("want two ticks and one hint:\n%s", html)
	}
	if !strings.Contains(html, `<span class="tc-badge">repeats weekly</span>`) {
		t.Errorf("the row does not say it repeats:\n%s", html)
	}
}
