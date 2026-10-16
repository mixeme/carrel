package web_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/view"
)

// s-io-4: the folder of notes is chosen in a tree — the storages, and the
// folders of the one opened, level by level («Documents › notes») —, the
// tree of Move and of the quick upload; «Preview folder» takes the folder
// chosen, its own .md files and not its subfolders'.
func TestImportNotesFolderIsChosenInATree(t *testing.T) {
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	notebook := davtest.CalendarHome + "notes/"
	srv.AddCalendar(caldav.Calendar{Path: notebook, Name: "Notebook", SupportedComponentSet: []string{"VJOURNAL"}})
	srv.SeedFolder(davtest.FileRoot + "docs/")
	srv.SeedFile(davtest.FileRoot+"docs/top.md", "# Сверху\n")
	srv.SeedFolder(davtest.FileRoot + "docs/notes/")
	srv.SeedFile(davtest.FileRoot+"docs/notes/kovka.md", "# Ковка\n")
	srv.SeedFolder(davtest.FileRoot + "docs/notes/old/")
	srv.SeedFile(davtest.FileRoot+"docs/notes/old/deep.md", "# Глубоко\n")
	srv.SeedFolder(davtest.FileRoot + "docs/carrel/")
	f := connectFixture(t, srv, notebook, nil)
	addFileCollection(t, f, davtest.FileRoot+"docs/", "Docs")

	page := decode[view.ImportPage](t, f.h.do(get(f.path("notes")+"/import", f.sess)))
	tree := page.Source.Folder
	if tree == nil || len(tree.Nodes) != 1 || tree.Nodes[0].Label != "Docs" || tree.Nodes[0].Expanded || tree.Chosen != "" {
		t.Fatalf("the tree before a choice = %+v", tree)
	}
	page = decode[view.ImportPage](t, f.h.do(get(tree.Nodes[0].ID, f.sess)))
	root := page.Source.Folder.Nodes[0]
	var names []string
	for _, n := range root.Children {
		names = append(names, n.Label)
	}
	if !root.Expanded || strings.Join(names, " ") != "carrel notes" || page.Source.Folder.Chosen != root.ID {
		t.Fatalf("Docs opened: %+v", root)
	}
	notes := root.Children[1]
	page = decode[view.ImportPage](t, f.h.do(get(notes.ID, f.sess)))
	tree = page.Source.Folder
	if tree.Chosen != notes.ID || !tree.Nodes[0].Children[1].Expanded || len(tree.Nodes[0].Children[1].Children) != 1 || tree.Nodes[0].Children[1].Children[0].Label != "old" {
		t.Fatalf("notes chosen: %+v", tree)
	}
	page = decode[view.ImportPage](t, f.h.do(postForm(page.Source.Preview.Href, f.sess, url.Values{})))
	if page.Step != view.ImportStepPreview {
		t.Fatalf("step %q, notices %+v", page.Step, page.Notices)
	}
	if got := saidRuns(page.Subtitle); !strings.HasPrefix(got, "⟨mono:Docs › notes⟩ · ") {
		t.Errorf("subtitle = %q", got)
	}
	if len(page.Preview) != 1 || len(page.Preview[0].Lines) != 1 || page.Preview[0].Lines[0].Cells[0] != "kovka.md" {
		t.Errorf("the preview of notes/ = %+v", page.Preview)
	}

	// A path that leaves the storage is no folder of it.
	bad := strings.Replace(notes.ID, "in=notes", "in="+url.QueryEscape("../x"), 1)
	if got := decode[view.ImportPage](t, f.h.do(get(bad, f.sess))); got.Source.Folder.Chosen != "" {
		t.Errorf("a path out of the storage was chosen: %q", got.Source.Folder.Chosen)
	}
}
