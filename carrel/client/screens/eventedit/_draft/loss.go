// Drafted by frame2go from frame «Событие · сервер не сохранил свойства» (states-17) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// States17 is frame «Событие · сервер не сохранил свойства» (states-17).
func States17() interface{ Render() string } {
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
					{Icon: "back", Label: "Back to the card", Href: "#"}}},
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
						Title: &w.HeadTitle{Text: "Синк-ап по релизу"},
						Subs:  []w.HeadSub{w.HeadSub{Parts: []w.HeadSubPart{w.Text("nextcloud · Личный")}}},
						Acts:  &w.Acts{Items: []w.ActsPart{w.Button{Label: "Cancel"}, w.Button{Label: "Save", Kind: w.ButtonPrimary}}},
					},
					w.Msg{
						Parts: []w.MsgPart{
							w.Strong("The change was saved, but the server did not keep these properties."),
							w.Text(" Other clients will not see them either: "),
							w.Mono{Text: "X-CALENDARSERVER-PRIVATE-COMMENT"},
							w.Text(", "),
							w.Mono{Text: "X-MICROSOFT-CDO-BUSYSTATUS"},
							w.Text("."),
						},
						Space: w.MsgFlushTop2,
					},
					w.Form{
						Items: []w.FormPart{
							w.Field{
								Label: "Summary",
								Parts: []w.FieldPart{w.Input{Value: "Синк-ап по релизу", Label: "Summary", Focus: true}},
								Width: w.FieldXL,
							},
							w.Field{
								Label: "Location",
								Parts: []w.FieldPart{w.Input{Value: "Переговорная 2", Label: "Location"}},
								Width: w.FieldLG,
							},
							w.Field{Label: "Status", Parts: []w.FieldPart{w.Pick{Value: "Confirmed"}}, Width: w.FieldSM},
							w.Field{
								Label: "Tags",
								Parts: []w.FieldPart{w.Input{Value: "release, team", Label: "Tags"}},
								Width: w.FieldMD,
							},
							w.Field{
								Label: "Description",
								Parts: []w.FieldPart{
									w.Area{Text: "Обсудить оставшиеся блокеры, дату заморозки и кто пишет письмо пользователям.", Label: "Text"},
								},
								Width: w.FieldFull,
							},
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
										Parts: []w.FieldPart{w.Input{Kind: w.InputTime, Value: "12:00", Label: "Start time", Mono: true}},
										Width: w.FieldXS,
									},
									w.Field{
										Label: "End date",
										Parts: []w.FieldPart{w.Input{Kind: w.InputDate, Value: "2026-08-10", Label: "End date", Mono: true}},
										Width: w.FieldSM,
									},
									w.Field{
										Label: "End time",
										Parts: []w.FieldPart{w.Input{Kind: w.InputTime, Value: "13:00", Label: "End time", Mono: true}},
										Width: w.FieldXS,
									},
									w.Field{
										Label: "Time zone",
										Parts: []w.FieldPart{
											w.Drop{
												Trigger: w.Pick{
													Value: "Asia/Yekaterinburg",
													Aside: &w.Dim{Parts: []w.Inline{w.Text("as sent")}, Variant: w.DimIndent},
												},
												Menu: w.Menu{
													Items: []w.MenuPart{
														w.Input{Kind: w.InputSearch, Placeholder: "Find a zone", Label: "Find a zone"},
														w.Rubric{Text: "As sent", Form: w.RubricInline},
														w.MenuItem{Label: "Asia/Yekaterinburg", On: true, Name: "tz", Value: "Asia/Yekaterinburg"},
														w.MenuSep{},
														w.MenuItem{Label: "Europe/Moscow", Name: "tz", Value: "Europe/Moscow"},
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
									w.Field{Label: "Reminder", Parts: []w.FieldPart{w.Pick{Value: "1 day before"}}, Width: w.FieldMD},
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
													Parts: []w.TagInputPart{
														w.Tag{
															Label: "Мария Лебедева",
															Mark:  &w.Mark{Colour: "#6B4E7A", Size: w.MarkH11},
															Fixed: true,
															Aside: &w.Dim{Parts: []w.Inline{w.Text("· organiser")}},
															Title: "m.lebedeva@example.org",
															Name:  "attendee",
															Value: "mailto:m.lebedeva@example.org",
														},
														w.Tag{
															Label:     "Анна Ковалёва",
															Mark:      &w.Mark{Colour: "#4A6B52", Size: w.MarkH11},
															Removable: true,
															Aside:     &w.Dim{Parts: []w.Inline{w.Text("· accepted")}},
															Title:     "a.kovaleva@example.org",
															Name:      "attendee",
															Value:     "mailto:a.kovaleva@example.org",
														},
														w.Tag{
															Label:     "Артём Белов",
															Mark:      &w.Mark{Colour: "#6B4E7A", Size: w.MarkH11},
															Removable: true,
															Aside:     &w.Dim{Parts: []w.Inline{w.Text("· tentative")}},
															Title:     "a.belov@example.org",
															Name:      "attendee",
															Value:     "mailto:a.belov@example.org",
														},
														w.Tag{
															Address:   &w.Mono{Text: "v@example.org"},
															Removable: true,
															Aside:     &w.Dim{Parts: []w.Inline{w.Text("· no reply")}},
															Title:     "v@example.org",
															Name:      "attendee",
															Value:     "mailto:v@example.org",
														},
													},
													Columns: true,
													Name:    "attendee",
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
											w.Faced{
												Face: w.FaceLarge,
												Items: []w.LinePart{
													w.Text("release-checklist.md "),
													w.Dim{Parts: []w.Inline{w.Text("text/markdown · 3 KB")}},
													w.Text(" "),
													w.Badge{Label: "in the event"},
												},
											},
											w.Button{Label: "Remove", Kind: w.ButtonDanger, Small: true},
										},
										Gap:   w.LineGap8,
										Block: true,
										Place: w.Grown,
									},
								},
							},
							w.FieldSet{
								Rubric: w.Rubric{Text: "Repeats"},
								Items: []w.FieldSetPart{
									w.Field{Label: "Frequency", Parts: []w.FieldPart{w.Pick{Value: "Weekly"}}, Width: w.FieldSM},
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
													{Label: "MO", On: true, Value: "MO"},
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
										Parts: []w.FieldPart{w.Input{Kind: w.InputDate, Value: "2026-12-28", Label: "Until", Mono: true}},
										Width: w.FieldSM,
									},
									w.Field{Label: "Count", Parts: []w.FieldPart{w.Input{Placeholder: "—", Label: "Count"}}, Width: w.FieldXS},
									w.Field{
										Label: "Advanced RRULE",
										Parts: []w.FieldPart{
											w.Input{Placeholder: "FREQ=WEEKLY;BYDAY=MO;UNTIL=20261228", Label: "Advanced RRULE", Mono: true},
											w.FieldHint{Parts: []w.Inline{w.Text("The advanced value takes precedence when set.")}},
										},
										Width: w.FieldXL,
									},
								},
							},
							w.FieldSet{
								Rubric: w.Rubric{Text: "Kept as the server sent them"},
								Items: []w.FieldSetPart{
									w.Diag{
										Text:  "X-MOZ-GENERATION  3\nX-APPLE-TRAVEL-ADVISORY-BEHAVIOR  AUTOMATIC\nSEQUENCE  2",
										Place: w.Grown,
									},
								},
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
