package web_test

import (
	"net/url"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// from-typecase note 6: the lists of the select fields are drawn beside
// their screens, and a list the server sends is the frame's — the same
// choices, in the same order, a line where the frame draws one. The frame is
// read here, not copied into the test.

// An item of a list is a button of the menu, or a radio button of the form
// the menu stands in (MenuItem with a Name, Typecase's task 73: the time
// zone of s-agenda-8).
var frameListItem = regexp.MustCompile(`(?s)<span class="tc-menu__sep"></span>|<button class="[^"]*tc-menu__item[^"]*"[^>]*role="menuitem"[^>]*>(.*?)</button>|<label class="[^"]*tc-menu__item[^"]*"><input[^>]*role="menuitemradio"[^>]*>(.*?)</label>`)

// frameList is the list a frame draws open, item by item, «— » before an
// item a line stands above. The item shown chosen is one of them.
func frameList(t *testing.T, id string) []string {
	t.Helper()
	var out []string
	line := false
	for _, m := range frameListItem.FindAllStringSubmatch(mockupFrame(t, id), -1) {
		if strings.HasPrefix(m[0], "<span") {
			line = true
			continue
		}
		item := frameText(m[1] + m[2])
		if line {
			item, line = "— "+item, false
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		t.Fatalf("%s draws no list: the frame changed, read it again", id)
	}
	return out
}

// frameForeign splits a label list that draws, under a line, the label
// another client wrote (s-contacts-12, -13, -22; from-typecase notes 20, 22
// and 24): the list any row is offered, and that label, as it is. The frame
// draws the list of the row whose label it is, and there it is chosen.
func frameForeign(t *testing.T, id string) (own []string, foreign string) {
	t.Helper()
	list := frameList(t, id)
	last := list[len(list)-1]
	if !strings.HasPrefix(last, "— ") {
		t.Fatalf("%s = %q, no label under a line: read it again", id, list)
	}
	foreign = strings.TrimPrefix(last, "— ")
	if chosen := frameChosen(t, id); chosen != foreign {
		t.Fatalf("%s chooses %q, not the label under the line %q: read it again", id, chosen, foreign)
	}
	return list[:len(list)-1], foreign
}

var frameChosenItem = regexp.MustCompile(`<button class="(?:[^"]* )?is-on [^"]*tc-menu__item[^"]*"[^>]*role="menuitem"[^>]*>(.*?)</button>`)

// frameChosen is the item a frame draws chosen in an open list.
func frameChosen(t *testing.T, id string) string {
	t.Helper()
	m := frameChosenItem.FindAllStringSubmatch(mockupFrame(t, id), -1)
	if len(m) != 1 {
		t.Fatalf("%s chooses %d items: read it again", id, len(m))
	}
	return frameText(m[0][1])
}

// sameOwnList is sameList for a row without a label of another client's:
// the frame's list without the one under the line.
func sameOwnList(t *testing.T, what string, got []view.Option, frame string) {
	t.Helper()
	if want, _ := frameForeign(t, frame); !equalStrings(serverList(got), want) {
		t.Errorf("%s = %q,\nwant (%s without the foreign label) %q", what, serverList(got), frame, want)
	}
}

// serverList is a list the server sent, written the way frameList writes one.
func serverList(opts []view.Option) []string {
	out := []string{}
	for _, o := range opts {
		item := string(o.Label)
		if o.Separator {
			item = "— " + item
		}
		out = append(out, item)
	}
	return out
}

func sameList(t *testing.T, what string, got []view.Option, frame string) {
	t.Helper()
	if g, want := serverList(got), frameList(t, frame); !equalStrings(g, want) {
		t.Errorf("%s = %q,\nwant (%s) %q", what, g, frame, want)
	}
}

func selectedOf(opts []view.Option) []string {
	var out []string
	for _, o := range opts {
		if o.Selected {
			out = append(out, o.Value)
		}
	}
	return out
}

// The task's status and priority are the lists of s-tasks-8 and s-tasks-9. A
// task without STATUS shows Needs action, and a save that does not touch the
// field writes none: "" and NEEDS-ACTION are the same state (S-7).
func TestTaskListsAreTheFrames(t *testing.T) {
	f := calendarFixture(t, nil,
		"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Slides\nPRIORITY:1\nEND:VTODO"))
	rec := f.path("tasks") + "/t1"
	page := decode[view.TaskEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
	status := rowNamed(t, page.Form, "status")
	sameList(t, "task status", status.Options, "s-tasks-8")
	if sel := selectedOf(status.Options); len(sel) != 1 || sel[0] != "NEEDS-ACTION" {
		t.Errorf("a task without STATUS selects %q, want Needs action", sel)
	}
	sameList(t, "priority", rowNamed(t, page.Form, "priority").Options, "s-tasks-9")

	form := formFields(page.Form)
	form.Set("location", "Room 4")
	if resp := f.h.do(postForm(rec+"/edit", f.sess, form)); resp.Code != 303 {
		t.Fatalf("save: %d %s", resp.Code, resp.Body.String())
	}
	got := body(t, f, "t1.ics")
	mustContain(t, got, "LOCATION:Room 4")
	if strings.Contains(got, "STATUS") {
		t.Errorf("a save that left Status alone wrote one:\n%s", got)
	}
}

// The event's status, reminders, repeat and floating zone are the lists of
// s-agenda-7, -9, -10 and -8; the card names the status as the list does.
func TestEventListsAreTheFrames(t *testing.T) {
	f := calendarFixture(t, nil,
		"e1.ics", ics("BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260101T000000Z\nDTSTART:20260810T100000\nDTEND:20260810T110000\n"+
			"SUMMARY:Sync-up\nSTATUS:CONFIRMED\nRRULE:FREQ=WEEKLY;BYDAY=MO\nBEGIN:VALARM\nACTION:DISPLAY\nTRIGGER:-PT10M\nDESCRIPTION:x\nEND:VALARM\nEND:VEVENT"),
		"e2.ics", ics("BEGIN:VEVENT\nUID:e2\nDTSTAMP:20260101T000000Z\nDTSTART:20260811T100000Z\nSUMMARY:Odd\n"+
			"BEGIN:VALARM\nACTION:DISPLAY\nTRIGGER:-PT7M\nDESCRIPTION:x\nEND:VALARM\nEND:VEVENT"))
	rec := f.path("calendar") + "/e1"
	page := decode[view.EventEditPage](t, f.h.do(get(rec+"/edit", f.sess)))
	sameList(t, "event status", rowNamed(t, page.Form, "status").Options, "s-agenda-7")
	sameList(t, "reminder", rowNamed(t, page.Form, "reminder.0").Options, "s-agenda-9")
	sameList(t, "note reminder list", rowNamed(t, page.Form, "reminder.0").Options, "s-notes-18")
	sameList(t, "frequency", rowNamed(t, page.Form, "repeat.freq").Options, "s-agenda-10")
	zones := frameList(t, "s-agenda-8")
	floating := strings.TrimPrefix(zones[len(zones)-1], "— ")
	offered := false
	for _, o := range rowNamed(t, page.Form, "tz").Options {
		offered = offered || (o.Value == "floating" && string(o.Label) == floating)
	}
	if !offered {
		t.Errorf("zones = %+v, want %q for a floating event", rowNamed(t, page.Form, "tz").Options, floating)
	}

	// A reminder no step of the list names comes as «Custom…» with its
	// numbers, not as one more item (Typecase's task 156, s-agenda-19).
	odd := decode[view.EventEditPage](t, f.h.do(get(f.path("calendar")+"/e2/edit", f.sess)))
	if sel := selectedOf(rowNamed(t, odd.Form, "reminder.0").Options); len(sel) != 1 || sel[0] != "custom" ||
		rowNamed(t, odd.Form, "reminder.0.count").Value != "7" || rowNamed(t, odd.Form, "reminder.0.unit").Value != "minutes" {
		t.Errorf("a held 7 minutes selects %q", sel)
	}

	card := eventAt(t, f, "e1")
	for _, fl := range card.Fields {
		if fl.Label == "Status" && fl.Value != "Confirmed" {
			t.Errorf("the card says %q, the list says Confirmed", fl.Value)
		}
	}
}

// A phone is labelled from s-contacts-12, an address from s-contacts-13.
// TYPE=CELL is «mobile», whatever its case, and is offered once; a row with
// no label selects nothing.
func TestContactLabelListsAreTheFrames(t *testing.T) {
	f := bookFixture(t, nil, "BEGIN:VCARD\nVERSION:3.0\nUID:ann\nFN:Ann\n"+
		"TEL;TYPE=CELL:+1 555 0100\nTEL:+1 555 0101\nTEL;TYPE=WORK:+1 555 0102\nEMAIL;TYPE=INTERNET:ann@example.org\nEND:VCARD\n")
	page := decode[view.ContactEditPage](t, f.h.do(get(f.path("contacts")+"/ann/edit", f.sess)))
	phone := rowNamed(t, page.Form, "tel.0.type")
	sameOwnList(t, "phone label", phone.Options, "s-contacts-12")
	if sel := selectedOf(phone.Options); len(sel) != 1 || sel[0] != "CELL" {
		t.Errorf("TYPE=CELL selects %q, want mobile keeping the value", sel)
	}
	if sel := selectedOf(rowNamed(t, page.Form, "tel.1.type").Options); len(sel) != 0 {
		t.Errorf("a row with no label selects %q", sel)
	}
	if sel := selectedOf(rowNamed(t, page.Form, "tel.2.type").Options); len(sel) != 1 || sel[0] != "WORK" {
		t.Errorf("TYPE=WORK selects %q", sel)
	}
	sameOwnList(t, "new phone label", addRow(t, page.Form, "tel").Template[0].Options, "s-contacts-12")
	sameOwnList(t, "email label", addRow(t, page.Form, "email").Template[0].Options, "s-contacts-13")
	// INTERNET is no label (TestTypeValuesThatAreNoLabel): nothing is
	// chosen, and the save below keeps it.
	if sel := selectedOf(rowNamed(t, page.Form, "email.0.type").Options); len(sel) != 0 {
		t.Errorf("TYPE=INTERNET selects %q", sel)
	}

	// Saved untouched, nothing of the labels changes.
	before := body(t, f, "ann.vcf")
	form := formFields(page.Form)
	form.Set("fn", "Ann B")
	if resp := f.h.do(postForm(f.path("contacts")+"/ann/edit", f.sess, form)); resp.Code != 303 {
		t.Fatalf("save: %d %s", resp.Code, resp.Body.String())
	}
	after := body(t, f, "ann.vcf")
	for _, line := range []string{"TEL;TYPE=CELL:+1 555 0100", "TEL:+1 555 0101", "TEL;TYPE=WORK:+1 555 0102", "EMAIL;TYPE=INTERNET:ann@example.org"} {
		if strings.Contains(before, line) && !strings.Contains(after, line) {
			t.Errorf("%q changed:\n%s", line, after)
		}
	}

	// «mobile» and «main» are written the way other clients write them.
	form = formFields(decode[view.ContactEditPage](t, f.h.do(get(f.path("contacts")+"/ann/edit", f.sess))).Form)
	form.Set("tel.1.type", optionValue(t, rowNamed(t, page.Form, "tel.1.type").Options, "main"))
	form.Set("tel.3", "+1 555 0103")
	form.Set("tel.3.type", optionValue(t, addRow(t, page.Form, "tel").Template[0].Options, "mobile"))
	if resp := f.h.do(postForm(f.path("contacts")+"/ann/edit", f.sess, form)); resp.Code != 303 {
		t.Fatalf("save: %d %s", resp.Code, resp.Body.String())
	}
	mustContain(t, body(t, f, "ann.vcf"), "TEL;TYPE=main:+1 555 0101", "TEL;TYPE=cell:+1 555 0103")
}

// An address is labelled from s-contacts-22 (from-typecase note 20): home,
// work, other, and under a line the label another client wrote, as it is —
// not turned into a type, not lost, not recased.
func TestAddressLabelListIsTheFrame(t *testing.T) {
	frame := frameList(t, "s-contacts-22")
	_, foreign := frameForeign(t, "s-contacts-22")
	f := bookFixture(t, nil, "BEGIN:VCARD\nVERSION:3.0\nUID:ann\nFN:Ann\n"+
		"ADR;TYPE="+foreign+":;;Lesnaya 5;Tver;;;\nADR;TYPE=Dacha:;;Lake 1;Tver;;;\nADR;TYPE=WORK:;;Main 2;Tver;;;\nEND:VCARD\n")
	page := decode[view.ContactEditPage](t, f.h.do(get(f.path("contacts")+"/ann/edit", f.sess)))
	held := rowNamed(t, page.Form, "adr.0.type").Options
	sameList(t, "address label", held, "s-contacts-22")
	if sel := selectedOf(held); len(sel) != 1 || sel[0] != foreign {
		t.Errorf("TYPE=%s selects %q", foreign, sel)
	}
	if got := serverList(rowNamed(t, page.Form, "adr.1.type").Options); got[len(got)-1] != "— Dacha" {
		t.Errorf("another client's label = %q, want it as it is under a line", got)
	}
	if sel := selectedOf(rowNamed(t, page.Form, "adr.2.type").Options); len(sel) != 1 || sel[0] != "WORK" {
		t.Errorf("TYPE=WORK selects %q", sel)
	}
	if got := serverList(addRow(t, page.Form, "adr").Template[0].Options); !equalStrings(got, frame[:len(frame)-1]) {
		t.Errorf("new address label = %q, want (s-contacts-22) %q", got, frame[:len(frame)-1])
	}
}

// A phone and an e-mail are labelled as an address is (from-typecase note
// 22): under a line, the label another client wrote, as it is — «iPhone» of
// s-contacts-12, «Личная» of s-contacts-13 —, chosen, not recased, and kept
// by a save that leaves it alone. A new row is not offered it.
func TestForeignPhoneAndEmailLabelsAreTheFrames(t *testing.T) {
	_, phone := frameForeign(t, "s-contacts-12")
	_, mail := frameForeign(t, "s-contacts-13")
	f := bookFixture(t, nil, "BEGIN:VCARD\nVERSION:3.0\nUID:ann\nFN:Ann\n"+
		"TEL;TYPE="+phone+":+1 555 0100\nEMAIL;TYPE="+mail+",INTERNET:ann@example.org\nEND:VCARD\n")
	page := decode[view.ContactEditPage](t, f.h.do(get(f.path("contacts")+"/ann/edit", f.sess)))
	for _, c := range []struct{ row, frame, label string }{{"tel.0.type", "s-contacts-12", phone}, {"email.0.type", "s-contacts-13", mail}} {
		row := rowNamed(t, page.Form, c.row)
		sameList(t, c.row, row.Options, c.frame)
		if sel := selectedOf(row.Options); len(sel) != 1 || sel[0] != c.label {
			t.Errorf("%s: TYPE=%s selects %q", c.row, c.label, sel)
		}
	}
	sameOwnList(t, "new phone label", addRow(t, page.Form, "tel").Template[0].Options, "s-contacts-12")
	sameOwnList(t, "new email label", addRow(t, page.Form, "email").Template[0].Options, "s-contacts-13")

	form := formFields(page.Form)
	form.Set("fn", "Ann B")
	if resp := f.h.do(postForm(f.path("contacts")+"/ann/edit", f.sess, form)); resp.Code != 303 {
		t.Fatalf("save: %d %s", resp.Code, resp.Body.String())
	}
	mustContain(t, typeLists(body(t, f, "ann.vcf")), "TEL;TYPE="+phone+":+1 555 0100", "EMAIL;TYPE="+mail+",INTERNET:ann@example.org")
}

// A TYPE value that is not a label — INTERNET, the vCard 3 default of an
// e-mail, VOICE of a phone, PREF, X400, the postal kinds of an address — is
// no label: it is not offered as another client's, the label beside it is
// chosen (WORK,PREF is work), and it stays in the card whatever the save,
// a label changed beside it included (S-7).
func TestTypeValuesThatAreNoLabel(t *testing.T) {
	f := bookFixture(t, nil, "BEGIN:VCARD\nVERSION:3.0\nUID:ann\nFN:Ann\n"+
		"TEL;TYPE=VOICE:+1 555 0100\nTEL;TYPE=WORK,PREF:+1 555 0101\nTEL;TYPE=CELL,VOICE:+1 555 0102\n"+
		"EMAIL;TYPE=INTERNET:ann@example.org\nEMAIL;TYPE=INTERNET,X400:ann@example.net\n"+
		"ADR;TYPE=dom,postal:;;Main 2;Tver;;;\nEND:VCARD\n")
	page := decode[view.ContactEditPage](t, f.h.do(get(f.path("contacts")+"/ann/edit", f.sess)))
	for _, c := range []struct {
		row, frame string
		chosen     []string
	}{
		{"tel.0.type", "s-contacts-12", nil},
		{"tel.1.type", "s-contacts-12", []string{"work"}},
		{"tel.2.type", "s-contacts-12", []string{"mobile"}},
		{"email.0.type", "s-contacts-13", nil},
		{"email.1.type", "s-contacts-13", nil},
		{"adr.0.type", "s-contacts-22", nil},
	} {
		row := rowNamed(t, page.Form, c.row)
		want := frameList(t, c.frame)
		if strings.HasPrefix(want[len(want)-1], "— ") {
			want = want[:len(want)-1] // s-contacts-22 draws a foreign label too
		}
		if got := serverList(row.Options); !equalStrings(got, want) {
			t.Errorf("%s = %q, want (%s) %q with no foreign label", c.row, got, c.frame, want)
		}
		var chosen []string
		for _, o := range row.Options {
			if o.Selected {
				chosen = append(chosen, string(o.Label))
			}
		}
		if !equalStrings(chosen, c.chosen) && !(len(chosen) == 0 && len(c.chosen) == 0) {
			t.Errorf("%s chooses %q, want %q", c.row, chosen, c.chosen)
		}
	}

	// Saved as a browser posts it — each list its chosen value, or nothing —
	// the card keeps every TYPE.
	form := formFields(page.Form)
	for _, set := range page.Form.Sets {
		for _, r := range set.Rows {
			if strings.HasSuffix(r.Name, ".type") {
				form.Set(r.Name, first(selectedOf(r.Options)))
			}
		}
	}
	form.Set("fn", "Ann B")
	before := body(t, f, "ann.vcf")
	if resp := f.h.do(postForm(f.path("contacts")+"/ann/edit", f.sess, form)); resp.Code != 303 {
		t.Fatalf("save: %d %s", resp.Code, resp.Body.String())
	}
	// TYPE=a,b and TYPE=a;TYPE=b are one parameter (RFC 6350 §5.6); the
	// card's writer uses the second.
	after := typeLists(body(t, f, "ann.vcf"))
	for _, line := range []string{"TEL;TYPE=VOICE:+1 555 0100", "TEL;TYPE=WORK,PREF:+1 555 0101", "TEL;TYPE=CELL,VOICE:+1 555 0102",
		"EMAIL;TYPE=INTERNET:ann@example.org", "EMAIL;TYPE=INTERNET,X400:ann@example.net", "ADR;TYPE=dom,postal:;;Main 2;Tver;;;"} {
		if strings.Contains(before, line) && !strings.Contains(after, line) {
			t.Errorf("%q changed:\n%s", line, after)
		}
	}

	// A label chosen beside them replaces the label, not what is no label.
	page = decode[view.ContactEditPage](t, f.h.do(get(f.path("contacts")+"/ann/edit", f.sess)))
	form = formFields(page.Form)
	form.Set("tel.1.type", optionValue(t, rowNamed(t, page.Form, "tel.1.type").Options, "home"))
	form.Set("email.0.type", optionValue(t, rowNamed(t, page.Form, "email.0.type").Options, "work"))
	if resp := f.h.do(postForm(f.path("contacts")+"/ann/edit", f.sess, form)); resp.Code != 303 {
		t.Fatalf("save: %d %s", resp.Code, resp.Body.String())
	}
	mustContain(t, typeLists(body(t, f, "ann.vcf")), "TEL;TYPE=home,PREF:+1 555 0101", "EMAIL;TYPE=work,INTERNET:ann@example.org")
}

var repeatedType = regexp.MustCompile(`(;TYPE=[^;:]*);TYPE=`)

// typeLists writes a TYPE given several times as one list.
func typeLists(card string) string {
	for repeatedType.MatchString(card) {
		card = repeatedType.ReplaceAllString(card, "$1,")
	}
	return card
}

func first(v []string) string {
	if len(v) == 0 {
		return ""
	}
	return v[0]
}

func optionValue(t *testing.T, opts []view.Option, label string) string {
	t.Helper()
	for _, o := range opts {
		if string(o.Label) == label {
			return o.Value
		}
	}
	t.Fatalf("no option %q in %+v", label, opts)
	return ""
}

// The mail encryption of s-admin-9, and the link term of s-publish-14.
func TestSettingsListsAreTheFrames(t *testing.T) {
	f := bookFixture(t, nil, adaCard())
	page := decode[view.AdminSettingsPage](t, f.h.do(get("/admin/settings", f.sess)))
	// No hint under it: s-admin-3 and s-admin-9 draw none (Ад19).
	enc := rowNamed(t, page.Form, "security")
	sameList(t, "encryption", enc.Options, "s-admin-9")

	q := url.Values{"k": {"contacts"}, "a": {f.id}, "c": {f.col}}
	sheet := decode[view.PublishSheet](t, f.h.do(get("/app/publish?"+q.Encode(), f.sess)))
	sameList(t, "link term", rowNamed(t, sheet.Form, "lives").Options, "s-publish-14")
}

// A backup goes to a file collection of s-backup-9: a read-only one is shown
// and cannot be taken, with the reason.
func TestBackupCollectionListShowsReadOnly(t *testing.T) {
	f := backupFixture(t)
	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		b.Collections = append(b.Collections, sealed.Collection{AccountID: f.id, Path: "/files/public/", Name: "Public", Kinds: []view.Kind{view.KindFile}, ReadOnly: true})
	})
	page := decode[view.BackupPage](t, f.h.do(get("/app/settings/backup", f.sess)))
	var ro *view.Option
	for _, o := range rowNamed(t, page.Add, "dest").Options {
		if strings.HasSuffix(o.Value, "/files/public/") {
			ro = &o
		}
	}
	if ro == nil || !ro.Disabled || ro.Reason == "" {
		t.Errorf("the read-only collection = %+v, want it shown and closed with a reason", ro)
	}
}

// «Written as» is the list of attachments-3, stored per person; the name of a
// large attachment follows it.
func TestAttachmentNamesAreChosen(t *testing.T) {
	f := attachFixture(t, 1024, []string{"carrel"}, "e1.ics", attachEvent("e1", "Sync-up", ""))
	page := decode[view.AttachmentsPage](t, f.h.do(get("/app/settings/attachments", f.sess)))
	name := rowNamed(t, page.Form, "name")
	sameList(t, "written as", name.Options, "attachments-3")
	if name.ReadOnly || name.Kind != view.InputSelect {
		t.Errorf("Written as = %+v, want a choice", name)
	}
	if sel := selectedOf(name.Options); len(sel) != 1 || sel[0] != name.Options[0].Value {
		t.Errorf("the default selects %q, want the first", sel)
	}

	upload := func(file string) {
		t.Helper()
		href := actionHref(t, eventAt(t, f, "e1").Actions, "attach.add")
		if resp := f.h.do(uploadRequest(href, f.sess, false, file, make([]byte, 2048))); resp.Code != 303 {
			t.Fatalf("attach: %d %s", resp.Code, resp.Body.String())
		}
	}
	upload("one.pdf")
	if _, ok := f.srv.File(attachFiles + "carrel/2026-08-10 one.pdf"); !ok {
		t.Errorf("by date: the folder holds %+v", folderRows(t, f, "carrel"))
	}

	for _, c := range []struct{ label, file, want string }{
		{"The original name", "two.pdf", "two.pdf"},
		{"Record title, then the original name", "three.pdf", "Sync-up - three.pdf"},
	} {
		label := c.label
		form := formFields(page.Form)
		form.Set("name", optionValue(t, name.Options, label))
		if resp := f.h.do(postForm("/app/settings/attachments", f.sess, form)); resp.Code != 303 {
			t.Fatalf("save %s: %d %s", label, resp.Code, resp.Body.String())
		}
		again := decode[view.AttachmentsPage](t, f.h.do(get("/app/settings/attachments", f.sess)))
		if sel := selectedOf(rowNamed(t, again.Form, "name").Options); len(sel) != 1 || sel[0] != optionValue(t, name.Options, label) {
			t.Errorf("%s: stored %q", label, sel)
		}
		upload(c.file)
		if _, ok := f.srv.File(attachFiles + "carrel/" + c.want); !ok {
			t.Errorf("%s: the folder holds %+v, want %q", label, folderRows(t, f, "carrel"), c.want)
		}
	}
	form := formFields(page.Form)
	form.Set("name", "whatever")
	if again := decode[view.AttachmentsPage](t, f.h.do(postForm("/app/settings/attachments", f.sess, form))); len(again.Form.Errors) == 0 {
		t.Error("a name the list does not offer was taken")
	}
}

// The journal is filtered by action (s-admin-7, s-admin-10): All actions,
// a line, then the actions the journal shows, as the server writes them.
func TestAuditFilterByAction(t *testing.T) {
	h := newHarness(t)
	h.admin("owner", "password123")
	sess := h.login("owner", "password123")
	h.do(h.anonPostForm("/login", "/login", url.Values{"login": {"owner"}, "password": {"nope"}}))

	page := decode[view.AdminAuditPage](t, h.do(get("/admin/audit", sess)))
	row := rowNamed(t, page.Filter, "action")
	frame := frameList(t, "s-admin-10")
	if len(row.Options) < 2 || string(row.Options[0].Label) != frame[0] || !row.Options[0].Selected || !row.Options[1].Separator ||
		!strings.HasPrefix(frame[1], "— ") {
		t.Fatalf("actions = %+v, want %q first and a line under it", row.Options, frame[0])
	}
	offered := map[string]bool{}
	for _, o := range row.Options[1:] {
		offered[o.Value] = true
		if string(o.Label) != o.Value {
			t.Errorf("option %+v: the label is the code the column shows", o)
		}
	}
	for _, r := range page.Rows {
		if !offered[string(r.What)] {
			t.Errorf("%q is in the journal and not in the list", r.What)
		}
	}
	if page.Filter.Method != "GET" {
		t.Errorf("filter method = %q", page.Filter.Method)
	}

	only := decode[view.AdminAuditPage](t, h.do(get("/admin/audit?action=login.fail", sess)))
	if len(only.Rows) == 0 {
		t.Fatal("login.fail shows nothing")
	}
	for _, r := range only.Rows {
		if r.What != "login.fail" {
			t.Errorf("filtered journal holds %q", r.What)
		}
	}
	if sel := selectedOf(rowNamed(t, only.Filter, "action").Options); len(sel) != 1 || sel[0] != "login.fail" {
		t.Errorf("the chosen action selects %q", sel)
	}
	for _, seg := range only.Bar.Segments {
		if !strings.Contains(seg.Href, "action=login.fail") {
			t.Errorf("segment %s drops the action: %s", seg.ID, seg.Href)
		}
	}
	if a := actionByID(only.Bar.Actions, "audit.export"); a == nil || !strings.Contains(a.Href, "action=login.fail") {
		t.Errorf("the export drops the action: %+v", a)
	}
}

// «Into» of the import ends, under a line, with a new collection of the kind
// (s-io-10…13): the new-collection sheet opens from it, and making one brings
// the wizard back with it chosen.
func TestImportIntoOffersANewCollection(t *testing.T) {
	f := calendarFixture(t, nil)
	for segment, frame := range map[string]string{"calendar": "s-io-12", "tasks": "s-io-13", "notes": "s-io-11"} {
		items := frameList(t, frame)
		last := items[len(items)-1]
		page := decode[view.ImportPage](t, f.h.do(get(f.path(segment)+"/import", f.sess)))
		if !strings.HasPrefix(last, "— ") || page.Into.New == nil || "— "+string(page.Into.New.Label) != last {
			t.Errorf("%s: Into.New = %+v, want (%s) %q under a line", segment, page.Into.New, frame, last)
		}
	}
	book := bookFixture(t, nil, adaCard())
	items := frameList(t, "s-io-10")
	if page := decode[view.ImportPage](t, book.h.do(get(book.path("contacts")+"/import", book.sess))); page.Into.New == nil ||
		"— "+string(page.Into.New.Label) != items[len(items)-1] {
		t.Errorf("contacts: Into.New = %+v, want %q", page.Into.New, items[len(items)-1])
	}

	wizard := f.path("tasks") + "/import"
	page := decode[view.ImportPage](t, f.h.do(get(wizard, f.sess)))
	if page.Into.New == nil {
		t.Fatal("no way to a new list from the wizard")
	}
	sheet := decode[view.CollectionNewSheet](t, f.h.do(get(page.Into.New.Href, f.sess)))
	if sheet.Kind != view.KindTask {
		t.Fatalf("sheet = %+v", sheet)
	}
	rec := f.h.do(postForm(sheet.Form.Action, f.sess, url.Values{"account": {f.id}, "name": {"Errands"}, "components": {"VTODO"}}))
	loc := rec.Header().Get("Location")
	if rec.Code != 303 || !strings.HasPrefix(loc, wizard+"?") {
		t.Fatalf("create: %d %q, want back into the wizard", rec.Code, loc)
	}
	back := decode[view.ImportPage](t, f.h.do(get(loc, f.sess)))
	chosen := ""
	for _, o := range back.Into.Options {
		if o.ID == back.Into.Chosen {
			chosen = string(o.Label)
		}
	}
	if chosen != "Errands" {
		t.Errorf("the wizard came back with %q chosen, want the new list", chosen)
	}

	// Somewhere else than a wizard is not a way back.
	sheet = decode[view.CollectionNewSheet](t, f.h.do(get("/app/collections/new?kind=task&back="+url.QueryEscape("https://elsewhere.example.org/"), f.sess)))
	rec = f.h.do(postForm(sheet.Form.Action, f.sess, url.Values{"account": {f.id}, "name": {"Chores"}, "components": {"VTODO"}}))
	if loc := rec.Header().Get("Location"); strings.Contains(loc, "elsewhere") || strings.HasSuffix(loc, "/import") {
		t.Errorf("an outside back went to %q", loc)
	}
}
