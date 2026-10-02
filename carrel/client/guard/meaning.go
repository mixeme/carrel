package guard

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
)

// Meaning (U-8, U-15.2; U-5.1 and Typecase's Р-25, Р-27, docs/guide/screen.md
// «Проверка»). The frame draws how a control looks; where it leads and what
// it sends is the meaning of HTML, and a screen that draws the frame without
// it is built by look and does not work without scripts. The guard walks a
// screen's tree of widget values and holds every control in it to the
// contract of Typecase's guide: a link has its Href; a button leads (Href),
// submits a form by name (Form), is a submit button inside a form that sends
// (Submit in a Form with Action), adds a row (Adds), opens a sheet (Opens) or
// is disabled, or is the button of a closed menu, which opens it without a
// script; a field has its Name and stands in a form that sends; a tick
// box sends (Send.Form, or Send.Name inside a form); a search, a filter and
// a bar's ticks are forms (Action). A Form a button or a menu's item names
// must be on the page: a Form with that ID or a PostForm.
//
// The contract is a table by widget type (contract below). A widget of a
// type the table does not name, but that carries a field of meaning — Href,
// Action, To, a Form of text, a Send — is a finding too: a screen uses a
// control the guard cannot judge yet, and the table grows with the screens,
// by a decision written here, not by the control passing unseen.
//
// It is Typecase's TestEveryControlHasMeaning turned from the library to a
// screen: there every widget can carry its meaning, here every widget a
// screen draws carries it.

// meaningFields are the names that give a field of a widget its meaning in
// HTML, whatever the widget: where it goes or what it sends. Name is not
// among them — a person's name, a file's name are data — and is judged by
// the contract of each type that has it.
var meaningFields = []string{"Href", "Action", "To", "Form", "Send"}

// control is the state of the walk at a widget: the path to it, whether it
// stands in a form that sends, the type of the widget it is a part of, and
// whether it is the button of a closed menu (opens).
type control struct {
	v      reflect.Value
	path   string
	inForm bool
	holder string
	// opens: the widget is the trigger of a closed Drop or TopDrop with a
	// menu — the library prints it as the summary of a details, and it opens
	// the menu without a script (Typecase's task 60, U-8).
	opens bool
}

func (c control) str(name string) string {
	f := c.v.FieldByName(name)
	if !f.IsValid() || f.Kind() != reflect.String {
		return ""
	}
	return f.String()
}

func (c control) flag(name string) bool {
	f := c.v.FieldByName(name)
	return f.IsValid() && f.Kind() == reflect.Bool && f.Bool()
}

// send is the value of a field Send (Typecase's widget.Send): its form, name.
func (c control) send() (form, name string) {
	f := c.v.FieldByName("Send")
	if !f.IsValid() || f.Kind() != reflect.Struct {
		return "", ""
	}
	sc := control{v: f}
	return sc.str("Form"), sc.str("Name")
}

// rule judges one widget: what is wrong with its meaning, or "".
type rule func(c control) string

// needs is a rule that wants one of the fields filled.
func needs(fields ...string) rule {
	return func(c control) string {
		for _, f := range fields {
			if c.str(f) != "" {
				return ""
			}
		}
		return "no " + strings.Join(fields, ", no ")
	}
}

// field is the rule of a control that takes a value: its Name, in a form
// that sends. Disabled, it sends nothing and needs nothing.
func field(c control) string {
	if c.flag("Disabled") {
		return ""
	}
	if c.str("Name") == "" {
		return "a field without Name"
	}
	if !c.inForm {
		return "a field outside a form that sends (a Form with Action)"
	}
	return ""
}

// ticks is the rule of a tick box (a field Send): a submit button of a form
// named, or a checkbox of the form it stands in.
func ticks(c control) string {
	if c.flag("Glyph") { // only the drawn box inside another button
		return ""
	}
	if c.flag("Sign") { // a task's tick in a text: a sign, not pressed (Typecase's task 99)
		return ""
	}
	form, name := c.send()
	switch {
	case form != "":
		return ""
	case name != "" && c.inForm:
		return ""
	case name != "":
		return "a tick box with Send.Name outside a form that sends"
	}
	return "a tick box that sends nothing (Send)"
}

// links says whether every option of a switch (Seg) is a link.
func links(c control) bool {
	opts := c.v.FieldByName("Options")
	if !opts.IsValid() || opts.Kind() != reflect.Slice || opts.Len() == 0 {
		return false
	}
	for i := range opts.Len() {
		if (control{v: opts.Index(i)}).str("Href") == "" {
			return false
		}
	}
	return true
}

// buttonDisabled is widget.ButtonDisabled, inputSearch widget.InputSearch;
// the guard reads the widget's value by reflection and does not import the
// library.
const (
	buttonDisabled = 2
	inputSearch    = 3
)

// contract is the meaning each type of control of Typecase's widget package
// must carry (docs/guide/screen.md of Typecase, §3 and §5).
var contract = map[string]rule{
	"Button": func(c control) string {
		if c.v.FieldByName("State").Int() == buttonDisabled || c.opens || picksFile(c) {
			return ""
		}
		for _, f := range []string{"Href", "Form", "Adds", "Opens"} {
			if c.str(f) != "" {
				return ""
			}
		}
		if c.flag("Submit") && c.inForm {
			return ""
		}
		if c.flag("Submit") {
			return "a submit button outside a form that sends, and no Form"
		}
		return "a button with nothing to follow or send (Href, Form, Submit in a form, Adds, Opens)"
	},
	"Close": needs("Href"),
	"Link":  needs("Href"),
	// Words in a face (tc-dim--…): with Href a link — «Already have an
	// account? Sign in» (s-auth-2), an action of the model drawn as words
	// (bind.Span) —, without it words nobody presses, as a Text is. Neither
	// needs more; whether the words lead somewhere is the model's.
	"Span":    func(control) string { return "" },
	"NavItem": needs("Href"),
	// The account's button goes to the account's page, or opens its menu
	// (a closed TopDrop: the summary of a details).
	"UserMenu": func(c control) string {
		if c.opens {
			return ""
		}
		return needs("Href")(c)
	},
	// A line of text is a field — but the line of a menu that finds its
	// items (a search in a Menu: «Find a zone», s-agenda-8), which the
	// library's script filters by and which sends nothing; without a script
	// it is not there (Typecase's Menu).
	"Input": func(c control) string {
		if c.holder == "Menu" && c.v.FieldByName("Kind").Int() == inputSearch {
			return ""
		}
		return field(c)
	},
	// A choice is a field — but the choice that opens its menu (the trigger
	// of a closed Drop: the time zone, Typecase's task 73), whose radio items
	// send the value in the form it stands in.
	"Pick": func(c control) string {
		if c.opens {
			return ""
		}
		return field(c)
	},
	"Check": field,
	"Query": field,
	"Seg": func(c control) string {
		if c.flag("Theme") { // the theme is this browser's, kept by the script (U-12)
			return ""
		}
		if links(c) { // a switch of links: each option goes (SegOption.Href)
			return ""
		}
		return field(c)
	},
	// An option of a switch is judged on the switch (Seg).
	"SegOption": func(control) string { return "" },
	// A range of dates is a form (GET) that sends From and To: its fields
	// and its Show are judged in it.
	"Range": needs("Action"),
	// A line of a card: its value may go somewhere (Href: tel:, mailto:, the
	// record in another book) or be words — whether it goes is the model's,
	// as with a Span.
	"FieldsItem": func(control) string { return "" },
	// The rows of a list that are a record each, and the line of the build:
	// links (U-8 — every record is read without a script).
	"FaceRow":   needs("Href"),
	"FindRow":   needs("Href"),
	"LinkedRow": needs("Href"),
	"TimeRow":   needs("Href"),
	// A task's row (Typecase's task 95): its title is the link — the row
	// cannot be one, its tick is a control of its own, judged as a Tick.
	"DueRow": needs("Href"),
	// A row of the columns a person chose (Columns; s-tasks-13,
	// s-contacts-23): a link to the record, as the list's own row; a
	// task's tick in it is judged as a Tick.
	"ColumnRow": needs("Href"),
	"About":     needs("Href"),
	// A note's row (s-notes-1) and a neighbour of the rail's tree (s-notes-2):
	// links to the note, as the other rows of a list.
	"ExcerptRow": needs("Href"),
	"TreeRow":    needs("Href"),
	// A record a document's column points at (Related, the same note in
	// other notebooks) and an attachment there: a link where the model gives
	// an address — a record not loaded, a file that has none, are words —, as
	// with a Span.
	"DocRef":    func(control) string { return "" },
	"DocAttach": func(control) string { return "" },
	// A category (Typecase's task 63): with Href it goes to the list filtered
	// by it, without it is words — whether it goes is the model's (C-6.2
	// Tag.Href), as with a Span; in a row that is a link itself the library
	// prints it as words.
	"Tag": func(control) string { return "" },
	// A source of the rail with its tick (the owner's decision of
	// 30.09.2026, C-6.3): with the rail's form its tick sends the source's
	// choice (Send, as a Tick); on the rail of return the choice is made and
	// the rail sends none (Rail.Ticks is nil): the tick shows it, sends
	// nothing, and is no finding.
	"TickRow": func(c control) string {
		if form, name := c.send(); form == "" && name == "" {
			return ""
		}
		return ticks(c)
	},
	"MenuItem": func(c control) string {
		switch {
		case c.str("Reason") != "": // switched off, says why
			return ""
		case picksFile(c): // the label of a file input, judged as its FileField
			return ""
		case c.str("Href") != "":
			return ""
		case c.str("Form") != "": // submits a PostForm of the page (Sign out, Typecase's task 68)
			return ""
		case c.str("Name") != "" && c.inForm:
			return ""
		case c.str("Name") != "":
			return "a menu item with Name in a menu that is no form (Menu.Action)"
		}
		return "a menu item that leads nowhere (Href) and sends nothing (Name)"
	},
	// A file without a script (Typecase's task 109): the file input whose
	// label a button or a menu's item is (Button.File, MenuItem.File) — «From
	// this computer», the photo's Replace — sends its file under its Name
	// with a PostForm of the page (Form), which «Upload» beside it submits
	// without a script (NoScript, a Button or a MenuItem with Form).
	"FileField": func(c control) string {
		var msg []string
		if m := needs("Form")(c); m != "" {
			msg = append(msg, m)
		}
		if c.str("Name") == "" {
			msg = append(msg, "a file field without Name")
		}
		return strings.Join(msg, "; ")
	},
	// The forms: what sends. Their parts are judged in the form's context.
	"Form": func(control) string { return "" }, // a Form without Action is a group of fields: its fields say so
	// A document is a form in its edit (Typecase's task 98): with Action it
	// sends its fields, which the edit bar's Save names by its ID; read,
	// it has none and sends nothing.
	"Doc":      func(control) string { return "" },
	"PostForm": needs("Action"),
	"Menu":     func(control) string { return "" }, // a menu without Action is a list: its items say so
	"Search":   needs("Action"),
	"Filter":   needs("Action"),
	// A bar's ticks that are a form (GET, Typecase's task 67): its checks and
	// its Apply are judged in it.
	"CheckForm": needs("Action"),
	// A tick box. The rows and cells that hold one (TickRow, CardPick,
	// TableCell…) join when a screen draws them: whether their box is there
	// depends on the row, and that is decided with the screen.
	"Tick": ticks,
	// The tick box of a menu's item draws the item's state: the item sends
	// (its Name in the menu's form) and is judged as a MenuItem; elsewhere
	// (a row's pick) the box sends itself.
	"TickBox": func(c control) string {
		if c.holder == "MenuItem" {
			return ""
		}
		return ticks(c)
	},
	// A cell of a table: with a tick box that picks the row (Pick) it sends
	// the row's name (Send); its other parts are judged themselves.
	"TableCell": func(c control) string {
		if p := c.v.FieldByName("Pick"); p.IsValid() && p.Kind() == reflect.Pointer && !p.IsNil() {
			return ticks(c)
		}
		return ""
	},
	// What a tick box sends is judged on the box that has it.
	"Send": func(control) string { return "" },
}

// picksFile says whether a button or a menu's item is the label of a file
// input (File, Typecase's task 109): what it sends is its FileField's.
func picksFile(c control) bool {
	f := c.v.FieldByName("File")
	return f.IsValid() && f.Kind() == reflect.Pointer && !f.IsNil()
}

// opensForm says whether the widget is a form that sends, for its parts.
func opensForm(c control) bool {
	switch c.v.Type().Name() {
	case "Form", "Menu", "Search", "Filter", "Range", "CheckForm", "Doc":
		return c.str("Action") != ""
	}
	return false
}

// closedMenu says whether the widget is a closed button with its menu (Drop,
// TopDrop not Open) whose menu has items: its trigger opens the menu, without
// a script too — the library prints a closed one as a details whose summary
// is the button (Typecase's task 60). A trigger that is a link (Href) stays
// a link: it is judged as one.
func closedMenu(c control) bool {
	switch c.v.Type().Name() {
	case "Drop", "TopDrop":
	default:
		return false
	}
	if c.flag("Open") {
		return false
	}
	items := c.v.FieldByName("Menu").FieldByName("Items")
	return items.IsValid() && items.Kind() == reflect.Slice && items.Len() > 0
}

// carriesMeaning says whether a widget type has a field of meaning.
func carriesMeaning(t reflect.Type) bool {
	for _, name := range meaningFields {
		f, ok := t.FieldByName(name)
		if !ok || !f.IsExported() {
			continue
		}
		if f.Type.Kind() == reflect.String || (name == "Send" && f.Type.Kind() == reflect.Struct) {
			return true
		}
	}
	return false
}

// Meaning walks the widget tree v and names every control without its
// meaning, as «path: what». forms are the IDs of the PostForms the page
// prints beside the tree (page.Document.Forms); the Forms and PostForms of
// the tree are found in it.
func Meaning(v any, forms ...string) []string {
	rv := deref(reflect.ValueOf(v))
	if !rv.IsValid() || rv.Kind() != reflect.Struct {
		return []string{"the tree is not a widget"}
	}
	pkg := rv.Type().PkgPath()
	ids := slices.Clone(forms)
	type named struct{ path, form string }
	var out []string
	var names []named
	var walk func(c control)
	walk = func(c control) {
		t := c.v.Type()
		name := t.Name()
		if r, ok := contract[name]; ok {
			if msg := r(c); msg != "" {
				out = append(out, c.path+": "+msg)
			}
		} else if carriesMeaning(t) {
			out = append(out, fmt.Sprintf("%s: %s carries meaning the guard's contract does not know", c.path, name))
		}
		switch name {
		case "Form", "PostForm", "Doc":
			if id := c.str("ID"); id != "" && (name == "PostForm" || c.str("Action") != "") {
				ids = append(ids, id)
			}
		case "Button", "MenuItem", "FileField":
			if f := c.str("Form"); f != "" {
				names = append(names, named{c.path, f})
			}
		}
		if f, _ := c.send(); f != "" {
			names = append(names, named{c.path, f})
		}
		inForm := c.inForm || opensForm(c)
		closed := closedMenu(c)
		for i := range t.NumField() {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			fv := c.v.Field(i)
			at := c.path + "." + f.Name
			opens := closed && f.Name == "Trigger"
			switch f.Type.Kind() {
			case reflect.Struct, reflect.Pointer, reflect.Interface:
				if w := deref(fv); isControl(w, pkg) {
					walk(control{v: w, path: at + "(" + w.Type().Name() + ")", inForm: inForm, holder: name, opens: opens})
				}
			case reflect.Slice, reflect.Array:
				for j := range fv.Len() {
					if w := deref(fv.Index(j)); isControl(w, pkg) {
						walk(control{v: w, path: fmt.Sprintf("%s[%d](%s)", at, j, w.Type().Name()), inForm: inForm, holder: name})
					}
				}
			}
		}
	}
	walk(control{v: rv, path: rv.Type().Name()})
	for _, n := range names {
		if !slices.Contains(ids, n.form) {
			out = append(out, fmt.Sprintf("%s: submits form %q, which is not on the page", n.path, n.form))
		}
	}
	return out
}

// isControl says whether v is a value of the widget package to walk: every
// struct of it, the meaning-only ones included (a Hidden, a PostForm).
func isControl(v reflect.Value, pkg string) bool {
	return v.IsValid() && v.Kind() == reflect.Struct && v.Type().PkgPath() == pkg
}
