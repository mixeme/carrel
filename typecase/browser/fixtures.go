package browser

import (
	"strings"

	"carrel.dev/typecase/widget"
)

// GoodFixtures returns passing L-9.2 page bodies keyed by check name.
func GoodFixtures() map[Name]Fixture {
	longLabel := strings.Repeat("Export everything now ", 8)
	menu := widget.Menu{
		Label: longLabel,
		Items: []widget.MenuItem{{Label: "One", Href: "/1"}, {Label: "Two", Href: "/2"}},
		ID:    "menu-long",
	}.Render()
	menuShort := widget.Menu{
		Label: "Actions",
		Items: []widget.MenuItem{{Label: "Edit", Href: "/edit"}},
		ID:    "menu-short",
	}.Render()

	var actions []widget.Action
	for i := 0; i < 6; i++ {
		actions = append(actions, widget.Action{Label: "Action", Enabled: true})
	}
	bar := widget.Bar{Left: actions, Right: []widget.Action{{Label: "Save", Enabled: true}}}.Render()

	fields := widget.Fields{Rows: []widget.FieldRow{
		{Label: "Path", Value: "/very/long/path/that/must/truncate/in/its/cell/without/overflow", Path: true},
	}}.Render()

	nav := widget.Nav{
		Layout: "bottom",
		Items:  []widget.NavItem{{Label: "Home", Icon: "⌂", Href: "/"}, {Label: "Settings", Icon: "⚙", Href: "/settings"}},
	}.Render()

	sheet := widget.Sheet{
		Title: "Sheet", Target: "Item", Body: "<p>Under</p>", Open: true, ID: "bottom-sheet",
		Actions: []widget.Action{{Label: "OK", Enabled: true}},
	}.Render()
	dialog := widget.Dialog{
		Title: "Confirm", Body: "On top", Open: true, ID: "top-dialog",
		Confirm: widget.Action{Label: "Yes", Enabled: true},
		Cancel:  widget.Action{Label: "No", Enabled: true},
	}.Render()

	sample := `<p class="tc-sample">Sample text</p>`
	sampleCSS := `.tc-sample { display: inline-block; width: 180px; padding: 8px; background: var(--tc-surface); color: var(--tc-ink); }`

	return map[Name]Fixture{
		MenuLabelArrow: {
			Body:  menu,
			Width: 220,
			Extra: `.demo { width: 200px; }`,
		},
		PopoverAnchor: {Body: menuShort, Width: 800, Height: 600},
		PopoverFlip: {
			Body:   menuShort,
			Width:  400,
			Height: 300,
			Extra:  `.demo { margin-top: 240px; }`,
		},
		Ellipsis: {
			Body:  fields,
			Width: 320,
			Extra: `.demo { width: 280px; }`,
		},
		BarOneRow: {
			Body:  bar,
			Width: 280,
			Extra: `.demo { width: 260px; }`,
		},
		TapTarget: {Body: nav, Width: 375, Height: 200},
		FocusReturn: {Body: menuShort},
		EscLayers:   {Body: sheet + dialog, Width: 800, Height: 600},
		Contrast:      {Body: sample, Extra: sampleCSS},
		ThemeGeometry: {Body: sample, Extra: sampleCSS},
	}
}
