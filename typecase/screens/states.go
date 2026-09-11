package screens

import "carrel.dev/typecase/widget"

// States assembles mockup m-states: empty, loading skeletons, offline refusal,
// partial failure, and an unavailable source.
func States() string {
	return statesFrame("Nothing connected", "Empty screen explains how the product works.",
		statesEmpty()) +
		statesFrame("Loading", "Row skeletons keep the same height as real rows.",
			statesLoading()) +
		statesFrame("Offline", "Shell stays up; writing is refused in words, not hidden buttons.",
			statesOffline()) +
		statesFrame("Partial response", "What arrived in time is shown; the missing source is named.",
			statesPartial()) +
		statesFrame("Unavailable source", "The source is marked unavailable in the rail.",
			statesUnavailable())
}

func statesFrame(label, caption, shell string) string {
	return `<section class="screens-frame"><h2 class="screens-frame__label">` + label +
		`</h2><div class="screens-frame__mock">` + shell +
		`</div><p class="screens-frame__caption">` + caption + `</p></section>`
}

func statesTop(refresh bool) widget.Top {
	top := shellTop()
	top.Create = widget.Menu{}
	if refresh {
		top.Create = widget.Menu{
			Label: "New note",
			Icon:  "✎",
			Items: []widget.MenuItem{{Label: "New note", Href: "/notes/new"}},
		}
	}
	return top
}

func statesRailFoot() widget.RailFoot {
	return widget.RailFoot{
		Actions: []widget.Action{{Label: "Connect account", Icon: "⊕"}},
		Build:   demoBuild,
	}
}

func statesEmpty() string {
	rail := shellNav("/calendar") +
		widget.Rail{
			Sections: []widget.RailSection{{
				Title: "Sources",
				State: widget.RailSecMuted,
				Rows:  []widget.TickRow{},
			}},
			Note: "Nothing connected yet",
			Foot: statesRailFoot(),
		}.Render()
	main := `<main class="tc-main" aria-label="Calendar"><div class="tc-main__body">` +
		widget.Head{Title: "Calendar", Sub: "No server connected yet"}.Render() +
		widget.Empty{
			Text: "Carrel stores nothing of its own: calendars, contacts and notes come from your DAV servers. Connect the first one — you will need an address, a user name and a password.",
			Action: widget.Action{
				Label: "Connect account",
				Kind:  widget.ButtonPrimary,
				Icon:  "+",
			},
		}.Render() +
		`</div></main>`
	return shellWrap(rail, main, "", "")
}

func statesLoading() string {
	skel18 := widget.SkelWidth("18px")
	skel16 := widget.SkelWidth("16px")
	rail := shellNav("/calendar") +
		widget.Rail{
			Sections: []widget.RailSection{
				{Rows: []widget.TickRow{{
					Label: "Combined calendar",
					Mark:  widget.Mark{Colour: "#4a6b52", Label: "Combined"},
					Tick:  widget.TickPartial,
				}}},
				{
					Title: "nextcloud · home",
					Rows: []widget.TickRow{
						{Label: "Личный", Mark: widget.Mark{Colour: "#4A6B52", Label: "Личный"}},
						{Label: "Работа", Mark: widget.Mark{Colour: "#3F6E8C", Label: "Работа"}},
					},
				},
				{
					Title: "radicale · vps",
					Rows: []widget.TickRow{
						{Label: "Проекты", Mark: widget.Mark{Colour: "#6B4E7A", Label: "Проекты"}},
					},
				},
			},
			Foot: widget.RailFoot{Build: demoBuild},
		}.Render() +
		widget.Skel{Shape: widget.SkelLine, Width: skel18, Lines: 1}.Render() +
		widget.Skel{Shape: widget.SkelLine, Width: skel16, Lines: 1}.Render()

	p22 := 22
	run := widget.RunBar{
		Summary: "Polling 6 sources…",
		At:      "0.4 s",
		Running: true,
		Percent: &p22,
	}.Render()
	head := widget.Head{
		Title: "Combined calendar",
		Sub:   "5 of 7 calendars · 10 – 16 August 2026",
		Actions: []widget.Action{
			{Label: "New event", Kind: widget.ButtonPrimary, Icon: "+"},
		},
	}.Render()
	list := widget.List{Groups: []widget.ListGroup{{
		Rubric: widget.Rubric{Text: "Monday · 10 August"},
		Rows: []widget.Row{
			{Variant: widget.RowLead, Mark: widget.Mark{Colour: "#c9bfa6", Label: "pending"}, State: widget.RowMuted, Cells: widget.RowCells{Lead: "…", Title: "…", Subtitle: "…"}},
			{Variant: widget.RowLead, Mark: widget.Mark{Colour: "#c9bfa6", Label: "pending"}, State: widget.RowMuted, Cells: widget.RowCells{Lead: "…", Title: "…", Subtitle: "…"}},
			{Variant: widget.RowLead, Mark: widget.Mark{Colour: "#c9bfa6", Label: "pending"}, State: widget.RowMuted, Cells: widget.RowCells{Lead: "…", Title: "…"}},
		},
	}}}.Render()
	skelBlock := widget.Skel{Shape: widget.SkelLine, Lines: 3}.Render()
	main := `<main class="tc-main" aria-label="Calendar"><div class="tc-main__body">` +
		head + run + list + skelBlock + widget.Kbd{Keys: []string{"Ctrl", "R"}}.Render() + `</div></main>`
	return `<div class="tc-shell" data-tc-shell data-tc-root>` + statesTop(true).Render() +
		`<div class="tc-shell__body"><div class="tc-shell__rail">` + rail +
		`</div><div class="tc-shell__main">` + main + `</div></div></div>`
}

func statesOffline() string {
	rail := shellNav("/calendar") +
		widget.Rail{
			Sections: []widget.RailSection{
				{Rows: []widget.TickRow{{
					Label: "Combined calendar",
					Mark:  widget.Mark{Colour: "#4a6b52", Label: "Combined"},
					Tick:  widget.TickPartial,
				}}},
				{
					Title: "nextcloud · home",
					State: widget.RailSecAlert,
					Rows: []widget.TickRow{
						{Label: "Личный", Mark: widget.Mark{Colour: "#4A6B52", Label: "Личный"}, State: widget.TickRowUnavailable},
						{Label: "Работа", Mark: widget.Mark{Colour: "#3F6E8C", Label: "Работа"}, State: widget.TickRowUnavailable},
					},
				},
				{
					Title: "radicale · vps",
					State: widget.RailSecAlert,
					Rows: []widget.TickRow{
						{Label: "Проекты", Mark: widget.Mark{Colour: "#6B4E7A", Label: "Проекты"}, State: widget.TickRowUnavailable},
					},
				},
			},
			Foot: widget.RailFoot{Build: demoBuild},
		}.Render()
	run := widget.RunBar{
		Summary: "No connection to carrel.example · last successful attempt 4 minutes ago",
		Actions: []widget.Action{{Label: "Retry"}},
	}.Render()
	head := widget.Head{
		Title: "Combined calendar",
		Sub:   "shown from this session’s memory",
	}.Render()
	n3 := 3
	list := widget.List{Groups: []widget.ListGroup{{
		Rubric: widget.Rubric{Text: "Monday · 10 August", Count: intPtr(n3)},
		Rows: []widget.Row{
			{Variant: widget.RowLead, Mark: widget.Mark{Colour: "#4A6B52", Label: "Личный"}, State: widget.RowMuted, Cells: widget.RowCells{Lead: "10:00", Title: "Синк-ап по релизу", Subtitle: "Переговорная 2", Detail: "nextcloud · Личный"}},
			{Variant: widget.RowLead, Mark: widget.Mark{Colour: "#3F6E8C", Label: "Работа"}, State: widget.RowMuted, Cells: widget.RowCells{Lead: "15:00", Title: "Созвон с подрядчиком", Subtitle: "Jitsi", Detail: "nextcloud · Работа"}},
		},
	}}}.Render()
	note := widget.Msg{
		Role: widget.MsgAlert,
		Text: "Creating and editing are not possible now: writing goes to the server, and the server is not there. The buttons are not hidden — they say the same if pressed.",
	}.Render()
	main := `<main class="tc-main" aria-label="Calendar"><div class="tc-main__body">` +
		run + head + list + note + `</div></main>`
	return shellWrap(rail, main, "", "")
}

func statesPartial() string {
	n2, n1, p83 := 2, 1, 83
	rail := shellNav("/calendar") +
		widget.Rail{
			Sections: []widget.RailSection{
				{Rows: []widget.TickRow{{
					Label: "Combined calendar",
					Mark:  widget.Mark{Colour: "#4a6b52", Label: "Combined"},
					Tick:  widget.TickPartial,
					Count: intPtr(n2),
				}}},
				{
					Title: "nextcloud · home",
					Rows: []widget.TickRow{
						{Label: "Личный", Mark: widget.Mark{Colour: "#4A6B52", Label: "Личный"}, Count: intPtr(n1), Tick: widget.TickOn},
						{Label: "Работа", Mark: widget.Mark{Colour: "#3F6E8C", Label: "Работа"}, Count: intPtr(n1), Tick: widget.TickOn},
					},
				},
				{
					Title: "baikal · office",
					State: widget.RailSecAlert,
					Rows: []widget.TickRow{
						{
							Label: "Общий",
							Mark:  widget.Mark{Colour: "#9A9280", Label: "Общий"},
							State: widget.TickRowUnavailable,
						},
					},
				},
			},
			Foot: widget.RailFoot{Build: demoBuild},
		}.Render()
	run := widget.RunBar{
		Summary: "Polled 5 of 6 · baikal · Общий: 401 Unauthorized",
		Running: true,
		Percent: &p83,
		Actions: []widget.Action{{Label: "Retry"}},
	}.Render()
	head := widget.Head{
		Title: "Combined calendar",
		Sub:   "6 of 7 calendars · one source did not answer",
		Actions: []widget.Action{
			{Label: "New event", Kind: widget.ButtonPrimary, Icon: "+"},
		},
	}.Render()
	list := widget.List{Groups: []widget.ListGroup{{
		Rubric: widget.Rubric{Text: "Monday · 10 August", Count: intPtr(n2)},
		Rows: []widget.Row{
			{Variant: widget.RowLead, Mark: widget.Mark{Colour: "#4A6B52", Label: "Личный"}, Cells: widget.RowCells{Lead: "10:00", Title: "Синк-ап по релизу", Subtitle: "Переговорная 2", Detail: "nextcloud · Личный"}},
			{Variant: widget.RowLead, Mark: widget.Mark{Colour: "#3F6E8C", Label: "Работа"}, Cells: widget.RowCells{Lead: "15:00", Title: "Созвон с подрядчиком", Subtitle: "Jitsi", Detail: "nextcloud · Работа"}},
		},
	}}}.Render()
	note := widget.Msg{
		Role: widget.MsgAccent,
		Text: "What arrived in time is shown. Records from the unavailable source are missing — not “there are none”, but “we have not seen them”.",
	}.Render()
	diag := widget.Diag{
		Summary: "baikal · Общий: 401 Unauthorized",
		Lines:   []string{"HTTP/1.1 401 Unauthorized", "WWW-Authenticate: Basic realm=\"Baikal\""},
	}.Render()
	main := `<main class="tc-main" aria-label="Calendar"><div class="tc-main__body">` +
		head + run + list + note + diag + `</div></main>`
	return shellWrap(rail, main, "", "")
}

func statesUnavailable() string {
	n13 := 13
	rail := shellNav("/contacts") +
		widget.Rail{
			Sections: []widget.RailSection{
				{Rows: []widget.TickRow{{
					Label: "Combined address book",
					Mark:  widget.Mark{Colour: "#4a6b52", Label: "Combined"},
					Tick:  widget.TickPartial,
					Count: intPtr(312),
				}}},
				{
					Title: "nextcloud · home",
					Rows: []widget.TickRow{
						{Label: "Контакты", Mark: widget.Mark{Colour: "#4A6B52", Label: "Контакты"}, Count: intPtr(248), Tick: widget.TickOn},
					},
				},
				{
					Title: "radicale · vps",
					Rows: []widget.TickRow{
						{
							Label: "Команда",
							Mark:  widget.Mark{Colour: "#6B4E7A", Label: "Команда"},
							Count: intPtr(n13),
							Tick:  widget.TickOn,
							State: widget.TickRowUnavailable,
						},
					},
				},
			},
			Foot: widget.RailFoot{Build: demoBuild},
		}.Render()
	head := widget.Head{
		Title: "Команда",
		Sub:   "radicale · vps · source unavailable",
	}.Render()
	list := widget.List{Groups: []widget.ListGroup{{Rows: []widget.Row{
		{
			Variant: widget.RowAvatar,
			Mark:    widget.Mark{Colour: "#6B4E7A", Label: "Команда"},
			State:   widget.RowMuted,
			Cells: widget.RowCells{
				Avatar:   widget.Avatar{Initials: "ПК", Colour: "#6B4E7A"},
				Title:    "Пётр Ковалёв",
				Subtitle: "p.kovalev@example.org",
				Detail:   "radicale · Команда",
			},
		},
	}}}}.Render()
	main := `<main class="tc-main" aria-label="Contacts"><div class="tc-main__body">` +
		head + list + `</div></main>`
	return shellWrap(rail, main, "", "")
}
