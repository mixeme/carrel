package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/internal/session"
	"carrel.dev/carrel/server/internal/store"
	"carrel.dev/carrel/server/view"
)

const (
	searchBook = davtest.AddressBookHome + "people/"
	searchWork = davtest.CalendarHome + "work/"
	searchDocs = davtest.FileRoot + "docs/"
)

func adaCard() string {
	return "BEGIN:VCARD\nVERSION:4.0\nUID:ada\nFN:Ada Lovelace\nEMAIL:ada@example.com\nEND:VCARD\n"
}

func searchServer(t *testing.T) *davtest.Server {
	t.Helper()
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	return srv
}

// tickAll marks every live collection for every kind it holds, the way the
// rail will (S-14 writes Selected; search only reads it). extra goes into the
// catalog first: discovery does not find a file root on a CalDAV server.
func tickAll(t *testing.T, h *harness, sess *session.Session, extra ...sealed.Collection) {
	t.Helper()
	key, _ := sess.DataKey()
	blob, ok := h.users.Catalog(sess.AccountID(), key)
	if !ok {
		t.Fatal("no catalog")
	}
	blob.Collections = append(blob.Collections, extra...)
	blob.Selected = nil
	for _, c := range blob.Collections {
		for _, k := range c.Kinds {
			blob.Selected = append(blob.Selected, sealed.Selection{Kind: k, AccountID: c.AccountID, Path: c.Path})
		}
	}
	sealedBlob, err := sealed.Seal(key, blob, sess.Login())
	if err != nil {
		t.Fatal(err)
	}
	if err := h.st.Mutate(func(st *store.State) ([]store.AuditEntry, error) {
		st.UserByID(sess.AccountID()).SealedBlob = sealedBlob
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
}

func asPoll(req *http.Request) *http.Request { return fragment(req, "poll") }

type pollFragment struct {
	Target string       `json:"target"`
	Body   view.PollBar `json:"body"`
}

func search(t *testing.T, h *harness, sess *session.Session, query string) view.SearchPage {
	t.Helper()
	return decode[view.SearchPage](t, h.do(get("/app/search?"+query, sess)))
}

func pollOf(t *testing.T, h *harness, sess *session.Session, task string) view.PollBar {
	t.Helper()
	return decode[pollFragment](t, h.do(asPoll(get("/app/search/progress/"+task+"/poll", sess)))).Body
}

// waitPoll reads the fallback route until the task is over or until until
// says the bar is what the test waits for.
func waitPoll(t *testing.T, h *harness, sess *session.Session, task string, until func(view.PollBar) bool) view.PollBar {
	t.Helper()
	if until == nil {
		until = func(b view.PollBar) bool { return !b.Running }
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		bar := pollOf(t, h, sess, task)
		if until(bar) {
			return bar
		}
		if time.Now().After(deadline) {
			t.Fatalf("poll never got there: %+v", bar)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func sourceNamed(t *testing.T, bar view.PollBar, name string) view.SourceProgress {
	t.Helper()
	for _, sp := range bar.Sources {
		if strings.HasSuffix(string(sp.Stamp), " · "+name) {
			return sp
		}
	}
	t.Fatalf("no source %q in %+v", name, bar.Sources)
	return view.SourceProgress{}
}

func action(bar view.PollBar, id string) view.Action {
	for _, a := range bar.Actions {
		if a.ID == id {
			return a
		}
	}
	return view.Action{}
}

// S-15: one query over the ticked contacts, calendar and files; the page does
// not wait; results keep their kinds' order and their origin; files match by
// name without a body read; a return opens from memory; kind and filter cut
// what arrived; "again" asks the servers anew; the history keeps one row.
func TestSearchAcrossTickedSources(t *testing.T) {
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work"})
	srv.Seed(searchBook+"ada.vcf", adaCard())
	srv.Seed(searchBook+"bob.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:bob\nFN:Bob Builder\nEND:VCARD\n")
	srv.Seed(searchWork+"call.ics", ics("BEGIN:VEVENT\nUID:call\nDTSTAMP:20260101T000000Z\nDTSTART:20260910T150000Z\nSUMMARY:Call with Ada\nLOCATION:Zoom\nEND:VEVENT"))
	srv.Seed(searchWork+"early.ics", ics("BEGIN:VEVENT\nUID:early\nDTSTAMP:20260101T000000Z\nDTSTART:20260901T090000Z\nSUMMARY:Ada at the office\nEND:VEVENT"))
	srv.Seed(searchWork+"other.ics", ics("BEGIN:VEVENT\nUID:other\nDTSTAMP:20260101T000000Z\nDTSTART:20260902T090000Z\nSUMMARY:Standup\nEND:VEVENT"))
	srv.Seed(searchWork+"todo.ics", ics("BEGIN:VTODO\nUID:todo\nDTSTAMP:20260101T000000Z\nDUE:20260912T100000Z\nSUMMARY:Write to Ada\nEND:VTODO"))
	srv.Seed(searchWork+"note.ics", ics("BEGIN:VJOURNAL\nUID:note\nDTSTAMP:20260101T000000Z\nDTSTART:20260905T100000Z\nSUMMARY:Ada notes\nDESCRIPTION:Talked about engines\nEND:VJOURNAL"))
	srv.SeedFile(searchDocs+"Ada contract.pdf", "0123456789")
	srv.SeedFile(searchDocs+"old/Ada.md", "x")
	srv.SeedFile(searchDocs+"other.txt", "y")

	f := connectFixture(t, srv, searchBook, nil)
	tickAll(t, f.h, f.sess, sealed.Collection{AccountID: f.id, Path: searchDocs, Name: "Docs", Kinds: []view.Kind{view.KindFile}})

	page := search(t, f.h, f.sess, "q=Ada")
	task := page.Poll.TaskID
	if task == "" || page.Query != "Ada" || page.Top.Search.Query != "Ada" || len(page.Poll.Sources) != 3 {
		t.Fatalf("page = %+v", page.Poll)
	}
	bar := waitPoll(t, f.h, f.sess, task, nil)
	for _, name := range []string{"People", "Work", "Docs"} {
		if sp := sourceNamed(t, bar, name); sp.State != view.SourceDone || sp.Count == nil || sp.Retry != nil {
			t.Errorf("%s = %+v", name, sp)
		}
	}
	if bar.Percent != 100 || bar.AnsweredAt == "" || !strings.Contains(saidRuns(bar.Summary), "3 of 3 sources answered · 7 found") {
		t.Errorf("bar = %+v", bar)
	}
	if action(bar, "poll.stop").Enabled || !action(bar, "poll.again").Enabled {
		t.Errorf("actions = %+v", bar.Actions)
	}
	if n := srv.CountRequests("GET", davtest.FileRoot); n != 0 {
		t.Errorf("GET under the file root = %d: a file body was read", n)
	}

	asked := len(srv.Requests())
	page = search(t, f.h, f.sess, "q=Ada")
	if len(srv.Requests()) != asked || page.Poll.TaskID != task || page.Poll.AnsweredAt == "" {
		t.Fatalf("return to the result asked the server again or lost the task: %d → %d", asked, len(srv.Requests()))
	}
	var got []string
	for _, row := range page.Results {
		got = append(got, string(row.Kind)+":"+string(row.Title))
	}
	want := "contact:Ada Lovelace,event:Ada at the office,event:Call with Ada,task:Write to Ada,note:Ada notes,file:Ada contract.pdf,file:Ada.md"
	if strings.Join(got, ",") != want {
		t.Fatalf("results\n got %s\nwant %s", strings.Join(got, ","), want)
	}
	contact, event, doc := page.Results[0], page.Results[2], page.Results[5]
	if contact.Href != "/app/contacts/"+f.id+"/"+url.PathEscape(searchBook)+"/ada" || !strings.HasSuffix(string(contact.Stamp), " · People") ||
		contact.Detail != "ada@example.com" || contact.Colour == "" {
		t.Errorf("contact = %+v", contact)
	}
	if event.Detail != "Zoom" || event.When == "" || !strings.HasSuffix(string(event.Stamp), " · Work") {
		t.Errorf("event = %+v", event)
	}
	if doc.Detail != "10 B" || doc.Href != "/app/files/"+f.id+"/"+url.PathEscape(searchDocs)+"/Ada%20contract.pdf" {
		t.Errorf("file = %+v", doc)
	}
	if page.Results[6].Href != "/app/files/"+f.id+"/"+url.PathEscape(searchDocs)+"/old/Ada.md" {
		t.Errorf("deep file = %+v", page.Results[6])
	}
	if !strings.Contains(string(page.Note), "by name") {
		t.Errorf("note = %q", page.Note)
	}
	if page.Bar.Total == nil || *page.Bar.Total != 7 || len(page.Bar.Segments) != 6 {
		t.Errorf("bar = %+v", page.Bar)
	}
	// s-find-1 writes the kinds without counts.
	sameSegments(t, "search", page.Bar.Segments, "s-find-1", 0)
	if h := page.Top.Search.History; len(h) != 1 || h[0].Query != "Ada" || h[0].Outcome != "7 found" || h[0].Href != "/app/search?q=Ada" || h[0].When == "" {
		t.Errorf("history = %+v", h)
	}

	files := search(t, f.h, f.sess, "q=Ada&kind=file")
	calls := search(t, f.h, f.sess, "q=Ada&filter=zoom")
	if len(files.Results) != 2 || files.Bar.Loaded != 2 || len(calls.Results) != 1 || calls.Results[0].Title != "Call with Ada" {
		t.Errorf("kind %d, filter %+v", len(files.Results), calls.Results)
	}
	if len(srv.Requests()) != asked {
		t.Error("a segment or the filter asked the server")
	}

	again := search(t, f.h, f.sess, "q=Ada&again=1")
	if again.Poll.TaskID == "" || again.Poll.TaskID == task {
		t.Fatalf("again kept the task: %q", again.Poll.TaskID)
	}
	waitPoll(t, f.h, f.sess, again.Poll.TaskID, nil)
	if len(srv.Requests()) == asked {
		t.Error("again did not ask the servers")
	}
	if h := search(t, f.h, f.sess, "q=Ada").Top.Search.History; len(h) != 1 {
		t.Errorf("a repeat added a row: %+v", h)
	}

	// Hints come from what the session already loaded: no search starts
	// for them, and a record list opened before is where they look.
	f.h.do(get(f.path("contacts"), f.sess))
	hint := search(t, f.h, f.sess, "q=Lovelace")
	var hinted bool
	for _, j := range hint.Top.Search.Jump {
		hinted = hinted || (j.Title == "Ada Lovelace" && j.Kind == view.KindContact && j.Href == contact.Href)
	}
	if !hinted {
		t.Errorf("jump = %+v", hint.Top.Search.Jump)
	}
	waitPoll(t, f.h, f.sess, hint.Poll.TaskID, nil)
}

// docs/servers.md, SV-23: a server with no addressbook-query at all — Yandex
// answers 403 with the supported-report precondition, and 400 to a plain one.
// The book is listed and fetched by multiget instead, so the cards arrive
// whole; the searching is then ours, and the source says so rather than the
// whole book falling over, which is what used to happen.
func TestSearchFetchesWhatAServerCannotQuery(t *testing.T) {
	srv := searchServer(t)
	srv.RefuseCardQuery = true
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.Seed(searchBook+"ada.vcf", adaCard())
	srv.Seed(searchBook+"bob.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:bob\nFN:Bob Builder\nEND:VCARD\n")

	f := connectFixture(t, srv, searchBook, nil)
	tickAll(t, f.h, f.sess)
	page := search(t, f.h, f.sess, "q=Ada")
	bar := waitPoll(t, f.h, f.sess, page.Poll.TaskID, nil)
	if src := sourceNamed(t, bar, "People"); !strings.Contains(string(src.Message), "sifted") {
		t.Errorf("the source does not say the server could not search: %+v", src)
	}
	page = search(t, f.h, f.sess, "q=Ada")
	if len(page.Results) != 1 || string(page.Results[0].Title) != "Ada Lovelace" {
		t.Fatalf("results = %+v: only the card that holds the text stays", page.Results)
	}
}

// docs/servers.md, SV-26: a server whose addressbook-query answers nothing
// about a book that holds cards — Google. The empty answer is checked against
// the book, the cards are read the round-about way, and the source says the
// searching was ours.
func TestSearchLooksAgainWhenAQueryAnswersNothing(t *testing.T) {
	srv := searchServer(t)
	srv.HollowCardQuery = true
	srv.RefuseAllPropInAddressData = true
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.Seed(searchBook+"ada.vcf", adaCard())
	srv.Seed(searchBook+"bob.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:bob\nFN:Bob Builder\nEND:VCARD\n")

	f := connectFixture(t, srv, searchBook, nil)
	tickAll(t, f.h, f.sess)
	page := search(t, f.h, f.sess, "q=Ada")
	bar := waitPoll(t, f.h, f.sess, page.Poll.TaskID, nil)
	if src := sourceNamed(t, bar, "People"); !strings.Contains(string(src.Message), "sifted") {
		t.Errorf("the source does not say the searching was ours: %+v", src)
	}
	page = search(t, f.h, f.sess, "q=Ada")
	if len(page.Results) != 1 || string(page.Results[0].Title) != "Ada Lovelace" {
		t.Fatalf("results = %+v", page.Results)
	}
}

// docs/servers.md, SV-16: a server that answers a text-match with the whole
// collection has not searched at all. Carrel sifts what came back through the
// properties it asked about, and the source says so beside the results — the
// promise of S-15 is that what is shown holds the text.
func TestSearchSiftsWhatAServerDoesNotFilter(t *testing.T) {
	srv := searchServer(t)
	srv.SearchIgnoresFilter = true
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work"})
	srv.Seed(searchWork+"call.ics", ics("BEGIN:VEVENT\nUID:call\nDTSTAMP:20260101T000000Z\nDTSTART:20260910T150000Z\nSUMMARY:Call with Ada\nEND:VEVENT"))
	srv.Seed(searchWork+"other.ics", ics("BEGIN:VEVENT\nUID:other\nDTSTAMP:20260101T000000Z\nDTSTART:20260902T090000Z\nSUMMARY:Standup\nEND:VEVENT"))
	srv.Seed(searchWork+"third.ics", ics("BEGIN:VEVENT\nUID:third\nDTSTAMP:20260101T000000Z\nDTSTART:20260903T090000Z\nSUMMARY:Retro\nEND:VEVENT"))

	f := connectFixture(t, srv, searchWork, nil)
	tickAll(t, f.h, f.sess)
	page := search(t, f.h, f.sess, "q=Ada")
	bar := waitPoll(t, f.h, f.sess, page.Poll.TaskID, nil)
	if work := sourceNamed(t, bar, "Work"); !strings.Contains(string(work.Message), "sifted") {
		t.Errorf("the source does not say the server did not search: %+v", work)
	}
	page = search(t, f.h, f.sess, "q=Ada")
	if len(page.Results) != 1 || string(page.Results[0].Title) != "Call with Ada" {
		t.Fatalf("results = %+v: only what holds the text stays", page.Results)
	}
}

// S-15.2: a calendar server that refuses i;unicode-casemap is asked with its
// default. The source answers with a note that case counts outside Latin, and
// the note on events does not stop the tasks of the same collection.
func TestSearchNotesLatinOnlyCase(t *testing.T) {
	srv := searchServer(t)
	srv.RefuseCollation = true
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work"})
	srv.Seed(searchBook+"ada.vcf", adaCard())
	srv.Seed(searchWork+"call.ics", ics("BEGIN:VEVENT\nUID:call\nDTSTAMP:20260101T000000Z\nDTSTART:20260910T150000Z\nSUMMARY:Call with Ada\nEND:VEVENT"))
	srv.Seed(searchWork+"todo.ics", ics("BEGIN:VTODO\nUID:todo\nDTSTAMP:20260101T000000Z\nDUE:20260912T100000Z\nSUMMARY:Write to Ada\nEND:VTODO"))

	f := connectFixture(t, srv, searchBook, nil)
	tickAll(t, f.h, f.sess)
	page := search(t, f.h, f.sess, "q=Ada")
	bar := waitPoll(t, f.h, f.sess, page.Poll.TaskID, nil)
	if work := sourceNamed(t, bar, "Work"); work.State != view.SourceDone || !strings.Contains(string(work.Message), "Latin script only") {
		t.Errorf("work = %+v", work)
	}
	if people := sourceNamed(t, bar, "People"); people.State != view.SourceDone || people.Message != "" {
		t.Errorf("people = %+v", people)
	}
	var got []string
	for _, row := range search(t, f.h, f.sess, "q=Ada").Results {
		got = append(got, string(row.Kind)+":"+string(row.Title))
	}
	if want := "contact:Ada Lovelace,event:Call with Ada,task:Write to Ada"; strings.Join(got, ",") != want {
		t.Errorf("results\n got %s\nwant %s", strings.Join(got, ","), want)
	}
}

// S-15.1 and S-21: a refusal is a state on the screen, the other source's
// answer stays, and a retry asks the refused source and nobody else.
func TestSearchRefusalRetriesOneSource(t *testing.T) {
	good := searchServer(t)
	good.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	good.Seed(searchBook+"ada.vcf", adaCard())
	bad := searchServer(t)
	bad.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work"})
	bad.Seed(searchWork+"call.ics", ics("BEGIN:VEVENT\nUID:call\nDTSTAMP:20260101T000000Z\nDTSTART:20260910T150000Z\nSUMMARY:Call with Ada\nEND:VEVENT"))

	f := connectFixture(t, good, searchBook, nil)
	if rec := f.h.do(postForm("/app/settings/connections", f.sess, url.Values{
		"server": {bad.URL}, "login": {bad.Login}, "password": {bad.Password},
	})); rec.Code != http.StatusSeeOther {
		t.Fatalf("second connection: %d %s", rec.Code, rec.Body.String())
	}
	tickAll(t, f.h, f.sess)
	bad.Refuse("REPORT", http.StatusForbidden, "<error/>")

	page := search(t, f.h, f.sess, "q=Ada")
	bar := waitPoll(t, f.h, f.sess, page.Poll.TaskID, nil)
	people, work := sourceNamed(t, bar, "People"), sourceNamed(t, bar, "Work")
	if people.State != view.SourceDone || people.Count == nil || *people.Count != 1 {
		t.Errorf("people = %+v", people)
	}
	if work.State != view.SourceError || work.Retry == nil || work.Count != nil || !strings.Contains(string(work.Message), "403") {
		t.Fatalf("work = %+v", work)
	}
	if !strings.Contains(saidRuns(bar.Summary), "1 of 2 sources answered · 1 found · 1 refused") {
		t.Errorf("summary = %q", bar.Summary)
	}
	if h := search(t, f.h, f.sess, "q=Ada"); len(h.Results) != 1 || h.Top.Search.History[0].Outcome != "1 found" {
		t.Errorf("results %+v, history %+v", h.Results, h.Top.Search.History)
	}

	goodReports := good.CountRequests("REPORT", "")
	bad.Accept("REPORT")
	rec := f.h.do(asPoll(postForm(work.Retry.Href, f.sess, url.Values{})))
	if p := decode[pollFragment](t, rec); p.Target != "poll" || p.Body.TaskID != page.Poll.TaskID {
		t.Fatalf("retry answered %+v", p)
	}
	bar = waitPoll(t, f.h, f.sess, page.Poll.TaskID, nil)
	if sourceNamed(t, bar, "Work").State != view.SourceDone || good.CountRequests("REPORT", "") != goodReports {
		t.Fatalf("after retry: %+v; People asked %d → %d", bar.Sources, goodReports, good.CountRequests("REPORT", ""))
	}
	after := search(t, f.h, f.sess, "q=Ada")
	if len(after.Results) != 2 || len(after.Top.Search.History) != 1 || after.Top.Search.History[0].Outcome != "2 found" {
		t.Errorf("results %d, history %+v", len(after.Results), after.Top.Search.History)
	}
}

// S-15.1: Stop keeps what arrived and marks the rest; the ceiling ends a
// poll that a server never answers; neither leaves an endless indicator.
func TestSearchStopAndCeiling(t *testing.T) {
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.Seed(searchBook+"ada.vcf", adaCard())
	srv.SeedFile(searchDocs+"Ada.md", "x")
	f := connectFixture(t, srv, searchBook, func(h *harness) { h.cfg.ProgressTotalTimeout = 300 * time.Millisecond })
	tickAll(t, f.h, f.sess, sealed.Collection{AccountID: f.id, Path: searchDocs, Name: "Docs", Kinds: []view.Kind{view.KindFile}})
	release := srv.Stall("PROPFIND")
	defer release()

	page := search(t, f.h, f.sess, "q=Ada")
	bar := waitPoll(t, f.h, f.sess, page.Poll.TaskID, func(b view.PollBar) bool {
		return sourceNamed(t, b, "People").State == view.SourceDone
	})
	if !bar.Running || !action(bar, "poll.stop").Enabled {
		t.Fatalf("docs is stalled, the poll should run: %+v", bar)
	}
	stop := decode[pollFragment](t, f.h.do(asPoll(postForm(action(bar, "poll.stop").Href, f.sess, url.Values{})))).Body
	docs := sourceNamed(t, stop, "Docs")
	if stop.Running || docs.State != view.SourceCancelled || docs.Retry == nil || !strings.Contains(saidRuns(stop.Summary), "1 stopped") {
		t.Fatalf("after stop: %+v", stop)
	}
	kept := search(t, f.h, f.sess, "q=Ada")
	if len(kept.Results) != 1 || kept.Results[0].Title != "Ada Lovelace" {
		t.Errorf("stop lost what arrived: %+v", kept.Results)
	}

	again := search(t, f.h, f.sess, "q=Ada&again=1")
	bar = waitPoll(t, f.h, f.sess, again.Poll.TaskID, nil)
	if sourceNamed(t, bar, "Docs").State != view.SourceTimeout || !strings.Contains(saidRuns(bar.Summary), "1 timed out") {
		t.Fatalf("after the ceiling: %+v", bar)
	}
	if h := search(t, f.h, f.sess, "q=Ada").Top.Search.History; len(h) != 1 || h[0].Outcome != "1 found" {
		t.Errorf("history = %+v", h)
	}
	// s-find-1: the rail's row of a source that did not answer says so in
	// the frame's words, in the hint of its mark (from-typecase notes 18,
	// 32) — the search has no list read before to fall back on.
	noReply := regexp.MustCompile(`<span class="tc-tickrow__err" role="img" aria-label="([^"]*)" title="([^"]*)">`).FindStringSubmatch(mockupFrame(t, "s-find-1"))
	if noReply == nil || noReply[1] != noReply[2] {
		t.Fatal("s-find-1 draws no mark of a source that did not answer: read the frame again")
	}
	if _, docs := railRow(t, search(t, f.h, f.sess, "q=Ada").Rail, "Docs"); docs.State != view.CollectionUnavailable || string(docs.Detail) != frameText(noReply[2]) {
		t.Errorf("Docs in the rail = %s «%s», the frame draws «%s»", docs.State, docs.Detail, frameText(noReply[2]))
	}
}

// S-15.3 and C-8.1: no query, no ticks and no free slot each start nothing
// and ask no server.
func TestSearchStartsOnlyWithTextTicksAndASlot(t *testing.T) {
	f := contactsFixture(t, false, adaCard())
	asked := len(f.srv.Requests())

	if page := search(t, f.h, f.sess, "q=%20%20"); page.Poll.TaskID != "" || page.Query != "" {
		t.Errorf("an empty query started %+v", page.Poll)
	}
	if page := search(t, f.h, f.sess, "q=Ada"); page.Poll.TaskID != "" || !strings.Contains(saidRuns(page.Poll.Summary), "No collections are ticked") {
		t.Errorf("no ticks: %+v", page.Poll)
	}
	tickAll(t, f.h, f.sess)
	for i := 0; i < 4; i++ {
		if !f.sess.AcquireFanout() {
			t.Fatal("slot")
		}
	}
	if page := search(t, f.h, f.sess, "q=Ada"); page.Poll.TaskID != "" || !hasNotice(page.Notices, "Too many searches") {
		t.Errorf("no slot: %+v %+v", page.Poll, page.Notices)
	}
	if len(f.srv.Requests()) != asked {
		t.Error("a search that did not start asked the server")
	}
	if h := search(t, f.h, f.sess, "q=Ada").Top.Search.History; len(h) != 0 {
		t.Errorf("history = %+v", h)
	}
	for i := 0; i < 4; i++ {
		f.sess.ReleaseFanout()
	}
	page := search(t, f.h, f.sess, "q=Ada")
	if page.Poll.TaskID == "" {
		t.Fatal("a freed slot did not start the search")
	}
	waitPoll(t, f.h, f.sess, page.Poll.TaskID, nil)
}

// C-5.3: the stream says it is alive before it waits, sends the bar and
// poll-finished; a task answers its own session only; poll mode has no stream.
func TestSearchProgressStream(t *testing.T) {
	f := contactsFixture(t, false, adaCard())
	tickAll(t, f.h, f.sess)
	task := search(t, f.h, f.sess, "q=Ada").Poll.TaskID

	rec := f.h.do(get("/app/search/progress/"+task, f.sess))
	body := rec.Body.String()
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "text/event-stream" || !strings.HasPrefix(body, ": ping\n\n") ||
		!strings.Contains(body, "event: poll\ndata: {\"target\":\"poll\"") || !strings.Contains(body, "event: poll-finished\n") {
		t.Fatalf("stream %d %q:\n%s", rec.Code, rec.Header().Get("Content-Type"), body)
	}
	// An event goes past the page's stamping: its Stop is a POST with the
	// session's token, aimed at the bar (C-11, C-7.2) all the same.
	if !strings.Contains(body, `"id":"poll.stop"`) || !strings.Contains(body, `"csrf":"`+f.sess.CSRFToken()+`"`) || !strings.Contains(body, `"target":"poll"}`) {
		t.Errorf("the bar of the stream carries no token or no target:\n%s", body)
	}

	other := f.h.login("owner", "password123")
	for _, req := range []*http.Request{
		get("/app/search/progress/"+task, other),
		asPoll(get("/app/search/progress/"+task+"/poll", other)),
		asPoll(postForm("/app/search/progress/"+task+"/cancel", other, url.Values{})),
		asPoll(postForm("/app/search/progress/"+task+"/retry?source=x", other, url.Values{})),
		asPoll(get("/app/search/progress/t-NOTISSUED0/poll", f.sess)),
	} {
		if rec := f.h.do(req); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s from another session: %d", req.Method, req.URL, rec.Code)
		}
	}

	f.h.cfg.ProgressMode = "poll"
	f.h.rebuild()
	if rec := f.h.do(get("/app/search/progress/"+task, f.sess)); rec.Code != http.StatusNotFound {
		t.Errorf("poll mode stream: %d", rec.Code)
	}
	if bar := pollOf(t, f.h, f.sess, task); bar.TaskID != task || bar.Running {
		t.Errorf("poll mode bar = %+v", bar)
	}
}
