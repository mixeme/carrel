// Drafted by frame2go from frame «Заметка · правка» (s-notes-20) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// Notes20 is frame «Заметка · правка» (s-notes-20).
func Notes20() interface{ Render() string } {
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
						w.TickRow{Mark: w.Mark{Colour: "#4A6B52"}, Label: "Записи", Meta: "2", Current: true},
						w.TickRow{Mark: w.Mark{Colour: "#6B4E7A"}, Label: "Дневник", Meta: "1"},
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
					w.Docbar{
						Items: []w.DocbarPart{
							w.Button{Label: "Save", Kind: w.ButtonPrimary, Form: "note-edit"},
							w.Button{Label: "Cancel", Href: "/app/notes/nextcloud/zapisi/9e31-40b7-c012"},
							w.BarSep{},
							w.Button{Label: "B", Bold: true},
							w.Button{Label: "I", Variant: w.ButtonItalic},
							w.Button{Label: "H2", Tertiary: true},
							w.Button{Label: "• list", Tertiary: true},
							w.Button{Folds: true, Label: "task", Icon: "task", Title: "Task", Tertiary: true},
							w.Button{Icon: "link", Title: "Link", Tertiary: true},
							w.Drop{
								Trigger: w.Button{Folds: true, Label: "Attach", Menu: true, Icon: "up", Title: "Attach"},
								Menu: w.Menu{
									Items: []w.MenuPart{
										w.MenuItem{
											Icon: "up",
											File: &w.FileField{Form: "note-edit", Name: "file", Label: "From this computer", List: "note-attachments"},
											Row: w.DocAttach{
												Name:   "{name}",
												Size:   w.Dim{Parts: []w.Inline{w.Text("· {size}")}},
												Remove: &w.Button{Label: "Remove", Kind: w.ButtonDanger, Small: true, Removes: true},
											},
											Label: "From this computer",
										},
										w.MenuItem{Icon: "file", Label: "From Files"},
									},
									Width: w.MenuWidth212,
									Pop:   w.MenuUnder,
								},
							},
						},
						More: &w.Button{Icon: "dots", Title: "More"},
						End: []w.DocbarEnd{
							w.Check{Parts: []w.CheckPart{w.Text("Auto-save")}, Tertiary: true},
							w.DraftNote{Text: "Draft in this window ·", Time: w.Mono{Text: "11:42"}, Secondary: true},
							w.Kbd{Keys: "Ctrl+S", Secondary: true},
							w.Seg{Options: []w.SegOption{{Label: "Full width", On: true}, {Label: "Reading width"}}, Secondary: true},
							w.Button{Folds: true, Label: "Focus", Icon: "focus", Title: "Focus"},
						},
						Edit: true,
					},
					w.Runbar{
						Meter:    w.Meter{Percent: 0},
						Progress: "note-edit",
						Parts:    []w.RunbarPart{w.Text("Saving · "), w.Mono{Text: "0 of 0 MB"}},
					},
					w.DocLayout{
						Items: []w.DocLayoutPart{
							w.Doc{
								Items: []w.DocPart{
									w.Area{Text: "Заметка со встречи 10.08", Variant: w.AreaTitle, Name: "summary", Label: "Text"},
									w.DocFields{
										Items: []w.Field{
											w.Field{
												Label: "Notebook",
												Parts: []w.FieldPart{
													w.Drop{
														Trigger: w.Pick{Value: "nextcloud · Записи", Mark: &w.Mark{Colour: "#4A6B52"}},
														Menu: w.Menu{
															Items: []w.MenuPart{
																w.MenuItem{
																	Mark:  &w.Mark{Colour: "#4A6B52"},
																	Label: "nextcloud · Записи",
																	On:    true,
																	Name:  "notebook",
																	Value: "nextcloud/zapisi",
																},
																w.MenuItem{
																	Mark:  &w.Mark{Colour: "#6B4E7A"},
																	Label: "nextcloud · Дневник",
																	Name:  "notebook",
																	Value: "nextcloud/dnevnik",
																},
																w.MenuItem{
																	Mark:   &w.Mark{Colour: "#3F6E8C"},
																	Trail:  w.Badge{Label: "ro", Place: w.Pushed},
																	Label:  "nextcloud · Общие с семьёй",
																	Reason: "Read-only: this account cannot write here",
																	Name:   "notebook",
																	Value:  "nextcloud/semya",
																},
															},
															Width: w.MenuWidth290,
															Pop:   w.MenuUnder,
														},
													},
												},
												Width: w.FieldFlex2_280,
											},
											w.Field{
												Label: "Date",
												Parts: []w.FieldPart{w.Input{Kind: w.InputDate, Name: "dtstart.date", Value: "2026-08-10", Label: "Date", Mono: true}},
												Width: w.FieldFix150,
											},
											w.Field{
												Label: "Time",
												Parts: []w.FieldPart{w.Input{Kind: w.InputTime, Name: "dtstart.time", Value: "11:05", Label: "Time", Mono: true}},
												Width: w.FieldFix130,
											},
											w.Field{
												Label: "Tags",
												Parts: []w.FieldPart{w.Input{Name: "categories", Value: "встречи, релиз", Label: "Tags"}},
												Width: w.FieldFlex3_320,
											},
											w.Field{
												Label: "Related to",
												Parts: []w.FieldPart{
													w.TagInput{
														Parts: []w.TagInputPart{w.Tag{Label: "Синк-ап по релизу", Removable: true}},
														Add:   &w.TagAdd{Name: "related.add", Placeholder: "Title or UID", Label: "Add a link"},
													},
												},
												Width: w.FieldFlex3_340,
											},
										},
									},
									w.Area{
										Text:    "Договорились заморозить ветку в среду.\n\nПисьмо пользователям пишет Анна, черновик к вечеру вторника. Виктор проверяет миграцию на копии базы — если не успевает, переносим на следующий спринт.\n\n## Что делаем\n\n- [ ] проверить changelog\n- [x] попросить у Артёма скриншоты\n\nПеренос старых вложений в новую папку — Виктор посмотрит, сколько их вообще, и скажет в четверг.",
										Variant: w.AreaBody,
										Focus:   true,
										Name:    "description",
										Label:   "Text",
									},
									w.DocNote{
										Parts: []w.Inline{
											w.Text("Properties Carrel does not know — "),
											w.Mono{Text: "X-JTX-COLOR"},
											w.Text(" and two more — are kept unchanged and not shown here."),
										},
									},
								},
								ID:     "note-edit",
								Action: "/app/notes/nextcloud/zapisi/9e31-40b7-c012",
							},
							w.Docmeta{
								Sections: []w.DocmetaSec{
									{
										Title: w.Rubric{
											Text: "Attachments",
											Items: []w.RubricPart{
												w.Drop{
													Trigger: w.Button{Label: "Attach", Menu: true, Small: true},
													Menu: w.Menu{
														Items: []w.MenuPart{
															w.MenuItem{
																Icon: "up",
																File: &w.FileField{Form: "note-edit", Name: "file", Label: "From this computer", List: "note-attachments"},
																Row: w.DocAttach{
																	Name:   "{name}",
																	Size:   w.Dim{Parts: []w.Inline{w.Text("· {size}")}},
																	Remove: &w.Button{Label: "Remove", Kind: w.ButtonDanger, Small: true, Removes: true},
																},
																Label: "From this computer",
															},
															w.MenuItem{Icon: "file", Label: "From Files"},
														},
														Width: w.MenuWidth212,
														Pop:   w.MenuUnder,
													},
												},
											},
										},
										Items: []w.DocmetaPart{
											w.DocAttach{
												Name:   "2026-08-10-sync-board.png",
												Size:   w.Dim{Parts: []w.Inline{w.Text("· 412 KB")}},
												Where:  w.Badge{Label: "in the note"},
												Remove: &w.Button{Label: "Remove", Kind: w.ButtonDanger, Small: true},
												Href:   "#",
											},
											w.DocAttach{
												Name:   "запись-встречи.m4a",
												Size:   w.Dim{Parts: []w.Inline{w.Text("· 24 MB")}},
												Where:  w.Badge{Label: "Documents / carrel"},
												Remove: &w.Button{Label: "Remove", Kind: w.ButtonDanger, Small: true},
												Href:   "#",
											},
										},
										ID: "note-attachments",
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
			},
		},
	}
}
