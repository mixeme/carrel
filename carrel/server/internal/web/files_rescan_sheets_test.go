package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// The client's second scan of the files section, the sheets (client.md,
// «Повторный скан по кадрам Typecase 0fe0244», ФН11, ФН12): the words of a
// rename (s-files-21) and of a deletion (s-files-22).

var (
	frameOpPath       = regexp.MustCompile(`(?s)<span class="tc-push tc-dim tc-dim--t11-5 tc-dim--wrap">(.*?)</span>`)
	frameOpFieldLabel = regexp.MustCompile(`(?s)<div class="tc-field tc-field--w100"><label>(.*?)</label>`)
	frameOpFlushMsg   = regexp.MustCompile(`(?s)<div class="tc-msg tc-msg--flush">(.*?)</div>`)
	frameOpFlushAlert = regexp.MustCompile(`(?s)<div class="tc-msg is-alert tc-msg--flush">(.*?)</div>`)
)

// opSheetOf asks for the sheet of an operation over names of the folder at
// segs of Docs.
func opSheetOf(t *testing.T, f davFixture, op string, names []string, segs ...string) view.FileOpSheet {
	t.Helper()
	at := f.at("op", append(segs, "")...) + "?" + url.Values{"op": {op}, "item": names}.Encode()
	return decode[view.FileOpSheet](t, f.h.do(postForm(at, f.sess, url.Values{})))
}

// ФН11: a rename says where the item is — the path alone, no «from» —,
// calls its field «Name», and says whom the new address will lose
// (s-files-21).
func TestRenameSheetIsTheFrame(t *testing.T) {
	frame := mockupFrame(t, "s-files-21")
	path := frameText(frameOne(t, frameOpPath, frame, "path")[1])
	label := frameText(frameOne(t, frameOpFieldLabel, frame, "field")[1])
	msg := frameText(frameOne(t, frameOpFlushMsg, frame, "message")[1])
	segs := strings.Split(path, " / ")
	if len(segs) < 2 || strings.HasPrefix(path, "from ") {
		t.Fatalf("s-files-21 says %q under the title: read the frame again", path)
	}

	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+strings.Join(segs[1:], "/")+"/a.txt", "alpha")
	sheet := opSheetOf(t, f, "rename", []string{"a.txt"}, segs[1:]...)
	if want := "Docs / " + strings.Join(segs[1:], " / "); string(sheet.Subject) != want {
		t.Errorf("subject = %q, want %q", sheet.Subject, want)
	}
	if sheet.Form == nil || formRow(*sheet.Form, "name") == nil || string(formRow(*sheet.Form, "name").Label) != label {
		t.Errorf("form = %+v, want the field %q", sheet.Form, label)
	}
	if plainText(sheet.Warning) != msg || len(sheet.Alert) != 0 {
		t.Errorf("warning = %q, alert %q; want %q and none", plainText(sheet.Warning), plainText(sheet.Alert), msg)
	}
}

// ФН12: a deletion says first what goes, how much and from which server,
// and that Carrel holds no copy — the count strong —, then that nothing
// counted the links to these files (s-files-22); the path stands without
// «from». A folder goes with everything in it.
func TestDeleteSheetIsTheFrame(t *testing.T) {
	frame := mockupFrame(t, "s-files-22")
	path := frameText(frameOne(t, frameOpPath, frame, "path")[1])
	raw := frameOne(t, frameOpFlushAlert, frame, "alert")[1]
	alert := regexp.MustCompile(`^(\d+) files, (.+?), are deleted on (.+?)\. (Carrel holds no copy.*)$`).FindStringSubmatch(strings.Join(strings.Fields(raw), " "))
	msg := frameText(frameOne(t, frameOpFlushMsg, frame, "message")[1])
	segs := strings.Split(path, " / ")
	if alert == nil || alert[1] != "3" || len(segs) < 2 {
		t.Fatalf("s-files-22 says %q under %q: read the frame again", raw, path)
	}

	f := filesFixture(t, true)
	dir := filesDocs + strings.Join(segs[1:], "/") + "/"
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		f.srv.SeedFile(dir+name, strings.Repeat(".", 1500))
	}
	f.srv.SeedFolder(dir + "sub/")
	f.srv.SeedFile(dir+"sub/inside.txt", "x")
	server := string(decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess))).Accounts[0].Title)
	sheet := opSheetOf(t, f, "delete", []string{"a.txt", "b.txt", "c.txt"}, segs[1:]...)
	if want := "Docs / " + strings.Join(segs[1:], " / "); string(sheet.Subject) != want {
		t.Errorf("subject = %q, want %q", sheet.Subject, want)
	}
	want := "3 files, 4.4 KB, are deleted on " + server + ". " + frameText(alert[4])
	if plainText(sheet.Alert) != want || len(sheet.Alert) == 0 || sheet.Alert[0].Mark != view.RunStrong || sheet.Alert[0].Text != "3" {
		t.Errorf("alert = %+v\nwant %q, the count strong", sheet.Alert, want)
	}
	if plainText(sheet.Warning) != msg {
		t.Errorf("warning = %q, want %q", plainText(sheet.Warning), msg)
	}
	if a := actionByID(sheet.Actions, "file.delete.confirm"); a == nil || !a.Danger || a.Primary {
		t.Errorf("Delete = %+v", a)
	}

	one := opSheetOf(t, f, "delete", []string{"a.txt"}, segs[1:]...)
	if got := plainText(one.Alert); !strings.HasPrefix(got, "1 file, 1.5 KB, is deleted on "+server+". ") {
		t.Errorf("one file: %q", got)
	}
	folder := opSheetOf(t, f, "delete", []string{"sub"}, segs[1:]...)
	if got := plainText(folder.Alert); !strings.HasPrefix(got, "The folder and everything in it are deleted on "+server+". ") {
		t.Errorf("a folder: %q", got)
	}
	mixed := opSheetOf(t, f, "delete", []string{"sub", "a.txt"}, segs[1:]...)
	if got := plainText(mixed.Alert); !strings.HasPrefix(got, "2 items, the folder among them with everything in it, are deleted on "+server+". ") {
		t.Errorf("a folder and a file: %q", got)
	}
}

// ФН10 (the owner's decision of 09.10.2026: the frame is right): the sheet
// of a move and of a copy is a form (s-files-4) — a node of the tree is a
// tick of its field «to», the sheet's button sends it. What cannot be done
// is switched off (the owner's rule of the same day): while the folder
// chosen cannot be the destination the button stands off and says why. The
// button is «Move», «Copy» — the owner's word; the frames still say «here»
// (Typecase's task 147).
func TestMoveSheetIsAForm(t *testing.T) {
	frame := mockupFrame(t, "s-files-4")
	form := frameOne(t, regexp.MustCompile(`<form class="tc-sheet[^"]*" action="[^"]*\?op=move" method="(post)">`), frame, "form of the sheet")
	field := frameOne(t, regexp.MustCompile(`<input class="tc-tree__box" type="radio" name="([^"]+)" value="[^"]+" checked>`), frame, "chosen node")[1]
	word := frameOne(t, regexp.MustCompile(`<button class="tc-btn is-primary" type="submit">([^<]+)</button>`), frame, "button of the form")[1]
	if form[1] != "post" || (word != "Move here" && word != "Move") {
		t.Fatalf("s-files-4 sends its sheet with %q: read the frame again", word)
	}

	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"a.txt", "alpha")
	f.srv.SeedFolder(filesDocs + "sub/")
	sub := f.at("", "sub", "")
	for op, label := range map[string]string{"move": "Move", "copy": "Copy"} {
		at := f.at("op") + "?" + url.Values{"op": {op}, "item": {"a.txt"}, "to": {sub}}.Encode()
		sheet := decode[view.FileOpSheet](t, f.h.do(postForm(at, f.sess, url.Values{})))
		confirm := sheet.Actions[0]
		if sheet.Form == nil || sheet.Form.Method != "POST" || formRow(*sheet.Form, "_csrf") == nil || sheet.Form.Action != confirm.Href ||
			!strings.Contains(sheet.Form.Action, "go=1") {
			t.Fatalf("%s: form = %+v, confirm %+v", op, sheet.Form, confirm)
		}
		if sheet.Target == nil || sheet.Target.Field != field || sheet.Target.Chosen != sub {
			t.Fatalf("%s: the tree sends %+v, want the field %q and sub chosen", op, sheet.Target, field)
		}
		if !confirm.Enabled || !confirm.Submits || confirm.Reason != "" || string(confirm.Label) != label {
			t.Errorf("%s: confirm = %+v, want the form's button %q, on", op, confirm, label)
		}
	}

	// The sheet opens with the folder of the items chosen — New folder
	// makes one in it (s-files-13) —, and a move there cannot be done: the
	// button is off and says why, the form and the nodes' addresses stand.
	sheet := opSheetOf(t, f, "move", []string{"a.txt"})
	if c := sheet.Actions[0]; c.Enabled || c.Href != "" || c.Reason != "These items are already here." || !c.Submits {
		t.Errorf("a move to its own folder: confirm = %+v, want it off with why", c)
	}
	if sheet.Form == nil || sheet.Form.Action == "" || sheet.Target.Chosen != f.at("") {
		t.Fatalf("a move to its own folder: form %+v, chosen %q", sheet.Form, sheet.Target.Chosen)
	}
	// Another node ticked, the form sends its ID and the file goes there.
	fields := formFields(*sheet.Form)
	fields.Set(field, sub)
	if rec := f.h.do(postForm(sheet.Form.Action, f.sess, fields)); rec.Code >= 400 {
		t.Fatalf("move: %d %s", rec.Code, rec.Body)
	}
	if got, ok := f.srv.File(filesDocs + "sub/a.txt"); !ok || got != "alpha" {
		t.Errorf("after the move sub/a.txt = %q %v", got, ok)
	}
}

// ФН13: New folder of a folder's head and of the rail's foot opens a sheet
// (s-files-23) — «New folder», where it will be made, the field «Name»,
// Create and Cancel —, not a line on the page. A name already taken is
// refused under the field in the words of the move sheet, and the sheet
// stays with what was typed.
func TestNewFolderIsASheet(t *testing.T) {
	frame := mockupFrame(t, "s-files-23")
	title := frameText(frameOne(t, regexp.MustCompile(`(?s)<b class="tc-sheet__title">(.*?)</b>`), frame, "title")[1])
	where := frameText(frameOne(t, frameOpPath, frame, "path")[1])
	label := frameText(frameOne(t, frameOpFieldLabel, frame, "field")[1])
	create := frameText(frameOne(t, regexp.MustCompile(`<button class="tc-btn is-primary" type="button">([^<]+)</button>`), frame, "button")[1])
	refusal := frameText(frameOne(t, frameOpFlushAlert, frame, "refusal")[1])
	taken := regexp.MustCompile(`«([^»]+)» in (\S+) already`).FindStringSubmatch(refusal)
	segs := strings.Split(strings.TrimPrefix(where, "in "), " / ")
	if taken == nil || !strings.HasPrefix(where, "in ") || len(segs) < 2 || segs[len(segs)-1] != taken[2] {
		t.Fatalf("s-files-23 makes %q with the refusal %q: read the frame again", where, refusal)
	}

	f := filesFixture(t, true)
	dir := filesDocs + strings.Join(segs[1:], "/") + "/"
	f.srv.SeedFolder(dir + taken[1] + "/")
	here := f.at("", append(segs[1:], "")...)
	page := decode[view.FilesPage](t, f.h.do(get(here, f.sess)))
	bar := actionHref(t, page.Bar.Actions, "folder.new")
	rail := ""
	for _, a := range page.Rail.Foot {
		if a.ID == "rail.folder.new" {
			rail = a.Href
		}
	}
	if rail != bar || actionByID(page.Bar.Actions, "folder.new").Target != "sheet" {
		t.Fatalf("New folder of the rail %q, of the bar %q (%+v): want one address, answered by a sheet", rail, bar, actionByID(page.Bar.Actions, "folder.new"))
	}
	sheet := decode[view.FileOpSheet](t, f.h.do(postForm(bar, f.sess, url.Values{})))
	if sheet.Op != view.FileNewFolder || string(sheet.Title) != title || sheet.Failure != nil || sheet.Target != nil ||
		string(sheet.Subject) != "in Docs / "+strings.Join(segs[1:], " / ") {
		t.Fatalf("sheet = %+v, want %q %q", sheet, title, where)
	}
	name := formRow(*sheet.Form, "name")
	if name == nil || string(name.Label) != label || !name.Focus || !name.Required || name.Value != "" || sheet.Form.Action != bar || sheet.Form.Method != "POST" {
		t.Fatalf("form = %+v", sheet.Form)
	}
	if c := sheet.Actions[0]; c.ID != "folder.new.create" || string(c.Label) != create || !c.Primary || !c.Submits || !c.Enabled ||
		actionHref(t, sheet.Actions, "folder.new.cancel") != here {
		t.Errorf("actions = %+v", sheet.Actions)
	}
	// A name taken: the sheet again, the refusal of the frame under the
	// field, the name as typed.
	fields := formFields(*sheet.Form)
	fields.Set("name", taken[1])
	again := decode[view.FileOpSheet](t, f.h.do(postForm(sheet.Form.Action, f.sess, fields)))
	if again.Failure == nil || again.Failure.Role != view.NoticeAlert || string(again.Failure.Body) != refusal ||
		string(formRow(*again.Form, "name").Value) != taken[1] {
		t.Errorf("a taken name: failure %+v, form %+v\nthe frame %q", again.Failure, again.Form, refusal)
	}
	// A free one is made, and the answer is the folder.
	fields.Set("name", "inbox")
	if rec := f.h.do(postForm(sheet.Form.Action, f.sess, fields)); rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != here {
		t.Fatalf("create: %d to %q", rec.Code, rec.Header().Get("Location"))
	}
	if !fileRow(t, decode[view.FilesPage](t, f.h.do(get(here, f.sess))), "inbox").IsDir {
		t.Error("no folder made")
	}
}
