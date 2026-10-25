package document

import (
	"time"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/client/screens"
	"carrel.dev/carrel/client/zone"
	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
	"carrel.dev/typecase/widget"
)

// The zones a link or a form may ask for: the ones whose answer the live
// layer puts in place — the page, the rail, the panel, the sheet and the
// poll's bar, whose answers carry the forms their buttons submit after the
// zone's widget (Typecase's note 124: the layer puts each in place of the
// page's form of that ID). The queue goes the plain way.
func init() {
	bind.Zones = []string{render.TargetMain, render.TargetRail, render.TargetDetails, render.TargetSheet, render.TargetPoll}
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

// PollZone is a poll's bar as the zone poll: named, and while its run goes
// following it — the stream of its task with the events of C-7.4, else its
// poll every Every milliseconds (PollBar.Stream, Poll, Every). A bar of a
// run that is over follows nothing: the stream would only say the end
// again.
func PollZone(bar widget.Runbar, p *view.PollBar) widget.Runbar {
	bar.Zone = render.TargetPoll
	if p == nil || bar.Closed || !p.Running {
		return bar
	}
	bar.Stream, bar.Poll, bar.Every = p.Stream, p.Poll, time.Duration(p.Every)*time.Millisecond
	if p.Stream != "" {
		bar.Changed, bar.Ended = render.EventPoll, render.EventPollFinished
	}
	return bar
}

// EmptyPoll is the zone of a poll's bar on a page whose poll does not run:
// hidden and empty, the place Retry and «Search again» answer into.
var EmptyPoll = PollZone(widget.Runbar{Closed: true}, nil)

// Root is a screen's root as a page prints it: the zones of C-7.2 named
// (Zoned), the poll's bar the zone poll (withPoll).
func Root(scr screens.Screen, model any) screens.Root {
	return withPoll(Zoned(scr.Draw(model)), model)
}

// withPoll is a page's root with its poll's bar the zone poll (PollZone):
// the bar the screen draws, or, where none stands, its empty place after
// the head and what stands with it — the bar of the list, its messages —,
// where a bar stands when there is one (s-agenda-17). Under a sheet the
// page's bar is named too — its Retry names the zone —, and follows
// nothing: the sheet is what is open.
func withPoll(root screens.Root, model any) screens.Root {
	shell, ok := root.(widget.Shell)
	if !ok {
		return root
	}
	if over, ok := shell.Body.(widget.Overlay); ok {
		if items, named := namedPoll(over.Body.Main.Items, nil); named {
			over.Body.Main.Items = items
			shell.Body = over
		}
		return shell
	}
	body, ok := shell.Body.(widget.ShellBody)
	if !ok || !zone.Carries(model, render.TargetPoll) {
		return root
	}
	var p *view.PollBar
	if v, err := zone.Of(model, render.TargetPoll); err == nil {
		p = v.(*view.PollBar)
	}
	items, named := namedPoll(body.Main.Items, p)
	if !named {
		lead := 0
		for i, it := range items {
			switch it.(type) {
			case widget.Head, widget.Bar, widget.Msg, widget.MainDiag:
				if lead == i {
					lead = i + 1
				}
			}
		}
		items = append(items[:lead], append([]widget.MainPart{EmptyPoll}, items[lead:]...)...)
	}
	body.Main.Items = items
	shell.Body = body
	return shell
}

// namedPoll are a page's parts, a copy, with its poll's bar named the zone
// poll, and whether the page has a bar.
func namedPoll(parts []widget.MainPart, p *view.PollBar) ([]widget.MainPart, bool) {
	items := append([]widget.MainPart(nil), parts...)
	for i, it := range items {
		if bar, ok := it.(widget.Runbar); ok {
			items[i] = PollZone(bar, p)
			return items, true
		}
	}
	return items, false
}
