package screens

import (
	_ "embed"

	"carrel.dev/typecase"
)

//go:embed carrel-logo.svg
var carrelLogoSVG []byte

const carrelBrandMarkup = `<svg class="tc-brand__logo" viewBox="4.79 -31.36 291.08 116.63" width="56" height="22" role="img" aria-label="Carrel"><use href="#logo-word" class="tc-brand__wordpath"/><use href="#logo-shade" class="tc-brand__shadepath"/></svg>`

// InlineSVG returns the library sprite and Carrel logo defs for demo pages.
func InlineSVG() string {
	sprite, err := typecase.Sprite()
	if err != nil {
		return string(carrelLogoSVG)
	}
	return string(sprite) + string(carrelLogoSVG)
}
