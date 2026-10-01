// Drafted by frame2go from frame «Человек · хронология» (s-contacts-7) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// Contacts7 is frame «Человек · хронология» (s-contacts-7).
func Contacts7() interface{ Render() string } {
	return w.Shell{
		Top: &w.Top{
			Brand:  w.Brand{Logo: &w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63"}},
			Search: &w.Search{Query: w.Query{Value: "Search everything…", Label: "Search everything…"}},
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
					On: 1,
				},
				w.Nav{Items: []w.NavItem{
					{Icon: "back", Label: "Back to list", Short: "Back", Href: "#"}}, Apart: true},
				w.RailSec{
					Items: []w.RailSecPart{
						w.Rubric{Text: "Where to look"},
						w.TickRow{
							Ticked:  w.TickedPart,
							Kind:    w.TickRowAll,
							Mark:    w.Mark{Kind: w.MarkMulti},
							Label:   "Combined calendar",
							Meta:    "20",
							Current: true,
						},
						w.TickRow{Mark: w.Mark{Colour: "#4A6B52"}, Label: "Личный", Meta: "9"},
						w.TickRow{Mark: w.Mark{Colour: "#3F6E8C"}, Label: "Работа", Meta: "7"},
						w.TickRow{Ticked: w.TickedOff, Mark: w.Mark{Colour: "#A05A2C"}, Label: "Дни рождения", Meta: "—", Hidden: true},
						w.TickRow{Mark: w.Mark{Colour: "#6B4E7A"}, Label: "Проекты", Meta: "4"},
						w.TickRow{Mark: w.Mark{Colour: "#8C6239"}, Label: "Дежурства", Meta: "0"},
						w.TickRow{
							Mark:      w.Mark{Colour: "#9A9280"},
							Label:     "Общий",
							Meta:      "password · 401",
							MetaTitle: "401 Unauthorized — the saved password was refused. Open Connections to update it.",
							Error:     true,
							Trail:     w.Button{Label: "Retry", Small: true},
						},
						w.TickRow{Kind: w.TickRowExternal, Mark: w.Mark{Colour: "#8C6239"}, Label: "Праздники РФ", Meta: "ro"},
						w.TickRow{
							Ticked: w.TickedOff,
							Kind:   w.TickRowExternal,
							Mark:   w.Mark{Colour: "#6B4E7A"},
							Label:  "Расписание лиги",
							Meta:   "ro",
							Hidden: true,
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
						Avatar: &w.Avatar{Initials: "АК", Size: w.Avatar44},
						Kicker: &w.Rubric{Text: "Timeline", Variant: w.RubricBottom2},
						Title:  &w.HeadTitle{Text: "Анна Ковалёва"},
						Subs: []w.HeadSub{
							w.HeadSub{
								Parts: []w.HeadSubPart{
									w.Text("Издательство «Слово» · Выпускающий редактор "),
									w.Badge{Label: "in 2 books", Tone: w.BadgeLinked},
								},
							},
						},
						Acts: &w.Acts{
							Items: []w.ActsPart{
								w.Button{Label: "Write a note", Icon: "quill", Kind: w.ButtonPrimary},
								w.Button{Label: "New event", Icon: "cal"},
								w.Button{Label: "New task", Icon: "task"},
								w.Button{Label: "Print", Icon: "print"},
							},
						},
						Pad: w.HeadPadBottom10,
					},
					w.Bar{
						Items: []w.BarPart{
							w.Range{
								From:   w.Input{Kind: w.InputDate, Name: "from", Value: "2026-01-01", Label: "From", Mono: true},
								To:     w.Input{Kind: w.InputDate, Name: "to", Value: "2026-11-10", Label: "To", Mono: true},
								Show:   w.Button{Label: "Show"},
								Action: "/app/contacts/nextcloud/kontakty/4f2b-9c31-a0e7/timeline",
							},
							w.SegFold{
								Seg: w.Seg{
									Options: []w.SegOption{
										{
											Label: "All",
											Count: "24",
											Href:  "/app/contacts/nextcloud/kontakty/4f2b-9c31-a0e7/timeline",
											On:    true,
										},
										{
											Label: "Events",
											Count: "14",
											Href:  "/app/contacts/nextcloud/kontakty/4f2b-9c31-a0e7/timeline?kind=event",
										},
										{
											Label: "Tasks",
											Count: "2",
											Href:  "/app/contacts/nextcloud/kontakty/4f2b-9c31-a0e7/timeline?kind=task",
										},
										{
											Label: "Notes",
											Count: "4",
											Href:  "/app/contacts/nextcloud/kontakty/4f2b-9c31-a0e7/timeline?kind=note",
										},
										{
											Label: "Files",
											Count: "4",
											Href:  "/app/contacts/nextcloud/kontakty/4f2b-9c31-a0e7/timeline?kind=file",
										},
									},
								},
							},
							w.Button{Label: "Columns", Icon: "cols", Secondary: true},
						},
						More:    &w.Button{Icon: "dots", Title: "More"},
						Right:   &w.BarRight{Items: []w.BarPart{w.Mono{Text: "newest first"}}, Secondary: true},
						Variant: w.BarWide,
					},
					w.Runbar{
						Meter: w.Meter{Percent: 83},
						Parts: []w.RunbarPart{
							w.Text("Polled 5 of 6 calendars · matched by "),
							w.Mono{Text: "a.kovaleva@example.org"},
							w.Text(", "),
							w.Mono{Text: "anna.kovaleva@slovo.example"},
							w.Text(" and the name · "),
							w.Span{Face: w.FaceDanger, Text: "baikal · Общий unavailable"},
						},
					},
					w.ListGroup{Title: w.Rubric{Text: "August 2026", Form: w.RubricInline}, Count: &w.Num{Value: 3}},
					w.List{
						Rows: []w.ListRow{
							w.FindRow{
								Mark:   w.Mark{Colour: "#4A6B52"},
								Date:   "10 Aug",
								Title:  []w.Inline{w.Text("Синк-ап по релизу")},
								Detail: []w.Inline{w.Text("attendee · accepted")},
								What:   "event",
								Source: "nextcloud · Личный",
							},
							w.FindRow{
								Mark:   w.Mark{Colour: "#4A6B52"},
								Date:   "4 Aug",
								Title:  []w.Inline{w.Text("2026-08-04-обложка.pdf")},
								Detail: []w.Inline{w.Text("attached to «Согласовать обложку»")},
								What:   "file",
								Source: "sftpgo · vps · Documents",
							},
							w.FindRow{
								Mark:   w.Mark{Colour: "#3F6E8C"},
								Date:   "4 Aug",
								Title:  []w.Inline{w.Text("Согласовать обложку")},
								Detail: []w.Inline{w.Text("organiser · done")},
								What:   "task",
								Source: "nextcloud · Работа",
							},
						},
					},
					w.ListGroup{Title: w.Rubric{Text: "July 2026", Form: w.RubricInline}, Count: &w.Num{Value: 5}},
					w.List{
						Rows: []w.ListRow{
							w.FindRow{
								Mark:   w.Mark{Colour: "#6B4E7A"},
								Date:   "29 Jul",
								Title:  []w.Inline{w.Text("Ревью вёрстки сборника")},
								Detail: []w.Inline{w.Text("attendee · no reply")},
								What:   "event",
								Source: "radicale · vps · Проекты",
							},
						},
					},
					w.Empty{
						Parts:   []w.EmptyPart{w.Text("Nothing was looked for before January 2026. Widen the range if you need it.")},
						Variant: w.EmptySmallTop,
					},
				},
			},
			Side: &w.Side{
				Items: []w.SidePart{
					w.Head{
						Photo:  &w.HeadPhoto{Initials: "АК", Large: true},
						Kicker: &w.Rubric{Text: "Card"},
						Tools: []w.HeadTool{
							w.Button{Label: "Open the card", Small: true},
							w.Button{Label: "Edit", Small: true},
							w.Drop{
								Trigger: w.Button{Icon: "dots", Small: true, Title: "More"},
								Menu: w.Menu{
									Items: []w.MenuPart{
										w.MenuItem{Icon: "move", Label: "Move"},
										w.MenuItem{Icon: "dup", Label: "Copy"},
										w.MenuItem{Icon: "down", Detail: &w.Mono{Text: ".vcf", Variant: w.Mono11}, Label: "Export"},
										w.MenuItem{Icon: "link", Label: "Publish a link"},
									},
									Width: w.MenuWidth212,
									Pop:   w.MenuUnder,
								},
							},
						},
						Acts:   &w.Acts{Items: []w.ActsPart{w.Close{Label: "Close the panel"}}},
						Middle: true,
						Pad:    w.HeadPadBottom10,
					},
					w.Fields{
						Items: []w.FieldsItem{
							{Term: "Phone · mobile", Value: []w.ValuePart{w.Text("+7 912 445-10-88")}, Face: w.ValueMono},
							{Term: "Phone · work", Value: []w.ValuePart{w.Text("+7 495 120-45-00")}, Face: w.ValueMono},
							{Term: "Email · work", Value: []w.ValuePart{w.Text("a.kovaleva@example.org")}},
							{Term: "Email · second", Value: []w.ValuePart{w.Text("anna.kovaleva@slovo.example")}},
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
								Gap: 6,
							},
							w.SecHint{
								Text:  "Linked on the duplicates screen. The records stay on their own servers; here they are shown as one person.",
								Apart: true,
							},
						},
					},
					w.Sec{
						Items: []w.SecPart{
							w.Rubric{Text: "Source"},
							w.Diag{Text: "UID   4f2b-9c31-a0e7\nRead  2 min ago"}},
					},
					w.Danger{Button: w.Button{Label: "Delete contact", Kind: w.ButtonDangerSolid}},
				},
			},
		},
	}
}
