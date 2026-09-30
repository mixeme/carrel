// Package about is the page about the program, /about, drawn by frame «О
// программе» (rest-3) of the mockup: the draft frame2go printed for it, with
// the frame's data become the view model's (U-5.1, U-6.14).
package about

import (
	"carrel.dev/carrel/client/bind"
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
	source, third := bind.Button(p.Source, ""), bind.Button(p.Third, "")
	source.Small, third.Small = true, true
	return w.Page{
		Items: []w.PagePart{
			w.Auth{
				Dialog: w.Dialog{
					Items: []w.DialogPart{
						w.DialogLogo{Logo: w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63", Size: w.Logo42}, Wide: true},
						w.DialogTitle{Text: string(p.Title)},
						w.DialogSub{Parts: []w.Inline{w.Mono{Text: string(p.Version) + " · " + string(p.Commit)}}},
						w.Tagline{Lead: bind.Words(p.Lede), Text: string(p.Tagline)},
						w.Prose{
							Parts:   []w.Inline{w.Text(p.Licence)},
							Variant: w.ProseMuted,
						},
						w.FormFoot{
							Items: []w.FormFootPart{
								source,
								third,
								w.Push{Items: []w.PushPart{bind.Button(p.Back, "")}},
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
