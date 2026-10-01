package web_test

import (
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// The form of a contact as the frames draw it (the client's scan of the
// contacts section, 30.09.2026, group Д): the head with the person's name
// and the book, the photograph's buttons and line (s-contacts-3) or Add
// (s-contacts-4), hints only where the frame draws them, and the empty
// fields folded under «Show N empty fields».

var (
	frameEditHead  = regexp.MustCompile(`(?s)<div class="tc-main">\s*<div class="tc-head">\s*<div class="tc-head__titles">\s*<div class="tc-head__title">([^<]*)</div>\s*<div class="tc-head__sub">([^<]*)</div>`)
	framePhotoSide = regexp.MustCompile(`(?s)<div class="tc-stack[^"]*tc-field__photo-side">(.*?)<span class="tc-field__hint[^"]*">([^<]*)</span>`)
	frameEditField = regexp.MustCompile(`(?s)<div class="tc-field[^"]*">\s*<label>([^<]*)</label>(.*?)</div>`)
	frameShowEmpty = regexp.MustCompile(`<div class="tc-field">\s*<button class="tc-btn" type="button">(Show \d+ empty fields?)</button>\s*</div>`)
)

// framePhotoButtons are the photograph's buttons of a form frame, «!» after
// a destructive one, and the line under them.
func framePhotoButtons(t *testing.T, id string) (buttons []string, line string) {
	t.Helper()
	m := framePhotoSide.FindStringSubmatch(mockupFrame(t, id))
	if m == nil {
		t.Fatalf("%s draws no photograph: read the frame again", id)
	}
	for _, b := range frameHeadButton.FindAllStringSubmatch(m[1], -1) {
		one := frameText(b[2])
		if strings.Contains(b[1], "is-danger") {
			one += "!"
		}
		buttons = append(buttons, one)
	}
	return buttons, frameText(m[2])
}

func photoButtons(b view.PhotoBlock) []string {
	var out []string
	for _, a := range b.Actions {
		one := string(a.Label)
		if a.Danger {
			one += "!"
		}
		out = append(out, one)
	}
	return out
}

func TestContactFormIsTheFrames(t *testing.T) {
	app := newMockupApp(t)
	card := app.listPath(t, "contacts", "kontakty") + "/4f2b-9c31-a0e7"

	// Without a photograph: Add and the formats (s-contacts-4).
	page := decode[view.ContactEditPage](t, app.h.do(get(card+"/edit", app.sess)))
	want, line := framePhotoButtons(t, "s-contacts-4")
	if got := photoButtons(page.Photo); strings.Join(got, ",") != strings.Join(want, ",") || string(page.Photo.Note) != line {
		t.Errorf("no photograph: %q %q, the frame %q %q", got, page.Photo.Note, want, line)
	}

	mockupAnnaPhoto(t, app)
	page = decode[view.ContactEditPage](t, app.h.do(get(card+"/edit", app.sess)))
	frame, _, _ := strings.Cut(mockupFrame(t, "s-contacts-3"), "<figcaption")

	head := frameEditHead.FindStringSubmatch(frame)
	if head == nil {
		t.Fatal("s-contacts-3 draws no head: read the frame again")
	}
	if string(page.Name) != frameText(head[1]) || string(page.Subtitle) != frameText(head[2]) {
		t.Errorf("head = %q %q, the frame %q %q", page.Name, page.Subtitle, head[1], head[2])
	}

	// With one: Replace, Crop, Remove and the line (s-contacts-3).
	want, line = framePhotoButtons(t, "s-contacts-3")
	if got := photoButtons(page.Photo); strings.Join(got, ",") != strings.Join(want, ",") || string(page.Photo.Note) != line {
		t.Errorf("photograph: %q %q, the frame %q %q", got, page.Photo.Note, want, line)
	}

	// A hint where the frame draws one, and nowhere else.
	hinted := map[string]bool{}
	for _, m := range frameEditField.FindAllStringSubmatch(frame, -1) {
		hinted[frameText(m[1])] = strings.Contains(m[2], "tc-field__hint")
	}
	for _, s := range page.Form.Sets {
		for _, r := range s.Rows {
			if h, drawn := hinted[string(r.Label)]; drawn && h != (r.Hint != "") {
				t.Errorf("%q: hint %q, the frame draws one: %v", r.Label, r.Hint, h)
			}
		}
	}

	// The empty fields, folded under their button, last.
	m := frameShowEmpty.FindStringSubmatch(frame)
	if m == nil {
		t.Fatal("s-contacts-3 draws no «Show N empty fields»: read the frame again")
	}
	last := page.Form.Sets[len(page.Form.Sets)-1]
	shape := regexp.MustCompile("^" + frameDigits.ReplaceAllString(strings.TrimSuffix(m[1], "s"), `\d+`) + "s?$")
	if !last.Folded || !shape.MatchString(string(last.Legend)) {
		t.Errorf("the last set = %q folded %v, the frame %q", last.Legend, last.Folded, m[1])
	}
	for _, s := range page.Form.Sets[:len(page.Form.Sets)-1] {
		if s.Folded {
			t.Errorf("%q is folded", s.Legend)
		}
	}
}

var (
	frameFset    = regexp.MustCompile(`(?s)<div class="tc-fset">\s*<div class="tc-rubric">([^<]*)</div>(.*?)\n\s*</div>\n`)
	frameFsetRow = regexp.MustCompile(`(?s)<div class="tc-field[^"]*"><label>([^<]*)</label><(?:button|input) class="tc-in[^"]*"[^>]*?(?:placeholder="([^"]*)")?[^>]*>([^<]*)|<button class="tc-btn[^"]*" type="button">(.*?)</button>`)
)

// The full form of a new contact (s-contacts-27, /new): the same form as an
// edit, empty — the head «New contact» over the book, and under Phone
// numbers one empty row, «mobile», with its Remove, before Add phone.
func TestNewContactFormIsTheFrame(t *testing.T) {
	app := newMockupApp(t)
	page := decode[view.ContactEditPage](t, app.h.do(get(app.listPath(t, "contacts", "kontakty")+"/new", app.sess)))
	frame, _, _ := strings.Cut(mockupFrame(t, "s-contacts-27"), "<figcaption")

	head := frameEditHead.FindStringSubmatch(frame)
	if head == nil {
		t.Fatal("s-contacts-27 draws no head: read the frame again")
	}
	if string(page.Name) != frameText(head[1]) || string(page.Subtitle) != frameText(head[2]) {
		t.Errorf("head = %q %q, the frame %q %q", page.Name, page.Subtitle, head[1], head[2])
	}
	// The photograph's Add the frame draws is not here yet: the card does
	// not exist before Save, and the photo steps are a card's
	// (plan/server.md, «Открыто»).

	fset := frameFset.FindStringSubmatch(frame)
	if fset == nil {
		t.Fatal("s-contacts-27 draws no set: read the frame again")
	}
	var drawn []string
	for _, m := range frameFsetRow.FindAllStringSubmatch(fset[2], -1) {
		if m[4] != "" {
			drawn = append(drawn, "["+frameText(m[4])+"]")
			continue
		}
		drawn = append(drawn, frameText(m[1])+"="+frameText(m[3]))
	}
	var got []string
	for _, s := range page.Form.Sets {
		if string(s.Legend) != frameText(fset[1]) {
			continue
		}
		for _, r := range s.Rows {
			switch r.Kind {
			case view.InputAdd, view.InputRemove:
				got = append(got, "["+string(r.Label)+"]")
			case view.InputSelect:
				shown := ""
				for _, o := range r.Options {
					if o.Selected {
						shown = string(o.Label)
					}
				}
				got = append(got, string(r.Label)+"="+shown)
			default:
				got = append(got, string(r.Label)+"="+string(r.Value))
			}
		}
	}
	if strings.Join(got, " ") != strings.Join(drawn, " ") {
		t.Errorf("%s = %q, the frame %q", fset[1], got, drawn)
	}
}
