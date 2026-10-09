package web_test

import (
	"image"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// The client's scan of the files section (client.md, «Раздел файлов:
// пробелы контракта»): what the frames of the root (s-files-6) and of a
// folder (s-files-1, -2, -5, -7) draw that the view model did not carry.

// ФА1, ФА2, ФА3, ФБ1…ФБ4: the root — its refresh, its rail row open with
// «1 of 2», the line under the title, the server of each storage, the
// message naming what is not ticked; empty, a button that ticks them all.
func TestFilesRootIsTheFrame(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFolder(filesDocs)
	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		b.Collections = append(b.Collections, sealed.Collection{AccountID: f.id, Path: davFilesOld, Name: "Old", Kinds: []view.Kind{view.KindFile}})
	})
	root := decode[view.FilesRootPage](t, f.h.do(get("/app/files", f.sess)))
	if a := root.Top.Refresh.Action; a.ID != "files.refresh" || a.Href != "/app/files" || !a.Enabled {
		t.Errorf("refresh = %+v", a)
	}
	if root.Subtitle != "1 of 2 storages on 1 server" {
		t.Errorf("subtitle = %q", root.Subtitle)
	}
	all := root.Rail.Sections[0].Rows[0]
	if !all.On || all.Meta != "1 of 2" {
		t.Errorf("root row = %+v", all)
	}
	if len(root.Collections) != 1 || root.Collections[0].Server == "" || strings.Contains(string(root.Collections[0].Server), "Docs") {
		t.Errorf("rows = %+v", root.Collections)
	}
	var strong []string
	for _, r := range root.Note {
		if r.Mark == view.RunStrong {
			strong = append(strong, string(r.Text))
		}
	}
	if strings.Join(strong, ",") != "Old" || !strings.Contains(plainText(root.Note), "is not ticked, so it is not here") {
		t.Errorf("note = %+v", root.Note)
	}

	// A folder read says when; its storage is the open row of the rail.
	page := decode[view.FilesPage](t, f.h.do(get(f.at(""), f.sess)))
	if page.Top.Refresh.Answered == "" {
		t.Errorf("folder refresh = %+v", page.Top.Refresh)
	}
	if _, row := railRow(t, page.Rail, "Docs"); !row.On {
		t.Errorf("the storage of the folder is not open in the rail: %+v", row)
	}
	if root = decode[view.FilesRootPage](t, f.h.do(get("/app/files", f.sess))); root.Top.Refresh.Answered == "" {
		t.Errorf("root refresh after a folder was read = %+v", root.Top.Refresh)
	}

	none := filesFixture(t, false)
	empty := decode[view.FilesRootPage](t, none.h.do(get("/app/files", none.sess)))
	if empty.Empty == nil || actionByID(empty.Empty.Actions, "rail.tick.all") == nil {
		t.Fatalf("nothing ticked: %+v", empty.Empty)
	}
}

const davFilesOld = davtest.FileRoot + "old/"

// ФБ6, ФБ7: one file chosen opens its panel (s-files-1): preview, Download,
// Open, Rename, Move, the fields, Delete; Details toggles it, Print is in
// «⋯» of the head (s-files-7).
func TestFolderPanelIsTheFrame(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"a.png", string(pngOf(t, image.NewRGBA(image.Rect(0, 0, 4, 4)))))
	f.srv.SeedFile(filesDocs+"b.txt", "beta")
	here := f.at("")
	page := decode[view.FilesPage](t, f.h.do(get(here+"?sort=size&select=a.png", f.sess)))
	d := page.Details
	if d == nil {
		t.Fatal("one file chosen and no panel")
	}
	if d.Title != "a.png" || !strings.HasSuffix(string(d.Subtitle), " · Docs") || d.Close != here+"?sort=size" {
		t.Errorf("panel head = %q %q %q", d.Title, d.Subtitle, d.Close)
	}
	if d.Preview == nil || !strings.HasPrefix(d.Preview.Href, f.at("download", "a.png")+"?thumb=1") || d.Preview.Href != fileRow(t, page, "a.png").Thumb || d.Preview.Icon != "i-img" {
		t.Errorf("preview = %+v", d.Preview)
	}
	var ids []string
	for _, a := range d.Actions {
		ids = append(ids, a.ID)
	}
	if strings.Join(ids, ",") != "file.download,file.view,file.rename,file.move,file.delete" || !d.Actions[0].Primary || !d.Actions[4].Danger {
		t.Errorf("panel actions = %v", ids)
	}
	var terms []string
	for _, fl := range d.Fields {
		terms = append(terms, string(fl.Label))
		if !fl.Mono {
			t.Errorf("field %q is not monospaced", fl.Label)
		}
	}
	// The image's head says its size (ФБ11, TestChosenImageSaysItsSize).
	if strings.Join(terms, ",") != "Size,Type,Dimensions,Changed,Path" || d.Fields[2].Value != "4×4" || d.Fields[4].Value != view.Text(filesDocs+"a.png") {
		t.Errorf("fields = %+v", d.Fields)
	}
	toggle := actionByID(page.Bar.Actions, "files.details")
	if toggle == nil || !toggle.On || toggle.Icon != "i-panel" {
		t.Fatalf("Details = %+v", toggle)
	}
	if p := actionByID(page.Bar.Actions, "list.print"); p == nil || p.Href != here+"?print=1" {
		t.Errorf("Print = %+v", p)
	}
	hidden := decode[view.FilesPage](t, f.h.do(get(toggle.Href, f.sess)))
	if hidden.Details != nil || !fileRow(t, hidden, "a.png").Selected {
		t.Errorf("Details off: panel %+v", hidden.Details)
	}
	if back := actionByID(hidden.Bar.Actions, "files.details"); back == nil || back.On || !back.Enabled {
		t.Errorf("Details back on = %+v", back)
	} else if again := decode[view.FilesPage](t, f.h.do(get(back.Href, f.sess))); again.Details == nil {
		t.Error("Details did not bring the panel back")
	}
	two := decode[view.FilesPage](t, f.h.do(get(here+"?select=a.png&select=b.txt", f.sess)))
	if two.Details != nil {
		t.Errorf("two chosen, a panel: %+v", two.Details)
	}
	if a := actionByID(two.Bar.Actions, "files.details"); a == nil || a.Enabled || a.Reason == "" {
		t.Errorf("Details with two = %+v", a)
	}
	if txt := fileRow(t, two, "b.txt"); txt.Icon != "i-doc" {
		t.Errorf("b.txt icon %q", txt.Icon)
	}
}

// ФБ8, ФБ10, ФБ11, ФБ15: the bar's filter without scripts, the size of the
// selection and of the folder, the icon of a row and why an image has no
// thumbnail, the view switch's icons.
func TestFolderBarIsTheFrame(t *testing.T) {
	f := filesFixture(t, true)
	f.srv.SeedFile(filesDocs+"big.jpg", "j")
	f.srv.Grow(filesDocs+"big.jpg", 17<<20) // over the thumbnail's ceiling
	f.srv.SeedFile(filesDocs+"note.md", strings.Repeat("n", 2048))
	f.srv.SeedFolder(filesDocs + "sub/")
	here := f.at("")
	page := decode[view.FilesPage](t, f.h.do(get(here+"?view=tiles&select=note.md", f.sess)))
	ff := page.Bar.FilterForm
	if ff == nil || ff.Method != "GET" || ff.Action != here || formRow(*ff, "filter") == nil || formRow(*ff, "view") == nil || formRow(*ff, "view").Kind != view.InputHidden {
		t.Errorf("filter form = %+v", ff)
	}
	if page.Bar.Size != "17 MB" || page.Bar.SelectedSize != "2 KB" {
		t.Errorf("sizes: folder %q, selected %q", page.Bar.Size, page.Bar.SelectedSize)
	}
	big, sub := fileRow(t, page, "big.jpg"), fileRow(t, page, "sub")
	if big.Icon != "i-img" || big.Note != "too large" || big.Thumb != "" || sub.Icon != "i-file" || sub.Note != "" {
		t.Errorf("big %+v, sub %+v", big, sub)
	}
	icons := map[string]string{}
	for _, a := range page.Bar.Segments {
		icons[a.ID] = a.Icon
	}
	if icons["files.view.list"] != "i-menu" || icons["files.view.tiles"] != "i-grid" {
		t.Errorf("segments = %+v", page.Bar.Segments)
	}
}
