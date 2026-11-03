package web_test

import (
	"net/http"
	"net/url"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Б45 of the audit of 11.10.2026: a threshold or a weight that is not a
// number in range was dropped without a word, and the screen came back
// showing the old one. It is said, and nothing of it is saved.
func TestAWrongNumberOnTheDuplicatesScreenIsSaid(t *testing.T) {
	f := newDupFixture(t)
	f.load(t)
	// The owner's words of 03.11.2026: the field by its name, what it takes,
	// and what it stays.
	rec := f.h.do(postForm("/app/duplicates/decide", f.sess, url.Values{"do": {"threshold"}, "value": {"7x"}}))
	if rec.Code != http.StatusOK {
		t.Fatalf("a threshold that is no number: %d, want the screen with a notice", rec.Code)
	}
	page := decode[view.DuplicatesPage](t, rec)
	if n := mustNotice(t, page.Notices, "Match threshold not changed: «7x» is not a whole number from 0 to 1000."); n.Body != "It stays 60." || page.Threshold.Value != 60 {
		t.Errorf("body %q, threshold %+v", n.Body, page.Threshold)
	}
	if got := f.decisions(t).Threshold; got != 0 {
		t.Errorf("the threshold saved = %d", got)
	}

	// Weights as a delete in Files: how many of how many, the rule in the
	// line, what was saved; each by its signal under «Not saved».
	bad, good := page.Threshold.Weights[0], page.Threshold.Weights[1]
	sig := bad.Signal
	rec = f.h.do(postForm("/app/duplicates/decide", f.sess, url.Values{"do": {"weights"}, bad.Name: {"-3"}, good.Name: {"5"}}))
	if rec.Code != http.StatusOK {
		t.Fatalf("a weight out of range: %d, want the screen with a notice", rec.Code)
	}
	n := mustNotice(t, decode[view.DuplicatesPage](t, rec).Notices, "1 of 2 weights not saved: a weight is a whole number from 0 to 1000.")
	if n.Body != "1 saved, 1 unchanged." || n.Details == nil || detailText(n.Details) != "Not saved | "+string(bad.Label)+" · «-3»" {
		t.Errorf("notice = %q / %+v", n.Body, n.Details)
	}
	if got := f.decisions(t).Weights[good.Signal]; got != 5 {
		t.Errorf("the right weight was not saved: %+v", f.decisions(t).Weights)
	}
	if _, set := f.decisions(t).Weights[sig]; set {
		t.Errorf("the weight was saved: %+v", f.decisions(t).Weights)
	}
}
