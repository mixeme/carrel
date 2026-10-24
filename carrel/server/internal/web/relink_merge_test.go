package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// relatedTodo is a task tied to a record by one RELATED-TO line.
func relatedTodo(uid, related string) string {
	return strings.Join([]string{"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//T//EN", "BEGIN:VTODO",
		"UID:" + uid, "DTSTAMP:20260101T000000Z", "SUMMARY:" + uid, related,
		"END:VTODO", "END:VCALENDAR", ""}, "\r\n")
}

// A merge keeps one card and deletes the others (S-16), and with them their
// UIDs. A task made from the timeline of the Ada who is deleted is tied to
// her by RELATED-TO alone; left as it was, it would point at nothing and
// drop out of the timeline of the Ada who stays. Before anything is deleted,
// what pointed at the deleted card is made to point at the kept one — and
// the page says so, on the kept card, instead of a redirect that would say
// nothing.
func TestAMergeKeepsWhatPointedAtTheGoneCard(t *testing.T) {
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	srv.AddAddressBook(carddav.AddressBook{Path: peopleBook, Name: "People"})
	tasks := davtest.CalendarHome + "tasks/"
	srv.AddCalendar(caldav.Calendar{Path: tasks, Name: "Tasks", SupportedComponentSet: []string{"VTODO"}})
	archive := davtest.CalendarHome + "archive/"
	srv.AddCalendar(caldav.Calendar{Path: archive, Name: "Archive", SupportedComponentSet: []string{"VTODO"}, ReadOnly: true})
	srv.Seed(peopleBook+"ada-here.vcf", adaHere)
	f := connectFixture(t, srv, peopleBook, nil)

	other := davtest.New(t)
	other.Login, other.Password = "bob", "other"
	other.AddAddressBook(carddav.AddressBook{Path: workBook, Name: "Work"})
	other.Seed(workBook+"ada-there.vcf", adaThere)
	df := dupFixture{moveFixture{davFixture: f, other: other, otherID: addAccount(t, f, other)}}

	srv.Seed(tasks+"call.ics", relatedTodo("call", "RELATED-TO:ada-there"))
	srv.Seed(tasks+"both.ics", relatedTodo("both", "RELATED-TO:ada-here\r\nRELATED-TO:ada-there"))
	srv.Seed(archive+"old.ics", relatedTodo("old", "RELATED-TO:ada-there"))
	oldBefore, _, _ := srv.Object(archive + "old.ics")
	key, _ := df.sess.DataKey()
	if err := df.h.users.SetSelection(df.sess.AccountID(), key, view.KindTask, []sealed.Selection{
		{Kind: view.KindTask, AccountID: df.id, Path: tasks},
		{Kind: view.KindTask, AccountID: df.id, Path: archive},
	}); err != nil {
		t.Fatal(err)
	}

	df.load(t)
	g := onlyGroup(t, df.page(t))
	p := openMerge(t, df.h, df.sess, g, nil)
	if strings.HasSuffix(string(p.Target.Stamp), "Work") {
		p = mergePage(t, df, mergeAction(t, p, "duplicates.merge.into").Href)
	}
	rec := df.h.do(postForm(mergeAction(t, p, "duplicates.merge.confirm").Href, df.sess, url.Values{}))
	if rec.Code != http.StatusOK {
		t.Fatalf("merge answered %d, want the kept card with what became of the links: %s", rec.Code, rec.Body.String())
	}
	if _, _, ok := other.Object(workBook + "ada-there.vcf"); ok {
		t.Fatal("the other card was not deleted")
	}
	call, _, _ := srv.Object(tasks + "call.ics")
	if !strings.Contains(call, "RELATED-TO:ada-here") || strings.Contains(call, "ada-there") {
		t.Errorf("the task still points at the deleted card:\n%s", call)
	}
	// A task tied to both Adas is tied to the one who stays once, not twice.
	both, _, _ := srv.Object(tasks + "both.ics")
	if strings.Count(both, "RELATED-TO:ada-here") != 1 || strings.Contains(both, "ada-there") {
		t.Errorf("a task tied to both cards:\n%s", both)
	}
	// A read-only calendar cannot be written: its task is left as it was,
	// the merge goes on, and the page names what still points at the gone
	// card and why.
	if old, _, _ := srv.Object(archive + "old.ics"); old != oldBefore {
		t.Errorf("a read-only calendar was written:\n%s", old)
	}
	page := decode[view.ContactPage](t, rec)
	var merged *view.Notice
	for i := range page.Notices {
		if page.Notices[i].Title == "Merged." {
			merged = &page.Notices[i]
		}
	}
	if merged == nil || merged.Role != view.NoticeAlert || merged.Details == nil {
		t.Fatalf("notices = %+v", page.Notices)
	}
	var named bool
	for _, l := range merged.Details.Lines {
		named = named || strings.Contains(string(l), "Still pointing at the old card: old · Archive") && strings.Contains(string(l), "read-only")
	}
	if !named {
		t.Errorf("the read-only task is not named with its reason: %+v", merged.Details.Lines)
	}
}

// docs/servers.md, SV-25: a card moved to a server that rewrites the UID of
// what is written to it — Google — did not finish moving. The copy was read
// back by the UID it was sent with, was not found, and the original was kept:
// the person had the card twice. It is read back at the address it was
// written to now, as a new card is, the original is deleted, and what pointed
// at the old UID points at the one the server gave.
func TestAMoveIntoAServerThatRenamesFinishesAndKeepsTheLinks(t *testing.T) {
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	srv.AddAddressBook(carddav.AddressBook{Path: peopleBook, Name: "People"})
	tasks := davtest.CalendarHome + "tasks/"
	srv.AddCalendar(caldav.Calendar{Path: tasks, Name: "Tasks", SupportedComponentSet: []string{"VTODO"}})
	srv.Seed(peopleBook+"ada.vcf", movedCard)
	srv.Seed(tasks+"call.ics", relatedTodo("call", "RELATED-TO:ada"))
	f := connectFixture(t, srv, peopleBook, nil)
	other := davtest.New(t)
	other.Login, other.Password = "bob", "other"
	other.AddAddressBook(carddav.AddressBook{Path: workBook, Name: "Work"})
	other.CreateRewritesUID = true
	mf := moveFixture{davFixture: f, other: other, otherID: addAccount(t, f, other)}
	key, _ := mf.sess.DataKey()
	if err := mf.h.users.SetSelection(mf.sess.AccountID(), key, view.KindTask,
		[]sealed.Selection{{Kind: view.KindTask, AccountID: mf.id, Path: tasks}}); err != nil {
		t.Fatal(err)
	}

	page := decode[view.ContactPage](t, mf.h.do(postConfirm(t, mf, sheetFor(mf, peopleBook, "ada", mf.otherID, workBook, false))))
	if !hasNotice(page.Notices, "Moved, but the server gave it another identity") {
		t.Fatalf("notices = %+v", page.Notices)
	}
	if _, _, ok := srv.Object(peopleBook + "ada.vcf"); ok {
		t.Error("the original is still in People: the move did not finish")
	}
	var opens string
	for _, n := range page.Notices {
		for _, a := range n.Actions {
			if a.ID == "record.open" {
				opens = a.Href
			}
		}
	}
	renamed, _ := url.PathUnescape(opens[strings.LastIndex(opens, "/")+1:])
	if !strings.HasPrefix(renamed, "sv25-") {
		t.Fatalf("the notice opens %q, want the card under the UID the server gave", opens)
	}
	if moved := decode[view.ContactPage](t, mf.h.do(get(opens, mf.sess))); moved.Name != "Ada" {
		t.Errorf("the card the notice opens = %+v", moved)
	}
	if call, _, _ := srv.Object(tasks + "call.ics"); !strings.Contains(call, "RELATED-TO:"+renamed) {
		t.Errorf("the task still points at the old card:\n%s", call)
	}
}

// A copy into the same server lands on the copy, as a new card does, not on
// an error over a record that was written whole.
func TestACopyIntoAServerThatRenamesLandsOnTheCopy(t *testing.T) {
	f := newMoveFixture(t)
	f.other.CreateRewritesUID = true
	resp := f.h.do(postConfirm(t, f, sheetFor(f, peopleBook, "ada", f.otherID, workBook, true)))
	if resp.Code != http.StatusSeeOther {
		t.Fatalf("copy: %d %s", resp.Code, resp.Body.String())
	}
	loc := resp.Header().Get("Location")
	if !strings.HasPrefix(loc, f.record(f.otherID, workBook, "sv25-")) {
		t.Fatalf("Location = %q, want the copy under the UID the server gave", loc)
	}
	if _, _, ok := f.srv.Object(peopleBook + "ada.vcf"); !ok {
		t.Error("a copy deleted the original")
	}
}
