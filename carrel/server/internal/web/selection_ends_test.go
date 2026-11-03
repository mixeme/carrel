package web_test

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Every selection bar ends with Select all and Clear selection (Typecase's
// note 160, by the owner's remarks of 02.11.2026): a folder's selection by
// the address has both, Select all the folder with every row it shows
// selected — its filter and order kept —, Clear selection the folder with
// none.
func TestFolderSelectionEndsWithSelectAllAndClear(t *testing.T) {
	f := filesFixture(t, true)
	for _, name := range []string{"a.txt", "b.txt", "c.md"} {
		f.srv.SeedFile(filesDocs+name, name)
	}
	f.srv.SeedFolder(filesDocs + "sub/")
	here := f.at("")

	ends := func(query string) (all, clear url.Values) {
		t.Helper()
		page := decode[view.FilesPage](t, f.h.do(get(here+"?"+query, f.sess)))
		var ids []string
		for _, a := range page.Select.Actions {
			ids = append(ids, a.ID)
		}
		if !slices.Equal(ids, []string{"files.select.all", "files.select.clear"}) {
			t.Fatalf("%s: the selection's actions = %q, want Select all, Clear selection", query, ids)
		}
		read := func(id, label string) url.Values {
			a := actionByID(page.Select.Actions, id)
			if string(a.Label) != label || a.Method != "GET" || !a.Enabled {
				t.Errorf("%s: %s = %+v", query, id, a)
			}
			path, q, _ := strings.Cut(a.Href, "?")
			u, err := url.ParseQuery(q)
			if err != nil || path != here {
				t.Fatalf("%s: %s goes to %q, want the folder", query, id, a.Href)
			}
			return u
		}
		return read("files.select.all", "Select all"), read("files.select.clear", "Clear selection")
	}

	all, clear := ends("sort=size&select=b.txt")
	got := all["select"]
	slices.Sort(got)
	if !slices.Equal(got, []string{"a.txt", "b.txt", "c.md", "sub"}) || all.Get("sort") != "size" {
		t.Errorf("Select all = %v, want every row selected, the order kept", all)
	}
	if clear.Has("select") || clear.Get("sort") != "size" {
		t.Errorf("Clear selection = %v", clear)
	}
	// What the filter hides is not selected.
	all, _ = ends("filter=txt&select=a.txt")
	got = all["select"]
	slices.Sort(got)
	if !slices.Equal(got, []string{"a.txt", "b.txt"}) || all.Get("filter") != "txt" {
		t.Errorf("Select all under a filter = %v", all)
	}
	// The address of Select all selects them all.
	page := decode[view.FilesPage](t, f.h.do(get(here+"?"+all.Encode(), f.sess)))
	if page.Bar.Selected != 2 {
		t.Errorf("after Select all: %d selected", page.Bar.Selected)
	}
}

// The bar of the restore's matching ends with Select all and Clear
// selection (s-backup-8, -20): Clear selection unticks every row and leaves
// where each goes as it was.
func TestRestoreBarEndsWithSelectAllAndClear(t *testing.T) {
	f := backupFixture(t)
	m := openArchive(t, f, nil)
	var ids []string
	for _, a := range m.Form.Actions {
		ids = append(ids, a.ID)
	}
	if !slices.Equal(ids[len(ids)-2:], []string{"restore.all", "restore.none"}) {
		t.Fatalf("the bar's actions = %q, want Select all, Clear selection at its end", ids)
	}
	none := actionByID(m.Form.Actions, "restore.none")
	if none.Label != "Clear selection" || none.Method != "POST" || !none.Enabled {
		t.Errorf("Clear selection = %+v", none)
	}
	rec := f.h.do(postForm(none.Href, f.sess, matchForm(m)))
	if rec.Code == http.StatusSeeOther {
		rec = f.h.do(get(rec.Header().Get("Location"), f.sess))
	}
	got := *decode[view.BackupPage](t, rec).Restore.Match
	if got.Selected != 0 {
		t.Errorf("after Clear selection: %d selected", got.Selected)
	}
	for i, r := range got.Rows {
		if r.Pick.Value == "true" || firstSelected(r.Into) != firstSelected(m.Rows[i].Into) {
			t.Errorf("row %d after Clear selection: ticked %q, into %q, was %q", i, r.Pick.Value, firstSelected(r.Into), firstSelected(m.Rows[i].Into))
		}
	}
	if run := actionByID(got.Form.Actions, "restore.run"); run == nil || run.Enabled {
		t.Errorf("Restore with nothing ticked = %+v", run)
	}
}
