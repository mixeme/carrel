package guard_test

import (
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/client/fixtures"
	"carrel.dev/carrel/client/screens"
)

func TestEditFilesWaitForSave(t *testing.T) {
	for _, dir := range []string{"app-contact-edit", "app-contact-new", "app-event-edit", "app-event-new", "app-task-edit"} {
		t.Run(dir, func(t *testing.T) {
			data, err := os.ReadFile(fixtureDir + "/" + dir + "/normal.json")
			if err != nil {
				t.Fatal(err)
			}
			model := fixtures.Models[dir]()
			if err := json.Unmarshal(data, model); err != nil {
				t.Fatal(err)
			}
			var html string
			for _, s := range screens.All {
				if reflect.TypeOf(s.Model()) == reflect.TypeOf(model) {
					html = s.Draw(model).Render()
					break
				}
			}
			fields := regexp.MustCompile(`<input[^>]*type="file"[^>]*>`).FindAllString(html, -1)
			if len(fields) != 1 {
				t.Fatalf("file inputs = %d", len(fields))
			}
			if strings.Contains(fields[0], ` form=`) || strings.Contains(fields[0], `data-tc-to=`) {
				t.Errorf("selection still submits separately: %s", fields[0])
			}
			if !strings.Contains(html, `enctype="multipart/form-data"`) {
				t.Error("Save form does not send files")
			}
			if strings.Contains(html, `>Upload<`) {
				t.Error("separate Upload remains")
			}
			if strings.Contains(dir, "contact") {
				if !strings.Contains(fields[0], `data-tc-preview="photo"`) || !strings.Contains(html, `id="photo"`) {
					t.Error("photo has no local preview")
				}
			} else if !strings.Contains(fields[0], `data-tc-list=`) {
				t.Error("chosen files have no pending rows")
			}
		})
	}
}
