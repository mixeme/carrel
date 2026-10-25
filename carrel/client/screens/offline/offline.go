// Package offline is the page the installed application shows without a
// network, /offline (U-11), drawn by frame «Без сети · установленное
// приложение» (offline-1): the library's worker keeps it and nothing else.
package offline

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// brand is the mark of the top bar, alone: no search, no actions — nothing
// on the page asks the network but Try again.
var brand = w.Brand{Logo: &w.Logo{Label: "Carrel", ViewBox: "4.79 -31.36 291.08 116.63"}}

// Screen draws the page: the mark in the top bar, no rail, the one message
// with Try again after its words (Notices).
func Screen(p *view.OfflinePage) w.Shell {
	return w.Shell{Top: &w.Top{Brand: brand}, Body: w.ShellBody{Main: w.Main{Items: bind.Notices(p.Notices, w.MsgSpacePage)}}}
}
