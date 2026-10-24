package web_test

import (
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// From-typecase note 12: move and copy are a sheet — over the selected files
// with the tree of every storage (s-files-4), over a record with the list of
// its kind's collections (s-contacts-2). Every string compared below is cut
// out of the frame.

var (
	// The line beside a sheet's title — «from …», «to …» —, after the pick of
	// the server the head has since Typecase 88c2a63 (from-typecase note 35,
	// Р-37: «sftpgo · vps»; the model names no server yet — the files round).
	frameSheetSub  = regexp.MustCompile(`(?s)<b class="tc-sheet__title">.*?</b>\s*(?:<details class="tc-drop[^"]*">.*?</details>\s*)?<span class="(?:tc-push )?tc-dim[^"]*">(.*?)</span>`)
	frameSheetMsg  = regexp.MustCompile(`(?s)<div class="tc-sheet__body">.*?<div class="tc-msg[^"]*">(.*?)</div>`)
	frameSheetMain = regexp.MustCompile(`(?s)<div class="tc-sheet__foot"><button class="tc-btn is-primary"[^>]*>(.*?)</button>`)
	frameTreeNew   = regexp.MustCompile(`(?s)<div class="tc-rubric">([^<]*?)\s*<button class="tc-btn is-sm"[^>]*>(.*?)</button></div>\s*<div class="tc-tree">`)
	frameClosedRow = regexp.MustCompile(`(?s)<label class="tc-tree__row tc-tree__row--op-55"><input[^>]*>(?:<span class="tc-mark"[^>]*></span>|<svg>.*?</svg>)?([^<]*?)\s*<span class="[^"]*">(.*?)</span></label>`)
)

// frameInline is a piece of a frame as it reads: inline tags go without a
// space of their own («<code>MOVE</code>:» is «MOVE:»).
func frameInline(s string) string {
	return strings.Join(strings.Fields(html.UnescapeString(frameTag.ReplaceAllString(s, ""))), " ")
}

func frameOne(t *testing.T, re *regexp.Regexp, frame, what string) []string {
	t.Helper()
	m := re.FindStringSubmatch(frame)
	if m == nil {
		t.Fatalf("the frame draws no %s: read it again", what)
	}
	for i := range m {
		m[i] = frameInline(m[i])
	}
	return m
}

// s-files-4: «Move 3 items» from a folder, the tree with New folder in its
// rubric, no message inside one storage (Р-77), «Move»; Copy is the same
// sheet with «Copy».
func TestFileMoveSheetIsTheFrame(t *testing.T) {
	frame := mockupFrame(t, "s-files-4")
	title := frameOne(t, frameSheetT, frame, "title")[1]
	from := frameOne(t, frameSheetSub, frame, "from line")[1]
	confirm := frameOne(t, frameSheetMain, frame, "main button")[1]
	newFolder := frameOne(t, frameTreeNew, frame, "button in the tree's rubric")[2]
	if m := frameSheetMsg.FindStringSubmatch(frame); m != nil {
		t.Fatalf("the frame says %q again: read it again", frameInline(m[1]))
	}

	count := regexp.MustCompile(`^Move (\d+) items$`).FindStringSubmatch(title)
	path, ok := strings.CutPrefix(from, "from ")
	if count == nil || !ok {
		t.Fatalf("title %q, from %q: the frame changed, read it again", title, from)
	}
	n, _ := strconv.Atoi(count[1])
	// «Documents / carrel / attachments»: the storage, then the folders.
	segs := strings.Split(path, " / ")[1:]

	f := filesFixture(t, true)
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("f%d.txt", i)
		f.srv.SeedFile(filesDocs+strings.Join(segs, "/")+"/"+names[i], "x")
	}
	f.srv.SeedFolder(filesDocs + "2026-07/")
	op := func(verb string) string {
		return f.at("op", append(append([]string{}, segs...), "")...) + "?" + url.Values{"op": {verb}, "item": names}.Encode()
	}
	there := f.at("", "2026-07", "")

	sheet := opSheet(t, f, op("move"), url.Values{"to": {there}})
	if string(sheet.Title) != title {
		t.Errorf("title = %q, the frame %q", sheet.Title, title)
	}
	if want := "from Docs / " + strings.Join(segs, " / "); string(sheet.Subject) != want {
		t.Errorf("subject = %q, want %q", sheet.Subject, want)
	}
	if len(sheet.Warning) != 0 {
		t.Errorf("message = %q, the frame none", plainText(sheet.Warning))
	}
	if sheet.Actions[0].Label != view.Text(confirm) || !sheet.Actions[0].Enabled {
		t.Errorf("confirm = %+v, the frame %q", sheet.Actions[0], confirm)
	}
	if sheet.Target == nil || sheet.Target.New == nil || sheet.Target.New.Label != view.Text(newFolder) || !sheet.Target.New.Enabled {
		t.Fatalf("the tree has no %q: %+v", newFolder, sheet.Target)
	}

	// New folder makes the folder in the chosen one, and the sheet comes
	// back with it chosen.
	// New opens the line of the name (a GET of the sheet, ФГ4); the line
	// makes it.
	line := decode[view.FileOpSheet](t, f.h.do(get(sheet.Target.New.Href, f.sess)))
	if line.Target == nil || line.Target.Name == nil {
		t.Fatalf("New did not open the line: %+v", line.Target)
	}
	made := opSheet(t, f, line.Target.Name.Action, url.Values{"name": {"2026-08"}})
	if want := f.at("", "2026-07", "2026-08", ""); made.Target == nil || made.Target.Chosen != want || made.Failure != nil {
		t.Fatalf("after New folder: chosen %+v, failure %+v", made.Target, made.Failure)
	}
	if row := fileRow(t, decode[view.FilesPage](t, f.h.do(get(there, f.sess))), "2026-08"); !row.IsDir {
		t.Errorf("no folder made: %+v", row)
	}
	if string(made.Title) != title || !made.Actions[0].Enabled {
		t.Errorf("the sheet did not come back whole: %+v", made)
	}
	taken := opSheet(t, f, line.Target.Name.Action, url.Values{"name": {"2026-08"}})
	if taken.Failure == nil || taken.Target.Chosen != there || taken.Target.Name == nil {
		t.Errorf("a taken name: failure %+v, chosen %q", taken.Failure, taken.Target.Chosen)
	}

	cp := opSheet(t, f, op("copy"), url.Values{"to": {there}})
	if want := "Copy " + strings.TrimPrefix(title, "Move "); string(cp.Title) != want || cp.Actions[0].Label != "Copy" {
		t.Errorf("copy: title %q, confirm %+v", cp.Title, cp.Actions[0])
	}
}

// The upload picker draws the same tree with the same New folder
// (s-notes-11, «Where to put it»).
func TestUploadPickerMakesAFolder(t *testing.T) {
	newFolder := frameOne(t, frameTreeNew, mockupFrame(t, "s-notes-11"), "button in the tree's rubric")[2]
	f := filesFixture(t, true)
	f.srv.SeedFolder(filesDocs + "carrel/")
	carrel := f.at("", "carrel", "")
	sheet := decode[view.FilePickerSheet](t, f.h.do(get("/app/files/picker?"+url.Values{"for": {"upload"}, "to": {carrel}}.Encode(), f.sess)))
	if sheet.Tree.New == nil || sheet.Tree.New.Label != view.Text(newFolder) || !sheet.Tree.New.Enabled {
		t.Fatalf("the picker has no %q: %+v", newFolder, sheet.Tree)
	}
	line := decode[view.FilePickerSheet](t, f.h.do(get(sheet.Tree.New.Href, f.sess)))
	if line.Tree.Name == nil {
		t.Fatalf("New did not open the line: %+v", line.Tree)
	}
	made := decode[view.FilePickerSheet](t, f.h.do(postForm(line.Tree.Name.Action, f.sess, url.Values{"name": {"2026-08"}})))
	if want := f.at("", "carrel", "2026-08", ""); made.Tree.Chosen != want {
		t.Fatalf("after New folder the chosen is %q, want %q", made.Tree.Chosen, want)
	}
	if !fileRow(t, decode[view.FilesPage](t, f.h.do(get(carrel, f.sess))), "2026-08").IsDir {
		t.Error("no folder made")
	}
}

// s-contacts-2: «Move contact», what and from where, the books in the
// rail's order — the current one and those holding Anna's UID already
// «already there», another account's book chosen — no message (Р-77: a move
// says nothing its name does not), «Move».
func TestRecordMoveSheetIsTheFrame(t *testing.T) {
	frame := mockupFrame(t, "s-contacts-2")
	title := frameOne(t, frameSheetT, frame, "title")[1]
	subject := frameOne(t, frameSheetSub, frame, "subject")[1]
	confirm := frameOne(t, frameSheetMain, frame, "main button")[1]
	if m := frameSheetMsg.FindStringSubmatch(frame); m != nil {
		t.Fatalf("the frame says %q again: read it again", frameInline(m[1]))
	}
	sheet := mockupMoveSheet(t, newMockupApp(t), "kontakty", "4f2b-9c31-a0e7", false)
	if string(sheet.Title) != title || string(sheet.Subject) != subject {
		t.Errorf("head = %q, %q; the frame %q, %q", sheet.Title, sheet.Subject, title, subject)
	}
	sameMoveRows(t, "s-contacts-2", frame, sheet)
	if len(sheet.Warning) != 0 || sheet.Cancels != "" {
		t.Errorf("the sheet says %q, %q; the frame nothing", plainText(sheet.Warning), sheet.Cancels)
	}
	if c := confirmOf(t, sheet); c.Label != view.Text(confirm) || !c.Enabled {
		t.Errorf("confirm = %+v, the frame %q", c, confirm)
	}
}

// frameMoveRow is a row of a move sheet's list: chosen, closed, its name and
// what stands at its right.
var frameMoveRow = regexp.MustCompile(`(?s)<label class="(is-on )?tc-tree__row( tc-tree__row--op-55)?"><input[^>]*>(?:<span class="tc-mark"[^>]*></span>)?([^<]*?)\s*<span class="[^"]*">(.*?)</span></label>`)

// sameMoveRows holds the sheet's collections to the frame's rows, in order:
// the name, the reason of a closed one or the stamp of an open one, which is
// chosen.
func sameMoveRows(t *testing.T, id, frame string, sheet view.MoveSheet) {
	t.Helper()
	rows := frameMoveRow.FindAllStringSubmatch(frame, -1)
	if len(rows) != len(sheet.Target.Options) {
		t.Fatalf("%s: %d rows, the sheet %d: %+v", id, len(rows), len(sheet.Target.Options), sheet.Target.Options)
	}
	for i, m := range rows {
		o := sheet.Target.Options[i]
		right := string(o.Stamp)
		if o.ReadOnly {
			right = string(o.Reason)
		}
		chosen := sheet.Target.Chosen != "" && sheet.Target.Chosen == o.ID
		if string(o.Label) != frameInline(m[3]) || right != frameInline(m[4]) || o.ReadOnly != (m[2] != "") || chosen != (m[1] != "") {
			t.Errorf("%s row %d = %q %q closed %v chosen %v; the frame %q %q closed %v chosen %v", id, i, o.Label, right, o.ReadOnly, chosen,
				frameInline(m[3]), frameInline(m[4]), m[2] != "", m[1] != "")
		}
	}
}

// mockupMoveSheet is the move (or copy) sheet of a record of the data set,
// opened from its card's menu.
func mockupMoveSheet(t *testing.T, app *mockupApp, book, uid string, copying bool) view.MoveSheet {
	t.Helper()
	page := decode[view.ContactPage](t, app.h.do(get(app.listPath(t, "contacts", book)+"/"+uid, app.sess)))
	id := "record.move"
	if copying {
		id = "record.copy"
	}
	for _, m := range page.Menu {
		if m.ID == id {
			return decode[view.MoveSheet](t, app.h.do(get(m.Href, app.sess)))
		}
	}
	t.Fatalf("the card's menu has no %s: %+v", id, page.Menu)
	return view.MoveSheet{}
}

// s-profile-8: moving a collection is the same kind of sheet — the server it
// goes to, the current one «already there», the one chosen saying what it
// takes, the message — the old address is lost (Р-77) —, «Move». The row
// menu of s-profile-1 has both Move and Copy to another server.
func TestCollectionMoveSheetIsTheFrame(t *testing.T) {
	frame := mockupFrame(t, "s-profile-8")
	subject := frameOne(t, frameSheetSub, frame, "subject")[1]
	msg := frameOne(t, frameSheetMsg, frame, "message")[1]
	confirm := frameOne(t, frameSheetMain, frame, "main button")[1]
	here := frameOne(t, frameClosedRow, frame, "the current server")[2]
	holds := frameOne(t, regexp.MustCompile(`(?s)<label class="is-on tc-tree__row"><input[^>]*>(?:<svg>.*?</svg>)?[^<]*<span class="[^"]*">(.*?)</span>`), frame, "the chosen server")[1]
	host, _, ok := strings.Cut(subject, " · ")
	// The frame's server schedules and its calendar has meetings: its last
	// sentence says the attendees may hear of it (note 112 p. 5). This one
	// does not schedule; the sentence is TestAMoveSaysTheAttendeesMayHearOfACancellation's.
	msg, cancels, _ := strings.Cut(msg, " "+host+" may send")
	if !ok || cancels == "" {
		t.Fatalf("subject %q, message %q: the frame changed, read it again", subject, msg)
	}
	menu := mockupFrame(t, "s-profile-1")
	var items []string
	for _, l := range []string{"Move to another server", "Copy to another server"} {
		if regexp.MustCompile(`role="menuitem">(?:<svg>.*?</svg>)?` + l + `</button>`).MatchString(menu) {
			items = append(items, l)
		}
	}
	if len(items) != 2 {
		t.Fatalf("s-profile-1 menu = %v: read it again", items)
	}

	f := newColMoveFixture(t)
	f.srv.Seed(f.col+"retro.ics", ics("BEGIN:VEVENT\nUID:retro\nDTSTAMP:20260101T000000Z\nDTSTART:20261007T100000Z\nSUMMARY:Retro\nEND:VEVENT"))
	f.h.do(get(f.path("calendar"), f.sess)) // the visit that learns the count
	page := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
	row := connectionRow(t, page, f.id, f.col)
	var got []string
	for _, m := range row.Menu {
		if strings.HasSuffix(string(m.Label), "another server") {
			got = append(got, string(m.Label))
		}
	}
	if !equalStrings(got, items) {
		t.Errorf("menu = %v, the frame %v", got, items)
	}
	sheet := decode[view.CollectionMoveSheet](t, f.h.do(get(menuHref(t, row.Menu, "collection.move"), f.sess)))
	src := sheet.Target.Options[0]
	if want := string(src.Label) + " · Work"; string(sheet.Subject) != want {
		t.Errorf("subject = %q, want %q (the frame %q)", sheet.Subject, want, subject)
	}
	if !src.ReadOnly || string(src.Reason) != here {
		t.Errorf("the current server = %+v, the frame %q", src, here)
	}
	chosen := decode[view.CollectionMoveSheet](t, f.h.do(get(sheet.Target.Options[1].ID, f.sess)))
	if o := chosen.Target.Options[1]; o.ReadOnly || string(o.Stamp) != holds {
		t.Errorf("the chosen server = %+v, the frame %q", o, holds)
	}
	if plainText(chosen.Warning) != msg {
		t.Errorf("message = %q\nthe frame %q", chosen.Warning, msg)
	}
	if len(chosen.Actions) != 2 || chosen.Actions[1].ID != "collection.cancel" || chosen.Actions[0].Label != view.Text(confirm) || !chosen.Actions[0].Enabled {
		t.Errorf("actions = %+v, the frame %q and Cancel", chosen.Actions, confirm)
	}
	cp := decode[view.CollectionMoveSheet](t, f.h.do(get(menuHref(t, row.Menu, "collection.copy"), f.sess)))
	if !cp.Copy || len(cp.Actions) != 2 || cp.Actions[0].Label != "Copy" {
		t.Errorf("copy = %+v", cp)
	}
}
