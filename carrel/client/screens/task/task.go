// Drafted by frame2go from frame «Задача · чтение» (s-tasks-2) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// Tasks2 is frame «Задача · чтение» (s-tasks-2).
func Tasks2() interface{ Render() string } {
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
				w.Nav{Items: []w.NavItem{
					{Icon: "back", Label: "Back to list", Short: "Back", Href: "#"}}},
				w.RailSec{
					Items: []w.RailSecPart{
						w.Rubric{Text: "nextcloud"},
						w.TickRow{Mark: w.Mark{Colour: "#4A6B52"}, Label: "Дела", Meta: "21", Current: true},
						w.TickRow{Mark: w.Mark{Colour: "#3F6E8C"}, Label: "Работа", Meta: "9"},
					},
				},
				w.RailFoot{
					Items: []w.RailFootPart{w.Emblem{Symbol: "davloom", Title: "davloom · running · 3 devices"}},
					About: &w.About{Name: "Carrel", Build: w.Mono{Text: "0.0.1 · 786949a"}, Href: "#rest"},
				},
			},
			Main: w.Main{
				Items: []w.MainPart{
					w.Head{
						Title: &w.HeadTitle{Text: "Собрать чемодан"},
						Subs: []w.HeadSub{
							w.HeadSub{
								Parts: []w.HeadSubPart{w.Text("nextcloud · Дела · "), w.Span{Face: w.FaceDanger, Text: "overdue by 3 days"}},
							},
						},
						Acts: &w.Acts{
							Items: []w.ActsPart{
								w.Button{Label: "Edit", Icon: "pencil", Kind: w.ButtonPrimary},
								w.Button{Label: "Mark done", Tick: &w.Tick{Glyph: true}},
								w.Button{Label: "Print", Icon: "print"},
								w.Button{Icon: "dots", Title: "More"},
							},
						},
					},
					w.Fields{
						Items: []w.FieldsItem{
							{Term: "Status", Value: []w.ValuePart{w.Text("Needs action")}},
							{
								Term: "Due",
								Value: []w.ValuePart{
									w.Mono{Text: "2026-08-12 18:00"},
									w.Text(" "),
									w.Span{Face: w.FaceDanger, Text: "· overdue by 3 days"},
								},
							},
							{Term: "Complete", Value: []w.ValuePart{w.Text("0%")}, Face: w.ValueMono},
							{Term: "Priority", Value: []w.ValuePart{w.Text("5")}, Face: w.ValueMono},
							{Term: "Reminders", Value: []w.ValuePart{w.Text("1 day before")}},
							{Term: "Tags", Value: []w.ValuePart{w.Tag{Label: "дом"}}},
						},
					},
					w.Sec{
						Items: []w.SecPart{
							w.Rubric{Text: "Description"},
							w.Prose{Parts: []w.Inline{w.Text("Зарядки, переходник, распечатать посадочный.")}},
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
										w.Dim{Parts: []w.Inline{w.Text("· event · 12 Aug")}},
									},
								},
							},
						},
					},
					w.Sec{
						Items: []w.SecPart{
							w.Rubric{
								Text:  "The same task in other lists",
								Items: []w.RubricPart{w.Badge{Label: "1", Tone: w.BadgeLinked}},
							},
							w.SecList{
								Items: [][]w.SecItemPart{
									[]w.SecItemPart{
										w.Mark{Colour: "#6B4E7A", Size: w.MarkH11Inline},
										w.Text(" "),
										w.Link{Text: "radicale · vps · Личное", Href: "#"},
										w.Text(" "),
										w.Dim{Parts: []w.Inline{w.Text("· Time differ")}},
									},
								},
								Gap: 5,
							},
						},
					},
					w.Sec{
						Items: []w.SecPart{
							w.Rubric{Text: "Source"},
							w.Diag{Text: "Path  /dav.php/calendars/mix/tasks/c41a.ics\nRead  2 min ago · not changed on server"},
						},
					},
					w.Danger{Button: w.Button{Label: "Delete task", Icon: "trash", Kind: w.ButtonDangerSolid}},
				},
			},
		},
	}
}
