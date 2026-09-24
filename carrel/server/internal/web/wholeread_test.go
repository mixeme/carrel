package web_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"carrel.dev/carrel/server/view"
)

// vcardOf is a minimal card with a UID and a name.
func vcardOf(uid, name string) string {
	return "BEGIN:VCARD\nVERSION:4.0\nUID:" + uid + "\nFN:" + name + "\nEND:VCARD\n"
}

// keptNames is the names a contacts list shows, in order.
func keptNames(t *testing.T, f davFixture) []string {
	t.Helper()
	page := decode[view.ContactsListPage](t, f.h.do(get(f.path("contacts"), f.sess)))
	var out []string
	for _, g := range page.Groups {
		for _, r := range g.Rows {
			out = append(out, string(r.Name))
		}
	}
	return out
}

// S-4.2: a collection read whole is kept, and read again only when its change
// tag moved. The second list of an unchanged book asks the tag and nothing
// else: no REPORT, and the same list. Until 24.09.2026 every list, agenda and
// search read its collections whole, every time.
func TestAnUnchangedBookIsNotReadTwice(t *testing.T) {
	f := contactsFixture(t, false, vcardOf("ada", "Ada Lovelace"), vcardOf("bob", "Bob Builder"))
	first := keptNames(t, f)
	reports := f.srv.CountRequests("REPORT", f.col)

	second := keptNames(t, f)
	if n := f.srv.CountRequests("REPORT", f.col); n != reports {
		t.Errorf("REPORTs %d → %d: an unchanged book was read again", reports, n)
	}
	if strings.Join(second, "|") != strings.Join(first, "|") || len(second) != 2 {
		t.Errorf("the kept list %v differs from the read one %v", second, first)
	}
}

// The tag moves when the book changes — here another client writes a card —
// and the next list goes to the server and shows it.
func TestAChangedBookIsReadAgain(t *testing.T) {
	f := contactsFixture(t, false, vcardOf("ada", "Ada Lovelace"))
	keptNames(t, f)
	reports := f.srv.CountRequests("REPORT", f.col)

	f.srv.Seed(f.col+"cyd.vcf", vcardOf("cyd", "Cyd Charisse"))
	names := keptNames(t, f)
	if n := f.srv.CountRequests("REPORT", f.col); n == reports {
		t.Error("the book changed and was not read again")
	}
	if strings.Join(names, "|") != "Ada Lovelace|Cyd Charisse" {
		t.Errorf("list = %v, want the card another client wrote", names)
	}
}

// A server with no change tag: a kept read stands for the soft limit and no
// longer, and a write Carrel makes drops it at once — this session never shows
// itself a list older than its own write.
func TestWithoutATagTheSoftLimit(t *testing.T) {
	f := contactsFixture(t, false, vcardOf("ada", "Ada Lovelace"))
	f.srv.NoChangeTag = true
	start := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	f.h.srv.SetClock(start)
	keptNames(t, f)
	reports := f.srv.CountRequests("REPORT", f.col)

	f.h.srv.SetClock(start.Add(f.h.cfg.CacheCollectionTTL / 2))
	keptNames(t, f)
	if n := f.srv.CountRequests("REPORT", f.col); n != reports {
		t.Errorf("REPORTs %d → %d: a kept read within the soft limit was not used", reports, n)
	}

	// Another client's card is not seen within the limit — that is what the
	// limit means — but a contact made through the pages is, at once.
	f.srv.Seed(f.col+"cyd.vcf", vcardOf("cyd", "Cyd Charisse"))
	list := f.path("contacts")
	form := formFields(decode[view.ContactEditPage](t, f.h.do(get(list+"/new", f.sess))).Form)
	form.Set("fn", "Bob Builder")
	if rec := f.h.do(postForm(list+"/new", f.sess, form)); rec.Code != http.StatusSeeOther {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	if names := keptNames(t, f); strings.Join(names, "|") != "Ada Lovelace|Bob Builder|Cyd Charisse" {
		t.Errorf("list after Carrel's own write = %v", names)
	}

	// Past the limit the read goes to the server again.
	reports = f.srv.CountRequests("REPORT", f.col)
	f.h.srv.SetClock(start.Add(2 * f.h.cfg.CacheCollectionTTL))
	keptNames(t, f)
	if n := f.srv.CountRequests("REPORT", f.col); n == reports {
		t.Error("a kept read older than the soft limit was used")
	}
}

// The cache never stands in for the server: a collection that cannot be asked
// for its tag is not taken for an unchanged one, and the read goes to the
// server — even on a server without tags, where the kept read is within the
// soft limit and would otherwise be used.
func TestAnUnaskableTagReadsTheServer(t *testing.T) {
	f := contactsFixture(t, false, vcardOf("ada", "Ada Lovelace"))
	f.srv.NoChangeTag = true
	keptNames(t, f)
	reports := f.srv.CountRequests("REPORT", f.col)

	f.srv.Refuse("PROPFIND", http.StatusServiceUnavailable, "busy")
	f.h.do(get(f.path("contacts"), f.sess))
	if n := f.srv.CountRequests("REPORT", f.col); n == reports {
		t.Error("the tag could not be asked and the kept read was used anyway")
	}
}

// A calendar's whole read — a task list is one — is kept the same way.
func TestAnUnchangedCalendarIsNotReadTwice(t *testing.T) {
	f := calendarFixture(t, nil, "t1.ics", ics("BEGIN:VTODO\nUID:t1\nDTSTAMP:20260101T000000Z\nSUMMARY:Call Ada\nEND:VTODO"))
	tasks := f.path("tasks") + "?seg=all"
	decode[view.TasksPage](t, f.h.do(get(tasks, f.sess)))
	reports := f.srv.CountRequests("REPORT", f.col)

	page := decode[view.TasksPage](t, f.h.do(get(tasks, f.sess)))
	if n := f.srv.CountRequests("REPORT", f.col); n != reports {
		t.Errorf("REPORTs %d → %d: an unchanged calendar was read again", reports, n)
	}
	if len(page.Groups) != 1 || len(page.Groups[0].Rows) != 1 || string(page.Groups[0].Rows[0].Summary) != "Call Ada" {
		t.Errorf("kept tasks = %+v", page.Groups)
	}
}

// Refresh is how a person asks for what is there now: on a server without a
// change tag it reads the server again although the soft limit has not run
// out, where a plain load of the same page is answered from the kept read.
func TestRefreshReadsATaglessServerAgain(t *testing.T) {
	f := contactsFixture(t, false, vcardOf("ada", "Ada Lovelace"))
	f.srv.NoChangeTag = true
	tickRail(t, f.h, f.sess, "contacts", f.path("contacts"))
	combined(t, f.h, f.sess, "/app/contacts", contactsPoll)

	f.srv.Seed(f.col+"cyd.vcf", vcardOf("cyd", "Cyd Charisse"))
	plain := combined(t, f.h, f.sess, "/app/contacts", contactsPoll)
	if got := contactNames(plain); got != "Ada Lovelace · People" {
		t.Errorf("a plain load within the soft limit = %s, want the kept read", got)
	}
	again := combined(t, f.h, f.sess, plain.Top.Refresh.Action.Href, contactsPoll)
	if got := contactNames(again); got != "Ada Lovelace · People, Cyd Charisse · People" {
		t.Errorf("after Refresh = %s, want the card another client wrote", got)
	}
}

// titles is what a finished search found, by title.
func titles(t *testing.T, f davFixture, query string) []string {
	t.Helper()
	page := search(t, f.h, f.sess, query)
	waitPoll(t, f.h, f.sess, page.Poll.TaskID, nil)
	page = search(t, f.h, f.sess, query)
	var out []string
	for _, r := range page.Results {
		out = append(out, string(r.Title))
	}
	return out
}

// A search of a book read whole and unchanged is sifted from the kept read:
// the tag is asked, nothing else. A changed book is searched on the server.
func TestASearchOfAKeptBookAsksNothing(t *testing.T) {
	f := contactsFixture(t, false, vcardOf("ada", "Ada Lovelace"), vcardOf("bob", "Bob Builder"))
	tickAll(t, f.h, f.sess)
	keptNames(t, f)
	reports := f.srv.CountRequests("REPORT", f.col)

	if got := titles(t, f, "q=ada"); strings.Join(got, "|") != "Ada Lovelace" {
		t.Errorf("found %v, want Ada: the sifting is the server's, letter case aside", got)
	}
	if n := f.srv.CountRequests("REPORT", f.col); n != reports {
		t.Errorf("REPORTs %d → %d: a kept, unchanged book was searched on the server", reports, n)
	}

	f.srv.Seed(f.col+"ann.vcf", vcardOf("ann", "Ann Adams"))
	if got := titles(t, f, "q=Adams"); strings.Join(got, "|") != "Ann Adams" {
		t.Errorf("found %v after another client's write", got)
	}
	if n := f.srv.CountRequests("REPORT", f.col); n == reports {
		t.Error("the book changed and its search was not asked of the server")
	}
}

// The owner's point (23.09.2026): a calendar search that finds nothing is
// checked by reading the calendar whole (SV-47), and that read is kept — so
// the next search of the unchanged calendar is sifted from it and asks
// nothing but the tag.
func TestTheCheckOfAnEmptySearchIsPaidOnce(t *testing.T) {
	f := calendarFixture(t, nil,
		"call.ics", ics("BEGIN:VEVENT\nUID:call\nDTSTAMP:20260101T000000Z\nDTSTART:20260910T150000Z\nSUMMARY:Two words\nEND:VEVENT"),
		"other.ics", ics("BEGIN:VEVENT\nUID:other\nDTSTAMP:20260101T000000Z\nDTSTART:20260911T150000Z\nSUMMARY:Standup\nEND:VEVENT"))
	f.srv.CalendarTextMatchFindsNothing = true
	tickAll(t, f.h, f.sess)

	if got := titles(t, f, "q=Two+words"); strings.Join(got, "|") != "Two words" {
		t.Fatalf("found %v: the empty answer was not checked", got)
	}
	reports := f.srv.CountRequests("REPORT", f.col)
	if got := titles(t, f, "q=standup"); strings.Join(got, "|") != "Standup" {
		t.Errorf("found %v from the kept read", got)
	}
	if n := f.srv.CountRequests("REPORT", f.col); n != reports {
		t.Errorf("REPORTs %d → %d: the whole read of the check was not kept", reports, n)
	}
}

// weekSummaries is what an agenda week shows, by summary.
func weekSummaries(t *testing.T, f davFixture, from string) []string {
	t.Helper()
	page := decode[view.AgendaPage](t, f.h.do(get(f.path("calendar")+"?from="+from, f.sess)))
	var out []string
	for _, d := range page.Days {
		for _, r := range d.Rows {
			out = append(out, string(r.Summary))
		}
	}
	return out
}

// An agenda's week is kept under its window: what falls in it is the server's
// answer as it came. Going back to a week seen before asks the tag; another
// week is its own read; a change is read again.
func TestAWeekSeenBeforeIsNotReadAgain(t *testing.T) {
	f := calendarFixture(t, nil,
		"a.ics", ics("BEGIN:VEVENT\nUID:a\nDTSTAMP:20260101T000000Z\nDTSTART:20260908T100000Z\nSUMMARY:First week\nEND:VEVENT"),
		"b.ics", ics("BEGIN:VEVENT\nUID:b\nDTSTAMP:20260101T000000Z\nDTSTART:20260915T100000Z\nSUMMARY:Second week\nEND:VEVENT"))
	if got := weekSummaries(t, f, "2026-09-07"); strings.Join(got, "|") != "First week" {
		t.Fatalf("first week = %v", got)
	}
	reports := f.srv.CountRequests("REPORT", f.col)
	if got := weekSummaries(t, f, "2026-09-14"); strings.Join(got, "|") != "Second week" {
		t.Errorf("second week = %v", got)
	}
	if n := f.srv.CountRequests("REPORT", f.col); n == reports {
		t.Error("another week was answered from the first one's read")
	}
	reports = f.srv.CountRequests("REPORT", f.col)
	if got := weekSummaries(t, f, "2026-09-07"); strings.Join(got, "|") != "First week" {
		t.Errorf("first week again = %v", got)
	}
	if n := f.srv.CountRequests("REPORT", f.col); n != reports {
		t.Errorf("REPORTs %d → %d: a week seen before was read again", reports, n)
	}

	f.srv.Seed(f.col+"c.ics", ics("BEGIN:VEVENT\nUID:c\nDTSTAMP:20260101T000000Z\nDTSTART:20260909T100000Z\nSUMMARY:Added elsewhere\nEND:VEVENT"))
	if got := weekSummaries(t, f, "2026-09-07"); strings.Join(got, "|") != "First week|Added elsewhere" {
		t.Errorf("first week after another client's write = %v", got)
	}
}
