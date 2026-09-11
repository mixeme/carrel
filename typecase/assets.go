package typecase

import (
	"embed"
	"io/fs"

	"carrel.dev/typecase/css"
)

//go:embed js/*
var jsFS embed.FS

//go:embed fonts/*
var fontFS embed.FS

//go:embed icons/*
var iconFS embed.FS

// CSS returns the concatenated stylesheet built from css/ in declared order.
func CSS() (string, error) {
	return css.Bundle()
}

// JS returns the library behavior script (tc.js).
func JS() ([]byte, error) {
	return jsFS.ReadFile("js/tc.js")
}

// ThemeJS returns the theme boot script that must run before first paint (L-5.3).
func ThemeJS() ([]byte, error) {
	return jsFS.ReadFile("js/theme.js")
}

// Fonts returns the embedded font files.
func Fonts() fs.FS {
	return fontFS
}

// Icons returns the embedded icon sprite.
func Icons() fs.FS {
	return iconFS
}

// Sprite returns the neutral icon sprite SVG bytes.
func Sprite() ([]byte, error) {
	return iconFS.ReadFile("icons/sprite.svg")
}
