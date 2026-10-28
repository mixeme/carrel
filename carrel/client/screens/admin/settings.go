package admin

import (
	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// The words of the settings (U-1.3): what the test says when the relay took
// the letter, before its answer.
const (
	testSent    = "Sent."
	testAnswer  = " The relay answered "
	testEnd     = "."
	settingsID  = "admin-settings"
	saveID      = "admin.settings.save"
	publicURLID = "publicURL"
)

// settingsWidths are the widths the frame gives the fields, by their names
// (s-admin-3).
var settingsWidths = map[string]w.FieldWidth{
	"inviteTTL": w.FieldXS, "idle": w.FieldXS, "session": w.FieldXS, "name": w.FieldMD, publicURLID: w.FieldW580,
	"host": w.FieldLG, "port": w.FieldXS, "security": w.FieldSM, "username": w.FieldMD, "password": w.FieldMD,
	"from": w.FieldMD, "fromName": w.FieldMD,
	"attach": w.FieldXS, "letter": w.FieldXS, "recipients": w.FieldXS, "perDay": w.FieldXS,
	"to": w.FieldLG,
}

// Settings is the instance's settings, /admin/settings (AdminSettingsPage,
// s-admin-3; the test sent, -32; a field refused, -33): one form — Global,
// Mail with its badge, Letters from users, Test —, Discard and Save in the
// head; the relay's answer to the test under its field, in alarm where it
// was refused.
func Settings(p *view.AdminSettingsPage) w.Shell {
	h := w.Head{Title: &w.HeadTitle{Text: string(p.Title)}}
	if p.Subtitle != "" {
		h.Subs = []w.HeadSub{{Parts: []w.HeadSubPart{w.Text(p.Subtitle)}}}
	}
	acts := []w.ActsPart{bind.Button(p.Discard, "")}
	for _, a := range p.Form.Actions {
		if a.ID == saveID {
			acts = append(acts, bind.SubmitForm(a, settingsID))
		}
	}
	h.Acts = &w.Acts{Items: acts}
	f := bind.Form(p.Form, settingsID)
	for i, set := range p.Form.Sets {
		fs := w.FieldSet{Rubric: rubric(set)}
		if i == 0 {
			fs.Variant = w.FieldSetLead
		}
		for _, r := range set.Rows {
			if r.Kind == view.InputHidden {
				continue
			}
			fs.Items = append(fs.Items, settingsField(p, r))
		}
		if p.Test != nil && i == len(p.Form.Sets)-1 {
			fs.Items = append(fs.Items, w.Field{Parts: []w.FieldPart{testMsg(*p.Test)}, Width: w.FieldFull})
		}
		f.Items = append(f.Items, fs)
	}
	main := []w.MainPart{h}
	main = append(main, bind.ColumnMsgs(bind.Notices(p.Notices, w.MsgSpacePage))...)
	return shell(p.Top, p.Rail, append(main, f))
}

// rubric is a group's legend and its badges (Mail «not configured» in
// alarm, Letters from users «off»).
func rubric(set view.FieldSet) w.Rubric {
	r := w.Rubric{Text: string(set.Legend)}
	for _, b := range set.Badges {
		r.Items = append(r.Items, bind.Badge(b))
	}
	return r
}

// settingsField is a row of the settings as its frame draws it: the public
// address shown and not changed here; the tick on its own line, its words
// under it; the sender's two addresses as segments; the test's address with
// Send test beside it, which sends what is typed and saves nothing.
func settingsField(p *view.AdminSettingsPage, r view.FormRow) w.Field {
	switch {
	case r.Kind == view.InputCheck:
		return w.Field{Parts: append([]w.FieldPart{bind.Check(r)}, bind.Column(bind.Hint(r))...), OwnLine: true, Place: w.Grown}
	case r.Kind == view.InputRadio:
		seg := w.Seg{Name: r.Name, Variant: w.SegStart}
		for _, o := range r.Options {
			seg.Options = append(seg.Options, w.SegOption{Label: string(o.Label), On: o.Selected, Value: o.Value})
		}
		return w.Field{Label: string(r.Label), Parts: append([]w.FieldPart{seg}, bind.Column(bind.Hint(r))...), Place: w.Grown}
	case r.Name == publicURLID:
		// Read only, set with the instance's environment: shown, not sent
		// for a change.
		in := bind.Input(r)
		in.Disabled, in.ReadOnly = false, true
		return w.Field{Label: string(r.Label), Parts: append([]w.FieldPart{in}, bind.Column(bind.Hint(r))...), Width: settingsWidths[r.Name]}
	case r.Name == "to":
		in := bind.Input(r)
		in.Variant = w.InputFill
		line := w.Line{Items: []w.LinePart{in}, Gap: w.LineGap8}
		for _, a := range p.Form.Actions {
			if a.Submits {
				line.Items = append(line.Items, bind.Button(a, ""))
			}
		}
		return w.Field{Label: string(r.Label), Parts: append([]w.FieldPart{line}, bind.Column(bind.Hint(r))...), Width: settingsWidths[r.Name]}
	}
	return bind.ColumnField(r, settingsWidths[r.Name])
}

// testMsg is the relay's answer to the test (MailTestResult): taken — «Sent.»
// and the answer monospaced (s-admin-32) —, else the refusal in alarm
// (s-admin-33).
func testMsg(t view.MailTestResult) w.Msg {
	if t.OK {
		return w.Msg{Column: true, Parts: []w.MsgPart{w.Strong(testSent), w.Text(testAnswer), w.Mono{Text: string(t.Reply)}, w.Text(testEnd)}, Space: w.MsgFlush}
	}
	return w.Msg{Column: true, Parts: []w.MsgPart{w.Text(t.Reply)}, Tone: w.MsgAlert, Space: w.MsgFlush}
}

// SettingsForms are the forms of the page's actions that post with none of
// their own: the top bar's and the notices'.
func SettingsForms(p *view.AdminSettingsPage) []w.PostForm {
	return forms(p.Top, p.Notices)
}
