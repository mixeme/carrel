package web_test

import (
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Нс27 of the scan of 15.10.2026 (s-profile-10): the panel of a
// subscription is the frame's — the address and its hint, the name here and
// its hint, the label's colour derived from the address, the tick «This feed
// asks for a password» with the login and the password shown only while it
// is ticked, the sections a check found, the warning under the form, and
// Subscribe with Cancel.
func TestTheSubscribePanelIsTheFrame(t *testing.T) {
	frame := mockupFrame(t, "s-profile-10")
	panel := frame[strings.Index(frame, "Subscribe to a feed</div>"):]
	hints := map[string]string{}
	for _, m := range regexp.MustCompile(`(?s)<label>([^<]*)</label>.*?<span class="tc-field__hint">(.*?)</span>\s*</div>`).FindAllStringSubmatch(panel, -1) {
		if _, ok := hints[frameText(m[1])]; !ok {
			// A piece in mono ends with its tag, not a word break
			// («https://, webcal://»; the hint is Text, Нс3).
			hints[frameText(m[1])] = frameText(strings.ReplaceAll(m[2], "</span>", ""))
		}
	}
	tickHint := regexp.MustCompile(`(?s)This feed asks for a password</label>\s*<span class="tc-field__hint">(.*?)</span>\s*</div>`).FindStringSubmatch(panel)
	warn := regexp.MustCompile(`(?s)<div class="tc-msg is-alert[^"]*">(.*?)</div>`).FindStringSubmatch(panel)
	if len(hints) < 4 || tickHint == nil || warn == nil {
		t.Fatalf("s-profile-10 reads otherwise: %v", hints)
	}

	f := feedHarness(t)
	feed := newFeedServer(t, holidays, "")
	page := decode[view.ConnectionsPage](t, f.h.do(get("/app/settings/connections?op=feed", f.sess)))
	form := *page.Add
	for _, c := range []struct{ name, label string }{{"url", "Feed address"}, {"name", "Name here"}, {"colour", "Label colour"}} {
		r := formRow(form, c.name)
		if r == nil || string(r.Label) != c.label || string(r.Hint) != hints[c.label] {
			t.Errorf("%s = %+v, the frame's hint %q", c.name, r, hints[c.label])
		}
	}
	if r := formRow(form, "colour"); r.ValueAside != "derived from the address" || len(r.Options) != 6 {
		t.Errorf("colour = %+v", r)
	}
	asks := formRow(form, "asks")
	if asks == nil || asks.Kind != view.InputCheck || string(asks.Hint) != frameText(tickHint[1]) {
		t.Errorf("asks = %+v", asks)
	}
	for _, n := range []string{"login", "password"} {
		if r := formRow(form, n); r == nil || !r.Hidden || !slices.Equal(r.Changed, []string{"asks"}) {
			t.Errorf("%s = %+v, want hidden until the tick", n, r)
		}
	}
	if string(page.AddNote) != frameText(warn[1]) {
		t.Errorf("warning = %q, the frame %q", page.AddNote, frameText(warn[1]))
	}
	if actionByID(form.Actions, "feed.cancel") == nil {
		t.Error("no Cancel")
	}
	if formRow(form, "sections") != nil {
		t.Error("sections before a check")
	}

	// A check says the sections, before subscribing.
	checked := decode[view.ConnectionsPage](t, f.h.do(postForm("/app/settings/connections?op=feed.check", f.sess, url.Values{"url": {feed.URL + "/ru.ics"}})))
	if r := formRow(*checked.Add, "sections"); r == nil || !r.ReadOnly || len(r.Options) != 2 || r.Options[0].Label != "Calendar · 1" ||
		string(r.Hint) != hints["Sections"] {
		t.Errorf("sections = %+v, the frame's hint %q", r, hints["Sections"])
	}

	// The tick off: a login typed is not kept.
	sub := subscribe(t, f, url.Values{"url": {feed.URL + "/ru.ics"}, "asks": {""}, "login": {"someone"}, "colour": {"#3f6e8c"}})
	if sub.Login != "" || sub.Colour != "#3f6e8c" {
		t.Errorf("subscribed = login %q, colour %q", sub.Login, sub.Colour)
	}
}
