package web_test

import (
	"testing"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// The words of s-find-1 that depend on the result (note 114, the owner's
// decision of 23.10.2026): under the title only what was found — «7
// records»; of the sources the bar and the rail speak. The line about files
// is for the person: only the name is searched.
func TestTheSearchSaysWhatItAsked(t *testing.T) {
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work"})
	srv.Seed(searchBook+"ada.vcf", adaCard())
	srv.SeedFile(searchDocs+"Ada contract.pdf", "0123456789")
	f := connectFixture(t, srv, searchBook, nil)
	tickAll(t, f.h, f.sess, sealed.Collection{AccountID: f.id, Path: searchDocs, Name: "Docs", Kinds: []view.Kind{view.KindFile}})
	page := search(t, f.h, f.sess, "q=Ada")
	waitPoll(t, f.h, f.sess, page.Poll.TaskID, nil)
	page = search(t, f.h, f.sess, "q=Ada")
	if page.Subtitle != "2 records" || page.Empty != "" {
		t.Errorf("subtitle %q, empty %q", page.Subtitle, page.Empty)
	}
	note := ""
	for _, r := range page.Note {
		note += string(r.Text)
	}
	if note != "Files are found by their names; what is inside them is not searched." {
		t.Errorf("note %q", note)
	}
}
