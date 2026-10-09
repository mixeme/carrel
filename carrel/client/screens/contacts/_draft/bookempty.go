// Drafted by frame2go from frame «Книга пуста» (states-10) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// States10 is frame «Книга пуста» (states-10).
func States10() interface{ Render() string } {
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
						{Icon: "con", Label: "Contacts", Count: &w.Num{Value: 5}, Href: "#"},
						{Icon: "cal", Label: "Calendar", Count: &w.Num{Value: 6}, Href: "#"},
						{Icon: "task", Label: "Tasks", Count: &w.Num{Value: 2}, Href: "#"},
						{Icon: "note", Label: "Notes", Count: &w.Num{Value: 3}, Href: "#"},
						{Icon: "file", Label: "Files", Href: "#"},
						{Icon: "dup", Label: "Duplicates", Count: &w.Num{Value: 2}, Href: "#"},
					},
					On: 1,
				},
				w.RailSec{
					Items: []w.RailSecPart{
						w.TickRow{
							Ticked: w.TickedPart,
							Kind:   w.TickRowAll,
							Mark:   w.Mark{Kind: w.MarkMulti},
							Label:  "Combined address book",
							Meta:   "5",
						},
					},
				},
				w.RailSec{
					Items: []w.RailSecPart{
						w.Rubric{Text: "nextcloud"},
						w.TickRow{Mark: w.Mark{Colour: "#4A6B52"}, Label: "Контакты", Meta: "3"},
						w.TickRow{Mark: w.Mark{Colour: "#A05A2C"}, Label: "Архив", Meta: "1"},
					},
				},
				w.RailSec{
					Items: []w.RailSecPart{
						w.Rubric{Text: "radicale · vps"},
						w.TickRow{Mark: w.Mark{Colour: "#6B4E7A"}, Label: "Команда", Meta: "1", Trail: w.Badge{Label: "ro"}},
						w.TickRow{
							Ticked:  w.TickedOff,
							Mark:    w.Mark{Colour: "#8C6239"},
							Label:   "Рассылки",
							Meta:    "—",
							Current: true,
							Hidden:  true,
						},
					},
				},
				w.RailSec{
					Items: []w.RailSecPart{
						w.Rubric{Text: "Tags"}},
					Tags: []w.Tag{w.Tag{Label: "all", On: true}, w.Tag{Label: "работа"}, w.Tag{Label: "издательство"}},
				},
				w.RailFoot{
					Items: []w.RailFootPart{
						w.Button{Icon: "plus", Title: "New address book"},
						w.Button{Icon: "server", Title: "Connect account"},
						w.Button{Icon: "link", Title: "Subscribe to a feed"},
						w.Emblem{Symbol: "davloom", Title: "davloom · running · 3 devices"},
					},
					About: &w.About{Name: "Carrel", Build: w.Mono{Text: "0.0.1 · 786949a"}, Href: "#rest"},
				},
			},
			Main: w.Main{
				Items: []w.MainPart{
					w.Head{
						Title: &w.HeadTitle{Mark: &w.Mark{Colour: "#8C6239", Size: w.MarkH20}, Text: "Рассылки"},
						Subs:  []w.HeadSub{w.HeadSub{Parts: []w.HeadSubPart{w.Text("radicale · vps · 0 cards")}}},
						Acts: &w.Acts{
							Items: []w.ActsPart{
								w.Button{Label: "New contact", Icon: "plus", Kind: w.ButtonPrimary},
								w.Button{Label: "Import", Icon: "up"},
								w.Button{Label: "Export", Icon: "down"},
								w.Button{Label: "Print", Icon: "print"},
							},
						},
					},
					w.Empty{
						Parts: []w.EmptyPart{
							w.Strong("This book is empty."),
							w.Text(" Add a contact, or import a file of them. "),
							w.Button{Label: "New contact", Icon: "plus", Kind: w.ButtonPrimary},
							w.Text(" "),
							w.Button{Label: "Import", Icon: "up"},
						},
					},
				},
			},
		},
	}
}
