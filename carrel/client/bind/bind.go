// Package bind translates the types of the contract (C-6.2, C-6.8, C-8) into
// Typecase's widgets: a sentence in pieces into running text, an action into
// a button or a link that means what it does, a form's row into a field, an
// option into an option of a pick, a notice into a message. It is the one
// place of the client where a type of the contract becomes a widget, so the
// screens do not each translate them again (the owner's decision of
// 30.09.2026, U-5.7).
//
// Only translation: bind writes no markup, no style, no class and no words of
// its own — every word it hands a widget is the view model's — and it
// chooses no drawn variant the frame decides (a field's width, a dialog's
// size, a message's spacing): those are the screen's, taken from its draft,
// and the screen passes them in or sets them on what bind returns. The guards
// of the screens hold it too (carrel.dev/carrel/client/guard): it imports only
// the view models, the widgets and the standard packages a screen may, and
// no string constant in it holds a word. A screen imports it beside the
// widgets, and nothing else of the client.
package bind

import (
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

// Inlines is a sentence of the model in its pieces, each in its face (C-6.2
// Runs): a piece marked mono is monospaced (a login, an address), strong is
// bold (a name), danger an alarm (a source that did not answer), the rest
// plain text. The sentence is never cut or set again
// from the server's words.
func Inlines(r view.Runs) []w.Inline {
	out := make([]w.Inline, 0, len(r))
	for _, p := range r {
		switch p.Mark {
		case view.RunMono:
			out = append(out, w.Mono{Text: string(p.Text)})
		case view.RunStrong:
			out = append(out, w.Strong(p.Text))
		case view.RunDanger:
			out = append(out, w.Span{Face: w.FaceDanger, Text: string(p.Text)})
		default:
			out = append(out, w.Text(p.Text))
		}
	}
	return out
}

// Words is a sentence of the model as one string, its pieces one after
// another, for a widget that takes a sentence without faces (the first line
// of a tagline): the frame marks none of its pieces there.
func Words(r view.Runs) string {
	var s string
	for _, p := range r {
		s += string(p.Text)
	}
	return s
}

// Kind is how an action is drawn as a button (C-6.2): the primary action of
// the frame (Primary, is-primary), a destructive one (Danger — delete,
// reset, merge: it destroys, is-danger-solid), else a plain one. Primary
// wins: the contract gives no action both.
func Kind(a view.Action) w.ButtonKind {
	switch {
	case a.Primary:
		return w.ButtonPrimary
	case a.Danger:
		return w.ButtonDangerSolid
	}
	return w.ButtonPlain
}

// Button is an action as a button with its meaning (U-5.1, U-8): an action
// that goes somewhere (GET) is a link to its Href; one that posts (POST)
// submits the form named form — the Form it stands under, or a PostForm of
// the page (Button.Form), or — one that sends the form it stands in
// (Submits, without a file) — submits that form to its own Href
// (formaction: Remove of a new record's attachment); an action switched off
// is drawn disabled and says why (Reason), and leads nowhere. The label,
// the icon and the kind are the action's.
func Button(a view.Action, form string) w.Button {
	b := w.Button{Label: string(a.Label), Icon: Icon(a.Icon), Kind: Kind(a)}
	switch {
	case !a.Enabled:
		b.State = w.ButtonDisabled
		b.Reason = string(a.Reason)
	case a.Method == "POST" && a.Submits && a.File == "":
		b.Submit, b.To = true, a.Href
	case a.Method == "POST":
		b.Form = form
	default:
		b.Href = a.Href
	}
	return b
}

// FileButton is an action as a button (Button) — one that carries a file
// (Action.File: the photo's Replace and Add) the label of the file input of
// the form named form, which the browser opens without a script and whose
// pick sends the form with the library's script (Typecase's task 109); the
// input's accessible name is the action's label, the screen's where its
// frame names it otherwise. Without a script «Upload» beside it sends the
// file (Upload). One that sends the form it stands in (Submits: the photo of
// a new card) picks a file that sends that form to the action's address
// (FileField.To, Typecase's task 122): the typing goes with the file.
func FileButton(a view.Action, form string) w.Button {
	if a.File == "" || !a.Enabled || a.Method != "POST" {
		return Button(a, form)
	}
	f := &w.FileField{Form: form, Name: a.File, Label: string(a.Label)}
	if a.Submits {
		f.Form, f.To = "", a.Href
	}
	return w.Button{Label: string(a.Label), Icon: Icon(a.Icon), Kind: Kind(a), File: f}
}

// Upload is the button «Upload» beside a button that picks a file
// (FileButton): shown only on a page without a script, it sends the form
// named form with the file picked — or, for an action that sends the form
// it stands in (Submits), that form to the action's address; word is the
// screen's. nil where the action picks no file.
func Upload(a view.Action, form, word string) *w.Button {
	if a.File == "" || !a.Enabled || a.Method != "POST" {
		return nil
	}
	if a.Submits {
		return &w.Button{Label: word, Submit: true, To: a.Href, NoScript: true}
	}
	return &w.Button{Label: word, Form: form, NoScript: true}
}

// iconPrefix is what the contract's icon names start with (U-4: «i-down»),
// the id of the symbol in the sprite; the widget takes the name without it.
const iconPrefix = "i-"

// Icon is an icon of the contract (Action.Icon, «i-down») as the widget's
// name of it («down»): the same symbol of the sprite (U-4). A name without the
// prefix is taken as it is; none is none.
func Icon(name string) w.Icon {
	if len(name) > len(iconPrefix) && name[:len(iconPrefix)] == iconPrefix {
		name = name[len(iconPrefix):]
	}
	return w.Icon(name)
}

// Link is an action that goes somewhere, as a link of running text: its
// label and its address.
func Link(a view.Action) w.Link {
	return w.Link{Text: string(a.Label), Href: a.Href}
}

// Span is an action that goes somewhere, as words in a face the frame gives
// («Already have an account? Sign in» in the accent at 12 px): its label and
// its address.
func Span(a view.Action, face w.Face) w.Span {
	return w.Span{Face: face, Text: string(a.Label), Href: a.Href}
}

// Hidden are the hidden rows of a form, in order (C-6.8 InputHidden): the
// token, the version, the step — what the form carries and nobody fills in.
func Hidden(f view.Form) []w.Hidden {
	var out []w.Hidden
	for _, set := range f.Sets {
		for _, r := range set.Rows {
			if r.Kind == view.InputHidden {
				out = append(out, w.Hidden{Name: r.Name, Value: string(r.Value)})
			}
		}
	}
	return out
}

// Rows are the rows of a form a person sees, in order across its sets: all
// but the hidden ones.
func Rows(f view.Form) []view.FormRow {
	var out []view.FormRow
	for _, set := range f.Sets {
		for _, r := range set.Rows {
			if r.Kind != view.InputHidden {
				out = append(out, r)
			}
		}
	}
	return out
}

// Form is a form of the model as a form that sends: where and how (Action,
// Method), its hidden rows and the ID the buttons outside it name it by. What
// stands in it and how it is laid out — Items, Variant — are the screen's.
func Form(f view.Form, id string) w.Form {
	return w.Form{Action: f.Action, Method: f.Method, Fields: Hidden(f), ID: id}
}

// PostForm is a form of the model with no field a person fills in — one
// button's form (a card of the forgotten password, s-auth-6; the wipe's
// confirmation, s-auth-10) — as a PostForm the page prints at its end
// (page.Document.Forms), which the button submits by its ID from where it
// stands.
func PostForm(f view.Form, id string) w.PostForm {
	return w.PostForm{ID: id, Action: f.Action, Fields: Hidden(f)}
}

// Input is a row that is typed into as a field of one line (C-6.8): its name
// and value, its accessible name (Label), the pattern in the empty field
// (Placeholder), monospaced where the frame draws it so (Mono), the cursor
// where the model puts it (Focus), marked when it came back with a reason
// (Error), switched off — shown in its place, not read by the server —
// when ReadOnly, and Required. A password is a password field, a date and a
// time the browser's own, a number brings the digits' keyboard; any other
// kind is a line of text.
func Input(r view.FormRow) w.Input {
	in := w.Input{
		Name:        r.Name,
		Value:       string(r.Value),
		Placeholder: string(r.Placeholder),
		Label:       string(r.Label),
		Mono:        r.Mono,
		Focus:       r.Focus,
		Error:       r.Error != "",
		Disabled:    r.ReadOnly,
		Required:    r.Required,
		Numeric:     r.Kind == view.InputNumber,
		// An end that follows its start, and the date of a time (C-6.8
		// Follows, Day): handed to the library as they are.
		Follows: r.Follows,
		Day:     r.Day,
	}
	switch r.Kind {
	case view.InputPassword:
		in.Kind = w.InputPassword
	case view.InputDate:
		in.Kind = w.InputDate
	case view.InputTime:
		in.Kind = w.InputTime
	}
	return in
}

// Pick is a row chosen from a list (InputSelect) as a list of the browser: its
// name, its accessible name, the cursor, and its options.
func Pick(r view.FormRow) w.Pick {
	return w.Pick{Name: r.Name, Label: string(r.Label), Focus: r.Focus, Options: Options(r.Options)}
}

// Options are the options of a row as the options of a pick (C-6.8 Option):
// the value sent, the label, the one chosen (Selected) and the ones shown and
// not chosen (Disabled). What a pick of the browser cannot show — a rubric
// before an option, a line, a colour, a note on the right, a reason, a value
// typed by hand — is not here: the screen that draws such a list draws it
// with the widget its frame has.
func Options(os []view.Option) []w.PickOption {
	out := make([]w.PickOption, 0, len(os))
	for _, o := range os {
		out = append(out, w.PickOption{Value: o.Value, Label: string(o.Label), On: o.Selected, Unavailable: o.Disabled, Shows: o.Shows})
	}
	return out
}

// Area is a row of several lines (InputArea): its name, its text, its
// accessible name, monospaced and the cursor where the model says.
func Area(r view.FormRow) w.Area {
	return w.Area{Name: r.Name, Text: string(r.Value), Label: string(r.Label), Mono: r.Mono, Focus: r.Focus}
}

// Control is the control of a row by its kind: a pick for a choice from a
// list, an area for several lines, else a field of one line (Input). A tick,
// a set of radios and the buttons Add and Remove of a repeating field are not
// one control of this kind; their translation joins bind with the first
// screen that draws them.
func Control(r view.FormRow) w.FieldPart {
	switch r.Kind {
	case view.InputSelect:
		return Pick(r)
	case view.InputArea:
		return Area(r)
	}
	return Input(r)
}

// Hint is the line under a row's field: the reason it came back with (Error,
// in the danger's tone), else what the value means (Hint); none when the row
// has neither. The reason takes the hint's place: one line under a field.
func Hint(r view.FormRow) []w.FieldPart {
	switch {
	case r.Error != "":
		return []w.FieldPart{w.FieldHint{Parts: []w.Inline{w.Text(r.Error)}, Variant: w.HintDanger}}
	case r.Hint != "":
		return []w.FieldPart{w.FieldHint{Parts: []w.Inline{w.Text(r.Hint)}}}
	}
	return nil
}

// Field is a row of a form as a field: its label, the muted part after it
// (LabelAside: «(optional)»), its control (Control) and the line under it
// (Hint). The width is the frame's, from the screen's draft.
func Field(r view.FormRow, width w.FieldWidth) w.Field {
	f := w.Field{Label: string(r.Label), Parts: append([]w.FieldPart{Control(r)}, Hint(r)...), Width: width, Hidden: r.Hidden}
	if r.LabelAside != "" {
		f.LabelAside = w.Dim{Parts: []w.Inline{w.Text(r.LabelAside)}}
	}
	return f
}

// Tone is the look of a notice by its role (C-8 NoticeRole): an alert — the
// password nobody can recover, a letter that did not go — in the alert's
// tone, an accent in the accent's, information plain.
func Tone(r view.NoticeRole) w.MsgTone {
	switch r {
	case view.NoticeAlert:
		return w.MsgAlert
	case view.NoticeAccent:
		return w.MsgAccent
	}
	return w.MsgPlain
}

// Msg is a notice as a message (C-6.2 Notice): its tone by its role, its
// title bold and its body after it, a space between them. The spacing is the
// frame's (space). The notice's actions and its technical block are drawn by
// the screen where its frame draws them; a notice that has them is not all
// here.
func Msg(n view.Notice, space w.MsgSpace) w.Msg {
	var parts []w.MsgPart
	if n.Title != "" {
		parts = append(parts, w.Strong(n.Title))
		if n.Body != "" {
			parts = append(parts, w.Text(" "))
		}
	}
	if n.Body != "" {
		parts = append(parts, w.Text(n.Body))
	}
	return w.Msg{Parts: parts, Tone: Tone(n.Role), Space: space}
}

// Remove is the button that closes a line of a repeating field (InputRemove,
// C-6.8): a submit button of the form that sends its name and the line
// (remove=<line>) — without scripts the server answers without the line;
// with them the library empties and hides its fields. It is destructive in
// the plain way: it breaks the line off, the card keeps its data until it
// is saved.
func Remove(r view.FormRow) w.Button {
	return w.Button{Label: string(r.Label), Kind: w.ButtonDanger, Submit: true, Name: r.Name, Value: string(r.Value)}
}

// Add is the button «Add …» of a repeating field (InputAdd, C-6.8) and the
// template of the lines one press adds in place (Template), each line drawn
// by line — the screen's, which lays it out as its frame does. The
// template's ID is the row's name and value («add.tel»).
func Add(r view.FormRow, line func(view.FormRow) []w.FieldSetPart) (w.Button, w.Template) {
	id := r.Name + "." + string(r.Value)
	t := w.Template{ID: id}
	for _, row := range r.Template {
		t.Items = append(t.Items, line(row)...)
	}
	return w.Button{Label: string(r.Label), Adds: id}, t
}

// Loss is what the server did not keep of a save (C-9 LossReport) as a
// notice in the plain tone, as its frame draws it (states-12): what it means
// (Note), then the properties lost, monospaced, a comma between them. A
// notice, not a lock and not an alarm: the record is written.
func Loss(l view.LossReport, space w.MsgSpace) w.Msg {
	parts := []w.MsgPart{w.Text(l.Note)}
	for i, p := range l.Properties {
		sep := ", "
		if i == 0 {
			sep = " "
		}
		parts = append(parts, w.Text(sep), w.Mono{Text: string(p)})
	}
	return w.Msg{Parts: parts, Space: space}
}
