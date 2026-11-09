// Package published is the list of the links the person has out,
// /app/files/published (PublishedLinksPage), drawn by frame «Выданные
// ссылки» (s-publish-1) of the mockup and its states — none out
// (s-publish-18), no mail (s-publish-20; its rows — the fragment s-publish-17),
// a row whose thing is gone (the fragment s-publish-21).
package published

import (
	"strconv"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

var top = bind.TopWords{
	Brand:  w.Brand{Logo: &w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63"}},
	Search: "Search everything…",
	NewKey: "n", SearchKey: "/",
	Theme: bind.ThemeWords{Label: "Theme", Auto: "Auto", Light: "Light", Dark: "Dark"},
	Keys:  bind.KeysWords{Label: "Keyboard shortcuts", Icon: "search", Key: "?", Sheet: "keys"},
}

// The screen's words (U-1.3): the rubric of the rail, the emblem of
// davloom, the badge of a local instance and its hint, the heads of the
// columns, what Copy says once it copied, «⋯», what joins a link gone to
// its words.
const (
	settings   = "Settings"
	davloom    = "davloom"
	local      = "local"
	localHint  = "Local instance: may be unreachable from outside"
	colWhat    = "What is published"
	colKind    = "Kind"
	colCreated = "Created"
	colExpires = "Expires"
	colSeen    = "Last opened"
	colOpens   = "Opens"
	copied     = "Copied"
	more       = "More"
	goneWord   = "no longer there"
)

// The names of the contract the screen places by (C-7.5): the rail's way
// back, a row's Copy, Send and Revoke.
const (
	backID   = "settings.back"
	copyID   = "publish.copy"
	revokeID = "publish.revoke"
	// idField is what Revoke sends the list's one form with: the row's
	// link (PublishedRow.ID, s-publish-1).
	idField = "id"
)

// revokeForm is the ID of the form a row's Revoke sends — one for the list,
// as every row's Revoke goes to the same address; empty where it is off.
func revokeForm(r view.PublishedRow) string {
	for _, a := range r.Actions {
		if a.ID == revokeID && a.Enabled && a.Method == "POST" {
			return bind.FormID(a)
		}
	}
	return ""
}

// Screen draws the list from its view model, in the state the model is in.
//
//   - The shell: the top bar; the settings' rail — «Back to app» alone,
//     «Settings» over its pages (Published links on), the state of things
//     at the foot.
//   - The head: «Published links», the badge of a local instance, what a
//     link is (Subtitle; on a local instance that it does not leave the
//     network, in alarm).
//   - The table: a row a link — what it publishes by its icon or its
//     collection's mark, leading there (none for a link whose thing is
//     gone); its kind, when made, when it ends, when last opened (muted
//     never), how often, monospaced; Copy, which copies the whole address,
//     Send where mail goes, Revoke, and «⋯» with their twins as the table
//     narrows. A link whose thing is gone (Gone) is a row switched off: its
//     name no link, «no longer there» beside it with Gone for its hint, no
//     term muted, Copy and Send off with their reason, Revoke as any row's.
//   - No link out: what and where one is made (Empty).
func Screen(p *view.PublishedLinksPage) w.Shell {
	t := &w.HeadTitle{Text: string(p.Title)}
	if p.Top.Signed.Local {
		t.Badge = &w.Badge{Label: local, Tone: w.BadgeLocal, Title: localHint}
	}
	h := w.Head{Title: t}
	if len(p.Subtitle) > 0 {
		h.Subs = bind.Sub(p.Subtitle)
	}
	main := []w.MainPart{h}
	main = append(main, bind.Notices(p.Notices, w.MsgSpacePage)...)
	switch {
	case p.Empty != nil:
		main = append(main, w.Empty{Parts: bind.EmptyParts(*p.Empty)})
	default:
		main = append(main, table(p.Rows))
	}
	tb := bind.Top(p.Top, top)
	return w.Shell{Top: &tb, Body: w.ShellBody{Rail: bind.BackRail(p.Rail, backID, settings, davloom), Main: w.Main{Items: main}}}
}

// table is the links, a row each.
func table(rows []view.PublishedRow) w.Table {
	t := w.Table{Variant: w.TablePacked, Cols: []w.TableCol{
		{Label: colWhat},
		{Label: colKind, Rank: w.Rank1},
		{Label: colCreated, Rank: w.Rank3, Secondary: true},
		{Label: colExpires, Rank: w.Rank4, Main: true},
		{Label: colSeen, Secondary: true},
		{Label: colOpens},
		{Width: w.Col120},
	}}
	for _, r := range rows {
		name := w.TableFile{Name: string(r.Title), Href: r.Href}
		switch {
		case r.Icon != "":
			name.Icon = bind.Icon(r.Icon)
		case r.Colour != "":
			name.Mark = &w.Mark{Colour: string(r.Colour)}
		}
		seen := w.TableCell{Parts: []w.CellPart{w.Text(r.LastSeen)}, Mono: true}
		if r.Unopened {
			seen.Tone = w.CellDusty
		}
		what := []w.CellPart{name}
		state := w.RowPlain
		expires := w.TableCell{Parts: []w.CellPart{w.Text(r.Expires)}, Mono: true}
		if r.Missing != "" {
			// What the last guest was not given, why in the hint (s-publish-40).
			what = append(what, w.Badge{Label: string(r.Missing), Title: string(r.MissingHint)})
		}
		if r.Gone != "" {
			// No term of its own: «—» muted (s-publish-21).
			what, state = append(what, w.Badge{Label: goneWord, Tone: w.BadgeAlert, Title: string(r.Gone)}), w.RowOff
			expires.Tone = w.CellDusty
		}
		t.Rows = append(t.Rows, w.TableRow{State: state, Cells: []w.TableCell{
			{Parts: what},
			{Parts: []w.CellPart{w.Text(r.Kind)}, Mono: true},
			{Parts: []w.CellPart{w.Text(r.Created)}, Mono: true},
			expires,
			seen,
			{Parts: []w.CellPart{w.Text(strconv.Itoa(r.Opens))}, Mono: true},
			acts(r),
		}})
	}
	return t
}

// acts are a row's buttons: Copy — the whole address, its words once copied
// —, Send, Revoke, the last two going first as the table narrows, and «⋯»
// with their twins.
func acts(r view.PublishedRow) w.TableCell {
	c := w.TableCell{Acts: true}
	var twins []view.MenuItem
	for _, a := range r.Actions {
		b := bind.Button(a, bind.FormID(a))
		b.Small = true
		switch a.ID {
		case copyID:
			// Copy takes the address whole (Пб9); without a script it is the
			// link itself.
			b.Copy, b.Copied = string(r.Address), copied
		default:
			b.Secondary = true
			if a.ID == revokeID {
				// Revoke breaks a link and keeps the data (s-publish-1):
				// it sends the list's one form with its row's link.
				b.Kind = w.ButtonDanger
				if a.Enabled {
					b.Name, b.Value = idField, r.ID
				}
			}
			twins = append(twins, view.MenuItem{Action: a, Separator: a.ID == revokeID && len(twins) > 0})
		}
		c.Parts = append(c.Parts, b)
	}
	if len(twins) > 0 {
		items := bind.MenuItemsAt(twins, "", bind.FormID)
		for i, it := range items {
			// Revoke's twin sends the same form with the same link.
			if m, ok := it.(w.MenuItem); ok && m.Form != "" && m.Form == revokeForm(r) {
				m.Name, m.Value = idField, r.ID
				items[i] = m
			}
		}
		c.Drop = &w.Drop{
			Trigger: w.Button{Icon: "dots", Small: true, Variant: w.ButtonPadX3, Title: more},
			Row:     true,
			Menu:    w.Menu{Items: items, Width: w.MenuWidth212, Pop: w.MenuUnderEnd},
		}
	}
	return c
}

// Forms are the forms of the list's buttons that post (Send, Revoke), each
// by its action and address, and the top bar's.
func Forms(p *view.PublishedLinksPage) []w.PostForm {
	out := bind.TopForms(p.Top)
	seen := map[string]bool{}
	for _, r := range p.Rows {
		for _, a := range r.Actions {
			// Every row's Revoke goes to the one address: one form.
			if id := bind.FormID(a); a.Method == "POST" && a.Enabled && !seen[id] {
				seen[id] = true
				out = append(out, bind.ActionForm(a, id))
			}
		}
	}
	return append(out, bind.NoticeForms(p.Notices)...)
}
