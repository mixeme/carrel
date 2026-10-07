// Drafted by frame2go from frame «Заметка · чтение на весь экран» (s-notes-2) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// Notes2 is frame «Заметка · чтение на весь экран» (s-notes-2).
func Notes2() interface{ Render() string } {
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
						w.TickRow{Mark: w.Mark{Colour: "#4A6B52"}, Label: "Записи", Meta: "64", Current: true},
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
						w.Rubric{Text: "Neighbours"},
						w.Tree{
							Rows: []w.TreeRow{
								{Icon: "note", Label: "Планы на осень"},
								{Icon: "note", Label: "Заметка со встречи"},
								{Icon: "note", Label: "Что почитать про CRDT"},
							},
							On:      2,
							Variant: w.TreePadded,
						},
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
						Meta: &w.HeadSub{
							Parts: []w.HeadSubPart{
								w.Mark{Colour: "#4A6B52", Size: w.MarkH12},
								w.Text("nextcloud · Записи · "),
								w.Mono{Text: "10 August 2026, 11:05"},
							},
						},
						Acts: &w.Acts{
							Items: []w.ActsPart{
								w.Button{Label: "Edit", Icon: "pencil", Kind: w.ButtonPrimary},
								w.Button{Label: "Export Markdown"},
								w.Button{Label: "Print", Icon: "print"},
								w.Button{Label: "Source", Href: "/app/notes/nextcloud/zapisi/9e31-40b7-c012?view=source", Swap: "note-main"},
								w.Seg{
									Options: []w.SegOption{
										{Label: "Full width", Href: "/app/notes/nextcloud/zapisi/9e31-40b7-c012?width=full", On: true},
										{Label: "Reading width", Href: "/app/notes/nextcloud/zapisi/9e31-40b7-c012?width=read"},
									},
									Swap: "note-main",
								},
								w.Button{
									Icon:  "panel",
									Title: "Hide the side column",
									Href:  "/app/notes/nextcloud/zapisi/9e31-40b7-c012?side=0",
									Swap:  "note-main",
								},
								w.Drop{
									Trigger: w.Button{Icon: "dots", Title: "More"},
									Menu: w.Menu{
										Items: []w.MenuPart{
											w.MenuItem{Icon: "move", Label: "Move", Href: "/app/notes/nextcloud/zapisi/9e31-40b7-c012/move"},
											w.MenuItem{Icon: "dup", Label: "Copy", Href: "/app/notes/nextcloud/zapisi/9e31-40b7-c012/copy"},
											w.MenuItem{Icon: "link", Label: "Publish a link", Href: "/app/notes/nextcloud/zapisi/9e31-40b7-c012/publish"},
										},
										Width: w.MenuWidth212,
										Pop:   w.MenuUnderEnd,
									},
								},
							},
						},
						Pad: w.HeadRule,
					},
					w.DocLayout{
						Items: []w.DocLayoutPart{
							w.Doc{
								Items: []w.DocPart{
									w.DocTitle{Text: "Заметка со встречи 10.08"},
									w.Prose{
										Parts: []w.Inline{
											w.Text("Договорились заморозить ветку в среду.\n\nПисьмо пользователям пишет Анна, черновик к вечеру вторника. Виктор проверяет миграцию на копии базы — если не успевает, переносим на следующий спринт."),
										},
										Variant: w.ProseLead,
									},
									w.DocTasks{
										Title: w.Rubric{Text: "Что делаем", Variant: w.RubricBottom8},
										Items: []w.DocTask{
											{Tick: w.Tick{Variant: w.TickNudge, Sign: true}, Text: "проверить changelog"},
											{Tick: w.Tick{Done: true, Variant: w.TickNudge, Sign: true}, Text: "попросить у Артёма скриншоты"},
										},
									},
									w.Prose{
										Parts: []w.Inline{
											w.Text("Перенос старых вложений в новую папку — Виктор посмотрит, сколько их вообще, и скажет в четверг."),
										},
										Variant: w.ProseLeadAfter,
									},
								},
							},
							w.Docmeta{
								Sections: []w.DocmetaSec{
									{
										Title: w.Rubric{Text: "Tags"},
										Items: []w.DocmetaPart{
											w.Line{Block: true, Gap: w.LineGap4, Items: []w.LinePart{w.Tag{Label: "встречи"}, w.Tag{Label: "релиз"}}},
										},
									},
									{
										Title: w.Rubric{Text: "Related"},
										Items: []w.DocmetaPart{
											w.DocRefs{
												Items: []w.DocRef{
													{Name: "Синк-ап по релизу", What: w.Dim{Parts: []w.Inline{w.Text("· event · 10 Aug")}}},
													{Name: "Дописать раздел про кэш", What: w.Dim{Parts: []w.Inline{w.Text("· task")}}},
												},
											},
										},
									},
									{
										Title: w.Rubric{
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
										Items: []w.DocmetaPart{
											w.DocAttach{
												Name:  "2026-08-10-sync-board.png",
												Size:  w.Dim{Parts: []w.Inline{w.Text("· 412 KB")}},
												Where: w.Badge{Label: "in the note"},
												Href:  "#",
											},
											w.DocAttach{
												Name:  "запись-встречи.m4a",
												Size:  w.Dim{Parts: []w.Inline{w.Text("· 24 MB")}},
												Where: w.Badge{Label: "Documents / carrel"},
												Href:  "#",
											},
										},
									},
									{
										Title: w.Rubric{
											Text:  "The same note in other notebooks",
											Items: []w.RubricPart{w.Badge{Label: "1", Tone: w.BadgeLinked}},
										},
										Items: []w.DocmetaPart{
											w.DocRefs{
												Items: []w.DocRef{
													{
														Mark: &w.Mark{Colour: "#4A6B52", Size: w.MarkH11Inline},
														Name: "nextcloud · Дневник",
														Href: "#",
														What: w.Dim{Parts: []w.Inline{w.Text("· Text differ")}},
													},
												},
											},
										},
									},
									{
										Title: w.Rubric{Text: "Source"},
										Items: []w.DocmetaPart{w.Diag{Text: "UID   9e31-40b7-c012\nRead  2 min ago"}},
									},
								},
								Danger: &w.Danger{Button: w.Button{Label: "Delete note", Kind: w.ButtonDangerSolid}, Space: w.DangerAtEnd},
							},
						},
					},
				},
				ID: "note-main",
			},
		},
	}
}
