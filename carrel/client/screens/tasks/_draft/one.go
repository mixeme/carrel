// Drafted by frame2go from frame «Задачи · один список» (s-tasks-15) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// Tasks15 is frame «Задачи · один список» (s-tasks-15).
func Tasks15() interface{ Render() string } {
	return w.Shell{
		Top: &w.Top{
			Brand:  w.Brand{Logo: &w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63"}},
			Search: &w.Search{Query: w.Query{Placeholder: "Search everything…", Label: "Search everything…"}},
			Actions: []w.TopAction{
				w.Button{Label: "New note", Menu: true, Icon: "quill", Kind: w.ButtonPrimary},
				w.Button{Label: "2 min ago", Icon: "refresh"},
				w.UserMenu{Name: "mix"},
			},
		},
		Body: w.ShellBody{
			Rail: []w.RailPart{
				w.Nav{
					Items: []w.NavItem{
						{Icon: "con", Label: "Contacts", Count: &w.Num{Value: 5}, Href: "#"},
						{Icon: "cal", Label: "Calendar", Count: &w.Num{Value: 6}, Href: "#"},
						{Icon: "task", Label: "Tasks", Count: &w.Num{Value: 2}, Href: "#"},
						{Icon: "note", Label: "Notes", Count: &w.Num{Value: 3}, Href: "#"},
						{Icon: "file", Label: "Files", Href: "#"},
						{Icon: "dup", Label: "Duplicates", Count: &w.Num{Value: 2}, Href: "#"},
					},
					On: 3,
				},
				w.RailSec{
					Items: []w.RailSecPart{
						w.TickRow{
							Ticked: w.TickedPart,
							Kind:   w.TickRowAll,
							Mark:   w.Mark{Kind: w.MarkMulti},
							Label:  "Combined list",
							Meta:   "6",
						},
					},
				},
				w.RailSec{
					Items: []w.RailSecPart{
						w.Rubric{Text: "nextcloud"},
						w.TickRow{Mark: w.Mark{Colour: "#4A6B52"}, Label: "Дела", Meta: "4", Current: true},
						w.TickRow{Mark: w.Mark{Colour: "#3F6E8C"}, Label: "Работа", Meta: "2"},
					},
				},
				w.RailSec{
					Items: []w.RailSecPart{
						w.Rubric{Text: "Subscriptions", Items: []w.RubricPart{w.Badge{Label: "external"}}},
						w.TickRow{
							Ticked: w.TickedOff,
							Kind:   w.TickRowExternal,
							Mark:   w.Mark{Colour: "#8C6239"},
							Label:  "Праздники РФ",
							Meta:   "—",
							Hidden: true,
							Trail:  w.Badge{Label: "ro"},
						},
					},
				},
				w.RailSec{
					Items: []w.RailSecPart{
						w.Rubric{Text: "Tags"}},
					Tags: []w.Tag{w.Tag{Label: "all", On: true}, w.Tag{Label: "дом"}, w.Tag{Label: "релиз"}, w.Tag{Label: "read"}},
				},
				w.RailFoot{
					Items: []w.RailFootPart{
						w.Button{Icon: "plus", Title: "New list"},
						w.Button{Icon: "server", Title: "Connect account"},
						w.Button{Icon: "link", Title: "Subscribe to a feed"},
						w.Emblem{Symbol: "davloom", Title: "davloom · running · 3 devices"},
					},
					About: &w.About{Name: "Carrel", Build: w.Mono{Text: "0.0.1 · 786949a"}, Href: "#rest"},
				},
			},
			Main: w.Main{
				Items: []w.MainPart{
					w.Head{
						Title: &w.HeadTitle{Mark: &w.Mark{Colour: "#4A6B52", Size: w.MarkH20}, Text: "Дела"},
						Subs:  []w.HeadSub{w.HeadSub{Parts: []w.HeadSubPart{w.Text("nextcloud · 4 tasks")}}},
						Acts: &w.Acts{
							Items: []w.ActsPart{
								w.Button{Label: "New task", Icon: "plus", Kind: w.ButtonPrimary},
								w.Button{Label: "Import", Icon: "up"},
								w.Button{Label: "Export", Icon: "down"},
								w.Button{Label: "Print", Icon: "print"},
							},
						},
					},
					w.Bar{
						Items: []w.BarPart{
							w.Seg{Options: []w.SegOption{{Label: "Open 3", On: true}, {Label: "Done 1"}, {Label: "All 4"}}},
							w.Seg{
								Options:   []w.SegOption{{Label: "By due", On: true}, {Label: "By priority"}, {Label: "Recently changed"}},
								Secondary: true,
							},
							w.Button{Label: "Columns", Icon: "cols"},
						},
					},
					w.ListGroup{Title: w.Rubric{Text: "Overdue", Form: w.RubricInline}, Count: &w.Num{Value: 1, Alert: true}},
					w.List{
						Rows: []w.ListRow{
							w.DueRow{
								Tick:     w.Tick{Label: "Done"},
								Mark:     w.Mark{Colour: "#4A6B52"},
								Title:    []w.Inline{w.Text("Отправить документы в МФЦ")},
								Tags:     []w.RowTag{w.Tag{Label: "дом"}},
								Due:      "7 Aug",
								Progress: "—",
								Single:   true,
								Overdue:  true,
								Href:     "#",
							},
						},
					},
					w.ListGroup{Title: w.Rubric{Text: "This week", Form: w.RubricInline}, Count: &w.Num{Value: 1}},
					w.List{
						Rows: []w.ListRow{
							w.DueRow{
								Tick:       w.Tick{Label: "Done"},
								Mark:       w.Mark{Colour: "#4A6B52"},
								Title:      []w.Inline{w.Text("Собрать чемодан ")},
								TitleBadge: &w.Badge{Label: "linked ×2", Tone: w.BadgeLinked, Title: "nextcloud · Дела, nextcloud · Работа"},
								Tags:       []w.RowTag{w.Tag{Label: "дом"}},
								Due:        "12 Aug 18:00",
								Progress:   "—",
								Single:     true,
								Href:       "#",
							},
						},
					},
					w.ListGroup{Title: w.Rubric{Text: "No due date", Form: w.RubricInline}, Count: &w.Num{Value: 1}},
					w.List{
						Rows: []w.ListRow{
							w.DueRow{
								Tick:     w.Tick{Done: true, Label: "Done"},
								Mark:     w.Mark{Colour: "#4A6B52"},
								Title:    []w.Inline{w.Text("Продлить страховку")},
								Due:      "—",
								Progress: "100%",
								Single:   true,
								Href:     "#",
							},
						},
					},
				},
			},
		},
	}
}
