package screens

import "carrel.dev/typecase/widget"

// Notes assembles mockup s-notes figure 1: list, side panel, and a closed
// quick-note sheet in the DOM.
func Notes() string {
	return notesListFrame(false)
}

// NotesRead is full-screen reading with neighbour notes in the rail tree.
func NotesRead() string {
	return shellFrame(notesReadNav(), notesReadRail(), notesReadMain(), widget.Side{}, "")
}

// NotesEdit is focus editing: top bar and main column only, no rail or side.
func NotesEdit() string {
	return widget.Shell{
		Top:  shellTop(),
		Main: notesEditMain(),
	}.Render()
}

// NotesQuick is the list frame with the quick-note sheet open.
func NotesQuick() string {
	return notesListFrame(true)
}

func notesListFrame(quickOpen bool) string {
	return shellFrame(shellNav("/notes"), notesRail(), notesListMain(), notesSide(), notesQuickSheet(quickOpen))
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

func notesReadNav() widget.Nav {
	return widget.Nav{
		Items:   []widget.NavItem{{Label: "Combined notebook", Icon: "←", Href: "/notes"}},
		Current: "/notes",
	}
}

func notesReadRail() widget.Rail {
	n64, n22 := 64, 22
	return widget.Rail{
		Sections: []widget.RailSection{
			{
				Title: "nextcloud · home",
				Rows: []widget.TickRow{
					{Label: "Записи", Mark: widget.Mark{Colour: "#4A6B52", Label: "Записи"}, Count: intPtr(n64), Tick: widget.TickOn},
					{Label: "Дневник", Mark: widget.Mark{Colour: "#6B4E7A", Label: "Дневник"}, Count: intPtr(n22), State: widget.TickRowUnavailable},
				},
			},
			{
				Title: "Neighbours",
				Body: []widget.Node{
					widget.Tree{
						Nodes: []widget.TreeNode{
							{ID: "crdt", Label: "Что почитать про CRDT"},
							{ID: "meeting", Label: "Заметка со встречи"},
							{ID: "dacha", Label: "Про дорогу до дачи"},
						},
						Chosen: "meeting",
					},
				},
			},
		},
	}
}

func notesListMain() widget.Main {
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
				},
				Rank2: []widget.Action{
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
								Date:    "10 Aug",
								Source:  "nextcloud · Записи",
							},
						},
						{
							Variant: widget.RowExcerpt,
							Mark:    widget.Mark{Colour: "#4A6B52", Label: "Записи"},
							Cells: widget.RowCells{
								Title:   "Что почитать про CRDT",
								Excerpt: "Кляйн, глава 5; статья Шапиро 2011; демка automerge…",
								Tags:    []widget.Tag{{Label: "книги", Href: "/tags/книги"}},
								Date:    "8 Aug",
								Source:  "nextcloud · Записи",
							},
						},
						{
							Variant: widget.RowExcerpt,
							Mark:    widget.Mark{Colour: "#6B4E7A", Label: "Дневник"},
							Cells: widget.RowCells{
								Title:   "Про дорогу до дачи",
								Excerpt: "Съезд после заправки, потом грунтовка два километра…",
								Date:    "3 Aug",
								Source:  "nextcloud · Дневник",
							},
							Badges: []widget.Badge{{Label: "linked", Role: widget.BadgeLinked}},
						},
					},
				}},
			},
		},
	}
}

func notesReadMain() widget.Main {
	return widget.Main{
		Label: "Note",
		Body:  []widget.Node{notesReadDoc()},
	}
}

func notesReadDoc() widget.Doc {
	return widget.Doc{
		Mode:  widget.DocReading,
		Width: widget.DocWidthFull,
		Bar: widget.DocBar{
			Left: []widget.Action{
				{Label: "Edit", Kind: widget.ButtonPrimary, Icon: "✎"},
			},
			Rank2: []widget.Action{
				{Label: "Export Markdown"},
				{Label: "Print", Icon: "🖨"},
				{Label: "Source"},
			},
		},
		Body: widget.Prose{Blocks: []widget.ProseBlock{
			{Kind: widget.ProseH1, Text: "Заметка со встречи 10.08"},
			{Kind: widget.ProseP, Text: "Договорились заморозить ветку в среду."},
			{Kind: widget.ProseP, Text: "Письмо пользователям пишет Анна, черновик к вечеру вторника. Виктор проверяет миграцию на копии базы — если не успевает, переносим на следующий спринт."},
			{Kind: widget.ProseH2, Text: "Что делаем"},
			{Kind: widget.ProseUL, Items: []string{"проверить changelog", "попросить у Артёма скриншоты"}},
			{Kind: widget.ProseP, Text: "Перенос старых вложений в новую папку — Виктор посмотрит, сколько их вообще, и скажет в четверг."},
		}},
		Meta: widget.DocMeta{Sections: []widget.MetaSection{
			{Title: "Tags", Tags: []widget.Tag{{Label: "встречи", Href: "/tags/встречи"}, {Label: "релиз", Href: "/tags/релиз"}}},
			{Title: "Related", Rows: []widget.FieldRow{
				{Label: "Синк-ап по релизу", Value: "event · 10 Aug"},
				{Label: "Дописать раздел про кэш", Value: "task"},
			}},
			{Title: "Attachments", Rows: []widget.FieldRow{
				{Label: "2026-08-10-sync-board.png", Value: "412 KB · in the note"},
				{Label: "запись-встречи.m4a", Value: "24 MB · Documents / carrel"},
			}},
			{Title: "Source", Rows: []widget.FieldRow{
				{Label: "UID", Value: "9e31-40b7-c012", Path: true},
				{Label: "Read", Value: "2 min ago"},
			}},
		}},
	}
}

func notesEditMain() widget.Main {
	return widget.Main{
		Label: "Edit note",
		Body: []widget.Node{
			widget.Doc{
				Mode:  widget.DocEditing,
				Width: widget.DocWidthFull,
				Bar: widget.DocBar{
					Left: []widget.Action{
						{Label: "Save", Kind: widget.ButtonPrimary},
						{Label: "Cancel"},
					},
					Rank2: []widget.Action{
						{Label: "B"},
						{Label: "I"},
						{Label: "H2"},
						{Label: "• list"},
						{Label: "task", Icon: "☑"},
						{Label: "Link", Icon: "🔗"},
						{Label: "Attach", Icon: "↑"},
					},
					Right: []widget.Action{
						{Label: "Focus", Icon: "⤢"},
					},
					Dirty: true,
				},
				Body: widget.Prose{Blocks: []widget.ProseBlock{
					{Kind: widget.ProseH1, Text: "Заметка со встречи 10.08"},
					{Kind: widget.ProseP, Text: "Договорились заморозить ветку в среду."},
					{Kind: widget.ProseP, Text: "Письмо пользователям пишет Анна, черновик к вечеру вторника. Виктор проверяет миграцию на копии базы — если не успевает, переносим на следующий спринт."},
					{Kind: widget.ProseH2, Text: "Что делаем"},
					{Kind: widget.ProseUL, Items: []string{"проверить changelog", "попросить у Артёма скриншоты"}},
					{Kind: widget.ProseH2, Text: "Отложено до следующего спринта"},
					{Kind: widget.ProseP, Text: "Перенос старых вложений в новую папку — Виктор посмотрит, сколько их вообще."},
				}},
			},
		},
	}
}

func notesQuickSheet(open bool) string {
	return widget.Sheet{
		ID:     "quick-note",
		Title:  "New note",
		Target: "Записи · nextcloud",
		Open:   open,
		Body: []widget.Node{
			widget.Field{Name: "quick", Kind: widget.InputTextarea, Value: "Позвонить в МФЦ во вторник до 12:00 — уточнить, нужен ли оригинал доверенности."},
			widget.Paragraph("The title is taken from the first line. The date is now."),
		},
		Actions: []widget.Action{
			{Label: "Save note", Kind: widget.ButtonPrimary},
			{Label: "Open full screen"},
			{Label: "Cancel"},
		},
	}.Render()
}

func notesSide() widget.Side {
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
	}
}
