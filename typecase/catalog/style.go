package catalog

import _ "embed"

//go:embed catalog.css
var catalogCSS string

// CSS returns catalog page styles (not part of the widget bundle).
func CSS() string {
	return catalogCSS
}
