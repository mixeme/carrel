package contactedit

import (
	"strconv"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The frame page's words (U-1.3): its fields' names, the colour of the
// picture's place while it loads.
const (
	zoomLabel    = "Zoom"
	savedAsLabel = "Saved as"
	pictureSpot  = "#8C7B6A"
)

// Crop draws the page a contact's photo is framed on,
// /app/contacts/{account}/{col}/{uid}/photo-preview and the new card's
// …/new/photo-preview, from its view model (s-contacts-5): a form
// (PhotoPreviewPage.Form) in a dialog 520 wide.
//
//   - The head: «Photo», whose it is and that the file's metadata does not
//     go into the card (Subtitle).
//   - The picture with its frame beside the fields: how far it is zoomed
//     (the frame's hidden zoom, a percent of the largest square, as a
//     multiple), the turn as segments of rotate, what is saved into the card
//     read-only with its line under it; the note over them.
//   - The foot: Save photo — the form's button —, Cancel back to the form,
//     and pushed right Remove photo, which sends the same form to its own
//     address, when the card has a photo.
//
// The rail is the way back to the card with the book's sources, as the
// card's edit has it.
func Crop(p *view.PhotoPreviewPage) w.Shell {
	t := bind.Top(p.Top, top)
	h := w.Head{Title: &w.HeadTitle{Text: string(p.Title)}}
	if p.Subtitle != "" {
		h.Subs = []w.HeadSub{{Parts: []w.HeadSubPart{w.Text(p.Subtitle)}}}
	}
	crop := w.PhotoCrop{Colour: pictureSpot, Src: p.Href, Note: string(p.Note)}
	crop.Fields = append(crop.Fields, w.Field{Label: zoomLabel, Parts: []w.FieldPart{w.Zoom{Value: times(p.Form)}}, Width: w.FieldW100})
	for _, r := range bind.Rows(p.Form) {
		if r.Kind != view.InputRadio {
			continue
		}
		seg := w.Seg{Name: r.Name, Variant: w.SegStart}
		for _, o := range r.Options {
			seg.Options = append(seg.Options, w.SegOption{Label: string(o.Label), Value: o.Value, On: o.Selected})
		}
		crop.Fields = append(crop.Fields, w.Field{Label: string(r.Label), Parts: []w.FieldPart{seg}, Width: w.FieldW100})
	}
	if p.Saved != "" {
		saved := w.Field{Label: savedAsLabel, Width: w.FieldW100,
			Parts: []w.FieldPart{w.Input{Value: string(p.Saved), Label: savedAsLabel, Mono: true, ReadOnly: true}}}
		if p.SavedHint != "" {
			saved.Parts = append(saved.Parts, w.FieldHint{Parts: []w.Inline{w.Text(p.SavedHint)}})
		}
		crop.Fields = append(crop.Fields, saved)
	}
	foot := w.FormFoot{Variant: w.FormFootFlush}
	for _, a := range p.Actions {
		b := bind.Button(a, "")
		if a.Submits && a.Href == p.Form.Action {
			b.To = "" // the form's own address
		}
		if a.Danger {
			// Outlined: it takes away what the card keeps, not the card.
			b.Kind = w.ButtonDanger
			foot.Items = append(foot.Items, w.Push{Items: []w.PushPart{b}})
			continue
		}
		foot.Items = append(foot.Items, b)
	}
	main := append(bind.Notices(p.Notices, w.MsgSpacePage), w.Dialog{
		Items: []w.DialogPart{h, crop, foot}, Variant: w.Dialog520,
		Action: p.Form.Action, Method: p.Form.Method, Fields: bind.Hidden(p.Form),
	})
	return w.Shell{Top: &t, Body: w.ShellBody{Rail: railParts(p.Rail, false), Main: w.Main{Items: main}}}
}

// times is the frame's zoom as a multiple of the largest square: its
// hidden field in percent, 160 — «1.6×».
func times(f view.Form) string {
	for _, set := range f.Sets {
		for _, r := range set.Rows {
			if r.Name != "zoom" {
				continue
			}
			n, err := strconv.Atoi(string(r.Value))
			if err != nil {
				return ""
			}
			return strconv.Itoa(n/100) + "." + strconv.Itoa(n%100/10) + "×"
		}
	}
	return ""
}

// CropForms are the frame page's forms: the top bar's, the rail's foot's.
func CropForms(p *view.PhotoPreviewPage) []w.PostForm {
	return append(bind.TopForms(p.Top), bind.FootForms(p.Rail)...)
}
