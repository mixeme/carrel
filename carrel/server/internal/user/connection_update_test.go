package user

import (
	"context"
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// The password was changed on the server, or the server moved: the
// connection was dead for good (review Р-9). Edit keeps its ID
// (C-4.1), and every live link of it reads with the new credentials — the
// envelope of each is sealed again, or the public page goes on knocking with
// the old password.
func TestUpdateConnectionKeepsTheIDAndResealsItsLinks(t *testing.T) {
	f := newFixture(t)
	id := f.setupAdmin(t)
	ctx := context.Background()
	_, key, _, _ := f.svc.Authenticate(ctx, "admin", "password123")
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	conn := sealed.Connection{Server: "https://old.example.org/", Login: "ann", Password: "old", CalendarHome: "/old/cal/"}
	other := sealed.Connection{Server: "https://other.example.org/", Login: "bob", Password: "bob"}
	var err error
	conn.ID, err = f.svc.AddConnection(id, key, conn, []sealed.Collection{{Path: "/book/", Name: "Book", Kinds: []view.Kind{view.KindContact}}})
	must(err)
	other.ID, err = f.svc.AddConnection(id, key, other, nil)
	must(err)
	must(f.svc.SetSelection(id, key, view.KindContact, []sealed.Selection{{Kind: view.KindContact, AccountID: conn.ID, Path: "/book/"}}))
	link, err := f.svc.Publish(id, key, conn, PublishTarget{AccountID: conn.ID, Kind: "contact", Col: "/book/", Title: "Book"}, nil)
	must(err)
	otherLink, err := f.svc.Publish(id, key, other, PublishTarget{AccountID: other.ID, Kind: "contact", Col: "/x/", Title: "X"}, nil)
	must(err)

	edited := sealed.Connection{ID: conn.ID, Server: "https://new.example.org/", Login: "ann2", Password: "new", CertFingerprint: "ab:cd", CalendarHome: "/new/cal/"}
	must(f.svc.UpdateConnection(id, key, edited))

	blob, _ := f.svc.Catalog(id, key)
	got, ok := blob.Connection(conn.ID)
	if !ok {
		t.Fatal("the connection lost its ID")
	}
	if got != edited {
		t.Errorf("connection = %+v, want %+v", got, edited)
	}
	if len(blob.Connections) != 2 || len(blob.Selected) != 1 || len(blob.CollectionsOf(conn.ID)) != 1 {
		t.Error("the edit touched the catalog, the ticks or the neighbour")
	}
	open, ok := f.svc.OpenPublished(link)
	if !ok {
		t.Fatal("the link stopped opening after the edit")
	}
	if open.Conn.Server != edited.Server || open.Conn.Login != edited.Login || open.Conn.Password != edited.Password || open.Conn.CertFingerprint != edited.CertFingerprint {
		t.Errorf("the link reads with %+v, want the new credentials", open.Conn)
	}
	if open, ok := f.svc.OpenPublished(otherLink); !ok || open.Conn.Password != "bob" {
		t.Error("the neighbour's link changed")
	}

	// An empty password keeps the one stored: the form shows it empty.
	edited.Password = ""
	must(f.svc.UpdateConnection(id, key, edited))
	blob, _ = f.svc.Catalog(id, key)
	if got, _ := blob.Connection(conn.ID); got.Password != "new" {
		t.Errorf("password = %q after an edit with it empty, want the stored one", got.Password)
	}

	// «The server asks no password» (s-profile-14) is said by the mark, not by
	// emptiness: it clears the stored password, whatever came beside it, and
	// the link reads with none.
	edited.Password, edited.NoPassword = "typed-left-over", true
	must(f.svc.UpdateConnection(id, key, edited))
	blob, _ = f.svc.Catalog(id, key)
	if got, _ := blob.Connection(conn.ID); got.Password != "" || !got.NoPassword {
		t.Errorf("connection = %+v after an edit with the mark, want no password", got)
	}
	if open, ok := f.svc.OpenPublished(link); !ok || open.Conn.Password != "" {
		t.Errorf("the link reads with %q, want no password", open.Conn.Password)
	}

	found := false
	for _, e := range f.st.Snapshot().Audit {
		if e.What == "connection.edit" {
			found = true
			if e.Detail != conn.ID+" links 1" {
				t.Errorf("journal detail = %q: the address must not go in", e.Detail)
			}
		}
	}
	if !found {
		t.Error("no connection.edit line in the journal")
	}
	if err := f.svc.UpdateConnection(id, key, sealed.Connection{ID: "a-NONE", Server: "https://x/"}); err != ErrNotFound {
		t.Errorf("unknown connection: err = %v, want ErrNotFound", err)
	}
}
