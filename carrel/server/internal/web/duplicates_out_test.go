package web_test

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// One record comes out of a group of three and more (s-dup-9, -10;
// Typecase's note 209, task 267): Charles shares Ada's phone in the first
// book and is pulled into the group of her three copies. «Take one out»
// names each record with its collection; taking Charles out leaves Ada's
// three as a group, and Charles and the copy he was pulled to stand as a
// «not duplicates» pair. A pair has no such menu: that is Not duplicates.
func TestARecordComesOutOfItsGroup(t *testing.T) {
	f := newDupFixture(t)
	f.other.Seed(workBook+"ada-third.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:ada-third\nFN:Ada Lovelace\nEMAIL:ada@example.com\nEND:VCARD\n")
	f.srv.Seed(peopleBook+"charles.vcf", "BEGIN:VCARD\nVERSION:4.0\nUID:charles\nFN:Charles Babbage\nTEL:+44 20 7946 0000\nEND:VCARD\n")
	f.load(t)
	g := onlyGroup(t, f.page(t))
	if len(g.Rows) != 4 || g.Out == nil || g.Out.Label != "Take one out" || len(g.Out.Items) != 4 {
		t.Fatalf("the group of four: %d rows, out %+v", len(g.Rows), g.Out)
	}
	place := ""
	for i, it := range g.Out.Items {
		if !it.Submits || it.Name != "out" || it.Value != strconv.Itoa(i+1) || it.Colour != g.Rows[i].Colour {
			t.Errorf("item %d: %+v", i, it)
		}
		if !strings.HasPrefix(string(it.Label), string(g.Rows[i].Title)+" · ") {
			t.Errorf("item %d names %q, its row %q", i, it.Label, g.Rows[i].Title)
		}
		if g.Rows[i].Title == "Charles Babbage" {
			place = it.Value
		}
	}
	sent := formFields(*g.Form)
	sent.Set("out", place)
	if rec := f.h.do(postForm(g.Form.Action, f.sess, sent)); rec.Code != http.StatusSeeOther {
		t.Fatalf("out: %d %s", rec.Code, rec.Body.String())
	}
	p := f.page(t)
	var group, pair *view.DuplicateGroup
	for i := range p.Groups {
		switch p.Groups[i].Decision {
		case view.DecisionNone:
			group = &p.Groups[i]
		case view.DecisionIgnored:
			pair = &p.Groups[i]
		}
	}
	if group == nil || len(group.Rows) != 3 || group.Badge.Label != "candidate ×3" {
		t.Fatalf("the group without Charles: %+v", group)
	}
	for _, r := range group.Rows {
		if r.Title == "Charles Babbage" {
			t.Errorf("Charles is still in the group: %+v", group.Rows)
		}
	}
	if pair == nil || len(pair.Rows) != 2 || pair.Badge.Label != "not duplicates" || !strings.HasPrefix(string(pair.Points), "60 points") {
		t.Fatalf("the pair he was pulled into: %+v", pair)
	}
	if d := f.decisions(t); len(d.Ignored) != 1 {
		t.Errorf("decisions: %+v", d.Ignored)
	}
	// A group of two takes none out.
	f2 := newDupFixture(t)
	f2.load(t)
	if two := onlyGroup(t, f2.page(t)); two.Out != nil {
		t.Errorf("a pair: %+v", two.Out)
	}
}
