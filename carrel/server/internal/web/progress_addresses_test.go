package web_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/view"
)

// Stage 4 of the client's plan, С2: the poll bar says where to follow it —
// the stream, the fallback poll and its interval — from the configuration,
// and in poll mode there is no stream to name. The addresses carry the
// query of the page the bar stands on.
func TestPollBarSaysWhereToFollowIt(t *testing.T) {
	f := contactsFixture(t, false, adaCard())
	tickAll(t, f.h, f.sess)
	bar := search(t, f.h, f.sess, "q=Ada").Poll
	base := "/app/search/progress/" + bar.TaskID
	if bar.TaskID == "" || bar.Stream != base || bar.Poll != base+"/poll" || bar.Every != 700 {
		t.Errorf("search bar: stream %q, poll %q, every %d; want %s, %s/poll, 700", bar.Stream, bar.Poll, bar.Every, base, base)
	}

	f.h.cfg.ProgressMode, f.h.cfg.ProgressPollMillis = "poll", 1234
	f.h.rebuild()
	bar = search(t, f.h, f.sess, "q=Ada&again=1").Poll
	base = "/app/search/progress/" + bar.TaskID
	if bar.Stream != "" || bar.Poll != base+"/poll" || bar.Every != 1234 {
		t.Errorf("poll mode: stream %q, poll %q, every %d; want none, %s/poll, 1234", bar.Stream, bar.Poll, bar.Every, base)
	}

	list := decode[view.ContactsListPage](t, f.h.do(get("/app/contacts?tag=work", f.sess)))
	if list.Poll == nil || !strings.HasSuffix(list.Poll.Poll, "/poll?tag=work") {
		t.Errorf("the bar of a filtered list does not carry its filter: %+v", list.Poll)
	}
}

// П1 and С4: the stream says two things — the bar changed (poll) and the
// poll is over (poll-finished). The counts of the rail and the states of its
// rows come from the poll, so the end of it brings the page's rail (C-7.3);
// the fallback poll, finished, brings it out of band.
func TestTheEndOfAPollBringsTheRail(t *testing.T) {
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.Seed(searchBook+"ada.vcf", adaCard())
	srv.Seed(searchBook+"bob.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:bob\nFN:Bob Builder\nEND:VCARD\n")
	f := connectFixture(t, srv, searchBook, nil)
	tickAll(t, f.h, f.sess)

	first := decode[view.ContactsListPage](t, f.h.do(get("/app/contacts", f.sess)))
	task := first.Poll.TaskID
	if task == "" {
		t.Fatal("the combined book started no poll")
	}
	rec := f.h.do(get("/app/search/progress/"+task, f.sess))
	body := rec.Body.String()
	at := strings.Index(body, "event: poll-finished\ndata: ")
	if rec.Code != http.StatusOK || at < 0 || strings.Count(body, "event: ") != strings.Count(body, "event: poll\n")+1 {
		t.Fatalf("stream %d:\n%s", rec.Code, body)
	}
	// The bar as it ended, the rail out of band: the live layer puts the
	// event into the bar's zone (Typecase's note 124).
	var end struct {
		Target string       `json:"target"`
		Body   view.PollBar `json:"body"`
		Extra  []struct {
			Target string    `json:"target"`
			Body   view.Rail `json:"body"`
		} `json:"extra"`
	}
	data := strings.TrimSpace(strings.SplitN(body[at+len("event: poll-finished\ndata: "):], "\n", 2)[0])
	if err := json.Unmarshal([]byte(data), &end); err != nil || end.Target != "poll" || end.Body.Running || len(end.Extra) == 0 || end.Extra[0].Target != "rail" {
		t.Fatalf("poll-finished carries %s, want the bar and the rail out of band (%v)", data, err)
	}
	rail := end.Extra[0].Body
	if _, people := railRow(t, rail, "People"); people.Count == nil || *people.Count != 2 {
		t.Errorf("the rail at the end counts People %v, want 2", people.Count)
	}
	if c := rail.Sections[0].Rows[0]; c.Count == nil || *c.Count != 2 {
		t.Errorf("the combined row at the end = %+v", c)
	}

	var poll struct {
		Target string `json:"target"`
		Extra  []struct {
			Target string          `json:"target"`
			Body   json.RawMessage `json:"body"`
		} `json:"extra"`
	}
	raw := f.h.do(asPoll(get("/app/search/progress/"+task+"/poll", f.sess))).Body.Bytes()
	if err := json.Unmarshal(raw, &poll); err != nil || poll.Target != "poll" || len(poll.Extra) == 0 || poll.Extra[0].Target != "rail" {
		t.Errorf("the finished poll by the fallback = %s, want the bar and the rail out of band", raw)
	}
}

// U-15.1, found on the acceptance stand: a combined list and a search are
// drawn when they are opened, before the servers answer, and what the poll
// brings is their rows. The end of the poll brings the page as it reads
// after it (C-7.3): main out of band, after the rail — in the stream's
// poll-finished and in the fallback poll's last answer. Without it the page
// the person opened stays without rows.
func TestTheEndOfAPollBringsItsPage(t *testing.T) {
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.Seed(searchBook+"ada.vcf", adaCard())
	srv.Seed(searchBook+"bob.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:bob\nFN:Bob Builder\nEND:VCARD\n")
	f := connectFixture(t, srv, searchBook, nil)
	tickAll(t, f.h, f.sess)

	mainOf := func(what, data string) json.RawMessage {
		t.Helper()
		var end struct {
			Extra []struct {
				Target string          `json:"target"`
				Body   json.RawMessage `json:"body"`
			} `json:"extra"`
		}
		if err := json.Unmarshal([]byte(data), &end); err != nil {
			t.Fatalf("%s: %v in %.400s", what, err, data)
		}
		for _, x := range end.Extra {
			if x.Target == "main" {
				return x.Body
			}
		}
		t.Fatalf("%s carries no main out of band: %.400s", what, data)
		return nil
	}
	finished := func(task string) string {
		t.Helper()
		body := f.h.do(get("/app/search/progress/"+task, f.sess)).Body.String()
		at := strings.Index(body, "event: poll-finished\ndata: ")
		if at < 0 {
			t.Fatalf("no poll-finished:\n%s", body)
		}
		return strings.TrimSpace(strings.SplitN(body[at+len("event: poll-finished\ndata: "):], "\n", 2)[0])
	}
	names := func(groups []view.ContactGroup) []string {
		var out []string
		for _, g := range groups {
			for _, r := range g.Rows {
				out = append(out, string(r.Name))
			}
		}
		return out
	}

	list := decode[view.ContactsListPage](t, f.h.do(get("/app/contacts", f.sess)))
	var page view.ContactsListPage
	if err := json.Unmarshal(mainOf("the list's poll-finished", finished(list.Poll.TaskID)), &page); err != nil {
		t.Fatal(err)
	}
	if got := names(page.Groups); len(got) != 2 {
		t.Errorf("the list at the end of its poll shows %v, want Ada and Bob", got)
	}
	if page.Poll == nil || page.Poll.Running || page.Poll.TaskID != list.Poll.TaskID {
		t.Errorf("the list at the end stands on another poll or a running one: %+v", page.Poll)
	}
	// The page is drawn as the browser's own would be (U-15.1, the stand's
	// second round): its Look — the source column of the combined notebook
	// was gone without it — and its rows opening in the panel, as the list
	// they replace does.
	looks := func(what string, page view.ContactsListPage) {
		t.Helper()
		if page.Look != list.Look {
			t.Errorf("%s: look %+v, want the browser's %+v", what, page.Look, list.Look)
		}
		for _, g := range page.Groups {
			for _, r := range g.Rows {
				if !strings.HasPrefix(r.Href, "/app/contacts?") || !strings.Contains(r.Href, "open=") {
					t.Errorf("%s: %s leads to %s, not the panel of the list", what, r.Name, r.Href)
				}
			}
		}
	}
	looks("the list's poll-finished", page)
	raw := f.h.do(asPoll(get("/app/search/progress/"+list.Poll.TaskID+"/poll", f.sess))).Body.String()
	page = view.ContactsListPage{}
	if err := json.Unmarshal(mainOf("the list's last fallback answer", raw), &page); err != nil || len(names(page.Groups)) != 2 {
		t.Errorf("the list by the fallback poll at the end: %v, %v", names(page.Groups), err)
	}
	looks("the list's last fallback answer", page)

	found := search(t, f.h, f.sess, "q=Ada")
	var results view.SearchPage
	if err := json.Unmarshal(mainOf("the search's poll-finished", finished(found.Poll.TaskID)), &results); err != nil {
		t.Fatal(err)
	}
	if len(results.Results) == 0 {
		t.Errorf("the search at the end of its poll shows nothing: %+v", results)
	}
}
