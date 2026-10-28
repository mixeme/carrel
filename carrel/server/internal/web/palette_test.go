package web_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// mockupFrame is the markup of one frame of the mockup, from its opening tag
// to the next frame's.
func mockupFrame(t *testing.T, id string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "docs", "visual", "carrel-ui-mockups.html"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	mock := string(raw)
	at := strings.Index(mock, `data-frame="`+id+`"`)
	if at < 0 {
		t.Fatalf("no frame %s in the mockup", id)
	}
	end := strings.Index(mock[at+1:], `<div class="frame"`)
	if end < 0 {
		return columnless.Replace(mock[at:])
	}
	return columnless.Replace(mock[at : at+1+end])
}

// columnless takes out of a frame the classes that keep a settings page in
// its reading column of 580 px (Typecase's note 141, Р-82): they say how
// wide, not what — the frames are read here for their words — and a class
// more would part a hint or a message from the patterns that read it.
var columnless = strings.NewReplacer(
	` tc-field__hint--max580`, ``,
	` tc-msg--max580`, ``,
	` tc-field--w580`, ``,
)

// C-10, from-typecase note 14: the colours a collection sheet offers are the
// swatches the mockup draws in «Rename and colour» (s-profile-7), in their
// order — read from the frame, not from a list kept beside the palette.
func TestPaletteIsTheMockupsSwatches(t *testing.T) {
	var want []string
	swatch := regexp.MustCompile(`data-tc-colour="(#[0-9A-Fa-f]{6})" class="tc-field__swatch`)
	for _, m := range swatch.FindAllStringSubmatch(mockupFrame(t, "s-profile-7"), -1) {
		want = append(want, strings.ToLower(m[1]))
	}
	if len(want) == 0 {
		t.Fatal("s-profile-7 draws no swatches: the frame changed, read it again")
	}

	f := calendarFixture(t, nil)
	sheet := decode[view.CollectionNewSheet](t, f.h.do(get("/app/collections/new?kind=event&a="+f.id, f.sess)))
	var got []string
	for _, o := range rowNamed(t, sheet.Form, "colour").Options {
		got = append(got, o.Value)
	}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("palette = %v, want the swatches of s-profile-7 %v", got, want)
	}
}
