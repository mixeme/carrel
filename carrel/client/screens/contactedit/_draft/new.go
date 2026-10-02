// Drafted by frame2go from frame «Контакт · новый, полная форма» (s-contacts-27) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// Contacts27 is frame «Контакт · новый, полная форма» (s-contacts-27).
func Contacts27() interface{ Render() string } {
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
						Title: &w.HeadTitle{Text: "New contact"},
						Subs:  []w.HeadSub{w.HeadSub{Parts: []w.HeadSubPart{w.Text("nextcloud · Контакты")}}},
						Acts:  &w.Acts{Items: []w.ActsPart{w.Button{Label: "Cancel"}, w.Button{Label: "Save", Kind: w.ButtonPrimary}}},
					},
					w.Form{
						Items: []w.FormPart{
							w.Field{
								Label:     "Photo",
								LabelSpan: true,
								Parts: []w.FieldPart{
									w.Photo{
										Side: []w.FieldPart{
											w.Line{
												Items: []w.LinePart{
													w.Button{
														Label: "Add",
														File:  &w.FileField{Form: "post-photo", Name: "photo", Accept: "image/*", Label: "Photo"},
													},
													w.Button{Label: "Upload", Form: "post-photo", NoScript: true},
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
										Parts: []w.FieldPart{w.Input{Label: "Display name", Focus: true}},
										Width: w.FieldXL,
									},
									w.Field{Label: "Given", Parts: []w.FieldPart{w.Input{Placeholder: "—", Label: "Given"}}, Width: w.FieldSM},
									w.Field{Label: "Family", Parts: []w.FieldPart{w.Input{Placeholder: "—", Label: "Family"}}, Width: w.FieldSM},
									w.Field{
										Label: "Nickname",
										Parts: []w.FieldPart{w.Input{Placeholder: "—", Label: "Nickname"}},
										Width: w.FieldSM,
									},
									w.Field{
										Label: "Organisation",
										Parts: []w.FieldPart{w.Input{Placeholder: "—", Label: "Organisation"}},
										Width: w.FieldMD,
									},
									w.Field{Label: "Title", Parts: []w.FieldPart{w.Input{Placeholder: "—", Label: "Title"}}, Width: w.FieldMD},
									w.Field{
										Label: "Birthday",
										Parts: []w.FieldPart{
											w.Input{Placeholder: "DD.MM.YYYY", Label: "Birthday", Mono: true, Numeric: true},
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
										Parts: []w.FieldPart{w.Input{Placeholder: "—", Label: "Number", Mono: true}},
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
										Parts: []w.FieldPart{w.Input{Placeholder: "—", Label: "Address"}},
										Width: w.FieldLG,
									},
									w.Button{Label: "Remove", Kind: w.ButtonDanger},
									w.Grow{},
									w.Button{Label: "Add email", Icon: "plus", Small: true},
								},
							},
							w.FieldSet{
								Rubric: w.Rubric{Text: "Addresses"},
								Items:  []w.FieldSetPart{w.Button{Label: "Add address", Icon: "plus", Small: true}},
							},
							w.FieldSet{
								Rubric: w.Rubric{Text: "Other"},
								Items: []w.FieldSetPart{
									w.Field{Label: "Tags", Parts: []w.FieldPart{w.Input{Placeholder: "—", Label: "Tags"}}, Width: w.FieldLG},
									w.Field{Label: "Note", Parts: []w.FieldPart{w.Area{Text: "", Label: "Text"}}, Width: w.FieldFull},
								},
							},
							w.Field{Parts: []w.FieldPart{w.Button{Label: "Show 6 empty fields"}}},
						},
					},
				},
			},
		},
	}
}
