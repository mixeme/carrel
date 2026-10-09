// Drafted by frame2go from frame «Задача · новая» (s-tasks-14) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// Tasks14 is frame «Задача · новая» (s-tasks-14).
func Tasks14() interface{ Render() string } {
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
						w.TickRow{Mark: w.Mark{Colour: "#4A6B52"}, Label: "Дела", Meta: "4", Current: true},
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
						Title: &w.HeadTitle{Mark: &w.Mark{Colour: "#4A6B52", Size: w.MarkH20}, Text: "New task"},
						Subs:  []w.HeadSub{w.HeadSub{Parts: []w.HeadSubPart{w.Text("nextcloud · Дела")}}},
						Acts:  &w.Acts{Items: []w.ActsPart{w.Button{Label: "Cancel"}, w.Button{Label: "Save", Kind: w.ButtonPrimary}}},
					},
					w.Runbar{
						Meter:    w.Meter{Percent: 0},
						Progress: "task-new",
						Parts:    []w.RunbarPart{w.Text("Saving · "), w.Mono{Text: "0 of 0 MB"}},
					},
					w.Form{
						Items: []w.FormPart{
							w.Field{
								Label: "Summary",
								Parts: []w.FieldPart{w.Input{Placeholder: "—", Label: "Summary", Focus: true}},
								Width: w.FieldFull,
							},
							w.Field{Label: "Description", Parts: []w.FieldPart{w.Area{Text: "", Label: "Text"}}, Width: w.FieldFull},
							w.FieldSet{
								Rubric: w.Rubric{Text: "When"},
								Items: []w.FieldSetPart{
									w.Field{Parts: []w.FieldPart{w.Check{Parts: []w.CheckPart{w.Text("All day")}}}},
									w.Field{
										Label: "Start date",
										Parts: []w.FieldPart{w.Input{Kind: w.InputDate, Placeholder: "YYYY-MM-DD", Label: "Start date", Mono: true}},
										Width: w.FieldSM,
									},
									w.Field{
										Label: "Start time",
										Parts: []w.FieldPart{w.Input{Kind: w.InputTime, Placeholder: "--:--", Label: "Start time", Mono: true}},
										Width: w.FieldXS,
									},
									w.Field{
										Label: "Due date",
										Parts: []w.FieldPart{w.Input{Kind: w.InputDate, Placeholder: "YYYY-MM-DD", Label: "Due date", Mono: true}},
										Width: w.FieldSM,
									},
									w.Field{
										Label: "Due time",
										Parts: []w.FieldPart{w.Input{Kind: w.InputTime, Placeholder: "--:--", Label: "Due time", Mono: true}},
										Width: w.FieldXS,
									},
								},
							},
							w.FieldSet{
								Rubric: w.Rubric{Text: "Progress"},
								Items: []w.FieldSetPart{
									w.Field{Label: "Status", Parts: []w.FieldPart{w.Pick{Value: "Needs action"}}, Width: w.FieldSM},
									w.Field{Label: "Priority", Parts: []w.FieldPart{w.Pick{Value: "No priority"}}, Width: w.FieldSM},
									w.Field{
										Label: "Percent complete",
										Parts: []w.FieldPart{w.Input{Value: "0", Label: "Percent complete", Mono: true}},
										Width: w.FieldSM,
									},
									w.Field{Label: "\u00a0", Parts: []w.FieldPart{w.Check{Parts: []w.CheckPart{w.Text("Done")}}}},
								},
							},
							w.FieldSet{
								Rubric: w.Rubric{Text: "Reminders"},
								Items:  []w.FieldSetPart{w.Field{Parts: []w.FieldPart{w.Button{Label: "Add reminder"}}}},
							},
							w.FieldSet{
								Rubric: w.Rubric{Text: "Other"},
								Items: []w.FieldSetPart{
									w.Field{
										Label: "Location",
										Parts: []w.FieldPart{w.Input{Placeholder: "optional", Label: "Location"}},
										Width: w.FieldMD,
									},
									w.Field{Label: "Tags", Parts: []w.FieldPart{w.Input{Placeholder: "—", Label: "Tags"}}, Width: w.FieldMD},
									w.Field{
										Label: "Related to",
										Parts: []w.FieldPart{
											w.TagInput{
												Parts: []w.TagInputPart{},
												Add:   &w.TagAdd{Name: "related.add", Placeholder: "Title or UID", Label: "Add a link"},
											},
										},
										Width: w.FieldLG,
									},
								},
							},
							w.FieldSet{
								Rubric: w.Rubric{Text: "Attachments"},
								Items: []w.FieldSetPart{
									w.Line{
										Items: []w.LinePart{
											w.Drop{
												Trigger: w.Button{Label: "Attach", Menu: true, Icon: "up"},
												Menu: w.Menu{
													Items: []w.MenuPart{
														w.MenuItem{
															Icon: "up",
															File: &w.FileField{Form: "", Name: "file", Label: "From this computer", List: "task-new-attachments"},
															Row: w.Line{
																Items: []w.LinePart{
																	w.Faced{
																		Face:  w.FaceLarge,
																		Items: []w.LinePart{w.Text("{name} "), w.Dim{Parts: []w.Inline{w.Text("{size}")}}},
																	},
																	w.Button{Label: "Remove", Kind: w.ButtonDanger, Small: true, Removes: true},
																},
																Gap: w.LineGap8,
															},
															Label: "From this computer",
														},
														w.MenuItem{Icon: "file", Label: "From Files"},
													},
													Width: w.MenuWidth212,
													Pop:   w.MenuUnder,
												},
											},
											w.Dim{Parts: []w.Inline{w.Text("Nothing attached")}, Size: w.Dim11_5, Variant: w.DimWrap, Empty: true},
										},
										Gap:   w.LineGap8,
										Block: true,
										ID:    "task-new-attachments",
										Place: w.Grown,
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
