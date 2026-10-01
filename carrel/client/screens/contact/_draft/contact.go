// Drafted by frame2go from frame «Контакт · карточка, чтение» (s-contacts-6) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// Contacts6 is frame «Контакт · карточка, чтение» (s-contacts-6).
func Contacts6() interface{ Render() string } {
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
						Photo: &w.HeadPhoto{},
						Title: &w.HeadTitle{Text: "Анна Ковалёва"},
						Subs: []w.HeadSub{
							w.HeadSub{Parts: []w.HeadSubPart{w.Text("Издательство «Слово» · Выпускающий редактор")}},
							w.HeadSub{Parts: []w.HeadSubPart{w.Text("nextcloud · Контакты")}},
						},
						Acts: &w.Acts{
							Items: []w.ActsPart{
								w.Button{Label: "Edit", Icon: "pencil", Kind: w.ButtonPrimary},
								w.Button{Label: "Send", Icon: "mail"},
								w.Button{Label: "Print", Icon: "print"},
								w.Drop{
									Trigger: w.Button{Icon: "dots", Title: "More"},
									Menu: w.Menu{
										Items: []w.MenuPart{
											w.MenuItem{Icon: "move", Label: "Move"},
											w.MenuItem{Icon: "dup", Label: "Copy"},
											w.MenuItem{Icon: "down", Detail: &w.Mono{Text: ".vcf", Variant: w.Mono11}, Label: "Export"},
											w.MenuItem{Icon: "link", Label: "Publish a link"},
										},
										Width: w.MenuWidth212,
										Pop:   w.MenuUnderEnd,
									},
								},
							},
						},
					},
					w.Fields{
						Items: []w.FieldsItem{
							{
								Term:  "Phone · mobile",
								Value: []w.ValuePart{w.Text("+7 912 445-10-88")},
								Face:  w.ValueMono,
								Href:  "tel:+79124451088",
							},
							{
								Term:  "Phone · work",
								Value: []w.ValuePart{w.Text("+7 495 120-45-00")},
								Face:  w.ValueMono,
								Href:  "tel:+74951204500",
							},
							{
								Term:  "Email · work",
								Value: []w.ValuePart{w.Text("a.kovaleva@example.org")},
								Href:  "mailto:a.kovaleva@example.org",
							},
							{
								Term:  "Email · second",
								Value: []w.ValuePart{w.Text("anna.kovaleva@slovo.example")},
								Href:  "mailto:anna.kovaleva@slovo.example",
							},
							{Term: "Address · work", Value: []w.ValuePart{w.Text("Москва, Тверская 12, стр. 3")}},
							{
								Term:  "Birthday",
								Value: []w.ValuePart{w.Text("1987-03-14 "), w.Dim{Parts: []w.Inline{w.Text("· in 7 months")}}},
								Face:  w.ValueMono,
							},
							{
								Term:  "Tags",
								Value: []w.ValuePart{w.Tag{Label: "работа"}, w.Text(" "), w.Tag{Label: "издательство"}},
							},
							{Term: "Note", Value: []w.ValuePart{w.Text("Пишет по вторникам, отвечает в тот же день.")}},
						},
					},
					w.Sec{
						Items: []w.SecPart{
							w.Rubric{Text: "Kept as the server sent them", Items: []w.RubricPart{w.Button{Label: "Show 3", Small: true}}},
							w.SecHint{Text: "Properties Carrel does not edit are stored byte for byte and go back untouched."},
						},
					},
					w.Sec{
						Items: []w.SecPart{
							w.Rubric{
								Text:  "The same person in other books",
								Items: []w.RubricPart{w.Badge{Label: "2", Tone: w.BadgeLinked}},
							},
							w.SecList{
								Items: [][]w.SecItemPart{
									[]w.SecItemPart{
										w.Mark{Colour: "#6B4E7A", Size: w.MarkH11Inline},
										w.Text(" "),
										w.Link{Text: "radicale · vps · Команда", Href: "#"},
										w.Text(" "),
										w.Dim{Parts: []w.Inline{w.Text("· Phone, email differ")}},
									},
									[]w.SecItemPart{
										w.Mark{Colour: "#9A9280", Size: w.MarkH11Inline},
										w.Text(" "),
										w.Link{Text: "baikal · Общий", Href: "#"},
										w.Text(" "),
										w.Dim{Parts: []w.Inline{w.Text("· The server did not answer; the record stays linked")}},
									},
								},
								Gap: 5,
							},
						},
					},
					w.Sec{
						Items: []w.SecPart{
							w.Rubric{Text: "Timeline", Items: []w.RubricPart{w.Mono{Text: "11", Variant: w.MonoTight}}},
							w.List{
								Rows: []w.ListRow{
									w.LinkedRow{
										Mark:   w.Mark{Colour: "#4A6B52"},
										Icon:   "cal",
										Title:  "Синк-ап по релизу",
										Detail: "10 Aug · attendee · accepted",
									},
									w.LinkedRow{
										Mark:   w.Mark{Colour: "#4A6B52"},
										Icon:   "file",
										Title:  "2026-08-04-обложка.pdf",
										Detail: "04 Aug · attached to «Согласовать обложку»",
									},
									w.LinkedRow{
										Mark:   w.Mark{Colour: "#3F6E8C"},
										Icon:   "task",
										Title:  "Согласовать обложку",
										Detail: "04 Aug · organiser · done",
									},
								},
							},
							w.SecActs{Items: []w.Button{w.Button{Label: "Open the timeline", Icon: "all"}}},
						},
					},
					w.Sec{
						Items: []w.SecPart{
							w.Rubric{Text: "Source"},
							w.Diag{Text: "Path  /dav.php/addressbooks/mix/contacts/2b7e.vcf\nRead  2 min ago · not changed on server"},
						},
					},
					w.Danger{Button: w.Button{Label: "Delete contact", Icon: "trash", Kind: w.ButtonDangerSolid}},
				},
			},
		},
	}
}
