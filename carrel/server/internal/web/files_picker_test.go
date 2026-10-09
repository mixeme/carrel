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
	if sheet.Tree.Chosen != carrel || !strings.HasSuffix(sheet.Actions[0].Href, "/upload/carrel/?menu=1") {
		t.Fatalf("default = %q, upload = %+v", sheet.Tree.Chosen, sheet.Actions[0])
	}

	// The sheet has no «Choose this folder» (ФН14, s-notes-11): a folder is
	// chosen by its node, and it is the upload into it that remembers it —
	// choosing alone remembers nothing.
	if ch := actionByID(sheet.Actions, "files.picker.choose"); ch != nil {
		t.Fatalf("the upload sheet offers %+v, which no frame draws", ch)
	}
	inbox := "/app/files/" + f.id + "/" + url.PathEscape(filesDocs) + "/inbox/"
	chosen := decode[view.FilePickerSheet](t, f.h.do(get("/app/files/picker?"+url.Values{"for": {"upload"}, "to": {inbox}}.Encode(), f.sess)))
	if chosen.Tree.Chosen != inbox {
		t.Fatalf("chosen = %q", chosen.Tree.Chosen)
	}
	if again := decode[view.FilePickerSheet](t, f.h.do(get("/app/files/picker?for=upload", f.sess))); again.Tree.Chosen != carrel {
		t.Fatalf("a folder only looked at is remembered: %q", again.Tree.Chosen)
	}
	rec := f.h.do(uploadForm(chosen.Actions[0].Href, f.sess, false, upload{"note.txt", "hello"}))
	if rec.Code >= 400 {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body)
	}
	if got, ok := f.srv.File(filesDocs + "inbox/note.txt"); !ok || got != "hello" {
		t.Errorf("uploaded = %q %v", got, ok)
	}
	sheet = decode[view.FilePickerSheet](t, f.h.do(get("/app/files/picker?for=upload", f.sess)))
	if sheet.Tree.Chosen != inbox {
		t.Fatalf("after the upload the menu's folder is %q, want the one uploaded into", sheet.Tree.Chosen)
	}
	// An upload of the folder's own page remembers nothing for the menu.
	f.h.do(uploadForm(f.at("upload", "carrel", ""), f.sess, false, upload{"other.txt", "x"}))
	if sheet = decode[view.FilePickerSheet](t, f.h.do(get("/app/files/picker?for=upload", f.sess))); sheet.Tree.Chosen != inbox {
		t.Fatalf("an upload from a folder's page moved the menu's folder to %q", sheet.Tree.Chosen)
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
