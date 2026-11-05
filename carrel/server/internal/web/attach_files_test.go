package web_test

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// from-typecase note 4: Attach has an arrow wherever a file is attached to a
// record (s-agenda-6, s-tasks-7) — the card, the panel, the form and the
// note's page and edit. The button is From this computer; the arrow holds
// it again and From Files, the tree of Files choosing a file. The arrow is
// data of the action (C-6.2): its items, read from the frames drawn open.
func TestAttachArrowIsTheFrame(t *testing.T) {
	f := calendarFixture(t, nil,
		"e1.ics", ics("BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260101T000000Z\nDTSTART:20260810T100000Z\nSUMMARY:Review\nEND:VEVENT"),
		"t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Slides\nEND:VTODO"),
		"n1.ics", ics("BEGIN:VJOURNAL\nUID:n1\nDTSTAMP:20260101T000000Z\nSUMMARY:Plan\nDESCRIPTION:Text\nEND:VJOURNAL"))

	panel := func(kind, uid string) []view.Action {
		return decode[detailsFragment](t, f.h.do(fragment(get(f.path(kind)+"/"+uid, f.sess), "details"))).Body.Actions
	}
	cases := []struct {
		what, frame string
		actions     []view.Action
	}{
		{"event card", "s-agenda-6", decode[view.EventPage](t, f.h.do(get(f.path("calendar")+"/e1", f.sess))).Actions},
		{"event panel", "s-agenda-6", panel("calendar", "e1")},
		{"event form", "s-agenda-6", decode[view.EventEditPage](t, f.h.do(get(f.path("calendar")+"/e1/edit", f.sess))).Form.Actions},
		{"task card", "s-tasks-7", decode[view.TaskPage](t, f.h.do(get(f.path("tasks")+"/t1", f.sess))).Actions},
		{"task panel", "s-tasks-7", panel("tasks", "t1")},
		{"task form", "s-tasks-7", decode[view.TaskEditPage](t, f.h.do(get(f.path("tasks")+"/t1/edit", f.sess))).Form.Actions},
		{"note page", "s-agenda-6", decode[view.NotePage](t, f.h.do(get(f.path("notes")+"/n1", f.sess))).Meta.Actions},
		{"note panel", "s-agenda-6", panel("notes", "n1")},
		{"note edit", "s-agenda-6", decode[view.NotePage](t, f.h.do(get(f.path("notes")+"/n1/edit", f.sess))).Doc.Bar},
	}
	for _, c := range cases {
		a := actionByID(c.actions, "attach.add")
		if a == nil {
			t.Errorf("%s: no Attach among %v", c.what, ids(c.actions))
			continue
		}
		sameMenu(t, c.what+" Attach arrow", a.Arrow, c.frame)
		if len(a.Arrow) != 2 {
			continue
		}
		// The first item is the button itself; the second opens the tree
		// of Files for this record.
		if here := a.Arrow[0]; here.ID != a.ID || here.Href != a.Href || here.Method != a.Method || !here.Enabled {
			t.Errorf("%s: From this computer = %+v, want the button %+v", c.what, here, *a)
		}
		files := a.Arrow[1]
		if files.ID != "attach.files" || files.Method != "GET" || !files.Enabled ||
			!strings.HasPrefix(files.Href, "/app/files/picker?") || !strings.Contains(files.Href, "for=attach") {
			t.Errorf("%s: From Files = %+v", c.what, files)
		}
	}
}

// s-files-8: Upload of the files screen has the same arrow — Files and
// Folder, both the upload of the folder open; which dialog opens is the
// client's.
func TestUploadArrowIsTheFrame(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"report.pdf", "0123456789")
	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	up := actionByID(page.Bar.Actions, "file.upload")
	if up == nil {
		t.Fatalf("no Upload among %v", ids(page.Bar.Actions))
	}
	sameMenu(t, "Upload arrow", up.Arrow, "s-files-8")
	for _, it := range up.Arrow {
		if it.Href != up.Href || it.Method != "POST" || it.Enabled != up.Enabled {
			t.Errorf("%s = %+v, want the upload %+v", it.Label, it, *up)
		}
	}
}

// fromFiles opens the tree of From Files for a record, opens the folder of
// the file there and chooses the file: what a page does, node by node. It
// returns the sheet with the file chosen.
func fromFiles(t *testing.T, f davFixture, arrow []view.MenuItem, folder, name string) view.FilePickerSheet {
	t.Helper()
	href := menuAction(arrow, "attach.files")
	if href == nil {
		t.Fatalf("no From Files in %+v", arrow)
	}
	open := func(to string) view.FilePickerSheet {
		return decode[view.FilePickerSheet](t, f.h.do(get(href.Href+"&to="+url.QueryEscape(to), f.sess)))
	}
	sheet := decode[view.FilePickerSheet](t, f.h.do(get(href.Href, f.sess)))
	if a := actionByID(sheet.Actions, "files.picker.attach"); a == nil || a.Enabled || a.Reason == "" {
		t.Fatalf("nothing chosen yet, Attach = %+v", a)
	}
	sheet = open("/app/files/" + f.id + "/" + url.PathEscape(attachFiles) + "/" + folder)
	var node *view.FileTreeNode
	var walk func([]view.FileTreeNode)
	walk = func(nodes []view.FileTreeNode) {
		for i := range nodes {
			if nodes[i].File && nodes[i].Label == name {
				node = &nodes[i]
			}
			walk(nodes[i].Children)
		}
	}
	walk(sheet.Tree.Nodes)
	if node == nil {
		t.Fatalf("the tree shows no file %s: %+v", name, sheet.Tree.Nodes)
	}
	if node.ReadOnly || len(node.Children) != 0 {
		t.Errorf("file node = %+v: a source is chosen from a read-only storage too, and a file opens nothing", *node)
	}
	sheet = open(node.ID)
	if sheet.Tree.Chosen != node.ID {
		t.Errorf("chosen = %q, want %q", sheet.Tree.Chosen, node.ID)
	}
	return sheet
}

// S-20: a file that already lies in Files follows the rule of an upload. A
// small one is read and written into the record exactly as an uploaded one
// is; a larger one is not copied — the record links to it where it lies, in
// the form a large upload's link has. The storage may be read-only: it is
// only read.
func TestAttachFromFilesFollowsTheRule(t *testing.T) {
	f := attachFixture(t, 1024, []string{"carrel"},
		"e1.ics", attachEvent("e1", "Sync-up", ""))
	f.srv.SeedFile(attachFiles+"notes.txt", "small file")
	big := strings.Repeat("x", 2048)
	f.srv.SeedFile(attachFiles+"sub/big report.pdf", big)
	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		for i := range b.Collections {
			if b.Collections[i].Path == attachFiles {
				b.Collections[i].ReadOnly = true
			}
		}
	})
	rec := f.path("calendar") + "/e1"

	attach := actionByID(eventAt(t, f, "e1").Actions, "attach.add")
	sheet := fromFiles(t, f, attach.Arrow, "", "notes.txt")
	confirm := actionByID(sheet.Actions, "files.picker.attach")
	if confirm == nil || !confirm.Enabled || confirm.Method != "POST" {
		t.Fatalf("Attach = %+v", confirm)
	}
	resp := f.h.do(postForm(confirm.Href, f.sess, url.Values{}))
	if resp.Code != http.StatusSeeOther || resp.Header().Get("Location") != rec {
		t.Fatalf("attach from Files: %d %s %s", resp.Code, resp.Header().Get("Location"), resp.Body.String())
	}
	mustContain(t, body(t, f, "e1.ics"), "ATTACH;ENCODING=BASE64;", "FILENAME=notes.txt", "VALUE=BINARY:"+
		base64.StdEncoding.EncodeToString([]byte("small file")))

	page := eventAt(t, f, "e1")
	attach = actionByID(page.Actions, "attach.add")
	sheet = fromFiles(t, f, attach.Arrow, "sub/", "big report.pdf")
	resp = f.h.do(postForm(actionByID(sheet.Actions, "files.picker.attach").Href, f.sess, url.Values{}))
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("attach a large file from Files: %d %s", resp.Code, resp.Body.String())
	}
	doc := body(t, f, "e1.ics")
	mustContain(t, doc, "FILENAME=big report.pdf", "SIZE=2048", ":"+f.srv.URL+attachFiles+"sub/big%20report.pdf")
	if strings.Contains(doc, base64.StdEncoding.EncodeToString([]byte(big[:48]))) {
		t.Errorf("the large file went into the record:\n%s", doc)
	}
	if _, ok := f.srv.File(attachFiles + "carrel/2026-08-10-big report.pdf"); ok {
		t.Errorf("the large file was copied into the attachment folder")
	}
	page = eventAt(t, f, "e1")
	if len(page.Attach) != 2 {
		t.Fatalf("attachments = %+v", page.Attach)
	}
	row := page.Attach[1]
	if row.Inline || row.Name != "big report.pdf" || row.Href == "" {
		t.Fatalf("link row = %+v", row)
	}
	// The link opens the file in Files (task 93); it is removed on the edit.
	if got := f.h.do(get(row.Href, f.sess)); !strings.HasPrefix(row.Href, "/app/files/") || got.Code != http.StatusOK {
		t.Errorf("open the link %q: %d", row.Href, got.Code)
	}
	if edit := eventEditAt(t, f, "e1").Attach; len(edit) != 2 || len(edit[1].Actions) != 1 || edit[1].Actions[0].Label != "Remove link, keep the file" {
		t.Errorf("removing it = %+v", edit)
	}
}

// The file is attached with If-Match of the version the page showed, as an
// upload is: a record changed since writes nothing.
func TestAttachFromFilesOnAChangedRecordWritesNothing(t *testing.T) {
	f := attachFixture(t, 0, nil, "e1.ics", attachEvent("e1", "Sync-up", ""))
	f.srv.SeedFile(attachFiles+"notes.txt", "small file")
	sheet := fromFiles(t, f, actionByID(eventAt(t, f, "e1").Actions, "attach.add").Arrow, "", "notes.txt")
	f.srv.Seed(f.col+"e1.ics", attachEvent("e1", "Sync-up moved", ""))

	page := decode[view.EventPage](t, f.h.do(postForm(actionByID(sheet.Actions, "files.picker.attach").Href, f.sess, url.Values{})))
	if len(page.Notices) == 0 || !strings.HasPrefix(string(page.Notices[0].Title), "Changed elsewhere") {
		t.Errorf("notices = %+v", page.Notices)
	}
	if doc := body(t, f, "e1.ics"); strings.Contains(doc, "ATTACH") {
		t.Errorf("attached over a newer version:\n%s", doc)
	}
}

// The tree of an upload chooses folders only; files are shown only in the
// tree that chooses one.
func TestUploadTreeShowsNoFiles(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"report.pdf", "0123456789")
	f.srv.SeedFolder(filesDocs + "inbox/")
	sheet := decode[view.FilePickerSheet](t, f.h.do(get("/app/files/picker?for=upload&to="+url.QueryEscape(f.at("")), f.sess)))
	if len(sheet.Tree.Nodes) == 0 || len(sheet.Tree.Nodes[0].Children) != 1 || sheet.Tree.Nodes[0].Children[0].File {
		t.Errorf("upload tree = %+v", sheet.Tree.Nodes)
	}
}
