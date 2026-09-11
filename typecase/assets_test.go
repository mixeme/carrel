package typecase_test

import (
	"testing"

	"carrel.dev/typecase"
)

func TestAssets(t *testing.T) {
	css, err := typecase.CSS()
	if err != nil {
		t.Fatal(err)
	}
	if len(css) == 0 {
		t.Fatal("empty css")
	}
	js, err := typecase.JS()
	if err != nil || len(js) == 0 {
		t.Fatalf("js: %v", err)
	}
	theme, err := typecase.ThemeJS()
	if err != nil || len(theme) == 0 {
		t.Fatalf("theme.js: %v", err)
	}
	sprite, err := typecase.Sprite()
	if err != nil || len(sprite) == 0 {
		t.Fatalf("sprite: %v", err)
	}
}
