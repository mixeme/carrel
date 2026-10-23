package guard_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"carrel.dev/carrel/client/guard"
	"carrel.dev/carrel/client/screens"
)

// The answers of the client that are not pages (screens.Answers: the search
// line's suggestions) hold what a screen holds, on every fixture of their
// view model: the frame check.py gives the fixture is one the answer names;
// the part printed alone has the make-up of the same part of that frame —
// the element of the frame whose classes are the answer's —, the classes of
// Typecase's CSS, and every control its meaning.
func TestAnswersMatchTheirFrames(t *testing.T) {
	l := load(t)
	n := 0
	for _, a := range screens.Answers {
		as := screens.Screen{Model: a.Model}
		for _, path := range l.fixtures(as) {
			at := fmt.Sprintf("%T on %s", a.Model(), key(path))
			frame := l.frameOf(path)
			if frame == "" || !slices.Contains(a.Frames, frame) {
				t.Errorf("%s: frame %q is not one the answer names (%v)", at, frame, a.Frames)
				continue
			}
			model, err := decode(as, path)
			if err != nil {
				t.Fatal(err)
			}
			n++
			part := a.Draw(model)
			printed := part.Render()
			screen := parseHTML(printed)
			if len(screen) == 0 {
				t.Errorf("%s: the answer prints nothing", at)
				continue
			}
			class := strings.Join(classes(screen[0]), " ")
			drawn := findAll(parseHTML(frameBlock(l.mock, frame)), func(n *html.Node) bool {
				return strings.Join(classes(n), " ") == class
			})
			if len(drawn) == 0 {
				t.Errorf("%s: %s draws no %q", at, frame, class)
				continue
			}
			for _, f := range partFindings(l, frame, at, drawn[:1], screen) {
				t.Error(f)
			}
			for _, f := range classFindings(l, at, printed) {
				t.Error(f)
			}
			for _, f := range guard.Meaning(part) {
				t.Errorf("%s: %s", at, f)
			}
		}
	}
	if n == 0 {
		t.Fatal("no fixture of an answer")
	}
}
