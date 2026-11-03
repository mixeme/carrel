package web

import (
	"errors"
	"testing"

	"carrel.dev/carrel/server/internal/dav"
)

// The words of the subtasks a task marked done could not close (the
// owner's words of 03.11.2026): the answers differ — none in the line —, and
// the list could not be read again — its answer, by code and text.
func TestSubtaskWords(t *testing.T) {
	n := subtasksLeft(4, []subtaskRefusal{
		{"Write the letter", &dav.StatusError{Code: 403}},
		{"Tag the build", &dav.StatusError{Code: 507}},
	})
	if n.Title != "Task done. Mark done failed for 2 of 4 subtasks." || n.Body != "2 marked done, 2 still open." ||
		len(n.Details.Lines) != 2 || n.Details.Lines[1] != "Tag the build · 507 Insufficient Storage." {
		t.Errorf("subtasksLeft = %q / %q / %+v", n.Title, n.Body, n.Details)
	}
	for _, c := range []struct {
		err  error
		want string
	}{
		{&dav.StatusError{Code: 503}, "Task done. Subtasks not checked: 503 Service Unavailable."},
		{&dav.StatusError{Code: 403}, "Task done. Subtasks not checked: 403 Forbidden."},
		{errors.New("dial tcp: connection refused"), "Task done. Subtasks not checked: no response from the server."},
	} {
		n := subtasksUnread(c.err)
		if string(n.Title) != c.want || n.Body != "Carrel could not read the list again. Open subtasks stay open." {
			t.Errorf("subtasksUnread(%v) = %q / %q, want %q", c.err, n.Title, n.Body, c.want)
		}
	}
}
