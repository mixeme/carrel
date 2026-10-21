package web_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/emersion/go-webdav/caldav"

	"carrel.dev/carrel/server/internal/davtest"
	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
)

// feedServer is somebody else's feed: one .ics at an address, optionally
// behind a password, and a log of every method it was asked.
type feedServer struct {
	*httptest.Server
	mu      sync.Mutex
	body    string
	login   string
	methods []string
}

const holidays = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//feed//EN\r\nX-WR-CALNAME:Праздники\r\n" +
	"BEGIN:VEVENT\r\nUID:h1\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:Day of Knowledge\r\nDTSTART;VALUE=DATE:20260901\r\nEND:VEVENT\r\n" +
	"BEGIN:VTODO\r\nUID:t1\r\nDTSTAMP:20260801T000000Z\r\nSUMMARY:Buy flowers\r\nEND:VTODO\r\n" +
	"END:VCALENDAR\r\n"

func newFeedServer(t *testing.T, body, login string) *feedServer {
	t.Helper()
	f := &feedServer{body: body, login: login}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.methods = append(f.methods, r.Method)
		body, login := f.body, f.login
		f.mu.Unlock()
		if u, p, _ := r.BasicAuth(); login != "" && (u != login || p != "pw") {
			w.Header().Set("WWW-Authenticate", `Basic realm="feed"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *feedServer) set(body string) { f.mu.Lock(); f.body = body; f.mu.Unlock() }

func (f *feedServer) writes() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, m := range f.methods {
		if m != http.MethodGet {
			n++
		}
	}
	return n
}

func feedHarness(t *testing.T) davFixture {
	t.Helper()
	srv := davtest.New(t)
	srv.Login, srv.Password = "alice", "secret"
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work"})
	return connectFixture(t, srv, searchWork, nil)
}

func subscribe(t *testing.T, f davFixture, form url.Values) sealed.Subscription {
	t.Helper()
	rec := f.h.do(postForm("/app/settings/connections?op=feed.subscribe", f.sess, form))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("subscribe: %d %s", rec.Code, rec.Body.String())
	}
	key, _ := f.sess.DataKey()
	blob, _ := f.h.users.Catalog(f.sess.AccountID(), key)
	if len(blob.Subscriptions) != 1 {
		t.Fatalf("subscriptions = %+v", blob.Subscriptions)
	}
	return blob.Subscriptions[0]
}

// S-25.2: checking the address is a trace and saves nothing; subscribing
// keeps the address, the name the feed gives and the kinds it carries —
// never a record — and the card is on the connections page.
func TestSubscribeChecksThenKeepsNoRecords(t *testing.T) {
	f := feedHarness(t)
	feed := newFeedServer(t, holidays, "")
	form := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections?op=feed", f.sess)))
	// Check, Subscribe and Cancel (s-profile-10, Нс27).
	if len(form.Add.Actions) != 3 || actionByID(form.Add.Actions, "feed.cancel") == nil {
		t.Fatalf("subscribe form = %+v", form.Add)
	}
	checked := decode[view.ConnectionsPage](t, f.h.do(postForm("/app/settings/connections?op=feed.check", f.sess, url.Values{"url": {feed.URL + "/ru.ics"}})))
	// What the check found stands in the panel (s-profile-10).
	if checked.Trace == nil || !checked.Trace.OK || len(checked.Trace.Steps) < 3 || checked.Trace.Steps[0].Name != "fetch" {
		t.Fatalf("check = %+v", checked.Trace)
	}
	key, _ := f.sess.DataKey()
	if blob, _ := f.h.users.Catalog(f.sess.AccountID(), key); len(blob.Subscriptions) != 0 {
		t.Fatal("checking subscribed")
	}

	sub := subscribe(t, f, url.Values{"url": {feed.URL + "/ru.ics"}})
	if sub.Name != "Праздники" || len(sub.Kinds) != 2 || sub.Kinds[0] != view.KindEvent || sub.Kinds[1] != view.KindTask {
		t.Errorf("subscription = %+v", sub)
	}
	state := f.h.st.Snapshot()
	for _, u := range state.Users {
		if strings.Contains(string(u.SealedBlob), "Day of Knowledge") {
			t.Error("a record of the feed is kept")
		}
	}
	page := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
	if len(page.Subscriptions) != 1 || page.Subscriptions[0].Title != "Праздники" {
		t.Errorf("cards = %+v", page.Subscriptions)
	}
}

// The feed reads like a collection that cannot be written: its list and
// records are there, live on every visit; nothing that would write opens; it
// is in the rail and in the combined view when ticked.
func TestFeedIsAReadOnlyCollection(t *testing.T) {
	f := feedHarness(t)
	feed := newFeedServer(t, holidays, "")
	sub := subscribe(t, f, url.Values{"url": {feed.URL + "/ru.ics"}})
	list := "/app/calendar/" + sub.ID + "/feed"
	agenda := decode[view.AgendaPage](t, f.h.do(get(list+"?from=2026-08-31", f.sess)))
	if !strings.Contains(strings.Join(agendaLines(agenda), "|"), "Day of Knowledge") {
		t.Fatalf("feed agenda = %v", agendaLines(agenda))
	}
	for _, a := range agenda.Bar.Actions {
		if a.ID == "publish" || (a.ID == "record.new" && a.Enabled) {
			t.Errorf("a feed offers %+v", a)
		}
	}
	tasks := decode[view.TasksPage](t, f.h.do(get("/app/tasks/"+sub.ID+"/feed?show=all", f.sess)))
	if tasks.Bar.Loaded != 1 {
		t.Errorf("feed tasks = %+v", tasks.Groups)
	}
	card := decode[view.EventPage](t, f.h.do(get(list+"/h1", f.sess)))
	if card.Summary != "Day of Knowledge" || !card.ReadOnly {
		t.Errorf("feed record = %+v", card)
	}
	for _, req := range []*http.Request{
		get(list+"/h1/edit", f.sess), get(list+"/new", f.sess), get(list+"/import", f.sess),
		postForm(list+"/h1/delete", f.sess, url.Values{}), postForm(list+"/h1/edit", f.sess, url.Values{}),
	} {
		if rec := f.h.do(req); rec.Code != http.StatusNotFound {
			t.Errorf("%s %s on a feed: %d", req.Method, req.URL.Path, rec.Code)
		}
	}
	if rec := f.h.do(get(list+"/h1/move?copy=1", f.sess)); rec.Code != http.StatusOK {
		t.Errorf("copying out of a feed: %d", rec.Code)
	}

	feed.set(strings.Replace(holidays, "Day of Knowledge", "Knowledge Day", 1))
	again := decode[view.AgendaPage](t, f.h.do(get(list+"?from=2026-08-31", f.sess)))
	if !strings.Contains(strings.Join(agendaLines(again), "|"), "Knowledge Day") {
		t.Error("the feed was not read again")
	}

	tickRail(t, f.h, f.sess, "calendar", list)
	rail := decode[view.AgendaPage](t, f.h.do(get("/app/calendar", f.sess))).Rail
	sec, row := railRow(t, rail, "Праздники")
	if sec.Account != "Subscriptions" || row.Tick != view.TickOn || !row.ReadOnly {
		t.Errorf("rail row = %+v in %+v", row, sec)
	}
	all := combined(t, f.h, f.sess, "/app/calendar?from=2026-08-31", func(p view.AgendaPage) *view.PollBar { return p.Poll })
	if !strings.Contains(strings.Join(agendaLines(all), "|"), "Knowledge Day") {
		t.Errorf("the combined calendar lacks the feed: %v", agendaLines(all))
	}
	// A feed's count stands where a count stands, and «ro» apart from it
	// (Р-34 of Typecase, 80d1319: s-agenda-1 draws «0» and «ro»): read, the
	// feed says how many events it holds; it is not in the section's sum.
	if _, row := railRow(t, all.Rail, "Праздники"); row.Count == nil || *row.Count != 1 || !row.ReadOnly || !row.Feed {
		t.Errorf("the feed's row after its read = %+v, want its one event counted", row)
	}
	if _, row := railRow(t, all.Rail, "Combined calendar"); row.Count != nil && *row.Count != 0 {
		t.Errorf("the combined row = %+v: the feed is not in its sum", row)
	}
	if n := feed.writes(); n != 0 {
		t.Errorf("the feed was asked to write %d times", n)
	}
}

// A feed behind a password: the wrong one is a trace that says so and saves
// nothing; the right one is kept sealed with the blob. Renaming stays here;
// unsubscribing takes the subscription and its ticks away, and nothing else.
func TestFeedPasswordRenameUnsubscribe(t *testing.T) {
	f := feedHarness(t)
	feed := newFeedServer(t, holidays, "ann")
	bad := decode[view.ConnectionsPage](t, f.h.do(postForm("/app/settings/connections?op=feed.subscribe", f.sess, url.Values{
		"url": {feed.URL + "/league.ics"}, "login": {"ann"}, "password": {"nope"},
	})))
	if !hasNotice(bad.Notices, "The feed could not be read") {
		t.Fatalf("wrong password = %+v", bad.Notices)
	}
	sub := subscribe(t, f, url.Values{"url": {feed.URL + "/league.ics"}, "login": {"ann"}, "password": {"pw"}})
	if sub.Password != "pw" || sub.Name != "Праздники" {
		t.Errorf("subscription = %+v", sub)
	}

	rename := "/app/collections/rename?" + url.Values{"a": {sub.ID}, "c": {"feed"}}.Encode()
	sheet := decode[view.CollectionRenameSheet](t, f.h.do(get(rename, f.sess)))
	if sheet.Stamp != "" {
		t.Errorf("a feed's sheet names a server: %+v", sheet)
	}
	before := len(feed.methods)
	if rec := f.h.do(postForm(rename, f.sess, url.Values{"name": {"League"}})); rec.Code != http.StatusSeeOther {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body.String())
	}
	if len(feed.methods) != before {
		t.Error("renaming asked the feed")
	}
	for _, p := range []string{"/app/collections/delete?", "/app/collections/move?"} {
		if rec := f.h.do(get(p+url.Values{"a": {sub.ID}, "c": {"feed"}}.Encode(), f.sess)); rec.Code != http.StatusNotFound {
			t.Errorf("%s on a feed: %d", p, rec.Code)
		}
	}

	tickRail(t, f.h, f.sess, "calendar", "/app/calendar/"+sub.ID+"/feed")
	page := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
	if page.Subscriptions[0].Title != "League" {
		t.Errorf("renamed card = %+v", page.Subscriptions[0])
	}
	if rec := f.h.do(postForm(actionHref(t, page.Subscriptions[0].Actions, "feed.unsubscribe"), f.sess, url.Values{})); rec.Code != http.StatusSeeOther {
		t.Fatalf("unsubscribe: %d", rec.Code)
	}
	key, _ := f.sess.DataKey()
	blob, _ := f.h.users.Catalog(f.sess.AccountID(), key)
	if len(blob.Subscriptions) != 0 || len(selected(t, f.h, f.sess, view.KindEvent)) != 0 {
		t.Errorf("after unsubscribing: %+v ticks %v", blob.Subscriptions, selected(t, f.h, f.sess, view.KindEvent))
	}
	if n := feed.writes(); n != 0 {
		t.Errorf("unsubscribing wrote to the feed %d times", n)
	}
}

// Search reaches a ticked feed: a feed has no server to search, so Carrel
// sifts what it read, folded the same way as everywhere (S-15, S-25.2).
func TestSearchSiftsAFeed(t *testing.T) {
	f := feedHarness(t)
	feed := newFeedServer(t, holidays, "")
	sub := subscribe(t, f, url.Values{"url": {feed.URL + "/ru.ics"}})
	tickRail(t, f.h, f.sess, "calendar", "/app/calendar/"+sub.ID+"/feed")
	page := search(t, f.h, f.sess, "q=KNOWLEDGE")
	bar := waitPoll(t, f.h, f.sess, page.Poll.TaskID, nil)
	sp := sourceNamed(t, bar, "Праздники")
	if sp.State != view.SourceDone || sp.Count == nil || *sp.Count != 1 {
		t.Errorf("feed source = %+v", sp)
	}
	// In the search's rail a feed counts what was found in it (s-find-1):
	// «ro» stands only where the number would be what it holds.
	if _, row := railRow(t, search(t, f.h, f.sess, "q=KNOWLEDGE").Rail, "Праздники"); row.Count == nil || *row.Count != 1 {
		t.Errorf("feed in the search's rail = %+v", row)
	}
}

// The line of a subscription card (s-profile-1, -10): «… · read 2 min ago ·
// 34 KB · 0.4 s» — when the feed was last read, what that read weighed and
// how long it took. Nothing read in this session, nothing said; a read of
// the feed's list says all three, and the next read replaces them.
func TestSubscriptionCardSaysTheLastRead(t *testing.T) {
	f := feedHarness(t)
	feed := newFeedServer(t, holidays, "")
	sub := subscribe(t, f, url.Values{"url": {feed.URL + "/ru.ics"}})
	card := func() view.SubscriptionCard {
		t.Helper()
		page := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
		if len(page.Subscriptions) != 1 {
			t.Fatalf("cards = %+v", page.Subscriptions)
		}
		return page.Subscriptions[0]
	}
	if c := card(); c.Checked != "" || c.Size != "" || c.Took != "" {
		t.Errorf("before a read: checked %q, size %q, took %q", c.Checked, c.Size, c.Took)
	}
	took := regexp.MustCompile(`^(\d+ ms|\d+\.\d s)$`)
	list := "/app/calendar/" + sub.ID + "/feed"
	_ = f.h.do(get(list+"?from=2026-08-31", f.sess))
	c := card()
	if c.Checked == "" || c.Size != view.Text(strconv.Itoa(len(holidays))+" B") || !took.MatchString(string(c.Took)) {
		t.Errorf("after a read: checked %q, size %q, took %q", c.Checked, c.Size, c.Took)
	}
	// 41 092 bytes: «40 KB», whole kilobytes as the frame writes them.
	pad := strings.Repeat("X-PAD:"+strings.Repeat("x", 60)+"\r\n", 600)
	feed.set(strings.Replace(holidays, "END:VCALENDAR", pad+"END:VCALENDAR", 1))
	_ = f.h.do(get(list+"?from=2026-08-31", f.sess))
	if c := card(); c.Size != "40 KB" || !took.MatchString(string(c.Took)) {
		t.Errorf("after a larger read: size %q, took %q", c.Size, c.Took)
	}
}
