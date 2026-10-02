// Drafted by frame2go from frame «Правка · сервер не сохранил свойства» (states-12) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// States12 is frame «Правка · сервер не сохранил свойства» (states-12).
func States12() interface{ Render() string } {
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
						w.TickRow{Mark: w.Mark{Colour: "#4A6B52"}, Label: "Контакты", Meta: "248", Current: true},
						w.TickRow{Mark: w.Mark{Colour: "#A05A2C"}, Label: "Архив", Meta: "51"},
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
						Title: &w.HeadTitle{Text: "Анна Ковалёва"},
						Subs:  []w.HeadSub{w.HeadSub{Parts: []w.HeadSubPart{w.Text("nextcloud · Контакты")}}},
						Acts:  &w.Acts{Items: []w.ActsPart{w.Button{Label: "Cancel"}, w.Button{Label: "Save", Kind: w.ButtonPrimary}}},
					},
					w.Msg{
						Parts: []w.MsgPart{
							w.Strong("The change was saved, but the server did not keep these properties."),
							w.Text(" Other clients will not see them either: "),
							w.Mono{Text: "X-ABLABEL"},
							w.Text(", "),
							w.Mono{Text: "X-CARREL-UNKNOWN-FLAG"},
							w.Text(", "),
							w.Mono{Text: "X-EVOLUTION-FILE-AS"},
							w.Text("."),
						},
						Space: w.MsgFlushTop2,
					},
					w.Form{
						Items: []w.FormPart{
							w.Field{
								Label:     "Photo",
								LabelSpan: true,
								Parts: []w.FieldPart{
									w.Photo{
										Initials: "АК",
										ID:       "photo",
										Side: []w.FieldPart{
											w.Line{
												Items: []w.LinePart{
													w.Button{
														Label: "Add",
														File:  &w.FileField{Form: "", Name: "photo", Accept: "image/*", Label: "Photo", Preview: "photo"},
													},
												},
												Gap: w.LineGap6,
											},
											w.FieldHint{Parts: []w.Inline{w.Text("JPEG, PNG, GIF or WebP.")}, Variant: w.HintNarrow},
										},
									},
								},
								Width: w.FieldFixAuto,
							},
							w.FormCols{
								Items: []w.Field{
									w.Field{
										Label: "Display name",
										Parts: []w.FieldPart{w.Input{Value: "Анна Ковалёва", Label: "Display name", Focus: true}},
										Width: w.FieldXL,
									},
									w.Field{Label: "Given", Parts: []w.FieldPart{w.Input{Value: "Анна", Label: "Given"}}, Width: w.FieldSM},
									w.Field{Label: "Family", Parts: []w.FieldPart{w.Input{Value: "Ковалёва", Label: "Family"}}, Width: w.FieldSM},
									w.Field{
										Label: "Nickname",
										Parts: []w.FieldPart{w.Input{Placeholder: "—", Label: "Nickname"}},
										Width: w.FieldSM,
									},
									w.Field{
										Label: "Organisation",
										Parts: []w.FieldPart{w.Input{Value: "Издательство «Слово»", Label: "Organisation"}},
										Width: w.FieldMD,
									},
									w.Field{
										Label: "Title",
										Parts: []w.FieldPart{w.Input{Value: "Выпускающий редактор", Label: "Title"}},
										Width: w.FieldMD,
									},
									w.Field{
										Label: "Birthday",
										Parts: []w.FieldPart{
											w.Input{Value: "14.03.1987", Label: "Birthday", Mono: true, Numeric: true},
											w.FieldHint{Parts: []w.Inline{w.Text("Type it: 14.03.1987, or 14.03 without the year.")}},
										},
										Width: w.FieldSM,
									},
								},
							},
							w.FieldSet{
								Rubric: w.Rubric{Text: "Phone numbers"},
								Items: []w.FieldSetPart{
									w.Field{Label: "Label", Parts: []w.FieldPart{w.Pick{Value: "mobile"}}, Width: w.FieldXS},
									w.Field{
										Label: "Number",
										Parts: []w.FieldPart{w.Input{Value: "+7 912 445-10-88", Label: "Number", Mono: true}},
										Width: w.FieldMD,
									},
									w.Button{Label: "Remove", Kind: w.ButtonDanger},
									w.Grow{},
									w.Field{Parts: []w.FieldPart{w.Pick{Value: "work", Label: "Label"}}, Width: w.FieldXS},
									w.Field{
										Parts: []w.FieldPart{w.Input{Value: "+7 495 120-45-00", Label: "Number", Mono: true}},
										Width: w.FieldMD,
									},
									w.Button{Label: "Remove", Kind: w.ButtonDanger},
									w.Grow{},
									w.Button{Label: "Add phone", Icon: "plus", Small: true},
								},
							},
							w.FieldSet{
								Rubric: w.Rubric{Text: "Email addresses"},
								Items: []w.FieldSetPart{
									w.Field{Label: "Label", Parts: []w.FieldPart{w.Pick{Value: "work"}}, Width: w.FieldXS},
									w.Field{
										Label: "Address",
										Parts: []w.FieldPart{w.Input{Value: "a.kovaleva@example.org", Label: "Address"}},
										Width: w.FieldLG,
									},
									w.Button{Label: "Remove", Kind: w.ButtonDanger},
									w.Grow{},
									w.Button{Label: "Add email", Icon: "plus", Small: true},
								},
							},
							w.FieldSet{
								Rubric: w.Rubric{Text: "Addresses"},
								Items: []w.FieldSetPart{
									w.Field{Label: "Label", Parts: []w.FieldPart{w.Pick{Value: "home"}}, Width: w.FieldXS},
									w.Field{
										Label: "Street",
										Parts: []w.FieldPart{w.Input{Value: "ул. Садовая, 12, кв. 5", Label: "Street"}},
										Width: w.FieldLG,
									},
									w.Field{Label: "City", Parts: []w.FieldPart{w.Input{Value: "Москва", Label: "City"}}, Width: w.FieldSM},
									w.Field{Label: "Region", Parts: []w.FieldPart{w.Input{Placeholder: "—", Label: "Region"}}, Width: w.FieldSM},
									w.Field{
										Label: "Postcode",
										Parts: []w.FieldPart{w.Input{Value: "123001", Label: "Postcode", Mono: true}},
										Width: w.FieldXS,
									},
									w.Field{Label: "Country", Parts: []w.FieldPart{w.Input{Value: "Россия", Label: "Country"}}, Width: w.FieldSM},
									w.Button{Label: "Remove", Kind: w.ButtonDanger},
									w.Grow{},
									w.Button{Label: "Add address", Icon: "plus", Small: true},
								},
							},
							w.FieldSet{
								Rubric: w.Rubric{Text: "Other"},
								Items: []w.FieldSetPart{
									w.Field{
										Label: "Tags",
										Parts: []w.FieldPart{w.Input{Value: "работа, издательство", Label: "Tags"}},
										Width: w.FieldLG,
									},
									w.Field{
										Label: "Note",
										Parts: []w.FieldPart{w.Area{Text: "Пишет по вторникам, отвечает в тот же день.", Label: "Text"}},
										Width: w.FieldFull,
									},
								},
							},
							w.Field{Parts: []w.FieldPart{w.Button{Label: "Show 6 empty fields"}}},
						},
					},
					w.FormFoot{
						Items: []w.FormFootPart{
							w.Dim{Parts: []w.Inline{w.Text("Properties Carrel does not know are kept unchanged.")}, Size: w.Dim11_5},
						},
					},
				},
			},
		},
	}
}
