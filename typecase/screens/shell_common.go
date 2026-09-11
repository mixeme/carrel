package screens

import (
	"strings"

	"carrel.dev/typecase/widget"
)

func shellTop() widget.Top {
	return widget.Top{
		Brand: widget.Brand{SVG: carrelBrandMarkup, Href: "/"},
		Search: widget.Search{
			Action:      "/search",
			Placeholder: "Search everything…",
		},
		Create: widget.Menu{
			Label: "New note",
			Kind:  widget.ButtonPrimary,
			Icon:  "tc-i-quill",
			Items: []widget.MenuItem{
				{Label: "New note", Href: "/notes/new"},
				{Separator: true},
				{Label: "Quick note", Href: "/notes/quick"},
			},
		},
		Refresh: widget.Action{Label: "2 min ago", Icon: "tc-i-refresh"},
		User: widget.UserMenu{
			Name:  "mix",
			Items: []widget.MenuItem{{Label: "Sign out", Href: "/logout"}},
		},
	}
}

func shellNav(current string) widget.Nav {
	n312, n14, n3, n86, n3dup := 312, 14, 3, 86, 3
	return widget.Nav{
		Items: []widget.NavItem{
			{Label: "Contacts", Icon: "tc-i-con", Href: "/contacts", Count: intPtr(n312)},
			{Label: "Calendar", Icon: "tc-i-cal", Href: "/calendar", Count: intPtr(n14)},
			{Label: "Tasks", Icon: "tc-i-task", Href: "/tasks", Count: intPtr(n3)},
			{Label: "Notes", Icon: "tc-i-note", Href: "/notes", Count: intPtr(n86)},
			{Label: "Files", Icon: "tc-i-file", Href: "/files"},
			{Label: "Duplicates", Icon: "tc-i-dup", Href: "/duplicates", Count: intPtr(n3dup)},
		},
		Current: current,
	}
}

func shellFrame(nav widget.Nav, rail widget.Rail, main widget.Main, side widget.Side, tail string) string {
	html := widget.Shell{
		Top:  shellTop(),
		Nav:  nav,
		Rail: rail,
		Main: main,
		Side: side,
	}.Render()
	if tail == "" {
		return html
	}
	i := strings.LastIndex(html, "</div>")
	if i < 0 {
		return html + tail
	}
	return html[:i] + tail + html[i:]
}
