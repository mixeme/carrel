package web_test

import (
	"html"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/view"
)

// The owner's deferred round, part 3: the contact form in the rubrics of
// s-contacts-3, «Add phone» and «Add reminder» as buttons that add a row
// (C-6.8 InputAdd), and the time zone of an event as the whole list of
// s-agenda-8. The frames are read here, not copied into the tests.

var frameAddButton = regexp.MustCompile(`(?s)<div class="tc-rubric">(.*?)</div>|<button class="tc-btn is-sm"[^>]*>(.*?)</button>`)

// frameAdds are the buttons of a frame's form that add a row, as «rubric /
// label».
func frameAdds(t *testing.T, id string) []string {
	t.Helper()
	frame := mockupFrame(t, id)
	frame = frame[strings.Index(frame, `class="tc-form"`):]
	var out []string
	rubric := ""
	for _, m := range frameAddButton.FindAllStringSubmatch(frame, -1) {
		if strings.HasPrefix(m[0], "<div") {
			rubric = frameText(m[1])
			continue
		}
		out = append(out, rubric+" / "+frameText(m[2]))
	}
	return out
}

// shownForm is serverForm without the fold of empty fields and without the
// buttons that add a row; a repeated row of a rubric is named once, as the
// frame labels only the first.
func shownForm(f view.Form) (fields, adds []string) {
	fields = []string{}
	for _, set := range f.Sets {
		if strings.HasPrefix(string(set.Legend), "Show ") {
			continue
		}
		for _, row := range set.Rows {
			item := string(set.Legend) + " / " + string(row.Label)
			switch {
			case row.Kind == view.InputHidden, row.Kind == view.InputRemove:
			case row.Name == "notebook":
				// Where the record is saved: drawn in the head, left of Cancel,
				// with no label (Typecase's note 72) — not a field of the body.
			case row.Kind == view.InputAdd:
				adds = append(adds, item)
			case !slices.Contains(fields, item):
				fields = append(fields, item)
			}
		}
	}
	return fields, adds
}

func headOf(items []string) []string {
	var out []string
	for _, it := range items {
		if strings.HasPrefix(it, " / ") {
			out = append(out, it)
		}
	}
	return out
}

// s-contacts-3: the head (Display name … Birthday), then Phone numbers,
// Email addresses and Addresses — each row a Label and a Number, an Address
// or the five parts of an address, and a button that adds one —, then Other.
// s-contacts-4: a card without a nickname or a birthday still shows both in
// the head. The empty fields fold into «Show N empty fields» at the end, and
// a card without addresses folds «Add address» there too (to-typecase 28).
func TestContactFormIsTheFramesRubrics(t *testing.T) {
	f := bookFixture(t, nil,
		"BEGIN:VCARD\nVERSION:3.0\nUID:anna\nFN:Анна Ковалёва\nN:Ковалёва;Анна;;;\nORG:Издательство «Слово»\n"+
			"TITLE:Выпускающий редактор\nBDAY:1987-03-14\nTEL;TYPE=CELL:+7 912 445-10-88\nTEL;TYPE=WORK:+7 495 120-45-00\n"+
			"EMAIL;TYPE=WORK:a.kovaleva@example.org\nADR;TYPE=home:;;ул. Тверская 12;Москва;;123001;Россия\n"+
			"CATEGORIES:работа,издательство\nNOTE:Пишет по вторникам.\nEND:VCARD\n",
		"BEGIN:VCARD\nVERSION:3.0\nUID:bare\nFN:Анна Ковалёва\nN:Ковалёва;Анна;;;\nORG:Издательство «Слово»\n"+
			"TITLE:Выпускающий редактор\nTEL;TYPE=CELL:+7 912 445-10-88\nEND:VCARD\n")
	page := decode[view.ContactEditPage](t, f.h.do(get(f.path("contacts")+"/anna/edit", f.sess)))
	fields, adds := shownForm(page.Form)
	if want := frameForm(t, "s-contacts-3"); !equalStrings(fields, want) {
		t.Errorf("contact form =\n%q,\nwant (s-contacts-3)\n%q", fields, want)
	}
	if want := frameAdds(t, "s-contacts-3"); !equalStrings(adds, want) {
		t.Errorf("buttons that add a row = %q, want (s-contacts-3) %q", adds, want)
	}
	for _, name := range []string{"tel.new", "tel.new.type", "email.new", "email.new.type"} {
		if hasRow(page.Form, name) {
			t.Errorf("row %q: the frame draws a button that adds a row, not an empty one", name)
		}
	}

	// The button carries the row it adds (C-6.8): the label from the list of
	// s-contacts-12, then the number.
	phone := addRow(t, page.Form, "tel")
	var names, labels []string
	for _, r := range phone.Template {
		names, labels = append(names, r.Name), append(labels, string(r.Label))
	}
	if !equalStrings(names, []string{"tel.{n}.type", "tel.{n}", "remove"}) || !equalStrings(labels, []string{"Label", "Number", "Remove"}) ||
		phone.Template[2].Kind != view.InputRemove || phone.Template[2].Value != "tel.{n}" {
		t.Errorf("Add phone adds %q %q %+v", names, labels, phone.Template)
	}
	if len(phone.Template) > 0 {
		sameOwnList(t, "added phone label", phone.Template[0].Options, "s-contacts-12")
	}

	// An address row is its label, picked from a list as a phone's is, then
	// the five parts.
	if label := rowNamed(t, page.Form, "adr.0.type"); label.Kind != view.InputSelect || selectedOf(label.Options)[0] != "home" {
		t.Errorf("the address's label = %+v", label)
	}
	for _, part := range []string{"adr.0.street", "adr.0.city", "adr.0.region", "adr.0.code", "adr.0.country"} {
		rowNamed(t, page.Form, part)
	}

	bare := decode[view.ContactEditPage](t, f.h.do(get(f.path("contacts")+"/bare/edit", f.sess)))
	got, _ := shownForm(bare.Form)
	if want := headOf(frameForm(t, "s-contacts-4")); !equalStrings(headOf(got), want) {
		t.Errorf("head of a card without a nickname and a birthday = %q, want (s-contacts-4) %q", headOf(got), want)
	}
	// The fold is the last thing of the form, after every rubric, and counts
	// the fields folded: the address, the websites and the dates (their
	// buttons), Tags and Note (s-contacts-3; Р26 of scans/kinds-parity.md,
	// Typecase's task 171).
	show := regexp.MustCompile(`<button class="tc-btn" type="button">(Show \d+ empty fields)</button>`).FindStringSubmatch(mockupFrame(t, "s-contacts-3"))
	if show == nil {
		t.Fatal("s-contacts-3 draws no «Show N empty fields»: read it again")
	}
	fold := bare.Form.Sets[len(bare.Form.Sets)-1]
	var folded []string
	for _, r := range fold.Rows {
		folded = append(folded, string(r.Label))
	}
	if want := regexp.MustCompile(`\d+`).ReplaceAllString(show[1], "5"); string(fold.Legend) != want ||
		!equalStrings(folded, []string{"Add address", "Add website", "Add date", "Tags", "Note"}) || fold.Rows[0].Kind != view.InputAdd {
		t.Errorf("fold = %q %q, want %q with Add address, Add website, Add date, Tags, Note", fold.Legend, folded, want)
	}
}

// s-contacts-3: every row of a phone, an e-mail and an address has its
// Remove. Without scripts it posts the form with remove=<row>: the form comes
// back without the row and nothing is written (C-6.8); Save then takes out
// exactly that value, and the rows after it keep their own parameters (S-7).
func TestRemoveIsOnEveryRow(t *testing.T) {
	frame := mockupFrame(t, "s-contacts-3")
	want := map[string]int{}
	for _, m := range regexp.MustCompile(`(?s)<div class="tc-fset">\s*<div class="tc-rubric">([^<]+)</div>(.*?)\n\s*</div>\n`).FindAllStringSubmatch(frame, -1) {
		if n := strings.Count(m[2], `<button class="tc-btn is-danger" type="button">Remove</button>`); n > 0 {
			want[m[1]] = n
		}
	}
	if want["Phone numbers"] != 2 || want["Email addresses"] != 1 || want["Addresses"] != 1 || len(want) != 3 {
		t.Fatalf("s-contacts-3 Remove buttons = %v: read it again", want)
	}
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada\n"+
		"TEL;TYPE=cell:+1 555 0100\nTEL;TYPE=work;PREF=1:+1 555 0101\nEMAIL;TYPE=work:ada@example.org\n"+
		"ADR;TYPE=home:;;1 Main St;London;;N1;UK\nEND:VCARD\n")
	edit := f.path("contacts") + "/ada/edit"
	page := decode[view.ContactEditPage](t, f.h.do(get(edit, f.sess)))
	got := map[string]int{}
	var removes []string
	for _, set := range page.Form.Sets {
		for i, row := range set.Rows {
			if row.Kind != view.InputRemove {
				continue
			}
			got[string(set.Legend)]++
			removes = append(removes, string(row.Value))
			if row.Name != "remove" || row.Label != "Remove" || i == 0 || !strings.HasPrefix(set.Rows[i-1].Name, string(row.Value)) {
				t.Errorf("Remove %+v does not close its row", row)
			}
		}
	}
	if len(got) != len(want) || got["Phone numbers"] != 2 || got["Email addresses"] != 1 || got["Addresses"] != 1 {
		t.Errorf("Remove per rubric = %v, the frame %v", got, want)
	}
	if !equalStrings(removes, []string{"tel.0", "tel.1", "email.0", "adr.0"}) {
		t.Errorf("Remove rows = %q", removes)
	}

	puts := f.srv.CountRequests("PUT", f.col)
	form := formFields(page.Form)
	form.Set("remove", "tel.0")
	less := decode[view.ContactEditPage](t, f.h.do(postForm(edit, f.sess, form)))
	if n := f.srv.CountRequests("PUT", f.col) - puts; n != 0 {
		t.Errorf("removing a row wrote %d record(s)", n)
	}
	for _, set := range less.Form.Sets {
		for _, row := range set.Rows {
			if strings.HasPrefix(row.Name, "tel.0") && row.Kind != view.InputHidden || row.Kind == view.InputRemove && row.Value == "tel.0" {
				t.Errorf("the removed row is drawn: %+v", row)
			}
		}
	}
	if v := rowNamed(t, less.Form, "tel.1").Value; v != "+1 555 0101" {
		t.Errorf("tel.1 = %q after removing tel.0", v)
	}
	// A second press, on the address, keeps the first.
	form = formFields(less.Form)
	form.Set("remove", "adr.0")
	less = decode[view.ContactEditPage](t, f.h.do(postForm(edit, f.sess, form)))
	if hasRow(less.Form, "adr.0.street") && rowNamed(t, less.Form, "adr.0.street").Kind != view.InputHidden {
		t.Error("the removed address is drawn")
	}
	form = formFields(less.Form)
	form.Del("remove")
	if resp := f.h.do(postForm(edit, f.sess, form)); resp.Code != http.StatusSeeOther {
		t.Fatalf("save: %d %s", resp.Code, resp.Body.String())
	}
	card := body(t, f, "ada.vcf")
	mustContain(t, card, "TEL;PREF=1;TYPE=work:+1 555 0101", "EMAIL;TYPE=work:ada@example.org")
	// The number whole: REV is a time, and one written at 00:10 holds «0100».
	if strings.Contains(card, "+1 555 0100") || strings.Contains(card, "ADR") {
		t.Errorf("the removed rows are still there:\n%s", card)
	}
}

func hasRow(f view.Form, name string) bool {
	for _, set := range f.Sets {
		for _, row := range set.Rows {
			if row.Name == name {
				return true
			}
		}
	}
	return false
}

// addRow is the button that adds a row of the field.
func addRow(t *testing.T, f view.Form, field string) view.FormRow {
	t.Helper()
	for _, set := range f.Sets {
		for _, row := range set.Rows {
			if row.Kind == view.InputAdd && string(row.Value) == field {
				return row
			}
		}
	}
	t.Fatalf("no button that adds a row of %q in %+v", field, f)
	return view.FormRow{}
}

// s-agenda-3: «Add reminder» is a button. Without scripts it posts the form
// with add=reminder, and the form comes back with one more row and nothing
// written (C-7); the row is then saved as any other.
func TestAddReminderIsAButton(t *testing.T) {
	f := calendarFixture(t, nil, "meet.ics", ics("BEGIN:VEVENT\nUID:meet\nDTSTAMP:20260101T000000Z\n"+
		"DTSTART:20260115T100000Z\nDTEND:20260115T110000Z\nSUMMARY:Review\n"+
		"BEGIN:VALARM\nACTION:DISPLAY\nTRIGGER:-PT10M\nDESCRIPTION:x\nEND:VALARM\nEND:VEVENT"))
	rec := f.path("calendar") + "/meet"
	page := decode[view.EventEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
	if hasRow(page.Form, "reminder.new") || hasRow(page.Form, "reminder.1") {
		t.Errorf("an empty reminder row is drawn: %+v", page.Form)
	}
	add := addRow(t, page.Form, "reminder")
	// The row and the fields «Custom…» opens, hidden (s-agenda-19).
	if add.Name != "add" || add.Label != "Add reminder" || len(add.Template) != 3 || add.Template[0].Name != "reminder.{n}" ||
		!add.Template[1].Hidden || !add.Template[2].Hidden {
		t.Fatalf("Add reminder = %+v", add)
	}
	sameList(t, "added reminder", add.Template[0].Options, "s-agenda-9")

	puts := f.srv.CountRequests("PUT", f.col)
	form := formFields(page.Form)
	form.Set("summary", "Review, typed")
	form.Set("add", "reminder")
	more := decode[view.EventEditPage](t, f.h.do(postForm(rec+"/edit", f.sess, form)))
	if n := f.srv.CountRequests("PUT", f.col) - puts; n != 0 {
		t.Errorf("adding a row wrote %d record(s)", n)
	}
	if v := rowNamed(t, more.Form, "reminder.0").Value; v != "-PT10M" {
		t.Errorf("reminder.0 = %q after adding a row", v)
	}
	if v := rowNamed(t, more.Form, "summary").Value; v != "Review, typed" {
		t.Errorf("summary = %q: what was typed is lost", v)
	}
	rowNamed(t, more.Form, "reminder.1")
	addRow(t, more.Form, "reminder")

	form = formFields(more.Form)
	form.Set("reminder.1", "-PT1H")
	if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != http.StatusSeeOther {
		t.Fatalf("save: %d %s", resp.Code, resp.Body.String())
	}
	mustContain(t, body(t, f, "meet.ics"), "TRIGGER:-PT10M", "TRIGGER:-PT1H", `SUMMARY:Review\, typed`)
}

// The same button on the contact form, and on a new contact, which has no
// row until one is added (s-contacts-3: «not empty fields poured in
// beforehand»).
func TestAddPhoneIsAButton(t *testing.T) {
	f := contactsFixture(t, false, "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada\nTEL;TYPE=cell:+1 555 0100\nEND:VCARD\n")
	list := f.path("contacts")

	// A new contact comes with one empty phone row (s-contacts-27), and
	// nothing more: no empty e-mail or address.
	fresh := decode[view.ContactEditPage](t, f.h.do(get(list+"/new", f.sess)))
	if !hasRow(fresh.Form, "tel.0") || hasRow(fresh.Form, "tel.1") || hasRow(fresh.Form, "email.0") {
		t.Errorf("a new contact comes without one empty phone row, or with more")
	}
	form := formFields(fresh.Form)
	form.Set("fn", "Bob")
	form.Set("add", "tel")
	more := decode[view.ContactEditPage](t, f.h.do(postForm(list+"/new", f.sess, form)))
	if len(f.srv.Paths(f.col)) != 1 {
		t.Errorf("adding a row created a record: %v", f.srv.Paths(f.col))
	}
	rowNamed(t, more.Form, "tel.1")
	rowNamed(t, more.Form, "tel.1.type")

	page := decode[view.ContactEditPage](t, f.h.do(get(list+"/ada/edit", f.sess)))
	form = formFields(page.Form)
	form.Set("add", "tel")
	more = decode[view.ContactEditPage](t, f.h.do(postForm(list+"/ada/edit", f.sess, form)))
	form = formFields(more.Form)
	form.Set("tel.1", "+1 555 0101")
	form.Set("tel.1.type", "work")
	if resp := f.h.do(postForm(list+"/ada/edit", f.sess, form)); resp.Code != http.StatusSeeOther {
		t.Fatalf("save: %d %s", resp.Code, resp.Body.String())
	}
	mustContain(t, body(t, f, "ada.vcf"), "TEL;TYPE=cell:+1 555 0100", "TEL;TYPE=work:+1 555 0101")
}

var (
	frameSheetLabel = regexp.MustCompile(`<label>([^<]*)</label>`)
	frameSheetField = regexp.MustCompile(`<label>([^<]*)</label>(?:<input(?:[^>]*?placeholder="([^"]*)")?[^>]*>|<label class="tc-check">(?:<input[^>]*>)?(.*?)</label>)?`)
	framePickLabel  = regexp.MustCompile(`<label>([^<]*)</label><span class="tc-in is-pick`)
)

// sheetFields are the fields of a quick sheet, «label [placeholder]»: the
// frame's, and the server's. A box labelled with a blank is named by its
// text (s-notes-9: All day).
func frameSheetFields(t *testing.T, id string) []string {
	t.Helper()
	var out []string
	for _, m := range frameSheetField.FindAllStringSubmatch(mockupFrame(t, id), -1) {
		label := frameText(m[1])
		if label == "" {
			label = frameText(m[3])
		}
		out = append(out, label+" ["+html.UnescapeString(m[2])+"]")
	}
	return out
}

// sheetFields are the rows a sheet shows: not its hidden fields, nor the
// numbers of «Custom…» while a step is chosen (FormRow.Hidden, s-agenda-19).
func sheetFields(f view.Form) []string {
	out := []string{}
	for _, set := range f.Sets {
		for _, row := range set.Rows {
			if row.Kind != view.InputHidden && !row.Hidden {
				ph := string(row.Placeholder)
				if row.Kind == view.InputSelect {
					ph = "" // a list's placeholder is its search (C-6.8)
				}
				out = append(out, string(row.Label)+" ["+ph+"]")
			}
		}
	}
	return out
}

// The quick sheets hold one row of a repeating field, without its label and
// without a button: Phone and Email (s-notes-8), Remind (s-notes-9). They
// are labelled their own way — Title, Date, Start, End, then All day — and an
// empty Phone and Email show the frame's samples (to-typecase 28). The
// event's sheet has one Date: the end is on its day, or on the next when it
// is earlier than the start.
func TestQuickSheetRowsAreTheFrames(t *testing.T) {
	f := contactsFixture(t, false)
	sheet := decode[view.NewSheet](t, f.h.do(get(f.path("contacts")+"/new/quick", f.sess)))
	if got, want := sheetFields(sheet.Form), frameSheetFields(t, "s-notes-8"); !equalStrings(got, want) {
		t.Errorf("contact quick sheet = %q, want (s-notes-8) %q", got, want)
	}
	form := formFields(sheet.Form)
	form.Set("fn", "Ada")
	form.Set("tel.0", "+1 555 0100")
	form.Set("email.0", "ada@example.org")
	if resp := f.h.do(postForm(f.path("contacts")+"/new/quick", f.sess, form)); resp.Code != http.StatusSeeOther {
		t.Fatalf("save: %d %s", resp.Code, resp.Body.String())
	}

	c := calendarFixture(t, nil)
	quick := c.path("calendar") + "/new/quick"
	events := decode[view.NewSheet](t, c.h.do(get(quick, c.sess)))
	if got, want := sheetFields(events.Form), frameSheetFields(t, "s-notes-9"); !equalStrings(got, want) {
		t.Errorf("event quick sheet = %q, want (s-notes-9) %q", got, want)
	}
	remind := framePickLabel.FindStringSubmatch(mockupFrame(t, "s-notes-9"))
	row := rowNamed(t, events.Form, "reminder.0")
	if remind == nil || string(row.Label) != frameText(remind[1]) {
		t.Errorf("the quick sheet's reminder = %q, want (s-notes-9) %q", row.Label, remind)
	}
	saved := func(set map[string]string) string {
		t.Helper()
		before := c.srv.Paths(c.col)
		form := formFields(events.Form)
		for k, v := range set {
			form.Set(k, v)
		}
		if resp := c.h.do(postForm(quick, c.sess, form)); resp.Code != http.StatusSeeOther {
			t.Fatalf("save %v: %d %s", set, resp.Code, resp.Body.String())
		}
		for _, p := range c.srv.Paths(c.col) {
			if !slices.Contains(before, p) {
				b, _, _ := c.srv.Object(p)
				return unfolded(b)
			}
		}
		t.Fatalf("save %v made nothing", set)
		return ""
	}
	mustContain(t, saved(map[string]string{"summary": "Documents", "start.date": "2026-08-11", "start.time": "18:00", "end.time": "18:40"}),
		"DTSTART:20260811T180000Z", "DTEND:20260811T184000Z")
	mustContain(t, saved(map[string]string{"summary": "Night", "start.date": "2026-08-11", "start.time": "23:00", "end.time": "01:00"}),
		"DTSTART:20260811T230000Z", "DTEND:20260812T010000Z")
	mustContain(t, saved(map[string]string{"summary": "Day", "start.date": "2026-08-11", "start.allday": "on"}),
		"DTSTART;VALUE=DATE:20260811")
	for _, set := range events.Form.Sets {
		for _, r := range set.Rows {
			if r.Kind == view.InputAdd {
				t.Errorf("the quick sheet draws %q; s-notes-9 has one reminder and no button", r.Label)
			}
		}
	}
}

var frameSheetLine = regexp.MustCompile(`(?s)</div>\s*<span class="tc-dim[^"]*tc-dim--wrap">(.*?)</span>\s*</div>\s*<div class="tc-sheet__foot">`)

// The quick sheets say their line under the fields (s-notes-8, s-notes-9,
// s-notes-10). The task's has no All day (from-typecase note 20, owner's
// decision): a due date without a time is due the whole day, no date is no
// due at all, and the date and the time show the frame's samples.
func TestQuickTaskSheetIsTheFrame(t *testing.T) {
	c := calendarFixture(t, nil)
	for _, k := range []struct{ kind, frame string }{{"contacts", "s-notes-8"}, {"calendar", "s-notes-9"}, {"tasks", "s-notes-10"}} {
		f := c
		if k.kind == "contacts" {
			f = contactsFixture(t, false)
		}
		line := frameOne(t, frameSheetLine, mockupFrame(t, k.frame), "line under the fields")[1]
		sheet := decode[view.NewSheet](t, f.h.do(get(f.path(k.kind)+"/new/quick", f.sess)))
		if string(sheet.Note) != line {
			t.Errorf("%s quick sheet line = %q, want (%s) %q", k.kind, sheet.Note, k.frame, line)
		}
	}
	// The note's sheet has no fields, only the body, and no line under them
	// (s-notes-5, from-typecase note 22).
	if frameSheetLine.MatchString(mockupFrame(t, "s-notes-5")) {
		t.Fatal("s-notes-5 draws a line under the body: read it again")
	}
	if note := decode[view.NewSheet](t, c.h.do(get(c.path("notes")+"/new/quick", c.sess))); note.Note != "" {
		t.Errorf("note quick sheet line = %q; s-notes-5 draws none", note.Note)
	}
	quick := c.path("tasks") + "/new/quick"
	tasks := decode[view.NewSheet](t, c.h.do(get(quick, c.sess)))
	// The frame's rows, «Remind» among them, as the event's sheet holds it
	// (Р9 of scans/kinds-parity.md; drawn since note 69, task 167).
	if got, want := sheetFields(tasks.Form), frameSheetFields(t, "s-notes-10"); !equalStrings(got, want) {
		t.Errorf("task quick sheet = %q, want (s-notes-10) %q", got, want)
	}
	// «No reminder» chosen at first (note 69).
	for _, o := range rowNamed(t, tasks.Form, "reminder.0").Options {
		if o.Selected && o.Label != "No reminder" {
			t.Errorf("task quick sheet reminds %q at first, the frame «No reminder»", o.Label)
		}
	}
	saved := func(set map[string]string) string {
		t.Helper()
		before := c.srv.Paths(c.col)
		form := formFields(tasks.Form)
		for k, v := range set {
			form.Set(k, v)
		}
		if resp := c.h.do(postForm(quick, c.sess, form)); resp.Code != http.StatusSeeOther {
			t.Fatalf("save %v: %d %s", set, resp.Code, resp.Body.String())
		}
		for _, p := range c.srv.Paths(c.col) {
			if !slices.Contains(before, p) {
				b, _, _ := c.srv.Object(p)
				return unfolded(b)
			}
		}
		t.Fatalf("save %v made nothing", set)
		return ""
	}
	mustContain(t, saved(map[string]string{"summary": "Documents", "due.date": "2026-08-11"}), "DUE;VALUE=DATE:20260811")
	mustContain(t, saved(map[string]string{"summary": "Call", "due.date": "2026-08-11", "due.time": "18:00"}), "DUE:20260811T180000Z")
	if b := saved(map[string]string{"summary": "Someday"}); strings.Contains(b, "DUE") || strings.Contains(b, "DTSTART") {
		t.Errorf("a task with no due date got one:\n%s", b)
	}
}

// A zone of s-agenda-8 is a radio button of the edit form (Typecase's task
// 73): its name, its value, its words.
var frameZoneItem = regexp.MustCompile(`(?s)<label class="[^"]*tc-menu__item[^"]*"><input[^>]*name="([^"]*)" value="([^"]*)"[^>]*>(.*?)</label>|<input[^>]*placeholder="([^"]*)"|<span class="tc-rubric">(.*?)</span>|<span class="tc-menu__sep"></span>`)

// zoneMenu is the zone list as s-agenda-8 draws it: «find: » the search,
// «# » a heading, «—» a line, then the zone; and the value each zone sends
// under the name of the form's row (sent).
func frameZoneMenu(t *testing.T) (out []string, sent map[string]string) {
	t.Helper()
	sent = map[string]string{}
	for _, m := range frameZoneItem.FindAllStringSubmatch(mockupFrame(t, "s-agenda-8"), -1) {
		switch {
		case strings.HasPrefix(m[0], "<label"):
			label := frameText(m[3])
			out = append(out, label)
			sent[label] = m[1] + "=" + m[2]
		case strings.HasPrefix(m[0], "<input"):
			out = append(out, "find: "+m[4])
		case strings.HasPrefix(m[0], `<span class="tc-rubric"`):
			out = append(out, "# "+frameText(m[5]))
		default:
			out = append(out, "—")
		}
	}
	return out, sent
}

func serverZoneMenu(row view.FormRow) []string {
	out := []string{"find: " + string(row.Placeholder)}
	for _, o := range row.Options {
		if o.Separator {
			out = append(out, "—")
		}
		if o.Group != "" {
			out = append(out, "# "+string(o.Group))
		}
		out = append(out, string(o.Label))
	}
	return out
}

// s-agenda-8: a search over all zones; the zone the event came in under «As
// sent» — Asia/Yekaterinburg, as the frame's event (task 77) —; a line; the
// zones; a line and Floating, always last. Every zone offered loads, and
// choosing one or Floating is written (S-11), at the clock the form shows:
// the form is in the event's own zone (s-agenda-3).
func TestZoneListIsTheFramesList(t *testing.T) {
	f := calendarFixture(t, nil,
		"meet.ics", ics("BEGIN:VEVENT\nUID:meet\nDTSTAMP:20260101T000000Z\n"+
			"DTSTART;TZID=Asia/Yekaterinburg:20260115T120000\nDTEND;TZID=Asia/Yekaterinburg:20260115T130000\nSUMMARY:Review\nEND:VEVENT"),
		"float.ics", ics("BEGIN:VEVENT\nUID:float\nDTSTAMP:20260101T000000Z\n"+
			"DTSTART:20260116T100000\nDTEND:20260116T110000\nSUMMARY:Anywhere\nEND:VEVENT"))
	rec := f.path("calendar") + "/meet"
	page := decode[view.EventEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
	row := rowNamed(t, page.Form, "tz")
	frame, sent := frameZoneMenu(t)
	got := serverZoneMenu(row)
	if len(frame) < 6 || len(sent) < 3 {
		t.Fatalf("s-agenda-8 = %q: the frame changed, read it again", frame)
	}
	// Each zone the frame draws sends what the row's option of it sends.
	for _, o := range row.Options {
		if want, ok := sent[string(o.Label)]; ok && want != row.Name+"="+o.Value {
			t.Errorf("zone %q sends %s=%s, the frame %s", o.Label, row.Name, o.Value, want)
		}
	}
	head, tail := frame[:4], frame[len(frame)-2:]
	if len(got) < 6 || !equalStrings(got[:4], head) || !equalStrings(got[len(got)-2:], tail) {
		t.Fatalf("zones begin %q and end %q, want (s-agenda-8) %q … %q", got[:min(4, len(got))], got[max(0, len(got)-2):], head, tail)
	}
	middle := got[4 : len(got)-2]
	at := 0
	for _, z := range frame[4 : len(frame)-2] {
		i := slices.Index(middle[at:], z)
		if i < 0 {
			t.Errorf("zone %q of s-agenda-8 is not offered after %q", z, middle[:at])
			continue
		}
		at += i + 1
	}
	if len(middle) < 300 {
		t.Errorf("%d zones offered; there are hundreds", len(middle))
	}
	for _, o := range row.Options[1 : len(row.Options)-1] {
		if _, err := time.LoadLocation(o.Value); err != nil || o.Separator && o != row.Options[1] || o.Group != "" {
			t.Errorf("zone %+v: %v", o, err)
		}
	}
	if !row.Options[0].Selected || row.Options[0].Value != "Asia/Yekaterinburg" || row.Options[len(row.Options)-1].Value != "floating" {
		t.Errorf("first %+v, last %+v", row.Options[0], row.Options[len(row.Options)-1])
	}

	save := func(zone string) string {
		t.Helper()
		form := formFields(decode[view.EventEditPage](t, f.h.do(get(rec+"/edit", f.sess))).Form)
		form.Set("tz", zone)
		if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != http.StatusSeeOther {
			t.Fatalf("POST %s: %d %s", zone, resp.Code, resp.Body.String())
		}
		return body(t, f, "meet.ics")
	}
	// The form shows 12:00 in Yekaterinburg; another zone keeps that clock.
	mustContain(t, save("Asia/Kathmandu"), "DTSTART;TZID=Asia/Kathmandu:20260115T120000", "TZID:Asia/Kathmandu", "BEGIN:VTIMEZONE")
	// Floating drops TZID: the clock the form shows, anywhere.
	floating := save("floating")
	mustContain(t, floating, "DTSTART:20260115T120000\r\n", "DTEND:20260115T130000\r\n")

	// An event that came floating has Floating under «As sent», once.
	float := rowNamed(t, decode[view.EventEditPage](t, f.h.do(get(f.path("calendar")+"/float/edit", f.sess))).Form, "tz")
	n := 0
	for _, o := range float.Options {
		if o.Value == "floating" {
			n++
		}
	}
	if float.Options[0].Value != "floating" || float.Options[0].Group != "As sent" || n != 1 {
		t.Errorf("a floating event's zones: first %+v, Floating %d times", float.Options[0], n)
	}
}
