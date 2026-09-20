package web_test

import (
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// linkedRowOf finds the single folded row of a combined list.
func linkedRowOf(t *testing.T, p view.ContactsListPage) view.ContactRow {
	t.Helper()
	var out []view.ContactRow
	for _, g := range p.Groups {
		for _, r := range g.Rows {
			if len(r.Sources) > 0 {
				out = append(out, r)
			}
		}
	}
	if len(out) != 1 {
		t.Fatalf("folded rows = %d, want 1", len(out))
	}
	return out[0]
}

// relink rewrites the one linked group with the members in this order.
func relink(t *testing.T, f dupFixture, members ...sealed.RecordRef) {
	t.Helper()
	key, _ := f.sess.DataKey()
	err := f.h.users.EditDuplicates(f.sess.AccountID(), key, "duplicates.link", "", func(d *sealed.Duplicates) error {
		d.Linked = []sealed.LinkedGroup{{ID: "g-1", Kind: view.KindContact, Members: members}}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// A folded row shows one of the linked records, and which one is a decision
// of S-16 — the first member of the group — not a race. It used to be
// whichever server answered first, so the same list gave a different row,
// a different address and a different source stamp on the next run.
func TestAFoldedRowShowsTheFirstMemberOfItsGroup(t *testing.T) {
	f := newDupFixture(t)
	f.load(t)
	here := sealed.RecordRef{AccountID: f.id, Col: peopleBook, UID: "ada-here"}
	there := sealed.RecordRef{AccountID: f.otherID, Col: workBook, UID: "ada-there"}
	tickRail(t, f.h, f.sess, "contacts", f.book(f.id, peopleBook), f.book(f.otherID, workBook))

	relink(t, f, there, here)
	row := linkedRowOf(t, combined(t, f.h, f.sess, "/app/contacts", contactsPoll))
	if row.UID != "ada-there" {
		t.Errorf("the row is %q, want the first member ada-there", row.UID)
	}
	if len(row.Sources) != 2 || row.Sources[0] != row.Stamp {
		t.Errorf("sources = %v, want the group's order behind the stamp %q", row.Sources, row.Stamp)
	}

	relink(t, f, here, there)
	row = linkedRowOf(t, combined(t, f.h, f.sess, "/app/contacts", contactsPoll))
	if row.UID != "ada-here" {
		t.Errorf("the row is %q, want the first member ada-here", row.UID)
	}
	if len(row.Sources) != 2 || row.Sources[0] != row.Stamp {
		t.Errorf("sources = %v, want the group's order behind the stamp %q", row.Sources, row.Stamp)
	}
}
