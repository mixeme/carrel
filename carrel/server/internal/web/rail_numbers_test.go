package web_test

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// from-typecase note 15: the numbers and the navigation of three rails —
// the search (s-find-1), a person's timeline (s-contacts-7) and the
// conflict of every kind (s-conflict-1, -3, -4, -5) — read from the frames.

var (
	frameTickRow    = regexp.MustCompile(`(?s)<div class="((?:[^" ]+ )*tc-tickrow(?: [^"]*)?)">(.*?)</div>`)
	frameTickLabel  = regexp.MustCompile(`<span class="tc-tickrow__label">([^<]*)</span>`)
	frameTickMeta   = regexp.MustCompile(`<span class="tc-tickrow__meta"[^>]*>([^<]*)</span>`)
	frameSourcesRub = regexp.MustCompile(`<div class="tc-rubric">Sources <span[^>]*>(\d+) / (\d+)</span>`)
	frameNavNum     = regexp.MustCompile(`(?s)<span class="tc-num">.*?</span>`)
	frameTags       = regexp.MustCompile(`(?s)<[^>]+>`)
)

type frameTick struct {
	label, meta string
	feed, error bool
	retry       bool
	hint        string // the hint of the error mark (from-typecase note 32)
}

var frameTickErr = regexp.MustCompile(`<span class="tc-tickrow__err"[^>]* title="([^"]*)">`)

// frameTicks are the rows of a frame's rail, as it draws them.
func frameTicks(frame string) []frameTick {
	var out []frameTick
	for _, m := range frameTickRow.FindAllStringSubmatch(frame, -1) {
		lab := frameTickLabel.FindStringSubmatch(m[2])
		if lab == nil {
			continue
		}
		row := frameTick{label: lab[1], feed: strings.Contains(m[1], "is-ext"), error: strings.Contains(m[1], "is-error"),
			retry: strings.Contains(m[2], ">Retry<")}
		if meta := frameTickMeta.FindStringSubmatch(m[2]); meta != nil {
			row.meta = meta[1]
		}
		if hint := frameTickErr.FindStringSubmatch(m[2]); hint != nil {
			row.hint = frameText(hint[1])
		}
		out = append(out, row)
	}
	return out
}

// frameNav are the labels of a frame's navigation rows, in order.
func frameNav(frame string) []string {
	var out []string
	for _, m := range frameNavItem.FindAllStringSubmatch(frame, -1) {
		item := frameNavShort.ReplaceAllString(m[1], "")
		if lab := strings.TrimSpace(frameTags.ReplaceAllString(frameNavNum.ReplaceAllString(item, ""), "")); lab != "" {
			out = append(out, lab)
		}
	}
	return out
}

// s-find-1: every source of the search counts its finds, the one that did
// not answer is marked, its reason in the mark's hint — its Retry is the
// poll bar's (from-typecase note 32) —, and the rubric says how many sources
// were asked of how many are ticked. Until v0.2.59 the rail was built before
// the poll and carried no numbers and no troubles at all.
func TestSearchRailCountsWhatEachSourceFound(t *testing.T) {
	frame := mockupFrame(t, "s-find-1")
	ticks := frameTicks(frame)
	rub := frameSourcesRub.FindStringSubmatch(frame)
	if rub == nil {
		t.Fatal("s-find-1 has no «Sources a / b» rubric: read the frame again")
	}
	// The frame asks all its fourteen sources and one of them does not
	// answer — and the rubric still says 14 / 14: it counts what was asked
	// of what is ticked, not what answered (the poll bar says that).
	asked, _ := strconv.Atoi(rub[1])
	ticked, _ := strconv.Atoi(rub[2])
	var down, counted int
	for _, r := range ticks {
		switch {
		case r.error:
			down++
			if r.retry || r.meta != "" || r.hint == "" {
				t.Errorf("s-find-1: the row that did not answer is not a mark with a hint: %+v", r)
			}
		case regexp.MustCompile(`^\d+$`).MatchString(r.meta):
			counted++
		}
	}
	if asked != len(ticks) || ticked != len(ticks) || down != 1 || counted != len(ticks)-1 {
		t.Fatalf("s-find-1 reads otherwise: rubric %s / %s, %d rows, %d down, %d counted", rub[1], rub[2], len(ticks), down, counted)
	}

	good := searchServer(t)
	good.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	good.Seed(searchBook+"ada.vcf", adaCard())
	bad := searchServer(t)
	bad.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work"})
	f := connectFixture(t, good, searchBook, nil)
	if rec := f.h.do(postForm("/app/settings/connections", f.sess, url.Values{
		"server": {bad.URL}, "login": {bad.Login}, "password": {bad.Password},
	})); rec.Code != http.StatusSeeOther {
		t.Fatalf("second connection: %d %s", rec.Code, rec.Body.String())
	}
	tickAll(t, f.h, f.sess)
	bad.Refuse("REPORT", http.StatusForbidden, "<error/>")

	want := fmt.Sprintf("Sources %d / %d", 2, 2)
	empty := search(t, f.h, f.sess, "")
	if len(empty.Rail.Sections) != 1 || empty.Rail.Sections[0].Account != view.Text(want) {
		t.Fatalf("rail without a query = %+v, want the rubric %q", empty.Rail.Sections, want)
	}
	for _, r := range empty.Rail.Sections[0].Rows {
		if r.Count != nil || r.State == view.CollectionUnavailable {
			t.Errorf("without a query nothing was asked, yet %q = %+v", r.Label, r)
		}
	}

	page := search(t, f.h, f.sess, "q=Ada")
	waitPoll(t, f.h, f.sess, page.Poll.TaskID, nil)
	page = search(t, f.h, f.sess, "q=Ada")
	if len(page.Rail.Sections) != 1 || page.Rail.Sections[0].Account != view.Text(want) {
		t.Fatalf("rail = %+v, want the rubric %q", page.Rail.Sections, want)
	}
	_, people := railRow(t, page.Rail, "People")
	if people.Count == nil || *people.Count != 1 || people.Detail != "" || people.State != view.CollectionOK {
		t.Errorf("People found one: %+v", people)
	}
	_, work := railRow(t, page.Rail, "Work")
	if work.Count != nil || work.State != view.CollectionUnavailable || work.Detail == "" {
		t.Errorf("Work did not answer: %+v", work)
	}
	if retry := sourceNamed(t, page.Poll, "Work").Retry; retry == nil || !strings.Contains(retry.Href, "/app/search/progress/"+page.Poll.TaskID+"/retry?source=") {
		t.Errorf("Work is retried from the poll bar: %+v", retry)
	}
}

// s-contacts-7: the timeline's navigation is the sections in full, as on
// every screen of the section, and then the way back; a feed has «ro» in
// place of a number, as in the agenda, and is not in the combined row's sum.
func TestTimelineRailHasTheSectionsAndNoFeedNumber(t *testing.T) {
	frame := mockupFrame(t, "s-contacts-7")
	nav := frameNav(frame)
	if len(nav) < 2 || nav[len(nav)-1] != "Back to list" {
		t.Fatalf("s-contacts-7 nav = %q: read the frame again", nav)
	}
	var feeds int
	for _, r := range frameTicks(frame) {
		if r.feed {
			feeds++
			if r.meta != "ro" {
				t.Fatalf("s-contacts-7: feed %q shows %q", r.label, r.meta)
			}
		}
	}
	if feeds == 0 {
		t.Fatal("s-contacts-7 draws no feed: read the frame again")
	}
	// The combined row is the sum of the calendars' numbers — the feeds'
	// «ro» are not numbers (from-typecase note 18: 9 + 7 + 4 + 0 = 20).
	var sum, combined int
	for i, r := range frameTicks(frame) {
		n, err := strconv.Atoi(r.meta)
		switch {
		case i == 0:
			combined = n
		case err == nil && !r.feed:
			sum += n
		}
	}
	if combined == 0 || combined != sum {
		t.Fatalf("s-contacts-7: the combined row %d, the calendars sum to %d: read the frame again", combined, sum)
	}

	now := time.Now()
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.Seed(searchBook+"ann.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:ann\nFN:Anna Kovaleva\nEMAIL:anna@example.com\nEND:VCARD\n")
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work", SupportedComponentSet: []string{"VEVENT"}})
	srv.Seed(searchWork+"e1.ics", ics("BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260101T000000Z\nDTSTART:"+icsDay(now.AddDate(0, -1, 0))+
		"\nSUMMARY:Review with Anna\nATTENDEE:mailto:anna@example.com\nEND:VEVENT"))
	f := connectFixture(t, srv, searchBook, nil)
	feed := newFeedServer(t, "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//feed//EN\r\nX-WR-CALNAME:League\r\n"+
		"BEGIN:VEVENT\r\nUID:m1\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:Match with Anna\r\nDTSTART:"+icsDay(now.AddDate(0, -1, 0))+
		"\r\nATTENDEE:mailto:anna@example.com\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n", "")
	sub := subscribe(t, f, url.Values{"url": {feed.URL + "/league.ics"}})
	tickAll(t, f.h, f.sess)
	tickRail(t, f.h, f.sess, "calendar", "/app/calendar/"+f.id+"/"+url.PathEscape(searchWork), "/app/calendar/"+sub.ID+"/feed")
	card := f.path("contacts") + "/ann"

	page := decode[view.ContactTimelinePage](t, f.h.do(get(card+"/timeline", f.sess)))
	waitPoll(t, f.h, f.sess, page.Poll.TaskID, nil)
	page = decode[view.ContactTimelinePage](t, f.h.do(get(card+"/timeline", f.sess)))
	if _, ok := timelineRows(page)["Match with Anna"]; !ok {
		t.Fatalf("the feed was not looked in: %v", timelineRows(page))
	}

	var got []string
	for _, n := range page.Rail.Nav {
		got = append(got, string(n.Label))
	}
	if !equalStrings(got, nav) {
		t.Errorf("nav = %q, want %q", got, nav)
	}
	for _, n := range page.Rail.Nav {
		if n.On != (n.ID == "nav.contacts") {
			t.Errorf("nav %q on = %v: the open section is Contacts", n.Label, n.On)
		}
	}
	if back := page.Rail.Nav[len(page.Rail.Nav)-1]; back.ID != "nav.back" || back.Href != f.path("contacts") {
		t.Errorf("back = %+v", back)
	}
	sections := decode[view.ContactsListPage](t, f.h.do(get(f.path("contacts"), f.sess))).Rail.Nav
	if len(page.Rail.Nav) != len(sections)+1 {
		sections = nil // said above
	}
	for i, n := range sections {
		if page.Rail.Nav[i].Label != n.Label || page.Rail.Nav[i].Href != n.Href {
			t.Errorf("section %d = %+v, the list has %+v", i, page.Rail.Nav[i], n)
		}
	}

	_, league := railRow(t, page.Rail, "League")
	if league.Count != nil || league.Tick != view.TickOn {
		t.Errorf("the feed row = %+v, want no number", league)
	}
	_, work := railRow(t, page.Rail, "Work")
	_, all := railRow(t, page.Rail, "Combined calendar")
	if work.Count == nil || *work.Count != 1 || all.Count == nil || *all.Count != 1 {
		t.Errorf("Work %v, combined %v: the combined row sums the calendars, not the feed", work.Count, all.Count)
	}
}

// A ticked collection that is gone is not asked (S-6.1): the search's rail
// still lists it, marked gone and without a number, and it is not among the
// sources asked in «Sources a / b».
func TestSearchRailListsAGoneCollectionUnasked(t *testing.T) {
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.Seed(searchBook+"ada.vcf", adaCard())
	f := connectFixture(t, srv, searchBook, nil)
	tickAll(t, f.h, f.sess, sealed.Collection{AccountID: f.id, Path: teamBook, Name: "Team", Kinds: []view.Kind{view.KindContact}, Gone: true})

	page := search(t, f.h, f.sess, "q=Ada")
	waitPoll(t, f.h, f.sess, page.Poll.TaskID, nil)
	page = search(t, f.h, f.sess, "q=Ada")
	if len(page.Poll.Sources) != 1 {
		t.Errorf("sources asked = %+v, want People alone", page.Poll.Sources)
	}
	if len(page.Rail.Sections) != 1 || page.Rail.Sections[0].Account != "Sources 1 / 2" {
		t.Fatalf("rail = %+v, want the rubric «Sources 1 / 2»", page.Rail.Sections)
	}
	if _, team := railRow(t, page.Rail, "Team"); team.State != view.CollectionGone || team.Count != nil || team.Detail != "" {
		t.Errorf("gone Team = %+v", team)
	}
	if _, people := railRow(t, page.Rail, "People"); people.Count == nil || *people.Count != 1 {
		t.Errorf("People = %+v", people)
	}
}
