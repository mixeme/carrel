// Package conflict is the screen of a save refused because the record
// changed on the server meanwhile, /app/{kind}/{account}/{col}/{uid}/conflict
// (ConflictPage), drawn by frames «Конфликт версий» of a contact, an event, a
// task and a note (s-conflict-1, -3, -4, -5; U-6.8): one card a property,
// its versions a column of radios — not a table of three columns; and by
// its states (s-conflict-6…-11: whose edit is not known, deleted on the
// server, refused, the write refused, unavailable, a note still to move).
package conflict

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

var (
	top = bind.TopWords{
		Brand:  w.Brand{Logo: &w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63"}},
		Search: "Search everything…",
	}
	rail = bind.RailWords{Toggle: "toggle", None: "—", ReadOnly: "ro", Tags: "Tags", Davloom: "davloom", External: "external"}
)

// The screen's words (U-1.3): the title, what a property alike on both
// sides is, how a property differs and what its versions are called, and
// what stands for nothing.
const (
	title    = "Conflict"
	agrees   = "agrees — kept as it is."
	yours    = "Your edit"
	yourForm = "Your form"
	server   = "Server"
	include  = " · include"
	leave    = "Leave it out"
	writeIt  = " · write it back"
	leaveIt  = " · leave it deleted"
	nothing  = "—"
	formID   = "conflict"
	back     = "Back"
)

// NoticeSpace is where the screen's notices stand: messages of its dialog,
// flush with its sides (s-conflict-8…-11).
const NoticeSpace = w.MsgFlushSides

// asides are the words of a kind of difference (C-8 ConflictDiff,
// s-conflict-1…-5); an unknown kind has none.
var asides = map[view.ConflictDiff]string{
	view.DiffBoth:   "both changed",
	view.DiffMine:   "only in your edit",
	view.DiffTheirs: "only on the server",
}

// Screen draws the conflict:
//
//   - the head: «Conflict», what happened under it (Subtitle) and, where
//     nothing is chosen beforehand, what to do (Ask);
//   - the notices of the page as messages of the dialog, flush with its
//     sides, each with its technical block under it (a combination that
//     cannot be written, the write refused, the source unavailable, a note
//     still to move with its notebook's mark);
//   - each property the person changed as the server holds it already, a
//     line (Agree);
//   - the cards, a form (Form): the property by its own name, monospaced,
//     how it differs, and its two versions as radios of its row's name —
//     «mine» the edit, «theirs» the server's —, the chosen one on (Choice);
//     a card with nothing chosen is required. A version only one side holds
//     is «… · include» against «Leave it out»; where whose edit it is cannot
//     be told, the edit is «Your form»; a record deleted on the server is
//     one card by its name, «Your edit · write it back» against «Server ·
//     leave it deleted»; nothing is a dash;
//   - the foot: Write this combination (the form's; waiting while a card
//     has no side), Use only my edit and Use only the server version (each
//     a form of its own), Cancel.
func Screen(p *view.ConflictPage) w.Shell {
	items := []w.DialogPart{w.Head{Title: &w.HeadTitle{Text: title}, Subs: subs(p.Subtitle, p.Ask)}}
	for _, n := range p.Notices {
		items = append(items, bind.Notice(n, NoticeSpace))
		if d := bind.NoticeDiag(n); d != nil {
			items = append(items, d.Diag)
		}
	}
	for _, a := range p.Agree {
		items = append(items, w.DiffAgree{Property: w.Mono{Text: string(a)}, Text: agrees})
	}
	if p.Form != nil {
		diff := w.Diff{Action: p.Form.Action, Method: p.Form.Method, Fields: bind.Hidden(*p.Form), ID: formID}
		for _, r := range p.Rows {
			diff.Items = append(diff.Items, card(r, p.Deleted))
		}
		items = append(items, diff)
	}
	items = append(items, foot(p))
	t := bind.Top(p.Top, top)
	return w.Shell{Top: &t, Body: w.ShellBody{Rail: railParts(p.Rail), Main: w.Main{Items: []w.MainPart{w.Dialog{Items: items}}}}}
}

// subs are the lines under the title — what happened, then what to do —;
// none for what is not said.
func subs(lines ...view.Text) []w.HeadSub {
	var out []w.HeadSub
	for _, t := range lines {
		if t != "" {
			out = append(out, w.HeadSub{Parts: []w.HeadSubPart{w.Text(t)}})
		}
	}
	return out
}

// card is a property's card: its name, how it differs, its two versions; of
// a record deleted on the server (deleted), the record by its name.
func card(r view.ConflictRow, deleted bool) w.DiffItem {
	t := w.Rubric{Lead: &w.Mono{Text: string(r.Property)}}
	if a := asides[r.Diff]; a != "" {
		t.Items = []w.RubricPart{w.RubricAside{Text: a}}
	}
	mineWord, theirsWord := yours, server
	switch {
	case deleted:
		t = w.Rubric{Text: string(r.Property)}
		mineWord, theirsWord = yours+writeIt, server+leaveIt
	case r.Diff == view.DiffMine:
		mineWord, theirsWord = yours+include, leave
	case r.Diff == view.DiffTheirs:
		theirsWord = server + include
	case r.Diff == view.DiffUnknown:
		// Whose change it is Carrel cannot tell: the form may hold fields
		// the person did not touch (s-conflict-6; the owner, 17.10.2026).
		mineWord = yourForm
	}
	// A card whose change Carrel cannot tell as anyone's has neither on
	// (ChoiceUnset, the owner's decision of 16.10.2026), and the form is not
	// sent without its side (Check.Required, Typecase's 198).
	need := r.Choice == view.ChoiceUnset
	return w.DiffItem{Title: t, Options: []w.Check{
		version(r.Name, string(view.ChoiceMine), r.Choice == view.ChoiceMine, need, mineWord, r.Mine),
		version(r.Name, string(view.ChoiceTheirs), r.Choice == view.ChoiceTheirs, need, theirsWord, r.Theirs),
	}}
}

// version is one side of a card: a radio of the row's name.
func version(name, value string, on, required bool, source string, v view.Text) w.Check {
	shown := string(v)
	if shown == "" {
		shown = nothing
	}
	return w.Check{Name: name, Value: value, Radio: true, On: on, Required: required, Parts: []w.CheckPart{w.Choice{Source: source, Value: shown}}}
}

// foot is the row of answers: the form's own first, then each action — a
// form of its own when it posts.
func foot(p *view.ConflictPage) w.FormFoot {
	f := w.FormFoot{Variant: w.FormFootWrap}
	if p.Form != nil {
		for _, a := range p.Form.Actions {
			f.Items = append(f.Items, bind.Button(a, formID))
		}
	}
	for _, a := range p.Actions {
		f.Items = append(f.Items, bind.Button(a, a.ID))
	}
	return f
}

// Forms are the forms of the answers that post with none of their own: Use
// only my edit, Use only the server version; and the top's.
func Forms(p *view.ConflictPage) []w.PostForm {
	out := append(bind.TopForms(p.Top), bind.NoticeForms(p.Notices)...)
	for _, a := range p.Actions {
		if a.Method == "POST" && a.Enabled {
			out = append(out, bind.ActionForm(a, a.ID))
		}
	}
	return out
}

// railParts is the record's rail: Back to list, the collections of the
// section, the foot.
func railParts(r view.Rail) []w.RailPart {
	nav := w.Nav{}
	for _, n := range r.Nav {
		it := bind.NavItem(n)
		it.Short = back
		nav.Items = append(nav.Items, it)
	}
	parts := []w.RailPart{nav}
	parts = append(parts, bind.Sources(r, rail)...)
	return append(parts, bind.Foot(r, rail))
}
