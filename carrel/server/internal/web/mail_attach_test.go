package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Batch 4 of the publish-mail scan (С24–С28; s-mail-5, -6, -12, -13) and
// Typecase's note 110 p. 1: an attach sheet names what it attaches; a
// record is searched for under «All …» by a GET form of its own, the one
// chosen on and «All» kept; Attach stands always, off with why until one is
// chosen; a file says before it is chosen whether it goes as a file or a
// link, the ceiling named; a folder's sheet says what is in it, its files
// off, and that a folder always goes as a link.
func TestAttachSheetsAreTheirFrames(t *testing.T) {
	f, _ := letterFixture(t)
	sheet := func(q url.Values) view.MailAttachSheet {
		t.Helper()
		return decode[view.MailAttachSheet](t, f.h.do(get("/app/mail/attach?"+q.Encode(), f.sess)))
	}

	all := sheet(url.Values{"what": {"contact"}})
	if all.Title != "Attach a contact" || all.What != "contact" || all.All == nil || !all.All.On || all.All.Label != "All address books" {
		t.Fatalf("contact sheet = %q %q, all %+v", all.Title, all.What, all.All)
	}
	attach := actionByID(all.Actions, "mail.attach.add")
	if attach == nil || attach.Enabled || attach.Reason != "Choose a contact." {
		t.Errorf("Attach before a choice = %+v", attach)
	}
	if all.Search == nil || all.Search.Method != "GET" || formRow(*all.Search, "q") == nil || formFields(*all.Search).Get("what") != "contact" ||
		formFields(*all.Search).Has("a") {
		t.Errorf("search form = %+v", all.Search)
	}
	if len(all.Rows) != 1 || !strings.Contains(all.Rows[0].Href, "all=1") {
		t.Fatalf("rows = %+v", all.Rows)
	}
	chosen := decode[view.MailAttachSheet](t, f.h.do(get(all.Rows[0].Href, f.sess)))
	if !chosen.All.On || len(chosen.Rows) != 1 || !chosen.Rows[0].On || len(chosen.Preview) != 1 {
		t.Errorf("chosen under All: all %+v, rows %+v, preview %+v", chosen.All, chosen.Rows, chosen.Preview)
	}
	if a := actionByID(chosen.Actions, "mail.attach.add"); a == nil || !a.Enabled || !a.Submits || a.Href != chosen.Form.Action {
		t.Errorf("Attach chosen = %+v, the form's %q", a, chosen.Form.Action)
	}

	// A file: what it would go as before it is chosen (attachment 1000 B).
	files := sheet(url.Values{"what": {"file"}, "a": {f.id}, "c": {filesDocs}})
	goes := map[string]string{}
	for _, r := range files.Rows {
		goes[string(r.Title)] = string(r.Goes)
	}
	if files.Title != "Attach a file" || files.Tree == nil || len(files.Crumbs) != 1 || goes["small.txt"] != "as a file" || goes["big.bin"] != "over 1000 B · as a link" {
		t.Errorf("file sheet = %q, tree %v, crumbs %+v, goes %v", files.Title, files.Tree != nil, files.Crumbs, goes)
	}
	big := sheet(url.Values{"what": {"file"}, "a": {f.id}, "c": {filesDocs}, "path": {"big.bin"}})
	if !strings.HasPrefix(runsText(big.Note), "This file is over the limit for an attachment") || !strings.Contains(saidRuns(big.Note), "⟨strong:Published links⟩") {
		t.Errorf("big file's note = %q", saidRuns(big.Note))
	}

	// A folder: what is in it, its files off, always a link.
	folder := sheet(url.Values{"what": {"folder"}, "a": {f.id}, "c": {filesDocs}, "dir": {"share"}})
	if folder.Title != "Attach a folder" || folder.In != "In it · 1 file" || len(folder.Rows) != 1 || !folder.Rows[0].Off || folder.Rows[0].Href != "" ||
		!strings.HasPrefix(runsText(folder.Note), "A folder always goes as a link") {
		t.Errorf("folder sheet = %q, in %q, rows %+v, note %q", folder.Title, folder.In, folder.Rows, runsText(folder.Note))
	}
}
