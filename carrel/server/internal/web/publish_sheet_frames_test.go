package web_test

import (
	"net/url"
	"regexp"
	"strings"
	"testing"

	"carrel.dev/carrel/server/view"
)

// Note 70 of Typecase (task 163; s-publish-2, -19; the owner's decision
// Б39): the sheet of a calendar asks how much of it, then what the visitor
// sees — the time always, «always shown» beside it, the title on, the rest
// off, and under them that without the title an event shows as «Busy» —,
// then whether its .ics goes, then how long the link lives. One event is the
// same sheet without «How much of it», its .ics asked too.
func TestPublishSheetIsTheFrames(t *testing.T) {
	hint := func(id string) string {
		m := regexp.MustCompile(`(?s)What the visitor sees</label>.*?<span class="tc-field__hint">(.*?)</span>`).FindStringSubmatch(mockupFrame(t, id))
		if m == nil {
			t.Fatalf("%s draws no hint under What the visitor sees: read the frame again", id)
		}
		return frameText(m[1])
	}
	names := func(set view.FieldSet) []string {
		var out []string
		for _, r := range set.Rows {
			if r.Kind != view.InputHidden {
				out = append(out, r.Name)
			}
		}
		return out
	}
	f := publishFixture(t)
	check := func(name string, sheet view.PublishSheet, want [][]string, frame string) {
		t.Helper()
		var got [][]string
		for _, set := range sheet.Form.Sets {
			got = append(got, names(set))
		}
		if len(got) != len(want) {
			t.Fatalf("%s: sets = %v, want %v", name, got, want)
		}
		for i := range want {
			if strings.Join(got[i], ",") != strings.Join(want[i], ",") {
				t.Errorf("%s: set %d = %v, want %v", name, i, got[i], want[i])
			}
		}
		var sees view.FieldSet
		for _, set := range sheet.Form.Sets {
			if set.Legend == "What the visitor sees" {
				sees = set
			}
		}
		if string(sees.Hint) != hint(frame) {
			t.Errorf("%s: hint = %q, %s draws %q", name, sees.Hint, frame, hint(frame))
		}
		if tm := sees.Rows[0]; tm.Name != "time" || tm.LabelAside != "always shown" || tm.Hint != "" || !tm.ReadOnly {
			t.Errorf("%s: Time = %+v", name, tm)
		}
	}
	calendar := decode[view.PublishSheet](t, f.h.do(get(f.publishHref("calendar", searchWork, ""), f.sess)))
	check("calendar", calendar, [][]string{{"how", "from", "to"}, {"time", "title", "location", "description", "attendees", "attachments", "tags"}, {"download", "lives"}}, "s-publish-2")
	event := decode[view.PublishSheet](t, f.h.do(get(f.publishHref("calendar", searchWork, "e1"), f.sess)))
	check("event", event, [][]string{{"time", "title", "location", "description", "attendees", "attachments", "tags"}, {"download", "lives"}}, "s-publish-19")
	// Off at first, as every kind's (the owner's decision of 23.10.2026).
	if rowNamed(t, event.Form, "download").Value != "" {
		t.Errorf("one event's .ics is on at first")
	}

	// One event's .ics goes when ticked, and not when not.
	create := actionHref(t, event.Actions, "publish.create")
	for _, c := range []struct {
		form url.Values
		file bool
	}{
		{url.Values{"lives": {"30"}, "title": {"on"}, "download": {"on"}}, true},
		{url.Values{"lives": {"30"}, "title": {"on"}}, false},
	} {
		made := decode[view.PublishSheet](t, f.h.do(postForm(create, f.sess, c.form)))
		page := publicPage(t, f, string(made.Link))
		if got := actionByID(page.Actions, "published.download") != nil; got != c.file {
			t.Errorf("%v: the .ics offered = %v", c.form, got)
		}
	}
}
