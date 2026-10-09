// Drafted by frame2go from frame «Контакты · ни одна книга не отмечена» (states-7) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// States7 is frame «Контакты · ни одна книга не отмечена» (states-7).
func States7() interface{ Render() string } {
	return w.Shell{
		Top: &w.Top{
			Brand:  w.Brand{Logo: &w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63"}},
			Search: &w.Search{Query: w.Query{Placeholder: "Search everything…", Label: "Search everything…"}},
			Actions: []w.TopAction{
				w.Button{Label: "New note", Menu: true, Icon: "quill", Kind: w.ButtonPrimary},
				w.Button{Label: "Refresh", Icon: "refresh", Title: "Nothing is ticked to ask.", State: w.ButtonDisabled},
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
							Ticked:  w.TickedOff,
							Kind:    w.TickRowAll,
							Mark:    w.Mark{Kind: w.MarkMulti},
							Label:   "Combined address book",
							Meta:    "—",
							Current: true,
						},
					},
				},
				w.RailSec{
					Items: []w.RailSecPart{
						w.Rubric{Text: "nextcloud"},
						w.TickRow{Ticked: w.TickedOff, Mark: w.Mark{Colour: "#4A6B52"}, Label: "Контакты", Meta: "3", Hidden: true},
						w.TickRow{Ticked: w.TickedOff, Mark: w.Mark{Colour: "#A05A2C"}, Label: "Архив", Meta: "1", Hidden: true},
					},
				},
				w.RailSec{
					Items: []w.RailSecPart{
						w.Rubric{Text: "radicale · vps"},
						w.TickRow{
							Ticked: w.TickedOff,
							Mark:   w.Mark{Colour: "#6B4E7A"},
							Label:  "Команда",
							Meta:   "1",
							Hidden: true,
							Trail:  w.Badge{Label: "ro"},
						},
						w.TickRow{Ticked: w.TickedOff, Mark: w.Mark{Colour: "#8C6239"}, Label: "Рассылки", Meta: "—", Hidden: true},
					},
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
						Title: &w.HeadTitle{Mark: &w.Mark{Kind: w.MarkMulti, Size: w.MarkH20}, Text: "Combined address book"},
						Subs:  []w.HeadSub{w.HeadSub{Parts: []w.HeadSubPart{w.Text("0 of 4 books · 0 accounts")}}},
						Acts: &w.Acts{
							Items: []w.ActsPart{
								w.Button{Label: "New contact", Menu: true, Icon: "plus", Kind: w.ButtonPrimary},
								w.Button{Label: "Import", Icon: "up"},
								w.Button{Label: "Export", Icon: "down"},
								w.Button{Label: "Print", Icon: "print"},
							},
						},
					},
					w.Empty{
						Parts: []w.EmptyPart{
							w.Strong("No books are ticked."),
							w.Text(" Tick the books to show in the rail, or all of them at once. "),
							w.Button{Label: "Tick every book", Kind: w.ButtonPrimary},
						},
					},
				},
			},
		},
	}
}
