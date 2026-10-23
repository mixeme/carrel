package duplicates

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The merge's words (U-1.3): the rubrics of its plan (s-dup-2, -4) and of
// what happened (s-dup-5).
const (
	staysRubric   = "Where the merged record stays"
	deletedRubric = "Deleted afterwards"
	gainsRubric   = "The record that stays gains"
	notCarried    = "Not carried over"
	happened      = "What happened"
)

// Merge draws a merge on the server from its view model: «Merge on the
// server», whom it is of, and in a dialog (s-dup-2, -4, -5; from-typecase
// note 117 p. 3: it asks nothing, the card did):
//
//   - the warning of what it does — said, not alarmed; what went wrong,
//     alarmed;
//   - where the merged record stays: the sources of the group as a
//     segment of links, the record rewritten there;
//   - what is deleted afterwards, what the record that stays gains and what
//     is not carried over — the plan, which is the barrier;
//   - what happened, when a merge stopped halfway;
//   - its buttons: write the merge, Cancel; after it, the kept record and
//     the way back.
func Merge(p *view.DuplicatesMergePage) w.Shell {
	t := bind.Top(p.Top, top)
	h := w.Head{Title: &w.HeadTitle{Text: string(p.Title)}}
	if p.Subtitle != "" {
		h.Subs = []w.HeadSub{{Parts: []w.HeadSubPart{w.Text(p.Subtitle)}}}
	}
	dialog := []w.DialogPart{h}
	if p.Warning.Title != "" || p.Warning.Body != "" {
		dialog = append(dialog, bind.Msg(p.Warning, w.MsgFlushSides))
	}
	if len(p.Stays) > 0 {
		seg := w.Seg{}
		for _, a := range p.Stays {
			seg.Options = append(seg.Options, bind.SegOption(a))
		}
		stays := w.Card{Items: []w.CardPart{w.Rubric{Text: staysRubric}, seg}, Variant: w.CardApart}
		if p.StaysNote != "" {
			stays.Items = append(stays.Items, w.CardText{Kind: w.CardLine, Parts: []w.CardTextPart{w.Dim{Parts: []w.Inline{w.Text(p.StaysNote)}}}})
		}
		dialog = append(dialog, stays)
	}
	if len(p.Deleted) > 0 {
		gone := w.Card{Items: []w.CardPart{w.Rubric{Text: deletedRubric}}}
		for _, d := range p.Deleted {
			gone.Items = append(gone.Items, w.CardText{Kind: w.CardLine, Parts: []w.CardTextPart{
				w.Text(string(d.Title) + " "), w.Dim{Parts: []w.Inline{w.Text(sep + string(d.Stamp))}},
			}})
		}
		dialog = append(dialog, gone)
	}
	for _, block := range []struct {
		rubric string
		lines  []view.Text
		apart  bool
	}{{gainsRubric, p.Gains, false}, {notCarried, p.NotCarried, false}, {happened, p.Happened, true}} {
		if len(block.lines) == 0 {
			continue
		}
		list := w.CardList{}
		for _, l := range block.lines {
			list.Items = append(list.Items, string(l))
		}
		c := w.Card{Items: []w.CardPart{w.Rubric{Text: block.rubric}, list}}
		if block.apart {
			c.Variant = w.CardApart
		}
		dialog = append(dialog, c)
	}
	foot := w.FormFoot{Variant: w.FormFootNoRule}
	for _, a := range p.Actions {
		foot.Items = append(foot.Items, bind.Button(a, a.ID))
	}
	dialog = append(dialog, foot)
	main := append(bind.Notices(p.Notices, w.MsgSpacePage), w.Dialog{Items: dialog})
	return w.Shell{Top: &t, Body: w.ShellBody{Rail: railParts(p.Rail), Main: w.Main{Items: main}}}
}

// MergeForms are the merge's forms: the top bar's, the rail's foot's, the
// one of each button that posts — Write the merge.
func MergeForms(p *view.DuplicatesMergePage) []w.PostForm {
	out := bind.TopForms(p.Top)
	out = append(out, bind.FootForms(p.Rail)...)
	for _, a := range p.Actions {
		if a.Method == "POST" && a.Enabled {
			out = append(out, bind.ActionForm(a, a.ID))
		}
	}
	return append(out, bind.NoticeForms(p.Notices)...)
}
