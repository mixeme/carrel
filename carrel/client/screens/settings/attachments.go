package settings

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// attachmentsID is the ID of the page's form, which Save in the head
// submits.
const attachmentsID = "attachments"

// The rows of what is in the folder now, monospaced by what they say
// (attachments-1): the address small.
const nowPath = "now.path"

// attachmentsWidths are the widths the frame gives the fields.
var attachmentsWidths = map[string]w.FieldWidth{"collection": w.FieldMD, "folder": w.FieldW580, "name": w.FieldW580}

// Attachments is /app/settings/attachments (AttachmentsPage,
// attachments-1): what stays in the record and what goes to the folder,
// and which connections take less; the folder — its collection, the folder
// in it with Browse, a subfolder per month —; how a file is named; what is
// in the folder now; Open in Files, Discard and Save in the head; under the
// form, that changing the folder moves nothing. None chosen yet, or one
// its server refused, says so in its rubric (attachments-5, -6).
func Attachments(p *view.AttachmentsPage) w.Shell {
	h := head(p.Title, p.Subtitle)
	acts := formActs(view.Form{Actions: openFirst(p.Form.Actions)}, attachmentsID)
	h.Acts = acts
	f := bind.Form(p.Form, attachmentsID)
	for i, set := range p.Form.Sets {
		// The state of a set in its rubric (Typecase's note 93): «not set»,
		// «unavailable».
		r := w.Rubric{Text: string(set.Legend)}
		for _, b := range set.Badges {
			r.Items = append(r.Items, bind.Badge(b))
		}
		fs := w.FieldSet{Rubric: r}
		if i == 0 {
			fs.Variant = w.FieldSetLead
		}
		var terms w.Fields
		var hints []w.FieldSetPart
		for _, r := range set.Rows {
			switch {
			case r.Kind == view.InputHidden:
			case r.ReadOnly:
				// What the page says, not what is typed: a term and its value.
				it := w.FieldsItem{Term: string(r.Label), Value: []w.ValuePart{w.Text(r.Value)}}
				if set.Legend != p.Form.Sets[0].Legend {
					it.Face = w.ValueMono
					if r.Name == nowPath {
						it.Face = w.ValueMonoSmall
					}
				}
				terms.Items = append(terms.Items, it)
				if h := bind.Column(bind.Hint(r)); len(h) > 0 {
					hints = append(hints, w.Field{Parts: h, Place: w.Grown})
				}
			case r.Kind == view.InputCheck:
				fs.Items = append(fs.Items, w.Field{Parts: append([]w.FieldPart{bind.Check(r)}, bind.Column(bind.Hint(r))...), Gap: 7, OwnLine: true, Place: w.Grown})
			case r.Extra != nil:
				in := bind.Input(r)
				in.Variant = w.InputFill
				line := w.Line{Items: []w.LinePart{in, bind.Button(*r.Extra, "")}, Gap: w.LineGap8}
				fs.Items = append(fs.Items, w.Field{Label: string(r.Label), Parts: append([]w.FieldPart{line}, bind.Column(bind.Hint(r))...), Width: attachmentsWidths[r.Name]})
			default:
				fs.Items = append(fs.Items, bind.ColumnField(r, attachmentsWidths[r.Name]))
			}
		}
		if len(terms.Items) > 0 {
			terms.Variant, terms.Place = w.FieldsBleed130, w.Grown
			fs.Items = append([]w.FieldSetPart{terms}, fs.Items...)
		}
		fs.Items = append(fs.Items, hints...)
		if set.Diag != "" {
			// What the server refused, one line (attachments-6).
			fs.Items = append(fs.Items, w.Diag{Text: string(set.Diag), Place: w.Grown})
		}
		f.Items = append(f.Items, fs)
	}
	main := []w.MainPart{h}
	main = append(main, bind.ColumnMsgs(bind.Notices(p.Notices, w.MsgSpacePage))...)
	main = append(main, f)
	if p.Note != "" {
		main = append(main, w.Msg{Column: true, Parts: []w.MsgPart{w.Text(p.Note)}})
	}
	return shell(p.Top, p.Rail, main)
}

// openFirst are the head's actions in the frame's order: Open in Files
// before Discard and Save (attachments-1).
func openFirst(as []view.Action) []view.Action {
	var open, rest []view.Action
	for _, a := range as {
		if a.ID == "attachments.open" {
			open = append(open, a)
			continue
		}
		rest = append(rest, a)
	}
	return append(open, rest...)
}

// AttachmentsForms are the forms of the page's actions that post with none
// of their own.
func AttachmentsForms(p *view.AttachmentsPage) []w.PostForm {
	return forms(p.Top, p.Notices)
}
