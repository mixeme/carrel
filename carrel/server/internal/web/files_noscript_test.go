package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// U-8: the files section without scripts — every operation of the client's
// scan of the section (client.md, «Раздел файлов: пробелы контракта»,
// ФГ3, ФБ12, ФБ9, ФБ13, ФГ1, ФГ2, ФБ14) is reachable through what the view
// model carries, without a field the client would have to know of itself.

// ФГ3: the rename sheet carries the field of the new name. Its form posts to
// the confirmed operation; the name it shows is the one there now.
func TestRenameSheetCarriesTheNewName(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"a.txt", "alpha")
	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	sheet := opSheet(t, f, rowMenu(t, fileRow(t, page, "a.txt"), "file.rename"), url.Values{})
	if sheet.Form == nil {
		t.Fatalf("the rename sheet has no form for the new name: %+v", sheet)
	}
	name := formRow(*sheet.Form, "name")
	if name == nil || name.Value != "a.txt" || !name.Focus || !name.Required || sheet.Form.Method != "POST" ||
		sheet.Form.Action != confirmHref(t, sheet) || !strings.Contains(sheet.Form.Action, "go=1") {
		t.Fatalf("form = %+v", sheet.Form)
	}
	fields := formFields(*sheet.Form)
	fields.Set("name", "b.txt")
	if rec := f.h.do(postForm(sheet.Form.Action, f.sess, fields)); rec.Code != http.StatusSeeOther {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body)
	}
	if got, ok := f.srv.File(filesDocs + "b.txt"); !ok || got != "alpha" {
		t.Errorf("renamed: %q %v", got, ok)
	}
	// Move, copy and delete name nothing new: a move's form (ФН10) has no
	// field of a name, a deletion has no form.
	page = decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	if move := opSheet(t, f, rowMenu(t, fileRow(t, page, "b.txt"), "file.move"), url.Values{}); move.Form == nil || formRow(*move.Form, "name") != nil {
		t.Errorf("a move sheet with a name: %+v", move.Form)
	}
	if del := opSheet(t, f, rowMenu(t, fileRow(t, page, "b.txt"), "file.delete"), url.Values{}); del.Form != nil {
		t.Errorf("a deletion with a form: %+v", del.Form)
	}
}

// ФБ12: New folder of the bar and of the rail posts no name; the answer is
// the folder with the line of the name open, and that line makes it.
func TestNewFolderWithoutANameAsksForIt(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"a.txt", "alpha")
	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	bar := actionHref(t, page.Bar.Actions, "folder.new")
	var rail string
	for _, a := range page.Rail.Foot {
		if a.ID == "rail.folder.new" {
			rail = a.Href
		}
	}
	if rail != bar {
		t.Fatalf("New folder of the rail %q, of the bar %q", rail, bar)
	}
	asked := decode[view.FilesPage](t, f.h.do(postForm(bar, f.sess, url.Values{})))
	if len(asked.Notices) != 0 || asked.Name == nil {
		t.Fatalf("New folder without a name: notices %+v, line %+v", asked.Notices, asked.Name)
	}
	name := formRow(*asked.Name, "name")
	if name == nil || !name.Focus || !name.Required || asked.Name.Action != bar || asked.Name.Method != "POST" ||
		actionHref(t, asked.Name.Actions, "folder.new.cancel") != f.at("") {
		t.Fatalf("line = %+v", asked.Name)
	}
	fields := formFields(*asked.Name)
	fields.Set("name", "inbox")
	if rec := f.h.do(postForm(asked.Name.Action, f.sess, fields)); rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	if !fileRow(t, decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess))), "inbox").IsDir {
		t.Error("no folder made")
	}
	// A refusal keeps the line open with what was typed.
	taken := decode[view.FilesPage](t, f.h.do(postForm(bar, f.sess, url.Values{"name": {"inbox"}})))
	if taken.Name == nil || formRow(*taken.Name, "name").Value != "inbox" ||
		!hasNoticeBody(taken.Notices, "There is a folder called «inbox» in Docs already.") {
		t.Errorf("taken: line %+v, notices %+v", taken.Name, taken.Notices)
	}
	if plain := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess))); plain.Name != nil {
		t.Errorf("the line stands on a folder nobody asked: %+v", plain.Name)
	}
}

// ФБ9: selection is a form of its own (as RestoreMatch.Form): a GET of the
// folder whose ticks toggle one name, the head's tick all of them; what is
// selected travels in select, and Clear selection is the folder without it.
func TestSelectionWithoutScripts(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"a.txt", "alpha")
	f.srv.SeedFile(filesDocs+"b.txt", "beta")
	f.srv.SeedFile(filesDocs+"c.md", "gamma")
	here := f.at("")
	page := decode[view.FilesPage](t, f.h.do(get(here+"?sort=size", f.sess)))
	if page.Select == nil {
		t.Fatal("the folder has no selection form")
	}
	if page.Select.Method != "GET" || page.Select.Action != here || formRow(*page.Select, "sort") == nil ||
		formRow(*page.Select, "sort").Kind != view.InputHidden || formRow(*page.Select, "_csrf") != nil {
		t.Fatalf("form = %+v", page.Select)
	}
	for _, a := range page.Select.Actions {
		if a.ID == "files.select.clear" {
			t.Errorf("Clear selection with nothing selected: %+v", a)
		}
	}
	follow := func(q url.Values) url.Values {
		t.Helper()
		rec := f.h.do(get(here+"?"+q.Encode(), f.sess))
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("%v: %d %s", q, rec.Code, rec.Body)
		}
		loc, _ := url.Parse(rec.Header().Get("Location"))
		if loc.EscapedPath() != here {
			t.Fatalf("%v went to %q", q, loc)
		}
		return loc.Query()
	}
	fields := formFields(*page.Select)
	fields.Set("toggle", "b.txt")
	got := follow(fields)
	if strings.Join(got["select"], ",") != "b.txt" || got.Get("sort") != "size" || got.Has("toggle") {
		t.Fatalf("toggle on: %v", got)
	}
	page = decode[view.FilesPage](t, f.h.do(get(here+"?"+got.Encode(), f.sess)))
	if !fileRow(t, page, "b.txt").Selected || page.Bar.Selected != 1 ||
		actionHref(t, page.Select.Actions, "files.select.clear") != here+"?sort=size" {
		t.Fatalf("selected: bar %d, form %+v", page.Bar.Selected, page.Select)
	}
	fields = formFields(*page.Select)
	fields.Set("toggle", "a.txt")
	if got = follow(fields); strings.Join(got["select"], ",") != "a.txt,b.txt" {
		t.Errorf("a second toggle: %v", got)
	}
	fields.Set("toggle", "b.txt")
	if got = follow(fields); got.Has("select") {
		t.Errorf("toggle off: %v", got)
	}
	fields.Del("toggle")
	fields.Set("all", "1")
	if got = follow(fields); strings.Join(got["select"], ",") != "a.txt,b.txt,c.md" {
		t.Errorf("all: %v", got)
	}
	if got = follow(url.Values{"select": {"a.txt", "b.txt", "c.md"}, "all": {"1"}}); got.Has("select") {
		t.Errorf("all of all: %v", got)
	}
	// A name the folder does not hold is not selected.
	if got = follow(url.Values{"toggle": {"../x"}}); got.Has("select") {
		t.Errorf("a stranger toggled: %v", got)
	}
}

// ФБ13: the actions that carry files name their field; S-17.1's sizes are
// what C-5 names, not only the code.
func TestUploadActionsNameTheirFileField(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFolder(filesDocs)
	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	var upload view.Action
	for _, a := range page.Bar.Actions {
		if a.ID == "file.upload" {
			upload = a
		}
	}
	if upload.File != "file" || len(upload.Arrow) != 2 || upload.Arrow[0].File != "file" || upload.Arrow[1].File != "file" {
		t.Errorf("Upload = %+v", upload)
	}
	sheet := decode[view.FilePickerSheet](t, f.h.do(get("/app/files/picker?"+url.Values{"for": {"upload"}, "to": {f.at("")}}.Encode(), f.sess)))
	if a := sheet.Actions[0]; a.ID != "files.picker.upload" || a.File != "file" {
		t.Errorf("Upload = %+v", a)
	}
}

// ФГ1, ФГ2: every node of a tree has the address of the same sheet with that
// node chosen — a GET, so a link chooses without scripts; the confirmed
// operation stays a POST, and a GET of it changes nothing.
func TestTreeNodesHaveAddresses(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"a.txt", "alpha")
	f.srv.SeedFolder(filesDocs + "sub/")
	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	move := opSheet(t, f, rowMenu(t, fileRow(t, page, "a.txt"), "file.move"), url.Values{})
	root := move.Target.Nodes[0]
	if !root.Expanded || len(root.Children) != 1 {
		t.Fatalf("tree = %+v", move.Target)
	}
	sub := root.Children[0]
	if sub.Href == "" || root.Href == "" {
		t.Fatalf("nodes without an address: %+v", move.Target)
	}
	u, _ := url.Parse(sub.Href)
	if u.EscapedPath() != f.at("op") || u.Query().Get("to") != sub.ID || u.Query().Get("op") != "move" || u.Query().Get("item") != "a.txt" || u.Query().Has("go") {
		t.Fatalf("node address %q", sub.Href)
	}
	chosen := decode[view.FileOpSheet](t, f.h.do(get(sub.Href, f.sess)))
	if chosen.Target.Chosen != sub.ID || !chosen.Actions[0].Enabled {
		t.Fatalf("the node's address chose %q: %+v", chosen.Target.Chosen, chosen.Actions[0])
	}
	if a := chosen.Actions[0]; !strings.Contains(a.Href, "go=1") || a.Method != "POST" {
		t.Errorf("confirm = %+v", a)
	}
	// A GET of the confirmed operation is the sheet again, and moves nothing.
	if rec := f.h.do(get(chosen.Actions[0].Href, f.sess)); rec.Code != http.StatusOK {
		t.Errorf("GET of go=1: %d", rec.Code)
	}
	if _, ok := f.srv.File(filesDocs + "a.txt"); !ok {
		t.Fatal("a GET moved the file")
	}

	picker := decode[view.FilePickerSheet](t, f.h.do(get("/app/files/picker?for=upload", f.sess)))
	node := picker.Tree.Nodes[0]
	want := "/app/files/picker?" + url.Values{"for": {"upload"}, "to": {node.ID}}.Encode()
	if node.Href != want {
		t.Errorf("picker node %q, want %q", node.Href, want)
	}
	// Choose this folder names the chosen one in its address: without
	// scripts there is no field to send it.
	chosenPicker := decode[view.FilePickerSheet](t, f.h.do(get(node.Href, f.sess)))
	choose := actionHref(t, chosenPicker.Actions, "files.picker.choose")
	if cu, _ := url.Parse(choose); cu.Query().Get("to") != node.ID {
		t.Errorf("Choose this folder = %q", choose)
	}
}

// ФБ14: a folder of the list takes a drop: its row carries the upload into
// it; a file's row does not.
func TestFolderRowsTakeADrop(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"a.txt", "alpha")
	f.srv.SeedFolder(filesDocs + "sub/")
	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	sub := fileRow(t, page, "sub")
	if sub.Upload == nil || sub.Upload.Href != f.at("upload", "sub", "") || sub.Upload.File != "file" || sub.Upload.Method != "POST" || !sub.Upload.Enabled {
		t.Fatalf("folder row's upload = %+v", sub.Upload)
	}
	if a := fileRow(t, page, "a.txt"); a.Upload != nil {
		t.Errorf("a file takes a drop: %+v", a.Upload)
	}
}
