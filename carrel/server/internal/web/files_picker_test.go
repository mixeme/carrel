package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// "Upload file" of the create menu (frame «Пункт Upload file — куда
// именно»): the sheet asks where, the attachment folder first; a folder
// chosen in the tree is remembered for the next upload from the menu, and
// Upload goes to the ordinary upload of that folder.
func TestUploadFromTheMenuAsksWhere(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFolder(filesDocs + "inbox/")
	f.srv.SeedFolder(filesDocs + "carrel/")

	sheet := decode[view.FilePickerSheet](t, f.h.do(get("/app/files/picker?for=upload", f.sess)))
	if up := sheet.Actions[0]; up.ID != "files.picker.upload" || up.Enabled {
		t.Fatalf("no folder yet, upload = %+v", up)
	}

	carrel := "/app/files/" + f.id + "/" + url.PathEscape(filesDocs) + "/carrel/"
	if rec := f.h.do(postForm("/app/settings/attachments", f.sess, attachAt(t, carrel))); rec.Code != 303 {
		t.Fatalf("attachment folder: %d %s", rec.Code, rec.Body)
	}
	sheet = decode[view.FilePickerSheet](t, f.h.do(get("/app/files/picker?for=upload", f.sess)))
	if sheet.Tree.Chosen != carrel || !strings.HasSuffix(sheet.Actions[0].Href, "/upload/carrel/") {
		t.Fatalf("default = %q, upload = %+v", sheet.Tree.Chosen, sheet.Actions[0])
	}

	inbox := "/app/files/" + f.id + "/" + url.PathEscape(filesDocs) + "/inbox/"
	f.h.do(postForm(actionHref(t, sheet.Actions, "files.picker.choose"), f.sess, url.Values{"to": {inbox}}))
	sheet = decode[view.FilePickerSheet](t, f.h.do(get("/app/files/picker?for=upload", f.sess)))
	if sheet.Tree.Chosen != inbox {
		t.Fatalf("remembered = %q", sheet.Tree.Chosen)
	}
	rec := f.h.do(uploadForm(sheet.Actions[0].Href, f.sess, false, upload{"note.txt", "hello"}))
	if rec.Code >= 400 {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body)
	}
	if got, ok := f.srv.File(filesDocs + "inbox/note.txt"); !ok || got != "hello" {
		t.Errorf("uploaded = %q %v", got, ok)
	}

	page := decode[view.AccountPage](t, f.h.do(get("/app/settings/account", f.sess)))
	found := false
	for _, m := range page.Top.New.Arrow {
		if m.ID == "new.upload" && m.Separator && strings.Contains(m.Href, "/app/files/picker?for=upload") {
			found = true
		}
	}
	if !found {
		t.Errorf("menu = %+v", page.Top.New.Arrow)
	}
}
