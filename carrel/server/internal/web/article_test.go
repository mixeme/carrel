package web

import "testing"

// Кл14: the move sheet of an address book says «an address book».
func TestArticle(t *testing.T) {
	for noun, want := range map[string]string{"address book": "an address book", "calendar": "a calendar", "task list": "a task list", "notebook": "a notebook"} {
		if got := article(noun); got != want {
			t.Errorf("%s: %q, want %q", noun, got, want)
		}
	}
}
