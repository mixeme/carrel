// Package about is the page about the program, /about, drawn by frame «О
// программе» (rest-3) of the mockup: the draft frame2go printed for it, with
// the frame's data become the view model's (U-5.1, U-6.14).
package about

import (
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// Screen draws the page about the program from its view model: the mark and
// the name (Title), the version and the commit, the tagline (Lede and
// Tagline), the line of the licence, and under it the visible link to the
// source of the running version (Source), «Third-party licences» (Third) and
// the way back (Back: to sign-in without a session, where one was from the
// shell).
func Screen(p *view.AboutPage) w.Page {
	return w.Page{
		Items: []w.PagePart{
			w.Auth{
				Dialog: w.Dialog{
					Items: []w.DialogPart{
						w.DialogLogo{Logo: w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63", Size: w.Logo42}, Wide: true},
						w.DialogTitle{Text: string(p.Title)},
						w.DialogSub{Parts: []w.Inline{w.Mono{Text: string(p.Version) + " · " + string(p.Commit)}}},
						w.Tagline{Lead: lead(p.Lede), Text: string(p.Tagline)},
						w.Prose{
							Parts:   []w.Inline{w.Text(p.Licence)},
							Variant: w.ProseMuted,
						},
						w.FormFoot{
							Items: []w.FormFootPart{
								w.Button{Label: string(p.Source.Label), Small: true, Href: p.Source.Href},
								w.Button{Label: string(p.Third.Label), Small: true, Href: p.Third.Href},
								w.Push{Items: []w.PushPart{w.Button{Label: string(p.Back.Label), Href: p.Back.Href}}},
							},
							Variant: w.FormFootFlush,
						},
					},
					Variant: w.DialogNarrow,
				},
			},
		},
	}
}

// lead is the first line of the tagline, the pieces of Lede one after
// another: the tagline marks none of them (C-6.2 Runs).
func lead(r view.Runs) string {
	var s string
	for _, p := range r {
		s += string(p.Text)
	}
	return s
}
