package web_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/view"
)

// The owner, 24.11.2026: «Refresh не активен стал почему-то». A list is
// drawn while its poll runs, so its top bar's Refresh is off with «The poll
// is still running.»; the end of the poll brought the bar, the rail and the
// list, not the top bar, and Refresh stayed off until the page was loaded
// again — which starts another poll. The end brings the top bar too, its
// Refresh on.
func TestTheEndOfAPollTurnsRefreshOn(t *testing.T) {
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.Seed(searchBook+"ada.vcf", adaCard())
	f := connectFixture(t, srv, searchBook, nil)
	tickAll(t, f.h, f.sess)

	list := decode[view.ContactsListPage](t, f.h.do(get("/app/contacts", f.sess)))
	if !list.Poll.Running || list.Top.Refresh.Action.Enabled {
		t.Fatalf("the list as drawn: poll running %v, refresh %+v", list.Poll.Running, list.Top.Refresh.Action)
	}
	body := f.h.do(get("/app/search/progress/"+list.Poll.TaskID, f.sess)).Body.String()
	at := strings.Index(body, "event: poll-finished\ndata: ")
	if at < 0 {
		t.Fatalf("no poll-finished:\n%s", body)
	}
	data := strings.TrimSpace(strings.SplitN(body[at+len("event: poll-finished\ndata: "):], "\n", 2)[0])
	var end struct {
		Extra []struct {
			Target string          `json:"target"`
			Body   json.RawMessage `json:"body"`
		} `json:"extra"`
	}
	if err := json.Unmarshal([]byte(data), &end); err != nil {
		t.Fatal(err)
	}
	for _, x := range end.Extra {
		if x.Target != "topbar" {
			continue
		}
		var page view.ContactsListPage
		if err := json.Unmarshal(x.Body, &page); err != nil {
			t.Fatal(err)
		}
		if !page.Top.Refresh.Action.Enabled || page.Top.Refresh.Answered == "" {
			t.Errorf("the top bar at the end: refresh %+v", page.Top.Refresh)
		}
		return
	}
	t.Errorf("the end of the poll brings no top bar: %.300s", data)
}
