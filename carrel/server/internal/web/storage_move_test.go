package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// Р-74, s-profile-35 (Typecase's note 112 p. 8): the whole storage — the
// address the connection is made with — moves too. What it holds is copied
// into a folder of its name on the other server, then deleted a member at a
// time; the address stays, empty. What the server will not let go stays,
// copied too, and the result names it.
func TestAWholeStorageMovesWhatItHolds(t *testing.T) {
	f := calendarFixture(t, nil)
	vps, vpsID := fileStorage(t, f, "vps", "Docs", "Public")
	nas, nasID := fileStorage(t, f, "nas")
	nas.SeedFile(davtest.FileRoot+"a.txt", "alpha")
	nas.SeedFile(davtest.FileRoot+"photos/b.txt", "beta")
	// The connection is made with the files' folder: that address is the
	// storage, and the storage is its one collection.
	editBlob(t, f.h, f.sess, func(blob *sealed.Blob) {
		for i := range blob.Connections {
			if blob.Connections[i].ID == nasID {
				blob.Connections[i].FileRoot = nas.URL + davtest.FileRoot
			}
		}
		blob.Collections = append(blob.Collections, sealed.Collection{AccountID: nasID, Path: davtest.FileRoot, Name: "Archive", Kinds: []view.Kind{view.KindFile}})
	})
	q := url.Values{"a": {nasID}, "c": {davtest.FileRoot}, "to": {vpsID}}
	sheet := decode[view.CollectionMoveSheet](t, f.h.do(get("/app/collections/move?"+q.Encode(), f.sess)))
	words := saidRuns(sheet.Warning)
	if !strings.HasPrefix(words, "Carrel makes a folder «Archive» on vps beside Docs and Public, copies everything in nas · Archive into it") ||
		!strings.HasSuffix(words, "The storage itself stays on nas, empty: it is the address this connection is made with.") {
		t.Errorf("message = %q", words)
	}
	if sheet.Stays != "What nas will not let delete — folders shared with you, folders its administrator mounted — is copied and stays there too; the result names it." {
		t.Errorf("second line = %q", sheet.Stays)
	}
	if c := sheet.Actions[0]; !c.Enabled {
		t.Fatalf("Move = %+v", c)
	}

	nas.RefuseAt(http.MethodDelete, davtest.FileRoot+"photos", http.StatusForbidden, "")
	page := decode[view.ConnectionsPage](t, f.h.do(postForm("/app/collections/move?"+url.Values{"a": {nasID}, "c": {davtest.FileRoot}}.Encode(), f.sess, url.Values{"to": {vpsID}})))
	if a, _ := vps.File(davtest.FileRoot + "Archive/a.txt"); a != "alpha" {
		t.Errorf("vps Archive/a.txt = %q, there: %v", a, vps.Names(davtest.FileRoot))
	}
	if b, _ := vps.File(davtest.FileRoot + "Archive/photos/b.txt"); b != "beta" {
		t.Errorf("vps Archive/photos/b.txt = %q", b)
	}
	if _, ok := nas.File(davtest.FileRoot + "a.txt"); ok {
		t.Error("what was copied stayed on nas")
	}
	if b, _ := nas.File(davtest.FileRoot + "photos/b.txt"); b != "beta" {
		t.Errorf("the folder nas would not let go: %q", b)
	}
	if len(page.Notices) != 1 || page.Notices[0].Title != "Moved, but some of it stayed" || page.Notices[0].Details == nil ||
		len(page.Notices[0].Details.Lines) != 1 || !strings.HasPrefix(string(page.Notices[0].Details.Lines[0]), "photos · 403") {
		t.Errorf("notices = %+v", page.Notices)
	}
	// The storage stays in the catalog, and the folder joins it on vps.
	blob := catalogOf(t, f)
	if _, ok := blob.Collection(nasID, davtest.FileRoot); !ok {
		t.Error("the storage left the catalog")
	}
	if _, ok := blob.Collection(vpsID, davtest.FileRoot+"Archive/"); !ok {
		t.Errorf("the folder is not in the catalog: %+v", blob.CollectionsOf(vpsID))
	}
}
