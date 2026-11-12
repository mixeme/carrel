package settings

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// appearanceID is the ID of the page's form, which Save in the head submits.
const appearanceID = "appearance"

// appearanceWidths are the widths the frame gives the fields (appearance-1).
var appearanceWidths = map[string]w.FieldWidth{"weekStart": w.FieldMD, "clock": w.FieldMD}

// Appearance is /app/settings/appearance (AppearancePage, appearance-1):
// one form — Theme and Density as segments, Lists as ticks with Reset
// remembered view settings under them, Dates and times with the instance's
// time zone a line of text after the fields —, each group's words under it; Discard
// and Save in the head.
func Appearance(p *view.AppearancePage) w.Shell {
	h := head(p.Title, p.Subtitle)
	h.Acts = formActs(p.Form, appearanceID)
	f := bind.Form(p.Form, appearanceID)
	lead := true
	for _, set := range p.Form.Sets {
		if set.Legend == "" {
			continue
		}
		fs := w.FieldSet{Rubric: w.Rubric{Text: string(set.Legend)}}
		if lead {
			fs.Variant, lead = w.FieldSetLead, false
		}
		var ticks []w.FieldPart
		for _, r := range set.Rows {
			switch {
			case r.Kind == view.InputRadio:
				fs.Items = append(fs.Items, segField(r))
			case r.Kind == view.InputCheck:
				ticks = append(ticks, bind.Check(r))
			default:
				fs.Items = append(fs.Items, bind.Field(r, appearanceWidths[r.Name]))
			}
		}
		if len(ticks) > 0 {
			fs.Items = append(fs.Items, w.Field{Parts: ticks, Gapped: true, Place: w.Grown})
		}
		if len(set.Facts) > 0 {
			fs.Items = append(fs.Items, bind.Facts(set.Facts))
		}
		if set.Hint != "" {
			fs.Items = append(fs.Items, hintField(set.Hint))
		}
		if len(ticks) > 0 && p.Reset.ID != "" {
			// Reset stands under the ticks it resets (appearance-1).
			b := bind.Button(p.Reset, "")
			if p.Reset.Enabled {
				b.Form, b.Submit, b.To = "", true, p.Reset.Href
			}
			fs.Items = append(fs.Items, w.Field{Parts: []w.FieldPart{b}})
		}
		f.Items = append(f.Items, fs)
	}
	main := []w.MainPart{h}
	main = append(main, bind.ColumnMsgs(bind.Notices(p.Notices, w.MsgSpacePage))...)
	return shell(p.Top, p.Rail, append(main, f))
}

// AppearanceForms are the forms of the page's actions that post with none of
// their own.
func AppearanceForms(p *view.AppearancePage) []w.PostForm {
	return forms(p.Top, p.Notices)
}
