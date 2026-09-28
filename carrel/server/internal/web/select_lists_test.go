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

var frameListItem = regexp.MustCompile(`(?s)<span class="tc-menu__sep"></span>|<button class="[^"]*tc-menu__item[^"]*"[^>]*role="menuitem"[^>]*>(.*?)</button>`)

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
		item := frameText(m[1])
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

	// A reminder no step of the list names stays a choice.
	odd := decode[view.EventEditPage](t, f.h.do(get(f.path("calendar")+"/e2/edit", f.sess)))
	if sel := selectedOf(rowNamed(t, odd.Form, "reminder.0").Options); len(sel) != 1 || sel[0] != "-PT7M" {
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
	sameList(t, "phone label", phone.Options, "s-contacts-12")
	if sel := selectedOf(phone.Options); len(sel) != 1 || sel[0] != "CELL" {
		t.Errorf("TYPE=CELL selects %q, want mobile keeping the value", sel)
	}
	if sel := selectedOf(rowNamed(t, page.Form, "tel.1.type").Options); len(sel) != 0 {
		t.Errorf("a row with no label selects %q", sel)
	}
	if sel := selectedOf(rowNamed(t, page.Form, "tel.2.type").Options); len(sel) != 1 || sel[0] != "WORK" {
		t.Errorf("TYPE=WORK selects %q", sel)
	}
	sameList(t, "new phone label", addRow(t, page.Form, "tel").Template[0].Options, "s-contacts-12")
	sameList(t, "email label", addRow(t, page.Form, "email").Template[0].Options, "s-contacts-13")
	held := rowNamed(t, page.Form, "email.0.type").Options
	if last := held[len(held)-1]; last.Value != "INTERNET" || !last.Selected {
		t.Errorf("email labels = %+v, want another client's INTERNET kept as a choice", held)
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
	enc := rowNamed(t, page.SMTP, "security")
	sameList(t, "encryption", enc.Options, "s-admin-9")
	if !strings.Contains(string(enc.Hint), "relay on this host or network") {
		t.Errorf("encryption hint = %q, want what None is for", enc.Hint)
	}

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
