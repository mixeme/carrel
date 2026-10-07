// Drafted by frame2go from frame «Заметки · список и заметка в панели» (s-notes-1) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// Notes1 is frame «Заметки · список и заметка в панели» (s-notes-1).
func Notes1() interface{ Render() string } {
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
					On: 4,
				},
				w.RailSec{
					Items: []w.RailSecPart{
						w.TickRow{
							Ticked:  w.TickedPart,
							Kind:    w.TickRowAll,
							Mark:    w.Mark{Kind: w.MarkMulti},
							Label:   "Combined notebook",
							Meta:    "86",
							Current: true,
						},
					},
				},
				w.RailSec{
					Items: []w.RailSecPart{
						w.Rubric{Text: "nextcloud"},
						w.TickRow{Mark: w.Mark{Colour: "#4A6B52"}, Label: "Записи", Meta: "64"},
						w.TickRow{Mark: w.Mark{Colour: "#6B4E7A"}, Label: "Дневник", Meta: "22"},
						w.TickRow{
							Ticked: w.TickedOff,
							Mark:   w.Mark{Colour: "#3F6E8C"},
							Label:  "Общие с семьёй",
							Meta:   "—",
							Hidden: true,
							Trail:  w.Badge{Label: "ro"},
						},
					},
				},
				w.RailSec{
					Items: []w.RailSecPart{
						w.Rubric{Text: "Tags"}},
					Tags: []w.Tag{
						w.Tag{Label: "all", On: true},
						w.Tag{Label: "встречи"},
						w.Tag{Label: "идеи"},
						w.Tag{Label: "книги"},
						w.Tag{Label: "рецепты"},
					},
				},
				w.RailFoot{
					Items: []w.RailFootPart{
						w.Button{Icon: "plus", Title: "New notebook"},
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
						Title: &w.HeadTitle{Mark: &w.Mark{Kind: w.MarkMulti, Size: w.MarkH20}, Text: "Combined notebook"},
						Subs:  []w.HeadSub{w.HeadSub{Parts: []w.HeadSubPart{w.Text("2 of 3 notebooks · 86 notes · VJOURNAL")}}},
						Acts: &w.Acts{
							Items: []w.ActsPart{
								w.Button{Label: "New note", Menu: true, Icon: "plus", Kind: w.ButtonPrimary},
								w.Button{Label: "Import", Icon: "up"},
								w.Button{Label: "Export", Icon: "down"},
							},
						},
					},
					w.Bar{
						Items: []w.BarPart{
							w.Filter{
								Query:   w.Query{Placeholder: "Filter by word", Label: "Filter by word"},
								Waiting: true,
								Variant: w.FilterH26W200,
							},
							w.Seg{Options: []w.SegOption{{Label: "Newest", On: true}, {Label: "Oldest"}, {Label: "Title"}}},
							w.Button{Label: "Columns", Icon: "cols"},
						},
						Right: &w.BarRight{Items: []w.BarPart{w.Mono{Text: "86"}}, Secondary: true},
					},
					w.ListGroup{Title: w.Rubric{Text: "August 2026", Form: w.RubricInline}, Count: &w.Num{Value: 9}},
					w.List{
						Rows: []w.ListRow{
							w.ExcerptRow{
								Mark:    w.Mark{Colour: "#4A6B52"},
								Title:   []w.Inline{w.Text("Заметка со встречи 10.08")},
								Strong:  true,
								Excerpt: "Договорились заморозить ветку в среду; письмо пользователям пишет Анна…",
								Tags:    []w.RowTag{w.Tag{Label: "встречи"}},
								Date:    "10 Aug",
								Source:  "nextcloud · Записи",
								On:      true,
							},
							w.ExcerptRow{
								Mark:       w.Mark{Colour: "#4A6B52"},
								Title:      []w.Inline{w.Text("Что почитать про CRDT ")},
								TitleBadge: &w.Badge{Label: "linked ×2", Tone: w.BadgeLinked},
								Excerpt:    "Кляйн, глава 5; статья Шапиро 2011; демка automerge…",
								Tags:       []w.RowTag{w.Tag{Label: "книги"}},
								Date:       "8 Aug",
								Source:     "2 sources",
								SourceHint: "nextcloud · Записи, radicale · vps · Дневник",
							},
							w.ExcerptRow{
								Mark:    w.Mark{Colour: "#6B4E7A"},
								Title:   []w.Inline{w.Text("Про дорогу до дачи")},
								Excerpt: "Съезд после заправки, потом грунтовка два километра…",
								Tags:    []w.RowTag{w.Badge{Label: "linked", Tone: w.BadgeLinked, Icon: "link"}},
								Date:    "3 Aug",
								Source:  "nextcloud · Дневник",
							},
						},
					},
				},
			},
			Side: &w.Side{
				Items: []w.SidePart{
					w.Head{
						Title: &w.HeadTitle{Text: "Заметка со встречи 10.08", Size: w.HeadTitle19},
						Tools: []w.HeadTool{
							w.Button{Label: "Edit", Small: true},
							w.Button{Icon: "expand", Small: true, Title: "Open the note page"},
							w.Button{Icon: "dots", Small: true, Title: "More"},
						},
						Subs: []w.HeadSub{
							w.HeadSub{Parts: []w.HeadSubPart{w.Text("nextcloud · Записи · "), w.Mono{Text: "10 August 2026, 11:05"}}},
						},
						Acts:   &w.Acts{Items: []w.ActsPart{w.Close{Label: "Close the panel"}}},
						Middle: true,
					},
					w.Sec{
						Items: []w.SecPart{
							w.Prose{
								Parts: []w.Inline{
									w.Text("Договорились заморозить ветку в среду.\n\nПисьмо пользователям пишет Анна, черновик к вечеру вторника. Виктор проверяет миграцию на копии базы — если не успевает, переносим на следующий спринт."),
								},
								Variant: w.ProseRead,
							},
							w.SecTasks{
								Items: []w.DocTask{
									{Tick: w.Tick{Variant: w.TickNudge, Sign: true}, Text: "проверить changelog"},
									{Tick: w.Tick{Done: true, Variant: w.TickNudge, Sign: true}, Text: "попросить у Артёма скриншоты"},
								},
							},
						},
					},
					w.Fields{
						Items: []w.FieldsItem{
							{Term: "Tags", Value: []w.ValuePart{w.Tag{Label: "встречи"}, w.Text(" "), w.Tag{Label: "релиз"}}}},
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
										w.Link{Text: "2026-08-10-sync-board.png", Href: "#"},
										w.Text(" "),
										w.Dim{Parts: []w.Inline{w.Text("image/png · 412 KB")}},
										w.Text(" "),
										w.Badge{Label: "in the note"},
									},
									[]w.SecItemPart{
										w.Link{Text: "запись-встречи.m4a", Href: "#"},
										w.Text(" "),
										w.Dim{Parts: []w.Inline{w.Text("audio/mp4 · 24 MB")}},
										w.Text(" "),
										w.Badge{Label: "Documents / carrel"},
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
										w.Span{Face: w.FaceAccent, Text: "Синк-ап по релизу"},
										w.Text(" "),
										w.Dim{Parts: []w.Inline{w.Text("· event · 10 Aug")}},
									},
									[]w.SecItemPart{
										w.Span{Face: w.FaceAccent, Text: "Дописать раздел про кэш"},
										w.Text(" "),
										w.Dim{Parts: []w.Inline{w.Text("· task")}},
									},
								},
								Gap: 6,
							},
						},
					},
				},
			},
		},
	}
}
