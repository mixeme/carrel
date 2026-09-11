package screens

import "carrel.dev/typecase/widget"

func shellTop() widget.Top {
	return widget.Top{
		Brand: widget.Brand{Mark: "◆", Word: "Carrel", Href: "/"},
		Search: widget.Search{
			Action: "/search",
		},
		Create: widget.Menu{
			Label: "New note",
			Icon:  "✎",
			Items: []widget.MenuItem{
				{Label: "New note", Href: "/notes/new"},
				{Separator: true},
				{Label: "Quick note", Href: "/notes/quick"},
			},
		},
		User: widget.UserMenu{
			Name:  "mix",
			Note:  "Signed in",
			Items: []widget.MenuItem{{Label: "Sign out", Href: "/logout"}},
		},
	}
}

func shellNav(current string) string {
	n312, n14, n3, n86, n3dup := 312, 14, 3, 86, 3
	return widget.Nav{
		Items: []widget.NavItem{
			{Label: "Contacts", Icon: "👤", Href: "/contacts", Count: intPtr(n312)},
			{Label: "Calendar", Icon: "📅", Href: "/calendar", Count: intPtr(n14)},
			{Label: "Tasks", Icon: "☑", Href: "/tasks", Count: intPtr(n3)},
			{Label: "Notes", Icon: "📝", Href: "/notes", Count: intPtr(n86)},
			{Label: "Files", Icon: "📁", Href: "/files"},
			{Label: "Duplicates", Icon: "⧉", Href: "/duplicates", Count: intPtr(n3dup)},
		},
		Current: current,
	}.Render()
}

func shellWrap(rail, main, side, extra string) string {
	return `<div class="tc-shell" data-tc-shell data-tc-root>` + shellTop().Render() +
		`<div class="tc-shell__body"><div class="tc-shell__rail">` + rail +
		`</div><div class="tc-shell__main">` + main +
		`</div><div class="tc-shell__side">` + side +
		`</div></div>` + extra + `</div>`
}
