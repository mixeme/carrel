// Drafted by frame2go from frame «Событие · чтение» (s-agenda-2) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// Agenda2 is frame «Событие · чтение» (s-agenda-2).
func Agenda2() interface{ Render() string } {
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
						w.TickRow{Ticked: w.TickedOff, Mark: w.Mark{Colour: "#A05A2C"}, Label: "Дни рождения", Meta: "—", Hidden: true},
						w.TickRow{Mark: w.Mark{Colour: "#4A6B52"}, Label: "Личный", Meta: "2", Current: true},
						w.TickRow{Mark: w.Mark{Colour: "#3F6E8C"}, Label: "Работа", Meta: "2"},
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
						Title: &w.HeadTitle{Mark: &w.Mark{Colour: "#4A6B52", Size: w.MarkH19}, Text: "Синк-ап по релизу"},
						Subs:  []w.HeadSub{w.HeadSub{Parts: []w.HeadSubPart{w.Text("nextcloud · Личный")}}},
						Acts: &w.Acts{
							Items: []w.ActsPart{
								w.Button{Label: "Edit", Icon: "pencil", Kind: w.ButtonPrimary},
								w.Button{Label: "New task", Icon: "task"},
								w.Button{Label: "Print", Icon: "print"},
								w.Button{Icon: "dots", Title: "More"},
							},
						},
					},
					w.Fields{
						Items: []w.FieldsItem{
							{
								Term: "When",
								Value: []w.ValuePart{
									w.Mono{Text: "10:00 – 11:00"},
									w.Text(", Monday 10 August "),
									w.Badge{Label: "created 12:00 Asia/Yekaterinburg", Title: "Created in another time zone"},
								},
							},
							{Term: "Repeats", Value: []w.ValuePart{w.Text("Weekly on Mon "), w.Badge{Label: "until 2026-12-28"}}},
							{Term: "Reminders", Value: []w.ValuePart{w.Text("10 minutes before")}},
							{Term: "Where", Value: []w.ValuePart{w.Text("Переговорная 2")}},
							{Term: "Status", Value: []w.ValuePart{w.Text("Confirmed")}},
							{Term: "Tags", Value: []w.ValuePart{w.Tag{Label: "release"}, w.Text(" "), w.Tag{Label: "team"}}},
						},
					},
					w.Sec{
						Items: []w.SecPart{
							w.Rubric{Text: "Description"},
							w.Prose{
								Parts: []w.Inline{w.Text("Обсудить оставшиеся блокеры, дату заморозки и кто пишет письмо пользователям.")},
							},
						},
					},
					w.Sec{
						Items: []w.SecPart{
							w.Rubric{Text: "Attendees", Items: []w.RubricPart{w.Mono{Text: "4", Variant: w.MonoTight}}},
							w.SecList{
								Items: [][]w.SecItemPart{
									[]w.SecItemPart{
										w.Mark{Colour: "#6B4E7A", Size: w.MarkH11Inline},
										w.Text(" "),
										w.Link{Text: "Мария Лебедева", Href: "#"},
										w.Text(" "),
										w.Dim{Parts: []w.Inline{w.Text("· organiser")}},
									},
									[]w.SecItemPart{
										w.Mark{Colour: "#4A6B52", Size: w.MarkH11Inline},
										w.Text(" "),
										w.Link{Text: "Анна Ковалёва", Href: "#"},
										w.Text(" "),
										w.Dim{Parts: []w.Inline{w.Text("· accepted")}},
									},
									[]w.SecItemPart{
										w.Mark{Colour: "#6B4E7A", Size: w.MarkH11Inline},
										w.Text(" "),
										w.Link{Text: "Артём Белов", Href: "#"},
										w.Text(" "),
										w.Dim{Parts: []w.Inline{w.Text("· tentative")}},
									},
									[]w.SecItemPart{w.Mono{Text: "v@example.org"}, w.Text(" "), w.Dim{Parts: []w.Inline{w.Text("· no reply")}}},
								},
								Columns: true,
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
												w.MenuItem{
													Icon:  "up",
													File:  &w.FileField{Form: "post-attach", Name: "file", Label: "From this computer"},
													Label: "From this computer",
												},
												w.MenuItem{Icon: "file", Label: "From Files"},
												w.MenuItem{Label: "Upload", NoScript: true, Form: "post-attach"},
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
										w.Link{Text: "release-checklist.md", Href: "#"},
										w.Text(" "),
										w.Dim{Parts: []w.Inline{w.Text("text/markdown · 3 KB")}},
										w.Text(" "),
										w.Badge{Label: "in the event"},
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
										w.Span{Face: w.FaceAccent, Text: "Заметка со встречи 10.08"},
										w.Text(" "),
										w.Dim{Parts: []w.Inline{w.Text("· note")}},
									},
								},
							},
						},
					},
					w.Sec{
						Items: []w.SecPart{
							w.Rubric{
								Text:  "The same event in other calendars",
								Items: []w.RubricPart{w.Badge{Label: "1", Tone: w.BadgeLinked}},
							},
							w.SecList{
								Items: [][]w.SecItemPart{
									[]w.SecItemPart{
										w.Mark{Colour: "#6B4E7A", Size: w.MarkH11Inline},
										w.Text(" "),
										w.Link{Text: "radicale · vps · Проекты", Href: "#"},
										w.Text(" "),
										w.Dim{Parts: []w.Inline{w.Text("· Holds the same")}},
									},
								},
								Gap: 5,
							},
						},
					},
					w.Sec{
						Items: []w.SecPart{
							w.Rubric{Text: "Source"},
							w.Diag{
								Text: "Path  /dav.php/calendars/mix/personal/8f1c.ics\nRead  2 min ago · changed on server 3 days ago",
							},
						},
					},
					w.Danger{
						Button: w.Button{Label: "Delete event", Icon: "trash", Kind: w.ButtonDangerSolid},
						Note:   "Series is edited and deleted whole.",
					},
				},
			},
		},
	}
}
