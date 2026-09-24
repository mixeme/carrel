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
	p := mergePage(t, df, actionOf(t, g, "duplicates.merge").Href)
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
		if page.Notices[i].Title == "Merged" {
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
