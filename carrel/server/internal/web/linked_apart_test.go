package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/internal/sealed"
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
	f.h.do(decideRequest(t, f.sess, g, "duplicates.link", asItStands(g, url.Values{"choice.FN": {pick}})))
	if d := f.decisions(t); len(d.Linked) != 1 {
		t.Fatalf("not linked: %+v", d.Linked)
	}

	// The other copy is deleted where it lives.
	_, etag, _ := f.other.Object(workBook + "ada-there.vcf")
	f.h.do(postForm(f.record(f.otherID, workBook, "ada-there")+"/delete", f.sess, url.Values{"etag": {etag}, "sheet": {"1"}, "copy": {f.otherID + "/" + url.PathEscape(workBook)}}))
	if _, _, ok := f.other.Object(workBook + "ada-there.vcf"); ok {
		t.Fatal("the other copy was not deleted")
	}

	// The owner's words of 03.11.2026: what Carrel saw — no record at the
	// address —, not what happened to it.
	card := decode[view.ContactPage](t, f.h.do(get(f.record(f.id, peopleBook, "ada-here"), f.sess)))
	apart := noticeTitled(card.Shell.Notices, "Link removed.")
	if apart == nil || !strings.HasPrefix(string(apart.Body), "The linked copy in ") || !strings.Contains(string(apart.Body), "Work") ||
		!strings.HasSuffix(string(apart.Body), " is no longer at its address (deleted or moved). This record is unchanged.") {
		t.Errorf("the card says nothing of the group it was in: %+v", card.Shell.Notices)
	}
	if d := f.decisions(t); len(d.Linked) != 0 {
		t.Errorf("the group stands with one member: %+v", d.Linked)
	}
	// Said once: the next open is the card of one record.
	again := decode[view.ContactPage](t, f.h.do(get(f.record(f.id, peopleBook, "ada-here"), f.sess)))
	if noticeTitled(again.Shell.Notices, "Link removed.") != nil {
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
	f.h.do(decideRequest(t, f.sess, g, "duplicates.link", asItStands(g, url.Values{"choice.FN": {pick}})))

	_, etag, _ := f.other.Object(workBook + "ada-there.vcf")
	f.h.do(postForm(f.record(f.otherID, workBook, "ada-there")+"/delete", f.sess, url.Values{"etag": {etag}, "sheet": {"1"}, "copy": {f.otherID + "/" + url.PathEscape(workBook)}}))
	f.load(t)
	page := f.page(t)
	n := noticeTitled(page.Shell.Notices, "1 linked group removed.")
	if n == nil || string(n.Body) != "Its other records are no longer at their addresses (deleted or moved). The records left are unchanged on the server." {
		t.Errorf("notices = %+v", page.Shell.Notices)
	}
}

// linkedThree links Ada's two copies and adds a third member that is not on
// its server: what is left of a group of three whose one copy was deleted or
// moved elsewhere.
func linkedThree(t *testing.T) dupFixture {
	t.Helper()
	f := newDupFixture(t)
	f.load(t)
	g := onlyGroup(t, f.page(t))
	pick := choiceValue(t, g.Choices[0], "LOVELACE Ada")
	f.h.do(decideRequest(t, f.sess, g, "duplicates.link", asItStands(g, url.Values{"choice.FN": {pick}})))
	editBlob(t, f.h, f.sess, func(b *sealed.Blob) {
		b.Duplicates.Linked[0].Members = append(b.Duplicates.Linked[0].Members, sealed.RecordRef{AccountID: f.id, Col: peopleBook, UID: "ada-gone"})
	})
	return f
}

// A group of three that lost one stays a group of two, and says so: the card
// of a copy, and the person's timeline, name where the copy was (Б16, its
// rest), in the owner's words of 03.11.2026.
func TestALinkedGroupThatLostOneOfThreeSaysSo(t *testing.T) {
	said := func(t *testing.T, notices []view.Notice) {
		t.Helper()
		n := noticeTitled(notices, "Link removed.")
		if n == nil || !strings.HasPrefix(string(n.Body), "The linked copy in ") ||
			!strings.HasSuffix(string(n.Body), " is no longer at its address (deleted or moved). This record is unchanged.") {
			t.Errorf("notices = %+v", notices)
		}
	}

	f := linkedThree(t)
	card := decode[view.ContactPage](t, f.h.do(get(f.record(f.id, peopleBook, "ada-here"), f.sess)))
	said(t, card.Shell.Notices)
	if d := f.decisions(t); len(d.Linked) != 1 || len(d.Linked[0].Members) != 2 {
		t.Errorf("the group = %+v, want the two copies left", d.Linked)
	}
	if len(card.Books) != 1 {
		t.Errorf("the card's rows of the others = %+v, want the one left", card.Books)
	}

	f = linkedThree(t)
	tl := decode[view.ContactTimelinePage](t, f.h.do(get(f.record(f.id, peopleBook, "ada-here")+"/timeline", f.sess)))
	said(t, tl.Shell.Notices)
	if tl.Linked == nil || tl.Linked.Label != "in 2 books" {
		t.Errorf("timeline linked = %+v, want «in 2 books»", tl.Linked)
	}
}

// The screen of duplicates says it on the card of a group that lost one of
// its copies and stands (the owner's choice of 15.11.2026: in the group's
// card, under its rows — not above the list, where a group that came apart
// whole is said, having no card left): where the copy was, in the words of
// the record's card, and that the others stay linked. Said once, as there.
func TestTheDuplicatesScreenSaysAGroupLostACopyInItsCard(t *testing.T) {
	f := linkedThree(t)
	page := f.page(t)
	g := onlyGroup(t, page)
	if g.Badge.Label != "linked ×2" {
		t.Errorf("badge = %q, want the two copies left", g.Badge.Label)
	}
	const tail = " is no longer at its address (deleted or moved). The other copies stay linked and are unchanged on the server."
	if !strings.HasPrefix(string(g.Hint), "The linked copy in ") || !strings.HasSuffix(string(g.Hint), tail) {
		t.Errorf("the group's card says %q", g.Hint)
	}
	for _, n := range page.Shell.Notices {
		if strings.Contains(string(n.Title), "linked group") {
			t.Errorf("said above the list too: %+v", n)
		}
	}
	if again := onlyGroup(t, f.page(t)); strings.Contains(string(again.Hint), "no longer at its address") {
		t.Errorf("said on every read: %q", again.Hint)
	}
}

// The timeline of a person whose group came apart whole says so too.
func TestATimelineSaysItsGroupCameApart(t *testing.T) {
	f := newDupFixture(t)
	f.load(t)
	g := onlyGroup(t, f.page(t))
	pick := choiceValue(t, g.Choices[0], "LOVELACE Ada")
	f.h.do(decideRequest(t, f.sess, g, "duplicates.link", asItStands(g, url.Values{"choice.FN": {pick}})))
	_, etag, _ := f.other.Object(workBook + "ada-there.vcf")
	f.h.do(postForm(f.record(f.otherID, workBook, "ada-there")+"/delete", f.sess, url.Values{"etag": {etag}, "sheet": {"1"}, "copy": {f.otherID + "/" + url.PathEscape(workBook)}}))
	tl := decode[view.ContactTimelinePage](t, f.h.do(get(f.record(f.id, peopleBook, "ada-here")+"/timeline", f.sess)))
	if noticeTitled(tl.Shell.Notices, "Link removed.") == nil {
		t.Errorf("the timeline says nothing: %+v", tl.Shell.Notices)
	}
}
