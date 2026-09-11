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
		actions = append(actions, widget.Action{Label: "Action"})
	}
	bar := widget.Bar{Left: widget.ActionsAsNodes(actions), Right: widget.ActionsAsNodes([]widget.Action{{Label: "Save"}})}.Render()

	fields := widget.Fields{Rows: []widget.FieldRow{
		{Label: "Path", Value: "/very/long/path/that/must/truncate/in/its/cell/without/overflow", Path: true},
	}}.Render()

	nav := widget.Nav{
		Layout: widget.NavLayoutBottom,
		Items:  []widget.NavItem{{Label: "Home", Icon: "⌂", Href: "/"}, {Label: "Settings", Icon: "⚙", Href: "/settings"}},
	}.Render()

	sheet := widget.Sheet{
		Title: "Sheet", Target: "Item", Body: widget.Paragraph("Under"), Open: true, ID: "bottom-sheet",
		Actions: []widget.Action{{Label: "OK"}},
	}.Render()
	dialog := widget.Dialog{
		Title: "Confirm", Text: "On top", Open: true, ID: "top-dialog",
		Confirm: widget.Action{Label: "Yes"},
		Cancel:  widget.Action{Label: "No"},
	}.Render()

	// Каждый мелкий контрол на одной странице: у всех зона нажатия обязана
	// добирать до 44 px в узкой раскладке, не увеличивая нарисованный размер.
	taps := `<div class="tap-grid">` +
		widget.Tick{Done: true, Label: "Mark done"}.Render() +
		widget.TickRow{Label: "Inbox", Tick: widget.TickOn}.Render() +
		widget.Check{Name: "agree", Label: "I agree", Checked: true}.Render() +
		widget.Close{For: "tap-demo", Label: "Close"}.Render() +
		nav + `</div>`
	// Контролы разведены, чтобы зона одного не перекрывалась соседом: проверка
	// спрашивает про каждый в отдельности, а не про их соседство.
	tapsCSS := `.tap-grid { display: flex; flex-direction: column; gap: 40px; align-items: stretch; }`

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
		TapTarget:     {Body: taps, Width: 375, Height: 600, Extra: tapsCSS},
		FocusReturn:   {Body: menuShort},
		EscLayers:     {Body: sheet + dialog, Width: 800, Height: 600},
		Contrast:      {Body: sample, Extra: sampleCSS},
		ThemeGeometry: {Body: sample, Extra: sampleCSS},
	}
}
