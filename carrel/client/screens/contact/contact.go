// Package contact is the card of a contact, /app/contacts/{account}/{col}/{uid},
// drawn by frame «Контакт · карточка, чтение» (s-contacts-6) of the mockup,
// and after the sheet «rewrite the address» by its outcome's frame
// (s-contacts-16): the drafts frame2go printed for them, with the frames'
// data become the view model's (U-5.1, U-6.1).
package contact

import (
	"strconv"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The IDs of the page's forms: Send and the deletion post with no form of
// their own.
const (
	formSend   = "send"
	formDelete = "delete"
)

// The screen's words (U-1.3) for the shell: the mark, the search, the rail.
var (
	top = bind.TopWords{
		Brand:  w.Brand{Logo: &w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63"}},
		Search: "Search everything…",
	}
	rail = bind.RailWords{Toggle: "toggle", None: "—", ReadOnly: "ro", Tags: "Tags", Davloom: "davloom"}
)

// icons are the icons of a record's kind in the timeline's inset (U-4).
var icons = map[view.Kind]w.Icon{
	view.KindEvent:   "cal",
	view.KindTask:    "task",
	view.KindNote:    "note",
	view.KindFile:    "file",
	view.KindContact: "con",
}

// Screen draws the card from its view model. It is read, not edited (U-6.1).
//
//   - The shell: the top bar; the rail of return — «Back to list», the
//     book's sources, the foot.
//   - The head: the photo (or the initials while there is none), the name,
//     where and who (Subtitle), the book; the actions on view — Edit, Send
//     where a letter can go, Print — and «⋯» with the record's menu (Move,
//     Copy, Export .vcf, Publish a link).
//   - After the sheet «rewrite the address», its outcome (Notices).
//   - The fields: phones and the birthday monospaced, the birthday's «in N
//     months» after it, the categories as tags, a phone or an address going
//     where it goes.
//   - The properties kept as the server sent them, folded, the button that
//     shows them (ForeignShow), and what they are.
//   - The same person in other books: the book's mark, its name leading to
//     the record there, how it differs; their number in the rubric.
//   - The timeline's inset: what the session's timeline found (its number in
//     the rubric), the last records, «Open the timeline».
//   - The source: the record's path, when it was read and whether it changed
//     on the server since.
//   - The deletion at the foot.
func Screen(p *view.ContactPage) w.Shell {
	h := w.Head{Title: &w.HeadTitle{Text: string(p.Name)}}
	if p.PhotoHref != "" {
		// ждёт библиотеку: U-5.5 #11 — the photo in the head (PhotoHref)
		h.Photo = &w.HeadPhoto{}
	} else {
		h.Photo = &w.HeadPhoto{Initials: string(p.Avatar.Initials)}
	}
	if p.Subtitle != "" {
		h.Subs = append(h.Subs, w.HeadSub{Parts: []w.HeadSubPart{w.Text(p.Subtitle)}})
	}
	h.Subs = append(h.Subs, w.HeadSub{Parts: []w.HeadSubPart{w.Text(p.Source.Stamp)}})
	var acts []w.ActsPart
	var danger []w.Button
	for _, a := range p.Actions {
		form := formSend
		if a.Danger {
			form = formDelete
		}
		b := bind.Button(a, form)
		if a.Danger {
			danger = append(danger, b)
			continue
		}
		acts = append(acts, b)
	}
	if len(p.Menu) > 0 {
		// ждёт библиотеку: U-5.5 #1 — the card's «⋯», a closed menu without scripts
		acts = append(acts, w.Drop{
			Trigger: w.Button{Icon: "dots", Title: "More"},
			Menu:    w.Menu{Items: bind.MenuItems(p.Menu, rail.ReadOnly), Width: w.MenuWidth212, Pop: w.MenuUnderEnd},
		})
	}
	h.Acts = &w.Acts{Items: acts}
	main := []w.MainPart{h}
	for _, n := range p.Notices {
		main = append(main, bind.Notice(n, w.MsgSpacePage))
	}
	fields := w.Fields{}
	for _, f := range p.Fields {
		fields.Items = append(fields.Items, bind.FieldsItem(f, "· "))
	}
	main = append(main, fields)
	if p.ForeignShow != nil || len(p.Foreign) > 0 {
		sec := []w.SecPart{w.Rubric{Text: "Kept as the server sent them"}}
		if p.ForeignShow != nil {
			show := bind.Button(*p.ForeignShow, "")
			show.Small = true
			sec[0] = w.Rubric{Text: "Kept as the server sent them", Items: []w.RubricPart{show}}
		} else {
			var lines [][]w.SecItemPart
			for _, f := range p.Foreign {
				lines = append(lines, []w.SecItemPart{w.Mono{Text: string(f.Label)}, w.Text(" "), w.Text(f.Value)})
			}
			sec = append(sec, w.SecList{Items: lines})
		}
		sec = append(sec, w.SecHint{Text: "Properties Carrel does not edit are stored byte for byte and go back untouched."})
		main = append(main, w.Sec{Items: sec})
	}
	if len(p.Books) > 0 {
		var lines [][]w.SecItemPart
		for _, f := range p.Books {
			lines = append(lines, bind.Book(f, "· "))
		}
		main = append(main, w.Sec{Items: []w.SecPart{
			w.Rubric{Text: "The same person in other books", Items: []w.RubricPart{w.Badge{Label: strconv.Itoa(len(p.Books)), Tone: w.BadgeLinked}}},
			w.SecList{Items: lines, Gap: 5},
		}})
	}
	rubric := w.Rubric{Text: "Timeline"}
	if p.TimelineCount != nil {
		rubric.Items = []w.RubricPart{w.Mono{Text: strconv.Itoa(*p.TimelineCount), Variant: w.MonoTight}}
	}
	timeline := []w.SecPart{rubric}
	if len(p.Timeline) > 0 {
		var rows []w.ListRow
		for _, r := range p.Timeline {
			rows = append(rows, w.LinkedRow{
				Mark:   w.Mark{Colour: string(r.Colour)},
				Icon:   icons[r.Kind],
				Title:  string(r.Title),
				Detail: string(r.When) + " · " + string(r.Why),
				Href:   r.Href,
			})
		}
		timeline = append(timeline, w.List{Rows: rows})
	}
	timeline = append(timeline, w.SecActs{Items: []w.Button{bind.Button(p.More, "")}})
	main = append(main, w.Sec{Items: timeline})
	read := string(p.Source.LastRead)
	if p.Source.Compared != "" {
		read += " · " + string(p.Source.Compared)
	}
	main = append(main, w.Sec{Items: []w.SecPart{
		w.Rubric{Text: "Source"},
		bind.Diag(bind.DiagLine{Name: "Path", Value: string(p.Source.Path)}, bind.DiagLine{Name: "Read", Value: read}),
	}})
	for _, b := range danger {
		main = append(main, w.Danger{Button: b})
	}
	t := bind.Top(p.Top, top)
	return w.Shell{Top: &t, Body: w.ShellBody{Rail: railParts(p.Rail), Main: w.Main{Items: main}}}
}

// Forms are the forms of the actions that post with none of their own: Send
// and the deletion.
func Forms(p *view.ContactPage) []w.PostForm {
	out := bind.NoticeForms(p.Notices)
	for _, a := range p.Actions {
		if a.Method != "POST" || !a.Enabled {
			continue
		}
		id := formSend
		if a.Danger {
			id = formDelete
		}
		out = append(out, bind.ActionForm(a, id))
	}
	return out
}

// railParts is the rail of return: «Back to list», the book's sources, the
// foot.
func railParts(r view.Rail) []w.RailPart {
	nav := w.Nav{}
	for _, n := range r.Nav {
		it := bind.NavItem(n)
		it.Short = "Back"
		nav.Items = append(nav.Items, it)
	}
	parts := []w.RailPart{nav}
	parts = append(parts, bind.Sources(r, rail)...)
	return append(parts, bind.Foot(r, rail))
}
