// Drafted by frame2go from frame «Задачи · фильтры, просрочка» (s-tasks-1) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// Tasks1 is frame «Задачи · фильтры, просрочка» (s-tasks-1).
func Tasks1() interface{ Render() string } {
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
						{Icon: "con", Label: "Contacts", Count: &w.Num{Value: 312}, Href: "#"},
						{Icon: "cal", Label: "Calendar", Count: &w.Num{Value: 14}, Href: "#"},
						{Icon: "task", Label: "Tasks", Count: &w.Num{Value: 3}, Href: "#"},
						{Icon: "note", Label: "Notes", Count: &w.Num{Value: 86}, Href: "#"},
						{Icon: "file", Label: "Files", Href: "#"},
						{Icon: "dup", Label: "Duplicates", Count: &w.Num{Value: 3}, Href: "#"},
					},
					On: 3,
				},
				w.RailSec{
					Items: []w.RailSecPart{
						w.TickRow{
							Ticked:  w.TickedPart,
							Kind:    w.TickRowAll,
							Mark:    w.Mark{Kind: w.MarkMulti},
							Label:   "Combined list",
							Meta:    "30",
							Current: true,
						},
					},
				},
				w.RailSec{
					Items: []w.RailSecPart{
						w.Rubric{Text: "nextcloud"},
						w.TickRow{Mark: w.Mark{Colour: "#4A6B52"}, Label: "Дела", Meta: "21"},
						w.TickRow{Mark: w.Mark{Colour: "#3F6E8C"}, Label: "Работа", Meta: "9"},
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
						Title: &w.HeadTitle{Mark: &w.Mark{Kind: w.MarkMulti, Size: w.MarkH20}, Text: "Combined list"},
						Subs:  []w.HeadSub{w.HeadSub{Parts: []w.HeadSubPart{w.Text("2 of 3 lists · nextcloud · 30 tasks")}}},
						Acts: &w.Acts{
							Items: []w.ActsPart{
								w.Button{Label: "New task", Menu: true, Icon: "plus", Kind: w.ButtonPrimary},
								w.Button{Label: "Import", Icon: "up"},
								w.Button{Label: "Export", Icon: "down"},
								w.Button{Label: "Print", Icon: "print"},
							},
						},
					},
					w.Bar{
						Items: []w.BarPart{
							w.Seg{Options: []w.SegOption{{Label: "Open 12", On: true}, {Label: "Done 48"}, {Label: "All 60"}}},
							w.Seg{
								Options:   []w.SegOption{{Label: "By due", On: true}, {Label: "By priority"}, {Label: "Recently changed"}},
								Secondary: true,
							},
							w.Button{Label: "Columns", Icon: "cols"},
						},
					},
					w.ListGroup{Title: w.Rubric{Text: "Overdue", Form: w.RubricInline}, Count: &w.Num{Value: 3, Alert: true}},
					w.List{
						Rows: []w.ListRow{
							w.DueRow{
								Tick:     w.Tick{Label: "Done"},
								Mark:     w.Mark{Colour: "#4A6B52"},
								Title:    []w.Inline{w.Text("Отправить документы в МФЦ")},
								Tags:     []w.RowTag{w.Tag{Label: "дом"}},
								Due:      "7 Aug",
								Progress: "—",
								Source:   "nextcloud · Дела",
								Overdue:  true,
								Href:     "#",
							},
							w.DueRow{
								Tick:     w.Tick{Label: "Done"},
								Mark:     w.Mark{Colour: "#3F6E8C"},
								Title:    []w.Inline{w.Text("Ответить подрядчику по смете")},
								Tags:     []w.RowTag{w.Tag{Label: "релиз"}},
								Due:      "8 Aug",
								Progress: "40%",
								Source:   "nextcloud · Работа",
								Overdue:  true,
								Href:     "#",
							},
						},
					},
					w.ListGroup{Title: w.Rubric{Text: "This week", Form: w.RubricInline}, Count: &w.Num{Value: 5}},
					w.List{
						Rows: []w.ListRow{
							w.DueRow{
								Tick:       w.Tick{Label: "Done"},
								Mark:       w.Mark{Colour: "#4A6B52"},
								Title:      []w.Inline{w.Text("Собрать чемодан ")},
								Strong:     true,
								TitleBadge: &w.Badge{Label: "linked ×2", Tone: w.BadgeLinked},
								Tags:       []w.RowTag{w.Tag{Label: "дом"}},
								Due:        "12 Aug 18:00",
								Progress:   "—",
								Source:     "2 sources",
								SourceHint: "nextcloud · Дела, radicale · vps · Личное",
								On:         true,
								Href:       "#",
							},
							w.DueRow{
								Tick:     w.Tick{Label: "Done"},
								Mark:     w.Mark{Colour: "#3F6E8C"},
								Title:    []w.Inline{w.Text("Дописать раздел про кэш")},
								Tags:     []w.RowTag{w.Tag{Label: "релиз"}},
								Due:      "14 Aug",
								Progress: "70%",
								Source:   "nextcloud · Работа",
								Href:     "#",
							},
						},
					},
					w.ListGroup{Title: w.Rubric{Text: "No due date", Form: w.RubricInline}, Count: &w.Num{Value: 4}},
					w.List{
						Rows: []w.ListRow{
							w.DueRow{
								Tick:     w.Tick{Done: true, Label: "Done"},
								Mark:     w.Mark{Colour: "#4A6B52"},
								Title:    []w.Inline{w.Text("Продлить страховку")},
								Due:      "—",
								Progress: "100%",
								Source:   "nextcloud · Дела",
								Href:     "#",
							},
						},
					},
				},
			},
			Side: &w.Side{
				Items: []w.SidePart{
					w.Head{
						Title: &w.HeadTitle{Text: "Собрать чемодан", Size: w.HeadTitle19},
						Subs:  []w.HeadSub{w.HeadSub{Parts: []w.HeadSubPart{w.Text("nextcloud · Дела")}}},
						Acts: &w.Acts{
							Items: []w.ActsPart{
								w.Button{Label: "Edit", Small: true},
								w.Button{Icon: "dots", Small: true, Title: "More"},
								w.Close{Label: "Close the panel"},
							},
						},
					},
					w.Fields{
						Items: []w.FieldsItem{
							{Term: "Status", Value: []w.ValuePart{w.Text("Needs action")}},
							{Term: "Due", Value: []w.ValuePart{w.Text("2026-08-12 18:00")}, Face: w.ValueMono},
							{Term: "Priority", Value: []w.ValuePart{w.Text("5")}, Face: w.ValueMono},
							{Term: "Complete", Value: []w.ValuePart{w.Text("0%")}, Face: w.ValueMono},
							{Term: "Tags", Value: []w.ValuePart{w.Tag{Label: "дом"}}},
						},
					},
					w.Sec{
						Items: []w.SecPart{
							w.Rubric{Text: "Description"},
							w.Prose{Parts: []w.Inline{w.Text("Зарядки, переходник, распечатать посадочный.")}, Variant: w.Prose13},
						},
					},
					w.Sec{
						Items: []w.SecPart{
							w.Rubric{Text: "Related"},
							w.SecList{
								Items: [][]w.SecItemPart{
									[]w.SecItemPart{
										w.Span{Face: w.FaceAccent, Text: "Поездка в Казань"},
										w.Text(" "),
										w.Dim{Parts: []w.Inline{w.Text("· event")}},
									},
								},
							},
						},
					},
					w.Sec{
						Items: []w.SecPart{
							w.Rubric{
								Text: "Attachments",
								Items: []w.RubricPart{
									w.Drop{
										Trigger: w.Button{Label: "Attach", Menu: true, Small: true},
										Menu: w.Menu{
											Items: []w.MenuPart{
												w.MenuItem{Icon: "up", Label: "From this computer"},
												w.MenuItem{Icon: "file", Label: "From Files"},
											},
											Width: w.MenuWidth212,
											Pop:   w.MenuUnder,
										},
									},
								},
							},
							w.SecList{
								Items: [][]w.SecItemPart{
									[]w.SecItemPart{
										w.Link{Text: "посадочный-казань.pdf", Href: "#"},
										w.Text(" "),
										w.Dim{Parts: []w.Inline{w.Text("application/pdf · 128 KB")}},
										w.Text(" "),
										w.Badge{Label: "in the task"},
									},
								},
								Gap: 6,
							},
							w.SecHint{
								Text:  "The same block events and notes have: up to 2 MB the file is written into the task itself, above that it goes to the attachments folder and the task keeps a link. Pasting and dropping work the same way.",
								Apart: true,
							},
						},
					},
					w.Danger{Button: w.Button{Label: "Delete task", Kind: w.ButtonDangerSolid}},
				},
			},
		},
	}
}
