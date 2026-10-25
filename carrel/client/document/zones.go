package document

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/client/screens"
	"carrel.dev/carrel/server/render"
	"carrel.dev/typecase/widget"
)

// The zones a link or a form may ask for: the ones whose answer the live
// layer puts in place — the page, the rail, the panel and the sheet, whose
// answers carry the forms their buttons submit after the zone's widget
// (Typecase's note 124: the layer puts each in place of the page's form of
// that ID). Poll and queue go the plain way: the events of the run's stream
// are not pages yet, and the end of a run brings the rail without the bar.
func init() {
	bind.Zones = []string{render.TargetMain, render.TargetRail, render.TargetDetails, render.TargetSheet}
}

// Zoned is a screen's root with the zones of C-7.2 named, as the library's
// live layer finds them (Typecase's task 84; the client's stage 4): the top
// bar is topbar, the rail rail, the page main, the panel details, a sheet
// sheet. A page with no panel open carries it empty and hidden, so that an
// answer has where to go — the frames, drawn without the layer, have none;
// a page of the shell with no sheet over it carries the sheet's place over
// the whole shell, closed (Shell.Sheet; Typecase's note 124, shell-1). The
// names are the server's (render), not the client's: a screen does not know
// them.
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
		shell.Sheet = &EmptySheet
	case widget.Overlay:
		body.Scrim.Zone = render.TargetSheet
		shell.Body = body
	}
	return shell
}

// EmptyDetails is the zone of a panel that is not open: hidden and empty,
// the place an answer goes.
var EmptyDetails = widget.Side{Zone: render.TargetDetails, Closed: true}

// EmptySheet is the place of a sheet over a page of the shell that has none
// open: closed, dimming the whole shell once an answer fills it.
var EmptySheet = widget.Scrim{Zone: render.TargetSheet, Closed: true}
