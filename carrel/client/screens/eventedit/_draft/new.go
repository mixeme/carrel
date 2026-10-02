// Drafted by frame2go from frame «Событие · новое» (s-agenda-15) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// Agenda15 is frame «Событие · новое» (s-agenda-15).
func Agenda15() interface{ Render() string } {
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
						w.TickRow{Mark: w.Mark{Colour: "#4A6B52"}, Label: "Личный", Meta: "7", Current: true},
						w.TickRow{Mark: w.Mark{Colour: "#3F6E8C"}, Label: "Работа", Meta: "4"},
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
						Title: &w.HeadTitle{Text: "New event"},
						Subs:  []w.HeadSub{w.HeadSub{Parts: []w.HeadSubPart{w.Text("nextcloud · Личный")}}},
						Acts:  &w.Acts{Items: []w.ActsPart{w.Button{Label: "Cancel"}, w.Button{Label: "Save", Kind: w.ButtonPrimary}}},
					},
					w.Runbar{
						Meter:    w.Meter{Percent: 0},
						Progress: "event-new",
						Parts:    []w.RunbarPart{w.Text("Saving · "), w.Mono{Text: "0 of 0 MB"}},
					},
					w.Form{
						Items: []w.FormPart{
							w.Field{
								Label: "Summary",
								Parts: []w.FieldPart{w.Input{Placeholder: "—", Label: "Summary", Focus: true}},
								Width: w.FieldXL,
							},
							w.Field{
								Label: "Location",
								Parts: []w.FieldPart{w.Input{Placeholder: "—", Label: "Location"}},
								Width: w.FieldLG,
							},
							w.Field{Label: "Status", Parts: []w.FieldPart{w.Pick{Value: "No status"}}, Width: w.FieldSM},
							w.Field{Label: "Tags", Parts: []w.FieldPart{w.Input{Placeholder: "—", Label: "Tags"}}, Width: w.FieldMD},
							w.Field{Label: "Description", Parts: []w.FieldPart{w.Area{Text: "", Label: "Text"}}, Width: w.FieldFull},
							w.FieldSet{
								Rubric: w.Rubric{Text: "When"},
								Items: []w.FieldSetPart{
									w.Field{Parts: []w.FieldPart{w.Check{Parts: []w.CheckPart{w.Text("All day")}}}},
									w.Field{
										Label: "Start date",
										Parts: []w.FieldPart{w.Input{Kind: w.InputDate, Value: "2026-08-10", Label: "Start date", Mono: true}},
										Width: w.FieldSM,
									},
									w.Field{
										Label: "Start time",
										Parts: []w.FieldPart{w.Input{Kind: w.InputTime, Value: "10:00", Label: "Start time", Mono: true}},
										Width: w.FieldXS,
									},
									w.Field{
										Label: "End date",
										Parts: []w.FieldPart{w.Input{Kind: w.InputDate, Value: "2026-08-10", Label: "End date", Mono: true}},
										Width: w.FieldSM,
									},
									w.Field{
										Label: "End time",
										Parts: []w.FieldPart{w.Input{Kind: w.InputTime, Value: "11:00", Label: "End time", Mono: true}},
										Width: w.FieldXS,
									},
									w.Field{
										Label: "Time zone",
										Parts: []w.FieldPart{
											w.Drop{
												Trigger: w.Pick{Value: "Europe/Moscow"},
												Menu: w.Menu{
													Items: []w.MenuPart{
														w.Input{Kind: w.InputSearch, Placeholder: "Find a zone", Label: "Find a zone"},
														w.MenuItem{Label: "Europe/Moscow", On: true, Name: "tz", Value: "Europe/Moscow"},
														w.MenuSep{},
														w.MenuItem{Label: "Asia/Yekaterinburg", Name: "tz", Value: "Asia/Yekaterinburg"},
														w.MenuItem{Label: "UTC", Name: "tz", Value: "UTC"},
														w.MenuSep{},
														w.MenuItem{Label: "Floating — the same clock time anywhere", Name: "tz", Value: "floating"},
													},
													Width: w.MenuWidth290,
													Pop:   w.MenuUnder,
												},
											},
										},
										Width: w.FieldMD,
									},
								},
							},
							w.FieldSet{
								Rubric: w.Rubric{Text: "Reminders"},
								Items: []w.FieldSetPart{
									w.Field{Label: "First", Parts: []w.FieldPart{w.Pick{Value: "10 minutes before"}}, Width: w.FieldMD},
									w.Field{Label: "\u00a0", Parts: []w.FieldPart{w.Button{Label: "Add reminder"}}},
								},
							},
							w.FieldSet{
								Rubric: w.Rubric{Text: "Attendees"},
								Items: []w.FieldSetPart{
									w.Field{
										Parts: []w.FieldPart{
											w.Drop{
												Trigger: w.TagInput{
													Parts:   []w.TagInputPart{},
													Columns: true,
													Add: &w.TagAdd{
														Name:        "attendee.add",
														Placeholder: "Name or address",
														Label:       "Add attendee",
														Suggest:     "/app/contacts/suggest",
													},
												},
												Menu: w.Menu{Items: []w.MenuPart{}, Width: w.MenuWidth420, Pop: w.MenuUnder},
											},
											w.FieldHint{
												Parts: []w.Inline{
													w.Text("A name or an address: the ticked address books are searched. On save, nextcloud sends the invitations and their changes — Carrel sends no mail itself."),
												},
											},
										},
										Width: w.FieldFull,
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
															File: &w.FileField{Form: "", Name: "file", Label: "From this computer", List: "event-attachments"},
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
										ID:    "event-attachments",
										Place: w.Grown,
									},
								},
							},
							w.FieldSet{
								Rubric: w.Rubric{Text: "Repeats"},
								Items: []w.FieldSetPart{
									w.Field{Label: "Frequency", Parts: []w.FieldPart{w.Pick{Value: "Does not repeat"}}, Width: w.FieldSM},
									w.Field{
										Label: "Every",
										Parts: []w.FieldPart{w.Input{Value: "1", Label: "Every", Mono: true}},
										Width: w.FieldXS,
									},
									w.Field{
										Label: "On",
										Parts: []w.FieldPart{
											w.Seg{
												Options: []w.SegOption{
													{Label: "MO", Value: "MO"},
													{Label: "TU", Value: "TU"},
													{Label: "WE", Value: "WE"},
													{Label: "TH", Value: "TH"},
													{Label: "FR", Value: "FR"},
													{Label: "SA", Value: "SA"},
													{Label: "SU", Value: "SU"},
												},
												Name:     "byday",
												Multiple: true,
											},
										},
									},
									w.Field{
										Label: "Until",
										Parts: []w.FieldPart{w.Input{Kind: w.InputDate, Label: "Until", Mono: true}},
										Width: w.FieldSM,
									},
									w.Field{Label: "Count", Parts: []w.FieldPart{w.Input{Placeholder: "—", Label: "Count"}}, Width: w.FieldXS},
									w.Field{
										Label: "Advanced RRULE",
										Parts: []w.FieldPart{
											w.Input{Placeholder: "—", Label: "Advanced RRULE", Mono: true},
											w.FieldHint{Parts: []w.Inline{w.Text("The advanced value takes precedence when set.")}},
										},
										Width: w.FieldXL,
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
