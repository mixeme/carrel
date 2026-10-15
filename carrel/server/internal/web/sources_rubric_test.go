package web_test

import (
	"regexp"
	"testing"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/view"
)

// The rubric of the sources of a search and of the duplicates is «Sources»,
// its count apart and monospaced (Дб2, Пк2; s-dup-1, s-find-1: «12 / 12»).
func TestTheSourcesRubricCountsApart(t *testing.T) {
	count := regexp.MustCompile(`^\d+ / \d+$`)
	check := func(name string, rail view.Rail) {
		t.Helper()
		if len(rail.Sections) != 1 || rail.Sections[0].Account != "Sources" || !count.MatchString(string(rail.Sections[0].Tally)) {
			t.Errorf("%s: sections %+v", name, rail.Sections)
		}
	}
	f := newDupFixture(t)
	f.load(t)
	check("duplicates", f.page(t).Rail)
	m := openMerge(t, f.h, f.sess, actionOf(t, onlyGroup(t, f.page(t)), "duplicates.merge"), nil)
	check("merge", m.Rail)

	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work"})
	srv.Seed(searchBook+"ada.vcf", adaCard())
	s := connectFixture(t, srv, searchBook, nil)
	tickAll(t, s.h, s.sess)
	page := search(t, s.h, s.sess, "q=Ada")
	waitPoll(t, s.h, s.sess, page.Poll.TaskID, nil)
	check("search", search(t, s.h, s.sess, "q=Ada").Rail)
}
