package web_test

import (
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// The words of s-dup-1 and s-dup-2 that depend on the data (Дб1, Дб13,
// Дб16): what was compared and how many groups wait, under the title of the
// groups; whom the merge is of and how many records, under its title; the
// plan a message of no alarm — the alarm is the button's.
func TestTheDuplicatesSayWhatTheyCompared(t *testing.T) {
	f := newDupFixture(t)
	if p := f.page(t); p.Subtitle != "0 records compared" {
		t.Errorf("nothing loaded: subtitle %q", p.Subtitle)
	}
	f.load(t)
	p := f.page(t)
	if p.Subtitle != "4 records compared · 1 to look at" {
		t.Errorf("subtitle %q", p.Subtitle)
	}
	m := openMerge(t, f.h, f.sess, actionOf(t, onlyGroup(t, p), "duplicates.merge"), nil)
	if want := string(m.Target.Title) + " · 2 records"; string(m.Subtitle) != want || m.Target.Title == "" {
		t.Errorf("merge subtitle %q, want %q", m.Subtitle, want)
	}
	if m.Warning.Role != view.NoticeInfo {
		t.Errorf("the plan is said as %q, want info", m.Warning.Role)
	}
}

// A photo is told by where its record lives, the source whole (Дб8, s-dup-1:
// «nextcloud · Архив», «radicale · vps · Команда»), not by the server alone.
func TestAPhotoIsToldByItsSource(t *testing.T) {
	f := newDupFixture(t)
	f.srv.Seed(peopleBook+"ada-here.vcf", strings.Replace(adaHere, "END:VCARD", "PHOTO:data:image/png;base64,AAAA\nEND:VCARD", 1))
	f.other.Seed(workBook+"ada-there.vcf", strings.Replace(adaThere, "END:VCARD", "PHOTO:data:image/png;base64,BBBB\nEND:VCARD", 1))
	f.load(t)
	g := onlyGroup(t, f.page(t))
	for _, c := range g.Choices {
		if c.Property != "Photo" {
			continue
		}
		for _, o := range c.Options {
			if !strings.HasSuffix(string(o), " · People") && !strings.HasSuffix(string(o), " · Work") {
				t.Errorf("photo option %q: not its source", o)
			}
		}
		return
	}
	t.Fatalf("no photo question in %+v", g.Choices)
}
