package web_test

import (
	"net/url"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Нс21 of the scan of 15.10.2026 (s-profile-1, -10): the first column of a
// collection's row is its tick — «a collection unticked shows an unticked
// box: it is not polled». Ticked is polled in every kind the collection
// holds, part in some, off in none; the tick flips it in all of them at
// once, and the page comes back.
func TestACollectionRowCarriesItsTick(t *testing.T) {
	f := calendarFixture(t, nil)
	row := func() view.CollectionRow {
		t.Helper()
		p := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections", f.sess)))
		return p.Accounts[0].Collections[0]
	}
	press := func(r view.CollectionRow) {
		t.Helper()
		if r.Toggle.ID != "collection.tick" || r.Toggle.Method != "POST" || !r.Toggle.Enabled || r.Toggle.Label != r.Label {
			t.Fatalf("the tick = %+v", r.Toggle)
		}
		rec := f.h.do(postForm(r.Toggle.Href, f.sess, url.Values{"pick": {r.Pick}}))
		if rec.Code != 303 || rec.Header().Get("Location") != "/app/settings/connections" {
			t.Fatalf("tick: %d %q", rec.Code, rec.Header().Get("Location"))
		}
	}
	first := row()
	if len(first.Kinds) < 2 {
		t.Fatalf("the calendar holds %v: the part state needs two kinds", first.Kinds)
	}
	every := func(want bool) {
		t.Helper()
		for _, k := range first.Kinds {
			if got := len(selected(t, f.h, f.sess, k)) == 1; got != want {
				t.Errorf("%s ticked = %v, want %v", k, got, want)
			}
		}
	}

	// Unticked everywhere: off, and the tick ticks it in every kind.
	for _, k := range first.Kinds {
		tickRail(t, f.h, f.sess, recordKindSegment(t, k))
	}
	if r := row(); r.Tick != view.TickOff {
		t.Fatalf("unticked everywhere, tick = %q", r.Tick)
	}
	press(row())
	every(true)
	if r := row(); r.Tick != view.TickOn {
		t.Errorf("ticked everywhere, tick = %q", r.Tick)
	}

	// Ticked in one kind of its several: part, and the tick ticks the rest.
	for _, k := range first.Kinds[1:] {
		tickRail(t, f.h, f.sess, recordKindSegment(t, k))
	}
	if r := row(); r.Tick != view.TickPart {
		t.Errorf("ticked in one kind, tick = %q", r.Tick)
	}
	press(row())
	every(true)

	// Ticked everywhere: the tick unticks it everywhere.
	press(row())
	every(false)
	if r := row(); r.Tick != view.TickOff {
		t.Errorf("after untick, tick = %q", r.Tick)
	}
}

// recordKindSegment is the address segment of a kind's sources.
func recordKindSegment(t *testing.T, k view.Kind) string {
	t.Helper()
	seg, ok := map[view.Kind]string{view.KindContact: "contacts", view.KindEvent: "calendar", view.KindTask: "tasks", view.KindNote: "notes"}[k]
	if !ok {
		t.Fatalf("no sources of %s", k)
	}
	return seg
}
