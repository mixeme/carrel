package web_test

import (
	"net/url"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// views-tail.md (s-publish-2): a published calendar is asked how much of it
// the link shows, whether its .ics may be downloaded and subscribed to, and
// whether the text of its events goes with the times.
func TestPublishCalendarOptions(t *testing.T) {
	f := publishFixture(t)
	href := f.publishHref("calendar", searchWork, "")

	sheet := decode[view.PublishSheet](t, f.h.do(get(href, f.sess)))
	for _, name := range []string{"from", "to", "download", "descriptions"} {
		if rowNamed(t, sheet.Form, name).Name != name {
			t.Fatalf("the sheet has no %q: %+v", name, sheet.Form)
		}
	}
	if rowNamed(t, sheet.Form, "download").Value != "on" || rowNamed(t, sheet.Form, "descriptions").Value != "on" {
		t.Errorf("the switches do not start on: %+v", sheet.Form)
	}

	// The days of the link, no file, no descriptions.
	create := actionHref(t, sheet.Actions, "publish.create")
	made := decode[view.PublishSheet](t, f.h.do(postForm(create, f.sess, url.Values{
		"from": {"2026-08-10"}, "to": {"2026-08-10"}, "descriptions": {""},
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
		"download": {"on"}, "descriptions": {""},
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
