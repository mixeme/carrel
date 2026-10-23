package document

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/client/screens"
	"carrel.dev/carrel/server/render"
	"carrel.dev/typecase/widget"
)

// The zones a link or a form may ask for: the ones whose answer the live
// layer puts in place. A panel's and a sheet's answer carries the forms
// its buttons submit after it, and a sheet's zone has no place over the
// whole shell — they wait for Typecase's task 226 and go the plain way;
// poll and queue too.
func init() {
	bind.Zones = []string{render.TargetMain, render.TargetRail}
}

// Zoned is a screen's root with the zones of C-7.2 named, as the library's
// live layer finds them (Typecase's task 84; the client's stage 4): the top
// bar is topbar, the rail rail, the page main, the panel details, a sheet
// sheet. A page with no panel open carries it empty and hidden, so that an
// answer has where to go — the frames, drawn without the layer, have none;
// an empty sheet has no place yet (Typecase's task 226). The names are the
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
		shell.Body = body
	case widget.Overlay:
		body.Scrim.Zone = render.TargetSheet
		shell.Body = body
	}
	return shell
}

// EmptyDetails is the zone of a panel that is not open: hidden and empty,
// the place an answer goes.
var EmptyDetails = widget.Side{Zone: render.TargetDetails, Closed: true}
