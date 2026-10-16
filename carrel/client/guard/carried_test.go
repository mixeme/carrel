package guard_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"carrel.dev/carrel/client/screens/eventedit"
	"carrel.dev/carrel/client/screens/taskedit"
	"carrel.dev/carrel/server/view"
)

// A task's Repeats is drawn as an event's, after the attachments (note 71,
// task 173; s-tasks-3): the frequency chosen, the days ticked as one row,
// the rule the record has kept with the form.
func TestATaskFormDrawsItsRule(t *testing.T) {
	p := &view.TaskEditPage{Form: view.Form{Action: "/app/tasks/a/c/t1/edit", Method: "POST", Sets: []view.FieldSet{
		{Rows: []view.FormRow{{Name: "summary", Label: "Summary", Kind: view.InputText, Value: "Water"}}},
		{Legend: "Repeats", Rows: []view.FormRow{
			{Name: "repeat.freq", Label: "Frequency", Kind: view.InputSelect, Value: "WEEKLY",
				Options: []view.Option{{Value: ""}, {Value: "WEEKLY", Selected: true}}},
			{Name: "repeat.interval", Label: "Every", Kind: view.InputNumber, Value: "1"},
			{Name: "repeat.on", Label: "On", Kind: view.InputCheck,
				Options: []view.Option{{Value: "MO", Selected: true}, {Value: "TU"}, {Value: "TH", Selected: true}}},
			{Name: "repeat.count", Label: "Count", Kind: view.InputNumber, Value: "3"},
			{Name: "repeat.base", Kind: view.InputHidden, Value: "FREQ=WEEKLY;BYDAY=MO,TH;COUNT=3"},
		}},
	}}}
	html := taskedit.Screen(p).Render()
	for _, want := range []string{
		">Repeats<", `value="WEEKLY" selected`, `name="repeat.interval" value="1"`,
		`name="repeat.on" value="MO" checked`, `name="repeat.on" value="TH" checked`, `name="repeat.count" value="3"`,
		`name="repeat.base" value="FREQ=WEEKLY;BYDAY=MO,TH;COUNT=3"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("the form does not draw %s", want)
		}
	}
	if strings.Contains(html, `name="repeat.on" value="TU" checked`) {
		t.Error("the form ticks a day not ticked")
	}
	if strings.Index(html, ">Repeats<") < strings.Index(html, ">Attachments<") {
		t.Error("Repeats stands before the attachments")
	}
}

// The Calendar an event lies in and the List of a task are drawn in their
// edit (Р19; note 71, task 175: s-agenda-3, s-tasks-3): a save into another
// moves the record.
func TestAnEventChoosesItsCalendar(t *testing.T) {
	for _, c := range []struct {
		fixture string
		drawn   bool
		draw    func([]byte) (string, error)
	}{
		{"app-event-edit/normal.json", true, func(raw []byte) (string, error) {
			var p view.EventEditPage
			err := json.Unmarshal(raw, &p)
			return eventedit.Screen(&p).Render(), err
		}},
		{"app-task-edit/normal.json", true, func(raw []byte) (string, error) {
			var p view.TaskEditPage
			err := json.Unmarshal(raw, &p)
			return taskedit.Screen(&p).Render(), err
		}},
	} {
		raw, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "spec", "fixtures", filepath.FromSlash(c.fixture)))
		if err != nil {
			t.Fatal(err)
		}
		html, err := c.draw(raw)
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(html, `name="notebook"`); got != c.drawn {
			t.Errorf("%s: the collection drawn %v, want %v", c.fixture, got, c.drawn)
		}
		// No Priority of an event: content of the kind (note 69 of
		// Typecase, task 177).
		if c.fixture == "app-event-edit/normal.json" && strings.Contains(html, `name="priority"`) {
			t.Errorf("%s: a Priority drawn", c.fixture)
		}
	}
}

// A task's Time zone (Р6) is drawn in When, as an event's (note 71, task
// 180; s-tasks-14): a choice with its menu, the zone chosen.
func TestATaskFormDrawsItsZone(t *testing.T) {
	p := &view.TaskEditPage{Form: view.Form{Action: "/app/tasks/a/c/t1/edit", Method: "POST", Sets: []view.FieldSet{
		{Rows: []view.FormRow{{Name: "summary", Label: "Summary", Kind: view.InputText, Value: "Call"}}},
		{Legend: "When", Rows: []view.FormRow{
			{Name: "allday", Label: "All day", Kind: view.InputCheck},
			{Name: "due.date", Label: "Due date", Kind: view.InputDate, Value: "2026-08-15"},
			{Name: "due.time", Label: "Due time", Kind: view.InputTime, Value: "12:00"},
			{Name: "tz", Label: "Time zone", Kind: view.InputSelect, Value: "Asia/Yekaterinburg", Placeholder: "Find a zone",
				Options: []view.Option{{Value: "Asia/Yekaterinburg", Selected: true}, {Value: "Europe/London"}}},
		}},
	}}}
	html := taskedit.Screen(p).Render()
	for _, want := range []string{">Time zone<", `name="tz" value="Asia/Yekaterinburg" checked`, `name="tz" value="Europe/London"`, "Find a zone"} {
		if !strings.Contains(html, want) {
			t.Errorf("the form does not draw %s", want)
		}
	}
}
