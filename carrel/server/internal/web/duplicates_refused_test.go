package web_test

import (
	"net/http"
	"net/url"
	"testing"

	"carrel.dev/carrel/server/internal/dupes"
	"carrel.dev/carrel/server/view"
)

// Б45 of the audit of 11.10.2026: a threshold or a weight that is not a
// number in range was dropped without a word, and the screen came back
// showing the old one. It is said, and nothing of it is saved.
func TestAWrongNumberOnTheDuplicatesScreenIsSaid(t *testing.T) {
	f := newDupFixture(t)
	f.load(t)
	rec := f.h.do(postForm("/app/duplicates/decide", f.sess, url.Values{"do": {"threshold"}, "value": {"many"}}))
	if rec.Code != http.StatusOK {
		t.Fatalf("a threshold that is no number: %d, want the screen with a notice", rec.Code)
	}
	if page := decode[view.DuplicatesPage](t, rec); !hasNotice(page.Notices, "The threshold was not changed") || page.Threshold.Value != 60 {
		t.Errorf("notices = %+v, threshold %+v", page.Notices, page.Threshold)
	}
	if got := f.decisions(t).Threshold; got != 0 {
		t.Errorf("the threshold saved = %d", got)
	}

	sig := string(dupes.Signals[0])
	rec = f.h.do(postForm("/app/duplicates/decide", f.sess, url.Values{"do": {"weights"}, "weight." + sig: {"-5"}}))
	if rec.Code != http.StatusOK {
		t.Fatalf("a weight out of range: %d, want the screen with a notice", rec.Code)
	}
	if page := decode[view.DuplicatesPage](t, rec); !hasNotice(page.Notices, "1 weight not changed") {
		t.Errorf("notices = %+v", page.Notices)
	}
	if _, set := f.decisions(t).Weights[sig]; set {
		t.Errorf("the weight was saved: %+v", f.decisions(t).Weights)
	}
}
