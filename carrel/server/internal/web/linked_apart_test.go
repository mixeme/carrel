package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Б16 of the audit of 11.10.2026: a linked group came apart without a word —
// a member deleted or moved away, its collection gone. The record that is
// left says so where the group is found to have come apart: on its card
// when it is opened, on the screen of duplicates when it is read.
func TestALinkedGroupThatCameApartSaysSo(t *testing.T) {
	f := newDupFixture(t)
	f.load(t)
	g := onlyGroup(t, f.page(t))
	pick := choiceValue(t, g.Choices[0], "LOVELACE Ada")
	f.h.do(postForm(actionOf(t, g, "duplicates.link").Href, f.sess, url.Values{"choice.FN": {pick}}))
	if d := f.decisions(t); len(d.Linked) != 1 {
		t.Fatalf("not linked: %+v", d.Linked)
	}

	// The other copy is deleted where it lives.
	_, etag, _ := f.other.Object(workBook + "ada-there.vcf")
	f.h.do(postForm(f.record(f.otherID, workBook, "ada-there")+"/delete", f.sess, url.Values{"etag": {etag}, "sheet": {"1"}, "copy": {f.otherID + "/" + url.PathEscape(workBook)}}))
	if _, _, ok := f.other.Object(workBook + "ada-there.vcf"); ok {
		t.Fatal("the other copy was not deleted")
	}

	card := decode[view.ContactPage](t, f.h.do(get(f.record(f.id, peopleBook, "ada-here"), f.sess)))
	apart := noticeTitled(card.Shell.Notices, "No longer linked")
	if apart == nil || !strings.Contains(string(apart.Body), "Work") {
		t.Errorf("the card says nothing of the group it was in: %+v", card.Shell.Notices)
	}
	if d := f.decisions(t); len(d.Linked) != 0 {
		t.Errorf("the group stands with one member: %+v", d.Linked)
	}
	// Said once: the next open is the card of one record.
	again := decode[view.ContactPage](t, f.h.do(get(f.record(f.id, peopleBook, "ada-here"), f.sess)))
	if noticeTitled(again.Shell.Notices, "No longer linked") != nil {
		t.Error("the notice stays on every open")
	}
}

// The screen of duplicates says how many groups came apart when it finds
// their members gone.
func TestTheDuplicatesScreenSaysGroupsCameApart(t *testing.T) {
	f := newDupFixture(t)
	f.load(t)
	g := onlyGroup(t, f.page(t))
	pick := choiceValue(t, g.Choices[0], "LOVELACE Ada")
	f.h.do(postForm(actionOf(t, g, "duplicates.link").Href, f.sess, url.Values{"choice.FN": {pick}}))

	_, etag, _ := f.other.Object(workBook + "ada-there.vcf")
	f.h.do(postForm(f.record(f.otherID, workBook, "ada-there")+"/delete", f.sess, url.Values{"etag": {etag}, "sheet": {"1"}, "copy": {f.otherID + "/" + url.PathEscape(workBook)}}))
	f.load(t)
	page := f.page(t)
	if n := noticeTitled(page.Shell.Notices, "1 linked group came apart"); n == nil {
		t.Errorf("notices = %+v", page.Shell.Notices)
	}
}
