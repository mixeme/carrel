package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/emersion/go-webdav/caldav"

	"carrel.dev/carrel/server/internal/davtest"
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

// Under the rows of a group of events with different UIDs the screen says
// what that means (s-dup-1, «Дежурство»; Дб6): the agenda shows it twice,
// and here the person decides. A group of contacts says nothing.
func TestAGroupOfEventsSaysWhatItMeans(t *testing.T) {
	srv := searchServer(t)
	duty := davtest.CalendarHome + "duty/"
	srv.AddCalendar(caldav.Calendar{Path: searchWork, Name: "Work"})
	srv.AddCalendar(caldav.Calendar{Path: duty, Name: "Duty"})
	srv.Seed(searchWork+"e1.ics", ics("BEGIN:VEVENT\nUID:e1\nDTSTAMP:20260801T000000Z\nSUMMARY:Standup\nDTSTART:20260810T070000Z\nDTEND:20260810T080000Z\nEND:VEVENT"))
	srv.Seed(duty+"e2.ics", ics("BEGIN:VEVENT\nUID:e2\nDTSTAMP:20260801T000000Z\nSUMMARY:Standup\nDTSTART:20260810T070000Z\nDTEND:20260810T080000Z\nEND:VEVENT"))
	f := connectFixture(t, srv, searchWork, nil)
	for _, col := range []string{searchWork, duty} {
		f.h.do(get("/app/calendar/"+f.id+"/"+url.PathEscape(col)+"?from=2026-08-10", f.sess))
	}
	g := onlyGroup(t, decode[view.DuplicatesPage](t, f.h.do(get("/app/duplicates", f.sess))))
	if g.Hint != "The same event in two calendars, different UIDs. Agenda shows it twice; here you decide whether to keep both or write one and delete the other." {
		t.Errorf("hint %q", g.Hint)
	}

	d := newDupFixture(t)
	d.load(t)
	if g := onlyGroup(t, d.page(t)); g.Hint != "" {
		t.Errorf("a group of contacts says %q", g.Hint)
	}
}

// A single value one record has and the other has not is a choice too
// (Дб7; s-dup-1: Вера's «Organisation» — «—» or «Библиотека №4», the second
// chosen): the value is chosen beforehand, and «—» chosen leaves the
// property out of the merged record.
func TestAnEmptyValueIsAChoice(t *testing.T) {
	f := newDupFixture(t)
	f.load(t)
	g := onlyGroup(t, f.page(t))
	var org *view.MergeRow
	for i := range g.Choices {
		if g.Choices[i].Name == "choice.ORG" {
			org = &g.Choices[i]
		}
	}
	if org == nil || len(org.Options) != 2 || org.Property != "Organisation" {
		t.Fatalf("no question about the organisation: %+v", g.Choices)
	}
	empty := optionIndex(t, *org, "")
	if org.Chosen == empty || !strings.HasPrefix(string(org.Options[org.Chosen]), "Analytical Engines · ") {
		t.Errorf("chosen beforehand %d of %q, want the value", org.Chosen, org.Options)
	}
	p := openMerge(t, f.h, f.sess, actionOf(t, g, "duplicates.merge"), url.Values{"choice.ORG": {org.Values[empty]}})
	confirm := mergeAction(t, p, "duplicates.merge.confirm")
	if rec := f.h.do(postForm(confirm.Href, f.sess, url.Values{})); rec.Code != http.StatusSeeOther {
		t.Fatalf("merge: %d %s", rec.Code, rec.Body.String())
	}
	kept, _, ok := f.srv.Object(peopleBook + "ada-here.vcf")
	if !ok {
		kept, _, _ = f.other.Object(workBook + "ada-there.vcf")
	}
	if strings.Contains(unfolded(kept), "ORG") {
		t.Errorf("«—» was chosen, and the merged record holds an organisation:\n%s", kept)
	}
}
