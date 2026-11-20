package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// A published calendar says whether its link shows where events are
// (PublishedPage.Places): with «Location» ticked a row without a place
// stands «—», as every list says nothing (s-publish-6: «Поездка в Казань»);
// with it off the column stays empty (s-publish-24, -39). The acceptance
// stand found the dash missing (U-15.1, round 6).
func TestAPublishedCalendarSaysItShowsPlaces(t *testing.T) {
	app := newMockupApp(t)
	page := func(shows ...string) view.PublishedPage {
		t.Helper()
		q := url.Values{"k": {"calendar"}, "a": {app.realAccount(t, "lichnyj")}, "c": {app.realPath(t, "lichnyj")},
			"how": {"range"}, "from": {"2026-08-10"}, "to": {"2026-08-31"}, "lives": {"30"}}
		for _, s := range shows {
			q.Set(s, "on")
		}
		link := string(decode[view.PublishSheet](t, app.h.do(postForm("/app/publish", app.sess, q))).Link)
		if !strings.Contains(link, "/p/") {
			t.Fatalf("publish %v: no link", q)
		}
		return decode[view.PublishedPage](t, app.h.do(get(link[strings.Index(link, "/p/"):], nil)))
	}
	if p := page("title", "location"); !p.Places {
		t.Errorf("a link that shows places: Places = false")
	}
	if p := page("title"); p.Places {
		t.Errorf("a link that hides places: Places = true")
	}
}
