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

// freshSession signs in again and opens Connections in the new session: it
// has counted nothing yet.
func freshSession(t *testing.T, app *mockupApp) {
	t.Helper()
	app.sess = app.h.login("mix", "password123")
	app.h.do(get("/app/settings/connections", app.sess))
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

	// Update collection list counts again. A recount keeps the number it
	// had and says nothing of counting meanwhile, so the page may stand
	// before the walk asks: what is waited for is the walk's own listing
	// (the full package under load, 17.11.2026).
	before = sftp.CountRequests("PROPFIND", inside)
	recount(t, app, "Documents")
	counted(t, app)
	for end := time.Now().Add(10 * time.Second); sftp.CountRequests("PROPFIND", inside) == before && time.Now().Before(end); {
		time.Sleep(20 * time.Millisecond)
	}
	if sftp.CountRequests("PROPFIND", inside) == before {
		t.Error("Update collection list did not count Documents again")
	}
}

func TestAFileCollectionSaysItIsCounting(t *testing.T) {
	app := newMockupApp(t)
	sftp := app.servers["sftpgo · vps"]
	release := sftp.StallAt(app.realPath(t, "Documents") + "carrel/")
	defer release()
	// A new session has counted nothing: its first walk shows the count
	// growing.
	freshSession(t, app)
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
	// A recount keeps the number it had and does not say it is counting
	// (TestARecountKeepsTheNumberUntilItIsFound): counted would find it
	// still and not wait. The walk is waited for until the refusal is kept;
	// a slow machine (the Windows gates of 15.11.2026) read the old number.
	deadline := time.Now().Add(10 * time.Second)
	r := fileRows(counted(t, app))["Documents"]
	for r.ObjectsNote == "" && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		r = fileRows(counted(t, app))["Documents"]
	}
	// The words of s-profile-37 (Typecase's note 206), then the request and
	// the server's answer.
	if r.Objects != nil || r.Counting || !strings.HasPrefix(string(r.ObjectsNote), "Counting stopped: the server refused to list a folder in it — PROPFIND ") ||
		!strings.HasSuffix(string(r.ObjectsNote), "carrel/attachments/ → 503 Service Unavailable.") {
		t.Errorf("Documents refused mid-way: %+v", r)
	}
	if r := fileRows(counted(t, app))["Public"]; r.Objects == nil || *r.Objects != 2 {
		t.Errorf("Public beside it: %+v", r)
	}
}

// A storage still waiting for the walk, with no count yet, says it is being
// counted too: between two storages the page does not look finished.
func TestAStorageWaitingForTheWalkSaysItIsCounting(t *testing.T) {
	app := newMockupApp(t)
	sftp := app.servers["sftpgo · vps"]
	// Documents is held at its root's listing; Public, after it on the same
	// connection, waits.
	release := sftp.StallAt(app.realPath(t, "Documents"))
	defer release()
	freshSession(t, app)
	deadline := time.Now().Add(10 * time.Second)
	for {
		rows := fileRows(decode[view.ConnectionsPage](t, app.h.do(get("/app/settings/connections", app.sess))))
		if rows["Documents"].Counting {
			if p := rows["Public"]; !p.Counting || p.Objects != nil {
				t.Errorf("Public waiting for the walk: %+v", p)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Documents while held: %+v", rows["Documents"])
		}
		time.Sleep(20 * time.Millisecond)
	}
	release()
	counted(t, app)
}

// Counted again (Update collection list), a storage keeps its number while
// the walk goes over it, and changes it when the walk is over.
func TestARecountKeepsTheNumberUntilItIsFound(t *testing.T) {
	app := newMockupApp(t)
	sftp := app.servers["sftpgo · vps"]
	inside := app.realPath(t, "Documents") + "carrel/"
	before := sftp.CountRequests("PROPFIND", inside)
	release := sftp.StallAt(inside)
	defer release()
	recount(t, app, "Documents")
	deadline := time.Now().Add(10 * time.Second)
	for sftp.CountRequests("PROPFIND", inside) == before {
		if time.Now().After(deadline) {
			t.Fatal("the recount did not reach «carrel»")
		}
		time.Sleep(20 * time.Millisecond)
	}
	r := fileRows(decode[view.ConnectionsPage](t, app.h.do(get("/app/settings/connections", app.sess))))["Documents"]
	if r.Counting || r.Objects == nil || *r.Objects != 10 {
		t.Errorf("Documents while counted again: %+v", r)
	}
	release()
	counted(t, app)
}
