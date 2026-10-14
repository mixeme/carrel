package guard_test

import (
	"strings"
	"testing"

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
