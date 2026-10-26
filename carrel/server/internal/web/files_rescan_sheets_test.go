package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
	"github.com/emersion/go-webdav/carddav"
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

// ФН12, note 135: a deletion says that Carrel keeps no copy, then that
// nobody looked for the files among the records (s-files-22); the path
// stands without «from». One folder is «Delete folder» with its own path,
// and says that everything in it goes too (s-files-27); the folder of
// attachments inside it points at it.
func TestDeleteSheetIsTheFrame(t *testing.T) {
	frame := mockupFrame(t, "s-files-22")
	path := frameText(frameOne(t, frameOpPath, frame, "path")[1])
	alert := frameText(frameOne(t, frameOpFlushAlert, frame, "alert")[1])
	msg := frameText(frameOne(t, frameOpFlushMsg, frame, "message")[1])
	segs := strings.Split(path, " / ")
	if alert != "Carrel keeps no copy: this cannot be undone." || len(segs) < 2 {
		t.Fatalf("s-files-22 says %q under %q: read the frame again", alert, path)
	}
	folderFrame := mockupFrame(t, "s-files-27")
	folderTitle := frameText(frameOne(t, regexp.MustCompile(`<b class="tc-sheet__title">(.*?)</b>`), folderFrame, "title")[1])
	folderAlert := frameText(frameOne(t, frameOpFlushAlert, folderFrame, "alert")[1])
	folderMsg := frameText(frameOne(t, frameOpFlushMsg, folderFrame, "message")[1])
	if !strings.HasSuffix(folderAlert, " "+alert) {
		t.Fatalf("s-files-27 says %q: read the frame again", folderAlert)
	}

	f := filesFixture(t, true)
	dir := filesDocs + strings.Join(segs[1:], "/") + "/"
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		f.srv.SeedFile(dir+name, strings.Repeat(".", 1500))
	}
	f.srv.SeedFolder(dir + "sub/")
	f.srv.SeedFile(dir+"sub/inside.txt", "x")
	sheet := opSheetOf(t, f, "delete", []string{"a.txt", "b.txt", "c.txt"}, segs[1:]...)
	if want := "Docs / " + strings.Join(segs[1:], " / "); string(sheet.Subject) != want {
		t.Errorf("subject = %q, want %q", sheet.Subject, want)
	}
	if plainText(sheet.Alert) != alert || plainText(sheet.Warning) != msg || len(sheet.References) != 0 {
		t.Errorf("alert = %q, warning %q, references %+v; the frame %q, %q and none", plainText(sheet.Alert), plainText(sheet.Warning), sheet.References, alert, msg)
	}
	if a := actionByID(sheet.Actions, "file.delete.confirm"); a == nil || !a.Danger || a.Primary {
		t.Errorf("Delete = %+v", a)
	}

	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		b.Attachments = &sealed.AttachmentFolder{AccountID: f.id, Path: filesDocs, Folder: append(segs[1:len(segs):len(segs)], "sub", "2026-07")}
	})
	folder := opSheetOf(t, f, "delete", []string{"sub"}, segs[1:]...)
	if want := "Docs / " + strings.Join(segs[1:], " / ") + " / sub"; string(folder.Title) != folderTitle || string(folder.Subject) != want {
		t.Errorf("a folder: title %q, subject %q; want %q, %q", folder.Title, folder.Subject, folderTitle, want)
	}
	if plainText(folder.Alert) != folderAlert || plainText(folder.Warning) != folderMsg {
		t.Errorf("a folder: alert %q, warning %q; the frame %q, %q", plainText(folder.Alert), plainText(folder.Warning), folderAlert, folderMsg)
	}
	if got := refLines(folder.References); len(got) != 1 || !strings.HasPrefix(got[0], "Attachments — ") {
		t.Errorf("a folder holding the attachments: references %q", got)
	}
	mixed := opSheetOf(t, f, "delete", []string{"sub", "a.txt"}, segs[1:]...)
	if got := plainText(mixed.Alert); got != "Everything in the folder goes too. "+alert || plainText(mixed.Warning) != msg {
		t.Errorf("a folder and a file: alert %q, warning %q", got, plainText(mixed.Warning))
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
	form := frameOne(t, regexp.MustCompile(`<form class="tc-sheet[^"]*"(?: id="[^"]*")? action="[^"]*\?op=move" method="(post)">`), frame, "form of the sheet")
	field := frameOne(t, regexp.MustCompile(`<input class="tc-tree__box" type="radio" name="([^"]+)" value="[^"]+" checked[ >]`), frame, "chosen node")[1]
	word := frameOne(t, regexp.MustCompile(`<button class="tc-btn is-primary" type="submit">([^<]+)</button>`), frame, "button of the form")[1]
	if form[1] != "post" || word != "Move" {
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

// ФН9 (the owner's decision of 09.10.2026: the frame is right): the head
// of a sheet with a tree chooses the server (s-files-4, s-notes-11,
// s-files-16: the pick «Server»), and the tree holds the storages of that
// server alone, their roots without the server's name. A server is chosen
// by its address — the sheet with it —, and with another server chosen
// nothing is ticked yet.
func TestSheetsChooseTheServer(t *testing.T) {
	for _, id := range []string{"s-files-4", "s-notes-11", "s-files-16"} {
		// The pick opens a menu of the servers, each a link (s-files-26).
		if !regexp.MustCompile(`<summary class="tc-in is-pick[^"]*" aria-label="Server">[^<]+</summary>`).MatchString(mockupFrame(t, id)) {
			t.Fatalf("%s draws no pick of the server: read the frame again", id)
		}
	}

	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"a.txt", "alpha")
	far := searchServer(t)
	far.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	far.SeedFolder(filesDocs)
	farID := addAccount(t, f, far)
	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		b.Collections = append(b.Collections, sealed.Collection{AccountID: farID, Path: filesDocs, Name: "Far", Kinds: []view.Kind{view.KindFile}})
	})
	farRoot := "/app/files/" + farID + "/" + url.PathEscape(filesDocs) + "/"
	check := func(what string, servers []view.FileServer, tree view.FileTree, on int, roots ...string) {
		t.Helper()
		if len(servers) != 2 || servers[on].On == servers[1-on].On || !servers[on].On || servers[0].Label == servers[1].Label ||
			servers[on].Href == "" || servers[1-on].Href == "" {
			t.Fatalf("%s: servers = %+v, want two, the %d-th chosen, both addresses", what, servers, on)
		}
		var got []string
		for _, n := range tree.Nodes {
			got = append(got, n.Label)
			if n.Stamp != "" {
				t.Errorf("%s: the root %q is stamped %q: the head names the server", what, n.Label, n.Stamp)
			}
		}
		if !equalStrings(got, roots) {
			t.Errorf("%s: roots = %q, want %q", what, got, roots)
		}
	}

	// A move: the server of the folder the sheet opens at; the other one by
	// its address, with nothing chosen there and the button off.
	move := opSheetOf(t, f, "move", []string{"a.txt"})
	check("move", move.Servers, *move.Target, 0, "Docs")
	other := decode[view.FileOpSheet](t, f.h.do(get(move.Servers[1].Href, f.sess)))
	check("move, the other server", other.Servers, *other.Target, 1, "Far")
	if other.Target.Chosen != "" || other.Actions[0].Enabled || other.Actions[0].Reason != "Choose a folder." {
		t.Errorf("the other server: chosen %q, confirm %+v", other.Target.Chosen, other.Actions[0])
	}
	// A node of it chosen, the sheet is of that server and the move can go.
	there := decode[view.FileOpSheet](t, f.h.do(get(other.Target.Nodes[0].Href, f.sess)))
	check("move, a folder of the other server", there.Servers, *there.Target, 1, "Far")
	if there.Target.Chosen != farRoot || !there.Actions[0].Enabled {
		t.Errorf("a folder of the other server: chosen %q, confirm %+v", there.Target.Chosen, there.Actions[0])
	}
	// A rename and a deletion have no tree and no pick.
	if del := opSheetOf(t, f, "delete", []string{"a.txt"}); len(del.Servers) != 0 {
		t.Errorf("a deletion picks a server: %+v", del.Servers)
	}

	// The upload of the menu and Browse of the attachment folder.
	up := decode[view.FilePickerSheet](t, f.h.do(get("/app/files/picker?for=upload&to="+url.QueryEscape(f.at("")), f.sess)))
	check("upload", up.Servers, up.Tree, 0, "Docs")
	upFar := decode[view.FilePickerSheet](t, f.h.do(get(up.Servers[1].Href, f.sess)))
	check("upload, the other server", upFar.Servers, upFar.Tree, 1, "Far")
	if upFar.Tree.Chosen != "" || upFar.Where != "" {
		t.Errorf("upload, the other server: chosen %q, where %q", upFar.Tree.Chosen, upFar.Where)
	}
	browse := decode[view.FilePickerSheet](t, f.h.do(get("/app/files/picker?for=attachments&to="+url.QueryEscape(farRoot), f.sess)))
	check("browse", browse.Servers, browse.Tree, 1, "Far")
}

// ФН16: the folder New folder of a tree has just made stands first among
// the folders beside it (s-notes-11: «2026-08 · just created» above
// «2026-07»), whatever its name; the others keep their order.
func TestFolderJustMadeStandsFirst(t *testing.T) {
	badge := regexp.MustCompile(`<span class="tc-badge tc-push">([^<]*)</span>`)
	var rows [][2]string // the name, the badge
	for _, m := range regexp.MustCompile(`(?s)<(?:div|a) class="(?:is-on )?tc-tree__row tc-tree__row--padl58"[^>]*>(.*?)</(?:div|a)>`).FindAllStringSubmatch(mockupFrame(t, "s-notes-11"), -1) {
		row := [2]string{frameText(badge.ReplaceAllString(m[1], ""))}
		if b := badge.FindStringSubmatch(m[1]); b != nil {
			row[1] = b[1]
		}
		rows = append(rows, row)
	}
	if len(rows) != 2 || rows[0][1] != "just created" || rows[1][1] != "" || rows[0][0] < rows[1][0] {
		t.Fatalf("s-notes-11 draws %q under the attachment folder: read the frame again", rows)
	}
	fresh, old := rows[0][0], rows[1][0]

	f := filesFixture(t, true)
	f.srv.SeedFolder(filesDocs + "carrel/" + old + "/")
	f.srv.SeedFolder(filesDocs + "carrel/2026-06/")
	made := decode[view.FilePickerSheet](t, f.h.do(postForm("/app/files/picker?"+url.Values{"for": {"upload"}, "mkcol": {"1"},
		"to": {f.at("", "carrel", "")}}.Encode(), f.sess, url.Values{"name": {fresh}})))
	var labels []string
	for _, n := range made.Tree.Nodes[0].Children[0].Children {
		labels = append(labels, string(n.Label))
	}
	if want := fresh + ",2026-06," + old; strings.Join(labels, ",") != want || !made.Tree.Nodes[0].Children[0].Children[0].Fresh {
		t.Errorf("under the folder: %v, want %s with the first just created", labels, want)
	}
	// Opened again, the sheet has nothing just made, and the order is by name.
	labels = nil
	again := decode[view.FilePickerSheet](t, f.h.do(get("/app/files/picker?for=upload&to="+url.QueryEscape(f.at("", "carrel", "")), f.sess)))
	for _, n := range again.Tree.Nodes[0].Children[0].Children {
		labels = append(labels, string(n.Label))
	}
	if want := "2026-06," + old + "," + fresh; strings.Join(labels, ",") != want {
		t.Errorf("opened again: %v, want %s", labels, want)
	}
}
