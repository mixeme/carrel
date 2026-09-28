package web_test

import (
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
			case row.Kind == view.InputHidden:
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

// s-contacts-3: the head (Display name … Birthday), then Phone numbers and
// Email addresses — each row a Label and a Number or an Address, and a button
// that adds one —, then Other. s-contacts-4: a card without a nickname or a
// birthday still shows both in the head.
func TestContactFormIsTheFramesRubrics(t *testing.T) {
	f := bookFixture(t, nil,
		"BEGIN:VCARD\nVERSION:3.0\nUID:anna\nFN:Анна Ковалёва\nN:Ковалёва;Анна;;;\nORG:Издательство «Слово»\n"+
			"TITLE:Выпускающий редактор\nBDAY:1987-03-14\nTEL;TYPE=CELL:+7 912 445-10-88\nTEL;TYPE=WORK:+7 495 120-45-00\n"+
			"EMAIL;TYPE=WORK:a.kovaleva@example.org\nCATEGORIES:работа,издательство\nNOTE:Пишет по вторникам.\nEND:VCARD\n",
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
	if !equalStrings(names, []string{"tel.{n}.type", "tel.{n}"}) || !equalStrings(labels, []string{"Label", "Number"}) {
		t.Errorf("Add phone adds %q %q", names, labels)
	}
	if len(phone.Template) == 2 {
		sameList(t, "added phone label", phone.Template[0].Options, "s-contacts-12")
	}

	bare := decode[view.ContactEditPage](t, f.h.do(get(f.path("contacts")+"/bare/edit", f.sess)))
	got, _ := shownForm(bare.Form)
	if want := headOf(frameForm(t, "s-contacts-4")); !equalStrings(headOf(got), want) {
		t.Errorf("head of a card without a nickname and a birthday = %q, want (s-contacts-4) %q", headOf(got), want)
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
	if add.Name != "add" || add.Label != "Add reminder" || len(add.Template) != 1 || add.Template[0].Name != "reminder.{n}" {
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

	fresh := decode[view.ContactEditPage](t, f.h.do(get(list+"/new", f.sess)))
	if hasRow(fresh.Form, "tel.0") {
		t.Errorf("a new contact comes with an empty phone row")
	}
	form := formFields(fresh.Form)
	form.Set("fn", "Bob")
	form.Set("add", "tel")
	more := decode[view.ContactEditPage](t, f.h.do(postForm(list+"/new", f.sess, form)))
	if len(f.srv.Paths(f.col)) != 1 {
		t.Errorf("adding a row created a record: %v", f.srv.Paths(f.col))
	}
	rowNamed(t, more.Form, "tel.0")
	rowNamed(t, more.Form, "tel.0.type")

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
	framePickLabel  = regexp.MustCompile(`<label>([^<]*)</label><button class="tc-in is-pick"`)
)

// The quick sheets hold one row of a repeating field, without its label and
// without a button: Phone and Email (s-notes-8), Remind (s-notes-9).
func TestQuickSheetRowsAreTheFrames(t *testing.T) {
	f := contactsFixture(t, false)
	sheet := decode[view.NewSheet](t, f.h.do(get(f.path("contacts")+"/new/quick", f.sess)))
	var want []string
	for _, m := range frameSheetLabel.FindAllStringSubmatch(mockupFrame(t, "s-notes-8"), -1) {
		want = append(want, frameText(m[1]))
	}
	got := []string{}
	for _, set := range sheet.Form.Sets {
		for _, row := range set.Rows {
			if row.Kind != view.InputHidden {
				got = append(got, string(row.Label))
			}
		}
	}
	if !equalStrings(got, want) {
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
	events := decode[view.NewSheet](t, c.h.do(get(c.path("calendar")+"/new/quick", c.sess)))
	remind := framePickLabel.FindStringSubmatch(mockupFrame(t, "s-notes-9"))
	row := rowNamed(t, events.Form, "reminder.0")
	if remind == nil || string(row.Label) != frameText(remind[1]) {
		t.Errorf("the quick sheet's reminder = %q, want (s-notes-9) %q", row.Label, remind)
	}
	for _, set := range events.Form.Sets {
		for _, r := range set.Rows {
			if r.Kind == view.InputAdd {
				t.Errorf("the quick sheet draws %q; s-notes-9 has one reminder and no button", r.Label)
			}
		}
	}
}

var frameZoneItem = regexp.MustCompile(`(?s)<input[^>]*placeholder="([^"]*)"|<span class="tc-rubric">(.*?)</span>|<span class="tc-menu__sep"></span>|<button class="[^"]*tc-menu__item[^"]*"[^>]*>(.*?)</button>`)

// zoneMenu is the zone list as s-agenda-8 draws it: «find: » the search,
// «# » a heading, «—» a line, then the zone.
func frameZoneMenu(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, m := range frameZoneItem.FindAllStringSubmatch(mockupFrame(t, "s-agenda-8"), -1) {
		switch {
		case strings.HasPrefix(m[0], "<input"):
			out = append(out, "find: "+m[1])
		case strings.HasPrefix(m[0], `<span class="tc-rubric"`):
			out = append(out, "# "+frameText(m[2]))
		case strings.HasPrefix(m[0], "<span"):
			out = append(out, "—")
		default:
			out = append(out, frameText(m[3]))
		}
	}
	return out
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
// sent»; a line; the zones; a line and Floating, always last. Every zone
// offered loads, and choosing one or Floating is written (S-11).
func TestZoneListIsTheFramesList(t *testing.T) {
	f := calendarFixture(t, nil,
		"meet.ics", ics("BEGIN:VEVENT\nUID:meet\nDTSTAMP:20260101T000000Z\n"+
			"DTSTART;TZID=Europe/Moscow:20260115T100000\nDTEND;TZID=Europe/Moscow:20260115T110000\nSUMMARY:Review\nEND:VEVENT"),
		"float.ics", ics("BEGIN:VEVENT\nUID:float\nDTSTAMP:20260101T000000Z\n"+
			"DTSTART:20260116T100000\nDTEND:20260116T110000\nSUMMARY:Anywhere\nEND:VEVENT"))
	rec := f.path("calendar") + "/meet"
	page := decode[view.EventEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
	row := rowNamed(t, page.Form, "tz")
	got, frame := serverZoneMenu(row), frameZoneMenu(t)
	if len(frame) < 6 {
		t.Fatalf("s-agenda-8 = %q: the frame changed, read it again", frame)
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
	if !row.Options[0].Selected || row.Options[0].Value != "Europe/Moscow" || row.Options[len(row.Options)-1].Value != "floating" {
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
	// Moscow at 10:00 is 07:00 UTC and 12:45 in Kathmandu.
	mustContain(t, save("Asia/Kathmandu"), "DTSTART;TZID=Asia/Kathmandu:20260115T124500", "TZID:Asia/Kathmandu", "BEGIN:VTIMEZONE")
	// Floating drops TZID: the clock time in the instance zone, anywhere.
	floating := save("floating")
	mustContain(t, floating, "DTSTART:20260115T070000\r\n", "DTEND:20260115T080000\r\n")

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
