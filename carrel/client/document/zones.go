package document

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/client/screens"
	"carrel.dev/carrel/server/render"
	"carrel.dev/typecase/widget"
)

// The zones every page of the client has a place for (Zoned), the ones a
// link or a form may ask for; poll and queue follow (stage 4).
func init() {
	bind.Zones = []string{render.TargetMain, render.TargetRail, render.TargetDetails, render.TargetSheet, render.TargetTopbar}
}

// Zoned is a screen's root with the zones of C-7.2 named, as the library's
// live layer finds them (Typecase's task 84; the client's stage 4): the top
// bar is topbar, the rail rail, the page main, the panel details, a sheet
// sheet. A page with no panel open and no sheet carries both empty and
// hidden, so that a link that asks for them has where to put the answer —
// the frames, drawn without the layer, have none of them. The names are the
// server's (render), not the client's: a screen does not know them.
func Zoned(root screens.Root) screens.Root {
	if alone, ok := root.(widget.Overlay); ok {
		// A sheet answered alone (a fragment): its scrim is the zone.
		alone.Scrim.Zone = render.TargetSheet
		return alone
	}
	shell, ok := root.(widget.Shell)
	if !ok {
		return root
	}
	if shell.Top != nil {
		top := *shell.Top
		top.Zone = render.TargetTopbar
		shell.Top = &top
	}
	switch body := shell.Body.(type) {
	case widget.ShellBody:
		body.Main.Zone = render.TargetMain
		if len(body.Rail) > 0 {
			body.RailZoneName = render.TargetRail
		}
		if body.Side == nil {
			body.Side = &EmptyDetails
		} else {
			side := *body.Side
			side.Zone = render.TargetDetails
			body.Side = &side
		}
		body.Main.Items = append(append([]widget.MainPart(nil), body.Main.Items...), EmptySheet)
		shell.Body = body
	case widget.Overlay:
		body.Scrim.Zone = render.TargetSheet
		shell.Body = body
	}
	return shell
}

// EmptyDetails and EmptySheet are the zones of a panel and a sheet that are
// not open: hidden and empty, the places an answer goes.
var (
	EmptyDetails = widget.Side{Zone: render.TargetDetails, Closed: true}
	EmptySheet   = widget.Scrim{Zone: render.TargetSheet, Closed: true}
)
