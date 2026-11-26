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
	// scripted: the widget stands in an edit's bar (Docbar.Edit, Typecase's
	// task 104): without a script the bar draws only what leads or sends —
	// its Save and Cancel —, and the rest is the page's script's: the buttons
	// of the markup, «⋯», Auto-save, the width, Focus. Such a control needs no
	// meaning of HTML, and one that has it is held as any other.
	//
	// It is also a part of the queue the library's live layer fills while an
	// upload goes (Queue.Progress, Typecase's note 108): the templates of a
	// file's row and of a name taken — their answers are values of «answer»
	// the layer sends with the file —, and a queue's Hide that folds it in
	// place (Queue.Fold) where the server gave it nothing to follow.
	scripted bool
	// dialog: the widget stands in a page's own sheet without an address
	// (Shell.Dialogs, Typecase's note 134): only the library's script opens
	// it, and what shuts it — its cross, a button that Closes — is the
	// script's too. Everything else in it is held as anywhere.
	dialog bool
	// picture: the widget is a row of a tree, not the rail's, none of whose
	// rows leads or sends — the tree is a picture of a place, not a choice:
	// what a client sees at davloom's address (s-davloom-9)
	picture bool
}

// editBar tells whether a scripted control stands in an edit's bar
// (Docbar.Edit) rather than in an upload's queue.
func editBar(c control) bool {
	return strings.Contains(c.path, "(Docbar)") || strings.HasPrefix(c.path, "Docbar.")
}

// editBarButton is the meaning of a button of an edit's bar, which the
// library's script gives it (Typecase's note 229): the markup marks the text
// (Marks), Focus hides the rail and the column (Focuses); the bar's «⋯» and
// the trigger of a closed menu (Attach) open what they hold, and one that
// leads or sends — Save, Cancel — is held as anywhere.
func editBarButton(c control) string {
	if c.opens || strings.HasSuffix(c.path, ".More(Button)") || c.str("Marks") != "" || c.flag("Focuses") || c.v.FieldByName("State").Int() == buttonDisabled {
		return ""
	}
	for _, f := range []string{"Href", "Form"} {
		if c.str(f) != "" {
			return ""
		}
	}
	return "a button of an edit's bar the script does not know (Marks, Focuses)"
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

// sendsOf is the form a checkbox belongs to without standing in it
// (Send.Of: a row's pick of the selection's form, s-files-1), or "".
func (c control) sendsOf() string {
	f := c.v.FieldByName("Send")
	if !f.IsValid() || f.Kind() != reflect.Struct {
		return ""
	}
	return control{v: f}.str("Of")
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
// that sends. Disabled, it sends nothing and needs nothing; read only and
// nameless, it is an address shown to be copied — nothing to send (the
// link just made, s-admin-1) — or the name of the file picked beside it
// (Picked, read-only too: the import's file, s-io-3).
func field(c control) string {
	if c.flag("Disabled") || (c.flag("ReadOnly") || c.flag("Picked")) && c.str("Name") == "" {
		return ""
	}
	if c.str("Name") == "" {
		return "a field without Name"
	}
	// A field that names its form (Of) stands where its frame puts it: the
	// address of the installation check in the head, its form a PostForm of
	// the page (s-admin-4).
	if !c.inForm && c.str("Of") == "" {
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
	case name != "" && c.sendsOf() != "": // a field of the form it names (Typecase's task 114)
		return ""
	case name != "":
		return "a tick box with Send.Name outside a form that sends"
	}
	return "a tick box that sends nothing (Send)"
}

// pictureTree says whether no row of a tree leads anywhere or sends
// anything: such a tree is a picture, its rows words (s-davloom-1).
func pictureTree(c control) bool {
	rows := c.v.FieldByName("Rows")
	if !rows.IsValid() || rows.Kind() != reflect.Slice {
		return false
	}
	for i := range rows.Len() {
		r := control{v: rows.Index(i)}
		if form, name := r.send(); r.str("Href") != "" || name != "" || form != "" {
			return false
		}
	}
	return true
}

// links says whether every option of a switch (Seg) goes somewhere: a link,
// or a button that sends a form of the page to its address (a davloom
// device's access, s-davloom-1).
func links(c control) bool {
	opts := c.v.FieldByName("Options")
	if !opts.IsValid() || opts.Kind() != reflect.Slice || opts.Len() == 0 {
		return false
	}
	for i := range opts.Len() {
		o := control{v: opts.Index(i)}
		if o.str("Href") == "" && (o.str("Form") == "" || o.str("To") == "") {
			return false
		}
	}
	return true
}

// tickRow is a row with a tick in front: the tick sends with Send, or shows
// a choice made and sends nothing.
func tickRow(c control) string {
	if form, name := c.send(); form == "" && name == "" {
		return ""
	}
	return ticks(c)
}

// buttonDisabled is widget.ButtonDisabled, inputSearch widget.InputSearch;
// the guard reads the widget's value by reflection and does not import the
// library.
const (
	buttonDisabled = 2
	inputSearch    = 3
)

// liveQueuePart says whether the field of a Queue is what its live layer
// works without the server (control.scripted): Row and Taken of a queue
// that follows an upload's form, Toggle of one that folds.
func liveQueuePart(c control, name, field string) bool {
	if name != "Queue" {
		return false
	}
	switch field {
	case "Row", "Taken":
		return c.str("Progress") != ""
	case "Toggle":
		fold := c.v.FieldByName("Fold")
		return fold.IsValid() && !fold.IsNil()
	}
	return false
}

// contract is the meaning each type of control of Typecase's widget package
// must carry (docs/guide/screen.md of Typecase, §3 and §5).
var contract = map[string]rule{
	"Button": func(c control) string {
		if c.scripted && editBar(c) {
			return editBarButton(c)
		}
		if c.v.FieldByName("State").Int() == buttonDisabled || c.opens || c.scripted || picksFile(c) || c.flag("ScriptOnly") || c.flag("Removes") {
			return ""
		}
		if c.dialog && c.flag("Closes") {
			return ""
		}
		// CopyFrom copies a block that stands on the page: without a script
		// it is read there (the password shown once, s-admin-30); Copy puts
		// the words it carries on the clipboard, and they stand beside it to
		// be selected (a device's password, s-plan-2).
		for _, f := range []string{"Href", "Form", "Adds", "Opens", "CopyFrom", "Copy"} {
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
	"Close": func(c control) string {
		if c.dialog {
			return ""
		}
		return needs("Href")(c)
	},
	"Link": needs("Href"),
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
	"Check": func(c control) string {
		if c.scripted && editBar(c) && c.str("AutoSave") == "" {
			return "Auto-save of an edit's bar the script does not know (no AutoSave)"
		}
		if c.scripted { // Auto-save of an edit's bar
			return ""
		}
		return field(c)
	},
	"Query": field,
	"Seg": func(c control) string {
		if c.flag("Theme") { // the theme is this browser's, kept by the script (U-12)
			return ""
		}
		if c.scripted { // the width of the text in an edit's bar
			return ""
		}
		if links(c) { // each option goes (SegOption.Href) or sends a form (Form, To)
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
	// A record found by an attach sheet, brief (s-mail-5): a link to the
	// sheet with it chosen.
	"BriefRow": needs("Href"),
	// A file found by the search (s-find-1): its collection's mark where a
	// folder's row has its tick, and a link to the file in its folder.
	"SizeRow": needs("Href"),
	// A node of a tree: a link (a neighbour of the rail, a folder that
	// opens), or a radio of the form it stands in — where to put a thing
	// (Typecase's task 116; the move sheet, s-contacts-2) —, whose Href, if
	// any, is the page with it chosen; a node that cannot be chosen is words.
	//
	// A row at the tree's first level with an icon and no mark that leads
	// nowhere heads the rows under it — a server over its collections or
	// storages (s-mail-5, -6, -10, -12) —: words.
	"TreeRow": func(c control) string {
		if c.str("Href") != "" || c.flag("Unavailable") || c.picture {
			return ""
		}
		if c.str("Icon") != "" && c.v.FieldByName("Indent").Int() == 0 && c.v.FieldByName("Mark").IsNil() {
			return ""
		}
		if form, name := c.send(); name != "" && (form != "" || c.inForm) {
			return ""
		}
		return "a node that leads nowhere (Href) and sends nothing (Send in a form)"
	},
	// A record a document's column points at (Related, the same note in
	// other notebooks) and an attachment there: a link where the model gives
	// an address — a record not loaded, a file that has none, are words —, as
	// with a Span.
	"DocRef":    func(control) string { return "" },
	"DocAttach": func(control) string { return "" },
	// A file given with a published note (s-publish-8): the link of its
	// own the guest fetches it by.
	"DocFile": needs("Href"),
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
	"TickRow": tickRow,
	// An account of a new device ticked where it goes in (s-plan-1): its
	// tick, as a source's, sends the choice or shows it.
	"SlimRow": tickRow,
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
		case c.str("To") != "" && c.inForm: // sends the form it stands in to its own address (Typecase's task 122)
			return ""
		case c.str("To") != "":
			return "a menu item with To outside a form that sends"
		case c.str("Name") != "" && c.inForm:
			return ""
		case c.str("Name") != "" && c.str("Of") != "": // a radio that names its form (Typecase's task 203)
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
	// without a script (NoScript, a Button or a MenuItem with Form). One of a
	// new record's form (To, Typecase's task 122) sends the form it stands in
	// to its own address, as its «Upload» does (a Button or a MenuItem with
	// To): it must stand in one. A field of the form it stands in, with
	// neither, goes with that form's own button: the archive of a restore
	// (s-backup-15).
	"FileField": func(c control) string {
		var msg []string
		switch {
		case c.str("Form") == "" && c.str("To") == "" && c.inForm:
		case c.str("Form") == "" && c.str("To") != "":
			if !c.inForm {
				msg = append(msg, "a file field with To outside a form that sends")
			}
		default:
			if m := needs("Form")(c); m != "" {
				msg = append(msg, m)
			}
		}
		if c.str("Name") == "" {
			msg = append(msg, "a file field without Name")
		}
		return strings.Join(msg, "; ")
	},
	// The forms: what sends. Their parts are judged in the form's context.
	"Form": func(control) string { return "" }, // a Form without Action is a group of fields: its fields say so
	// A field is a group — a label and its controls — and with Action a GET
	// form of its own, which Enter in its line sends (the duplicates'
	// threshold, Typecase's task 131, 80d1319): its controls say whether they
	// stand in a form that sends. A card with Action is a form the same way
	// (a group of duplicates sends its choices and the button pressed).
	"Field": func(control) string { return "" },
	"Card":  func(control) string { return "" },
	// A group of fields with Action is a form of its own too (a settings
	// page's name with its Save, Typecase's task 149, f30d992); without it
	// the group is a group, and its fields say where they stand.
	"FieldSet": func(control) string { return "" },
	// A dialog with Action is a form too (a wizard's step whose fields stand
	// in several cards, s-backup-7, Typecase's task 149, bd321d8); without it
	// the dialog is a frame, and its parts say where they stand. A dialog one
	// of whose parts is a form is printed plain by the library, and that part
	// sends — its own Action is judged on it.
	"Dialog": func(control) string { return "" },
	// A sheet with Action is a form too (Typecase's task 134; the move
	// sheet, s-contacts-2): its parts are judged in it; without Action it
	// is a frame, and its buttons say where they send.
	"Sheet": func(control) string { return "" },
	// A comparison with Action is a form too (Carrel's task 132; the
	// conflict, s-conflict-1): each card's versions are radios of its name,
	// and «Write this combination» names it by its ID; without Action it
	// shows and sends nothing.
	"Diff": func(control) string { return "" },
	// A document is a form in its edit (Typecase's task 98): with Action it
	// sends its fields, which the edit bar's Save names by its ID; read,
	// it has none and sends nothing.
	"Doc":      func(control) string { return "" },
	"PostForm": needs("Action"),
	"Menu":     func(control) string { return "" }, // a menu without Action is a list: its items say so
	"Search":   needs("Action"),
	// A filter of its own form (GET, Action) — or whose line is a field of a
	// form elsewhere on the page (Query.Of: the search of an attach sheet,
	// Typecase's note 110 p. 1), judged as that field.
	"Filter": func(c control) string {
		if c.str("Action") != "" || c.v.FieldByName("Query").FieldByName("Of").String() != "" {
			return ""
		}
		return "no Action"
	},
	// A bar's ticks that are a form (GET, Typecase's task 67): its checks and
	// its Apply are judged in it.
	"CheckForm": needs("Action"),
	// A bar that is a form (GET, Typecase's 195 p. 2; the journal's action
	// and day, s-admin-7): its menu's radios, its fields and its Show are
	// judged in it.
	"BarForm": needs("Action"),
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
	// The head of a table's column (the files, s-files-1): a tick box that
	// picks every row (Tick) sends what the selection's form names (Send), as
	// a row's pick does; with nothing to send it is the library's script's
	// «every row» (the frame draws it so); without a tick it is words.
	"TableCol": func(c control) string {
		if form, name := c.send(); name == "" && form == "" {
			return ""
		}
		if p := c.v.FieldByName("Tick"); p.IsValid() && p.Kind() == reflect.Pointer && !p.IsNil() {
			return ticks(c)
		}
		return ""
	},
	// A folder above the one open (Crumbs, s-files-1): a link to it; the open
	// one is words (Crumbs.Current).
	"Crumb": needs("Href"),
	// The name of a table's row (TableFile, Typecase's task 114): with Href
	// a link — a storage or a folder opens by it, a file is chosen
	// (s-files-6, -1) —, without it a name shown, as a collection's on the
	// connections' page (s-profile-1). Whether it leads somewhere is the
	// model's.
	"TableFile": func(control) string { return "" },
	// A tile's name (Tile, s-files-5): a link, as a row's is.
	"Tile": needs("Href"),
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
	case "Form", "Menu", "Search", "Filter", "Range", "CheckForm", "BarForm", "Doc", "Field", "Card", "FieldSet", "Dialog", "Sheet", "Diff":
		return c.str("Action") != ""
	}
	return false
}

// closedMenu says whether the widget is a closed button with its menu (Drop,
// TopDrop, a split's arrow SplitDrop — not Open) whose menu has items: its trigger opens the menu, without
// a script too — the library prints a closed one as a details whose summary
// is the button (Typecase's task 60). A trigger that is a link (Href) stays
// a link: it is judged as one.
func closedMenu(c control) bool {
	switch c.v.Type().Name() {
	case "Drop", "TopDrop", "SplitDrop":
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
		case "Form", "PostForm", "Doc", "Sheet", "Diff":
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
		scripted := name == "Docbar" && c.flag("Edit")
		// A tree none of whose rows leads or sends is a picture — what a
		// client sees at davloom's address, a sheet of its own (s-davloom-9);
		// a tree of the rail is a way somewhere, and a row of it that goes
		// nowhere is a finding.
		picture := name == "Tree" && c.holder != "RailSec" && pictureTree(c)
		for i := range t.NumField() {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			fv := c.v.Field(i)
			at := c.path + "." + f.Name
			opens := closed && (f.Name == "Trigger" || f.Name == "Arrow")
			scripted := scripted || (c.scripted && name == "QueueItem") || liveQueuePart(c, name, f.Name)
			dialog := c.dialog || (name == "Shell" && f.Name == "Dialogs")
			switch f.Type.Kind() {
			case reflect.Struct, reflect.Pointer, reflect.Interface:
				if w := deref(fv); isControl(w, pkg) {
					walk(control{v: w, path: at + "(" + w.Type().Name() + ")", inForm: inForm, holder: name, opens: opens, scripted: scripted, dialog: dialog, picture: picture})
				}
			case reflect.Slice, reflect.Array:
				for j := range fv.Len() {
					if w := deref(fv.Index(j)); isControl(w, pkg) {
						walk(control{v: w, path: fmt.Sprintf("%s[%d](%s)", at, j, w.Type().Name()), inForm: inForm, holder: name, scripted: scripted, dialog: dialog, picture: picture})
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
