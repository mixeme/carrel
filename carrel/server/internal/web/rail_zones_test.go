package web_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-webdav/caldav"
	"github.com/emersion/go-webdav/carddav"

	"carrel.dev/carrel/server/render"
	"carrel.dev/carrel/server/view"
)

// The rail's form asks the rail's zone (Typecase's note 186, components-43):
// its target is the rail, and the answer to a tick of the combined view is the
// rail with its ticks, the poll, and the list of the ticked collections as
// the zone main — the live layer puts them in place, the page kept. On a
// list of one collection the ticks do not change the list: rail and poll.
func TestTheRailsFormAnswersTheRailAndTheList(t *testing.T) {
	if !strings.Contains(mockupFrame(t, "components-43"), `id="post-sources" action="/app/sources" method="post" data-tc-target="rail"`) {
		t.Fatal("components-43 draws no rail form asking the rail's zone: read the frame again")
	}
	f := calendarFixture(t, nil)
	page := decode[struct {
		Rail view.Rail `json:"rail"`
	}](t, f.h.do(get("/app/calendar", f.sess)))
	ticks := page.Rail.Ticks
	if ticks == nil || ticks.Target != render.TargetRail {
		t.Fatalf("the rail's form = %+v, want the target rail", ticks)
	}
	form := url.Values{}
	for _, s := range ticks.Sets {
		for _, r := range s.Rows {
			form.Set(r.Name, string(r.Value))
		}
	}
	form.Set("toggle", "/app/calendar")
	req := postForm(ticks.Action, f.sess, form)
	req.Header.Set(render.HeaderView, render.ViewFragment)
	req.Header.Set(render.HeaderTarget, render.TargetRail)
	rec := f.h.do(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("the tick's answer: %d", rec.Code)
	}
	var answer struct {
		Target string `json:"target"`
		Extra  []struct {
			Target string          `json:"target"`
			Body   json.RawMessage `json:"body"`
		} `json:"extra"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil {
		t.Fatal(err)
	}
	var zones []string
	for _, x := range answer.Extra {
		zones = append(zones, x.Target)
	}
	if answer.Target != render.TargetRail || strings.Join(zones, " ") != "poll main" {
		t.Errorf("the answer is %s with %v, want rail with poll and main", answer.Target, zones)
	}
	for _, x := range answer.Extra {
		if x.Target != render.TargetMain {
			continue
		}
		var list view.AgendaPage
		if err := json.Unmarshal(x.Body, &list); err != nil || list.Rail.Ticks == nil {
			t.Errorf("main is not the combined calendar: %v %s", err, string(x.Body)[:min(200, len(x.Body))])
		}
	}
}

// A tick of a person's timeline's rail (s-contacts-7) answers the live
// layer as a list's does: the rail, the poll and the timeline asked again as
// the zone main — not a redirect, which reloaded the whole page (the owner,
// 27.11.2026).
func TestATimelinesRailTickKeepsThePage(t *testing.T) {
	now := time.Now()
	srv := searchServer(t)
	srv.AddAddressBook(carddav.AddressBook{Path: searchBook, Name: "People"})
	srv.Seed(searchBook+"ann.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:ann\nFN:Anna Kovaleva\nEMAIL:anna@example.com\nEND:VCARD\n")
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work"})
	srv.Seed(searchWork+"e1.ics", ics("BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260101T000000Z\nDTSTART:"+icsDay(now.AddDate(0, 0, -3))+
		"\nDTEND:"+icsDay(now.AddDate(0, 0, -3).Add(time.Hour))+"\nSUMMARY:Review with Anna\nATTENDEE;PARTSTAT=ACCEPTED:mailto:anna@example.com\nEND:VEVENT"))
	f := connectFixture(t, srv, searchBook, nil)
	tickAll(t, f.h, f.sess)
	card := f.path("contacts") + "/ann"

	tl := decode[view.ContactTimelinePage](t, f.h.do(get(card+"/timeline", f.sess)))
	ticks := tl.Rail.Ticks
	if ticks == nil {
		t.Fatal("the timeline's rail has no form")
	}
	form := url.Values{}
	for _, s := range ticks.Sets {
		for _, r := range s.Rows {
			form.Set(r.Name, string(r.Value))
		}
	}
	form.Set("toggle", "/app/calendar")
	req := postForm(ticks.Action, f.sess, form)
	req.Header.Set(render.HeaderView, render.ViewFragment)
	req.Header.Set(render.HeaderTarget, render.TargetRail)
	rec := f.h.do(req)
	if rec.Code != http.StatusOK {
		t.Fatalf("the tick's answer: %d %s", rec.Code, rec.Header().Get("Location"))
	}
	var answer struct {
		Target string `json:"target"`
		Extra  []struct {
			Target string          `json:"target"`
			Body   json.RawMessage `json:"body"`
		} `json:"extra"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &answer); err != nil {
		t.Fatal(err)
	}
	var zones []string
	for _, x := range answer.Extra {
		zones = append(zones, x.Target)
		if x.Target == render.TargetMain {
			var page view.ContactTimelinePage
			if err := json.Unmarshal(x.Body, &page); err != nil || page.Rail.Ticks == nil {
				t.Errorf("main is not the timeline: %v", err)
			}
		}
	}
	if answer.Target != render.TargetRail || strings.Join(zones, " ") != "poll main" {
		t.Errorf("the answer is %s with %v, want rail with poll and main", answer.Target, zones)
	}
}
