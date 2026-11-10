package web_test

import (
	"regexp"
	"testing"

	"carrel.dev/carrel/server/view"
)

// An attachment's Remove is a word without an icon in every frame that
// draws one (s-agenda-3, -15, -32; s-tasks-3; s-notes-4): the server's
// action carries no icon for the screen to draw.
func TestAttachmentRemoveHasNoIcon(t *testing.T) {
	removes := regexp.MustCompile(`<span class="tc-badge">in the event</span></span>(<button[^>]*>.*?</button>)`)
	frame := removes.FindAllStringSubmatch(mockupFrame(t, "s-agenda-3"), -1)
	if len(frame) != 1 || frame[0][1] != `<button class="tc-btn is-sm is-danger" type="button">Remove</button>` {
		t.Fatalf("s-agenda-3 draws %q: read the frame again", frame)
	}
	app := newMockupApp(t)
	page := decode[view.EventEditPage](t, app.h.do(get(app.listPath(t, "calendar", "lichnyj")+"/8f1c-4a02-b7d3/edit", app.sess)))
	if len(page.Attach) == 0 {
		t.Fatal("the event has no attachment: read the set again")
	}
	for _, a := range page.Attach {
		for _, act := range a.Actions {
			if act.ID == "attach.remove" && act.Icon != "" {
				t.Errorf("%s: Remove carries the icon %q; the frame draws none", a.Name, act.Icon)
			}
		}
	}
}
