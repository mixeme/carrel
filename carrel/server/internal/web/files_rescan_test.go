package web_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// The client's second scan of the files section (client.md, «Повторный скан
// по кадрам Typecase 6833ef4», ФН1…ФН4): what the frames of a folder
// (s-files-1, states-32, -33, -34) draw that the view model did not carry.

// ФН1: the line under the title counts the folder — «sftpgo · vps ·
// ‹attachment folder› · 5 items» (s-files-1), «… · 2 items · read-only:
// nothing can be uploaded or made here» (states-34) — and says none when the
// storage did not answer (states-33).
func TestFolderHeadCountsTheFolder(t *testing.T) {
	line := func(id string) string {
		return frameText(frameOne(t, frameHeadSub, mockupFrame(t, id), "line under the title")[1])
	}
	count := regexp.MustCompile(` · (\d+ items?)( · (read-only.*))?$`)
	plain, ro := count.FindStringSubmatch(line("s-files-1")), count.FindStringSubmatch(line("states-34"))
	if plain == nil || ro == nil || ro[3] == "" || count.MatchString(line("states-33")) {
		t.Fatalf("lines %q, %q, %q: the frames changed, read them again", line("s-files-1"), line("states-34"), line("states-33"))
	}

	f := filesFixture(t, true)
	for _, name := range []string{"a.txt", "b.txt", "c.txt", "d.txt"} {
		f.srv.SeedFile(filesDocs+name, name)
	}
	f.srv.SeedFolder(filesDocs + "sub/")
	title := string(decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess))).Accounts[0].Title)
	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	if string(page.Subtitle) != title || string(page.Count) != plain[1] || page.Access != "" {
		t.Errorf("line = %q, %q, %q; want %q, %q and no access", page.Subtitle, page.Count, page.Access, title, plain[1])
	}
	// A filter does not change what the folder holds.
	if page = decode[view.FilesPage](t, f.h.do(get(f.at("")+"?filter=a", f.sess))); string(page.Count) != plain[1] {
		t.Errorf("filtered, the folder counts %q, want %q", page.Count, plain[1])
	}
	if one := decode[view.FilesPage](t, f.h.do(get(f.at("", "sub", ""), f.sess))); one.Count != "0 items" {
		t.Errorf("an empty folder counts %q", one.Count)
	}

	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		for i := range b.Collections {
			if b.Collections[i].Path == filesDocs {
				b.Collections[i].ReadOnly = true
			}
		}
	})
	page = decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	if string(page.Subtitle) != title || string(page.Count) != plain[1] || string(page.Access) != ro[3] {
		t.Errorf("read-only line = %q, %q, %q; want %q, %q, %q", page.Subtitle, page.Count, page.Access, title, plain[1], ro[3])
	}
}

// ФН2: an empty folder says what to do with it — «This folder is empty.
// Upload files into it, or drop them onto the window.» with Upload and New
// folder (states-32); a read-only one has nothing to offer.
func TestEmptyFolderIsTheFrame(t *testing.T) {
	m := frameOne(t, regexp.MustCompile(`(?s)<div class="tc-empty"><b>(.*?)</b>(.*?)<button`), mockupFrame(t, "states-32"), "empty folder")
	title, body := frameText(m[1]), frameText(m[2])

	f := filesFixture(t, true)
	f.srv.SeedFolder(filesDocs + "sub/")
	page := decode[view.FilesPage](t, f.h.do(get(f.at("", "sub", ""), f.sess)))
	if page.Empty == nil || string(page.Empty.Title) != title || string(page.Empty.Body) != body {
		t.Fatalf("empty = %+v, want %q %q", page.Empty, title, body)
	}
	up, nf := actionByID(page.Empty.Actions, "file.upload"), actionByID(page.Empty.Actions, "folder.new")
	if up == nil || nf == nil || !up.Primary || len(up.Arrow) != 2 || !up.Enabled || !nf.Enabled || len(page.Empty.Actions) != 2 {
		t.Errorf("the empty folder's buttons = %+v, want Upload with its arrow and New folder", page.Empty.Actions)
	}
	// A folder with something in it, and one whose filter hides everything,
	// are not empty.
	if full := decode[view.FilesPage](t, f.h.do(get(f.at("")+"?filter=zzz", f.sess))); full.Empty != nil || len(full.Rows) != 0 {
		t.Errorf("a folder filtered to nothing = %+v, %d rows", full.Empty, len(full.Rows))
	}
	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		for i := range b.Collections {
			if b.Collections[i].Path == filesDocs {
				b.Collections[i].ReadOnly = true
			}
		}
	})
	page = decode[view.FilesPage](t, f.h.do(get(f.at("", "sub", ""), f.sess)))
	if page.Empty == nil || string(page.Empty.Title) != title || page.Empty.Body != "" || len(page.Empty.Actions) != 0 {
		t.Errorf("a read-only empty folder = %+v, want the title alone", page.Empty)
	}
}

// ФН3: a storage whose server did not answer says so and nothing else
// (states-33): the model says the folder was not read — no count, no rows,
// no queue —, and the message is the frame's.
func TestUnansweredFolderIsTheFrame(t *testing.T) {
	m := frameOne(t, regexp.MustCompile(`(?s)<div class="tc-msg is-alert"><b>(.*?)</b>(.*?)<button class="tc-btn is-sm" type="button">(.*?)</button>`), mockupFrame(t, "states-33"), "alert")
	title, body, again := frameText(m[1]), frameText(m[2]), frameText(m[3])
	if !strings.HasSuffix(title, " is unavailable.") {
		t.Fatalf("the alert of states-33 is %q: read the frame again", title)
	}

	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"a.txt", "alpha")
	f.srv.Refuse("PROPFIND", http.StatusServiceUnavailable, "")
	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	if !page.Unread || page.Count != "" || len(page.Rows) != 0 || page.Queue != nil || page.Empty != nil {
		t.Errorf("unread %v, count %q, %d rows, queue %+v, empty %+v", page.Unread, page.Count, len(page.Rows), page.Queue, page.Empty)
	}
	if len(page.Notices) != 1 {
		t.Fatalf("notices = %+v", page.Notices)
	}
	n := page.Notices[0]
	if n.Role != view.NoticeAlert || string(n.Title)+"." != strings.Replace(title, "Documents", "Docs", 1) || string(n.Body) != body || n.Details == nil {
		t.Errorf("notice = %+v, want %q %q", n, title, body)
	}
	if a := actionByID(n.Actions, "source.retry"); a == nil || string(a.Label) != again || a.Href != f.at("") {
		t.Errorf("retry = %+v", n.Actions)
	}
}

// ФН4: the name of a row is a link — a folder's to the folder, a file's to
// its folder with it chosen (s-files-1: TableFile.Href) —, a field of its
// own: Href of a file stays its download.
func TestRowNameHasItsAddress(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"a b.txt", "alpha")
	f.srv.SeedFolder(filesDocs + "sub/")
	page := decode[view.FilesPage](t, f.h.do(get(f.at("")+"?sort=size", f.sess)))
	if len(page.Rows) != 2 {
		t.Fatalf("rows = %+v", page.Rows)
	}
	dir, file := page.Rows[0], page.Rows[1]
	if dir.Open != dir.Href || dir.Open != f.at("", "sub", "") {
		t.Errorf("a folder's name leads to %q, the row to %q", dir.Open, dir.Href)
	}
	if file.Open != f.at("", "a b.txt") || file.Href == file.Open {
		t.Errorf("a file's name leads to %q (download %q), want %q", file.Open, file.Href, f.at("", "a b.txt"))
	}
	opened := decode[view.FilesPage](t, f.h.do(get(file.Open, f.sess)))
	if len(opened.Rows) != 2 || !opened.Rows[1].Selected || opened.Details == nil {
		t.Errorf("the name's address did not open the folder with the file chosen: %+v", opened.Rows)
	}
}

// ФН5 (the owner's decisions of 09.10.2026: the frame is right, and without
// scripts the selection bar always stands): a tick of a row or of a tile is
// a field of a form — «item», its value the name (s-files-5) —, and the
// form is the operations': Move, Copy, Delete and Rename of the bar send
// it. So the bar's operations are there with nothing selected too, and
// what is ticked travels with the one pressed.
func TestSelectionIsTheOperationsForm(t *testing.T) {
	field := frameOne(t, regexp.MustCompile(`<input class="tc-tile__box" type="checkbox" name="([^"]+)" value="[^"]+"`), mockupFrame(t, "s-files-5"), "tick of a tile")[1]

	f := filesFixture(t, true)
	for _, name := range []string{"a.txt", "b.txt", "c.md"} {
		f.srv.SeedFile(filesDocs+name, name)
	}
	f.srv.SeedFolder(filesDocs + "sub/")
	here := f.at("")
	page := decode[view.FilesPage](t, f.h.do(get(here, f.sess)))
	sel := page.Select
	if sel == nil || sel.Method != "POST" || sel.Action != f.at("op") || formRow(*sel, "_csrf") == nil || len(sel.Actions) != 0 {
		t.Fatalf("selection form = %+v, want a POST of the operations with nothing to clear", sel)
	}
	if page.SelectField != field {
		t.Fatalf("a tick is the field %q, the frame's %q", page.SelectField, field)
	}
	// Nothing selected, the operations stand: each is the form's button
	// with its operation in its address and no names of its own.
	for _, id := range []string{"file.rename", "file.move", "file.copy", "file.delete"} {
		a := actionByID(page.Bar.Actions, id)
		if a == nil || !a.Enabled || !a.Submits || a.Method != "POST" || strings.Contains(a.Href, "item=") ||
			!strings.HasPrefix(a.Href, f.at("op")+"?op=") {
			t.Errorf("%s = %+v, want a button of the selection form", id, a)
		}
	}
	if d := actionByID(page.Bar.Actions, "file.download"); d == nil || d.Enabled || d.Reason == "" {
		t.Errorf("Download with nothing selected = %+v", d)
	}
	send := func(id string, names ...string) *httptest.ResponseRecorder {
		t.Helper()
		fields := formFields(*sel)
		for _, n := range names {
			fields.Add(field, n)
		}
		return f.h.do(postForm(actionHref(t, page.Bar.Actions, id), f.sess, fields))
	}
	// Two ticked and Move pressed: the sheet of a move of the two.
	sheet := decode[view.FileOpSheet](t, send("file.move", "a.txt", "b.txt"))
	if sheet.Op != view.FileMove || len(sheet.Items) != 2 || sheet.Form == nil || !strings.Contains(sheet.Form.Action, "item=a.txt") || !strings.Contains(sheet.Form.Action, "item=b.txt") {
		t.Fatalf("the sheet of two ticked = %+v", sheet)
	}
	// Nothing ticked, and a rename of two, say so on the folder.
	if none := decode[view.FilesPage](t, send("file.delete")); !hasNoticeBody(none.Notices, "Tick the files and folders first.") {
		t.Errorf("nothing ticked: %+v", none.Notices)
	}
	if two := decode[view.FilesPage](t, send("file.rename", "a.txt", "b.txt")); !hasNoticeBody(two.Notices, "Rename takes one item.") {
		t.Errorf("a rename of two: %+v", two.Notices)
	}
	if one := decode[view.FileOpSheet](t, send("file.rename", "c.md")); one.Op != view.FileRename || one.Form == nil {
		t.Errorf("a rename of one ticked = %+v", one)
	}

	// What the address selects is ticked, counted and can be cleared; the
	// old ticks of the address — toggle, all — are no longer answered.
	picked := decode[view.FilesPage](t, f.h.do(get(here+"?sort=size&select=b.txt", f.sess)))
	if !fileRow(t, picked, "b.txt").Selected || picked.Bar.Selected != 1 ||
		actionHref(t, picked.Select.Actions, "files.select.clear") != here+"?sort=size" {
		t.Fatalf("selected by the address: bar %d, form %+v", picked.Bar.Selected, picked.Select)
	}
	if d := actionByID(picked.Bar.Actions, "file.download"); d == nil || !d.Enabled || d.Submits {
		t.Errorf("Download of the one selected = %+v", d)
	}
	// What cannot be done is off: a rename of the two the address selects.
	both := decode[view.FilesPage](t, f.h.do(get(here+"?select=a.txt&select=b.txt", f.sess)))
	if r := actionByID(both.Bar.Actions, "file.rename"); r == nil || r.Enabled || r.Reason != "Rename takes one item." {
		t.Errorf("Rename of two selected = %+v, want it off with why", r)
	}
	if m := actionByID(both.Bar.Actions, "file.move"); m == nil || !m.Enabled {
		t.Errorf("Move of two selected = %+v", m)
	}
	if rec := f.h.do(get(here+"?toggle=a.txt", f.sess)); rec.Code != http.StatusOK {
		t.Errorf("toggle in the address: %d, want the folder as it is", rec.Code)
	}
}

// ФН17…ФН19 (the owner's word of 09.10.2026: dates as the frame draws them,
// for now): the tables of files and of links say dates in numbers — «2026-
// 08-10», «2026-08-10 08:41» (s-files-1, s-publish-1) —; Copy of a link
// takes its whole address, and a link nobody opened says so by a mark, not
// by its words alone.
func TestFileAndLinkTablesAreTheFrames(t *testing.T) {
	day, stamp := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`), regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}$`)
	links := mockupFrame(t, "s-publish-1")
	cells := regexp.MustCompile(`<td class="tc-mono[^"]*">([^<]*)</td>`).FindAllStringSubmatch(links, -1)
	copied := frameOne(t, regexp.MustCompile(`data-tc-copy="([^"]+)"`), links, "address Copy takes")[1]
	if len(cells) < 10 || !day.MatchString(cells[1][1]) || !stamp.MatchString(cells[8][1]) || !regexp.MustCompile(`^https?://[^/]+/p/`).MatchString(copied) ||
		!strings.Contains(links, `<td class="tc-mono is-2nd tc-mono--dusty">never opened</td>`) {
		t.Fatalf("s-publish-1 draws %q and copies %q: read the frame again", cells, copied)
	}
	changed := regexp.MustCompile(`<td class="tc-mono">(\d[^<]*)</td>`).FindStringSubmatch(mockupFrame(t, "s-files-1"))
	if changed == nil || !stamp.MatchString(changed[1]) {
		t.Fatalf("s-files-1 says Changed as %q: read the frame again", changed)
	}

	f := publishFixture(t)
	link := publishLink(t, f, f.publishHref("calendar", searchWork, ""), false)
	row := decode[view.PublishedLinksPage](t, f.h.do(get("/app/files/published", f.sess))).Rows[0]
	if !day.MatchString(string(row.Created)) || !day.MatchString(string(row.Expires)) || row.LastSeen != "never opened" || !row.Unopened {
		t.Errorf("a link nobody opened: created %q, expires %q, last seen %q, unopened %v", row.Created, row.Expires, row.LastSeen, row.Unopened)
	}
	if a := string(row.Address); !regexp.MustCompile(`^https?://[^/]+`+regexp.QuoteMeta(link)+`$`).MatchString(a) || string(row.Link) != link {
		t.Errorf("address %q, link %q; want the whole address of %q", row.Address, row.Link, link)
	}
	publicPage(t, f, link)
	row = decode[view.PublishedLinksPage](t, f.h.do(get("/app/files/published", f.sess))).Rows[0]
	if !stamp.MatchString(string(row.LastSeen)) || row.Unopened || row.Opens != 1 {
		t.Errorf("a link opened once: last seen %q, unopened %v, opens %d", row.LastSeen, row.Unopened, row.Opens)
	}
	forever := publishLink(t, f, f.publishHref("notes", searchWork, "n1"), true)
	for _, r := range decode[view.PublishedLinksPage](t, f.h.do(get("/app/files/published", f.sess))).Rows {
		if string(r.Link) == forever && r.Expires != "No expiry" {
			t.Errorf("a link without an end expires %q", r.Expires)
		}
	}

	files := filesFixture(t, true)
	files.srv.SeedFile(filesDocs+"a.txt", "alpha")
	page := decode[view.FilesPage](t, files.h.do(get(files.at(""), files.sess)))
	if !stamp.MatchString(string(fileRow(t, page, "a.txt").Modified)) {
		t.Errorf("a file changed %q, want it in numbers", fileRow(t, page, "a.txt").Modified)
	}
}

// ФН22: the root with no storage ticked (states-35) — «0 of 3 storages»
// under the title, and one message: «No storages are ticked.», what to do,
// and the button that ticks them all.
func TestFilesRootWithNothingTickedIsTheFrame(t *testing.T) {
	frame := mockupFrame(t, "states-35")
	sub := frameText(frameOne(t, frameHeadSub, frame, "line under the title")[1])
	m := frameOne(t, regexp.MustCompile(`(?s)<div class="tc-empty"><b>(.*?)</b>(.*?)<button class="tc-btn is-primary" type="button">(.*?)</button>`), frame, "empty root")
	title, body, tick := strings.TrimSuffix(frameText(m[1]), "."), frameText(m[2]), frameText(m[3])
	count := regexp.MustCompile(`^0 of (\d+) storages$`).FindStringSubmatch(sub)
	if count == nil {
		t.Fatalf("states-35 says %q under the title: read the frame again", sub)
	}

	f := filesFixture(t, false)
	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		b.Collections = append(b.Collections,
			sealed.Collection{AccountID: f.id, Path: davFilesOld, Name: "Old", Kinds: []view.Kind{view.KindFile}},
			sealed.Collection{AccountID: f.id, Path: filesDocs + "third/", Name: "Third", Kinds: []view.Kind{view.KindFile}})
	})
	root := decode[view.FilesRootPage](t, f.h.do(get("/app/files", f.sess)))
	if string(root.Subtitle) != sub || len(root.Collections) != 0 || len(root.Note) != 0 {
		t.Errorf("subtitle %q, %d rows, note %+v; want %q and nothing else", root.Subtitle, len(root.Collections), root.Note, sub)
	}
	if root.Empty == nil || string(root.Empty.Title) != title || string(root.Empty.Body) != body || len(root.Empty.Actions) != 1 {
		t.Fatalf("empty = %+v, want %q %q", root.Empty, title, body)
	}
	if a := root.Empty.Actions[0]; a.ID != "rail.tick.all" || string(a.Label) != tick || !a.Primary || !a.Enabled {
		t.Errorf("the button = %+v, want %q", a, tick)
	}
}

// ФН15: Attach of From Files is drawn without an icon (s-files-16).
func TestAttachFromFilesHasNoIcon(t *testing.T) {
	if !strings.Contains(mockupFrame(t, "s-files-16"), `<button class="tc-btn is-primary" type="button" disabled>Attach</button>`) {
		t.Fatal("s-files-16 draws Attach otherwise: read the frame again")
	}
	f := attachFixture(t, 1024, []string{"carrel"}, "e1.ics", attachEvent("e1", "Sync-up", ""))
	from := menuAction(actionByID(eventAt(t, f, "e1").Actions, "attach.add").Arrow, "attach.files")
	sheet := decode[view.FilePickerSheet](t, f.h.do(get(from.Href, f.sess)))
	if a := actionByID(sheet.Actions, "files.picker.attach"); a == nil || a.Icon != "" || !a.Primary {
		t.Errorf("Attach = %+v, want it primary and without an icon", a)
	}
}
