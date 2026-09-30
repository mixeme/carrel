package bind_test

import (
	"reflect"
	"testing"

	"carrel.dev/carrel/client/bind"
	"carrel.dev/carrel/server/view"
	w "carrel.dev/typecase/widget"
)

func same(t *testing.T, what string, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s:\n got %#v\nwant %#v", what, got, want)
	}
}

func TestInlines(t *testing.T) {
	r := view.Runs{{Text: "The account "}, {Text: "mix", Mark: view.RunMono}, {Text: " of "}, {Text: "Anna", Mark: view.RunStrong}, {Text: "."}}
	same(t, "the pieces in their faces", bind.Inlines(r), []w.Inline{w.Text("The account "), w.Mono{Text: "mix"}, w.Text(" of "), w.Strong("Anna"), w.Text(".")})
	same(t, "no pieces", bind.Inlines(nil), []w.Inline{})
	alarm := view.Runs{{Text: "Polled 5 of 6 · "}, {Text: "baikal · Общий unavailable", Mark: view.RunDanger}}
	same(t, "an alarm", bind.Inlines(alarm), []w.Inline{w.Text("Polled 5 of 6 · "), w.Span{Face: w.FaceDanger, Text: "baikal · Общий unavailable"}})
	same(t, "the words", bind.Words(r), "The account mix of Anna.")
	same(t, "no words", bind.Words(nil), "")
}

func TestKind(t *testing.T) {
	for name, c := range map[string]struct {
		a    view.Action
		want w.ButtonKind
	}{
		"plain":   {view.Action{}, w.ButtonPlain},
		"primary": {view.Action{Primary: true}, w.ButtonPrimary},
		"danger":  {view.Action{Danger: true}, w.ButtonDangerSolid},
	} {
		same(t, name, bind.Kind(c.a), c.want)
	}
}

func TestButton(t *testing.T) {
	for name, c := range map[string]struct {
		a    view.Action
		form string
		want w.Button
	}{
		"a link": {
			view.Action{Label: "Back to sign in", Href: "/login", Method: "GET", Enabled: true}, "f",
			w.Button{Label: "Back to sign in", Href: "/login"},
		},
		"a link without a method": {
			view.Action{Label: "Back", Href: "/app", Enabled: true}, "",
			w.Button{Label: "Back", Href: "/app"},
		},
		"a post submits the form named": {
			view.Action{Label: "Create account", Href: "/register", Method: "POST", Primary: true, Enabled: true}, "register",
			w.Button{Label: "Create account", Kind: w.ButtonPrimary, Form: "register"},
		},
		"destructive": {
			view.Action{Label: "Erase", Href: "/forgot/t", Method: "POST", Danger: true, Enabled: true}, "wipe.confirm",
			w.Button{Label: "Erase", Kind: w.ButtonDangerSolid, Form: "wipe.confirm"},
		},
		"with an icon": {
			view.Action{Label: "New contact", Icon: "i-plus", Href: "/app/contacts/new", Method: "GET", Primary: true, Enabled: true}, "",
			w.Button{Label: "New contact", Icon: "plus", Kind: w.ButtonPrimary, Href: "/app/contacts/new"},
		},
		"switched off says why and leads nowhere": {
			view.Action{Label: "Delete", Href: "/x", Method: "POST", Danger: true, Enabled: false, Reason: "Read-only"}, "f",
			w.Button{Label: "Delete", Kind: w.ButtonDangerSolid, State: w.ButtonDisabled, Reason: "Read-only"},
		},
	} {
		same(t, name, bind.Button(c.a, c.form), c.want)
	}
}

func TestIcon(t *testing.T) {
	for name, want := range map[string]w.Icon{"i-down": "down", "i-gear": "gear", "down": "down", "": "", "i-": "i-"} {
		same(t, name, bind.Icon(name), want)
	}
}

func TestLinkAndSpan(t *testing.T) {
	a := view.Action{Label: "Already have an account? Sign in", Href: "/login", Method: "GET", Enabled: true}
	same(t, "link", bind.Link(a), w.Link{Text: "Already have an account? Sign in", Href: "/login"})
	same(t, "span", bind.Span(a, w.FaceAccent12), w.Span{Face: w.FaceAccent12, Text: "Already have an account? Sign in", Href: "/login"})
}

// form is a form of the model: a token, a login, a hidden step and a
// password, in two sets.
var form = view.Form{
	Action: "/register",
	Method: "POST",
	Sets: []view.FieldSet{
		{Rows: []view.FormRow{
			{Name: "_csrf", Kind: view.InputHidden, Value: "csrf"},
			{Name: "login", Label: "Login", Kind: view.InputText, Focus: true, Required: true},
		}},
		{Rows: []view.FormRow{
			{Name: "step", Kind: view.InputHidden, Value: "2"},
			{Name: "password", Label: "Password", Kind: view.InputPassword, Required: true},
		}},
	},
}

func TestForm(t *testing.T) {
	hidden := []w.Hidden{{Name: "_csrf", Value: "csrf"}, {Name: "step", Value: "2"}}
	same(t, "hidden", bind.Hidden(form), hidden)
	same(t, "no hidden", bind.Hidden(view.Form{}), []w.Hidden(nil))
	var names []string
	for _, r := range bind.Rows(form) {
		names = append(names, r.Name)
	}
	same(t, "rows", names, []string{"login", "password"})
	same(t, "form", bind.Form(form, "register"), w.Form{Action: "/register", Method: "POST", Fields: hidden, ID: "register"})
	same(t, "post form", bind.PostForm(form, "forgot.wipe"), w.PostForm{ID: "forgot.wipe", Action: "/register", Fields: hidden})
}

func TestInput(t *testing.T) {
	for name, c := range map[string]struct {
		r    view.FormRow
		want w.Input
	}{
		"text": {
			view.FormRow{Name: "login", Label: "Login", Kind: view.InputText, Value: "anna", Focus: true, Required: true},
			w.Input{Name: "login", Label: "Login", Value: "anna", Focus: true, Required: true},
		},
		"mono with a pattern": {
			view.FormRow{Name: "login", Label: "Login", Kind: view.InputText, Placeholder: "a-z 0-9 . _ -", Mono: true},
			w.Input{Name: "login", Label: "Login", Placeholder: "a-z 0-9 . _ -", Mono: true},
		},
		"password": {
			view.FormRow{Name: "password", Label: "Password", Kind: view.InputPassword},
			w.Input{Kind: w.InputPassword, Name: "password", Label: "Password"},
		},
		"email is a line of text": {
			view.FormRow{Name: "email", Label: "Email", Kind: view.InputEmail, Value: "a@b.c"},
			w.Input{Name: "email", Label: "Email", Value: "a@b.c"},
		},
		"read-only is switched off": {
			view.FormRow{Name: "email", Label: "Email", Kind: view.InputEmail, Value: "a@b.c", ReadOnly: true},
			w.Input{Name: "email", Label: "Email", Value: "a@b.c", Disabled: true},
		},
		"came back with a reason": {
			view.FormRow{Name: "password", Label: "Password", Kind: view.InputPassword, Error: "Login or password is wrong.", Focus: true},
			w.Input{Kind: w.InputPassword, Name: "password", Label: "Password", Error: true, Focus: true},
		},
		"date":   {view.FormRow{Name: "d", Kind: view.InputDate}, w.Input{Kind: w.InputDate, Name: "d"}},
		"time":   {view.FormRow{Name: "t", Kind: view.InputTime}, w.Input{Kind: w.InputTime, Name: "t"}},
		"number": {view.FormRow{Name: "n", Kind: view.InputNumber}, w.Input{Name: "n", Numeric: true}},
	} {
		same(t, name, bind.Input(c.r), c.want)
		same(t, name+" as a control", bind.Control(c.r), w.FieldPart(c.want))
	}
}

func TestPickAndArea(t *testing.T) {
	r := view.FormRow{Name: "tz", Label: "Time zone", Kind: view.InputSelect, Focus: true, Options: []view.Option{
		{Value: "UTC", Label: "UTC"},
		{Value: "Europe/Moscow", Label: "Moscow", Selected: true},
		{Value: "x", Label: "Read-only book", Disabled: true, Reason: "Read-only"},
	}}
	want := w.Pick{Name: "tz", Label: "Time zone", Focus: true, Options: []w.PickOption{
		{Value: "UTC", Label: "UTC"},
		{Value: "Europe/Moscow", Label: "Moscow", On: true},
		{Value: "x", Label: "Read-only book", Unavailable: true},
	}}
	same(t, "pick", bind.Pick(r), want)
	same(t, "pick as a control", bind.Control(r), w.FieldPart(want))
	same(t, "no options", bind.Options(nil), []w.PickOption{})
	a := view.FormRow{Name: "rrule", Label: "Advanced", Kind: view.InputArea, Value: "FREQ=WEEKLY", Mono: true}
	same(t, "area", bind.Control(a), w.FieldPart(w.Area{Name: "rrule", Label: "Advanced", Text: "FREQ=WEEKLY", Mono: true}))
}

func TestField(t *testing.T) {
	plain := view.FormRow{Name: "login", Label: "Login", Kind: view.InputText}
	same(t, "a field", bind.Field(plain, w.FieldMD), w.Field{Label: "Login", Width: w.FieldMD, Parts: []w.FieldPart{w.Input{Name: "login", Label: "Login"}}})
	aside := view.FormRow{Name: "email", Label: "Email", LabelAside: "(optional)", Kind: view.InputEmail, Placeholder: "—"}
	same(t, "the muted part of the label", bind.Field(aside, w.FieldMD), w.Field{
		Label: "Email", LabelAside: w.Dim{Parts: []w.Inline{w.Text("(optional)")}}, Width: w.FieldMD,
		Parts: []w.FieldPart{w.Input{Name: "email", Label: "Email", Placeholder: "—"}},
	})
	hinted := view.FormRow{Name: "url", Label: "Server URL", Kind: view.InputURL, Hint: "The address of the server."}
	same(t, "a hint", bind.Field(hinted, w.FieldW100).Parts, []w.FieldPart{
		w.Input{Name: "url", Label: "Server URL"},
		w.FieldHint{Parts: []w.Inline{w.Text("The address of the server.")}},
	})
	refused := view.FormRow{Name: "password", Label: "Password", Kind: view.InputPassword, Hint: "At least 12.", Error: "Login or password is wrong."}
	same(t, "the reason in the hint's place", bind.Field(refused, w.FieldW100).Parts, []w.FieldPart{
		w.Input{Kind: w.InputPassword, Name: "password", Label: "Password", Error: true},
		w.FieldHint{Parts: []w.Inline{w.Text("Login or password is wrong.")}, Variant: w.HintDanger},
	})
}

func TestMsg(t *testing.T) {
	for name, c := range map[string]struct {
		n    view.Notice
		want w.Msg
	}{
		"an alert": {
			view.Notice{Role: view.NoticeAlert, Body: "Your password is the only key."},
			w.Msg{Parts: []w.MsgPart{w.Text("Your password is the only key.")}, Tone: w.MsgAlert, Space: w.MsgFlushBottom12},
		},
		"a title and a body": {
			view.Notice{Role: view.NoticeInfo, Title: "Rewritten in 12 records.", Body: "Nothing else changed."},
			w.Msg{Parts: []w.MsgPart{w.Strong("Rewritten in 12 records."), w.Text(" "), w.Text("Nothing else changed.")}, Space: w.MsgFlushBottom12},
		},
		"a title alone": {
			view.Notice{Role: view.NoticeAccent, Title: "Saved."},
			w.Msg{Parts: []w.MsgPart{w.Strong("Saved.")}, Tone: w.MsgAccent, Space: w.MsgFlushBottom12},
		},
	} {
		same(t, name, bind.Msg(c.n, w.MsgFlushBottom12), c.want)
	}
	same(t, "info is plain", bind.Tone(view.NoticeInfo), w.MsgPlain)
}
