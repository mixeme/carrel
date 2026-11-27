package user

import (
	"context"
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// A tick of the rail is what the person looks at, not an event of the
// account: it goes into the blob and nothing into the journal (the owner,
// 27.11.2026: «sources.select» was noise in the audit log).
func TestASelectionIsNotJournaled(t *testing.T) {
	f := newFixture(t)
	id := f.setupAdmin(t)
	_, key, _, _ := f.svc.Authenticate(context.Background(), "admin", "password123")
	connID, err := f.svc.AddConnection(id, key, sealed.Connection{Server: "https://dav.example.org/", Login: "ann", Password: "x"},
		[]sealed.Collection{{Path: "/book/", Name: "Book", Kinds: []view.Kind{view.KindContact}}})
	if err != nil {
		t.Fatal(err)
	}
	before := len(f.st.Snapshot().Audit)
	if err := f.svc.SetSelection(id, key, view.KindContact, nil); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.SetSelection(id, key, view.KindContact, []sealed.Selection{{Kind: view.KindContact, AccountID: connID, Path: "/book/"}}); err != nil {
		t.Fatal(err)
	}
	snap := f.st.Snapshot()
	if len(snap.Audit) != before {
		t.Errorf("the journal grew by %v", snap.Audit[before:])
	}
	blob, _ := f.svc.Catalog(id, key)
	if len(blob.Selected) != 1 {
		t.Errorf("selected = %+v", blob.Selected)
	}
}
