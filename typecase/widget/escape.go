package widget

import (
	"html"
	"strings"
)

func esc(s string) string {
	return html.EscapeString(s)
}

func attr(name, value string) string {
	if value == "" {
		return ""
	}
	return " " + name + `="` + esc(value) + `"`
}

func classes(parts ...string) string {
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " ")
}

func classAttr(parts ...string) string {
	c := classes(parts...)
	if c == "" {
		return ""
	}
	return ` class="` + c + `"`
}

func boolAttr(name string, on bool) string {
	if on {
		return " " + name
	}
	return ""
}

func ariaBool(name string, on bool) string {
	if on {
		return ` ` + name + `="true"`
	}
	return ` ` + name + `="false"`
}
