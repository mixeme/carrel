package web_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"carrel.dev/carrel/server/internal/sealed"
	"carrel.dev/carrel/server/view"
	"github.com/emersion/go-webdav/carddav"
)

// hangingServer answers nothing: every request waits until the client goes,
// and the server counts the requests that were cut.
type hangingServer struct {
	*httptest.Server
	started, cut atomic.Int32
}

func newHangingServer(t *testing.T) *hangingServer {
	hs := &hangingServer{}
	hs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hs.started.Add(1)
		// Go notices a client going only once the body is read.
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
		hs.cut.Add(1)
	}))
	t.Cleanup(hs.Close)
	return hs
}

func goroutinesSettle(t *testing.T, base int, within time.Duration) int {
	t.Helper()
	deadline := time.Now().Add(within)
	for {
		runtime.GC()
		n := runtime.NumGoroutine()
		if n <= base || time.Now().After(deadline) {
			return n
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// S-30.3: leaving the page while a poll runs leaves no goroutine behind,
// and stopping the task closes its connections — what arrived stays.
func TestLeavingThePollLeavesNothingRunning(t *testing.T) {
	hang := newHangingServer(t)
	h, sess := davHarness(t)
	// Room for the race detector, which runs all of this several times
	// slower: the first request must reach the far server within the source's
	// time for the test to say anything.
	h.cfg.ProgressSourceTimeout = 5 * time.Second
	h.cfg.ProgressTotalTimeout = 6 * time.Second
	h.rebuild()
	editBlob(t, h, sess, func(b *sealed.Blob) {
		b.Connections = append(b.Connections, sealed.Connection{ID: "slow", Server: hang.URL + "/dav/"})
		b.Collections = append(b.Collections, sealed.Collection{AccountID: "slow", Path: "/dav/book/", Name: "Slow", Kinds: []view.Kind{view.KindContact}})
		b.Selected = append(b.Selected, sealed.Selection{Kind: view.KindContact, AccountID: "slow", Path: "/dav/book/"})
	})
	live := httptest.NewServer(h.srv.Handler())
	defer live.Close()
	http.DefaultClient.CloseIdleConnections()
	base := goroutinesSettle(t, 0, 200*time.Millisecond)

	page := decode[view.SearchPage](t, h.do(get("/app/search?q=ada", sess)))
	task := page.Poll.TaskID
	if task == "" || !page.Poll.Running {
		t.Fatalf("poll = %+v", page.Poll)
	}
	// The stream is open, the first bytes come, then the page goes away.
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, live.URL+"/app/search/progress/"+task, nil)
	req.AddCookie(&http.Cookie{Name: "carrel_sid", Value: sess.ID()})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 64)
	if _, err := resp.Body.Read(buf); err != nil {
		t.Fatalf("stream: %v", err)
	}
	cancel()
	resp.Body.Close()
	http.DefaultClient.CloseIdleConnections()

	if n := goroutinesSettle(t, base+2, 20*time.Second); n > base+2 {
		t.Errorf("goroutines: %d before, %d after the page went and the poll ended", base, n)
	}
	if hang.started.Load() == 0 || hang.cut.Load() != hang.started.Load() {
		t.Errorf("far requests: %d started, %d cut", hang.started.Load(), hang.cut.Load())
	}

	// The first poll has ended — its slot is free — before the next one.
	for wait := time.Now().Add(20 * time.Second); time.Now().Before(wait); {
		if !decode[pollFragment](t, h.do(asPoll(get("/app/search/progress/"+task+"/poll", sess)))).Body.Running {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Stop closes the connection of a source still asked, at once. The
	// counts are taken before the search: its request may land at once.
	before := hang.cut.Load()
	startedBefore := hang.started.Load()
	page = decode[view.SearchPage](t, h.do(get("/app/search?q=bob", sess)))
	for wait := time.Now().Add(20 * time.Second); hang.started.Load() == startedBefore && time.Now().Before(wait); {
		time.Sleep(20 * time.Millisecond)
	}
	if hang.started.Load() == startedBefore {
		t.Fatal("the second search did not ask the slow server")
	}
	h.do(asPoll(postForm("/app/search/progress/"+page.Poll.TaskID+"/cancel", sess, url.Values{})))
	deadline := time.Now().Add(5 * time.Second)
	for hang.cut.Load() == before && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if hang.cut.Load() == before {
		t.Error("Stop left the far request open")
	}
}

// S-30.3: one connection of three down breaks no screen — each answers,
// and none with a server error.
func TestOneConnectionOfThreeDown(t *testing.T) {
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.Seed(searchBook+"ada.vcf", adaCard())
	other := searchServer(t)
	other.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "Office"})
	f := connectFixture(t, srv, searchBook, nil)
	rec := f.h.do(postForm("/app/settings/connections", f.sess, url.Values{"server": {other.URL}, "login": {other.Login}, "password": {other.Password}}))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("second connection: %d", rec.Code)
	}
	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		b.Connections = append(b.Connections, sealed.Connection{ID: "down", Server: "http://127.0.0.1:1/dav/"})
		b.Collections = append(b.Collections,
			sealed.Collection{AccountID: "down", Path: "/dav/cal/", Name: "Lost", Kinds: []view.Kind{view.KindEvent, view.KindTask, view.KindNote}},
			sealed.Collection{AccountID: "down", Path: "/dav/files/", Name: "Lost files", Kinds: []view.Kind{view.KindFile}},
			sealed.Collection{AccountID: "down", Path: "/dav/book/", Name: "Lost book", Kinds: []view.Kind{view.KindContact}})
	})
	tickAll(t, f.h, f.sess)
	for _, path := range []string{
		"/app/contacts", "/app/calendar", "/app/tasks", "/app/notes", "/app/files", "/app/duplicates",
		"/app/search?q=ada", "/app/settings/connections", "/app/settings/davloom", "/app/settings/backup",
		"/app/files/published", "/app/settings/attachments", "/app/settings/devices",
		"/app/contacts/down/" + url.PathEscape("/dav/book/"), "/app/calendar/down/" + url.PathEscape("/dav/cal/"),
		"/app/files/down/" + url.PathEscape("/dav/files/") + "/",
	} {
		rec := f.h.do(get(path, f.sess))
		if rec.Code >= 500 {
			t.Errorf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
	}
	// Every screen above started a poll of its own, and a session runs at
	// most conf.MaxLiveFanouts of them at once. Alone they are done before
	// this line; in a full run under load some are not, and the search is
	// refused as one too many — the ceiling doing its job, not the dead
	// connection winning. The search is asked again until it is let in: what
	// this test holds is the answer of a search that ran (24.09.2026, a full
	// run failed here and the test could not say why).
	search := decode[view.SearchPage](t, f.h.do(get("/app/search?q=ada", f.sess)))
	for wait := time.Now().Add(30 * time.Second); hasNotice(search.Notices, "Too many searches") && time.Now().Before(wait); {
		time.Sleep(50 * time.Millisecond)
		search = decode[view.SearchPage](t, f.h.do(get("/app/search?q=ada", f.sess)))
	}
	for wait := time.Now().Add(30 * time.Second); search.Poll.Running && time.Now().Before(wait); {
		time.Sleep(50 * time.Millisecond)
		search = decode[view.SearchPage](t, f.h.do(get("/app/search?q=ada", f.sess)))
	}
	if len(search.Results) == 0 {
		t.Errorf("the connection that answers lost its result to the one that did not; notices %+v, sources %+v",
			search.Notices, search.Poll.Sources)
	}
}

// S-30.3, S-29: at debug level the log holds no password, token or record
// content — through a whole flow that handles every one of them.
func TestDebugLogHoldsNoSecrets(t *testing.T) {
	var buf safeBuffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(old)

	f, relay := letterFixture(t)
	_ = relay
	secretRecord := "Строго секретная запись"
	card := "BEGIN:VCARD\nVERSION:4.0\nUID:ada2\nFN:" + secretRecord + "\nEMAIL:ada2@example.com\nEND:VCARD\n"
	f.srv.Seed(searchBook+"ada2.vcf", card)

	// A record read and written, a link made, a device issued, a letter
	// sent, the relay's password stored.
	f.h.do(get("/app/contacts", f.sess))
	f.h.do(get("/app/search?q=Строго", f.sess))
	made := decode[view.PublishSheet](t, f.h.do(postForm("/app/publish?"+url.Values{"k": {"contacts"}, "a": {f.id}, "c": {searchBook}, "uid": {"ada2"}}.Encode(), f.sess, url.Values{})))
	f.h.do(fragment(postForm("/app/settings/unlock", f.sess, url.Values{"password": {"password123"}}), "sheet"))
	f.h.do(postForm("/app/settings/davloom", f.sess, url.Values{"op": {"issue"}, "name": {"Phone"}, "kind.contact": {"on"}, "col." + f.id + "|" + searchBook: {"on"}}))
	device := decode[view.DavloomPage](t, f.h.do(get("/app/settings/davloom", f.sess)))
	devicePassword := string(profileRow(device.Add, "password").Value)
	f.h.do(postForm("/app/mail/compose?op=send", f.sess, url.Values{"to": {"ann@example.org"}, "subject": {secretRecord}, "body": {secretRecord}}))
	f.h.do(postForm("/admin/settings", f.sess, url.Values{"host": {"127.0.0.1"}, "port": {"25"}, "security": {"none"}, "username": {"relay"}, "password": {"relay-password-xyz"}, "from": {"noreply@carrel.example"}}))
	f.h.do(get("/app/files/"+f.id+"/"+url.PathEscape(filesDocs)+"/..%2F..%2Fetc/", f.sess))

	out := buf.String()
	for name, secret := range map[string]string{
		"sign-in password": "password123", "DAV password": f.srv.Password, "device password": devicePassword,
		"link token": strings.TrimPrefix(string(made.Link), "/p/"), "relay password": "relay-password-xyz",
		"record content": secretRecord, "session ID": f.sess.ID(), "CSRF token": f.sess.CSRFToken(),
	} {
		if secret != "" && strings.Contains(out, secret) {
			t.Errorf("the log holds the %s", name)
		}
	}
	if !strings.Contains(out, "leaves its collection was refused") {
		t.Error("a path that leaves its collection went unseen")
	}
}

type safeBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}
