package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// Б33 of the audit of 11.10.2026: after a merge — and a move that changes a
// UID — what pointed at the card that went was made to point at the one that
// stays only in events, tasks and notes; the links of cards themselves —
// RELATED of a person, MEMBER of a group — kept the old UID and pointed at
// nothing. They are mended the same way now, in the ticked books, keeping
// how they were written (urn:uuid: or the bare UID) and their parameters; a
// group that named both cards names the one that stays once.
func TestAMergeKeepsTheLinksOfCards(t *testing.T) {
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	srv.AddAddressBook(carddav.AddressBook{Path: peopleBook, Name: "People"})
	srv.Seed(peopleBook+"ada-here.vcf", adaHere)
	srv.Seed(peopleBook+"bob.vcf", "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:bob\r\nFN:Bob\r\nRELATED;TYPE=friend:urn:uuid:ada-there\r\nEND:VCARD\r\n")
	srv.Seed(peopleBook+"team.vcf", "BEGIN:VCARD\r\nVERSION:4.0\r\nUID:team\r\nKIND:group\r\nFN:Team\r\n"+
		"MEMBER:urn:uuid:ada-here\r\nMEMBER:urn:uuid:ada-there\r\nMEMBER:urn:uuid:bob\r\nEND:VCARD\r\n")
	f := connectFixture(t, srv, peopleBook, nil)

	other := davtest.New(t)
	other.Login, other.Password = "bob", "other"
	other.AddAddressBook(carddav.AddressBook{Path: workBook, Name: "Work"})
	other.Seed(workBook+"ada-there.vcf", adaThere)
	df := dupFixture{moveFixture{davFixture: f, other: other, otherID: addAccount(t, f, other)}}
	// The book is ticked: the links are mended where the timeline reads.
	key, _ := df.sess.DataKey()
	if err := df.h.users.SetSelection(df.sess.AccountID(), key, view.KindContact, []sealed.Selection{
		{Kind: view.KindContact, AccountID: df.id, Path: peopleBook},
	}); err != nil {
		t.Fatal(err)
	}

	df.load(t)
	g := onlyGroup(t, df.page(t))
	p := openMerge(t, df.h, df.sess, g, nil)
	if strings.HasSuffix(string(p.Target.Stamp), "Work") {
		p = mergePage(t, df, mergeAction(t, p, "duplicates.merge.into").Href)
	}
	if rec := df.h.do(postForm(mergeAction(t, p, "duplicates.merge.confirm").Href, df.sess, url.Values{})); rec.Code != http.StatusOK && rec.Code != http.StatusSeeOther {
		t.Fatalf("merge: %d %s", rec.Code, rec.Body.String())
	}
	bob, _, _ := srv.Object(peopleBook + "bob.vcf")
	if !strings.Contains(bob, "RELATED;TYPE=friend:urn:uuid:ada-here") || strings.Contains(bob, "ada-there") {
		t.Errorf("a person's link still points at the gone card:\n%s", bob)
	}
	team, _, _ := srv.Object(peopleBook + "team.vcf")
	if strings.Count(team, "MEMBER:urn:uuid:ada-here") != 1 || strings.Contains(team, "ada-there") || !strings.Contains(team, "MEMBER:urn:uuid:bob") {
		t.Errorf("the group:\n%s", team)
	}
}
