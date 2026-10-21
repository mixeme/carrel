package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// The client's scan of the files section: the queue (s-files-2, ФВ2, ФВ3),
// the sheets' trees (ФГ4–ФГ6) and the published links (s-publish-1, ФА1,
// ФД1–ФД3), as their frames draw them.

// ФВ2, ФВ3: the queue's head says where the upload is and what it weighs;
// a taken name is answered Keep both · Replace · Skip.
func TestQueueIsTheFrame(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"taken.txt", "old")
	f.h.do(uploadForm(f.at("upload"), f.sess, false, upload{"new.txt", strings.Repeat("n", 3000)}, upload{"taken.txt", "again"}))
	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	q := page.Queue
	if q == nil || q.Title != "Uploading 2 of 2" || q.Total != "2.9 KB" {
		t.Fatalf("queue head = %+v", q)
	}
	ask := queueRow(t, q, "taken.txt")
	var labels []string
	for _, o := range ask.Question.Options {
		labels = append(labels, string(o.Label))
		if o.Danger {
			t.Errorf("%q is drawn as an ordinary button: %+v", o.Label, o)
		}
	}
	if strings.Join(labels, ",") != "Keep both,Replace,Skip" {
		t.Errorf("answers = %v", labels)
	}
	f.h.do(answerUpload(t, ask, "transfer.both", f.sess, upload{"taken.txt", "mine"}))
	page = decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	// What was shown finished goes; the answered row stays until shown.
	if page.Queue == nil || page.Queue.Title != "Uploaded 1 of 1" {
		t.Errorf("after the answer = %+v", page.Queue)
	}
}

// ФГ4: New folder of a tree is a link that opens the line of the name;
// without it the sheet has no line. ФГ5, ФГ6: the upload picker names the
// chosen folder by its path; a storage's node names its connection; the
// attachment folder and a folder just made are marked.
func TestTreeLineAndMarks(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"a.txt", "alpha")
	f.srv.SeedFolder(filesDocs + "carrel/")
	carrel := f.at("", "carrel", "")
	if rec := f.h.do(postForm("/app/settings/attachments", f.sess, attachAt(t, carrel))); rec.Code != 303 {
		t.Fatalf("attachment folder: %d %s", rec.Code, rec.Body)
	}
	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	move := opSheet(t, f, rowMenu(t, fileRow(t, page, "a.txt"), "file.move"), url.Values{"to": {carrel}})
	tree := move.Target
	if tree.Name != nil || tree.New == nil || tree.New.Method != "GET" || !strings.Contains(tree.New.Href, "new=1") {
		t.Fatalf("New = %+v, line %+v", tree.New, tree.Name)
	}
	opened := decode[view.FileOpSheet](t, f.h.do(get(tree.New.Href, f.sess)))
	if opened.Target.Name == nil || opened.Target.Name.Method != "POST" {
		t.Fatalf("the line did not open: %+v", opened.Target)
	}
	// The server is the head's (ФН9): the root does not name it.
	root := opened.Target.Nodes[0]
	if root.Stamp != "" || len(opened.Servers) != 1 || !opened.Servers[0].On || strings.Contains(string(opened.Servers[0].Label), "Docs") {
		t.Errorf("root stamp %q, servers %+v", root.Stamp, opened.Servers)
	}
	if !root.Children[0].Attachments {
		t.Errorf("the attachment folder is not marked: %+v", root.Children[0])
	}

	picker := decode[view.FilePickerSheet](t, f.h.do(get("/app/files/picker?"+url.Values{"for": {"upload"}, "to": {carrel}}.Encode(), f.sess)))
	if picker.Where != "Docs / carrel" || picker.Tree.Name != nil {
		t.Errorf("picker where %q, line %+v", picker.Where, picker.Tree.Name)
	}
	line := decode[view.FilePickerSheet](t, f.h.do(get(picker.Tree.New.Href, f.sess)))
	if line.Tree.Name == nil {
		t.Fatalf("the picker's line did not open: %+v", line.Tree)
	}
	made := decode[view.FilePickerSheet](t, f.h.do(postForm(line.Tree.Name.Action, f.sess, url.Values{"name": {"2026-08"}})))
	if made.Where != "Docs / carrel / 2026-08" || made.Tree.Name != nil {
		t.Fatalf("made: where %q, line %+v", made.Where, made.Tree.Name)
	}
	fresh := made.Tree.Nodes[0].Children[0].Children[0]
	if fresh.Label != "2026-08" || !fresh.Fresh {
		t.Errorf("the folder just made = %+v", fresh)
	}
}

// ФА1, ФД1–ФД3: the published links refresh; the line under the title says
// what a link is; a row's kind is a word, a file and a folder have their
// icons.
func TestPublishedIsTheFrame(t *testing.T) {
	f := pubDocs(t)
	publishFile(t, f, "share", true)
	publishFile(t, f, "secret.txt", false)
	page := decode[view.PublishedLinksPage](t, f.h.do(get("/app/files/published", f.sess)))
	if a := page.Top.Refresh.Action; a.ID != "published.refresh" || a.Href != "/app/files/published" {
		t.Errorf("refresh = %+v", a)
	}
	if !strings.HasPrefix(plainText(page.Subtitle), "Read-only, one object or one collection per link") {
		t.Errorf("subtitle = %+v", page.Subtitle)
	}
	// ФА4: the settings rail the list stands in has its icons.
	icons := map[string]string{}
	for _, n := range page.Rail.Nav {
		icons[n.ID] = n.Icon
	}
	if icons["settings.account"] != "i-user" || icons["settings.published"] != "i-link" || icons["settings.appearance"] != "i-theme" {
		t.Errorf("settings rail icons = %v", icons)
	}
	kinds := map[string]view.PublishedRow{}
	for _, r := range page.Rows {
		kinds[string(r.Kind)] = r
	}
	if r, ok := kinds["folder"]; !ok || r.Icon != "i-file" {
		t.Errorf("folder row = %+v", r)
	}
	if r, ok := kinds["file"]; !ok || r.Icon != "i-doc" {
		t.Errorf("file row = %+v", r)
	}
}
