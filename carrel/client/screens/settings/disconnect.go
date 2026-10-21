package settings

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// bullet is what starts each line of what goes with a connection
// (s-profile-26).
const bullet = "• "

// DisconnectSheet is Disconnect over the connections,
// /app/settings/connections/disconnect (ConnectionRemoveSheet,
// s-profile-26): what is pressed in its title with the server in alarm,
// the connection on its right; that nothing on the server is deleted, in
// alarm, and what Carrel removes, a line a count; Disconnect — the solid
// danger — and Cancel.
func DisconnectSheet(p *view.ConnectionRemoveSheet) w.Sheet {
	s := w.Sheet{Icon: "server", Tone: w.SheetIconDanger, Width: w.SheetWidth480,
		Action: p.Form.Action, Method: p.Form.Method, Fields: bind.Hidden(p.Form)}
	if p.Subject != "" {
		s.Head = []w.SheetHeadPart{w.Dim{Parts: []w.Inline{w.Text(p.Subject)}, Size: w.Dim11_5, Variant: w.DimWrap, Place: w.Pushed}}
	}
	if p.Warning != "" {
		s.Body = append(s.Body, w.Msg{Parts: []w.MsgPart{w.Text(p.Warning)}, Tone: w.MsgAlert, Space: w.MsgFlush})
	}
	if len(p.References) > 0 {
		var parts []w.MsgPart
		for i, r := range p.References {
			if i > 0 {
				parts = append(parts, w.Break{})
			}
			parts = append(parts, w.Text(bullet+string(r)))
		}
		s.Body = append(s.Body, w.Msg{Parts: parts, Space: w.MsgFlush})
	}
	for _, a := range p.Actions {
		if a.Method == "GET" {
			s.Foot = append(s.Foot, w.Line{Items: []w.LinePart{bind.Button(a, ""), w.Kbd{Keys: escKey}}, Gap: w.LineGap7, Place: w.Pushed})
			continue
		}
		// What is pressed is the sheet's title too (s-profile-26).
		s.Title = string(a.Label)
		b := bind.Submit(a)
		if a.Href != "" && a.Href != p.Form.Action {
			b.To = a.Href
		}
		s.Foot = append(s.Foot, b)
	}
	return s
}
