package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/view"
)

// The owner's decision of 13.11.2026: «Objects» of a file collection is how
// many files it holds at every depth — the frames' «Documents 9», «Public
// 2», «Files 1» (s-profile-1). No server says it, so Carrel walks the tree
// in the background when Connections opens, one folder at a time; while
// the walk goes the row says how many so far and that it is counting; the
// count is kept in the session and walked again when the root's change tag
// moved or on Update collection list; a refusal mid-way leaves a dash and
// says why.

// fileRows are the rows of the file collections of a Connections page, by
// name.
func fileRows(p view.ConnectionsPage) map[string]view.CollectionRow {
	out := map[string]view.CollectionRow{}
	for _, c := range p.Accounts {
		for _, r := range c.Collections {
			for _, k := range r.Kinds {
				if k == view.KindFile {
					out[string(r.Label)] = r
				}
			}
		}
	}
	return out
}

// counted opens Connections until no file collection is being counted.
func counted(t *testing.T, app *mockupApp) view.ConnectionsPage {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		page := decode[view.ConnectionsPage](t, app.h.do(get("/app/settings/connections", app.sess)))
		busy := false
		for _, r := range fileRows(page) {
			busy = busy || r.Counting
		}
		if !busy {
			return page
		}
		if time.Now().After(deadline) {
			t.Fatalf("still counting: %+v", fileRows(page))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// recount presses Update collection list on the connection of slug and
// opens Connections: its storages are walked again.
func recount(t *testing.T, app *mockupApp, slug string) {
	t.Helper()
	if rec := app.h.do(postForm("/app/settings/connections?"+url.Values{"op": {"rediscover"}, "account": {app.realAccount(t, slug)}}.Encode(), app.sess, url.Values{})); rec.Code >= 400 {
		t.Fatalf("update the list: %d", rec.Code)
	}
	app.h.do(get("/app/settings/connections", app.sess))
}

func TestFileCollectionsCountTheirFilesAtEveryDepth(t *testing.T) {
	app := newMockupApp(t)
	app.h.do(get("/app/settings/connections", app.sess))
	rows := fileRows(counted(t, app))
	for name, want := range map[string]int{"Documents": 10, "Public": 2, "Files": 1} {
		r, ok := rows[name]
		if !ok || r.Objects == nil || *r.Objects != want || r.Counting || r.ObjectsNote != "" {
			t.Errorf("%s: %+v, want %d files", name, r, want)
		}
	}

	// Opened again, the root unchanged: the count stands and nothing is
	// walked again — one PROPFIND of the root, no listing of a folder in it.
	sftp := app.servers["sftpgo · vps"]
	inside := app.realPath(t, "Documents") + "carrel/"
	before := sftp.CountRequests("PROPFIND", inside)
	again := fileRows(counted(t, app))
	if n := sftp.CountRequests("PROPFIND", inside) - before; n != 0 {
		t.Errorf("the unchanged storage was walked again: %d listings inside it", n)
	}
	if r := again["Documents"]; r.Objects == nil || *r.Objects != 10 {
		t.Errorf("Documents again: %+v", r)
	}

	// Update collection list counts again.
	before = sftp.CountRequests("PROPFIND", inside)
	recount(t, app, "Documents")
	counted(t, app)
	if sftp.CountRequests("PROPFIND", inside) == before {
		t.Error("Update collection list did not count Documents again")
	}
}

func TestAFileCollectionSaysItIsCounting(t *testing.T) {
	app := newMockupApp(t)
	sftp := app.servers["sftpgo · vps"]
	counted(t, app) // the set's own visit of Connections is over
	release := sftp.StallAt(app.realPath(t, "Documents") + "carrel/")
	defer release()
	recount(t, app, "Documents")
	deadline := time.Now().Add(10 * time.Second)
	for {
		r := fileRows(decode[view.ConnectionsPage](t, app.h.do(get("/app/settings/connections", app.sess))))["Documents"]
		// The root's one file is counted; the folder «carrel» is held.
		if r.Counting && r.Objects != nil && *r.Objects == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Documents while held: %+v", r)
		}
		time.Sleep(20 * time.Millisecond)
	}
	release()
	if r := fileRows(counted(t, app))["Documents"]; r.Objects == nil || *r.Objects != 10 {
		t.Errorf("Documents after: %+v", r)
	}
}

func TestAFileCollectionRefusedMidWayHasNoCount(t *testing.T) {
	app := newMockupApp(t)
	sftp := app.servers["sftpgo · vps"]
	counted(t, app)
	sftp.RefuseAt("PROPFIND", app.realPath(t, "Documents")+"carrel/attachments/", http.StatusServiceUnavailable, "")
	recount(t, app, "Documents")
	r := fileRows(counted(t, app))["Documents"]
	if r.Objects != nil || r.Counting || !strings.HasPrefix(string(r.ObjectsNote), "Not counted: PROPFIND ") ||
		!strings.HasSuffix(string(r.ObjectsNote), "carrel/attachments/ → 503 Service Unavailable.") {
		t.Errorf("Documents refused mid-way: %+v", r)
	}
	if r := fileRows(counted(t, app))["Public"]; r.Objects == nil || *r.Objects != 2 {
		t.Errorf("Public beside it: %+v", r)
	}
}
