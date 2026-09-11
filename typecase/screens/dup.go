package screens

import "carrel.dev/typecase/widget"

// Duplicates assembles mockup s-dup: candidate groups, property diffs, tick rows,
// and the merge confirmation dialog.
func Duplicates() string {
	return shellWrap(dupRail(), dupMain(), "", dupOverlays())
}

func dupRail() string {
	n248, n13, n51 := 248, 13, 51
	return shellNav("/duplicates") +
		widget.Rail{
			Sections: []widget.RailSection{
				{
					Title: "Sources",
					Rows: []widget.TickRow{
						{Label: "Контакты", Mark: widget.Mark{Colour: "#4A6B52", Label: "Контакты"}, Count: intPtr(n248)},
						{Label: "Команда", Mark: widget.Mark{Colour: "#6B4E7A", Label: "Команда"}, Count: intPtr(n13)},
						{Label: "Архив", Mark: widget.Mark{Colour: "#A05A2C", Label: "Архив"}, Count: intPtr(n51)},
					},
				},
			},
			Foot: widget.RailFoot{
				Actions: []widget.Action{},
				Build:   demoBuild,
			},
		}.Render()
}

func dupMain() string {
	head := widget.Head{
		Title: "Duplicates",
		Sub:   "361 records compared · 4 to look at · matching at 70 points and above",
	}.Render() +
		widget.Seg{
			Name:    "kind",
			Chosen:  "all",
			Options: []widget.SegOption{{Label: "All 4", Value: "all"}, {Label: "Contacts 1", Value: "contacts"}, {Label: "Events 1", Value: "events"}, {Label: "Tasks 1", Value: "tasks"}, {Label: "Notes 1", Value: "notes"}},
		}.Render()

	msg := widget.Msg{
		Role: widget.MsgAccent,
		Text: "Linking changes nothing on any server: the records stay where they are and are shown as one. Merging on the server is a separate, destructive step you are asked to confirm.",
	}.Render()

	contactGroup := dupCard("Артём Белов", "candidate ×2 · 86 points",
		widget.List{Groups: []widget.ListGroup{{Rows: []widget.Row{
			{
				Variant: widget.RowAvatar,
				Mark:    widget.Mark{Colour: "#4A6B52", Label: "Контакты"},
				Cells: widget.RowCells{
					Avatar:   widget.Avatar{Initials: "АБ", Colour: "#4A6B52"},
					Title:    "Артём Белов",
					Subtitle: "+7 903 118-22-04 · a.belov@example.org",
				},
			},
			{
				Variant: widget.RowAvatar,
				Mark:    widget.Mark{Colour: "#6B4E7A", Label: "Команда"},
				Cells: widget.RowCells{
					Avatar:   widget.Avatar{Image: "/img/sample.svg", Initials: "АБ"},
					Title:    "Белов Артём",
					Subtitle: "+7 903 118-22-04 · belov@team.example",
				},
			},
		}}}},
		[]widget.Diff{
			{Property: "Display name", Options: []widget.DiffOption{{Label: "Артём Белов", Value: "a"}, {Label: "Белов Артём", Value: "b"}}, Chosen: "a"},
			{Property: "Organisation", Options: []widget.DiffOption{{Label: "Тензор", Value: "a"}, {Label: "—", Value: "b"}}, Chosen: "a"},
			{Property: "Photo", Options: []widget.DiffOption{{Label: "nextcloud", Value: "a"}, {Label: "radicale", Value: "b"}}, Chosen: "b"},
		},
	)

	taskGroup := dupCard("Собрать чемодан", "candidate ×2 · 74 points",
		widget.List{Groups: []widget.ListGroup{{Rows: []widget.Row{
			{
				Variant: widget.RowTick,
				Mark:    widget.Mark{Colour: "#4A6B52", Label: "Дела"},
				Cells:   widget.RowCells{Title: "Собрать чемодан", Due: "12 Aug 18:00", Tags: []widget.Tag{{Label: "дом"}}},
			},
			{
				Variant: widget.RowTick,
				Mark:    widget.Mark{Colour: "#6B4E7A", Label: "Личное"},
				Cells:   widget.RowCells{Title: "Собрать чемодан", Due: "12 Aug"},
			},
		}}}},
		[]widget.Diff{
			{Property: "Due", Options: []widget.DiffOption{{Label: "12 Aug 18:00", Value: "a"}, {Label: "12 Aug", Value: "b"}}, Chosen: "a"},
			{Property: "Tags", Options: []widget.DiffOption{{Label: "дом", Value: "a"}, {Label: "—", Value: "b"}}, Chosen: "a"},
		},
	)

	noteGroup := dupCard("Что почитать про CRDT", "candidate ×2 · 80 points",
		widget.List{Groups: []widget.ListGroup{{Rows: []widget.Row{
			{
				Variant: widget.RowExcerpt,
				Mark:    widget.Mark{Colour: "#4A6B52", Label: "Записи"},
				Cells: widget.RowCells{
					Title:   "Что почитать про CRDT",
					Excerpt: "Кляйн, глава 5; статья Шапиро 2011…",
					Tags:    []widget.Tag{{Label: "книги"}},
				},
			},
			{
				Variant: widget.RowExcerpt,
				Mark:    widget.Mark{Colour: "#6B4E7A", Label: "Дневник"},
				Cells: widget.RowCells{
					Title:   "Что почитать про CRDT",
					Excerpt: "Кляйн, глава 5; статья Шапиро 2011…",
				},
			},
		}}}},
		[]widget.Diff{
			{Property: "Tags", Options: []widget.DiffOption{{Label: "книги", Value: "a"}, {Label: "—", Value: "b"}}, Chosen: "a"},
		},
	)

	agreed := widget.Diff{
		Property: "FN",
		Agreed:   true,
		Options:  []widget.DiffOption{{Label: "Анна Ковалёва", Value: "a"}},
		Chosen:   "a",
	}.Render()

	return `<main class="tc-main" aria-label="Duplicates"><div class="tc-main__body">` +
		head + msg + agreed + contactGroup + taskGroup + noteGroup +
		`</div></main>`
}

func dupCard(title, badge string, list widget.List, diffs []widget.Diff) string {
	var diffHTML string
	for _, d := range diffs {
		diffHTML += d.Render()
	}
	actions := widget.Bar{Left: []widget.Node{
		widget.Action{Label: "Link them", Kind: widget.ButtonPrimary},
		widget.Action{Label: "Not duplicates"},
		widget.Action{Label: "Merge on the server", Kind: widget.ButtonDanger},
	}}.Render()
	return widget.Card{
		Rubric: widget.Rubric{Text: title},
		Body:   []widget.Node{widget.Paragraph(badge)},
	}.Render() + list.Render() + diffHTML + actions
}

func dupOverlays() string {
	return widget.Dialog{
		ID:      "merge-dialog",
		Open:    true,
		Title:   "Merge on the server",
		Text:    "The kept record is rewritten with the values you chose. The other is deleted. This cannot be undone from here.",
		Danger:  true,
		Confirm: widget.Action{Label: "Write the merge and delete the rest", Kind: widget.ButtonDanger},
		Cancel:  widget.Action{Label: "Cancel"},
	}.Render()
}
