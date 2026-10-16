package web_test

import (
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// The words of s-find-1 that depend on the result (Пк5, Пк14): under the
// title what was asked, how much was found and what did not answer —
// «14 of 14 collections · 7 records · 1 source timed out»; the line about
// files with PROPFIND monospaced.
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
	if page.Subtitle != "3 of 3 collections · 2 records" {
		t.Errorf("subtitle %q", page.Subtitle)
	}
	note := ""
	mono := false
	for _, r := range page.Note {
		note += string(r.Text)
		mono = mono || r.Mark == view.RunMono && r.Text == "PROPFIND"
	}
	if !strings.HasPrefix(note, "Files are searched by name: PROPFIND returns names anyway") || !mono {
		t.Errorf("note %q, PROPFIND monospaced %v", note, mono)
	}
}
