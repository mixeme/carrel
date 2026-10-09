package web_test

import (
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
// tick of its field «to», «Move here» sends it. So the button does not
// depend on the node chosen when the sheet was drawn: a destination that
// cannot be one is refused when it is sent, in the words the button used to
// stand off with.
func TestMoveSheetIsAForm(t *testing.T) {
	frame := mockupFrame(t, "s-files-4")
	form := frameOne(t, regexp.MustCompile(`<form class="tc-sheet[^"]*" action="[^"]*\?op=move" method="(post)">`), frame, "form of the sheet")
	field := frameOne(t, regexp.MustCompile(`<input class="tc-tree__box" type="radio" name="([^"]+)" value="[^"]+" checked>`), frame, "chosen node")[1]
	if !regexp.MustCompile(`<button class="tc-btn is-primary" type="submit">Move here</button>`).MatchString(frame) || form[1] != "post" {
		t.Fatal("s-files-4 does not send its sheet with «Move here»: read the frame again")
	}

	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"a.txt", "alpha")
	f.srv.SeedFolder(filesDocs + "sub/")
	for _, op := range []string{"move", "copy"} {
		sheet := opSheetOf(t, f, op, []string{"a.txt"})
		confirm := sheet.Actions[0]
		if sheet.Form == nil || sheet.Form.Method != "POST" || formRow(*sheet.Form, "_csrf") == nil || sheet.Form.Action != confirm.Href ||
			!strings.Contains(sheet.Form.Action, "go=1") {
			t.Fatalf("%s: form = %+v, confirm %+v", op, sheet.Form, confirm)
		}
		if sheet.Target == nil || sheet.Target.Field != field {
			t.Fatalf("%s: the tree sends %+v, want the field %q", op, sheet.Target, field)
		}
		if !confirm.Enabled || !confirm.Submits || confirm.Reason != "" {
			t.Errorf("%s: confirm = %+v, want the form's button, on", op, confirm)
		}
	}

	// The sheet opens with the folder of the items chosen; sent as it is, a
	// move is refused and nothing moves.
	sheet := opSheetOf(t, f, "move", []string{"a.txt"})
	fields := formFields(*sheet.Form)
	fields.Set(field, sheet.Target.Chosen)
	page := decode[view.FilesPage](t, f.h.do(postForm(sheet.Form.Action, f.sess, fields)))
	if !hasNoticeBody(page.Notices, "These items are already here.") {
		t.Errorf("a move to its own folder: %+v", page.Notices)
	}
	// Another node ticked, the form sends its ID and the file goes there.
	fields.Set(field, f.at("", "sub", ""))
	if rec := f.h.do(postForm(sheet.Form.Action, f.sess, fields)); rec.Code >= 400 {
		t.Fatalf("move: %d %s", rec.Code, rec.Body)
	}
	if got, ok := f.srv.File(filesDocs + "sub/a.txt"); !ok || got != "alpha" {
		t.Errorf("after the move sub/a.txt = %q %v", got, ok)
	}
}
