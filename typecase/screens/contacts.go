package screens

import "carrel.dev/typecase/widget"

// Contacts assembles mockup s-contacts: list, card panel, edit form with
// field sets, crop, range filter, search jumps, and a loading run bar.
func Contacts() string {
	return shellFrame(shellNav("/contacts"), contactsRail(), contactsMain(), contactsSide(), contactsOverlays())
}

func contactsRail() widget.Rail {
	n312, n248, n51, n13 := 312, 248, 51, 13
	return widget.Rail{
		Sections: []widget.RailSection{
			{
				Rows: []widget.TickRow{{
					Label: "Combined address book",
					Count: intPtr(n312),
					Tick:  widget.TickPartial,
				}},
			},
			{
				Title: "nextcloud · home",
				Rows: []widget.TickRow{
					{Label: "Контакты", Mark: widget.Mark{Colour: "#4A6B52", Label: "Контакты"}, Count: intPtr(n248)},
					{Label: "Архив", Mark: widget.Mark{Colour: "#A05A2C", Label: "Архив"}, Count: intPtr(n51)},
				},
			},
			{
				Title: "radicale · vps",
				Rows: []widget.TickRow{
					{Label: "Команда", Mark: widget.Mark{Colour: "#6B4E7A", Label: "Команда"}, Count: intPtr(n13), State: widget.TickRowReadOnly},
					{Label: "Рассылки", Mark: widget.Mark{Colour: "#8C6239", Label: "Рассылки"}, Tick: widget.TickOff, State: widget.TickRowGone},
				},
			},
		},
		Chips: []widget.Chip{
			{Label: "all", On: true},
			{Label: "работа"},
			{Label: "издательство"},
		},
		Foot: widget.RailFoot{
			Actions: []widget.Action{
				{Label: "New address book", Icon: "+"},
				{Label: "Connect account", Icon: "⎈"},
				{Label: "Subscribe to a feed", Icon: "🔗"},
			},
			Build: demoBuild,
		},
	}
}

func contactsMain() widget.Main {
	head := widget.Head{
		Mark:  widget.Mark{Colour: "#4a6b52", Label: "Combined"},
		Title: "Combined address book",
		Sub:   "3 of 4 books · 2 accounts · 312 cards",
		Actions: []widget.Action{
			{Label: "New contact", Kind: widget.ButtonPrimary, Icon: "+"},
		},
		Rank2: []widget.Action{
			{Label: "Import", Icon: "↑"},
			{Label: "Export", Icon: "↓"},
			{Label: "Print", Icon: "🖨"},
		},
	}

	search := widget.Search{
		Action: "/search",
		History: []widget.SearchRow{
			{Query: "Ковалёва", Href: "/search?q=Ковалёва"},
		},
		Jump: []widget.JumpRow{
			{Label: "Анна Ковалёва", Hint: "Contact", Href: "/contacts/anna"},
		},
		Suggesting: true,
	}

	bar := widget.Bar{
		Left: []widget.Node{
			widget.Input{Name: "filter", Kind: widget.InputText, Placeholder: "Filter the loaded cards"},
			widget.Seg{
				Name:    "sort",
				Chosen:  "az",
				Options: []widget.SegOption{{Label: "A–Z", Value: "az"}, {Label: "Recently changed", Value: "changed"}},
			},
		},
		Right: []widget.Node{
			widget.Button{Label: "Columns", Icon: "▦", Size: widget.ButtonSizeBar},
			widget.Check{Name: "compact", Label: "Compact rows"},
			widget.Num{Value: intPtr(312)},
		},
	}

	rangeFilter := widget.Range{From: "2026-01-01", To: "2026-08-31", Presets: []widget.Action{{Label: "Last 7 days"}}}

	n3, n2 := 3, 2
	listA := widget.List{
		Groups: []widget.ListGroup{{
			Rubric: widget.Rubric{Text: "А", Count: intPtr(n3)},
			Rows: []widget.Row{
				{
					Variant:  widget.RowAvatar,
					Selected: true,
					Mark:     widget.Mark{Colour: "#4A6B52", Label: "Контакты"},
					Cells: widget.RowCells{
						Avatar:   widget.Avatar{Initials: "АК", Colour: "#4A6B52"},
						Title:    "Анна Ковалёва",
						Subtitle: "+7 912 445-10-88 · a.kovaleva@example.org",
					},
				},
				{
					Variant: widget.RowAvatar,
					Mark:    widget.Mark{Colour: "#6B4E7A", Label: "Команда"},
					Cells: widget.RowCells{
						Avatar:   widget.Avatar{Image: "/img/sample.svg", Initials: "АБ"},
						Title:    "Артём Белов",
						Subtitle: "+7 903 118-22-04 · a.belov@example.org",
					},
					Badges: []widget.Badge{{Label: "linked ×2", Role: widget.BadgeLinked}},
				},
				{
					Variant: widget.RowAvatar,
					Mark:    widget.Mark{Colour: "#4A6B52", Label: "Контакты"},
					Cells: widget.RowCells{
						Avatar:   widget.Avatar{Initials: "АС", Colour: "#4A6B52"},
						Title:    "Алексей Смирнов",
						Subtitle: "+7 916 200-71-15",
					},
				},
			},
		}},
	}

	listV := widget.List{
		Groups: []widget.ListGroup{{
			Rubric: widget.Rubric{Text: "В", Count: intPtr(n2)},
			Rows: []widget.Row{
				{
					Variant: widget.RowAvatar,
					Mark:    widget.Mark{Colour: "#4A6B52", Label: "Контакты"},
					Cells: widget.RowCells{
						Avatar:   widget.Avatar{Initials: "ВГ", Colour: "#4A6B52"},
						Title:    "Виктор Гончаров",
						Subtitle: "v.goncharov@example.org",
					},
				},
				{
					Variant: widget.RowAvatar,
					Mark:    widget.Mark{Colour: "#A05A2C", Label: "Архив"},
					Cells: widget.RowCells{
						Avatar:   widget.Avatar{Initials: "ВЛ", Colour: "#A05A2C"},
						Title:    "Вера Лебедева",
						Subtitle: "+7 905 331-09-42 · vera@example.net",
					},
				},
			},
		}},
	}

	runbar := widget.RunBar{
		Summary: "Loading more cards",
		Running: true,
		Percent: intPtr(42),
		Items: []widget.RunItem{
			{Label: "nextcloud · Контакты", State: widget.RunItemDone},
			{Label: "nextcloud · Архив", State: widget.RunItemRunning},
			{Label: "radicale · Команда", State: widget.RunItemWaiting},
		},
	}

	editForm := widget.Form{
		Action: "/contacts/save",
		Sets: []widget.FieldSet{
			{
				Legend: "Photo",
				Rows: []widget.Field{
					{Name: "display", Label: "Display name", Kind: widget.InputText, Value: "Анна Ковалёва"},
					{Name: "given", Label: "Given", Kind: widget.InputText, Value: "Анна"},
					{Name: "family", Label: "Family", Kind: widget.InputText, Value: "Ковалёва"},
				},
			},
			{
				Legend: "Phone numbers",
				Rows: []widget.Field{
					{Name: "phone-label", Label: "Label", Kind: widget.InputText, Value: "mobile"},
					{Name: "phone", Label: "Number", Kind: widget.InputTel, Value: "+7 912 445-10-88"},
					{Name: "phone2-label", Label: "Label", Kind: widget.InputText, Value: "work"},
					{Name: "phone2", Label: "Number", Kind: widget.InputTel, Value: "+7 495 120-45-00"},
				},
			},
			{
				Legend: "Email addresses",
				Rows: []widget.Field{
					{Name: "email-label", Label: "Label", Kind: widget.InputText, Value: "work"},
					{Name: "email", Label: "Address", Kind: widget.InputEmail, Value: "a.kovaleva@example.org"},
				},
			},
		},
		Foot: widget.FormFoot{
			Primary: widget.Action{Label: "Save", Kind: widget.ButtonPrimary},
			Rest:    []widget.Action{{Label: "Cancel"}},
		},
	}

	return widget.Main{
		Label: "Contacts",
		Body: []widget.Node{
			head,
			search,
			bar,
			rangeFilter,
			widget.Kbd{Keys: []string{"Ctrl", "F"}},
			listA,
			listV,
			runbar,
			editForm,
			widget.Crop{Href: "/img/sample.svg", Zoom: 100},
		},
	}
}

func contactsSide() widget.Side {
	fields := widget.Fields{Rows: []widget.FieldRow{
		{Label: "Organisation", Value: "Издательство «Слово»"},
		{Label: "Title", Value: "Выпускающий редактор"},
		{Label: "Phone · mobile", Value: "+7 912 445-10-88"},
		{Label: "Phone · work", Value: "+7 495 120-45-00"},
		{Label: "Email · work", Value: "a.kovaleva@example.org"},
		{Label: "Birthday", Value: "1987-03-14"},
		{Label: "Note", Value: "Пишет по вторникам, отвечает в тот же день."},
	}}

	source := widget.Sec{
		Rubric: widget.Rubric{Text: "Source"},
		Body:   []widget.Node{widget.Prose{Blocks: []widget.ProseBlock{{Kind: widget.ProseCode, Code: "UID   4f2b-9c31-a0e7\nRead  2 min ago"}}}},
	}

	danger := widget.Sec{
		Body: []widget.Node{widget.Danger{Title: "Danger zone", Rows: []widget.DangerRow{{
			Title:  "Delete contact",
			Text:   "Removes the card from the server.",
			Action: widget.Action{Label: "Delete contact", Kind: widget.ButtonDanger},
		}}}},
	}

	return widget.Side{
		Label: "Contact details",
		Title: "Анна Ковалёва",
		Open:  true,
		Blocks: []widget.SideBlock{
			{Sec: &widget.Sec{Body: []widget.Node{
				widget.Avatar{Initials: "АК", Colour: "#4A6B52", Size: widget.AvatarSizeCard},
				widget.Bar{Left: []widget.Node{widget.Action{Label: "Edit"}, widget.Action{Label: "All records"}}},
			}}},
			{Fields: &fields},
			{Sec: &source},
			{Sec: &danger},
		},
	}
}

func contactsOverlays() string {
	return widget.Dialog{
		ID:      "move-contact",
		Title:   "Move contact",
		Text:    "Анна Ковалёва · from Контакты · nextcloud",
		Confirm: widget.Action{Label: "Move here", Kind: widget.ButtonPrimary},
		Cancel:  widget.Action{Label: "Cancel"},
	}.Render()
}
