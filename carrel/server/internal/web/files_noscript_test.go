package web_test

import (
	"encoding/json"
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
	// The node's address is the sheet with it chosen: Upload goes there.
	chosenPicker := decode[view.FilePickerSheet](t, f.h.do(get(node.Href, f.sess)))
	if up := actionByID(chosenPicker.Actions, "files.picker.upload"); up == nil || chosenPicker.Tree.Chosen != node.ID || !strings.Contains(up.Href, "/upload/") {
		t.Errorf("the sheet with a node chosen: chosen %q, upload %+v", chosenPicker.Tree.Chosen, up)
	}
}

// A drop goes into the open folder, not onto a folder's row (Р-59 of
// Typecase, the owner's decision of 13.10.2026, note 60): no row carries an
// upload of its own — the folder's bar has Upload, the page its drop sheet.
func TestNoRowTakesADrop(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"a.txt", "alpha")
	f.srv.SeedFolder(filesDocs + "sub/")
	rec := f.h.do(get(f.at(""), f.sess))
	var raw struct {
		Rows []map[string]json.RawMessage `json:"rows"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil || len(raw.Rows) != 2 {
		t.Fatalf("rows: %v, %d", err, len(raw.Rows))
	}
	for _, r := range raw.Rows {
		if up, ok := r["upload"]; ok {
			t.Errorf("row %s carries an upload: %s", r["name"], up)
		}
	}
	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	if up := actionByID(page.Bar.Actions, "file.upload"); up == nil || !up.Enabled || up.File == "" {
		t.Errorf("the bar's Upload: %+v", up)
	}
}

// Note 61 of Typecase (s-files-4, -13): without scripts the sheet of a move
// has a second button of its form, «Open the folder», which sends the folder
// ticked and answers the same sheet with it chosen and open — nothing is
// moved.
func TestOpenTheFolderWithoutScripts(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"a.txt", "alpha")
	f.srv.SeedFolder(filesDocs + "sub/")
	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	move := opSheet(t, f, rowMenu(t, fileRow(t, page, "a.txt"), "file.move"), url.Values{})
	open := actionByID(move.Actions, "file.op.open")
	if open == nil || !open.Submits || !open.Enabled || open.Method != "POST" {
		t.Fatalf("Open the folder = %+v", open)
	}
	sub := move.Target.Nodes[0].Children[0]
	form := formFields(*move.Form)
	form.Set("to", sub.ID)
	form.Set("open", "1")
	sheet := decode[view.FileOpSheet](t, f.h.do(postForm(move.Form.Action, f.sess, form)))
	if sheet.Target == nil || sheet.Target.Chosen != sub.ID {
		t.Fatalf("Open the folder answered %+v", sheet.Target)
	}
	if _, ok := f.srv.File(filesDocs + "a.txt"); !ok {
		t.Fatal("Open the folder moved the file")
	}
}
