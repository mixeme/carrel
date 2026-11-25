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
// poll's bar and the queue of transfers, whose answers carry the forms
// their buttons submit after the zone's widget (Typecase's note 124: the
// layer puts each in place of the page's form of that ID).
func init() {
	bind.Zones = []string{render.TargetMain, render.TargetRail, render.TargetDetails, render.TargetSheet, render.TargetNotices, render.TargetPoll, render.TargetQueue}
	bind.MainZone = render.TargetMain
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
// (Zoned), the poll's bar the zone poll (withPoll), the queue of transfers
// the zone queue (withQueue), the sheet of the keys closed (withKeys), the
// drop sheet closed (withDrop).
func Root(scr screens.Screen, model any) screens.Root {
	return withDrop(withKeys(withQueue(withPoll(withNotices(Zoned(scr.Draw(model)), model), model))), scr, model)
}

// withNotices is a page's root with its band of notices the zone notices:
// the band the screen draws, or, where none stands, its empty place after
// the head and the bar of the list — the frames draw no empty band
// (Typecase's note 127 p. 1); the poll's bar comes after it. Under a sheet
// the page's band is named and none is added: the sheet is what is open.
func withNotices(root screens.Root, model any) screens.Root {
	shell, ok := root.(widget.Shell)
	if !ok {
		return root
	}
	if over, ok := shell.Body.(widget.Overlay); ok {
		if items, named := namedNotices(over.Body.Main.Items); named {
			over.Body.Main.Items = items
			shell.Body = over
		}
		return shell
	}
	body, ok := shell.Body.(widget.ShellBody)
	if !ok || !zone.Carries(model, render.TargetNotices) {
		return root
	}
	items, named := namedNotices(body.Main.Items)
	if !named {
		lead := 0
		for i, it := range items {
			switch it.(type) {
			case widget.Head, widget.Bar:
				if lead == i {
					lead = i + 1
				}
			}
		}
		items = append(items[:lead], append([]widget.MainPart{EmptyNotices}, items[lead:]...)...)
	}
	body.Main.Items = items
	shell.Body = body
	return shell
}

// namedNotices are a page's parts, a copy, with its band of notices named
// the zone notices, and whether the page has one.
func namedNotices(parts []widget.MainPart) ([]widget.MainPart, bool) {
	items := append([]widget.MainPart(nil), parts...)
	for i, it := range items {
		if n, ok := it.(widget.Notices); ok {
			items[i] = NoticesZone(n)
			return items, true
		}
	}
	return items, false
}

// BandOf is the band of notices a page's root carries (Root), and whether it
// has one: a page whose screen draws its notices in a dialog has none.
func BandOf(root screens.Root) (widget.Notices, bool) {
	shell, ok := root.(widget.Shell)
	if !ok {
		return widget.Notices{}, false
	}
	var items []widget.MainPart
	switch body := shell.Body.(type) {
	case widget.ShellBody:
		items = body.Main.Items
	case widget.Overlay:
		items = body.Body.Main.Items
	}
	for _, it := range items {
		if n, ok := it.(widget.Notices); ok {
			return n, true
		}
	}
	return widget.Notices{}, false
}

// NoticesZone is a band of notices as the zone notices.
func NoticesZone(n widget.Notices) widget.Notices {
	n.Zone = render.TargetNotices
	return n
}

// EmptyNotices is the zone of the notices on a page that has none: hidden
// and empty, the place an answer's notices go.
var EmptyNotices = widget.Notices{Zone: render.TargetNotices}

// withQueue is a page's root with its queue of transfers the zone queue:
// the queue the screen draws on the page, waiting hidden while an upload
// may come (Queue.Progress), or showing what goes — under a sheet too, as
// the page's bar (withPoll).
func withQueue(root screens.Root) screens.Root {
	shell, ok := root.(widget.Shell)
	if !ok {
		return root
	}
	switch body := shell.Body.(type) {
	case widget.ShellBody:
		if items, ok := namedQueue(body.Main.Items); ok {
			body.Main.Items = items
			shell.Body = body
		}
	case widget.Overlay:
		if items, ok := namedQueue(body.Body.Main.Items); ok {
			body.Body.Main.Items = items
			shell.Body = body
		}
	}
	return shell
}

// namedQueue are a page's parts, a copy, with its queue named the zone
// queue, and whether the page has one.
func namedQueue(parts []widget.MainPart) ([]widget.MainPart, bool) {
	items := append([]widget.MainPart(nil), parts...)
	for i, it := range items {
		if q, ok := it.(widget.Queue); ok {
			items[i] = QueueZone(q)
			return items, true
		}
	}
	return items, false
}

// QueueZone is a queue of transfers as the zone queue.
func QueueZone(q widget.Queue) widget.Queue {
	q.Zone = render.TargetQueue
	return q
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
			case widget.Head, widget.Bar, widget.Notices, widget.Msg, widget.MainDiag:
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
