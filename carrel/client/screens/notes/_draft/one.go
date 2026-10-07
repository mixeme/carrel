// Drafted by frame2go from frame «Заметки · одна тетрадь» (s-notes-31) of the Typecase mockup: the
// frame's widgets and its data, top to bottom. The data is the frame's; it
// is to become the application's (docs/guide/screen.md).
package draft

import w "carrel.dev/typecase/widget"

// Notes31 is frame «Заметки · одна тетрадь» (s-notes-31).
func Notes31() interface{ Render() string } {
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
						{Icon: "con", Label: "Contacts", Count: &w.Num{Value: 312}, Href: "#"},
						{Icon: "cal", Label: "Calendar", Count: &w.Num{Value: 14}, Href: "#"},
						{Icon: "task", Label: "Tasks", Count: &w.Num{Value: 3}, Href: "#"},
						{Icon: "note", Label: "Notes", Count: &w.Num{Value: 86}, Href: "#"},
						{Icon: "file", Label: "Files", Href: "#"},
						{Icon: "dup", Label: "Duplicates", Count: &w.Num{Value: 3}, Href: "#"},
					},
					On: 4,
				},
				w.RailSec{
					Items: []w.RailSecPart{
						w.TickRow{
							Ticked: w.TickedPart,
							Kind:   w.TickRowAll,
							Mark:   w.Mark{Kind: w.MarkMulti},
							Label:  "Combined notebook",
							Meta:   "86",
						},
					},
				},
				w.RailSec{
					Items: []w.RailSecPart{
						w.Rubric{Text: "nextcloud"},
						w.TickRow{Mark: w.Mark{Colour: "#4A6B52"}, Label: "Записи", Meta: "64", Current: true},
						w.TickRow{Mark: w.Mark{Colour: "#6B4E7A"}, Label: "Дневник", Meta: "22"},
						w.TickRow{
							Ticked: w.TickedOff,
							Mark:   w.Mark{Colour: "#3F6E8C"},
							Label:  "Общие с семьёй",
							Meta:   "—",
							Hidden: true,
							Trail:  w.Badge{Label: "ro"},
						},
					},
				},
				w.RailSec{
					Items: []w.RailSecPart{
						w.Rubric{Text: "Tags"}},
					Tags: []w.Tag{
						w.Tag{Label: "all", On: true},
						w.Tag{Label: "встречи"},
						w.Tag{Label: "идеи"},
						w.Tag{Label: "книги"},
						w.Tag{Label: "рецепты"},
					},
				},
				w.RailFoot{
					Items: []w.RailFootPart{
						w.Button{Icon: "plus", Title: "New notebook"},
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
						Title: &w.HeadTitle{Mark: &w.Mark{Colour: "#4A6B52", Size: w.MarkH20}, Text: "Записи"},
						Subs:  []w.HeadSub{w.HeadSub{Parts: []w.HeadSubPart{w.Text("nextcloud · 64 notes · VJOURNAL")}}},
						Acts: &w.Acts{
							Items: []w.ActsPart{
								w.Button{Label: "New note", Icon: "plus", Kind: w.ButtonPrimary},
								w.Button{Label: "Import", Icon: "up"},
								w.Button{Label: "Export", Icon: "down"},
							},
						},
					},
					w.Bar{
						Items: []w.BarPart{
							w.Filter{
								Query:   w.Query{Placeholder: "Filter by word", Label: "Filter by word"},
								Waiting: true,
								Variant: w.FilterH26W200,
							},
							w.Seg{Options: []w.SegOption{{Label: "Newest", On: true}, {Label: "Oldest"}, {Label: "Title"}}},
							w.Button{Label: "Columns", Icon: "cols"},
						},
						Right: &w.BarRight{Items: []w.BarPart{w.Mono{Text: "64"}}, Secondary: true},
					},
					w.ListGroup{Title: w.Rubric{Text: "August 2026", Form: w.RubricInline}, Count: &w.Num{Value: 7}},
					w.List{
						Rows: []w.ListRow{
							w.ExcerptRow{
								Mark:    w.Mark{Colour: "#4A6B52"},
								Title:   []w.Inline{w.Text("Заметка со встречи 10.08")},
								Excerpt: "Договорились заморозить ветку в среду; письмо пользователям пишет Анна…",
								Tags:    []w.RowTag{w.Tag{Label: "встречи"}},
								Date:    "10 Aug",
								Source:  "nextcloud · Записи",
							},
							w.ExcerptRow{
								Mark:       w.Mark{Colour: "#4A6B52"},
								Title:      []w.Inline{w.Text("Что почитать про CRDT ")},
								TitleBadge: &w.Badge{Label: "linked ×2", Tone: w.BadgeLinked},
								Excerpt:    "Кляйн, глава 5; статья Шапиро 2011; демка automerge…",
								Tags:       []w.RowTag{w.Tag{Label: "книги"}},
								Date:       "8 Aug",
								Source:     "2 sources",
								SourceHint: "nextcloud · Записи, nextcloud · Дневник",
							},
							w.ExcerptRow{
								Mark:    w.Mark{Colour: "#4A6B52"},
								Title:   []w.Inline{w.Text("Планы на осень")},
								Excerpt: "Дописать руководство, разобрать почту за лето, съездить к родителям…",
								Tags:    []w.RowTag{w.Tag{Label: "идеи"}},
								Date:    "2 Aug",
								Source:  "nextcloud · Записи",
							},
						},
					},
				},
			},
		},
	}
}
