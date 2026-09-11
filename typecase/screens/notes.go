package screens

import "carrel.dev/typecase/widget"

// Notes assembles mockup s-notes figure 1: list, side panel, shell, and a
// closed quick-note sheet in the DOM.
func Notes() string {
	top := notesTop().Render()
	rail := notesNav() + notesRail().Render()
	main := notesMain()
	side := notesSide()
	quick := widget.Sheet{
		ID:     "quick-note",
		Title:  "New note",
		Target: "Записи · nextcloud",
		Body:   widget.Paragraph("Позвонить в МФЦ во вторник до 12:00 — уточнить, нужен ли оригинал доверенности."),
		Actions: []widget.Action{
			{Label: "Save note", Kind: widget.ButtonPrimary},
			{Label: "Open full screen"},
			{Label: "Cancel"},
		},
	}.Render()

	return `<div class="tc-shell" data-tc-shell data-tc-root>` + top +
		`<div class="tc-shell__body"><div class="tc-shell__rail">` + rail +
		`</div><div class="tc-shell__main">` + main +
		`</div><div class="tc-shell__side">` + side +
		`</div></div>` + quick + `</div>`
}

func notesTop() widget.Top {
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

func notesNav() string {
	n312, n14, n3, n86 := 312, 14, 3, 86
	return widget.Nav{
		Items: []widget.NavItem{
			{Label: "Contacts", Icon: "👤", Href: "/contacts", Count: intPtr(n312)},
			{Label: "Calendar", Icon: "📅", Href: "/calendar", Count: intPtr(n14)},
			{Label: "Tasks", Icon: "☑", Href: "/tasks", Count: intPtr(n3)},
			{Label: "Notes", Icon: "📝", Href: "/notes", Count: intPtr(n86)},
			{Label: "Files", Icon: "📁", Href: "/files"},
			{Label: "Duplicates", Icon: "⧉", Href: "/duplicates", Count: intPtr(3)},
		},
		Current: "/notes",
	}.Render()
}

func notesRail() widget.Rail {
	n86, n64, n22 := 86, 64, 22
	return widget.Rail{
		Sections: []widget.RailSection{
			{
				Rows: []widget.TickRow{{
					Label: "Combined notebook",
					Mark:  widget.Mark{Colour: "#4a6b52", Label: "Combined"},
					Count: intPtr(n86),
					Tick:  widget.TickPartial,
				}},
			},
			{
				Title: "nextcloud · home",
				Rows: []widget.TickRow{
					{Label: "Записи", Mark: widget.Mark{Colour: "#4A6B52", Label: "Записи"}, Count: intPtr(n64), Tick: widget.TickOff},
					{Label: "Дневник", Mark: widget.Mark{Colour: "#6B4E7A", Label: "Дневник"}, Count: intPtr(n22), Tick: widget.TickOff},
					{Label: "Архив", Mark: widget.Mark{Colour: "#A05A2C", Label: "Архив"}, State: widget.TickRowUnavailable},
				},
			},
		},
		Chips: []widget.Chip{
			{Label: "all", On: true},
			{Label: "встречи"},
			{Label: "идеи"},
			{Label: "книги"},
			{Label: "рецепты"},
		},
		Foot: widget.RailFoot{
			Actions: []widget.Action{
				{Label: "New notebook", Icon: "+"},
				{Label: "Connect account", Icon: "⎈"},
				{Label: "Subscribe to a feed", Icon: "🔗"},
			},
			Build: demoBuild,
		},
	}
}

func notesMain() string {
	n86, n9 := 86, 9
	return widget.Main{
		Label: "Notes",
		Body: []widget.Node{
			widget.Head{
				Mark:  widget.Mark{Colour: "#4a6b52", Label: "Combined notebook"},
				Title: "Combined notebook",
				Sub:   "2 of 3 notebooks · 86 notes · VJOURNAL",
				Actions: []widget.Action{
					{Label: "New note", Kind: widget.ButtonPrimary, Icon: "+"},
					{Label: "Import Markdown"},
					{Label: "Export"},
				},
			},
			widget.Bar{
				Left: []widget.Node{
					widget.Input{Name: "filter", Kind: widget.InputText, Placeholder: "Filter by word"},
					widget.Seg{
						Name:    "sort",
						Chosen:  "newest",
						Options: []widget.SegOption{{Label: "Newest", Value: "newest"}, {Label: "Oldest", Value: "oldest"}, {Label: "Title", Value: "title"}},
					},
				},
				Right: []widget.Node{
					widget.Button{Label: "Columns", Icon: "▦", Size: widget.ButtonSizeBar},
					widget.Num{Value: intPtr(n86)},
				},
			},
			widget.List{
				Groups: []widget.ListGroup{{
					Rubric: widget.Rubric{Text: "August 2026", Count: intPtr(n9)},
					Rows: []widget.Row{
						{
							Variant:  widget.RowExcerpt,
							Selected: true,
							Mark:     widget.Mark{Colour: "#4A6B52", Label: "Записи"},
							Cells: widget.RowCells{
								Title:   "Заметка со встречи 10.08",
								Excerpt: "Договорились заморозить ветку в среду; письмо пользователям пишет Анна…",
								Tags:    []widget.Tag{{Label: "встречи", On: true, Href: "/tags/встречи"}},
							},
						},
						{
							Variant: widget.RowExcerpt,
							Mark:    widget.Mark{Colour: "#4A6B52", Label: "Записи"},
							Cells: widget.RowCells{
								Title:   "Что почитать про CRDT",
								Excerpt: "Кляйн, глава 5; статья Шапиро 2011; демка automerge…",
								Tags:    []widget.Tag{{Label: "книги", Href: "/tags/книги"}},
							},
						},
						{
							Variant: widget.RowExcerpt,
							Mark:    widget.Mark{Colour: "#6B4E7A", Label: "Дневник"},
							Cells: widget.RowCells{
								Title:   "Про дорогу до дачи",
								Excerpt: "Съезд после заправки, потом грунтовка два километра…",
							},
							Badges: []widget.Badge{{Label: "linked", Role: widget.BadgeLinked}},
						},
					},
				}},
			},
			notesDocPreview(),
		},
	}.Render()
}

func notesDocPreview() widget.Doc {
	return widget.Doc{
		Mode: widget.DocReading,
		Bar: widget.DocBar{
			Left: []widget.Action{{Label: "Edit"}, {Label: "Export"}},
		},
		Body: widget.Prose{Blocks: []widget.ProseBlock{
			{Kind: widget.ProseH2, Text: "Заметка со встречи 10.08"},
			{Kind: widget.ProseP, Text: "Договорились заморозить ветку в среду; письмо пользователям пишет Анна."},
		}},
		Meta: widget.DocMeta{Sections: []widget.MetaSection{{
			Title: "Tags",
			Tags:  []widget.Tag{{Label: "встречи", On: true, Href: "/tags/встречи"}},
		}}},
	}
}

func notesSide() string {
	title := widget.Field{Name: "title", Label: "Title", Kind: widget.InputText, Value: "Заметка со встречи 10.08"}
	text := widget.Field{Name: "text", Label: "Text", Kind: widget.InputTextarea, Value: "Договорились заморозить ветку в среду.\n\nПисьмо пользователям пишет Анна…"}
	date := widget.Field{Name: "date", Label: "Date", Kind: widget.InputDate, Value: "2026-08-10"}
	timeF := widget.Field{Name: "time", Label: "Time", Kind: widget.InputTime, Value: "11:05"}
	tags := widget.Field{Name: "tags", Label: "Tags", Kind: widget.InputText, Value: "встречи, релиз"}
	attach := widget.Sec{
		Rubric: widget.Rubric{Text: "Attachments"},
		Body:   []widget.Node{widget.Paragraph("2026-08-10-sync-board.png · 412 KB · in the note")},
	}
	related := widget.Sec{
		Rubric: widget.Rubric{Text: "Related"},
		Body:   []widget.Node{widget.Paragraph("Синк-ап по релизу · event · 10 Aug")},
	}
	return widget.Side{
		Label: "Note details",
		Title: "Заметка со встречи 10.08",
		Open:  true,
		Blocks: []widget.SideBlock{
			{Field: &title},
			{Field: &text},
			{Field: &date},
			{Field: &timeF},
			{Field: &tags},
			{Sec: &attach},
			{Sec: &related},
		},
		Actions: []widget.Action{
			{Label: "Save", Kind: widget.ButtonPrimary},
			{Label: "Open full screen", Icon: "⤢"},
		},
	}.Render()
}
