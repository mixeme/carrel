package screens

import _ "embed"

//go:embed screens.css
var screensCSS string

// CSS returns screen page styles (not part of the widget bundle).
func CSS() string {
	return screensCSS
}
