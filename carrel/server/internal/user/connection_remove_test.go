package user

import (
	"context"
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

// A connection made by mistake left the rail, the search and every poll only
// with the account (connections-tail.md). Disconnect takes it out whole —
// everything the blob and the state bind to its ID — and the neighbouring
// connection keeps all of its own.
func TestRemoveConnectionTakesEverythingOfIt(t *testing.T) {
	f := newFixture(t)
	id := f.setupAdmin(t)
	ctx := context.Background()
	_, key, _, _ := f.svc.Authenticate(ctx, "admin", "password123")

	cols := func() []sealed.Collection {
		return []sealed.Collection{
			{Path: "/book/", Name: "Book", Kinds: []view.Kind{view.KindContact}},
			{Path: "/files/", Name: "Files", Kinds: []view.Kind{view.KindFile}},
		}
	}
	gone := sealed.Connection{Server: "https://gone.example.org/", Login: "ann", Password: "pw"}
	kept := sealed.Connection{Server: "https://kept.example.org/", Login: "bob", Password: "pw"}
	var err error
	if gone.ID, err = f.svc.AddConnection(id, key, gone, cols()); err != nil {
		t.Fatal(err)
	}
	if kept.ID, err = f.svc.AddConnection(id, key, kept, cols()); err != nil {
		t.Fatal(err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	both := func(path string) []sealed.Selection {
		return []sealed.Selection{{Kind: view.KindContact, AccountID: gone.ID, Path: path}, {Kind: view.KindContact, AccountID: kept.ID, Path: path}}
	}
	must(f.svc.SetSelection(id, key, view.KindContact, both("/book/")))
	must(f.svc.SetLastUsed(id, key, sealed.Selection{Kind: view.KindContact, AccountID: gone.ID, Path: "/book/"}))
	must(f.svc.SetLastUsed(id, key, sealed.Selection{Kind: view.KindFile, AccountID: kept.ID, Path: "/files/"}))
	must(f.svc.SetAttachmentFolder(id, key, sealed.AttachmentFolder{AccountID: gone.ID, Path: "/files/"}))
	must(f.svc.SetUploadFolder(id, key, sealed.AttachmentFolder{AccountID: gone.ID, Path: "/files/"}))
	must(f.svc.SetDeviceExcluded(id, key, gone.ID, true))
	must(f.svc.SetDeviceExcluded(id, key, kept.ID, true))
	ref := func(acc, uid string) sealed.RecordRef {
		return sealed.RecordRef{AccountID: acc, Col: "/book/", UID: uid}
	}
	must(f.svc.EditDuplicates(id, key, "t", "", func(d *sealed.Duplicates) error {
		d.Linked = []sealed.LinkedGroup{
			// Two members, one of them here: the group of one dissolves.
			{ID: "g1", Kind: view.KindContact, Members: []sealed.RecordRef{ref(gone.ID, "a"), ref(kept.ID, "a")},
				Prefs: map[string]sealed.RecordRef{"FN": ref(gone.ID, "a")}},
			// Three members, one here: the other two stay linked, and a
			// preference that named the one gone is asked again.
			{ID: "g2", Kind: view.KindContact, Members: []sealed.RecordRef{ref(gone.ID, "b"), ref(kept.ID, "b"), ref(kept.ID, "c")},
				Prefs: map[string]sealed.RecordRef{"FN": ref(gone.ID, "b"), "ORG": ref(kept.ID, "c")}},
		}
		d.Ignored = []string{"sig"}
		return nil
	}))
	onlyGone, err := f.svc.SaveBackupJob(id, key, sealed.BackupJob{Name: "only", Mode: sealed.BackupToDownload, Keep: 1,
		Sources: []sealed.Selection{{Kind: view.KindContact, AccountID: gone.ID, Path: "/book/"}}})
	must(err)
	mixed, err := f.svc.SaveBackupJob(id, key, sealed.BackupJob{Name: "mixed", Mode: sealed.BackupToWebDAV, Keep: 1,
		Sources: both("/book/"), DestAccount: gone.ID, DestPath: "/files/"})
	must(err)
	goneLink, err := f.svc.Publish(id, key, gone, PublishTarget{AccountID: gone.ID, Kind: "contact", Col: "/book/", Title: "Book"}, nil)
	must(err)
	keptLink, err := f.svc.Publish(id, key, kept, PublishTarget{AccountID: kept.ID, Kind: "contact", Col: "/book/", Title: "Book"}, nil)
	must(err)
	scope := DeviceScope{Kinds: []view.Kind{view.KindContact}, Cols: []store.DeviceScope{{AccountID: gone.ID, Path: "/book/"}, {AccountID: kept.ID, Path: "/book/"}}}
	device, _, err := f.svc.IssueDevice(ctx, id, key, "Phone", scope)
	must(err)
	onlyGoneDevice, _, err := f.svc.IssueDevice(ctx, id, key, "Tablet", DeviceScope{Kinds: []view.Kind{view.KindContact}, Cols: []store.DeviceScope{{AccountID: gone.ID, Path: "/book/"}}})
	must(err)
	changed := 0
	f.svc.OnDevicesChanged(func(string) { changed++ })

	res, err := f.svc.RemoveConnection(id, key, gone.ID)
	must(err)
	if res.Collections != 2 || res.Links != 1 || res.Jobs != 2 || res.Devices != 2 {
		t.Errorf("counts = %+v, want 2 collections, 1 link, 2 jobs, 2 devices", res)
	}

	blob, ok := f.svc.Catalog(id, key)
	if !ok {
		t.Fatal("blob does not open")
	}
	if _, ok := blob.Connection(gone.ID); ok {
		t.Error("the connection is still in the blob")
	}
	if _, ok := blob.Connection(kept.ID); !ok {
		t.Error("the neighbouring connection went too")
	}
	for _, c := range blob.Collections {
		if c.AccountID == gone.ID {
			t.Errorf("collection %s of the connection stays", c.Path)
		}
	}
	if n := len(blob.CollectionsOf(kept.ID)); n != 2 {
		t.Errorf("the neighbour has %d collections, want 2", n)
	}
	if len(blob.Selected) != 1 || blob.Selected[0].AccountID != kept.ID {
		t.Errorf("selected = %+v, want the neighbour's tick only", blob.Selected)
	}
	if len(blob.LastUsed) != 1 || blob.LastUsed[0].AccountID != kept.ID {
		t.Errorf("last used = %+v, want the neighbour's only", blob.LastUsed)
	}
	if blob.Attachments != nil || blob.UploadTo != nil {
		t.Error("the attachment or the upload folder still points to the connection")
	}
	if len(blob.DeviceExcluded) != 1 || blob.DeviceExcluded[0] != kept.ID {
		t.Errorf("device excluded = %v, want the neighbour only", blob.DeviceExcluded)
	}
	d := blob.Duplicates
	if len(d.Linked) != 1 || d.Linked[0].ID != "g2" || len(d.Linked[0].Members) != 2 {
		t.Fatalf("linked = %+v, want g2 with the neighbour's two members", d.Linked)
	}
	if _, ok := d.Linked[0].Prefs["FN"]; ok {
		t.Error("a preference names a member that is gone")
	}
	if d.Linked[0].Prefs["ORG"] != ref(kept.ID, "c") {
		t.Error("the neighbour's preference was lost")
	}
	if len(d.Ignored) != 1 {
		t.Error("the ignored signatures were touched")
	}
	job, ok := blob.BackupJob(onlyGone)
	if !ok {
		t.Fatal("a job whose every source was the connection was deleted silently")
	}
	if len(job.Sources) != 0 {
		t.Errorf("job sources = %+v, want none", job.Sources)
	}
	job, ok = blob.BackupJob(mixed)
	if !ok || len(job.Sources) != 1 || job.Sources[0].AccountID != kept.ID {
		t.Errorf("mixed job = %+v, want the neighbour's source only", job)
	}
	if job.DestAccount != gone.ID {
		t.Error("the job whose destination was the connection lost the mark of it: it must say the destination is gone")
	}
	if _, ok := f.svc.OpenPublished(goneLink); ok {
		t.Error("the connection's link still opens")
	}
	if _, ok := f.svc.OpenPublished(keptLink); !ok {
		t.Error("the neighbour's link stopped opening")
	}
	if len(blob.Links) != 1 {
		t.Errorf("blob keeps %d links, want the neighbour's one", len(blob.Links))
	}
	for _, l := range f.st.Snapshot().Published {
		if l.AccountID == gone.ID {
			t.Error("the state still holds the connection's envelope")
		}
	}
	for _, dev := range f.svc.Devices(id) {
		for _, c := range dev.ScopeCols {
			if c.AccountID == gone.ID {
				t.Errorf("device %s still sees the connection", dev.ID)
			}
		}
		if dev.ID == device && len(dev.ScopeCols) != 1 {
			t.Errorf("device %s lost the neighbour's collection", dev.ID)
		}
		if dev.ID == onlyGoneDevice && len(dev.ScopeCols) != 0 {
			t.Error("a device that saw only this connection still sees something")
		}
	}
	if changed == 0 {
		t.Error("the proxy was not told the devices changed")
	}
	snap := f.st.Snapshot()
	if u := snap.UserByID(id); u.ConnCount != 1 {
		t.Errorf("conn count = %d, want 1", u.ConnCount)
	}
	var line *store.AuditEntry
	for _, e := range f.st.Snapshot().Audit {
		if e.What == "connection.remove" {
			line = &e
		}
	}
	if line == nil {
		t.Fatal("no connection.remove line in the journal")
	}
	if want := gone.ID + " collections 2 links 1 jobs 2 devices 2"; line.Detail != want {
		t.Errorf("journal detail = %q, want %q", line.Detail, want)
	}

	if _, err := f.svc.RemoveConnection(id, key, gone.ID); err != ErrNotFound {
		t.Errorf("second removal: err = %v, want ErrNotFound", err)
	}
}
