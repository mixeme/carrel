// Drafted by frame2go from frame «Календарь · пустая неделя» (states-15) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// States15 is frame «Календарь · пустая неделя» (states-15).
func States15() interface{ Render() string } {
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
				w.Nav{
					Items: []w.NavItem{
						{Icon: "con", Label: "Contacts", Href: "#"},
						{Icon: "cal", Label: "Calendar", Href: "#"},
						{Icon: "task", Label: "Tasks", Href: "#"},
						{Icon: "note", Label: "Notes", Href: "#"},
						{Icon: "file", Label: "Files", Href: "#"},
						{Icon: "dup", Label: "Duplicates", Href: "#"},
					},
					On: 2,
				},
				w.RailSec{
					Items: []w.RailSecPart{
						w.TickRow{
							Ticked: w.TickedPart,
							Kind:   w.TickRowAll,
							Mark:   w.Mark{Kind: w.MarkMulti},
							Label:  "Combined calendar",
							Meta:   "0",
						},
					},
				},
				w.RailSec{
					Items: []w.RailSecPart{
						w.Rubric{Text: "nextcloud"},
						w.TickRow{Ticked: w.TickedOff, Mark: w.Mark{Colour: "#A05A2C"}, Label: "Дни рождения", Meta: "—", Hidden: true},
						w.TickRow{Mark: w.Mark{Colour: "#4A6B52"}, Label: "Личный", Meta: "0", Current: true},
						w.TickRow{Mark: w.Mark{Colour: "#3F6E8C"}, Label: "Работа", Meta: "0"},
					},
				},
				w.RailSec{
					Items: []w.RailSecPart{
						w.Rubric{Text: "radicale · vps"},
						w.TickRow{Mark: w.Mark{Colour: "#8C6239"}, Label: "Дежурства", Meta: "0"},
						w.TickRow{Mark: w.Mark{Colour: "#6B4E7A"}, Label: "Проекты", Meta: "0"},
					},
				},
				w.RailSec{
					Items: []w.RailSecPart{
						w.Rubric{Text: "baikal"},
						w.TickRow{Mark: w.Mark{Colour: "#9A9280"}, Label: "Общий", Meta: "0"}},
				},
				w.RailSec{
					Items: []w.RailSecPart{
						w.Rubric{Text: "Subscriptions", Items: []w.RubricPart{w.Badge{Label: "external"}}},
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
				w.RailSec{
					Items: []w.RailSecPart{
						w.Rubric{Text: "Tags"}},
					Tags: []w.Tag{w.Tag{Label: "all", On: true}, w.Tag{Label: "релиз"}, w.Tag{Label: "работа"}},
				},
				w.RailFoot{
					Items: []w.RailFootPart{w.Emblem{Symbol: "davloom", Title: "davloom · running · 3 devices"}},
					About: &w.About{Name: "Carrel", Build: w.Mono{Text: "0.0.1 · 786949a"}, Href: "#rest"},
				},
			},
			Main: w.Main{
				Items: []w.MainPart{
					w.Head{
						Title: &w.HeadTitle{Mark: &w.Mark{Colour: "#4A6B52", Size: w.MarkH20}, Text: "Личный"},
						Subs:  []w.HeadSub{w.HeadSub{Parts: []w.HeadSubPart{w.Text("nextcloud · 2 – 8 November 2026 · 0 events")}}},
						Acts: &w.Acts{
							Items: []w.ActsPart{
								w.Button{Label: "New event", Icon: "plus", Kind: w.ButtonPrimary},
								w.Button{Label: "Import", Icon: "up"},
								w.Button{Label: "Export", Icon: "down"},
								w.Button{Label: "Print", Icon: "print"},
							},
						},
					},
					w.Bar{
						Items: []w.BarPart{
							w.Button{Icon: "back", Title: "Previous week", Href: "/app/calendar/nextcloud/lichny?from=2026-10-26"},
							w.Button{Label: "Today", Href: "/app/calendar/nextcloud/lichny"},
							w.Button{Icon: "next", Title: "Next week", Href: "/app/calendar/nextcloud/lichny?from=2026-11-09"},
							w.Seg{
								Options: []w.SegOption{{Label: "Day"}, {Label: "Week", On: true}, {Label: "Month"}, {Label: "Range"}},
							},
							w.Range{
								From:      w.Input{Kind: w.InputDate, Name: "from", Value: "2026-11-02", Label: "From", Mono: true},
								To:        w.Input{Kind: w.InputDate, Name: "to", Value: "2026-11-08", Label: "To", Mono: true},
								Show:      w.Button{Label: "Show"},
								Hidden:    []w.Hidden{{Name: "view", Value: "range"}},
								Secondary: true,
								Action:    "/app/calendar",
							},
							w.BarSep{Secondary: true},
							w.CheckForm{
								Checks: []w.Check{
									w.Check{On: true, Name: "show", Value: "events", Parts: []w.CheckPart{w.Text("Events")}},
									w.Check{Name: "show", Value: "tasks", Parts: []w.CheckPart{w.Text("Tasks due")}},
									w.Check{Name: "show", Value: "notes", Parts: []w.CheckPart{w.Text("Notes")}},
								},
								Action:   "/app/calendar",
								Tertiary: true,
								Apply:    w.Button{Label: "Apply", Small: true},
							},
							w.Drop{
								Trigger: w.Button{Icon: "dots", Title: "More"},
								More:    true,
								Menu: w.Menu{
									Items: []w.MenuPart{
										w.CheckForm{
											Checks: []w.Check{
												w.Check{On: true, Name: "show", Value: "events", Parts: []w.CheckPart{w.Text("Events")}},
												w.Check{Name: "show", Value: "tasks", Parts: []w.CheckPart{w.Text("Tasks due")}},
												w.Check{Name: "show", Value: "notes", Parts: []w.CheckPart{w.Text("Notes")}},
											},
											Action:   "/app/calendar",
											Tertiary: true,
											Apply:    w.Button{Label: "Apply", Small: true},
										},
										w.MenuSep{Tertiary: true},
										w.Range{
											From:   w.Input{Kind: w.InputDate, Name: "from", Value: "2026-11-02", Label: "From", Mono: true},
											To:     w.Input{Kind: w.InputDate, Name: "to", Value: "2026-11-08", Label: "To", Mono: true},
											Show:   w.Button{Label: "Show"},
											Hidden: []w.Hidden{{Name: "view", Value: "range"}},
											Action: "/app/calendar",
											Wrap:   true,
										},
										w.MenuSep{},
										w.CheckForm{
											Checks: []w.Check{w.Check{Name: "compact", Value: "1", Parts: []w.CheckPart{w.Text("Compact rows")}}},
											Action: "/app/calendar",
											Apply:  w.Button{Label: "Apply", Small: true},
										},
									},
									Pop:    w.MenuUnderEnd,
									Inline: true,
								},
							},
						},
						Right: &w.BarRight{
							Items: []w.BarPart{
								w.CheckForm{
									Checks: []w.Check{w.Check{Name: "compact", Value: "1", Parts: []w.CheckPart{w.Text("Compact rows")}}},
									Action: "/app/calendar",
									Apply:  w.Button{Label: "Apply", Small: true},
								},
							},
							Secondary: true,
						},
						Variant: w.BarWide,
					},
					w.Empty{
						Parts: []w.EmptyPart{w.Strong("No events this week."), w.Text(" Nothing in Личный from 2 to 8 November.")},
					},
				},
			},
		},
	}
}
