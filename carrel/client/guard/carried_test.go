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

// A task's Repeats has no frame yet (Typecase's task 173), and the screen
// draws none; what it holds goes with the form as it came, so a save from
// the client keeps the task's rule — a rubric left out would read as «Does
// not repeat» on the server and take the rule away.
func TestATaskFormCarriesItsRule(t *testing.T) {
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
		`name="repeat.freq" value="WEEKLY"`, `name="repeat.interval" value="1"`,
		`name="repeat.on" value="MO"`, `name="repeat.on" value="TH"`, `name="repeat.count" value="3"`,
		`name="repeat.base" value="FREQ=WEEKLY;BYDAY=MO,TH;COUNT=3"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("the form does not carry %s", want)
		}
	}
	if strings.Contains(html, `name="repeat.on" value="TU"`) || strings.Contains(html, ">Repeats<") {
		t.Error("the form carries a day not ticked, or draws the rubric the frames do not")
	}
}

// The Calendar an event lies in is drawn in its edit (Р19), where the frame
// lets a field stand beside its own: a save into another moves the event.
// The task's List waits for its frame (Typecase's task 175) and is not.
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
		{"app-task-edit/normal.json", false, func(raw []byte) (string, error) {
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
		if c.drawn && strings.Contains(html, `name="priority"`) {
			t.Errorf("%s: a Priority drawn", c.fixture)
		}
	}
}

// A task's Time zone (Р6) has no frame yet (Typecase's task 180), and When
// takes no select: the screen draws none, and the zone goes with the form
// as it came — the times are shown and read in it, and a form without it
// would rewrite them in the instance zone.
func TestATaskFormCarriesItsZone(t *testing.T) {
	p := &view.TaskEditPage{Form: view.Form{Action: "/app/tasks/a/c/t1/edit", Method: "POST", Sets: []view.FieldSet{
		{Rows: []view.FormRow{{Name: "summary", Label: "Summary", Kind: view.InputText, Value: "Call"}}},
		{Legend: "When", Rows: []view.FormRow{
			{Name: "allday", Label: "All day", Kind: view.InputCheck},
			{Name: "due.date", Label: "Due date", Kind: view.InputDate, Value: "2026-08-15"},
			{Name: "due.time", Label: "Due time", Kind: view.InputTime, Value: "12:00"},
			{Name: "tz", Label: "Time zone", Kind: view.InputSelect, Value: "Asia/Yekaterinburg",
				Options: []view.Option{{Value: "Asia/Yekaterinburg", Selected: true}, {Value: "Europe/London"}}},
		}},
	}}}
	html := taskedit.Screen(p).Render()
	if !strings.Contains(html, `name="tz" value="Asia/Yekaterinburg"`) {
		t.Error("the form does not carry the zone")
	}
	if strings.Contains(html, "Time zone") || strings.Contains(html, "Europe/London") {
		t.Error("the form draws the zone the frames do not")
	}
}
