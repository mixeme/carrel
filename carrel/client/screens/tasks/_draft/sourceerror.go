// Drafted by frame2go from frame «Список задач · сервер не ответил» (states-22) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// States22 is frame «Список задач · сервер не ответил» (states-22).
func States22() interface{ Render() string } {
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
					On: 3,
				},
				w.RailSec{
					Items: []w.RailSecPart{
						w.TickRow{
							Ticked: w.TickedPart,
							Kind:   w.TickRowAll,
							Mark:   w.Mark{Kind: w.MarkMulti},
							Label:  "Combined list",
							Meta:   "2",
						},
					},
				},
				w.RailSec{
					Items: []w.RailSecPart{
						w.Rubric{Text: "nextcloud"},
						w.TickRow{Mark: w.Mark{Colour: "#4A6B52"}, Label: "Дела", Current: true, Error: true},
						w.TickRow{Mark: w.Mark{Colour: "#3F6E8C"}, Label: "Работа", Meta: "2"},
					},
				},
				w.RailSec{
					Items: []w.RailSecPart{
						w.Rubric{Text: "Subscriptions", Items: []w.RubricPart{w.Badge{Label: "external"}}},
						w.TickRow{
							Ticked: w.TickedOff,
							Kind:   w.TickRowExternal,
							Mark:   w.Mark{Colour: "#8C6239"},
							Label:  "Праздники РФ",
							Meta:   "—",
							Hidden: true,
							Trail:  w.Badge{Label: "ro"},
						},
					},
				},
				w.RailSec{
					Items: []w.RailSecPart{
						w.Rubric{Text: "Tags"}},
					Tags: []w.Tag{w.Tag{Label: "all", On: true}, w.Tag{Label: "дом"}, w.Tag{Label: "релиз"}, w.Tag{Label: "read"}},
				},
				w.RailFoot{
					Items: []w.RailFootPart{
						w.Button{Icon: "plus", Title: "New list"},
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
						Title: &w.HeadTitle{Mark: &w.Mark{Colour: "#4A6B52", Size: w.MarkH20}, Text: "Дела"},
						Subs:  []w.HeadSub{w.HeadSub{Parts: []w.HeadSubPart{w.Text("nextcloud")}}},
						Acts: &w.Acts{
							Items: []w.ActsPart{
								w.Button{Label: "New task", Icon: "plus", Kind: w.ButtonPrimary},
								w.Button{Label: "Import", Icon: "up"},
								w.Button{Label: "Export", Icon: "down"},
								w.Button{Label: "Print", Icon: "print"},
							},
						},
					},
					w.Msg{
						Parts: []w.MsgPart{
							w.Strong("Дела is unavailable."),
							w.Text(" The server did not answer as expected, so nothing on this screen comes from it. "),
							w.Button{Label: "Try again", Small: true},
						},
						Tone: w.MsgAlert,
					},
					w.MainDiag{
						Diag: w.Diag{Text: "REPORT /remote.php/dav/calendars/mix/dela/\nnextcloud\ndav: HTTP 503: the server is busy"},
					},
				},
			},
		},
	}
}
