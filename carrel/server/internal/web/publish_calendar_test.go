package web_test

import (
	"net/url"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// review Р-12 (s-publish-2): a published calendar is asked how much of it
// the link shows and whether its .ics may be downloaded and subscribed to;
// what of its events the visitor sees is the ticks of Б39 (the text of the
// events among them: «Show event descriptions» became «Description»).
func TestPublishCalendarOptions(t *testing.T) {
	f := publishFixture(t)
	href := f.publishHref("calendar", searchWork, "")

	sheet := decode[view.PublishSheet](t, f.h.do(get(href, f.sess)))
	for _, name := range []string{"how", "from", "to", "lives", "download", "time", "title", "description"} {
		if rowNamed(t, sheet.Form, name).Name != name {
			t.Fatalf("the sheet has no %q: %+v", name, sheet.Form)
		}
	}
	// A date range is where it opens; neither the file (the owner's
	// decision of 23.10.2026: the download is off for every kind until
	// ticked) nor the text of the events (24.09.2026) unless ticked.
	if rowNamed(t, sheet.Form, "how").Value != "range" || rowNamed(t, sheet.Form, "download").Value != "" || rowNamed(t, sheet.Form, "description").Value != "" {
		t.Errorf("the sheet does not start as drawn: %+v", sheet.Form)
	}
	// The days stand whichever is chosen, from today to the month's last
	// day, and the whole calendar switches them off (Пб6, Пб7; s-publish-2).
	whole := decode[view.PublishSheet](t, f.h.do(get(href+"&how=whole", f.sess)))
	how := rowNamed(t, whole.Form, "how")
	if how.Value != "whole" || len(how.Options) != 2 || how.Options[1].Disables != "from to" || how.Options[0].Disables != "" {
		t.Errorf("how = %+v", how)
	}
	if from, to := rowNamed(t, whole.Form, "from").Value, rowNamed(t, whole.Form, "to").Value; from == "" || to == "" || to < from || string(to[8:]) < "28" {
		t.Errorf("the days open %q – %q", from, to)
	}

	// A range without its days is asked again, not guessed.
	create := actionHref(t, sheet.Actions, "publish.create")
	again := decode[view.PublishSheet](t, f.h.do(postForm(create, f.sess, url.Values{"how": {"range"}, "from": {"2026-10-01"}, "to": {""}, "download": {"on"}})))
	if again.Link != "" || rowNamed(t, again.Form, "to").Error != "Give both days, or choose The whole calendar." || rowNamed(t, again.Form, "from").Error != "" ||
		rowNamed(t, again.Form, "from").Value != "2026-10-01" {
		t.Errorf("a range without a day made a link, or said so elsewhere: %+v", again)
	}

	// The days of the link, no file, no descriptions.
	made := decode[view.PublishSheet](t, f.h.do(postForm(create, f.sess, url.Values{
		"how": {"range"}, "from": {"2026-08-10"}, "to": {"2026-08-10"},
	})))
	link := string(made.Link)
	if !strings.HasPrefix(link, "/p/") {
		t.Fatalf("no link: %+v", made)
	}
	page := publicPage(t, f, link)
	if len(page.Days) != 1 {
		t.Errorf("the page shows %d days, want the one the link names", len(page.Days))
	}
	if page.Range == nil || len(page.Range.Presets) != 0 {
		t.Errorf("the reader can step out of the link's days: %+v", page.Range)
	}
	if actionByID(page.Actions, "published.download") != nil {
		t.Errorf("the file is offered although the link refused it: %+v", page.Actions)
	}
	if rec := f.h.do(anon(link + "/download")); rec.Code != 404 {
		t.Errorf("the .ics is served although the link refused it: %d", rec.Code)
	}

	// With the file, without the descriptions.
	made = decode[view.PublishSheet](t, f.h.do(postForm(create, f.sess, url.Values{
		"how": {"whole"}, "download": {"on"},
	})))
	link = string(made.Link)
	if actionByID(publicPage(t, f, link).Actions, "published.download") == nil {
		t.Fatal("the file was allowed and is not offered")
	}
	rec := f.h.do(anon(link + "/download"))
	if rec.Code != 200 {
		t.Fatalf("download: %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "DESCRIPTION") {
		t.Errorf("the file carries the text the link left out:\n%s", rec.Body.String())
	}
	mustContain(t, rec.Body.String(), "SUMMARY:Standup")

	// A plain publish — no form at all — is what it was: everything.
	made = decode[view.PublishSheet](t, f.h.do(postForm(create, f.sess, url.Values{})))
	page = publicPage(t, f, string(made.Link))
	if actionByID(page.Actions, "published.download") == nil || page.Range == nil || len(page.Range.Presets) == 0 {
		t.Errorf("a publish without the form lost something: %+v", page.Actions)
	}
}

// from-typecase note 24: the publish sheet (s-publish-2) says «local» in
// its head as the published links do (s-publish-1). A sheet has no shell,
// so it carries the instance's flag itself — the same as SignedIn.Local.
func TestPublishSheetSaysLocal(t *testing.T) {
	badge := regexp.MustCompile(`<div class="tc-sheet__head">[^
]*?<span class="tc-badge is-local"[^>]*>local</span>`)
	if !badge.MatchString(mockupFrame(t, "s-publish-2")) {
		t.Fatal("s-publish-2 draws no «local» in the sheet's head: read the frame again")
	}
	f := publishFixture(t)
	href := f.publishHref("calendar", searchWork, "")
	for _, c := range []struct {
		public string
		local  bool
	}{{"", true}, {"https://carrel.lan", true}, {"https://carrel.example.org", false}} {
		f.h.cfg.PublicURL = c.public
		if sheet := decode[view.PublishSheet](t, f.h.do(get(href, f.sess))); sheet.Local != c.local {
			t.Errorf("CARREL_PUBLIC_URL %q: local = %v, want %v", c.public, sheet.Local, c.local)
		}
	}
}
