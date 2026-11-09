package settings

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// DavloomScopeSheet is what a device sees, over davloom
// (/app/settings/davloom/scope, DavloomScopeSheet, s-davloom-7): «What it
// sees» with the device on its right; the collections by connection, a
// column each, as the sources of a copy job; Save — Enter — and Cancel —
// Esc.
func DavloomScopeSheet(p *view.DavloomScopeSheet) w.Sheet {
	s := w.Sheet{Width: w.SheetWidth680, Action: p.Form.Action, Method: p.Form.Method, Fields: bind.Hidden(p.Form)}
	if p.Subject != "" {
		s.Head = []w.SheetHeadPart{w.Dim{Parts: []w.Inline{w.Text(p.Subject)}, Size: w.Dim11_5, Variant: w.DimWrap, Place: w.Pushed}}
	}
	fs := w.FieldSet{Variant: w.FieldSetLead}
	for _, r := range bind.Rows(p.Form) {
		if r.Kind != view.InputCheck {
			continue
		}
		s.Title = string(r.Label)
		fs.Items = append(fs.Items, sourceGrid(r))
		if r.Error != "" {
			fs.Items = append(fs.Items, w.Msg{Parts: []w.MsgPart{w.Text(r.Error)}, Tone: w.MsgAlert, Space: w.MsgFlushAboveTight, Place: w.Grown})
		}
	}
	s.Body = []w.SheetPart{w.Form{Variant: w.FormFlush, Items: []w.FormPart{fs}}}
	for _, a := range p.Form.Actions {
		if a.Method == "GET" {
			s.Foot = append(s.Foot, w.Line{Items: []w.LinePart{bind.Button(a, ""), w.Kbd{Keys: escKey}}, Gap: w.LineGapNormal, Place: w.Pushed})
			continue
		}
		b := bind.Submit(a)
		if a.Href != "" && a.Href != p.Form.Action {
			b.To = a.Href
		}
		s.Foot = append(s.Foot, b, w.Kbd{Keys: enterKey})
	}
	return s
}

// DavloomTreeSheet is what a client sees at the address
// (/app/settings/davloom/tree, DavloomTreeSheet, s-davloom-9): the address
// on the right of the title, the tree, Close — Esc.
func DavloomTreeSheet(p *view.DavloomTreeSheet) w.Sheet {
	s := w.Sheet{Title: treeRubric, Width: w.SheetWidth680}
	if p.Address != "" {
		s.Head = []w.SheetHeadPart{w.Dim{Parts: []w.Inline{w.Text(p.Address)}, Size: w.Dim11_5, Variant: w.DimWrap, Place: w.Pushed}}
	}
	s.Body = []w.SheetPart{proxyTree(p.Tree)}
	for _, a := range p.Actions {
		s.Foot = append(s.Foot, w.Line{Items: []w.LinePart{bind.Button(a, ""), w.Kbd{Keys: escKey}}, Gap: w.LineGapNormal, Place: w.Pushed})
	}
	return s
}
