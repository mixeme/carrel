// Drafted by frame2go from frame «Заметка · новая» (s-notes-30) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// Notes30 is frame «Заметка · новая» (s-notes-30).
func Notes30() interface{ Render() string } {
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
				w.RailFoot{
					Items: []w.RailFootPart{w.Emblem{Symbol: "davloom", Title: "davloom · running · 3 devices"}},
					About: &w.About{Name: "Carrel", Build: w.Mono{Text: "0.0.1 · 786949a"}, Href: "#rest"},
				},
			},
			Main: w.Main{
				Items: []w.MainPart{
					w.Docbar{
						Items: []w.DocbarPart{
							w.Button{Label: "Save", Kind: w.ButtonPrimary, Form: "note-new"},
							w.Button{Label: "Cancel", Href: "/app/notes/nextcloud/zapisi"},
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
											File: &w.FileField{Form: "note-new", Name: "file", Label: "From this computer", List: "note-new-attachments"},
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
							w.Kbd{Keys: "Ctrl+S", Secondary: true},
							w.Seg{Options: []w.SegOption{{Label: "Full width", On: true}, {Label: "Reading width"}}, Secondary: true},
							w.Button{Folds: true, Label: "Focus", Icon: "focus", Title: "Focus"},
						},
						Edit: true,
					},
					w.Runbar{
						Meter:    w.Meter{Percent: 0},
						Progress: "note-new",
						Parts:    []w.RunbarPart{w.Text("Saving · "), w.Mono{Text: "0 of 0 MB"}},
					},
					w.DocLayout{
						Items: []w.DocLayoutPart{
							w.Doc{
								Items: []w.DocPart{
									w.Area{Text: "", Variant: w.AreaTitle, Focus: true, Name: "summary", Label: "Text"},
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
												Parts: []w.FieldPart{w.Input{Name: "categories", Placeholder: "—", Label: "Tags"}},
												Width: w.FieldFlex3_320,
											},
											w.Field{
												Label: "Related to",
												Parts: []w.FieldPart{w.TagInput{Parts: []w.TagInputPart{}}},
												Width: w.FieldFlex3_340,
											},
										},
									},
									w.Area{Text: "", Variant: w.AreaBody, Name: "description", Label: "Text"},
								},
								ID:     "note-new",
								Action: "/app/notes/nextcloud/zapisi/new",
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
																File: &w.FileField{Form: "note-new", Name: "file", Label: "From this computer", List: "note-new-attachments"},
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
										Items: []w.DocmetaPart{},
										ID:    "note-new-attachments",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
